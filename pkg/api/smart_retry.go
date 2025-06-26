package api

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RetryStrategy defines contract for different retry approaches
// Follows Interface Segregation Principle
type RetryStrategy interface {
	Execute(ctx context.Context, fn func() error) error
	IsRetryable(err error) bool
	ShouldFailover(err error) bool
}

// SmartRetryWithFailover implements intelligent retry with URL failover
// Follows Single Responsibility: handles retry logic with URL pool awareness
type SmartRetryWithFailover struct {
	urlPool           *URLPool
	maxRetries        int
	retryDelay        time.Duration
	backoffMultiplier float64
	urlFailureCount   map[string]int // Track failures per URL
	mu                sync.RWMutex
}

// NewSmartRetryWithFailover creates retry strategy with URL pool awareness
func NewSmartRetryWithFailover(urlPool *URLPool, maxRetries int, retryDelay time.Duration) *SmartRetryWithFailover {
	return &SmartRetryWithFailover{
		urlPool:           urlPool,
		maxRetries:        maxRetries,
		retryDelay:        retryDelay,
		backoffMultiplier: 1.5, // Conservative backoff for API rate limits
		urlFailureCount:   make(map[string]int),
	}
}

// Execute with intelligent failover: fast failover + limited retries per URL
func (sr *SmartRetryWithFailover) Execute(ctx context.Context, fn func() error) error {
	if sr.urlPool.Size() == 0 {
		return fmt.Errorf("no URLs available")
	}

	urlsAttempted := make(map[string]bool)
	var lastErr error

	// Try each URL in pool once, with limited retries per URL
	for attempt := 0; attempt < sr.maxRetries && len(urlsAttempted) < sr.urlPool.Size(); attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute function
		err := fn()
		if err == nil {
			sr.recordSuccess() // Reset failure counters on success
			return nil
		}

		lastErr = err
		currentURL := sr.getCurrentURL() // Get URL used in this attempt
		urlsAttempted[currentURL] = true

		// Fast failover: if error suggests API endpoint issue, try next URL immediately
		if sr.ShouldFailover(err) && len(urlsAttempted) < sr.urlPool.Size() {
			sr.recordFailure(currentURL)
			continue // Try next URL without delay
		}

		// Non-retryable error
		if !sr.IsRetryable(err) {
			return err
		}

		// Standard retry with backoff
		if attempt < sr.maxRetries-1 {
			delay := time.Duration(float64(sr.retryDelay) * pow(sr.backoffMultiplier, float64(attempt)))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return fmt.Errorf("all retry attempts exhausted: %w", lastErr)
}

// ShouldFailover determines if error warrants immediate URL switching
func (sr *SmartRetryWithFailover) ShouldFailover(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Immediate failover conditions
	return contains(errStr, "connection refused") ||
		contains(errStr, "timeout") ||
		contains(errStr, "429") || // Rate limited
		contains(errStr, "503") || // Service unavailable
		contains(errStr, "502") || // Bad gateway
		contains(errStr, "504")    // Gateway timeout
}

// IsRetryable follows same logic as SimpleRetry for consistency
func (sr *SmartRetryWithFailover) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Don't retry on auth errors
	if contains(errStr, "401") || contains(errStr, "403") ||
		contains(errStr, "unauthorized") || contains(errStr, "forbidden") {
		return false
	}

	// Don't retry on client errors
	if contains(errStr, "400") || contains(errStr, "404") {
		return false
	}

	return true // Retry on network errors, timeouts, 5xx
}

// recordFailure tracks URL-specific failures (for future health checking)
func (sr *SmartRetryWithFailover) recordFailure(url string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.urlFailureCount[url]++
}

// recordSuccess resets failure counters
func (sr *SmartRetryWithFailover) recordSuccess() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.urlFailureCount = make(map[string]int) // Reset all counters
}

// getCurrentURL gets the URL that would be selected by Next() 
// Note: This is a simplified implementation
func (sr *SmartRetryWithFailover) getCurrentURL() string {
	if sr.urlPool.Size() > 0 {
		urls := sr.urlPool.URLs()
		return urls[0] // Simplified - in real implementation, would track current index
	}
	return ""
}

// GetFailureStats returns URL failure statistics (for monitoring)
func (sr *SmartRetryWithFailover) GetFailureStats() map[string]int {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	result := make(map[string]int)
	for k, v := range sr.urlFailureCount {
		result[k] = v
	}
	return result
}