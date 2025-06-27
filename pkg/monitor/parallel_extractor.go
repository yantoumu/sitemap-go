package monitor

import (
	"context"
	"runtime"
	"sync"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/extractor"
)

// ParallelKeywordExtractor extracts keywords from URLs using all CPU cores
type ParallelKeywordExtractor struct {
	extractor *extractor.URLKeywordExtractor
	workers   int
}

// NewParallelKeywordExtractor creates a new parallel keyword extractor
func NewParallelKeywordExtractor() *ParallelKeywordExtractor {
	return &ParallelKeywordExtractor{
		extractor: extractor.NewURLKeywordExtractor(),
		workers:   runtime.NumCPU() * 2, // Use 2x CPU cores for optimal performance
	}
}

// ExtractFromURLs extracts keywords from multiple URLs in parallel
// This is a CPU-bound operation that should use all available cores
func (pke *ParallelKeywordExtractor) ExtractFromURLs(ctx context.Context, urls []parser.URL, primarySelector func([]string) string) ([]string, []string, int) {
	if len(urls) == 0 {
		return []string{}, []string{}, 0
	}

	// Pre-allocate result slices with estimated capacity
	estimatedResults := len(urls) / 5 // Assume 20% URLs have keywords
	
	// Result collector with pre-allocated capacity
	type result struct {
		keyword string
		url     string
	}
	
	// Create buffered channels for work distribution
	urlChan := make(chan parser.URL, 100)
	resultChan := make(chan *result, 100)
	
	// Start worker goroutines
	var wg sync.WaitGroup
	failedCount := 0
	var failedMu sync.Mutex
	
	// Start workers - use all CPU cores for this CPU-bound task
	for i := 0; i < pke.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for url := range urlChan {
				// Check context cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				// Extract keywords (this is the CPU-intensive part)
				urlKeywords, err := pke.extractor.Extract(url.Address)
				if err != nil {
					failedMu.Lock()
					failedCount++
					failedMu.Unlock()
					continue
				}
				
				if len(urlKeywords) > 0 {
					primaryKeyword := primarySelector(urlKeywords)
					resultChan <- &result{
						keyword: primaryKeyword,
						url:     url.Address,
					}
				}
			}
		}()
	}
	
	// Start result collector goroutine
	keywords := make([]string, 0, estimatedResults)
	urlList := make([]string, 0, estimatedResults)
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	
	go func() {
		defer collectorWg.Done()
		for r := range resultChan {
			keywords = append(keywords, r.keyword)
			urlList = append(urlList, r.url)
		}
	}()
	
	// Feed URLs to workers
	for _, url := range urls {
		select {
		case urlChan <- url:
		case <-ctx.Done():
			close(urlChan)
			wg.Wait()
			close(resultChan)
			collectorWg.Wait()
			return keywords, urlList, failedCount
		}
	}
	
	// Close input channel and wait for workers
	close(urlChan)
	wg.Wait()
	
	// Close result channel and wait for collector
	close(resultChan)
	collectorWg.Wait()
	
	return keywords, urlList, failedCount
}