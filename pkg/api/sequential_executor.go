package api

import (
	"context"
	"sync"
	"time"
)

// SequentialExecutor ensures API requests are executed sequentially with delays
// Follows KISS principle - simple, single responsibility
type SequentialExecutor struct {
	mu           sync.Mutex
	lastRequest  time.Time
	minInterval  time.Duration
}

// NewSequentialExecutor creates a new sequential executor
func NewSequentialExecutor(minInterval time.Duration) *SequentialExecutor {
	return &SequentialExecutor{
		minInterval: minInterval,
	}
}

// Execute runs function with sequential execution and minimum interval
func (se *SequentialExecutor) Execute(ctx context.Context, fn func() error) error {
	se.mu.Lock()
	defer se.mu.Unlock()
	
	// Calculate required wait time
	now := time.Now()
	elapsed := now.Sub(se.lastRequest)
	
	if elapsed < se.minInterval {
		waitTime := se.minInterval - elapsed
		
		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue after wait
		}
	}
	
	// Execute function
	err := fn()
	se.lastRequest = time.Now()
	
	return err
}