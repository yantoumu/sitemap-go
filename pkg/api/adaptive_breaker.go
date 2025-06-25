package api

import (
	"context"
	"math"
	"sync"
	"time"

	"sitemap-go/pkg/logger"
)

// AdaptiveCircuitBreaker provides enhanced circuit breaker with adaptive thresholds
type AdaptiveCircuitBreaker struct {
	// Base configuration
	baseFailureThreshold int
	maxFailureThreshold  int
	resetTimeout         time.Duration
	
	// Adaptive parameters
	adaptiveEnabled      bool
	windowSize           time.Duration
	minSamples           int
	
	// State tracking
	mu                   sync.RWMutex
	state                CircuitState
	failures             int
	requests             int
	lastFailTime         time.Time
	successCount         int
	windowStart          time.Time
	
	// Statistics for adaptive behavior
	recentRequests       []RequestRecord
	currentThreshold     int
	log                  *logger.Logger
}

// RequestRecord tracks individual request outcomes
type RequestRecord struct {
	Timestamp time.Time
	Success   bool
	Duration  time.Duration
}

// NewAdaptiveCircuitBreaker creates an adaptive circuit breaker
func NewAdaptiveCircuitBreaker(baseThreshold, maxThreshold int, resetTimeout time.Duration) *AdaptiveCircuitBreaker {
	return &AdaptiveCircuitBreaker{
		baseFailureThreshold: baseThreshold,
		maxFailureThreshold:  maxThreshold,
		resetTimeout:         resetTimeout,
		adaptiveEnabled:      true,
		windowSize:           5 * time.Minute,
		minSamples:           20,
		state:                StateClosed,
		currentThreshold:     baseThreshold,
		recentRequests:       make([]RequestRecord, 0, 1000),
		log:                  logger.GetLogger().WithField("component", "adaptive_breaker"),
	}
}

// Execute runs the function with circuit breaker protection
func (acb *AdaptiveCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := acb.canExecute(); err != nil {
		acb.log.WithField("state", acb.state).Debug("Circuit breaker blocked execution")
		return err
	}
	
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	acb.recordResult(err, duration)
	return err
}

// canExecute checks if the circuit breaker allows execution
func (acb *AdaptiveCircuitBreaker) canExecute() error {
	acb.mu.Lock()
	defer acb.mu.Unlock()
	
	now := time.Now()
	
	switch acb.state {
	case StateClosed:
		acb.requests++
		return nil
		
	case StateOpen:
		if now.Sub(acb.lastFailTime) > acb.resetTimeout {
			acb.log.Info("Circuit breaker transitioning to half-open state")
			acb.state = StateHalfOpen
			acb.successCount = 0
			acb.requests = 1
			return nil
		}
		return ErrCircuitOpen
		
	case StateHalfOpen:
		acb.requests++
		if acb.requests > 10 { // Limit concurrent requests in half-open
			return ErrCircuitOpen
		}
		return nil
	}
	
	return nil
}

// recordResult updates the circuit breaker state based on request outcome
func (acb *AdaptiveCircuitBreaker) recordResult(err error, duration time.Duration) {
	acb.mu.Lock()
	defer acb.mu.Unlock()
	
	now := time.Now()
	record := RequestRecord{
		Timestamp: now,
		Success:   err == nil,
		Duration:  duration,
	}
	
	// Add to recent requests for adaptive behavior
	acb.recentRequests = append(acb.recentRequests, record)
	acb.cleanupOldRecords(now)
	
	if err != nil {
		acb.failures++
		acb.lastFailTime = now
		
		// Update adaptive threshold if enabled
		if acb.adaptiveEnabled {
			acb.updateAdaptiveThreshold()
		}
		
		if acb.state == StateHalfOpen {
			acb.log.WithError(err).Warn("Request failed in half-open state, opening circuit")
			acb.state = StateOpen
		} else if acb.failures >= acb.currentThreshold {
			acb.log.WithFields(map[string]interface{}{
				"failures":  acb.failures,
				"threshold": acb.currentThreshold,
			}).Warn("Failure threshold exceeded, opening circuit")
			acb.state = StateOpen
		}
	} else {
		// Success case
		if acb.state == StateHalfOpen {
			acb.successCount++
			if acb.successCount >= 5 { // Need multiple successes to close
				acb.log.Info("Circuit breaker closing after successful half-open period")
				acb.state = StateClosed
				acb.failures = 0
				acb.requests = 0
			}
		} else if acb.state == StateClosed {
			// Gradually reduce failure count on success
			if acb.failures > 0 {
				acb.failures = int(math.Max(0, float64(acb.failures)-0.1))
			}
		}
	}
}

// updateAdaptiveThreshold adjusts the failure threshold based on recent performance
func (acb *AdaptiveCircuitBreaker) updateAdaptiveThreshold() {
	if len(acb.recentRequests) < acb.minSamples {
		return
	}
	
	// Calculate recent error rate
	errorCount := 0
	totalCount := len(acb.recentRequests)
	
	for _, record := range acb.recentRequests {
		if !record.Success {
			errorCount++
		}
	}
	
	errorRate := float64(errorCount) / float64(totalCount)
	
	// Adjust threshold based on error rate
	if errorRate > 0.1 { // High error rate, lower threshold
		acb.currentThreshold = int(math.Max(
			float64(acb.baseFailureThreshold)/2,
			float64(acb.baseFailureThreshold)*(1-errorRate),
		))
	} else if errorRate < 0.02 { // Low error rate, raise threshold
		acb.currentThreshold = int(math.Min(
			float64(acb.maxFailureThreshold),
			float64(acb.baseFailureThreshold)*1.5,
		))
	} else {
		acb.currentThreshold = acb.baseFailureThreshold
	}
	
	acb.log.WithFields(map[string]interface{}{
		"error_rate":        errorRate,
		"current_threshold": acb.currentThreshold,
		"samples":           totalCount,
	}).Debug("Updated adaptive threshold")
}

// cleanupOldRecords removes records outside the time window
func (acb *AdaptiveCircuitBreaker) cleanupOldRecords(now time.Time) {
	cutoff := now.Add(-acb.windowSize)
	
	// Find first record within window
	start := 0
	for i, record := range acb.recentRequests {
		if record.Timestamp.After(cutoff) {
			start = i
			break
		}
	}
	
	// Keep only recent records
	if start > 0 {
		acb.recentRequests = acb.recentRequests[start:]
	}
	
	// Limit total records to prevent memory growth
	maxRecords := 1000
	if len(acb.recentRequests) > maxRecords {
		excess := len(acb.recentRequests) - maxRecords
		acb.recentRequests = acb.recentRequests[excess:]
	}
}

// GetState returns the current circuit breaker state
func (acb *AdaptiveCircuitBreaker) GetState() CircuitState {
	acb.mu.RLock()
	defer acb.mu.RUnlock()
	return acb.state
}

// GetMetrics returns current circuit breaker metrics
func (acb *AdaptiveCircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	acb.mu.RLock()
	defer acb.mu.RUnlock()
	
	errorCount := 0
	totalDuration := time.Duration(0)
	
	for _, record := range acb.recentRequests {
		if !record.Success {
			errorCount++
		}
		totalDuration += record.Duration
	}
	
	avgDuration := time.Duration(0)
	if len(acb.recentRequests) > 0 {
		avgDuration = totalDuration / time.Duration(len(acb.recentRequests))
	}
	
	errorRate := float64(0)
	if len(acb.recentRequests) > 0 {
		errorRate = float64(errorCount) / float64(len(acb.recentRequests))
	}
	
	return CircuitBreakerMetrics{
		State:               acb.state,
		Failures:            acb.failures,
		Requests:            acb.requests,
		CurrentThreshold:    acb.currentThreshold,
		ErrorRate:           errorRate,
		AverageResponseTime: avgDuration,
		WindowSize:          acb.windowSize,
		RecentSamples:       len(acb.recentRequests),
	}
}

// SetAdaptiveEnabled enables or disables adaptive behavior
func (acb *AdaptiveCircuitBreaker) SetAdaptiveEnabled(enabled bool) {
	acb.mu.Lock()
	defer acb.mu.Unlock()
	acb.adaptiveEnabled = enabled
	
	if !enabled {
		acb.currentThreshold = acb.baseFailureThreshold
	}
}

// CircuitBreakerMetrics provides detailed metrics about circuit breaker performance
type CircuitBreakerMetrics struct {
	State               CircuitState  `json:"state"`
	Failures            int           `json:"failures"`
	Requests            int           `json:"requests"`
	CurrentThreshold    int           `json:"current_threshold"`
	ErrorRate           float64       `json:"error_rate"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	WindowSize          time.Duration `json:"window_size"`
	RecentSamples       int           `json:"recent_samples"`
}