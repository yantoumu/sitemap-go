package api

import (
	"context"
	"strings"
	"sync"
)

// SimpleURLPool uses channel-based pool pattern for load balancing
// Much simpler than atomic counter approach, naturally handles overflow
type SimpleURLPool struct {
	urlChan chan string
	urls    []string
	mu      sync.RWMutex
	closed  bool
	ctx     context.Context
	cancel  context.CancelFunc
	once    sync.Once // Ensure fillChannel starts only once
}

// NewSimpleURLPool creates a simple channel-based URL pool
func NewSimpleURLPool(urlString string) *SimpleURLPool {
	if urlString == "" {
		ctx, cancel := context.WithCancel(context.Background())
		return &SimpleURLPool{
			urlChan: make(chan string, 1),
			urls:    []string{},
			ctx:     ctx,
			cancel:  cancel,
		}
	}
	
	// Parse URLs
	rawURLs := strings.Split(urlString, ",")
	urls := make([]string, 0, len(rawURLs))
	
	for _, url := range rawURLs {
		cleaned := strings.TrimSpace(url)
		if cleaned != "" {
			urls = append(urls, cleaned)
		}
	}
	
	if len(urls) == 0 {
		ctx, cancel := context.WithCancel(context.Background())
		return &SimpleURLPool{
			urlChan: make(chan string, 1),
			urls:    []string{},
			ctx:     ctx,
			cancel:  cancel,
		}
	}

	// Create buffered channel for round-robin
	ctx, cancel := context.WithCancel(context.Background())
	pool := &SimpleURLPool{
		urlChan: make(chan string, len(urls)*2), // Buffer for smooth operation
		urls:    urls,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Fill the channel with URLs in round-robin fashion
	pool.once.Do(func() {
		go pool.fillChannel()
	})

	return pool
}

// fillChannel continuously fills the channel with URLs in round-robin order
func (p *SimpleURLPool) fillChannel() {
	defer func() {
		if r := recover(); r != nil {
			// Gracefully handle any panic in fillChannel
			return
		}
	}()

	if len(p.urls) == 0 {
		return
	}

	index := 0
	for {
		// Check context cancellation first
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		p.mu.RLock()
		if p.closed || len(p.urls) == 0 {
			p.mu.RUnlock()
			return
		}

		url := p.urls[index]
		index = (index + 1) % len(p.urls) // Simple modulo, no overflow risk
		p.mu.RUnlock()

		select {
		case p.urlChan <- url:
			// URL sent successfully
		case <-p.ctx.Done():
			return
		}
	}
}

// Next returns the next URL using channel-based round-robin
// Thread-safe and naturally load-balanced
func (p *SimpleURLPool) Next() string {
	if len(p.urls) == 0 {
		return ""
	}
	
	// Fast path for single URL
	if len(p.urls) == 1 {
		return p.urls[0]
	}
	
	// Get URL from channel (blocking, but very fast)
	select {
	case url := <-p.urlChan:
		return url
	default:
		// Fallback if channel is temporarily empty
		p.mu.RLock()
		defer p.mu.RUnlock()
		if len(p.urls) > 0 {
			return p.urls[0] // Return first URL as fallback
		}
		return ""
	}
}

// URLs returns all URLs in the pool
func (p *SimpleURLPool) URLs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]string, len(p.urls))
	copy(result, p.urls)
	return result
}

// Size returns the number of URLs in the pool
func (p *SimpleURLPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.urls)
}

// IsEmpty checks if the pool has no URLs
func (p *SimpleURLPool) IsEmpty() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.urls) == 0
}

// Close stops the background goroutine safely
func (p *SimpleURLPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return // Already closed
	}

	p.closed = true

	// Cancel context to stop fillChannel goroutine
	if p.cancel != nil {
		p.cancel()
	}

	// Close channel safely
	select {
	case <-p.urlChan:
		// Drain any remaining items
	default:
	}
	close(p.urlChan)
}