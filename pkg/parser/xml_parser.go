package parser

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"sitemap-go/pkg/logger"
)

type xmlURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

type xmlSitemap struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []xmlURL `xml:"url"`
}

type xmlSitemapRef struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type xmlSitemapIndex struct {
	XMLName  xml.Name       `xml:"sitemapindex"`
	Sitemaps []xmlSitemapRef `xml:"sitemap"`
}

type XMLParser struct {
	httpClient      *HTTPClient
	filters         []Filter
	log             *logger.Logger
	concurrentLimit int
}

func NewXMLParser() *XMLParser {
	return &XMLParser{
		httpClient:      NewHTTPClient(),
		filters:         make([]Filter, 0),
		log:             logger.GetLogger().WithField("component", "xml_parser"),
		concurrentLimit: 2, // 降低并发数：减少服务器压力，避免被限流
	}
}

// SetConcurrentLimit sets the maximum number of concurrent sitemap fetches
func (p *XMLParser) SetConcurrentLimit(limit int) {
	if limit > 0 {
		p.concurrentLimit = limit
	}
}

func (p *XMLParser) Parse(ctx context.Context, sitemapURL string) ([]URL, error) {
	p.log.Debug("Starting sitemap parse")
	
	// Download sitemap content
	content, err := p.downloadSitemap(ctx, sitemapURL)
	if err != nil {
		p.log.WithError(err).Error("Failed to download sitemap")
		return nil, fmt.Errorf("failed to download sitemap: %w", err)
	}
	defer content.Close()

	// Read all content into memory first
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read sitemap content: %w", err)
	}

	var urls []URL

	// Try parsing as sitemap index first
	var sitemapIndex xmlSitemapIndex
	if err := xml.Unmarshal(data, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		p.log.WithField("count", len(sitemapIndex.Sitemaps)).Info("Processing sitemap index")
		
		// Process sitemaps concurrently
		urls = p.processSitemapsIndexConcurrently(ctx, sitemapIndex.Sitemaps)
		return urls, nil
	}

	// Try parsing as regular sitemap (using same data)
	var sitemap xmlSitemap
	if err := xml.Unmarshal(data, &sitemap); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// Convert XML URLs to our URL struct
	for _, xmlURL := range sitemap.URLs {
		if xmlURL.Loc == "" {
			continue
		}

			// Parse URL to apply filters
		parsedURL, err := url.Parse(xmlURL.Loc)
		if err != nil {
			// Skip invalid URLs silently to avoid log spam
			continue
		}

			// Apply filters
		if p.shouldExclude(parsedURL) {
			// Skip excluded URLs silently to avoid log spam
			continue
		}

		url := URL{
			ID:          generateURLID(xmlURL.Loc),
			Address:     xmlURL.Loc,
			Keywords:    []string{}, // Keywords will be extracted later
			LastUpdated: xmlURL.LastMod,
			Metadata: map[string]string{
				"changefreq": xmlURL.ChangeFreq,
				"priority":   xmlURL.Priority,
			},
		}
		urls = append(urls, url)
	}

	// Removed verbose success logging to reduce log noise
	return urls, nil
}

func (p *XMLParser) SupportedFormats() []string {
	return []string{"xml", "xml.gz"}
}

func (p *XMLParser) Validate(sitemapURL string) error {
	parsedURL, err := url.Parse(sitemapURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check if URL ends with supported format
	lowerURL := strings.ToLower(parsedURL.Path)
	for _, format := range p.SupportedFormats() {
		if strings.HasSuffix(lowerURL, "."+format) {
			return nil
		}
	}

	// Also accept URLs without extension (common for sitemaps)
	if strings.Contains(lowerURL, "sitemap") {
		return nil
	}

	return fmt.Errorf("unsupported sitemap format")
}

func (p *XMLParser) AddFilter(filter Filter) {
	p.filters = append(p.filters, filter)
}

func (p *XMLParser) downloadSitemap(ctx context.Context, sitemapURL string) (io.ReadCloser, error) {
	return p.httpClient.Download(ctx, sitemapURL)
}

// processSitemapsIndexConcurrently processes multiple sitemaps concurrently with improved safety
func (p *XMLParser) processSitemapsIndexConcurrently(ctx context.Context, sitemaps []xmlSitemapRef) []URL {
	// Use channel for safe result collection instead of shared slice
	type sitemapResult struct {
		urls []URL
		err  error
		url  string
	}
	
	resultChan := make(chan sitemapResult, len(sitemaps))
	sem := make(chan struct{}, p.concurrentLimit)
	var wg sync.WaitGroup

	for _, sitemap := range sitemaps {
		if sitemap.Loc == "" {
			continue
		}

		wg.Add(1)
		go func(sitemapLoc string) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check context cancellation
			select {
			case <-ctx.Done():
				p.log.Warn("Context cancelled, stopping sitemap processing")
				resultChan <- sitemapResult{urls: nil, err: ctx.Err(), url: sitemapLoc}
				return
			default:
			}

			p.log.Debug("Processing sub-sitemap")
			
			subURLs, err := p.Parse(ctx, sitemapLoc)
			
			// Send result via channel (thread-safe)
			resultChan <- sitemapResult{
				urls: subURLs,
				err:  err,
				url:  sitemapLoc,
			}
			
			if err != nil {
				p.log.WithError(err).Warn("Failed to parse sub-sitemap")
			}
		}(sitemap.Loc)
	}

	// Close result channel when all workers complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results efficiently without mutex contention
	var allURLs []URL
	totalExpectedURLs := 0 // Pre-estimate capacity
	
	// First pass: collect results and estimate total size
	results := make([]sitemapResult, 0, len(sitemaps))
	for result := range resultChan {
		results = append(results, result)
		if result.err == nil {
			totalExpectedURLs += len(result.urls)
		}
	}
	
	// Second pass: allocate exact capacity and copy (避免多次reallocation)
	allURLs = make([]URL, 0, totalExpectedURLs)
	for _, result := range results {
		if result.err == nil {
			allURLs = append(allURLs, result.urls...)
		}
	}

	// Removed verbose success logging to reduce log noise
	return allURLs
}

func (p *XMLParser) shouldExclude(u *url.URL) bool {
	for _, filter := range p.filters {
		if filter.ShouldExclude(u) {
			return true
		}
	}
	return false
}

func generateURLID(address string) string {
	// Simple ID generation - in production, use a proper hash
	return fmt.Sprintf("%d", hash(address))
}


