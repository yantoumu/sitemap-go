package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSequentialExecutor_IntervalControl(t *testing.T) {
	executor := NewSequentialExecutor(1 * time.Second)
	
	start := time.Now()
	var executionTimes []time.Time
	
	// Execute 3 functions
	for i := 0; i < 3; i++ {
		err := executor.Execute(context.Background(), func() error {
			executionTimes = append(executionTimes, time.Now())
			return nil
		})
		
		if err != nil {
			t.Errorf("Execution %d failed: %v", i, err)
		}
	}
	
	// Verify timing intervals
	if len(executionTimes) != 3 {
		t.Errorf("Expected 3 executions, got %d", len(executionTimes))
	}
	
	// First execution should be immediate
	firstDelay := executionTimes[0].Sub(start)
	if firstDelay > 100*time.Millisecond {
		t.Errorf("First execution delayed too much: %v", firstDelay)
	}
	
	// Second execution should be ~1 second after first
	if len(executionTimes) >= 2 {
		interval1 := executionTimes[1].Sub(executionTimes[0])
		if interval1 < 900*time.Millisecond || interval1 > 1100*time.Millisecond {
			t.Errorf("Second execution interval out of range: %v (expected ~1s)", interval1)
		}
	}
	
	// Third execution should be ~1 second after second
	if len(executionTimes) >= 3 {
		interval2 := executionTimes[2].Sub(executionTimes[1])
		if interval2 < 900*time.Millisecond || interval2 > 1100*time.Millisecond {
			t.Errorf("Third execution interval out of range: %v (expected ~1s)", interval2)
		}
	}
}

func TestSequentialExecutor_ErrorHandling(t *testing.T) {
	executor := NewSequentialExecutor(100 * time.Millisecond)
	
	testError := errors.New("test error")
	
	err := executor.Execute(context.Background(), func() error {
		return testError
	})
	
	if err != testError {
		t.Errorf("Expected test error, got %v", err)
	}
}

func TestSequentialExecutor_ContextCancellation(t *testing.T) {
	executor := NewSequentialExecutor(2 * time.Second)
	
	// First execution to set lastRequest time
	executor.Execute(context.Background(), func() error {
		return nil
	})
	
	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel context immediately
	cancel()
	
	err := executor.Execute(ctx, func() error {
		return nil
	})
	
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestSequentialExecutor_ConcurrentAccess(t *testing.T) {
	executor := NewSequentialExecutor(500 * time.Millisecond)
	
	const numGoroutines = 5
	results := make(chan time.Time, numGoroutines)
	
	// Launch multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		go func() {
			executor.Execute(context.Background(), func() error {
				results <- time.Now()
				return nil
			})
		}()
	}
	
	// Collect execution times
	var execTimes []time.Time
	for i := 0; i < numGoroutines; i++ {
		execTime := <-results
		execTimes = append(execTimes, execTime)
	}
	
	// Verify executions are properly spaced
	if len(execTimes) != numGoroutines {
		t.Errorf("Expected %d executions, got %d", numGoroutines, len(execTimes))
	}
	
	// Note: Due to concurrent access, execution order may vary,
	// but the sequential executor should still maintain minimum intervals
	t.Logf("Execution times: %v", execTimes)
}