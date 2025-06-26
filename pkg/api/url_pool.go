package api

import (
	"strings"
	"sync/atomic"
)

// URLPool provides thread-safe round-robin load balancing for multiple URLs
// Implements Strategy Pattern for URL selection with O(1) complexity
type URLPool struct {
	urls    []string
	current int64
}

// NewURLPool creates a URL pool from comma-separated URLs
// Backward compatible: single URL works without overhead
func NewURLPool(urlString string) *URLPool {
	if urlString == "" {
		return &URLPool{urls: []string{}}
	}
	
	// Split by comma and clean up whitespace
	rawURLs := strings.Split(urlString, ",")
	urls := make([]string, 0, len(rawURLs))
	
	for _, url := range rawURLs {
		cleaned := strings.TrimSpace(url)
		if cleaned != "" {
			urls = append(urls, cleaned)
		}
	}
	
	return &URLPool{
		urls:    urls,
		current: -1, // Start at -1 so first call returns index 0
	}
}

// Next returns the next URL using round-robin algorithm
// Thread-safe with atomic operations and overflow protection
func (p *URLPool) Next() string {
	if len(p.urls) == 0 {
		return ""
	}
	
	// Fast path for single URL - no atomic operations needed
	if len(p.urls) == 1 {
		return p.urls[0]
	}
	
	// Round-robin with atomic increment and safe modulo for overflow protection
	next := atomic.AddInt64(&p.current, 1)
	// Safe modulo: handles negative numbers from integer overflow
	// Formula: ((n % m) + m) % m ensures positive result
	urlsLen := int64(len(p.urls))
	index := ((next % urlsLen) + urlsLen) % urlsLen
	return p.urls[index]
}

// URLs returns all URLs in the pool (for debugging/monitoring)
func (p *URLPool) URLs() []string {
	result := make([]string, len(p.urls))
	copy(result, p.urls)
	return result
}

// Size returns the number of URLs in the pool
func (p *URLPool) Size() int {
	return len(p.urls)
}

// IsEmpty checks if the pool has no URLs
func (p *URLPool) IsEmpty() bool {
	return len(p.urls) == 0
}