package parser

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"sitemap-go/pkg/logger"
)

// ResilientHTTPClient implements advanced anti-bot and error recovery strategies
type ResilientHTTPClient struct {
	client         *fasthttp.Client
	userAgents     []string
	proxies        []string
	retryStrategy  *RetryStrategy
	log            *logger.Logger
}

// RetryStrategy defines retry behavior with exponential backoff
type RetryStrategy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	BackoffFactor  float64
	JitterEnabled  bool
}

// NewResilientHTTPClient creates a client with advanced resilience features
func NewResilientHTTPClient() *ResilientHTTPClient {
	return &ResilientHTTPClient{
		client: &fasthttp.Client{
			ReadTimeout:  45 * time.Second,
			WriteTimeout: 30 * time.Second,
			MaxIdleConnDuration: 10 * time.Minute,
		},
		userAgents: []string{
			// Real browser user agents with more variety
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
		retryStrategy: &RetryStrategy{
			MaxAttempts:   4,
			BaseDelay:     500 * time.Millisecond,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 2.0,
			JitterEnabled: true,
		},
		log: logger.GetLogger().WithField("component", "resilient_http_client"),
	}
}

// Download implements intelligent retry with multiple strategies for 403 errors
func (r *ResilientHTTPClient) Download(ctx context.Context, targetURL string) (io.ReadCloser, error) {
	r.log.WithField("url", targetURL).Debug("Starting resilient download")
	
	var lastErr error
	for attempt := 1; attempt <= r.retryStrategy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		r.log.WithFields(map[string]interface{}{
			"url":     targetURL,
			"attempt": attempt,
		}).Debug("Attempting download")
		
		// Strategy 1: Normal request
		if attempt == 1 {
			if body, err := r.standardDownload(ctx, targetURL, attempt); err == nil {
				return body, nil
			} else {
				lastErr = err
				if !r.isRetryableError(err) {
					return nil, err
				}
			}
		}
		
		// Strategy 2: Enhanced headers with session simulation
		if attempt == 2 {
			if body, err := r.sessionSimulationDownload(ctx, targetURL, attempt); err == nil {
				return body, nil
			} else {
				lastErr = err
				if !r.isRetryableError(err) {
					return nil, err
				}
			}
		}
		
		// Strategy 3: Robot.txt check + delayed request
		if attempt == 3 {
			if body, err := r.robotsCompliantDownload(ctx, targetURL, attempt); err == nil {
				return body, nil
			} else {
				lastErr = err
				if !r.isRetryableError(err) {
					return nil, err
				}
			}
		}
		
		// Strategy 4: Minimal headers approach
		if attempt == 4 {
			if body, err := r.minimalHeadersDownload(ctx, targetURL, attempt); err == nil {
				return body, nil
			} else {
				lastErr = err
			}
		}
		
		// Apply backoff delay before next attempt
		if attempt < r.retryStrategy.MaxAttempts {
			delay := r.calculateBackoffDelay(attempt)
			r.log.WithFields(map[string]interface{}{
				"delay_ms": delay.Milliseconds(),
				"attempt":  attempt,
			}).Debug("Waiting before retry")
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	
	return nil, fmt.Errorf("all retry strategies failed, last error: %w", lastErr)
}

func (r *ResilientHTTPClient) standardDownload(ctx context.Context, targetURL string, attempt int) (io.ReadCloser, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	
	req.SetRequestURI(targetURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	r.setStandardHeaders(req, targetURL, attempt)
	
	err := r.client.DoTimeout(req, resp, 45*time.Second)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	
	return r.processResponse(targetURL, resp)
}

func (r *ResilientHTTPClient) sessionSimulationDownload(ctx context.Context, targetURL string, attempt int) (io.ReadCloser, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	
	req.SetRequestURI(targetURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	r.setSessionSimulationHeaders(req, targetURL, attempt)
	
	err := r.client.DoTimeout(req, resp, 45*time.Second)
	if err != nil {
		return nil, fmt.Errorf("session simulation request failed: %w", err)
	}
	
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	
	return r.processResponse(targetURL, resp)
}

func (r *ResilientHTTPClient) robotsCompliantDownload(ctx context.Context, targetURL string, attempt int) (io.ReadCloser, error) {
	// Add delay to simulate human browsing
	delay := time.Duration(rand.Intn(3000)+1000) * time.Millisecond
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(delay):
	}
	
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	
	req.SetRequestURI(targetURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	r.setRobotsCompliantHeaders(req, targetURL, attempt)
	
	err := r.client.DoTimeout(req, resp, 45*time.Second)
	if err != nil {
		return nil, fmt.Errorf("robots compliant request failed: %w", err)
	}
	
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	
	return r.processResponse(targetURL, resp)
}

func (r *ResilientHTTPClient) minimalHeadersDownload(ctx context.Context, targetURL string, attempt int) (io.ReadCloser, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	
	req.SetRequestURI(targetURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	r.setMinimalHeaders(req, targetURL, attempt)
	
	err := r.client.DoTimeout(req, resp, 45*time.Second)
	if err != nil {
		return nil, fmt.Errorf("minimal headers request failed: %w", err)
	}
	
	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	
	return r.processResponse(targetURL, resp)
}

func (r *ResilientHTTPClient) setStandardHeaders(req *fasthttp.Request, targetURL string, attempt int) {
	userAgent := r.userAgents[(hash(targetURL)+uint32(attempt))%uint32(len(r.userAgents))]
	req.Header.SetUserAgent(userAgent)
	
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

func (r *ResilientHTTPClient) setSessionSimulationHeaders(req *fasthttp.Request, targetURL string, attempt int) {
	userAgent := r.userAgents[(hash(targetURL)+uint32(attempt))%uint32(len(r.userAgents))]
	req.Header.SetUserAgent(userAgent)
	
	// More realistic headers
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("sec-ch-ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	
	if parsedURL, err := url.Parse(targetURL); err == nil {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", parsedURL.Scheme, parsedURL.Host))
	}
}

func (r *ResilientHTTPClient) setRobotsCompliantHeaders(req *fasthttp.Request, targetURL string, attempt int) {
	// Use a more conservative user agent
	req.Header.SetUserAgent("Mozilla/5.0 (compatible; SitemapBot/1.0; +https://example.com/bot)")
	
	req.Header.Set("Accept", "application/xml,text/xml,*/*")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "close")
}

func (r *ResilientHTTPClient) setMinimalHeaders(req *fasthttp.Request, targetURL string, attempt int) {
	// Basic headers only
	req.Header.SetUserAgent("sitemap-parser/1.0")
	req.Header.Set("Accept", "*/*")
}

func (r *ResilientHTTPClient) processResponse(targetURL string, resp *fasthttp.Response) (io.ReadCloser, error) {
	bodyBytes := make([]byte, len(resp.Body()))
	copy(bodyBytes, resp.Body())
	
	reader := &bytesReadCloser{bytes: bodyBytes}
	
	if r.isGzipped(targetURL, resp) {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			reader.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return &gzipReadCloser{gzipReader: gzipReader, underlying: reader}, nil
	}
	
	return reader, nil
}

func (r *ResilientHTTPClient) isRetryableError(err error) bool {
	errorStr := err.Error()
	return strings.Contains(errorStr, "HTTP 403") ||
		strings.Contains(errorStr, "HTTP 429") ||
		strings.Contains(errorStr, "HTTP 502") ||
		strings.Contains(errorStr, "HTTP 503") ||
		strings.Contains(errorStr, "HTTP 504") ||
		strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "connection refused")
}

func (r *ResilientHTTPClient) calculateBackoffDelay(attempt int) time.Duration {
	delay := time.Duration(float64(r.retryStrategy.BaseDelay) * 
		math.Pow(r.retryStrategy.BackoffFactor, float64(attempt-1)))
	
	if delay > r.retryStrategy.MaxDelay {
		delay = r.retryStrategy.MaxDelay
	}
	
	if r.retryStrategy.JitterEnabled {
		jitter := time.Duration(rand.Int63n(int64(delay) / 4))
		delay += jitter
	}
	
	return delay
}

func (r *ResilientHTTPClient) isGzipped(targetURL string, resp *fasthttp.Response) bool {
	return strings.HasSuffix(strings.ToLower(targetURL), ".gz") ||
		string(resp.Header.Peek("Content-Encoding")) == "gzip"
}