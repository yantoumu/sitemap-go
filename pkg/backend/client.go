package backend

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valyala/fasthttp"
	"sitemap-go/pkg/logger"
)

type httpBackendClient struct {
	config              BackendConfig
	client              *fasthttp.Client
	log                 *logger.Logger
	concurrentSubmitter *ConcurrentSubmitter
}

// NewBackendClient creates a new backend API client
func NewBackendClient(config BackendConfig) (BackendClient, error) {
	if config.BatchSize == 0 {
		config.BatchSize = 4 // Default batch size: 4 keywords per request
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second // Default timeout
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("backend API key is required - set BACKEND_API_KEY environment variable")
	}

	// Create reusable client with production-optimized settings
	client := &fasthttp.Client{
		ReadTimeout:         config.Timeout,
		WriteTimeout:        config.Timeout,
		MaxConnsPerHost:     20,  // Reduced from 100 to prevent overwhelming backend
		MaxIdleConnDuration: 30 * time.Second,  // Reduced from 90s for better resource management
		MaxConnDuration:     5 * time.Minute,   // Add max connection duration
	}

	backendClient := &httpBackendClient{
		config: config,
		client: client,
		log:    logger.GetLogger().WithField("component", "backend_client"),
	}

	// Initialize concurrent submitter
	backendClient.concurrentSubmitter = NewConcurrentSubmitter(backendClient)

	return backendClient, nil
}

// SubmitBatch submits a single batch of keyword metrics with GZIP compression
func (c *httpBackendClient) SubmitBatch(batch KeywordMetricsBatch) (*BackendResponse, error) {
	c.log.WithField("batch_size", len(batch)).Debug("Submitting keyword metrics batch")

	// Marshal JSON
	jsonData, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch data: %w", err)
	}

	c.log.WithField("json_size_bytes", len(jsonData)).Debug("JSON data marshaled")

	var requestBody []byte
	var contentEncoding string

	// Apply GZIP compression if enabled
	if c.config.EnableGzip {
		var buf bytes.Buffer
		gzipWriter := gzip.NewWriter(&buf)
		
		if _, err := gzipWriter.Write(jsonData); err != nil {
			gzipWriter.Close()
			return nil, fmt.Errorf("failed to write to gzip: %w", err)
		}
		
		if err := gzipWriter.Close(); err != nil {
			return nil, fmt.Errorf("failed to close gzip writer: %w", err)
		}
		
		requestBody = buf.Bytes()
		contentEncoding = "gzip"
		
		c.log.WithFields(map[string]interface{}{
			"original_size":   len(jsonData),
			"compressed_size": len(requestBody),
			"compression_ratio": fmt.Sprintf("%.2f%%", float64(len(requestBody))/float64(len(jsonData))*100),
		}).Debug("Data compressed with GZIP")
	} else {
		requestBody = jsonData
	}

	// Create fasthttp request with safe resource management
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	// Ensure resources are always released, even in panic situations
	defer func() {
		if req != nil {
			fasthttp.ReleaseRequest(req)
		}
		if resp != nil {
			fasthttp.ReleaseResponse(resp)
		}
	}()

	// Set request properties
	url := c.config.BaseURL + "/api/v1/keyword-metrics/batch"
	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.Header.Set("X-API-Key", c.config.APIKey)
	
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	
	req.SetBody(requestBody)

	c.log.WithFields(map[string]interface{}{
		"url":              url,
		"content_encoding": contentEncoding,
		"request_size":     len(requestBody),
	}).Debug("Sending request to backend API")

	// Execute request with timeout using reusable client
	err = c.client.DoTimeout(req, resp, c.config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check status code - accept both 200 OK and 202 Accepted
	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK && statusCode != fasthttp.StatusAccepted {
		// Don't expose full response body in error - potential backend info leak
		return nil, fmt.Errorf("Backend API returned status %d (response body hidden for security)", statusCode)
	}

	// Parse response
	var backendResp BackendResponse
	if err := json.Unmarshal(resp.Body(), &backendResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.log.WithFields(map[string]interface{}{
		"response_code":    backendResp.Code,
		"response_message": backendResp.Message,
		"batch_size":       len(batch),
	}).Info("Backend submission completed")

	return &backendResp, nil
}

// SubmitBatches splits data into batches and submits them with controlled concurrency
func (c *httpBackendClient) SubmitBatches(data []KeywordMetricsData) error {
	return c.concurrentSubmitter.SubmitBatchesConcurrently(data, c.config.BatchSize)
}

