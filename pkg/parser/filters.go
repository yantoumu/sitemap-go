package parser

import (
	"net/url"
	"strings"
)

// PathFilter excludes URLs based on path patterns
type PathFilter struct {
	excludePaths []string
	name         string
}

func NewPathFilter(name string, excludePaths []string) *PathFilter {
	return &PathFilter{
		name:         name,
		excludePaths: excludePaths,
	}
}

func (f *PathFilter) ShouldExclude(u *url.URL) bool {
	path := strings.ToLower(u.Path)
	for _, excludePath := range f.excludePaths {
		if strings.Contains(path, strings.ToLower(excludePath)) {
			return true
		}
	}
	return false
}

func (f *PathFilter) Name() string {
	return f.name
}

// ExtensionFilter excludes URLs based on file extensions
type ExtensionFilter struct {
	excludeExts []string
	name        string
}

func NewExtensionFilter(name string, excludeExts []string) *ExtensionFilter {
	return &ExtensionFilter{
		name:        name,
		excludeExts: excludeExts,
	}
}

func (f *ExtensionFilter) ShouldExclude(u *url.URL) bool {
	path := strings.ToLower(u.Path)
	for _, ext := range f.excludeExts {
		if strings.HasSuffix(path, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

func (f *ExtensionFilter) Name() string {
	return f.name
}