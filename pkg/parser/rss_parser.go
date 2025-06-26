package parser

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"sitemap-go/pkg/logger"
)

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type RSSParser struct {
	httpClient      *HTTPClient
	filters         []Filter
	log             *logger.Logger
	concurrentLimit int
}

func NewRSSParser() *RSSParser {
	return &RSSParser{
		httpClient:      NewHTTPClient(),
		filters:         make([]Filter, 0),
		log:             logger.GetLogger().WithField("component", "rss_parser"),
		concurrentLimit: 5,
	}
}

func (p *RSSParser) SetConcurrentLimit(limit int) {
	if limit > 0 {
		p.concurrentLimit = limit
	}
}

func (p *RSSParser) Parse(ctx context.Context, rssURL string) ([]URL, error) {
	p.log.WithField("url", rssURL).Debug("Starting RSS parse")
	
	// Download RSS content
	content, err := p.downloadRSS(ctx, rssURL)
	if err != nil {
		p.log.WithError(err).WithField("url", rssURL).Error("Failed to download RSS")
		return nil, fmt.Errorf("failed to download RSS: %w", err)
	}
	defer content.Close()

	// Parse RSS XML
	decoder := xml.NewDecoder(content)
	var feed rssFeed
	if err := decoder.Decode(&feed); err != nil {
		p.log.WithError(err).WithField("url", rssURL).Error("Failed to parse RSS XML")
		return nil, fmt.Errorf("failed to parse RSS XML: %w", err)
	}

	// Convert RSS items to URL structs
	urls := make([]URL, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		if item.Link == "" {
			continue
		}

		// Parse URL to apply filters
		parsedURL, err := url.Parse(item.Link)
		if err != nil {
			p.log.WithError(err).WithField("url", item.Link).Debug("Failed to parse URL")
			continue
		}

		// Apply filters
		if p.shouldExclude(parsedURL) {
			p.log.WithField("url", item.Link).Debug("URL excluded by filter")
			continue
		}

		urlStruct := URL{
			ID:          generateURLID(item.Link),
			Address:     item.Link,
			Keywords:    []string{}, // Keywords will be extracted later
			LastUpdated: p.parseRSSDate(item.PubDate),
			Metadata: map[string]string{
				"title":       item.Title,
				"description": item.Description,
				"guid":        item.GUID,
				"source":      "rss",
			},
		}
		urls = append(urls, urlStruct)
	}

	p.log.WithField("count", len(urls)).Info("Successfully parsed RSS feed")
	return urls, nil
}

func (p *RSSParser) SupportedFormats() []string {
	return []string{"rss", "xml", "feed"}
}

func (p *RSSParser) Validate(rssURL string) error {
	parsedURL, err := url.Parse(rssURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check if URL indicates RSS format
	lowerURL := strings.ToLower(parsedURL.Path)
	for _, format := range p.SupportedFormats() {
		if strings.Contains(lowerURL, format) {
			return nil
		}
	}

	// Also accept common RSS paths
	rssPatterns := []string{"/feed", "/rss", "/atom"}
	for _, pattern := range rssPatterns {
		if strings.Contains(lowerURL, pattern) {
			return nil
		}
	}

	return fmt.Errorf("unsupported RSS format")
}

func (p *RSSParser) AddFilter(filter Filter) {
	p.filters = append(p.filters, filter)
}

func (p *RSSParser) downloadRSS(ctx context.Context, rssURL string) (io.ReadCloser, error) {
	return p.httpClient.Download(ctx, rssURL)
}

func (p *RSSParser) shouldExclude(u *url.URL) bool {
	for _, filter := range p.filters {
		if filter.ShouldExclude(u) {
			return true
		}
	}
	return false
}

func (p *RSSParser) parseRSSDate(dateStr string) string {
	if dateStr == "" {
		return time.Now().Format(time.RFC3339)
	}

	// Common RSS date formats
	formats := []string{
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
		time.RFC822Z,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if parsedTime, err := time.Parse(format, dateStr); err == nil {
			return parsedTime.Format(time.RFC3339)
		}
	}

	// If parsing fails, return current time
	p.log.WithField("date", dateStr).Debug("Failed to parse RSS date, using current time")
	return time.Now().Format(time.RFC3339)
}