package api

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// DualAPIClient supports load balancing between two API endpoints
type DualAPIClient struct {
	primaryClient   APIClient
	secondaryClient APIClient
	primaryHealthy  bool
	secondaryHealthy bool
	mu              sync.RWMutex
	lastCheck       time.Time
}

// NewDualAPIClient creates a client that can use two API endpoints
func NewDualAPIClient(primary, secondary string) APIClient {
	return &DualAPIClient{
		primaryClient:    NewHTTPAPIClient(primary, ""),
		secondaryClient:  NewHTTPAPIClient(secondary, ""),
		primaryHealthy:   true,
		secondaryHealthy: true,
		lastCheck:        time.Now(),
	}
}

// Query distributes requests between two APIs with health checking
func (d *DualAPIClient) Query(ctx context.Context, keywords []string) (*APIResponse, error) {
	// Check API health periodically (every 30 seconds)
	d.checkHealth()
	
	// Select API based on health status
	client := d.selectClient()
	if client == nil {
		return nil, fmt.Errorf("no healthy API endpoints available")
	}
	
	// Make the actual query
	resp, err := client.Query(ctx, keywords)
	
	// Update health status based on response
	if err != nil {
		d.updateHealth(client, false)
		// Try the other API if available
		if otherClient := d.getOtherClient(client); otherClient != nil {
			resp, err = otherClient.Query(ctx, keywords)
			if err == nil {
				d.updateHealth(otherClient, true)
			}
		}
	} else {
		d.updateHealth(client, true)
	}
	
	return resp, err
}

// selectClient chooses a healthy client with load balancing
func (d *DualAPIClient) selectClient() APIClient {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	// If only one is healthy, use it
	if d.primaryHealthy && !d.secondaryHealthy {
		return d.primaryClient
	}
	if !d.primaryHealthy && d.secondaryHealthy {
		return d.secondaryClient
	}
	if !d.primaryHealthy && !d.secondaryHealthy {
		return nil
	}
	
	// Both healthy - random load balancing
	if rand.Intn(2) == 0 {
		return d.primaryClient
	}
	return d.secondaryClient
}

// getOtherClient returns the other client for failover
func (d *DualAPIClient) getOtherClient(current APIClient) APIClient {
	if current == d.primaryClient {
		return d.secondaryClient
	}
	return d.primaryClient
}

// updateHealth updates the health status of a client
func (d *DualAPIClient) updateHealth(client APIClient, healthy bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if client == d.primaryClient {
		d.primaryHealthy = healthy
	} else {
		d.secondaryHealthy = healthy
	}
}

// checkHealth periodically resets health status
func (d *DualAPIClient) checkHealth() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	// Reset health status every 30 seconds to retry failed endpoints
	if time.Since(d.lastCheck) > 30*time.Second {
		d.primaryHealthy = true
		d.secondaryHealthy = true
		d.lastCheck = time.Now()
	}
}

// Close closes both clients if they support closing
func (d *DualAPIClient) Close() error {
	// APIClient interface doesn't have Close method
	// This is for compatibility with the monitor's Close method
	return nil
}