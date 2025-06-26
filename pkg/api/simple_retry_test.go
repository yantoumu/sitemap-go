package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSimpleRetry_Success(t *testing.T) {
	retry := NewSimpleRetry(3, 10*time.Millisecond)
	
	attempts := 0
	err := retry.Execute(context.Background(), func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil // Success on second attempt
	})
	
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestSimpleRetry_MaxRetriesExceeded(t *testing.T) {
	retry := NewSimpleRetry(2, 10*time.Millisecond)
	
	attempts := 0
	err := retry.Execute(context.Background(), func() error {
		attempts++
		return errors.New("persistent error")
	})
	
	if err == nil {
		t.Error("Expected error, got nil")
	}
	
	if attempts != 3 { // 1 initial + 2 retries
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestSimpleRetry_NonRetryableError(t *testing.T) {
	retry := NewSimpleRetry(3, 10*time.Millisecond)
	
	attempts := 0
	err := retry.Execute(context.Background(), func() error {
		attempts++
		return errors.New("401 unauthorized") // Non-retryable
	})
	
	if err == nil {
		t.Error("Expected error, got nil")
	}
	
	if attempts != 1 { // Should not retry
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestSimpleRetry_ContextCancellation(t *testing.T) {
	retry := NewSimpleRetry(3, 100*time.Millisecond)
	
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	
	err := retry.Execute(ctx, func() error {
		return errors.New("some error")
	})
	
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}