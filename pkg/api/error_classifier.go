package api

import (
	"strings"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity int

const (
	ErrorSeverityTemporary ErrorSeverity = iota // 临时错误，可重试
	ErrorSeverityRetryable                      // 可重试错误
	ErrorSeverityFatal                          // 致命错误，应停止处理
)

// ErrorClassifier defines interface for error classification
type ErrorClassifier interface {
	ClassifyError(err error) ErrorSeverity
	ShouldStopProcessing(err error) bool
}

// CircuitBreakerErrorClassifier implements error classification for circuit breaker patterns
type CircuitBreakerErrorClassifier struct{}

// NewCircuitBreakerErrorClassifier creates new error classifier
func NewCircuitBreakerErrorClassifier() ErrorClassifier {
	return &CircuitBreakerErrorClassifier{}
}

// ClassifyError classifies error by severity level
func (c *CircuitBreakerErrorClassifier) ClassifyError(err error) ErrorSeverity {
	if err == nil {
		return ErrorSeverityTemporary
	}

	errStr := strings.ToLower(err.Error())

	// Circuit breaker errors are fatal - stop all processing
	if strings.Contains(errStr, "circuit breaker is open") ||
		strings.Contains(errStr, "circuit breaker") {
		return ErrorSeverityFatal
	}

	// Rate limiting errors should stop processing temporarily  
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") {
		return ErrorSeverityFatal
	}

	// Auth errors are fatal
	if strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "forbidden") {
		return ErrorSeverityFatal
	}

	// Temporary network issues
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "dns") {
		return ErrorSeverityRetryable
	}

	// Default to retryable for unknown errors
	return ErrorSeverityRetryable
}

// ShouldStopProcessing determines if processing should be halted
func (c *CircuitBreakerErrorClassifier) ShouldStopProcessing(err error) bool {
	return c.ClassifyError(err) == ErrorSeverityFatal
}