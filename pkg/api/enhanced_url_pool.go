package api

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// URLHealth represents health status of an API endpoint
type URLHealth struct {
	URL            string    `json:"url"`
	IsHealthy      bool      `json:"is_healthy"`
	LastSuccess    time.Time `json:"last_success"`
	LastFailure    time.Time `json:"last_failure"`
	ConsecutiveFails int     `json:"consecutive_fails"`
	TotalRequests  int64     `json:"total_requests"`
	SuccessCount   int64     `json:"success_count"`
	FailureCount   int64     `json:"failure_count"`
}

// HealthChecker defines interface for URL health monitoring
// Follows Interface Segregation Principle
type HealthChecker interface {
	IsHealthy(url string) bool
	RecordSuccess(url string)
	RecordFailure(url string)
	GetHealthStats() map[string]URLHealth
}

// EnhancedURLPool with health checking and weighted selection
// Follows Single Responsibility: URL selection with health awareness
type EnhancedURLPool struct {
	urls         []string
	healthMap    map[string]*URLHealth
	current      int64
	mu           sync.RWMutex
	healthyOnly  bool // Whether to skip unhealthy URLs
	failureThreshold int // Consecutive failures before marking unhealthy
}

// NewEnhancedURLPool creates URL pool with health monitoring
func NewEnhancedURLPool(urlString string, healthyOnly bool) *EnhancedURLPool {
	basePool := NewURLPool(urlString)
	
	healthMap := make(map[string]*URLHealth)
	for _, url := range basePool.URLs() {
		healthMap[url] = &URLHealth{
			URL:          url,
			IsHealthy:    true, // Start optimistic
			LastSuccess:  time.Now(),
			LastFailure:  time.Time{},
			ConsecutiveFails: 0,
		}
	}

	return &EnhancedURLPool{
		urls:            basePool.URLs(),
		healthMap:       healthMap,
		current:         -1,
		healthyOnly:     healthyOnly,
		failureThreshold: 3, // Mark unhealthy after 3 consecutive failures
	}
}

// Next returns next healthy URL with fallback to any URL if none healthy
func (p *EnhancedURLPool) Next() string {
	if len(p.urls) == 0 {
		return ""
	}

	// Fast path for single URL
	if len(p.urls) == 1 {
		return p.urls[0]
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try to find healthy URL
	if p.healthyOnly {
		healthyURLs := p.getHealthyURLs()
		if len(healthyURLs) > 0 {
			next := atomic.AddInt64(&p.current, 1)
			index := int(next % int64(len(healthyURLs)))
			return healthyURLs[index]
		}
	}

	// Fallback to round-robin on all URLs (even if unhealthy)
	next := atomic.AddInt64(&p.current, 1)
	index := int(((next % int64(len(p.urls))) + int64(len(p.urls))) % int64(len(p.urls)))
	return p.urls[index]
}

// IsHealthy checks if URL is considered healthy
func (p *EnhancedURLPool) IsHealthy(url string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if health, exists := p.healthMap[url]; exists {
		return health.IsHealthy
	}
	return true // Unknown URLs are optimistically healthy
}

// RecordSuccess updates health stats for successful request
func (p *EnhancedURLPool) RecordSuccess(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if health, exists := p.healthMap[url]; exists {
		atomic.AddInt64(&health.TotalRequests, 1)
		atomic.AddInt64(&health.SuccessCount, 1)
		health.LastSuccess = time.Now()
		health.ConsecutiveFails = 0
		health.IsHealthy = true // Recovery
	}
}

// RecordFailure updates health stats for failed request
func (p *EnhancedURLPool) RecordFailure(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if health, exists := p.healthMap[url]; exists {
		atomic.AddInt64(&health.TotalRequests, 1)
		atomic.AddInt64(&health.FailureCount, 1)
		health.LastFailure = time.Now()
		health.ConsecutiveFails++
		
		// Mark unhealthy if too many consecutive failures
		if health.ConsecutiveFails >= p.failureThreshold {
			health.IsHealthy = false
		}
	}
}

// GetHealthStats returns health information for all URLs
func (p *EnhancedURLPool) GetHealthStats() map[string]URLHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]URLHealth)
	for url, health := range p.healthMap {
		// Create copy to avoid race conditions
		result[url] = URLHealth{
			URL:             health.URL,
			IsHealthy:       health.IsHealthy,
			LastSuccess:     health.LastSuccess,
			LastFailure:     health.LastFailure,
			ConsecutiveFails: health.ConsecutiveFails,
			TotalRequests:   atomic.LoadInt64(&health.TotalRequests),
			SuccessCount:    atomic.LoadInt64(&health.SuccessCount),
			FailureCount:    atomic.LoadInt64(&health.FailureCount),
		}
	}
	return result
}

// getHealthyURLs returns list of currently healthy URLs (caller must hold lock)
func (p *EnhancedURLPool) getHealthyURLs() []string {
	var healthy []string
	for _, url := range p.urls {
		if health, exists := p.healthMap[url]; exists && health.IsHealthy {
			healthy = append(healthy, url)
		}
	}
	return healthy
}

// StartHealthRecovery starts background goroutine to periodically recover URLs
// Follows Open/Closed Principle - extensible without modifying core logic
func (p *EnhancedURLPool) StartHealthRecovery(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.attemptHealthRecovery()
			}
		}
	}()
}

// attemptHealthRecovery marks URLs as healthy if enough time has passed
func (p *EnhancedURLPool) attemptHealthRecovery() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	recoveryWindow := 5 * time.Minute // URLs can recover after 5 minutes

	for _, health := range p.healthMap {
		if !health.IsHealthy && now.Sub(health.LastFailure) > recoveryWindow {
			health.IsHealthy = true // Give it another chance
			health.ConsecutiveFails = 0
		}
	}
}

// Size returns total number of URLs
func (p *EnhancedURLPool) Size() int {
	return len(p.urls)
}

// HealthySize returns number of currently healthy URLs
func (p *EnhancedURLPool) HealthySize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.getHealthyURLs())
}