package monitor

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"sitemap-go/pkg/logger"
)

// AtomicConcurrencyLimiter provides thread-safe concurrency control using atomic operations
// Inspired by 1.js distributed lock mechanism for precise concurrent request management
// Follows Single Responsibility Principle - only manages concurrent access permits
type AtomicConcurrencyLimiter struct {
	maxConcurrent  int64         // Maximum allowed concurrent operations
	current        int64         // Current number of active operations (atomic)
	acquireTimeout time.Duration // Maximum time to wait for permit acquisition
	log            *logger.Logger
	
	// Metrics for monitoring
	totalAcquires   int64 // Total acquire attempts (atomic)
	totalReleases   int64 // Total releases (atomic)
	timeoutFailures int64 // Failed acquisitions due to timeout (atomic)
}

// NewAtomicConcurrencyLimiter creates a new atomic concurrency limiter
// maxConcurrent: maximum number of concurrent operations allowed
// acquireTimeout: maximum time to wait for permit acquisition
func NewAtomicConcurrencyLimiter(maxConcurrent int, acquireTimeout time.Duration) *AtomicConcurrencyLimiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // Default safe value
	}
	if acquireTimeout <= 0 {
		acquireTimeout = 5 * time.Second // Default timeout
	}

	return &AtomicConcurrencyLimiter{
		maxConcurrent:  int64(maxConcurrent),
		current:        0,
		acquireTimeout: acquireTimeout,
		log:            logger.GetLogger().WithField("component", "atomic_concurrency_limiter"),
	}
}

// Acquire attempts to acquire a concurrency permit with timeout
// Returns error if unable to acquire within timeout period
// Thread-safe using atomic compare-and-swap operations
func (acl *AtomicConcurrencyLimiter) Acquire(ctx context.Context) error {
	atomic.AddInt64(&acl.totalAcquires, 1)
	
	// Fast path: try immediate acquisition
	if acl.tryAcquire() {
		return nil
	}

	// Slow path: wait with timeout and jitter
	deadline := time.Now().Add(acl.acquireTimeout)
	retryCount := 0
	maxRetries := 50 // Limit retries to prevent infinite loops

	for time.Now().Before(deadline) && retryCount < maxRetries {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to acquire permit
		if acl.tryAcquire() {
			if retryCount > 0 {
				acl.log.WithField("retry_count", retryCount).Debug("Acquired permit after retries")
			}
			return nil
		}

		// Wait with exponential backoff and jitter (similar to 1.js)
		retryCount++
		baseDelay := time.Duration(retryCount) * 5 * time.Millisecond
		jitter := time.Duration(retryCount) * 2 * time.Millisecond
		delay := baseDelay + jitter

		// Cap maximum delay
		if delay > 50*time.Millisecond {
			delay = 50 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue retry loop
		}
	}

	// Timeout reached - record failure
	atomic.AddInt64(&acl.timeoutFailures, 1)
	current := atomic.LoadInt64(&acl.current)

	// Determine if it was context timeout or acquire timeout
	timeoutType := "acquire_timeout"
	if ctx.Err() != nil {
		timeoutType = "context_timeout"
	}

	acl.log.WithFields(map[string]interface{}{
		"current_concurrent": current,
		"max_concurrent":     acl.maxConcurrent,
		"retry_count":        retryCount,
		"timeout_type":       timeoutType,
		"acquire_timeout":    acl.acquireTimeout,
	}).Warn("Failed to acquire concurrency permit within timeout")

	return fmt.Errorf("failed to acquire concurrency permit within %v (current: %d, max: %d, type: %s)",
		acl.acquireTimeout, current, acl.maxConcurrent, timeoutType)
}

// tryAcquire attempts to acquire a permit using atomic compare-and-swap
// Returns true if successful, false if at capacity
func (acl *AtomicConcurrencyLimiter) tryAcquire() bool {
	for {
		current := atomic.LoadInt64(&acl.current)
		
		// Check if at capacity
		if current >= acl.maxConcurrent {
			return false
		}
		
		// Try to increment atomically
		if atomic.CompareAndSwapInt64(&acl.current, current, current+1) {
			return true // Successfully acquired
		}
		
		// CAS failed due to concurrent modification, retry
		// This is expected in high-concurrency scenarios
	}
}

// Release releases a concurrency permit
// Must be called after successful Acquire() to prevent permit leakage
// Thread-safe using atomic operations
func (acl *AtomicConcurrencyLimiter) Release() {
	atomic.AddInt64(&acl.totalReleases, 1)
	
	for {
		current := atomic.LoadInt64(&acl.current)
		
		// Prevent underflow
		if current <= 0 {
			acl.log.Warn("Attempted to release permit when none were held")
			return
		}
		
		// Try to decrement atomically
		if atomic.CompareAndSwapInt64(&acl.current, current, current-1) {
			return // Successfully released
		}
		
		// CAS failed due to concurrent modification, retry
	}
}

// GetStats returns current limiter statistics
// Useful for monitoring and debugging
func (acl *AtomicConcurrencyLimiter) GetStats() ConcurrencyStats {
	return ConcurrencyStats{
		MaxConcurrent:    int(acl.maxConcurrent),
		CurrentActive:    int(atomic.LoadInt64(&acl.current)),
		TotalAcquires:    atomic.LoadInt64(&acl.totalAcquires),
		TotalReleases:    atomic.LoadInt64(&acl.totalReleases),
		TimeoutFailures:  atomic.LoadInt64(&acl.timeoutFailures),
		AcquireTimeout:   acl.acquireTimeout,
	}
}

// UpdateMaxConcurrent dynamically updates the maximum concurrency limit
// Useful for adaptive concurrency management
func (acl *AtomicConcurrencyLimiter) UpdateMaxConcurrent(newMax int) {
	if newMax <= 0 {
		acl.log.WithField("invalid_max", newMax).Warn("Ignoring invalid max concurrent value")
		return
	}
	
	oldMax := atomic.SwapInt64(&acl.maxConcurrent, int64(newMax))
	acl.log.WithFields(map[string]interface{}{
		"old_max": oldMax,
		"new_max": newMax,
	}).Info("Updated maximum concurrency limit")
}

// ConcurrencyStats holds statistics about concurrency limiter performance
type ConcurrencyStats struct {
	MaxConcurrent   int           `json:"max_concurrent"`
	CurrentActive   int           `json:"current_active"`
	TotalAcquires   int64         `json:"total_acquires"`
	TotalReleases   int64         `json:"total_releases"`
	TimeoutFailures int64         `json:"timeout_failures"`
	AcquireTimeout  time.Duration `json:"acquire_timeout"`
}

// UtilizationRate returns the current utilization as a percentage (0-100)
func (cs ConcurrencyStats) UtilizationRate() float64 {
	if cs.MaxConcurrent == 0 {
		return 0
	}
	return float64(cs.CurrentActive) / float64(cs.MaxConcurrent) * 100
}

// SuccessRate returns the acquisition success rate as a percentage (0-100)
func (cs ConcurrencyStats) SuccessRate() float64 {
	if cs.TotalAcquires == 0 {
		return 100 // No attempts yet, assume perfect
	}
	successful := cs.TotalAcquires - cs.TimeoutFailures
	return float64(successful) / float64(cs.TotalAcquires) * 100
}

// AtomicLimiterAdapter adapts AtomicConcurrencyLimiter to api.ConcurrencyLimiter interface
// This allows seamless integration with API clients
type AtomicLimiterAdapter struct {
	limiter *AtomicConcurrencyLimiter
}

// NewAtomicLimiterAdapter creates an adapter for API client integration
func NewAtomicLimiterAdapter(limiter *AtomicConcurrencyLimiter) *AtomicLimiterAdapter {
	return &AtomicLimiterAdapter{limiter: limiter}
}

// Acquire implements api.ConcurrencyLimiter interface
func (a *AtomicLimiterAdapter) Acquire(ctx context.Context) error {
	return a.limiter.Acquire(ctx)
}

// Release implements api.ConcurrencyLimiter interface
func (a *AtomicLimiterAdapter) Release() {
	a.limiter.Release()
}
