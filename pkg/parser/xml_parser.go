package parser

import (
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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
	httpClient      *http.Client
	filters         []Filter
	log             *logger.Logger
	concurrentLimit int
}

func NewXMLParser() *XMLParser {
	return &XMLParser{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		filters:         make([]Filter, 0),
		log:             logger.GetLogger().WithField("component", "xml_parser"),
		concurrentLimit: 5, // Default concurrent sitemap processing limit
	}
}

// SetConcurrentLimit sets the maximum number of concurrent sitemap fetches
func (p *XMLParser) SetConcurrentLimit(limit int) {
	if limit > 0 {
		p.concurrentLimit = limit
	}
}

func (p *XMLParser) Parse(ctx context.Context, sitemapURL string) ([]URL, error) {
	p.log.WithField("url", sitemapURL).Debug("Starting sitemap parse")
	
	// Download sitemap content
	content, err := p.downloadSitemap(ctx, sitemapURL)
	if err != nil {
		p.log.WithError(err).WithField("url", sitemapURL).Error("Failed to download sitemap")
		return nil, fmt.Errorf("failed to download sitemap: %w", err)
	}
	defer content.Close()

	// Parse XML
	decoder := xml.NewDecoder(content)
	var urls []URL

	// Try parsing as sitemap index first
	var sitemapIndex xmlSitemapIndex
	if err := decoder.Decode(&sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		p.log.WithField("count", len(sitemapIndex.Sitemaps)).Info("Processing sitemap index")
		
		// Process sitemaps concurrently
		urls = p.processSitemapsIndexConcurrently(ctx, sitemapIndex.Sitemaps)
		return urls, nil
	}

	// Reset decoder and try parsing as regular sitemap
	content.Close()
	content, err = p.downloadSitemap(ctx, sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to re-download sitemap: %w", err)
	}
	defer content.Close()

	decoder = xml.NewDecoder(content)
	var sitemap xmlSitemap
	if err := decoder.Decode(&sitemap); err != nil {
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
			p.log.WithError(err).WithField("url", xmlURL.Loc).Debug("Failed to parse URL")
			continue
		}

			// Apply filters
		if p.shouldExclude(parsedURL) {
			p.log.WithField("url", xmlURL.Loc).Debug("URL excluded by filter")
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

	p.log.WithField("count", len(urls)).Info("Successfully parsed sitemap")
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
	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Check if content is gzipped
	if strings.HasSuffix(strings.ToLower(sitemapURL), ".gz") ||
		resp.Header.Get("Content-Encoding") == "gzip" {
		return gzip.NewReader(resp.Body)
	}

	return resp.Body, nil
}

// processSitemapsIndexConcurrently processes multiple sitemaps concurrently
func (p *XMLParser) processSitemapsIndexConcurrently(ctx context.Context, sitemaps []xmlSitemapRef) []URL {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		allURLs []URL
		sem    = make(chan struct{}, p.concurrentLimit)
	)

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
				return
			default:
			}

			p.log.WithField("url", sitemapLoc).Debug("Processing sub-sitemap")
			
			subURLs, err := p.Parse(ctx, sitemapLoc)
			if err != nil {
				p.log.WithError(err).WithField("url", sitemapLoc).Warn("Failed to parse sub-sitemap")
				return
			}

			// Safely append URLs
			mu.Lock()
			allURLs = append(allURLs, subURLs...)
			mu.Unlock()
			
			p.log.WithFields(map[string]interface{}{
				"url":   sitemapLoc,
				"count": len(subURLs),
			}).Debug("Sub-sitemap processed successfully")
		}(sitemap.Loc)
	}

	wg.Wait()
	p.log.WithField("total_urls", len(allURLs)).Info("Completed processing sitemap index")
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

func hash(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}