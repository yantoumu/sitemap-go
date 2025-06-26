package api

import (
	"context"
	"sync"
)

// SequentialExecutor ensures API requests are executed sequentially
// No forced delays - requests execute immediately after previous completes
// Follows KISS principle - simple, single responsibility
type SequentialExecutor struct {
	mu sync.Mutex
}

// NewSequentialExecutor creates a new sequential executor
func NewSequentialExecutor() *SequentialExecutor {
	return &SequentialExecutor{}
}

// Execute runs function with sequential execution 
// Each request waits for previous to complete, no forced delays
func (se *SequentialExecutor) Execute(ctx context.Context, fn func() error) error {
	se.mu.Lock()
	defer se.mu.Unlock()
	
	// Check context before execution
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue with execution
	}
	
	// Execute function immediately - no delays
	return fn()
}