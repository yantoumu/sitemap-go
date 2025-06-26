package api

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"sitemap-go/pkg/logger"
)

// ErrorType categorizes different types of errors
type ErrorType int

const (
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeHTTP
	ErrorTypeData
	ErrorTypeTimeout
	ErrorTypeAuth
	ErrorTypeRateLimit
)

// SmartCircuitBreaker implements intelligent circuit breaking with error categorization
type SmartCircuitBreaker struct {
	mu sync.RWMutex

	// Error tracking by type
	errorCounts    map[ErrorType]int
	errorWindows   map[ErrorType][]time.Time
	windowDuration time.Duration

	// Thresholds for different error types
	thresholds map[ErrorType]int

	// Circuit state
	state       CircuitState
	lastFailure time.Time
	nextRetry   time.Time

	// Configuration
	recoveryTimeout time.Duration
	halfOpenLimit   int
	halfOpenCount   int

	log *logger.Logger
}

// SmartBreakerConfig holds configuration for smart circuit breaker
type SmartBreakerConfig struct {
	NetworkErrorThreshold   int           `json:"network_error_threshold"`
	HTTPErrorThreshold      int           `json:"http_error_threshold"`
	DataErrorThreshold      int           `json:"data_error_threshold"`
	TimeoutErrorThreshold   int           `json:"timeout_error_threshold"`
	WindowDuration          time.Duration `json:"window_duration"`
	RecoveryTimeout         time.Duration `json:"recovery_timeout"`
	HalfOpenLimit          int           `json:"half_open_limit"`
}

// DefaultSmartBreakerConfig returns default configuration
func DefaultSmartBreakerConfig() SmartBreakerConfig {
	return SmartBreakerConfig{
		NetworkErrorThreshold: 3,  // Network issues should trigger quickly
		HTTPErrorThreshold:    5,  // HTTP errors are more tolerant
		DataErrorThreshold:    8,  // Data parsing errors are most tolerant
		TimeoutErrorThreshold: 2,  // Timeouts should trigger quickly
		WindowDuration:        5 * time.Minute,
		RecoveryTimeout:       30 * time.Second,
		HalfOpenLimit:        3,
	}
}

// NewSmartCircuitBreaker creates a new smart circuit breaker
func NewSmartCircuitBreaker(config SmartBreakerConfig) *SmartCircuitBreaker {
	return &SmartCircuitBreaker{
		errorCounts:  make(map[ErrorType]int),
		errorWindows: make(map[ErrorType][]time.Time),
		thresholds: map[ErrorType]int{
			ErrorTypeNetwork:   config.NetworkErrorThreshold,
			ErrorTypeHTTP:      config.HTTPErrorThreshold,
			ErrorTypeData:      config.DataErrorThreshold,
			ErrorTypeTimeout:   config.TimeoutErrorThreshold,
			ErrorTypeAuth:      2, // Auth errors should trigger quickly
			ErrorTypeRateLimit: 1, // Rate limit should trigger immediately
		},
		windowDuration:  config.WindowDuration,
		recoveryTimeout: config.RecoveryTimeout,
		halfOpenLimit:   config.HalfOpenLimit,
		state:           StateClosed,
		log:             logger.GetLogger().WithField("component", "smart_breaker"),
	}
}

// Execute executes a function with smart circuit breaking
func (scb *SmartCircuitBreaker) Execute(fn func() error) error {
	// Check if circuit is open
	if scb.shouldReject() {
		return fmt.Errorf("circuit breaker is open")
	}

	// Execute function
	err := fn()

	// Record result
	if err != nil {
		scb.recordFailure(err)
		return err
	}

	scb.recordSuccess()
	return nil
}

// shouldReject determines if the request should be rejected
func (scb *SmartCircuitBreaker) shouldReject() bool {
	scb.mu.RLock()
	defer scb.mu.RUnlock()

	switch scb.state {
	case StateClosed:
		return false
	case StateOpen:
		// Check if recovery timeout has passed
		if time.Now().After(scb.nextRetry) {
			scb.mu.RUnlock()
			scb.mu.Lock()
			// Double-check after acquiring write lock
			if scb.state == StateOpen && time.Now().After(scb.nextRetry) {
				scb.state = StateHalfOpen
				scb.halfOpenCount = 0
				scb.log.Info("Circuit breaker transitioning to half-open state")
			}
			scb.mu.Unlock()
			scb.mu.RLock()
			return scb.state == StateOpen
		}
		return true
	case StateHalfOpen:
		return scb.halfOpenCount >= scb.halfOpenLimit
	default:
		return false
	}
}

// recordFailure records a failure and categorizes the error
func (scb *SmartCircuitBreaker) recordFailure(err error) {
	scb.mu.Lock()
	defer scb.mu.Unlock()

	// Categorize error
	errorType := scb.categorizeError(err)
	
	scb.log.WithFields(map[string]interface{}{
		"error_type": errorType,
		"error":      err.Error(),
	}).Debug("Recording error in circuit breaker")

	// Clean old errors from window
	scb.cleanErrorWindow(errorType)

	// Record new error
	now := time.Now()
	scb.errorWindows[errorType] = append(scb.errorWindows[errorType], now)
	scb.errorCounts[errorType]++
	scb.lastFailure = now

	// Check if threshold is exceeded for this error type
	if scb.errorCounts[errorType] >= scb.thresholds[errorType] {
		scb.openCircuit(errorType)
	}

	// Handle half-open state
	if scb.state == StateHalfOpen {
		scb.openCircuit(errorType)
	}
}

// recordSuccess records a successful operation
func (scb *SmartCircuitBreaker) recordSuccess() {
	scb.mu.Lock()
	defer scb.mu.Unlock()

	if scb.state == StateHalfOpen {
		scb.halfOpenCount++
		if scb.halfOpenCount >= scb.halfOpenLimit {
			scb.state = StateClosed
			// Reset all error counts
			scb.errorCounts = make(map[ErrorType]int)
			scb.errorWindows = make(map[ErrorType][]time.Time)
			scb.log.Info("Circuit breaker closed after successful recovery")
		}
	}
}

// categorizeError categorizes an error by type
func (scb *SmartCircuitBreaker) categorizeError(err error) ErrorType {
	errStr := strings.ToLower(err.Error())

	// Network errors
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return ErrorTypeTimeout
		}
		return ErrorTypeNetwork
	}

	// Check error message for patterns
	switch {
	case strings.Contains(errStr, "timeout"):
		return ErrorTypeTimeout
	case strings.Contains(errStr, "connection"):
		return ErrorTypeNetwork
	case strings.Contains(errStr, "dns"):
		return ErrorTypeNetwork
	case strings.Contains(errStr, "network"):
		return ErrorTypeNetwork
	case strings.Contains(errStr, "401") || strings.Contains(errStr, "403"):
		return ErrorTypeAuth
	case strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit"):
		return ErrorTypeRateLimit
	case strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || 
		 strings.Contains(errStr, "503") || strings.Contains(errStr, "504"):
		return ErrorTypeHTTP
	case strings.Contains(errStr, "parse") || strings.Contains(errStr, "decode") ||
		 strings.Contains(errStr, "unmarshal") || strings.Contains(errStr, "syntax"):
		return ErrorTypeData
	default:
		return ErrorTypeHTTP
	}
}

// openCircuit opens the circuit breaker
func (scb *SmartCircuitBreaker) openCircuit(errorType ErrorType) {
	if scb.state != StateOpen {
		scb.state = StateOpen
		scb.nextRetry = time.Now().Add(scb.getRecoveryTimeout(errorType))
		
		scb.log.WithFields(map[string]interface{}{
			"error_type":   errorType,
			"error_count":  scb.errorCounts[errorType],
			"threshold":    scb.thresholds[errorType],
			"next_retry":   scb.nextRetry,
		}).Warn("Circuit breaker opened due to error threshold exceeded")
	}
}

// getRecoveryTimeout returns recovery timeout based on error type
func (scb *SmartCircuitBreaker) getRecoveryTimeout(errorType ErrorType) time.Duration {
	switch errorType {
	case ErrorTypeRateLimit:
		return 2 * time.Minute // Wait longer for rate limits
	case ErrorTypeAuth:
		return 5 * time.Minute // Wait longer for auth issues
	case ErrorTypeNetwork:
		return 1 * time.Minute // Network issues may resolve quickly
	case ErrorTypeTimeout:
		return 30 * time.Second // Timeouts may be temporary
	default:
		return scb.recoveryTimeout
	}
}

// cleanErrorWindow removes old errors outside the time window
func (scb *SmartCircuitBreaker) cleanErrorWindow(errorType ErrorType) {
	cutoff := time.Now().Add(-scb.windowDuration)
	
	// Filter out old errors
	var newWindow []time.Time
	for _, timestamp := range scb.errorWindows[errorType] {
		if timestamp.After(cutoff) {
			newWindow = append(newWindow, timestamp)
		}
	}
	
	scb.errorWindows[errorType] = newWindow
	scb.errorCounts[errorType] = len(newWindow)
}

// GetState returns the current circuit breaker state
func (scb *SmartCircuitBreaker) GetState() CircuitState {
	scb.mu.RLock()
	defer scb.mu.RUnlock()
	return scb.state
}

// GetMetrics returns detailed metrics about error patterns
func (scb *SmartCircuitBreaker) GetMetrics() SmartBreakerMetrics {
	scb.mu.RLock()
	defer scb.mu.RUnlock()

	// Clean all windows first
	for errorType := range scb.errorCounts {
		scb.cleanErrorWindow(errorType)
	}

	return SmartBreakerMetrics{
		State:        scb.state,
		ErrorCounts:  copyErrorCounts(scb.errorCounts),
		Thresholds:   copyErrorCounts(scb.thresholds),
		LastFailure:  scb.lastFailure,
		NextRetry:    scb.nextRetry,
		WindowDuration: scb.windowDuration,
	}
}

// SmartBreakerMetrics represents detailed circuit breaker metrics
type SmartBreakerMetrics struct {
	State          CircuitState         `json:"state"`
	ErrorCounts    map[ErrorType]int    `json:"error_counts"`
	Thresholds     map[ErrorType]int    `json:"thresholds"`
	LastFailure    time.Time            `json:"last_failure"`
	NextRetry      time.Time            `json:"next_retry"`
	WindowDuration time.Duration        `json:"window_duration"`
}

// helper function to copy error counts map
func copyErrorCounts(source map[ErrorType]int) map[ErrorType]int {
	copy := make(map[ErrorType]int)
	for k, v := range source {
		copy[k] = v
	}
	return copy
}