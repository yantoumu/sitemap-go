package parser

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"sitemap-go/pkg/logger"
)

// EmptyContentHandler handles edge cases like empty responses and redirects
type EmptyContentHandler struct {
	httpClient    DownloadClient
	webScraper    *WebpageScraper
	log           *logger.Logger
	enableScraping bool
}

// NewEmptyContentHandler creates a new handler for empty/invalid content
func NewEmptyContentHandler() *EmptyContentHandler {
	return &EmptyContentHandler{
		httpClient:    NewResilientHTTPClient(),
		webScraper:    NewWebpageScraper(),
		log:           logger.GetLogger().WithField("component", "empty_content_handler"),
		enableScraping: true, // Enable webpage scraping as fallback
	}
}

// Handle attempts to resolve empty or invalid sitemap responses
func (h *EmptyContentHandler) Handle(ctx context.Context, sitemapURL string) ([]URL, error) {
	h.log.WithField("url", sitemapURL).Debug("Handling potentially empty/invalid sitemap")
	
	// Strategy 1: Try direct download first
	content, err := h.httpClient.Download(ctx, sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer content.Close()
	
	rawBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}
	
	h.log.WithFields(map[string]interface{}{
		"url":         sitemapURL,
		"content_size": len(rawBytes),
	}).Debug("Downloaded content for analysis")
	
	// Check if content is empty or whitespace only
	contentStr := strings.TrimSpace(string(rawBytes))
	if len(contentStr) == 0 {
		// Strategy 2: Try alternate URLs
		return h.tryAlternateURLs(ctx, sitemapURL)
	}
	
	// Check if content is HTML error page
	if h.isHTMLErrorPage(contentStr) {
		h.log.WithField("url", sitemapURL).Debug("Detected HTML error page, trying alternatives")
		return h.tryAlternateURLs(ctx, sitemapURL)
	}
	
	// Strategy 3: If content exists but is invalid, try to extract URLs anyway
	urls := h.extractURLsFromAnyContent(contentStr)
	if len(urls) > 0 {
		return urls, nil
	}
	
	return nil, fmt.Errorf("no valid URLs found in content")
}

// tryAlternateURLs tries common sitemap URL patterns
func (h *EmptyContentHandler) tryAlternateURLs(ctx context.Context, originalURL string) ([]URL, error) {
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return nil, fmt.Errorf("invalid original URL: %w", err)
	}
	
	// Generate alternate URLs
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	alternates := []string{
		baseURL + "/sitemap.xml",
		baseURL + "/sitemap_index.xml",
		baseURL + "/sitemap-index.xml",
		baseURL + "/sitemap.txt",
		baseURL + "/sitemap-games.xml",
		baseURL + "/sitemap-posts.xml",
		baseURL + "/sitemap1.xml",
		baseURL + "/sitemap_main.xml",
		baseURL + "/robots.txt", // Sometimes contains sitemap references
		// Try without www prefix if current URL has www
		strings.Replace(baseURL, "://www.", "://", 1) + "/sitemap.xml",
	}
	
	// Remove the original URL if it's in the list
	var filteredAlternates []string
	for _, alt := range alternates {
		if alt != originalURL {
			filteredAlternates = append(filteredAlternates, alt)
		}
	}
	
	h.log.WithFields(map[string]interface{}{
		"original":    originalURL,
		"alternates":  len(filteredAlternates),
	}).Debug("Trying alternate URLs")
	
	// Try each alternate
	for _, altURL := range filteredAlternates {
		h.log.WithField("trying", altURL).Debug("Testing alternate URL")
		
		content, err := h.httpClient.Download(ctx, altURL)
		if err != nil {
			h.log.WithError(err).WithField("url", altURL).Debug("Alternate URL failed")
			continue
		}
		
		rawBytes, err := io.ReadAll(content)
		content.Close()
		if err != nil {
			continue
		}
		
		contentStr := strings.TrimSpace(string(rawBytes))
		if len(contentStr) == 0 {
			continue
		}
		
		// Try to extract URLs from this alternate
		urls := h.extractURLsFromAnyContent(contentStr)
		if len(urls) > 0 {
			h.log.WithFields(map[string]interface{}{
				"alternate_url": altURL,
				"urls_found":   len(urls),
			}).Info("Found URLs in alternate sitemap")
			return urls, nil
		}
	}
	
	// Final fallback: webpage scraping
	if h.enableScraping {
		h.log.WithField("original_url", originalURL).Info("Attempting webpage scraping as last resort")
		
		baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
		scrapedURLs, err := h.webScraper.ScrapeGameURLs(ctx, baseURL)
		if err != nil {
			h.log.WithError(err).Debug("Webpage scraping failed")
		} else if len(scrapedURLs) > 0 {
			h.log.WithFields(map[string]interface{}{
				"base_url":   baseURL,
				"urls_found": len(scrapedURLs),
			}).Info("Successfully scraped URLs from website")
			return scrapedURLs, nil
		}
	}
	
	return nil, fmt.Errorf("no valid alternate sitemaps found and scraping unsuccessful")
}

// isHTMLErrorPage detects if content is an HTML error page
func (h *EmptyContentHandler) isHTMLErrorPage(content string) bool {
	lowerContent := strings.ToLower(content)
	
	// Check for HTML indicators
	if strings.Contains(lowerContent, "<!doctype html") ||
		strings.Contains(lowerContent, "<html") ||
		strings.Contains(lowerContent, "<head>") ||
		strings.Contains(lowerContent, "<body>") {
		
		// Check for error indicators
		errorIndicators := []string{
			"404", "not found", "page not found",
			"500", "internal server error",
			"403", "forbidden", "access denied",
			"error", "oops", "something went wrong",
		}
		
		for _, indicator := range errorIndicators {
			if strings.Contains(lowerContent, indicator) {
				return true
			}
		}
		
		// If it's HTML but doesn't look like XML, it's probably an error page
		if !strings.Contains(lowerContent, "<?xml") &&
			!strings.Contains(lowerContent, "<urlset") &&
			!strings.Contains(lowerContent, "<sitemapindex") {
			return true
		}
	}
	
	return false
}

// extractURLsFromAnyContent tries to extract URLs from any text content
func (h *EmptyContentHandler) extractURLsFromAnyContent(content string) []URL {
	var urls []URL
	
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and common XML elements
		if line == "" || 
			strings.HasPrefix(line, "<?xml") ||
			strings.HasPrefix(line, "<urlset") ||
			strings.HasPrefix(line, "<sitemapindex") ||
			strings.HasPrefix(line, "</") ||
			strings.HasPrefix(line, "<!--") {
			continue
		}
		
		// Look for URLs in various formats
		urls = append(urls, h.findURLsInLine(line, lineNum)...)
	}
	
	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueURLs []URL
	for _, url := range urls {
		if !seen[url.Address] {
			seen[url.Address] = true
			uniqueURLs = append(uniqueURLs, url)
		}
	}
	
	h.log.WithField("urls_extracted", len(uniqueURLs)).Debug("Extracted URLs from content")
	return uniqueURLs
}

// findURLsInLine extracts URLs from a single line of text
func (h *EmptyContentHandler) findURLsInLine(line string, lineNum int) []URL {
	var urls []URL
	
	// Pattern 1: Direct URL (starts with http/https)
	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		if h.isValidURL(line) {
			urls = append(urls, URL{
				ID:       generateURLID(line),
				Address:  line,
				Keywords: []string{},
				Metadata: map[string]string{
					"source": "content_extraction",
					"line":   fmt.Sprintf("%d", lineNum),
				},
			})
		}
	}
	
	// Pattern 2: XML <loc> tags
	if strings.Contains(line, "<loc>") && strings.Contains(line, "</loc>") {
		start := strings.Index(line, "<loc>") + 5
		end := strings.Index(line, "</loc>")
		if start < end {
			urlStr := strings.TrimSpace(line[start:end])
			if h.isValidURL(urlStr) {
				urls = append(urls, URL{
					ID:       generateURLID(urlStr),
					Address:  urlStr,
					Keywords: []string{},
					Metadata: map[string]string{
						"source": "xml_loc_tag",
						"line":   fmt.Sprintf("%d", lineNum),
					},
				})
			}
		}
	}
	
	// Pattern 3: Quoted URLs
	parts := strings.Fields(line)
	for _, part := range parts {
		// Remove common quotes and brackets
		cleaned := strings.Trim(part, "\"'<>()[]")
		if h.isValidURL(cleaned) {
			urls = append(urls, URL{
				ID:       generateURLID(cleaned),
				Address:  cleaned,
				Keywords: []string{},
				Metadata: map[string]string{
					"source": "quoted_url",
					"line":   fmt.Sprintf("%d", lineNum),
				},
			})
		}
	}
	
	return urls
}

// isValidURL validates if a string is a valid URL using common utilities
func (h *EmptyContentHandler) isValidURL(urlStr string) bool {
	utils := NewCommonParserUtils()
	return utils.URLValidator().IsValidURL(urlStr)
}