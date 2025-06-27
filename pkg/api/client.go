package api

import (
	"context"
	"fmt"
	"net/url"
	"os"
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

	// Atomic concurrency control (inspired by 1.js)
	concurrencyLimiter ConcurrencyLimiter

	// Metrics
	totalRequests uint64
	failedRequests uint64
	totalLatency  uint64
	lastError     atomic.Value
}

// ConcurrencyLimiter interface for atomic concurrency control
// Allows different implementations (atomic, distributed, etc.)
type ConcurrencyLimiter interface {
	Acquire(ctx context.Context) error
	Release()
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
		concurrencyLimiter: nil, // Will be set by monitor when needed
	}
}

// NewHTTPAPIClientWithConcurrency creates client with atomic concurrency control
// This enables precise concurrent request management similar to 1.js implementation
func NewHTTPAPIClientWithConcurrency(baseURL, apiKey string, connConfig ConnectionConfig, limiter ConcurrencyLimiter) APIClient {
	urlPool := NewURLPool(baseURL)

	return &httpAPIClient{
		urlPool:            urlPool,
		apiKey:             apiKey,
		connManager:        NewConnectionManager(connConfig),
		retry:              NewSimpleRetry(3, 1*time.Second),
		log:                logger.GetLogger().WithField("component", "api_client_concurrent"),
		concurrencyLimiter: limiter,
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
	
	// Removed detailed debug logging for cleaner output

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
	
	// Removed success logging for cleaner output
	return result, nil
}


func (c *httpAPIClient) doQuery(ctx context.Context, keywords []string, result **APIResponse) error {
	// Acquire concurrency permit if limiter is configured (inspired by 1.js)
	if c.concurrencyLimiter != nil {
		if err := c.concurrencyLimiter.Acquire(ctx); err != nil {
			return fmt.Errorf("failed to acquire concurrency permit: %w", err)
		}
		defer c.concurrencyLimiter.Release() // Ensure permit is always released
	}

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
	
	// For seokey API, support batch queries with comma-separated keywords
	if len(keywords) == 0 {
		return fmt.Errorf("no keywords provided")
	}
	
	// Join keywords with comma for batch query: keyword=word1,word2,word3,word4
	keywordParam := strings.Join(keywords, ",")
	
	// Build URL with query parameter (auto-detect format)
	var fullURL string
	if strings.Contains(baseURL, "?keyword=") {
		// User provided template with ?keyword= - just append the value
		fullURL = baseURL + url.QueryEscape(keywordParam)
	} else {
		// User provided base URL - add the parameter
		fullURL = baseURL + "?keyword=" + url.QueryEscape(keywordParam)
	}
	req.SetRequestURI(fullURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	
	// Set headers for API
	req.Header.Set("User-Agent", "sitemap-go/1.0")
	req.Header.Set("Accept", "application/json")
	
	// Set authorization header only if API key is provided
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	
	// Execute request using connection manager with configurable timeout
	// Default to 80 seconds for SEOKey API as per user preference
	timeout := 80 * time.Second
	if envTimeout := os.Getenv("API_TIMEOUT"); envTimeout != "" {
		if parsedTimeout, parseErr := time.ParseDuration(envTimeout); parseErr == nil {
			timeout = parsedTimeout
		}
	}
	err := c.connManager.GetFastHTTPClient().DoTimeout(req, resp, timeout)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	
	// Check status code with environment-aware error handling
	if resp.StatusCode() != fasthttp.StatusOK {
		// In development, provide more detailed error information
		if os.Getenv("DEBUG") == "true" || os.Getenv("ENVIRONMENT") == "development" {
			// Safely truncate response body for debugging (max 200 chars)
			respBody := string(resp.Body())
			if len(respBody) > 200 {
				respBody = respBody[:200] + "..."
			}
			return fmt.Errorf("API returned status %d: %s", resp.StatusCode(), respBody)
		}
		// In production, hide response body for security
		return fmt.Errorf("API returned status %d (response body hidden for security)", resp.StatusCode())
	}
	
	// Use unified SEOKey parser for consistent response handling
	parser := NewSEOKeyParser()
	apiResp, err := parser.ParseResponse(resp.Body())
	if err != nil {
		return fmt.Errorf("failed to parse SEOKey response: %w", err)
	}

	*result = apiResp
	return nil
}

// SetConcurrencyLimiter sets the concurrency limiter for this client
// Allows dynamic configuration of concurrency control
// Implements ConcurrencyConfigurable interface
func (c *httpAPIClient) SetConcurrencyLimiter(limiter ConcurrencyLimiter) {
	c.concurrencyLimiter = limiter
	c.log.Info("Concurrency limiter configured for API client")
}


