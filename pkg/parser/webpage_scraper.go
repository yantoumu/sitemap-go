package parser

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"sitemap-go/pkg/logger"
)

// WebpageScraper extracts game URLs from website pages when sitemaps are unavailable
type WebpageScraper struct {
	httpClient DownloadClient
	log        *logger.Logger
	secureLog  *logger.SecurityLogger
	maxURLs    int
}

// NewWebpageScraper creates a new webpage scraper
func NewWebpageScraper() *WebpageScraper {
	return &WebpageScraper{
		httpClient: NewResilientHTTPClient(),
		log:        logger.GetLogger().WithField("component", "webpage_scraper"),
		secureLog:  logger.GetSecurityLogger(),
		maxURLs:    1000, // Limit to prevent excessive scraping
	}
}

// ScrapeGameURLs extracts game URLs from a website's pages
func (w *WebpageScraper) ScrapeGameURLs(ctx context.Context, baseURL string) ([]URL, error) {
	w.log.WithField("base_url", baseURL).Debug("Starting webpage scraping for game URLs")
	
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	
	var allURLs []URL
	visitedPages := make(map[string]bool)
	
	// Strategy 1: Try common game listing pages
	gamePages := w.generateGamePageURLs(parsedBase)
	
	for _, pageURL := range gamePages {
		if len(allURLs) >= w.maxURLs {
			break
		}
		
		if visitedPages[pageURL] {
			continue
		}
		visitedPages[pageURL] = true
		
		w.secureLog.DebugWithURL("Scraping game page", pageURL, nil)

		urls, err := w.scrapeGameLinksFromPage(ctx, pageURL, parsedBase)
		if err != nil {
			w.secureLog.ErrorWithURL("Failed to scrape page", pageURL, err, nil)
			continue
		}

		allURLs = append(allURLs, urls...)
		w.secureLog.DebugWithURL("Scraped URLs from page", pageURL, map[string]interface{}{
			"urls_found":  len(urls),
			"total_urls":  len(allURLs),
		})
	}
	
	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueURLs []URL
	for _, url := range allURLs {
		if !seen[url.Address] && len(uniqueURLs) < w.maxURLs {
			seen[url.Address] = true
			uniqueURLs = append(uniqueURLs, url)
		}
	}
	
	w.log.WithFields(map[string]interface{}{
		"base_url":   baseURL,
		"pages_scraped": len(visitedPages),
		"total_urls": len(uniqueURLs),
	}).Info("Completed webpage scraping")
	
	return uniqueURLs, nil
}

// generateGamePageURLs creates likely URLs for game listing pages
func (w *WebpageScraper) generateGamePageURLs(parsedBase *url.URL) []string {
	baseURL := fmt.Sprintf("%s://%s", parsedBase.Scheme, parsedBase.Host)
	
	gamePages := []string{
		baseURL + "/",                    // Homepage
		baseURL + "/games",               // Games section
		baseURL + "/games/",              // Games section with trailing slash
		baseURL + "/all-games",           // All games page
		baseURL + "/game-list",           // Game list page
		baseURL + "/popular",             // Popular games
		baseURL + "/new",                 // New games
		baseURL + "/category/games",      // Games category
		baseURL + "/arcade",              // Arcade games
		baseURL + "/action",              // Action games
		baseURL + "/puzzle",              // Puzzle games
	}
	
	return gamePages
}

// scrapeGameLinksFromPage extracts game URLs from a single page
func (w *WebpageScraper) scrapeGameLinksFromPage(ctx context.Context, pageURL string, baseURL *url.URL) ([]URL, error) {
	// Download page content
	content, err := w.httpClient.Download(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download page: %w", err)
	}
	defer content.Close()
	
	// Read content
	rawBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read page content: %w", err)
	}
	
	// Convert to string
	htmlContent := string(rawBytes)
	
	// Extract game URLs using multiple patterns
	var urls []URL
	
	// Pattern 1: Links with "game" in the URL path
	gameURLs := w.extractGameURLsWithRegex(htmlContent, baseURL)
	urls = append(urls, gameURLs...)
	
	// Pattern 2: Links in specific HTML structures (game cards, lists, etc.)
	structuralURLs := w.extractGameURLsFromStructure(htmlContent, baseURL)
	urls = append(urls, structuralURLs...)
	
	return urls, nil
}

// extractGameURLsWithRegex uses regex patterns to find game URLs
func (w *WebpageScraper) extractGameURLsWithRegex(htmlContent string, baseURL *url.URL) []URL {
	var urls []URL
	
	// Regex patterns for finding game links
	patterns := []string{
		`href=["']([^"']*(?:game|play|arcade)[^"']*)["']`,     // Links with game/play/arcade in path
		`href=["']([^"']*\/g\/[^"']*)["']`,                    // Links with /g/ pattern (common for games)
		`href=["']([^"']*\/games\/[^"']*)["']`,                // Links with /games/ in path
		`href=["']([^"']*\/play\/[^"']*)["']`,                 // Links with /play/ in path
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(htmlContent, -1)
		
		for _, match := range matches {
			if len(match) > 1 {
				urlStr := strings.TrimSpace(match[1])
				
				// Convert relative URLs to absolute
				absoluteURL := w.makeAbsoluteURL(urlStr, baseURL)
				
				if w.isValidGameURL(absoluteURL, baseURL) {
					urls = append(urls, URL{
						ID:       generateURLID(absoluteURL),
						Address:  absoluteURL,
						Keywords: []string{},
						Metadata: map[string]string{
							"source":     "regex_scraping",
							"pattern":    pattern,
							"extraction": "game_url_pattern",
						},
					})
				}
			}
		}
	}
	
	return urls
}

// extractGameURLsFromStructure finds game URLs in common HTML structures
func (w *WebpageScraper) extractGameURLsFromStructure(htmlContent string, baseURL *url.URL) []URL {
	var urls []URL
	
	// Look for common game card/item structures
	structures := []string{
		`<div[^>]*class="[^"]*game[^"]*"[^>]*>.*?href=["']([^"']*)["']`,
		`<a[^>]*class="[^"]*game[^"]*"[^>]*href=["']([^"']*)["']`,
		`<li[^>]*class="[^"]*game[^"]*"[^>]*>.*?href=["']([^"']*)["']`,
		`data-game-url=["']([^"']*)["']`,
	}
	
	for _, structure := range structures {
		re := regexp.MustCompile(structure)
		matches := re.FindAllStringSubmatch(htmlContent, -1)
		
		for _, match := range matches {
			if len(match) > 1 {
				urlStr := strings.TrimSpace(match[1])
				absoluteURL := w.makeAbsoluteURL(urlStr, baseURL)
				
				if w.isValidGameURL(absoluteURL, baseURL) {
					urls = append(urls, URL{
						ID:       generateURLID(absoluteURL),
						Address:  absoluteURL,
						Keywords: []string{},
						Metadata: map[string]string{
							"source":     "structure_scraping",
							"extraction": "html_structure",
						},
					})
				}
			}
		}
	}
	
	return urls
}

// makeAbsoluteURL converts relative URLs to absolute URLs
func (w *WebpageScraper) makeAbsoluteURL(urlStr string, baseURL *url.URL) string {
	// If already absolute, return as-is
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return urlStr
	}
	
	// Handle protocol-relative URLs
	if strings.HasPrefix(urlStr, "//") {
		return baseURL.Scheme + ":" + urlStr
	}
	
	// Handle absolute paths
	if strings.HasPrefix(urlStr, "/") {
		return fmt.Sprintf("%s://%s%s", baseURL.Scheme, baseURL.Host, urlStr)
	}
	
	// Handle relative paths (rare case)
	return fmt.Sprintf("%s://%s/%s", baseURL.Scheme, baseURL.Host, urlStr)
}

// isValidGameURL validates if a URL is likely a game URL
func (w *WebpageScraper) isValidGameURL(urlStr string, baseURL *url.URL) bool {
	if urlStr == "" || len(urlStr) > 512 {
		return false
	}
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	
	// Must be from the same domain
	if parsedURL.Host != baseURL.Host {
		return false
	}
	
	// Check if path looks like a game URL
	lowerPath := strings.ToLower(parsedURL.Path)
	
	gameIndicators := []string{
		"/game/", "/games/", "/play/", "/g/",
		"-game", "-play", "game-", "play-",
	}
	
	for _, indicator := range gameIndicators {
		if strings.Contains(lowerPath, indicator) {
			return true
		}
	}
	
	// Skip common non-game pages
	skipPatterns := []string{
		"/contact", "/about", "/privacy", "/terms",
		"/login", "/register", "/admin", "/api",
		"/css/", "/js/", "/images/", "/static/",
		".css", ".js", ".png", ".jpg", ".gif", ".ico",
	}
	
	for _, pattern := range skipPatterns {
		if strings.Contains(lowerPath, pattern) {
			return false
		}
	}
	
	return true
}