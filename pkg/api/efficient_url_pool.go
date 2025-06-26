package api

import (
	"strings"
	"sync/atomic"
)

// EfficientURLPool implements a lock-free, non-blocking URL pool
// Features: 复用, 无阻塞, 安全并发
type EfficientURLPool struct {
	urls    []string
	index   uint32   // Use uint32 to avoid overflow issues
	urlsLen uint32   // Cache length for performance
}

// NewEfficientURLPool creates a lock-free URL pool
func NewEfficientURLPool(urlString string) *EfficientURLPool {
	if urlString == "" {
		return &EfficientURLPool{
			urls:    []string{},
			urlsLen: 0,
		}
	}
	
	// Parse and clean URLs
	rawURLs := strings.Split(urlString, ",")
	urls := make([]string, 0, len(rawURLs))
	
	for _, url := range rawURLs {
		cleaned := strings.TrimSpace(url)
		if cleaned != "" {
			urls = append(urls, cleaned)
		}
	}
	
	return &EfficientURLPool{
		urls:    urls,
		index:   ^uint32(0), // Start at max value, so first increment gives 0
		urlsLen: uint32(len(urls)),
	}
}

// Next returns the next URL using lock-free round-robin
// 无阻塞 + 安全并发 + 复用
func (p *EfficientURLPool) Next() string {
	// Fast path for empty pool
	if p.urlsLen == 0 {
		return ""
	}
	
	// Fast path for single URL - 复用同一个URL，无原子操作开销
	if p.urlsLen == 1 {
		return p.urls[0]
	}
	
	// Lock-free round-robin using uint32 (no overflow risk)
	// 使用 uint32 自然溢出：4,294,967,295 + 1 = 0
	current := atomic.AddUint32(&p.index, 1)
	return p.urls[current%p.urlsLen]
}

// URLs returns all URLs (safe copy for debugging)
func (p *EfficientURLPool) URLs() []string {
	if p.urlsLen == 0 {
		return []string{}
	}
	result := make([]string, p.urlsLen)
	copy(result, p.urls)
	return result
}

// Size returns the number of URLs
func (p *EfficientURLPool) Size() int {
	return int(p.urlsLen)
}

// IsEmpty checks if pool is empty
func (p *EfficientURLPool) IsEmpty() bool {
	return p.urlsLen == 0
}