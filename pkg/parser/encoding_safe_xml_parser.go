package parser

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"sitemap-go/pkg/logger"
)

// EncodingSafeXMLParser handles XML with encoding issues and syntax errors
type EncodingSafeXMLParser struct {
	httpClient      DownloadClient
	filters         []Filter
	log             *logger.Logger
	concurrentLimit int
	cleaningEnabled bool
}

// DownloadClient interface for HTTP client abstraction
type DownloadClient interface {
	Download(ctx context.Context, url string) (io.ReadCloser, error)
}

// NewEncodingSafeXMLParser creates a new encoding-safe XML parser
func NewEncodingSafeXMLParser() *EncodingSafeXMLParser {
	return &EncodingSafeXMLParser{
		httpClient:      NewResilientHTTPClient(),
		filters:         make([]Filter, 0),
		log:             logger.GetLogger().WithField("component", "encoding_safe_xml_parser"),
		concurrentLimit: 5,
		cleaningEnabled: true,
	}
}

// SetHTTPClient allows injection of different HTTP client implementations
func (p *EncodingSafeXMLParser) SetHTTPClient(client DownloadClient) {
	p.httpClient = client
}

// Parse implements intelligent XML parsing with encoding detection and error recovery
func (p *EncodingSafeXMLParser) Parse(ctx context.Context, sitemapURL string) ([]URL, error) {
	p.log.Debug("Starting encoding-safe sitemap parse")
	
	// Download sitemap content
	content, err := p.httpClient.Download(ctx, sitemapURL)
	if err != nil {
		p.log.WithError(err).Error("Failed to download sitemap")
		return nil, fmt.Errorf("failed to download sitemap: %w", err)
	}
	defer content.Close()

	// Read all content into memory for multiple parsing attempts
	rawBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read sitemap content: %w", err)
	}

	p.log.WithFields(map[string]interface{}{
		"url":  sitemapURL,
		"size": len(rawBytes),
	}).Debug("Downloaded sitemap content")

	// Strategy 1: Standard UTF-8 parsing
	if urls, err := p.parseWithStrategy(ctx, rawBytes, p.standardUTF8Parse, "standard-utf8"); err == nil {
		return urls, nil
	}

	// Strategy 2: Encoding detection and conversion
	if urls, err := p.parseWithStrategy(ctx, rawBytes, p.encodingDetectionParse, "encoding-detection"); err == nil {
		return urls, nil
	}

	// Strategy 3: XML cleaning and sanitization
	if urls, err := p.parseWithStrategy(ctx, rawBytes, p.cleaningParse, "xml-cleaning"); err == nil {
		return urls, nil
	}

	// Strategy 4: Fallback regex extraction
	if urls, err := p.parseWithStrategy(ctx, rawBytes, p.regexFallbackParse, "regex-fallback"); err == nil {
		return urls, nil
	}

	return nil, fmt.Errorf("all parsing strategies failed for sitemap: %s", sitemapURL)
}

func (p *EncodingSafeXMLParser) parseWithStrategy(ctx context.Context, rawBytes []byte, strategy func([]byte) ([]URL, error), strategyName string) ([]URL, error) {
	p.log.WithField("strategy", strategyName).Debug("Attempting parsing strategy")
	
	urls, err := strategy(rawBytes)
	if err != nil {
		p.log.WithError(err).WithField("strategy", strategyName).Debug("Strategy failed")
		return nil, err
	}
	
	p.log.WithFields(map[string]interface{}{
		"strategy": strategyName,
		"count":    len(urls),
	}).Info("Strategy succeeded")
	
	return urls, nil
}

// standardUTF8Parse attempts standard XML parsing assuming UTF-8 encoding
func (p *EncodingSafeXMLParser) standardUTF8Parse(rawBytes []byte) ([]URL, error) {
	if !utf8.Valid(rawBytes) {
		return nil, fmt.Errorf("content is not valid UTF-8")
	}
	
	return p.parseXMLContent(rawBytes)
}

// encodingDetectionParse detects encoding and converts to UTF-8
func (p *EncodingSafeXMLParser) encodingDetectionParse(rawBytes []byte) ([]URL, error) {
	// Detect encoding from XML declaration or BOM
	encoding := p.detectEncoding(rawBytes)
	
	if encoding != nil {
		p.log.WithField("detected_encoding", encoding).Debug("Converting encoding")
		
		// Convert to UTF-8
		reader := transform.NewReader(bytes.NewReader(rawBytes), encoding.NewDecoder())
		convertedBytes, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("encoding conversion failed: %w", err)
		}
		
		return p.parseXMLContent(convertedBytes)
	}
	
	return nil, fmt.Errorf("could not detect encoding")
}

// cleaningParse removes illegal XML characters and attempts parsing
func (p *EncodingSafeXMLParser) cleaningParse(rawBytes []byte) ([]URL, error) {
	// Clean the XML content
	cleanedBytes := p.cleanXMLContent(rawBytes)
	
	// Ensure it's valid UTF-8 after cleaning
	if !utf8.Valid(cleanedBytes) {
		// Try to fix UTF-8 issues
		cleanedBytes = p.fixUTF8Issues(cleanedBytes)
	}
	
	return p.parseXMLContent(cleanedBytes)
}

// regexFallbackParse extracts URLs using regex patterns
func (p *EncodingSafeXMLParser) regexFallbackParse(rawBytes []byte) ([]URL, error) {
	p.log.Debug("Using regex fallback parsing")
	
	// Convert bytes to string, replacing invalid UTF-8
	content := string(rawBytes)
	if !utf8.ValidString(content) {
		content = strings.ToValidUTF8(content, "")
	}
	
	var urls []URL
	
	// Pattern for URL extraction from XML sitemaps
	patterns := []string{
		`<loc[^>]*>([^<]+)</loc>`,           // Standard sitemap format
		`<url[^>]*>([^<]+)</url>`,           // Alternative format  
		`<link[^>]*>([^<]+)</link>`,         // RSS/Atom format
		`<guid[^>]*>([^<]+)</guid>`,         // RSS guid format
		`href=["']([^"']+)["']`,             // HTML-style links
	}
	
	extractedURLs := make(map[string]bool)
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		
		for _, match := range matches {
			if len(match) > 1 {
				urlStr := strings.TrimSpace(match[1])
				
				// Validate URL format
				if p.isValidURL(urlStr) && !extractedURLs[urlStr] {
					extractedURLs[urlStr] = true
					
					// Apply filters
					if parsedURL, err := url.Parse(urlStr); err == nil {
						if !p.shouldExclude(parsedURL) {
							url := URL{
								ID:          generateURLID(urlStr),
								Address:     urlStr,
								Keywords:    []string{},
								LastUpdated: "",
								Metadata:    map[string]string{"source": "regex_fallback"},
							}
							urls = append(urls, url)
						}
					}
				}
			}
		}
	}
	
	if len(urls) == 0 {
		return nil, fmt.Errorf("no valid URLs found with regex fallback")
	}
	
	return urls, nil
}

// detectEncoding attempts to detect the character encoding of XML content
func (p *EncodingSafeXMLParser) detectEncoding(rawBytes []byte) encoding.Encoding {
	// Check for BOM
	if len(rawBytes) >= 3 {
		if bytes.Equal(rawBytes[:3], []byte{0xEF, 0xBB, 0xBF}) {
			return unicode.UTF8 // UTF-8 BOM
		}
	}
	if len(rawBytes) >= 2 {
		if bytes.Equal(rawBytes[:2], []byte{0xFF, 0xFE}) {
			// UTF-16 LE BOM - simplified for now
			return unicode.UTF8 // Fallback to UTF-8
		}
		if bytes.Equal(rawBytes[:2], []byte{0xFE, 0xFF}) {
			// UTF-16 BE BOM - simplified for now
			return unicode.UTF8 // Fallback to UTF-8
		}
	}
	
	// Check XML declaration
	content := string(rawBytes[:min(512, len(rawBytes))])
	
	// Look for encoding declaration
	re := regexp.MustCompile(`encoding=["']([^"']+)["']`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		encodingName := strings.ToLower(matches[1])
		
		switch encodingName {
		case "iso-8859-1", "latin1":
			return charmap.ISO8859_1
		case "iso-8859-15", "latin9":
			return charmap.ISO8859_15
		case "windows-1252", "cp1252":
			return charmap.Windows1252
		case "windows-1251", "cp1251":
			return charmap.Windows1251
		case "utf-8":
			return unicode.UTF8
		case "utf-16":
			return unicode.UTF8 // Simplified fallback for now
		}
	}
	
	// Heuristic: if content contains mostly valid UTF-8, assume UTF-8
	if utf8.Valid(rawBytes) {
		return unicode.UTF8
	}
	
	// Default fallback to ISO-8859-1 (covers most Western European content)
	return charmap.ISO8859_1
}

// cleanXMLContent removes illegal XML characters and fixes common issues
func (p *EncodingSafeXMLParser) cleanXMLContent(rawBytes []byte) []byte {
	content := string(rawBytes)
	
	// Remove illegal XML characters (control characters except tab, newline, carriage return)
	cleaned := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return r // Keep valid whitespace
		}
		if r < 0x20 || (r >= 0xFFFE && r <= 0xFFFF) {
			return -1 // Remove illegal characters
		}
		if (r >= 0xD800 && r <= 0xDFFF) {
			return -1 // Remove surrogate pairs
		}
		return r
	}, content)
	
	// Fix common XML issues
	cleaned = p.fixCommonXMLIssues(cleaned)
	
	return []byte(cleaned)
}

// fixCommonXMLIssues addresses typical XML syntax problems
func (p *EncodingSafeXMLParser) fixCommonXMLIssues(content string) string {
	// Fix unescaped ampersands - simple approach
	// Replace & that are not followed by common entities
	content = strings.ReplaceAll(content, "&amp;", "TEMP_AMPERSAND")
	content = strings.ReplaceAll(content, "&lt;", "TEMP_LT")
	content = strings.ReplaceAll(content, "&gt;", "TEMP_GT")
	content = strings.ReplaceAll(content, "&quot;", "TEMP_QUOT")
	content = strings.ReplaceAll(content, "&apos;", "TEMP_APOS")
	content = strings.ReplaceAll(content, "&#", "TEMP_NUMREF")
	
	// Now replace remaining & with &amp;
	content = strings.ReplaceAll(content, "&", "&amp;")
	
	// Restore valid entities
	content = strings.ReplaceAll(content, "TEMP_AMPERSAND", "&amp;")
	content = strings.ReplaceAll(content, "TEMP_LT", "&lt;")
	content = strings.ReplaceAll(content, "TEMP_GT", "&gt;")
	content = strings.ReplaceAll(content, "TEMP_QUOT", "&quot;")
	content = strings.ReplaceAll(content, "TEMP_APOS", "&apos;")
	content = strings.ReplaceAll(content, "TEMP_NUMREF", "&#")
	
	// Fix CDATA sections - simplified approach
	if strings.Contains(content, "<![CDATA[") {
		// Process CDATA sections manually
		parts := strings.Split(content, "<![CDATA[")
		for i := 1; i < len(parts); i++ {
			if idx := strings.Index(parts[i], "]]>"); idx > 0 {
				cdataContent := parts[i][:idx]
				// Escape content inside CDATA
				cdataContent = strings.ReplaceAll(cdataContent, "<", "&lt;")
				cdataContent = strings.ReplaceAll(cdataContent, ">", "&gt;")
				parts[i] = cdataContent + parts[i][idx:]
			}
		}
		content = strings.Join(parts, "<![CDATA[")
	}
	
	return content
}

// fixUTF8Issues attempts to fix UTF-8 encoding problems
func (p *EncodingSafeXMLParser) fixUTF8Issues(rawBytes []byte) []byte {
	// Replace invalid UTF-8 sequences with replacement character
	validUTF8 := make([]byte, 0, len(rawBytes))
	
	for len(rawBytes) > 0 {
		r, size := utf8.DecodeRune(rawBytes)
		if r == utf8.RuneError {
			// Skip invalid byte
			size = 1
		} else {
			// Append valid rune
			buf := make([]byte, utf8.RuneLen(r))
			utf8.EncodeRune(buf, r)
			validUTF8 = append(validUTF8, buf...)
		}
		rawBytes = rawBytes[size:]
	}
	
	return validUTF8
}

// parseXMLContent parses clean XML content into URL structures
func (p *EncodingSafeXMLParser) parseXMLContent(content []byte) ([]URL, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.CharsetReader = p.charsetReader
	
	var urls []URL
	
	// Try parsing as sitemap index first
	var sitemapIndex xmlSitemapIndex
	if err := decoder.Decode(&sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		p.log.WithField("count", len(sitemapIndex.Sitemaps)).Info("Processing sitemap index")
		// Note: This would need recursive processing, simplified for now
		for _, sitemap := range sitemapIndex.Sitemaps {
			if sitemap.Loc != "" {
				url := URL{
					ID:          generateURLID(sitemap.Loc),
					Address:     sitemap.Loc,
					Keywords:    []string{},
					LastUpdated: sitemap.LastMod,
					Metadata:    map[string]string{"type": "sitemap_index"},
				}
				urls = append(urls, url)
			}
		}
		return urls, nil
	}
	
	// Reset decoder and try parsing as regular sitemap
	decoder = xml.NewDecoder(bytes.NewReader(content))
	decoder.CharsetReader = p.charsetReader
	
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
			Keywords:    []string{},
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

// charsetReader provides custom charset reading for XML decoder
func (p *EncodingSafeXMLParser) charsetReader(charset string, input io.Reader) (io.Reader, error) {
	charset = strings.ToLower(charset)
	
	switch charset {
	case "iso-8859-1", "latin1":
		return transform.NewReader(input, charmap.ISO8859_1.NewDecoder()), nil
	case "iso-8859-15", "latin9":
		return transform.NewReader(input, charmap.ISO8859_15.NewDecoder()), nil
	case "windows-1252", "cp1252":
		return transform.NewReader(input, charmap.Windows1252.NewDecoder()), nil
	case "windows-1251", "cp1251":
		return transform.NewReader(input, charmap.Windows1251.NewDecoder()), nil
	default:
		return input, nil
	}
}

// isValidURL performs basic URL validation
func (p *EncodingSafeXMLParser) isValidURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	
	return parsedURL.Scheme != "" && parsedURL.Host != ""
}

// Implement other required methods
func (p *EncodingSafeXMLParser) SupportedFormats() []string {
	return []string{"xml", "xml.gz"}
}

func (p *EncodingSafeXMLParser) Validate(sitemapURL string) error {
	parsedURL, err := url.Parse(sitemapURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	lowerURL := strings.ToLower(parsedURL.Path)
	for _, format := range p.SupportedFormats() {
		if strings.HasSuffix(lowerURL, "."+format) {
			return nil
		}
	}

	if strings.Contains(lowerURL, "sitemap") {
		return nil
	}

	return fmt.Errorf("unsupported sitemap format")
}

func (p *EncodingSafeXMLParser) AddFilter(filter Filter) {
	p.filters = append(p.filters, filter)
}

func (p *EncodingSafeXMLParser) shouldExclude(u *url.URL) bool {
	for _, filter := range p.filters {
		if filter.ShouldExclude(u) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}