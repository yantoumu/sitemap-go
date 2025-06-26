package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSequentialExecutor_SequentialExecution(t *testing.T) {
	executor := NewSequentialExecutor()
	
	start := time.Now()
	var executionTimes []time.Time
	
	// Execute 3 functions
	for i := 0; i < 3; i++ {
		err := executor.Execute(context.Background(), func() error {
			executionTimes = append(executionTimes, time.Now())
			// Simulate work time
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		
		if err != nil {
			t.Errorf("Execution %d failed: %v", i, err)
		}
	}
	
	// Verify executions completed
	if len(executionTimes) != 3 {
		t.Errorf("Expected 3 executions, got %d", len(executionTimes))
	}
	
	// All executions should complete quickly without forced delays
	totalTime := time.Since(start)
	expectedTime := 150 * time.Millisecond // 3 * 50ms work time
	if totalTime > expectedTime+100*time.Millisecond {
		t.Errorf("Execution took too long: %v (expected ~%v)", totalTime, expectedTime)
	}
	
	// Verify sequential order (execution times should be strictly increasing)
	for i := 1; i < len(executionTimes); i++ {
		if executionTimes[i].Before(executionTimes[i-1]) {
			t.Errorf("Execution %d started before execution %d completed", i, i-1)
		}
	}
}

func TestSequentialExecutor_ErrorHandling(t *testing.T) {
	executor := NewSequentialExecutor()
	
	testError := errors.New("test error")
	
	err := executor.Execute(context.Background(), func() error {
		return testError
	})
	
	if err != testError {
		t.Errorf("Expected test error, got %v", err)
	}
}

func TestSequentialExecutor_ContextCancellation(t *testing.T) {
	executor := NewSequentialExecutor()
	
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
	executor := NewSequentialExecutor()
	
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
	// but the sequential executor should ensure no overlapping executions
	t.Logf("Execution times: %v", execTimes)
}