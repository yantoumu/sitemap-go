package extractor

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	// Common stop words to filter out
	stopWords = map[string]bool{
		"the": true, "and": true, "or": true, "in": true, "on": true,
		"at": true, "to": true, "for": true, "of": true, "with": true,
		"by": true, "from": true, "a": true, "an": true, "as": true,
		"is": true, "was": true, "are": true, "were": true, "been": true,
	}
	
	// Regex patterns for cleaning
	nonAlphaNumeric = regexp.MustCompile(`[^a-zA-Z0-9\s\-_]`)
	multiSpace      = regexp.MustCompile(`\s+`)
	multiDash       = regexp.MustCompile(`-+`)
)

type URLKeywordExtractor struct {
	filters        []Filter
	minWordLength  int
	maxWordLength  int
}

func NewURLKeywordExtractor() *URLKeywordExtractor {
	return &URLKeywordExtractor{
		filters:       make([]Filter, 0),
		minWordLength: 3,
		maxWordLength: 50,
	}
}

func (e *URLKeywordExtractor) Extract(urlStr string) ([]string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// Extract keywords from path
	pathKeywords := e.extractFromPath(parsedURL.Path)
	
	// Extract keywords from query parameters
	queryKeywords := e.extractFromQuery(parsedURL.Query())
	
	// Combine and deduplicate
	keywordMap := make(map[string]bool)
	for _, kw := range pathKeywords {
		keywordMap[kw] = true
	}
	for _, kw := range queryKeywords {
		keywordMap[kw] = true
	}
	
	// Convert map to slice
	keywords := make([]string, 0, len(keywordMap))
	for kw := range keywordMap {
		keywords = append(keywords, kw)
	}
	
	// Apply filters
	for _, filter := range e.filters {
		keywords = filter.Apply(keywords)
	}
	
	return keywords, nil
}

func (e *URLKeywordExtractor) SetFilters(filters []Filter) {
	e.filters = filters
}

func (e *URLKeywordExtractor) Normalize(keyword string) string {
	// Convert to lowercase
	normalized := strings.ToLower(keyword)
	
	// Remove non-alphanumeric characters (except spaces and dashes)
	normalized = nonAlphaNumeric.ReplaceAllString(normalized, " ")
	
	// Replace multiple spaces with single space
	normalized = multiSpace.ReplaceAllString(normalized, " ")
	
	// Replace multiple dashes with single dash
	normalized = multiDash.ReplaceAllString(normalized, "-")
	
	// Trim spaces and dashes
	normalized = strings.Trim(normalized, " -")
	
	return normalized
}

func (e *URLKeywordExtractor) extractFromPath(path string) []string {
	// Remove file extension
	if idx := strings.LastIndex(path, "."); idx > 0 {
		path = path[:idx]
	}
	
	// Split by common separators
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.'
	})
	
	keywords := make([]string, 0)
	for _, part := range parts {
		// Normalize the part
		normalized := e.Normalize(part)
		
		// Skip empty, too short, too long, or stop words
		if normalized == "" || 
		   len(normalized) < e.minWordLength || 
		   len(normalized) > e.maxWordLength ||
		   stopWords[normalized] {
			continue
		}
		
		// Split camelCase and PascalCase
		subWords := e.splitCamelCase(normalized)
		for _, word := range subWords {
			if len(word) >= e.minWordLength && !stopWords[word] {
				keywords = append(keywords, word)
			}
		}
	}
	
	return keywords
}

func (e *URLKeywordExtractor) extractFromQuery(params url.Values) []string {
	keywords := make([]string, 0)
	
	// Common parameter names that might contain keywords
	keywordParams := []string{"q", "query", "search", "keyword", "tag", "category", "title"}
	
	for _, param := range keywordParams {
		if values, exists := params[param]; exists {
			for _, value := range values {
				// Split by common separators
				words := strings.FieldsFunc(value, func(r rune) bool {
					return r == ' ' || r == ',' || r == '+' || r == '|'
				})
				
				for _, word := range words {
					normalized := e.Normalize(word)
					if normalized != "" && 
					   len(normalized) >= e.minWordLength && 
					   len(normalized) <= e.maxWordLength &&
					   !stopWords[normalized] {
						keywords = append(keywords, normalized)
					}
				}
			}
		}
	}
	
	return keywords
}

func (e *URLKeywordExtractor) splitCamelCase(word string) []string {
	var result []string
	var current strings.Builder
	
	for i, r := range word {
		if i > 0 && 'A' <= r && r <= 'Z' {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	
	return result
}