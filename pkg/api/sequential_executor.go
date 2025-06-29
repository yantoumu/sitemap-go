package api

import (
	"context"
	"time"
)

// SequentialExecutor implements concurrent API execution with minimal rate limiting
// Optimized for SEOKey API which tolerates higher request rates than Google Trends
type SequentialExecutor struct {
	// Removed mutex to allow true concurrent execution
}

// NewSequentialExecutor creates a new sequential executor
func NewSequentialExecutor() *SequentialExecutor {
	return &SequentialExecutor{}
}

// Execute runs function with minimal delay for true concurrent execution
// Reduced delay for SEOKey API which has better rate limiting tolerance
func (se *SequentialExecutor) Execute(ctx context.Context, fn func() error) error {
	// Check context before execution
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue with execution
	}
	
	// Execute function
	err := fn()

	// Reduced delay for SEOKey API - 50ms for better throughput
	// SEOKey API has better rate limiting tolerance than Google Trends
	select {
	case <-time.After(50 * time.Millisecond):
		// Minimal delay completed
	case <-ctx.Done():
		// Context cancelled during delay
		return ctx.Err()
	}

	return err
}