package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"sitemap-go/pkg/logger"
)

type httpAPIClient struct {
	baseURL       string
	apiKey        string
	httpClient    *http.Client
	breaker       *CircuitBreaker
	log           *logger.Logger
	
	// Metrics
	totalRequests uint64
	failedRequests uint64
	totalLatency  uint64
	lastError     atomic.Value
}

func NewHTTPAPIClient(baseURL, apiKey string) APIClient {
	return &httpAPIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		breaker: NewCircuitBreaker(5, 30*time.Second),
		log:     logger.GetLogger().WithField("component", "api_client"),
	}
}

func (c *httpAPIClient) Query(ctx context.Context, keywords []string) (*APIResponse, error) {
	atomic.AddUint64(&c.totalRequests, 1)
	start := time.Now()
	defer func() {
		atomic.AddUint64(&c.totalLatency, uint64(time.Since(start).Milliseconds()))
	}()
	
	c.log.WithField("keywords_count", len(keywords)).Debug("Starting API query")
	
	var result *APIResponse
	err := c.breaker.Execute(ctx, func() error {
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
	// Prepare request body
	reqBody := map[string]interface{}{
		"keywords": keywords,
	}
	
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/keywords/batch", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	*result = &apiResp
	return nil
}

func (c *httpAPIClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}
	
	return nil
}

func (c *httpAPIClient) GetMetrics() *APIMetrics {
	total := atomic.LoadUint64(&c.totalRequests)
	failed := atomic.LoadUint64(&c.failedRequests)
	latency := atomic.LoadUint64(&c.totalLatency)
	
	avgLatency := float64(0)
	if total > 0 {
		avgLatency = float64(latency) / float64(total)
	}
	
	successRate := float64(1)
	if total > 0 {
		successRate = float64(total-failed) / float64(total)
	}
	
	
	return &APIMetrics{
		RequestCount:  int64(total),
		ErrorCount:    int64(failed),
		SuccessRate:   successRate,
		AvgLatency:    avgLatency,
	}
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