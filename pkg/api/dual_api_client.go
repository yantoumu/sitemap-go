package api

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// DualAPIClient supports load balancing between two API endpoints
// Enhanced with API-aware rate limiting for optimal dual API utilization
type DualAPIClient struct {
	primaryClient   APIClient
	secondaryClient APIClient
	primaryURL      string // Store URLs for rate limiter identification
	secondaryURL    string
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
		primaryURL:       primary,   // Store for rate limiter identification
		secondaryURL:     secondary, // Store for rate limiter identification
		primaryHealthy:   true,
		secondaryHealthy: true,
		lastCheck:        time.Now(),
	}
}

// Query distributes requests between two APIs with health checking and smart failover
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

	// Enhanced error handling for 429/500 errors
	if err != nil {
		d.updateHealth(client, false)

		// For rate limit or server errors, try the other API immediately
		if d.isRateLimitOrServerError(err) {
			if otherClient := d.getOtherClient(client); otherClient != nil {
				resp, err = otherClient.Query(ctx, keywords)
				if err == nil {
					d.updateHealth(otherClient, true)
				} else {
					d.updateHealth(otherClient, false)
				}
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

// GetCurrentAPIEndpoint returns the URL of the currently selected API endpoint
// This enables API-aware rate limiting in the monitor
func (d *DualAPIClient) GetCurrentAPIEndpoint() string {
	client := d.selectClient()
	if client == d.primaryClient {
		return d.primaryURL
	} else if client == d.secondaryClient {
		return d.secondaryURL
	}
	return "" // No healthy endpoint
}

// GetAPIEndpointForClient returns the URL for a specific client
func (d *DualAPIClient) GetAPIEndpointForClient(client APIClient) string {
	if client == d.primaryClient {
		return d.primaryURL
	} else if client == d.secondaryClient {
		return d.secondaryURL
	}
	return ""
}

// isRateLimitOrServerError checks if error warrants immediate failover
func (d *DualAPIClient) isRateLimitOrServerError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for rate limit (429) or server errors (5xx)
	return containsAny(errStr, []string{"429", "500", "502", "503", "504", "rate limit", "too many requests"})
}

// containsAny checks if string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) && dualAPIContainsIgnoreCase(s, substr) {
			return true
		}
	}
	return false
}

// dualAPIContainsIgnoreCase performs case-insensitive substring search
func dualAPIContainsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]

			// Convert to lowercase
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}

			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Close closes both clients if they support closing
func (d *DualAPIClient) Close() error {
	// APIClient interface doesn't have Close method
	// This is for compatibility with the monitor's Close method
	return nil
}