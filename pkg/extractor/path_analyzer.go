package extractor

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"sitemap-go/pkg/logger"
)

var (
	// Enhanced patterns for URL path analysis
	numberPattern    = regexp.MustCompile(`\d+`)
	datePattern      = regexp.MustCompile(`\d{4}[-/]\d{1,2}[-/]\d{1,2}`)
	slugPattern      = regexp.MustCompile(`[a-z]+-[a-z-]+`)
	camelCasePattern = regexp.MustCompile(`[a-z][A-Z]`)
	
	// Industry-specific keywords
	techKeywords = map[string]bool{
		"api": true, "app": true, "dev": true, "tech": true, "code": true,
		"software": true, "programming": true, "database": true, "server": true,
	}
	
	businessKeywords = map[string]bool{
		"product": true, "service": true, "business": true, "company": true,
		"enterprise": true, "solution": true, "strategy": true, "marketing": true,
	}
	
	contentKeywords = map[string]bool{
		"blog": true, "news": true, "article": true, "post": true, "content": true,
		"story": true, "guide": true, "tutorial": true, "review": true,
	}
)

type PathAnalyzer struct {
	log               *logger.Logger
	enableSemantic    bool
	enableIndustry    bool
	enableHierarchy   bool
	maxDepth          int
	minWordLength     int
}

func NewPathAnalyzer() *PathAnalyzer {
	return &PathAnalyzer{
		log:               logger.GetLogger().WithField("component", "path_analyzer"),
		enableSemantic:    true,
		enableIndustry:    true,
		enableHierarchy:   true,
		maxDepth:          10,
		minWordLength:     3,
	}
}

func (pa *PathAnalyzer) SetOptions(enableSemantic, enableIndustry, enableHierarchy bool) {
	pa.enableSemantic = enableSemantic
	pa.enableIndustry = enableIndustry
	pa.enableHierarchy = enableHierarchy
}

func (pa *PathAnalyzer) AnalyzePath(path string) *PathAnalysis {
	pa.log.WithField("path", path).Debug("Analyzing URL path")
	
	analysis := &PathAnalysis{
		Path:       path,
		Segments:   make([]PathSegment, 0),
		Keywords:   make([]string, 0),
		Metadata:   make(map[string]interface{}),
	}
	
	// Split path into segments
	segments := strings.Split(strings.Trim(path, "/"), "/")
	
	// Analyze each segment
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		
		pathSegment := pa.analyzeSegment(segment, i)
		analysis.Segments = append(analysis.Segments, pathSegment)
		
		// Collect keywords from segment
		analysis.Keywords = append(analysis.Keywords, pathSegment.Keywords...)
	}
	
	// Add hierarchical analysis
	if pa.enableHierarchy {
		pa.addHierarchicalAnalysis(analysis)
	}
	
	// Add semantic analysis
	if pa.enableSemantic {
		pa.addSemanticAnalysis(analysis)
	}
	
	// Add industry classification
	if pa.enableIndustry {
		pa.addIndustryClassification(analysis)
	}
	
	pa.log.WithFields(map[string]interface{}{
		"path":         path,
		"segments":     len(analysis.Segments),
		"keywords":     len(analysis.Keywords),
		"path_type":    analysis.Metadata["path_type"],
	}).Debug("Path analysis completed")
	
	return analysis
}

func (pa *PathAnalyzer) analyzeSegment(segment string, position int) PathSegment {
	pathSegment := PathSegment{
		Value:     segment,
		Position:  position,
		Keywords:  make([]string, 0),
		Type:      pa.classifySegmentType(segment),
		Metadata:  make(map[string]interface{}),
	}
	
	// Extract keywords from segment
	pathSegment.Keywords = pa.extractSegmentKeywords(segment)
	
	// Add position-based metadata
	pathSegment.Metadata["depth"] = position
	pathSegment.Metadata["is_leaf"] = false // Will be updated by caller if needed
	
	return pathSegment
}

func (pa *PathAnalyzer) classifySegmentType(segment string) SegmentType {
	segment = strings.ToLower(segment)
	
	// Check for numeric patterns
	if numberPattern.MatchString(segment) {
		if len(segment) <= 10 && pa.isNumeric(segment) {
			return SegmentTypeID
		}
		if datePattern.MatchString(segment) {
			return SegmentTypeDate
		}
		return SegmentTypeNumeric
	}
	
	// Check for file extensions
	if strings.Contains(segment, ".") {
		return SegmentTypeFile
	}
	
	// Check for slug patterns
	if slugPattern.MatchString(segment) {
		return SegmentTypeSlug
	}
	
	// Check for category patterns
	if pa.isCategoryLike(segment) {
		return SegmentTypeCategory
	}
	
	return SegmentTypeGeneric
}

func (pa *PathAnalyzer) extractSegmentKeywords(segment string) []string {
	keywords := make([]string, 0)
	
	// Remove file extension
	if idx := strings.LastIndex(segment, "."); idx > 0 {
		segment = segment[:idx]
	}
	
	// Split by various separators
	parts := strings.FieldsFunc(segment, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || unicode.IsSpace(r)
	})
	
	for _, part := range parts {
		// Normalize and validate
		normalized := strings.ToLower(strings.TrimSpace(part))
		if len(normalized) < pa.minWordLength || pa.isStopWord(normalized) {
			continue
		}
		
		// Split camelCase
		subWords := pa.splitCamelCase(normalized)
		for _, word := range subWords {
			if len(word) >= pa.minWordLength && !pa.isStopWord(word) {
				keywords = append(keywords, word)
			}
		}
	}
	
	return pa.removeDuplicates(keywords)
}

func (pa *PathAnalyzer) addHierarchicalAnalysis(analysis *PathAnalysis) {
	// Mark leaf segment
	if len(analysis.Segments) > 0 {
		lastIdx := len(analysis.Segments) - 1
		analysis.Segments[lastIdx].Metadata["is_leaf"] = true
	}
	
	// Determine path pattern
	pathPattern := pa.determinePathPattern(analysis.Segments)
	analysis.Metadata["path_pattern"] = pathPattern
	analysis.Metadata["depth"] = len(analysis.Segments)
}

func (pa *PathAnalyzer) addSemanticAnalysis(analysis *PathAnalysis) {
	// Analyze semantic relationships between segments
	relationships := make([]string, 0)
	
	for i := 1; i < len(analysis.Segments); i++ {
		prev := analysis.Segments[i-1]
		curr := analysis.Segments[i]
		
		relationship := pa.determineRelationship(prev, curr)
		if relationship != "" {
			relationships = append(relationships, relationship)
		}
	}
	
	analysis.Metadata["relationships"] = relationships
}

func (pa *PathAnalyzer) addIndustryClassification(analysis *PathAnalysis) {
	industries := make([]string, 0)
	
	// Check keywords against industry patterns
	for _, keyword := range analysis.Keywords {
		if techKeywords[keyword] {
			industries = append(industries, "technology")
		}
		if businessKeywords[keyword] {
			industries = append(industries, "business")
		}
		if contentKeywords[keyword] {
			industries = append(industries, "content")
		}
	}
	
	analysis.Metadata["industries"] = pa.removeDuplicates(industries)
}

func (pa *PathAnalyzer) determinePathPattern(segments []PathSegment) string {
	if len(segments) == 0 {
		return "empty"
	}
	
	pattern := make([]string, len(segments))
	for i, segment := range segments {
		pattern[i] = string(segment.Type)
	}
	
	return strings.Join(pattern, "/")
}

func (pa *PathAnalyzer) determineRelationship(prev, curr PathSegment) string {
	if prev.Type == SegmentTypeCategory && curr.Type == SegmentTypeID {
		return "category_item"
	}
	if prev.Type == SegmentTypeDate && curr.Type == SegmentTypeSlug {
		return "dated_content"
	}
	if prev.Type == SegmentTypeGeneric && curr.Type == SegmentTypeFile {
		return "resource_file"
	}
	return ""
}

func (pa *PathAnalyzer) isCategoryLike(segment string) bool {
	categoryPatterns := []string{
		"category", "categories", "tag", "tags", "type", "types",
		"section", "sections", "group", "groups",
	}
	
	for _, pattern := range categoryPatterns {
		if strings.Contains(segment, pattern) {
			return true
		}
	}
	return false
}

func (pa *PathAnalyzer) isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func (pa *PathAnalyzer) isStopWord(word string) bool {
	return stopWords[word]
}

func (pa *PathAnalyzer) splitCamelCase(word string) []string {
	if !camelCasePattern.MatchString(word) {
		return []string{word}
	}
	
	var result []string
	var current strings.Builder
	
	for i, r := range word {
		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(rune(word[i-1])) {
			if current.Len() > 0 {
				result = append(result, strings.ToLower(current.String()))
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	
	if current.Len() > 0 {
		result = append(result, strings.ToLower(current.String()))
	}
	
	return result
}

func (pa *PathAnalyzer) removeDuplicates(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}