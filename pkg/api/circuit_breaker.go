package api

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	ErrMaxRetries  = errors.New("max retries exceeded")
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	
	mu           sync.RWMutex
	state        CircuitState
	failures     int
	lastFailTime time.Time
	successCount int
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := cb.canExecute(); err != nil {
		return err
	}
	
	err := fn()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) canExecute() error {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.successCount = 0
			cb.mu.Unlock()
			cb.mu.RLock()
			return nil
		}
		return ErrCircuitOpen
	case StateHalfOpen:
		return nil
	}
	return nil
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()
		
		if cb.state == StateHalfOpen {
			cb.state = StateOpen
		} else if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
	} else {
		if cb.state == StateHalfOpen {
			cb.successCount++
			if cb.successCount >= 3 { // Need 3 successes to close circuit
				cb.state = StateClosed
				cb.failures = 0
			}
		} else {
			cb.failures = 0
		}
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}