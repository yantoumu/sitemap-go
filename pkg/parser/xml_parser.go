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
	"time"
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

type xmlSitemapIndex struct {
	XMLName  xml.Name      `xml:"sitemapindex"`
	Sitemaps []xmlSitemap  `xml:"sitemap"`
}

type XMLParser struct {
	httpClient *http.Client
	filters    []Filter
}

func NewXMLParser() *XMLParser {
	return &XMLParser{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		filters: make([]Filter, 0),
	}
}

func (p *XMLParser) Parse(ctx context.Context, sitemapURL string) ([]URL, error) {
	// Download sitemap content
	content, err := p.downloadSitemap(ctx, sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download sitemap: %w", err)
	}
	defer content.Close()

	// Parse XML
	decoder := xml.NewDecoder(content)
	var urls []URL

	// Try parsing as sitemap index first
	var sitemapIndex xmlSitemapIndex
	if err := decoder.Decode(&sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		// It's a sitemap index, recursively parse each sitemap
		for _, sitemap := range sitemapIndex.Sitemaps {
			if sitemap.Loc != "" {
				subURLs, err := p.Parse(ctx, sitemap.Loc)
				if err != nil {
					// Log error but continue with other sitemaps
					continue
				}
				urls = append(urls, subURLs...)
			}
		}
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
			continue
		}

		// Apply filters
		if p.shouldExclude(parsedURL) {
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