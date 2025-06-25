package extractor

import (
	"net/url"
	"strings"
)

// SegmentType represents the type of a URL path segment
type SegmentType string

const (
	SegmentTypeGeneric  SegmentType = "generic"
	SegmentTypeCategory SegmentType = "category"
	SegmentTypeID       SegmentType = "id"
	SegmentTypeSlug     SegmentType = "slug"
	SegmentTypeDate     SegmentType = "date"
	SegmentTypeNumeric  SegmentType = "numeric"
	SegmentTypeFile     SegmentType = "file"
)

// PathSegment represents a single segment in a URL path
type PathSegment struct {
	Value     string                 `json:"value"`
	Position  int                    `json:"position"`
	Type      SegmentType            `json:"type"`
	Keywords  []string               `json:"keywords"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// PathAnalysis contains the complete analysis of a URL path
type PathAnalysis struct {
	Path      string                 `json:"path"`
	Segments  []PathSegment          `json:"segments"`
	Keywords  []string               `json:"keywords"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// EnhancedKeywordExtractor extends URLKeywordExtractor with path analysis
type EnhancedKeywordExtractor struct {
	*URLKeywordExtractor
	pathAnalyzer *PathAnalyzer
	enablePath   bool
}

// NewEnhancedKeywordExtractor creates a new enhanced keyword extractor
func NewEnhancedKeywordExtractor() *EnhancedKeywordExtractor {
	return &EnhancedKeywordExtractor{
		URLKeywordExtractor: NewURLKeywordExtractor(),
		pathAnalyzer:        NewPathAnalyzer(),
		enablePath:          true,
	}
}

// SetPathAnalysisEnabled enables or disables path analysis
func (e *EnhancedKeywordExtractor) SetPathAnalysisEnabled(enabled bool) {
	e.enablePath = enabled
}

// ExtractWithAnalysis extracts keywords and provides detailed path analysis
func (e *EnhancedKeywordExtractor) ExtractWithAnalysis(urlStr string) (*ExtractedKeywords, error) {
	// First get basic keywords
	basicKeywords, err := e.URLKeywordExtractor.Extract(urlStr)
	if err != nil {
		return nil, err
	}
	
	result := &ExtractedKeywords{
		URL:      urlStr,
		Keywords: basicKeywords,
		Metadata: make(map[string]interface{}),
	}
	
	// Add path analysis if enabled
	if e.enablePath {
		parsedURL, err := parseURL(urlStr)
		if err == nil {
			pathAnalysis := e.pathAnalyzer.AnalyzePath(parsedURL.Path)
			
			// Merge path keywords with basic keywords
			allKeywords := append(basicKeywords, pathAnalysis.Keywords...)
			result.Keywords = e.removeDuplicateKeywords(allKeywords)
			
			// Add analysis metadata
			result.PathAnalysis = pathAnalysis
			result.Metadata["has_path_analysis"] = true
			result.Metadata["path_depth"] = len(pathAnalysis.Segments)
			result.Metadata["path_pattern"] = pathAnalysis.Metadata["path_pattern"]
		}
	}
	
	return result, nil
}

// GetPathAnalyzer returns the internal path analyzer
func (e *EnhancedKeywordExtractor) GetPathAnalyzer() *PathAnalyzer {
	return e.pathAnalyzer
}

// ExtractedKeywords represents the result of keyword extraction with analysis
type ExtractedKeywords struct {
	URL          string                 `json:"url"`
	Keywords     []string               `json:"keywords"`
	PathAnalysis *PathAnalysis          `json:"path_analysis,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
}

func (e *EnhancedKeywordExtractor) removeDuplicateKeywords(keywords []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	
	for _, keyword := range keywords {
		normalized := strings.ToLower(keyword)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, keyword)
		}
	}
	
	return result
}

// parseURL is a helper function to parse URLs
func parseURL(urlStr string) (*url.URL, error) {
	return url.Parse(urlStr)
}