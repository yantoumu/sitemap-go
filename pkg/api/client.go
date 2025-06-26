package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"sitemap-go/pkg/logger"
)

type httpAPIClient struct {
	urlPool         *URLPool  // Replaced baseURL with URL pool for load balancing
	apiKey          string
	connManager     *ConnectionManager
	breaker         *CircuitBreaker
	adaptiveBreaker *AdaptiveCircuitBreaker
	log             *logger.Logger
	useAdaptive     bool
	
	// Metrics
	totalRequests uint64
	failedRequests uint64
	totalLatency  uint64
	lastError     atomic.Value
}

func NewHTTPAPIClient(baseURL, apiKey string) APIClient {
	return NewHTTPAPIClientWithConfig(baseURL, apiKey, HighThroughputConnectionConfig())
}

// NewHTTPAPIClientWithConfig creates a new HTTP API client with custom connection config
// Supports single URL or comma-separated multiple URLs for load balancing
func NewHTTPAPIClientWithConfig(baseURL, apiKey string, connConfig ConnectionConfig) APIClient {
	urlPool := NewURLPool(baseURL) // Create URL pool from single or multiple URLs
	
	return &httpAPIClient{
		urlPool:         urlPool,
		apiKey:          apiKey,
		connManager:     NewConnectionManager(connConfig),
		breaker:         NewCircuitBreaker(5, 30*time.Second),
		adaptiveBreaker: NewAdaptiveCircuitBreaker(3, 10, 30*time.Second),
		log:             logger.GetLogger().WithField("component", "api_client"),
		useAdaptive:     true, // Use adaptive breaker by default
	}
}

// NewHTTPAPIClientWithAdaptiveBreaker creates client with adaptive circuit breaker
func NewHTTPAPIClientWithAdaptiveBreaker(baseURL, apiKey string, connConfig ConnectionConfig, useAdaptive bool) APIClient {
	urlPool := NewURLPool(baseURL) // Create URL pool from single or multiple URLs
	
	client := &httpAPIClient{
		urlPool:         urlPool,
		apiKey:          apiKey,
		connManager:     NewConnectionManager(connConfig),
		breaker:         NewCircuitBreaker(5, 30*time.Second),
		adaptiveBreaker: NewAdaptiveCircuitBreaker(3, 10, 30*time.Second),
		log:             logger.GetLogger().WithField("component", "api_client"),
		useAdaptive:     useAdaptive,
	}
	return client
}

func (c *httpAPIClient) Query(ctx context.Context, keywords []string) (*APIResponse, error) {
	atomic.AddUint64(&c.totalRequests, 1)
	start := time.Now()
	defer func() {
		atomic.AddUint64(&c.totalLatency, uint64(time.Since(start).Milliseconds()))
	}()
	
	c.log.WithField("keywords_count", len(keywords)).Debug("Starting API query")
	
	var result *APIResponse
	var err error
	
	// Use adaptive breaker if enabled, otherwise use standard breaker
	if c.useAdaptive {
		err = c.adaptiveBreaker.Execute(ctx, func() error {
			return c.doQuery(ctx, keywords, &result)
		})
	} else {
		err = c.breaker.Execute(ctx, func() error {
			return c.doQuery(ctx, keywords, &result)
		})
	}
	
	if err != nil {
		atomic.AddUint64(&c.failedRequests, 1)
		c.lastError.Store(err.Error())
		c.log.WithError(err).WithField("keywords_count", len(keywords)).Error("API query failed")
		return nil, err
	}
	
	c.log.WithField("duration_ms", time.Since(start).Milliseconds()).Debug("API query completed successfully")
	return result, nil
}


func (c *httpAPIClient) doQuery(ctx context.Context, keywords []string, result **APIResponse) error {
	// Create fasthttp request
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	
	// Set request properties - use next URL from pool for load balancing
	baseURL := c.urlPool.Next()
	if baseURL == "" {
		return fmt.Errorf("no URLs available in URL pool")
	}
	
	// Build URL with keywords as query parameters: keyword=key1,key2,key3
	keywordParam := strings.Join(keywords, ",")
	fullURL := baseURL + keywordParam
	req.SetRequestURI(fullURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	
	// Set authorization header only if API key is provided
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	
	// Execute request using connection manager
	err := c.connManager.GetFastHTTPClient().DoTimeout(req, resp, 30*time.Second)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	
	// Check status code
	if resp.StatusCode() != fasthttp.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}
	
	// Parse response
	var apiResp APIResponse
	if err := json.Unmarshal(resp.Body(), &apiResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	*result = &apiResp
	return nil
}


func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}