package api

import (
	"context"
	"encoding/json"
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
	
	// Execute request using connection manager
	err := c.connManager.GetFastHTTPClient().DoTimeout(req, resp, 80*time.Second)
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
	
	// Parse seokey API response format
	var seokeyResp struct {
		Status string `json:"status"`
		Data   []struct {
			Keyword string `json:"keyword"`
			Metrics struct {
				AvgMonthlySearches int `json:"avg_monthly_searches"`
				Competition        string `json:"competition"`
				LatestSearches     int `json:"latest_searches"`
			} `json:"metrics"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(resp.Body(), &seokeyResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Convert to APIResponse format (handle multiple keywords)
	var apiResp APIResponse
	if seokeyResp.Status == "success" && len(seokeyResp.Data) > 0 {
		apiResp.Keywords = make([]Keyword, 0, len(seokeyResp.Data))
		
		for _, data := range seokeyResp.Data {
			// Map competition string to numeric value
			competitionValue := 0.5 // Default medium
			switch data.Metrics.Competition {
			case "LOW":
				competitionValue = 0.3
			case "HIGH":
				competitionValue = 0.8
			}
			
			apiResp.Keywords = append(apiResp.Keywords, Keyword{
				Word:         data.Keyword,
				SearchVolume: data.Metrics.AvgMonthlySearches,
				Competition:  competitionValue,
				CPC:          0, // SEOKey API doesn't provide CPC
			})
		}
	}
	
	*result = &apiResp
	return nil
}


