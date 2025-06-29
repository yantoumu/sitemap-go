package parser

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"sitemap-go/pkg/logger"
)

// EnhancedTXTParser handles TXT sitemaps with content-type flexibility
type EnhancedTXTParser struct {
	httpClient      DownloadClient
	filters         []Filter
	log             *logger.Logger
	secureLog       *logger.SecurityLogger
	maxLines        int
	maxLineLength   int
	acceptHTMLType  bool // Accept text/html content-type
}

// NewEnhancedTXTParser creates a new enhanced TXT parser
func NewEnhancedTXTParser() *EnhancedTXTParser {
	return &EnhancedTXTParser{
		httpClient:     NewResilientHTTPClient(), // Use resilient client
		filters:        make([]Filter, 0),
		log:            logger.GetLogger().WithField("component", "enhanced_txt_parser"),
		secureLog:      logger.GetSecurityLogger(),
		maxLines:       100000, // Increased limit
		maxLineLength:  4096,   // Increased line length
		acceptHTMLType: true,   // Accept text/html responses
	}
}

// SetHTTPClient allows injection of different HTTP client implementations
func (p *EnhancedTXTParser) SetHTTPClient(client DownloadClient) {
	p.httpClient = client
}

// Parse implements the SitemapParser interface with enhanced flexibility
func (p *EnhancedTXTParser) Parse(ctx context.Context, txtURL string) ([]URL, error) {
	p.log.Debug("Starting enhanced TXT sitemap parse")
	
	// Download content
	content, err := p.httpClient.Download(ctx, txtURL)
	if err != nil {
		p.log.WithError(err).Error("Failed to download TXT sitemap")
		return nil, fmt.Errorf("failed to download TXT sitemap: %w", err)
	}
	defer content.Close()

	// Read all content first to analyze it
	rawBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	p.log.WithFields(map[string]interface{}{
		"url":  txtURL,
		"size": len(rawBytes),
	}).Debug("Downloaded TXT content")

	// Parse the content
	return p.parseContent(rawBytes)
}

// parseContent processes the raw bytes as TXT sitemap
func (p *EnhancedTXTParser) parseContent(rawBytes []byte) ([]URL, error) {
	var urls []URL
	scanner := bufio.NewScanner(bytes.NewReader(rawBytes))
	
	// Set max token size to handle long lines
	buf := make([]byte, 0, p.maxLineLength)
	scanner.Buffer(buf, p.maxLineLength)
	
	lineCount := 0
	validURLCount := 0
	
	for scanner.Scan() && lineCount < p.maxLines {
		lineCount++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Try to parse as URL
		if p.isValidURL(line) {
			parsedURL, err := url.Parse(line)
			if err != nil {
				p.secureLog.DebugWithURL("Failed to parse URL", line, map[string]interface{}{
					"error": err.Error(),
				})
				continue
			}

			// Apply filters
			if p.shouldExclude(parsedURL) {
				p.secureLog.DebugWithURL("URL excluded by filter", line, nil)
				continue
			}
			
			urlEntry := URL{
				ID:          generateURLID(line),
				Address:     line,
				Keywords:    []string{},
				LastUpdated: "",
				Metadata: map[string]string{
					"source": "txt_sitemap",
					"line":   fmt.Sprintf("%d", lineCount),
				},
			}
			urls = append(urls, urlEntry)
			validURLCount++
		} else {
			p.log.WithField("line", line).Debug("Invalid URL format")
		}
	}
	
	if err := scanner.Err(); err != nil {
		p.log.WithError(err).Warn("Scanner error (partial results may be available)")
	}
	
	// Removed verbose success logging to reduce log noise
	
	return urls, nil
}

// isValidURL performs comprehensive URL validation using common utilities
func (p *EnhancedTXTParser) isValidURL(urlStr string) bool {
	// Use common URL validation logic
	utils := NewCommonParserUtils()
	return utils.URLValidator().IsValidURL(urlStr)
}

// SupportedFormats returns the formats supported by this parser
func (p *EnhancedTXTParser) SupportedFormats() []string {
	return []string{"txt", "text", "plain"}
}

// Validate checks if the URL is a TXT sitemap
func (p *EnhancedTXTParser) Validate(sitemapURL string) error {
	parsedURL, err := url.Parse(sitemapURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check URL path
	lowerPath := strings.ToLower(parsedURL.Path)
	
	// Accept .txt extension
	if strings.HasSuffix(lowerPath, ".txt") {
		return nil
	}
	
	// Accept URLs with "sitemap" and "txt" in path
	if strings.Contains(lowerPath, "sitemap") && strings.Contains(lowerPath, "txt") {
		return nil
	}
	
	// For enhanced parser, also accept if domain matches known TXT sitemap sites
	if p.isKnownTXTSite(parsedURL.Host) {
		return nil
	}
	
	return fmt.Errorf("not a TXT sitemap URL")
}

// isKnownTXTSite checks if the host is known to serve TXT sitemaps
func (p *EnhancedTXTParser) isKnownTXTSite(host string) bool {
	knownSites := []string{
		"lagged.com",
		"www.lagged.com",
	}
	
	for _, site := range knownSites {
		if strings.EqualFold(host, site) {
			return true
		}
	}
	
	return false
}

// AddFilter adds a filter to exclude certain URLs
func (p *EnhancedTXTParser) AddFilter(filter Filter) {
	p.filters = append(p.filters, filter)
}

// shouldExclude checks if a URL should be excluded by filters
func (p *EnhancedTXTParser) shouldExclude(u *url.URL) bool {
	for _, filter := range p.filters {
		if filter.ShouldExclude(u) {
			return true
		}
	}
	return false
}