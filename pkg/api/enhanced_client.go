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

// EnhancedHTTPAPIClient with smart failover and health monitoring
// Follows Decorator Pattern: enhances basic client with advanced features
type EnhancedHTTPAPIClient struct {
	urlPool       *EnhancedURLPool
	retryStrategy RetryStrategy
	apiKey        string
	connManager   *ConnectionManager
	log           *logger.Logger
	
	// Metrics
	totalRequests  uint64
	failedRequests uint64
	totalLatency   uint64
	lastError      atomic.Value
}

// NewEnhancedHTTPAPIClient creates client with intelligent failover
func NewEnhancedHTTPAPIClient(baseURL, apiKey string, config ConnectionConfig) APIClient {
	urlPool := NewEnhancedURLPool(baseURL, true) // Prefer healthy URLs
	retryStrategy := NewSmartRetryWithFailover(
		&URLPool{urls: urlPool.urls}, // Convert for compatibility
		3,
		1*time.Second,
	)

	client := &EnhancedHTTPAPIClient{
		urlPool:       urlPool,
		retryStrategy: retryStrategy,
		apiKey:        apiKey,
		connManager:   NewConnectionManager(config),
		log:          logger.GetLogger().WithField("component", "enhanced_api_client"),
	}

	// Start health recovery background process
	ctx := context.Background() // Long-lived context for health monitoring
	client.urlPool.StartHealthRecovery(ctx, 2*time.Minute)

	return client
}

// Query with enhanced error handling and automatic failover
func (c *EnhancedHTTPAPIClient) Query(ctx context.Context, keywords []string) (*APIResponse, error) {
	atomic.AddUint64(&c.totalRequests, 1)
	start := time.Now()
	defer func() {
		atomic.AddUint64(&c.totalLatency, uint64(time.Since(start).Milliseconds()))
	}()

	// Removed detailed debug logging for cleaner output

	var result *APIResponse
	var lastURL string

	// Enhanced retry with URL health tracking
	err := c.retryStrategy.Execute(ctx, func() error {
		return c.doQueryWithHealthTracking(ctx, keywords, &result, &lastURL)
	})

	if err != nil {
		atomic.AddUint64(&c.failedRequests, 1)
		c.lastError.Store(err.Error())

		// Record failure for health tracking
		if lastURL != "" {
			c.urlPool.RecordFailure(lastURL)
		}

		c.log.WithError(err).WithField("keywords_count", len(keywords)).Error("Enhanced API query failed")
		return nil, err
	}

	// Record success for health tracking
	if lastURL != "" {
		c.urlPool.RecordSuccess(lastURL)
	}

	// Removed success logging for cleaner output
	return result, nil
}

// doQueryWithHealthTracking performs single query attempt with health tracking
func (c *EnhancedHTTPAPIClient) doQueryWithHealthTracking(ctx context.Context, keywords []string, result **APIResponse, lastURL *string) error {
	// Create fasthttp request
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	// Get next healthy URL
	baseURL := c.urlPool.Next()
	*lastURL = baseURL // Track which URL was used
	
	if baseURL == "" {
		return fmt.Errorf("no URLs available in enhanced URL pool")
	}

	// Log URL selection for debugging
	healthyCount := c.urlPool.HealthySize()
	c.log.WithFields(map[string]interface{}{
		"selected_url":    c.maskURL(baseURL),
		"healthy_urls":    healthyCount,
		"total_urls":     c.urlPool.Size(),
		"url_is_healthy": c.urlPool.IsHealthy(baseURL),
	}).Debug("URL selected for API request")

	// Validate keywords
	if len(keywords) == 0 {
		return fmt.Errorf("no keywords provided")
	}

	// Build request URL
	keywordParam := strings.Join(keywords, ",")
	var fullURL string
	if strings.Contains(baseURL, "?keyword=") {
		fullURL = baseURL + url.QueryEscape(keywordParam)
	} else {
		fullURL = baseURL + "?keyword=" + url.QueryEscape(keywordParam)
	}

	req.SetRequestURI(fullURL)
	req.Header.SetMethod(fasthttp.MethodGet)

	// Set headers
	req.Header.Set("User-Agent", "sitemap-go/2.0-enhanced")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate") // No Brotli

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Execute request with configurable timeout
	// Default to 80 seconds for SEOKey API as per user preference
	timeout := 80 * time.Second
	if envTimeout := os.Getenv("API_TIMEOUT"); envTimeout != "" {
		if parsedTimeout, parseErr := time.ParseDuration(envTimeout); parseErr == nil {
			timeout = parsedTimeout
		}
	}
	err := c.connManager.GetFastHTTPClient().DoTimeout(req, resp, timeout)
	if err != nil {
		return fmt.Errorf("request failed for URL %s: %w", c.maskURL(baseURL), err)
	}

	// Check status
	if resp.StatusCode() != fasthttp.StatusOK {
		statusErr := fmt.Errorf("API %s returned status %d: %s", c.maskURL(baseURL), resp.StatusCode(), string(resp.Body()))
		
		// Enhanced error classification for better failover decisions
		if resp.StatusCode() >= 500 {
			c.log.WithField("url", c.maskURL(baseURL)).Warn("Server error detected, will trigger failover")
		}
		
		return statusErr
	}

	// Parse response (reuse existing logic)
	return c.parseResponse(resp.Body(), result)
}

// parseResponse handles API response parsing using unified SEOKey parser
func (c *EnhancedHTTPAPIClient) parseResponse(body []byte, result **APIResponse) error {
	// Use unified SEOKey parser for consistent response handling
	parser := NewSEOKeyParser()
	apiResp, err := parser.ParseResponse(body)
	if err != nil {
		return fmt.Errorf("failed to parse SEOKey response: %w", err)
	}

	*result = apiResp
	return nil
}

// GetHealthStats returns URL health information for monitoring
func (c *EnhancedHTTPAPIClient) GetHealthStats() map[string]URLHealth {
	return c.urlPool.GetHealthStats()
}

// GetMetrics returns client performance metrics
func (c *EnhancedHTTPAPIClient) GetMetrics() ClientMetrics {
	var lastErr string
	if err := c.lastError.Load(); err != nil {
		lastErr = err.(string)
	}

	return ClientMetrics{
		TotalRequests:  atomic.LoadUint64(&c.totalRequests),
		FailedRequests: atomic.LoadUint64(&c.failedRequests),
		AvgLatencyMs:   atomic.LoadUint64(&c.totalLatency) / max(atomic.LoadUint64(&c.totalRequests), 1),
		LastError:      lastErr,
		HealthyURLs:    c.urlPool.HealthySize(),
		TotalURLs:      c.urlPool.Size(),
	}
}

// maskURL masks URL for secure logging
func (c *EnhancedHTTPAPIClient) maskURL(fullURL string) string {
	if len(fullURL) > 20 {
		return fullURL[:10] + "***" + fullURL[len(fullURL)-7:]
	}
	return "***"
}

// ClientMetrics represents API client performance data
type ClientMetrics struct {
	TotalRequests  uint64 `json:"total_requests"`
	FailedRequests uint64 `json:"failed_requests"`
	AvgLatencyMs   uint64 `json:"avg_latency_ms"`
	LastError      string `json:"last_error,omitempty"`
	HealthyURLs    int    `json:"healthy_urls"`
	TotalURLs      int    `json:"total_urls"`
}

// Helper function to find max of two uint64 values
func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

