package api

import (
	"errors"
	"testing"
)

func TestCircuitBreakerErrorClassifier_ClassifyError(t *testing.T) {
	classifier := NewCircuitBreakerErrorClassifier()

	tests := []struct {
		name     string
		err      error
		expected ErrorSeverity
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: ErrorSeverityTemporary,
		},
		{
			name:     "circuit breaker is open",
			err:      errors.New("circuit breaker is open"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "circuit breaker error",
			err:      errors.New("circuit breaker failed"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "Chinese circuit breaker open",
			err:      errors.New("熔断器已打开"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "Chinese circuit breaker",
			err:      errors.New("熔断器打开"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "Breaker with open",
			err:      errors.New("breaker is open"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "Mixed language error",
			err:      errors.New("some breaker 打开 error"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "rate limit error",
			err:      errors.New("rate limit exceeded"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "HTTP 429 error",
			err:      errors.New("HTTP 429 Too Many Requests"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "HTTP 401 error",
			err:      errors.New("HTTP 401 Unauthorized"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "HTTP 403 error",
			err:      errors.New("HTTP 403 Forbidden"),
			expected: ErrorSeverityFatal,
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: ErrorSeverityRetryable,
		},
		{
			name:     "connection error",
			err:      errors.New("connection refused"),
			expected: ErrorSeverityRetryable,
		},
		{
			name:     "DNS error",
			err:      errors.New("DNS resolution failed"),
			expected: ErrorSeverityRetryable,
		},
		{
			name:     "unknown error",
			err:      errors.New("some unknown error"),
			expected: ErrorSeverityRetryable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyError(tt.err)
			if result != tt.expected {
				t.Errorf("ClassifyError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCircuitBreakerErrorClassifier_ShouldStopProcessing(t *testing.T) {
	classifier := NewCircuitBreakerErrorClassifier()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "circuit breaker is open - should stop",
			err:      errors.New("circuit breaker is open"),
			expected: true,
		},
		{
			name:     "Chinese circuit breaker - should stop",
			err:      errors.New("熔断器已打开"),
			expected: true,
		},
		{
			name:     "Chinese circuit breaker open - should stop",
			err:      errors.New("熔断器打开"),
			expected: true,
		},
		{
			name:     "rate limit error - should stop",
			err:      errors.New("rate limit exceeded"),
			expected: true,
		},
		{
			name:     "auth error - should stop",
			err:      errors.New("unauthorized access"),
			expected: true,
		},
		{
			name:     "timeout error - should not stop",
			err:      errors.New("request timeout"),
			expected: false,
		},
		{
			name:     "connection error - should not stop",
			err:      errors.New("connection failed"),
			expected: false,
		},
		{
			name:     "unknown error - should not stop",
			err:      errors.New("some unknown error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ShouldStopProcessing(tt.err)
			if result != tt.expected {
				t.Errorf("ShouldStopProcessing() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Benchmark the error classification performance
func BenchmarkCircuitBreakerErrorClassifier_ClassifyError(b *testing.B) {
	classifier := NewCircuitBreakerErrorClassifier()
	testError := errors.New("circuit breaker is open")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifier.ClassifyError(testError)
	}
}

func BenchmarkCircuitBreakerErrorClassifier_ShouldStopProcessing(b *testing.B) {
	classifier := NewCircuitBreakerErrorClassifier()
	testError := errors.New("circuit breaker is open")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifier.ShouldStopProcessing(testError)
	}
}