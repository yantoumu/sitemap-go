package parser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"sitemap-go/pkg/logger"
)

type TXTParser struct {
	httpClient      *HTTPClient
	filters         []Filter
	log             *logger.Logger
	maxLines        int
	maxLineLength   int
	utils           *CommonParserUtils // Add common utilities
}

func NewTXTParser() *TXTParser {
	return &TXTParser{
		httpClient:    NewHTTPClient(),
		filters:       make([]Filter, 0),
		log:           logger.GetLogger().WithField("component", "txt_parser"),
		maxLines:      50000,  // Limit to prevent memory issues
		maxLineLength: 2048,   // Limit line length
		utils:         NewCommonParserUtils(), // Initialize common utilities
	}
}

func (p *TXTParser) SetLimits(maxLines, maxLineLength int) {
	if maxLines > 0 {
		p.maxLines = maxLines
	}
	if maxLineLength > 0 {
		p.maxLineLength = maxLineLength
	}
}

func (p *TXTParser) Parse(ctx context.Context, txtURL string) ([]URL, error) {
	p.log.Debug("Starting TXT sitemap parse")
	
	// Download TXT content
	content, err := p.downloadTXT(ctx, txtURL)
	if err != nil {
		p.log.WithError(err).Error("Failed to download TXT sitemap")
		return nil, fmt.Errorf("failed to download TXT sitemap: %w", err)
	}
	defer content.Close()

	// Parse line by line
	urls := make([]URL, 0)
	scanner := bufio.NewScanner(content)
	
	// Set buffer limits
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, p.maxLineLength)
	
	lineCount := 0
	for scanner.Scan() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			p.log.Warn("Context cancelled, stopping TXT parsing")
			return urls, ctx.Err()
		default:
		}

		lineCount++
		if lineCount > p.maxLines {
			p.log.WithField("max_lines", p.maxLines).Warn("Reached maximum line limit")
			break
		}

		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Validate and process URL
		urlStruct, err := p.processURL(line)
		if err != nil {
			// Skip invalid URLs silently to avoid log spam
			continue
		}

		if urlStruct != nil {
			urls = append(urls, *urlStruct)
		}
	}

	if err := scanner.Err(); err != nil {
		p.log.WithError(err).Error("Error reading TXT sitemap")
		return nil, fmt.Errorf("error reading TXT sitemap: %w", err)
	}

	// Removed verbose success logging to reduce log noise
	
	return urls, nil
}

func (p *TXTParser) SupportedFormats() []string {
	return []string{"txt", "text"}
}

func (p *TXTParser) Validate(txtURL string) error {
	parsedURL, err := url.Parse(txtURL)
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

	// Also accept URLs with "sitemap" in path
	if strings.Contains(lowerURL, "sitemap") && strings.Contains(lowerURL, "txt") {
		return nil
	}

	return fmt.Errorf("unsupported TXT sitemap format")
}

func (p *TXTParser) AddFilter(filter Filter) {
	p.filters = append(p.filters, filter)
}

func (p *TXTParser) downloadTXT(ctx context.Context, txtURL string) (io.ReadCloser, error) {
	return p.httpClient.Download(ctx, txtURL)
}

func (p *TXTParser) processURL(urlStr string) (*URL, error) {
	// Use common URL validation
	if err := p.utils.URLValidator().ValidateURL(urlStr); err != nil {
		return nil, err
	}

	// Parse URL (we know it's valid now)
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Apply filters
	if p.shouldExclude(parsedURL) {
		p.log.WithField("url", urlStr).Debug("URL excluded by filter")
		return nil, nil
	}

	return &URL{
		ID:          generateURLID(urlStr),
		Address:     urlStr,
		Keywords:    []string{}, // Keywords will be extracted later
		LastUpdated: time.Now().Format(time.RFC3339),
		Metadata: map[string]string{
			"source": "txt",
		},
	}, nil
}

func (p *TXTParser) shouldExclude(u *url.URL) bool {
	for _, filter := range p.filters {
		if filter.ShouldExclude(u) {
			return true
		}
	}
	return false
}