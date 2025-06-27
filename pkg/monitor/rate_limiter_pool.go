package monitor

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiterPool manages shared rate limiters to prevent resource leakage
// Implements Resource Pool pattern for SOLID compliance
// Enhanced to support per-API-endpoint rate limiting for dual API scenarios
// Now includes atomic concurrency control for precise request management
type RateLimiterPool struct {
	limiters map[float64]*RateLimitedExecutor // Legacy: by rate only
	apiLimiters map[string]*RateLimitedExecutor // New: by API endpoint
	atomicLimiters map[string]*AtomicConcurrencyLimiter // Atomic concurrency control per API
	mu       sync.RWMutex
}

// NewRateLimiterPool creates a new rate limiter pool
func NewRateLimiterPool() *RateLimiterPool {
	return &RateLimiterPool{
		limiters: make(map[float64]*RateLimitedExecutor),
		apiLimiters: make(map[string]*RateLimitedExecutor),
		atomicLimiters: make(map[string]*AtomicConcurrencyLimiter),
	}
}

// GetOrCreate returns an existing rate limiter or creates a new one
// Follows Single Responsibility Principle - only manages rate limiters
func (p *RateLimiterPool) GetOrCreate(requestsPerSecond float64) *RateLimitedExecutor {
	// Try read lock first for better performance
	p.mu.RLock()
	if limiter, exists := p.limiters[requestsPerSecond]; exists {
		p.mu.RUnlock()
		return limiter
	}
	p.mu.RUnlock()
	
	// Need to create new limiter - acquire write lock
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Double-check pattern to prevent race condition
	if limiter, exists := p.limiters[requestsPerSecond]; exists {
		return limiter
	}
	
	// Create new limiter
	limiter := NewRateLimitedExecutor(requestsPerSecond)
	p.limiters[requestsPerSecond] = limiter
	return limiter
}

// GetOrCreateForAPI returns a rate limiter specific to an API endpoint
// This enables proper dual API utilization with independent rate limiting
func (p *RateLimiterPool) GetOrCreateForAPI(apiEndpoint string, requestsPerSecond float64) *RateLimitedExecutor {
	// Create unique key combining endpoint and rate
	key := fmt.Sprintf("%s@%.1f", apiEndpoint, requestsPerSecond)

	// Try read lock first for better performance
	p.mu.RLock()
	if limiter, exists := p.apiLimiters[key]; exists {
		p.mu.RUnlock()
		return limiter
	}
	p.mu.RUnlock()

	// Need to create new limiter - acquire write lock
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check pattern to prevent race condition
	if limiter, exists := p.apiLimiters[key]; exists {
		return limiter
	}

	// Create new limiter for this specific API endpoint
	limiter := NewRateLimitedExecutor(requestsPerSecond)
	p.apiLimiters[key] = limiter
	return limiter
}

// GetOrCreateAtomicLimiter returns an atomic concurrency limiter for specific API endpoint
// This provides precise concurrent request control similar to 1.js implementation
func (p *RateLimiterPool) GetOrCreateAtomicLimiter(apiEndpoint string, maxConcurrent int, timeout time.Duration) *AtomicConcurrencyLimiter {
	// Create unique key combining endpoint and concurrency limit
	key := fmt.Sprintf("%s@%d", apiEndpoint, maxConcurrent)

	// Try read lock first for better performance
	p.mu.RLock()
	if limiter, exists := p.atomicLimiters[key]; exists {
		p.mu.RUnlock()
		return limiter
	}
	p.mu.RUnlock()

	// Need to create new limiter - acquire write lock
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check pattern to prevent race condition
	if limiter, exists := p.atomicLimiters[key]; exists {
		return limiter
	}

	// Create new atomic limiter for this specific API endpoint
	limiter := NewAtomicConcurrencyLimiter(maxConcurrent, timeout)
	p.atomicLimiters[key] = limiter
	return limiter
}

// GetAtomicLimiterStats returns statistics for all atomic limiters
// Useful for monitoring and debugging concurrent API usage
func (p *RateLimiterPool) GetAtomicLimiterStats() map[string]ConcurrencyStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]ConcurrencyStats)
	for key, limiter := range p.atomicLimiters {
		stats[key] = limiter.GetStats()
	}
	return stats
}

// Close closes all rate limiters in the pool
// Implements proper resource cleanup
func (p *RateLimiterPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close legacy limiters
	for rate, limiter := range p.limiters {
		limiter.Close() // Close() doesn't return error
		delete(p.limiters, rate)
	}

	// Close API-specific limiters
	for key, limiter := range p.apiLimiters {
		limiter.Close()
		delete(p.apiLimiters, key)
	}

	// Clear atomic limiters (no explicit close needed for atomic operations)
	for key := range p.atomicLimiters {
		delete(p.atomicLimiters, key)
	}

	return nil
}

// Count returns the number of rate limiters in the pool
// Useful for monitoring and debugging
func (p *RateLimiterPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.limiters) + len(p.apiLimiters)
}

// CountByType returns counts for different limiter types
func (p *RateLimiterPool) CountByType() (legacy int, apiSpecific int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.limiters), len(p.apiLimiters)
}