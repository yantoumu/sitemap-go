package api

import (
	"context"
	"errors"
	"time"
)

// SimpleRetry provides basic retry logic without circuit breaker complexity
type SimpleRetry struct {
	maxRetries   int
	retryDelay   time.Duration
	backoffMultiplier float64
}

// NewSimpleRetry creates a simple retry mechanism
func NewSimpleRetry(maxRetries int, retryDelay time.Duration) *SimpleRetry {
	return &SimpleRetry{
		maxRetries:        maxRetries,
		retryDelay:        retryDelay,
		backoffMultiplier: 2.0, // Exponential backoff
	}
}

// Execute runs function with simple retry logic
func (sr *SimpleRetry) Execute(ctx context.Context, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= sr.maxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Execute function
		err := fn()
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Don't retry on final attempt
		if attempt == sr.maxRetries {
			break
		}
		
		// Check if error is retryable
		if !sr.isRetryable(err) {
			return err // Non-retryable error
		}
		
		// Calculate delay with exponential backoff
		delay := time.Duration(float64(sr.retryDelay) * pow(sr.backoffMultiplier, float64(attempt)))
		
		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
	
	return lastErr
}

// isRetryable determines if an error should be retried
func (sr *SimpleRetry) isRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Don't retry on auth errors
	if contains(errStr, "401") || contains(errStr, "403") || 
	   contains(errStr, "unauthorized") || contains(errStr, "forbidden") {
		return false
	}
	
	// Don't retry on client errors (4xx except 429)
	if contains(errStr, "400") || contains(errStr, "404") {
		return false
	}
	
	// Retry on network errors, timeouts, 5xx errors, rate limits
	return true
}

// Helper function for case-insensitive string contains
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsIgnoreCase(s, substr)
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			
			// Convert to lowercase
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Simple power function for exponential backoff
func pow(base float64, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}