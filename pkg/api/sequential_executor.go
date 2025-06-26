package api

import (
	"context"
	"sync"
	"time"
)

// SequentialExecutor ensures API requests are executed sequentially with 5-second delays
// Implements rate limiting to avoid 429 "Too Many Requests" errors
// 5-second delay based on Google Trends API rate limiting analysis
type SequentialExecutor struct {
	mu sync.Mutex
}

// NewSequentialExecutor creates a new sequential executor
func NewSequentialExecutor() *SequentialExecutor {
	return &SequentialExecutor{}
}

// Execute runs function with sequential execution and 5-second delay
// Each request waits for previous to complete + 5 second delay to avoid rate limiting
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
	
	// Execute function
	err := fn()
	
	// Add 5-second delay after execution to avoid 429 rate limit errors
	// Based on Google Trends API testing showing 429 errors with shorter delays
	select {
	case <-time.After(5 * time.Second):
		// Delay completed
	case <-ctx.Done():
		// Context cancelled during delay
		return ctx.Err()
	}
	
	return err
}