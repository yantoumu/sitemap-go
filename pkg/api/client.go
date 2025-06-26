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
	retry           *SimpleRetry
	log             *logger.Logger
	
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
		retry:           NewSimpleRetry(3, 1*time.Second), // 3 retries with 1s initial delay
		log:             logger.GetLogger().WithField("component", "api_client"),
	}
}

// NewHTTPAPIClientWithRetry creates client with configurable retry mechanism
func NewHTTPAPIClientWithRetry(baseURL, apiKey string, connConfig ConnectionConfig, maxRetries int, retryDelay time.Duration) APIClient {
	urlPool := NewURLPool(baseURL) // Create URL pool from single or multiple URLs
	
	client := &httpAPIClient{
		urlPool:         urlPool,
		apiKey:          apiKey,
		connManager:     NewConnectionManager(connConfig),
		retry:           NewSimpleRetry(maxRetries, retryDelay),
		log:             logger.GetLogger().WithField("component", "api_client"),
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
	
	// Use simple retry mechanism
	err := c.retry.Execute(ctx, func() error {
		return c.doQuery(ctx, keywords, &result)
	})
	
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


