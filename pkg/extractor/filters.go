package extractor

import "strings"

// LengthFilter filters keywords by length
type LengthFilter struct {
	minLength int
	maxLength int
	name      string
}

func NewLengthFilter(name string, minLength, maxLength int) *LengthFilter {
	return &LengthFilter{
		name:      name,
		minLength: minLength,
		maxLength: maxLength,
	}
}

func (f *LengthFilter) Apply(keywords []string) []string {
	filtered := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		if len(kw) >= f.minLength && len(kw) <= f.maxLength {
			filtered = append(filtered, kw)
		}
	}
	return filtered
}

func (f *LengthFilter) Name() string {
	return f.name
}

// StopWordFilter removes common stop words
type StopWordFilter struct {
	stopWords map[string]bool
	name      string
}

func NewStopWordFilter(name string, stopWords []string) *StopWordFilter {
	swMap := make(map[string]bool)
	for _, word := range stopWords {
		swMap[strings.ToLower(word)] = true
	}
	return &StopWordFilter{
		name:      name,
		stopWords: swMap,
	}
}

func (f *StopWordFilter) Apply(keywords []string) []string {
	filtered := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		if !f.stopWords[strings.ToLower(kw)] {
			filtered = append(filtered, kw)
		}
	}
	return filtered
}

func (f *StopWordFilter) Name() string {
	return f.name
}

// DuplicateFilter removes duplicate keywords
type DuplicateFilter struct {
	name string
}

func NewDuplicateFilter(name string) *DuplicateFilter {
	return &DuplicateFilter{name: name}
}

func (f *DuplicateFilter) Apply(keywords []string) []string {
	seen := make(map[string]bool)
	filtered := make([]string, 0, len(keywords))
	
	for _, kw := range keywords {
		normalized := strings.ToLower(kw)
		if !seen[normalized] {
			seen[normalized] = true
			filtered = append(filtered, kw)
		}
	}
	
	return filtered
}

func (f *DuplicateFilter) Name() string {
	return f.name
}