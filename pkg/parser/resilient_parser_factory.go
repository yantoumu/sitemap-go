package parser

import (
	"context"
	"fmt"
	"strings"

	"sitemap-go/pkg/logger"
)

// ResilientParserFactory implements the Factory pattern with multiple parser strategies
type ResilientParserFactory struct {
	parsers []SitemapParser
	log     *logger.Logger
}

// ParserStrategy defines different parsing approaches
type ParserStrategy int

const (
	StandardStrategy ParserStrategy = iota
	TXTStrategy
	EncodingSafeStrategy
	FallbackStrategy
	EmptyContentStrategy
)

// NewResilientParserFactory creates a factory with multiple parser implementations
func NewResilientParserFactory() *ResilientParserFactory {
	factory := &ResilientParserFactory{
		parsers: make([]SitemapParser, 0),
		log:     logger.GetLogger().WithField("component", "resilient_parser_factory"),
	}

	// Initialize parsers in order of preference
	factory.initializeParsers()
	return factory
}

// initializeParsers sets up all available parser strategies
func (f *ResilientParserFactory) initializeParsers() {
	// Strategy 1: Standard XML parser (fastest, for well-formed content)
	standardParser := NewXMLParser()
	f.parsers = append(f.parsers, standardParser)

	// Strategy 2: Enhanced TXT parser (for TXT sitemaps with flexible content-type)
	enhancedTXTParser := NewEnhancedTXTParser()
	f.parsers = append(f.parsers, enhancedTXTParser)

	// Strategy 3: Encoding-safe parser (for encoding/syntax issues)
	encodingSafeParser := NewEncodingSafeXMLParser()
	f.parsers = append(f.parsers, encodingSafeParser)

	// Strategy 4: Hybrid resilient parser (combines all strategies)
	hybridParser := NewEncodingSafeXMLParser()
	hybridParser.SetHTTPClient(NewResilientHTTPClient())
	f.parsers = append(f.parsers, hybridParser)

	// Strategy 5: Empty content handler (for empty responses and edge cases)
	emptyContentHandler := &EmptyContentHandlerWrapper{
		handler: NewEmptyContentHandler(),
	}
	f.parsers = append(f.parsers, emptyContentHandler)

	f.log.WithField("parser_count", len(f.parsers)).Info("Initialized resilient parser factory")
}

// CreateParser selects the best parser based on URL characteristics and error history
func (f *ResilientParserFactory) CreateParser(sitemapURL string, errorHistory []error) SitemapParser {
	f.log.WithFields(map[string]interface{}{
		"url":           sitemapURL,
		"error_history": len(errorHistory),
	}).Debug("Selecting optimal parser")

	// Analyze URL characteristics
	urlProfile := f.analyzeURL(sitemapURL)
	
	// Analyze error history
	errorProfile := f.analyzeErrorHistory(errorHistory)
	
	// Select best strategy based on analysis
	strategy := f.selectStrategy(urlProfile, errorProfile)
	
	parser := f.getParserByStrategy(strategy)
	
	f.log.WithFields(map[string]interface{}{
		"url":      sitemapURL,
		"strategy": strategy,
		"parser":   fmt.Sprintf("%T", parser),
	}).Debug("Selected parser strategy")
	
	return parser
}

// Parse attempts parsing with intelligent strategy selection and fallback
func (f *ResilientParserFactory) Parse(ctx context.Context, sitemapURL string) ([]URL, error) {
	f.log.WithField("url", sitemapURL).Debug("Starting resilient parsing")
	
	var errorHistory []error
	
	// Try each parser in order until success
	for i, parser := range f.parsers {
		f.log.WithFields(map[string]interface{}{
			"url":     sitemapURL,
			"attempt": i + 1,
			"parser":  fmt.Sprintf("%T", parser),
		}).Debug("Attempting parse with strategy")
		
		urls, err := parser.Parse(ctx, sitemapURL)
		if err == nil && len(urls) > 0 {
			f.log.WithFields(map[string]interface{}{
				"url":     sitemapURL,
				"parser":  fmt.Sprintf("%T", parser),
				"count":   len(urls),
				"attempt": i + 1,
			}).Info("Parsing succeeded")
			return urls, nil
		}
		
		if err != nil {
			errorHistory = append(errorHistory, err)
			f.log.WithError(err).WithFields(map[string]interface{}{
				"url":     sitemapURL,
				"parser":  fmt.Sprintf("%T", parser),
				"attempt": i + 1,
			}).Debug("Parser strategy failed")
		}
		
		// Early exit for non-retryable errors (if all parsers would fail)
		if f.isNonRetryableError(err) && i == 0 {
			f.log.WithError(err).WithField("url", sitemapURL).Warn("Non-retryable error detected, skipping remaining strategies")
			break
		}
	}
	
	// All strategies failed
	f.log.WithFields(map[string]interface{}{
		"url":            sitemapURL,
		"failed_parsers": len(f.parsers),
		"errors":         len(errorHistory),
	}).Error("All parsing strategies failed")
	
	return nil, fmt.Errorf("all %d parsing strategies failed for %s, last error: %w", 
		len(f.parsers), sitemapURL, errorHistory[len(errorHistory)-1])
}

// URLProfile contains characteristics of the sitemap URL
type URLProfile struct {
	IsHTTPS             bool
	HasGzipExtension    bool
	Domain              string
	PathContainsFeed    bool
	PathContainsIndex   bool
	IsTXTSitemap        bool
	LikelyProblemSite   bool
	LikelyEmptyContent  bool
}

// ErrorProfile contains analysis of previous parsing errors
type ErrorProfile struct {
	HasHTTPErrors     bool
	HasEncodingErrors bool
	HasXMLSyntaxErrors bool
	HasTimeoutErrors  bool
	ErrorCount        int
}

// analyzeURL examines URL characteristics to predict potential issues
func (f *ResilientParserFactory) analyzeURL(sitemapURL string) URLProfile {
	lower := strings.ToLower(sitemapURL)
	
	profile := URLProfile{
		IsHTTPS:           strings.HasPrefix(lower, "https://"),
		HasGzipExtension:  strings.HasSuffix(lower, ".gz"),
		PathContainsFeed:  strings.Contains(lower, "feed") || strings.Contains(lower, "rss"),
		PathContainsIndex: strings.Contains(lower, "index"),
		IsTXTSitemap:      strings.HasSuffix(lower, ".txt"),
	}
	
	// Extract domain
	if idx := strings.Index(lower, "://"); idx > 0 {
		remaining := lower[idx+3:]
		if idx := strings.Index(remaining, "/"); idx > 0 {
			profile.Domain = remaining[:idx]
		} else {
			profile.Domain = remaining
		}
	}
	
	// Identify sites known to have anti-bot protection
	problemSites := []string{
		"brightestgames.com", "puzzleplayground.com", "kizgame.com",
		"wordle2.io", "play-games.com", "superkidgames.com", "sprunki.org",
		"cloudflare", "incapsula", "akamai",
	}
	
	for _, site := range problemSites {
		if strings.Contains(profile.Domain, site) {
			profile.LikelyProblemSite = true
			break
		}
	}
	
	// Identify sites known to have empty content issues
	emptySites := []string{
		"playgame24.com",
	}
	
	for _, site := range emptySites {
		if strings.Contains(profile.Domain, site) {
			profile.LikelyEmptyContent = true
			break
		}
	}
	
	return profile
}

// analyzeErrorHistory examines previous errors to inform strategy selection
func (f *ResilientParserFactory) analyzeErrorHistory(errors []error) ErrorProfile {
	profile := ErrorProfile{
		ErrorCount: len(errors),
	}
	
	for _, err := range errors {
		errorStr := strings.ToLower(err.Error())
		
		if strings.Contains(errorStr, "http 403") || 
		   strings.Contains(errorStr, "http 429") ||
		   strings.Contains(errorStr, "http 502") ||
		   strings.Contains(errorStr, "http 503") {
			profile.HasHTTPErrors = true
		}
		
		if strings.Contains(errorStr, "encoding") || 
		   strings.Contains(errorStr, "utf-8") ||
		   strings.Contains(errorStr, "charset") {
			profile.HasEncodingErrors = true
		}
		
		if strings.Contains(errorStr, "xml") && 
		   (strings.Contains(errorStr, "syntax") || 
		    strings.Contains(errorStr, "illegal") ||
		    strings.Contains(errorStr, "invalid")) {
			profile.HasXMLSyntaxErrors = true
		}
		
		if strings.Contains(errorStr, "timeout") ||
		   strings.Contains(errorStr, "deadline") {
			profile.HasTimeoutErrors = true
		}
	}
	
	return profile
}

// EmptyContentHandlerWrapper adapts EmptyContentHandler to SitemapParser interface
type EmptyContentHandlerWrapper struct {
	handler *EmptyContentHandler
}

func (w *EmptyContentHandlerWrapper) Parse(ctx context.Context, sitemapURL string) ([]URL, error) {
	return w.handler.Handle(ctx, sitemapURL)
}

func (w *EmptyContentHandlerWrapper) SupportedFormats() []string {
	return []string{"empty", "invalid", "any"}
}

func (w *EmptyContentHandlerWrapper) Validate(sitemapURL string) error {
	// This handler accepts any URL as it's a fallback
	return nil
}

// selectStrategy chooses the best parsing strategy based on URL and error analysis
func (f *ResilientParserFactory) selectStrategy(urlProfile URLProfile, errorProfile ErrorProfile) ParserStrategy {
	// Check for TXT sitemaps first
	if urlProfile.IsTXTSitemap || strings.Contains(strings.ToLower(urlProfile.Domain), "lagged") {
		return TXTStrategy
	}
	
	// Check for sites likely to have empty content
	if urlProfile.LikelyEmptyContent {
		return EmptyContentStrategy
	}
	
	// If we have HTTP errors or known problem sites, use encoding-safe strategy
	if errorProfile.HasHTTPErrors || urlProfile.LikelyProblemSite {
		if errorProfile.HasEncodingErrors || errorProfile.HasXMLSyntaxErrors {
			return FallbackStrategy // Use hybrid approach
		}
		return EncodingSafeStrategy
	}
	
	// If we have encoding or XML syntax errors, use encoding-safe strategy
	if errorProfile.HasEncodingErrors || errorProfile.HasXMLSyntaxErrors {
		return EncodingSafeStrategy
	}
	
	// If we have timeout errors but no other issues, try encoding-safe
	if errorProfile.HasTimeoutErrors {
		return EncodingSafeStrategy
	}
	
	// For first attempt or no specific issues, use standard strategy
	if errorProfile.ErrorCount == 0 {
		return StandardStrategy
	}
	
	// Default to fallback strategy if we have multiple error types
	return FallbackStrategy
}

// getParserByStrategy returns the appropriate parser for the given strategy
func (f *ResilientParserFactory) getParserByStrategy(strategy ParserStrategy) SitemapParser {
	switch strategy {
	case StandardStrategy:
		return f.parsers[0] // Standard XML parser
	case TXTStrategy:
		return f.parsers[1] // Enhanced TXT parser
	case EncodingSafeStrategy:
		return f.parsers[2] // Encoding-safe parser
	case FallbackStrategy:
		return f.parsers[3] // Hybrid resilient parser
	case EmptyContentStrategy:
		return f.parsers[4] // Empty content handler
	default:
		return f.parsers[0] // Default to standard
	}
}

// isNonRetryableError determines if an error should stop all retry attempts
func (f *ResilientParserFactory) isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := strings.ToLower(err.Error())
	
	// These errors are unlikely to be resolved by different parsing strategies
	nonRetryablePatterns := []string{
		"invalid url",
		"unsupported protocol",
		"no such host",
		"connection refused",
		"network unreachable",
	}
	
	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}
	
	return false
}

// GetAvailableStrategies returns all available parsing strategies
func (f *ResilientParserFactory) GetAvailableStrategies() []string {
	return []string{
		"Standard XML Parser",
		"Enhanced TXT Parser",
		"Encoding-Safe XML Parser", 
		"Hybrid Resilient Parser",
		"Empty Content Handler",
	}
}

// GetParserCount returns the number of available parsers
func (f *ResilientParserFactory) GetParserCount() int {
	return len(f.parsers)
}