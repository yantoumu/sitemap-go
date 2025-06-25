package api

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNoAvailableClient = errors.New("no available API client")
	ErrPoolClosed        = errors.New("API pool is closed")
)

type clientWrapper struct {
	client        APIClient
	inUse         atomic.Bool
	lastUsed      time.Time
	healthy       atomic.Bool
	consecutiveFails int
	maxFails      int
}

type apiPool struct {
	clients      []*clientWrapper
	mu           sync.RWMutex
	closed       atomic.Bool
	checkTicker  *time.Ticker
	balanceIndex atomic.Uint32
}

func NewAPIPool(clients []APIClient) APIPool {
	if len(clients) == 0 {
		panic("API pool must have at least one client")
	}
	
	pool := &apiPool{
		clients:     make([]*clientWrapper, 0, len(clients)),
		checkTicker: time.NewTicker(30 * time.Second),
	}
	
	// Wrap clients
	for _, client := range clients {
		wrapper := &clientWrapper{
			client:          client,
			lastUsed:        time.Now(),
			consecutiveFails: 0,
			maxFails:        3, // Allow 3 consecutive failures before marking unhealthy
		}
		wrapper.healthy.Store(true)
		pool.clients = append(pool.clients, wrapper)
	}
	
	// Start health check routine
	go pool.healthCheckRoutine()
	
	return pool
}

func (p *apiPool) GetClient() (APIClient, error) {
	if p.closed.Load() {
		return nil, ErrPoolClosed
	}
	
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	// Try round-robin selection
	startIdx := p.balanceIndex.Add(1) % uint32(len(p.clients))
	
	for i := 0; i < len(p.clients); i++ {
		idx := (startIdx + uint32(i)) % uint32(len(p.clients))
		wrapper := p.clients[idx]
		
		// Check if client is healthy and not in use
		if wrapper.healthy.Load() && wrapper.inUse.CompareAndSwap(false, true) {
			wrapper.lastUsed = time.Now()
			return &pooledClient{
				client:  wrapper.client,
				wrapper: wrapper,
				pool:    p,
			}, nil
		}
	}
	
	return nil, ErrNoAvailableClient
}

func (p *apiPool) ReturnClient(client APIClient) {
	if pooledClient, ok := client.(*pooledClient); ok {
		pooledClient.wrapper.inUse.Store(false)
	}
}

func (p *apiPool) HealthStatus() map[string]HealthStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	status := make(map[string]HealthStatus)
	
	for i, wrapper := range p.clients {
		metrics := wrapper.client.GetMetrics()
		
		status[string(rune('A'+i))] = HealthStatus{
			Healthy:   wrapper.healthy.Load(),
			LastCheck: wrapper.lastUsed.Format("2006-01-02 15:04:05"),
			Message:   fmt.Sprintf("Requests: %d, Errors: %d, Success Rate: %.2f%%", metrics.RequestCount, metrics.ErrorCount, metrics.SuccessRate*100),
		}
	}
	
	return status
}

func (p *apiPool) healthCheckRoutine() {
	for range p.checkTicker.C {
		if p.closed.Load() {
			return
		}
		
		p.mu.RLock()
		clients := make([]*clientWrapper, len(p.clients))
		copy(clients, p.clients)
		p.mu.RUnlock()
		
		for _, wrapper := range clients {
			if !wrapper.inUse.Load() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := wrapper.client.HealthCheck(ctx)
				cancel()
				
				if err == nil {
					wrapper.consecutiveFails = 0
					wrapper.healthy.Store(true)
				} else {
					wrapper.consecutiveFails++
					if wrapper.consecutiveFails >= wrapper.maxFails {
						wrapper.healthy.Store(false)
					}
				}
			}
		}
	}
}

func (p *apiPool) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	
	p.checkTicker.Stop()
	return nil
}

// pooledClient wraps a client from the pool
type pooledClient struct {
	client  APIClient
	wrapper *clientWrapper
	pool    *apiPool
}

func (pc *pooledClient) Query(ctx context.Context, keywords []string) (*APIResponse, error) {
	return pc.client.Query(ctx, keywords)
}

func (pc *pooledClient) HealthCheck(ctx context.Context) error {
	return pc.client.HealthCheck(ctx)
}

func (pc *pooledClient) GetMetrics() *APIMetrics {
	return pc.client.GetMetrics()
}