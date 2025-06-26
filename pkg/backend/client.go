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
	config BackendConfig
	client *fasthttp.Client
	log    *logger.Logger
}

// NewBackendClient creates a new backend API client
func NewBackendClient(config BackendConfig) (BackendClient, error) {
	if config.BatchSize == 0 {
		config.BatchSize = 300 // Default batch size
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second // Default timeout
	}
	if config.APIKey == "" {
		return nil, fmt.Errorf("backend API key is required - set BACKEND_API_KEY environment variable")
	}

	// Create reusable client with optimized settings
	client := &fasthttp.Client{
		ReadTimeout:         config.Timeout,
		WriteTimeout:        config.Timeout,
		MaxConnsPerHost:     100,
		MaxIdleConnDuration: 90 * time.Second,
	}

	return &httpBackendClient{
		config: config,
		client: client,
		log:    logger.GetLogger().WithField("component", "backend_client"),
	}, nil
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

	// Create fasthttp request
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

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

	// Check status code
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
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

// SubmitBatches splits data into batches and submits them sequentially
func (c *httpBackendClient) SubmitBatches(data []KeywordMetricsData) error {
	if len(data) == 0 {
		c.log.Debug("No data to submit")
		return nil
	}

	totalBatches := (len(data) + c.config.BatchSize - 1) / c.config.BatchSize
	
	c.log.WithFields(map[string]interface{}{
		"total_keywords": len(data),
		"batch_size":     c.config.BatchSize,
		"total_batches":  totalBatches,
	}).Info("Starting batch submission")

	successCount := 0
	failureCount := 0

	for i := 0; i < len(data); i += c.config.BatchSize {
		end := i + c.config.BatchSize
		if end > len(data) {
			end = len(data)
		}

		batchData := data[i:end]
		batchNum := i/c.config.BatchSize + 1

		c.log.WithFields(map[string]interface{}{
			"batch_number": batchNum,
			"batch_size":   len(batchData),
			"progress":     fmt.Sprintf("%d/%d", batchNum, totalBatches),
		}).Info("Submitting batch")

		resp, err := c.SubmitBatch(KeywordMetricsBatch(batchData))
		if err != nil {
			c.log.WithError(err).WithField("batch_number", batchNum).Error("Batch submission failed")
			failureCount++
			continue
		}

		if resp.Code != 0 {
			c.log.WithFields(map[string]interface{}{
				"batch_number":     batchNum,
				"response_code":    resp.Code,
				"response_message": resp.Message,
			}).Error("Backend API returned error")
			failureCount++
			continue
		}

		successCount++
		c.log.WithField("batch_number", batchNum).Info("Batch submitted successfully")

		// Small delay between batches to avoid overwhelming the backend
		time.Sleep(100 * time.Millisecond)
	}

	c.log.WithFields(map[string]interface{}{
		"total_batches":    totalBatches,
		"successful_batches": successCount,
		"failed_batches":   failureCount,
		"success_rate":     fmt.Sprintf("%.1f%%", float64(successCount)/float64(totalBatches)*100),
	}).Info("Batch submission completed")

	if failureCount > 0 {
		return fmt.Errorf("failed to submit %d out of %d batches", failureCount, totalBatches)
	}

	return nil
}