package monitor

import (
	"sync"
)

// RateLimiterPool manages shared rate limiters to prevent resource leakage
// Implements Resource Pool pattern for SOLID compliance
type RateLimiterPool struct {
	limiters map[float64]*RateLimitedExecutor
	mu       sync.RWMutex
}

// NewRateLimiterPool creates a new rate limiter pool
func NewRateLimiterPool() *RateLimiterPool {
	return &RateLimiterPool{
		limiters: make(map[float64]*RateLimitedExecutor),
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

// Close closes all rate limiters in the pool
// Implements proper resource cleanup
func (p *RateLimiterPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for rate, limiter := range p.limiters {
		limiter.Close() // Close() doesn't return error
		delete(p.limiters, rate)
	}
	
	return nil
}

// Count returns the number of rate limiters in the pool
// Useful for monitoring and debugging
func (p *RateLimiterPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.limiters)
}