package parser

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

// HTTPClient provides a shared fasthttp client with browser-like headers
type HTTPClient struct {
	client     *fasthttp.Client
	userAgents []string
}

// NewHTTPClient creates a new HTTP client for sitemap parsing
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &fasthttp.Client{
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/121.0",
		},
	}
}

// Download fetches content from URL with browser-like headers
func (h *HTTPClient) Download(ctx context.Context, targetURL string) (io.ReadCloser, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	// Set request properties
	req.SetRequestURI(targetURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	
	// Add browser-like headers for anti-bot protection
	h.setRequestHeaders(req, targetURL)

	// Execute request with timeout
	err := h.client.DoTimeout(req, resp, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode())
	}

	// Copy response body
	bodyBytes := make([]byte, len(resp.Body()))
	copy(bodyBytes, resp.Body())
	
	reader := &bytesReadCloser{bytes: bodyBytes}

	// Check if content is gzipped
	if h.isGzipped(targetURL, resp) {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			reader.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return &gzipReadCloser{gzipReader: gzipReader, underlying: reader}, nil
	}

	return reader, nil
}

// setRequestHeaders adds browser-like headers to avoid bot detection
func (h *HTTPClient) setRequestHeaders(req *fasthttp.Request, targetURL string) {
	// Rotate user agents to avoid detection
	userAgent := h.userAgents[hash(targetURL)%uint32(len(h.userAgents))]
	req.Header.SetUserAgent(userAgent)
	
	// Set common browser headers
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	
	// Add referrer based on domain
	parsedURL, err := url.Parse(targetURL)
	if err == nil {
		req.Header.Set("Referer", fmt.Sprintf("%s://%s/", parsedURL.Scheme, parsedURL.Host))
	}
	
	// Cache control
	req.Header.Set("Cache-Control", "max-age=0")
}

// isGzipped checks if the content is gzipped
func (h *HTTPClient) isGzipped(targetURL string, resp *fasthttp.Response) bool {
	return strings.HasSuffix(strings.ToLower(targetURL), ".gz") ||
		string(resp.Header.Peek("Content-Encoding")) == "gzip"
}

// Hash function for consistent user agent rotation
func hash(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

// bytesReadCloser implements io.ReadCloser for byte slices
type bytesReadCloser struct {
	bytes  []byte
	offset int
}

func (b *bytesReadCloser) Read(p []byte) (n int, err error) {
	if b.offset >= len(b.bytes) {
		return 0, io.EOF
	}
	n = copy(p, b.bytes[b.offset:])
	b.offset += n
	return n, nil
}

func (b *bytesReadCloser) Close() error {
	return nil
}

// gzipReadCloser implements io.ReadCloser for gzip content
type gzipReadCloser struct {
	gzipReader *gzip.Reader
	underlying io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (n int, err error) {
	return g.gzipReader.Read(p)
}

func (g *gzipReadCloser) Close() error {
	if err := g.gzipReader.Close(); err != nil {
		g.underlying.Close()
		return err
	}
	return g.underlying.Close()
}