package monitor

import (
	"context"
	"runtime"
	"sync"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/extractor"
)

// stringSlicePool provides reusable string slices to reduce allocations
var stringSlicePool = sync.Pool{
	New: func() interface{} {
		return make([]string, 0, 32) // Increased initial capacity based on typical usage
	},
}

// getStringSlice gets a reusable string slice from pool
func getStringSlice() []string {
	slice := stringSlicePool.Get().([]string)
	return slice[:0] // Ensure slice is empty but retains capacity
}

// putStringSlice returns a string slice to pool after clearing it
func putStringSlice(s []string) {
	// Adaptive pooling: only pool slices within reasonable size range
	if cap(s) < 8 || cap(s) > 128 { // Don't pool too small or too large slices
		return
	}
	s = s[:0] // Clear slice but keep capacity
	stringSlicePool.Put(s)
}

// ParallelKeywordExtractor extracts keywords from URLs using optimized concurrency
type ParallelKeywordExtractor struct {
	extractor *extractor.URLKeywordExtractor
	workers   int
}

// NewParallelKeywordExtractor creates a new parallel keyword extractor with configurable workers
func NewParallelKeywordExtractor() *ParallelKeywordExtractor {
	// Use CPU cores count for CPU-bound keyword extraction, not 2x
	workers := runtime.NumCPU()
	if workers > 8 { // Cap at 8 to prevent excessive goroutines
		workers = 8
	}
	return &ParallelKeywordExtractor{
		extractor: extractor.NewURLKeywordExtractor(),
		workers:   workers,
	}
}

// NewParallelKeywordExtractorWithWorkers creates extractor with specific worker count
func NewParallelKeywordExtractorWithWorkers(workers int) *ParallelKeywordExtractor {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > 16 { // Reasonable upper limit
		workers = 16
	}
	return &ParallelKeywordExtractor{
		extractor: extractor.NewURLKeywordExtractor(),
		workers:   workers,
	}
}

// ExtractFromURLs extracts keywords from multiple URLs in parallel with optimized memory usage
// This is a CPU-bound operation optimized for memory efficiency and controlled concurrency
func (pke *ParallelKeywordExtractor) ExtractFromURLs(ctx context.Context, urls []parser.URL, primarySelector func([]string) string) ([]string, []string, int) {
	if len(urls) == 0 {
		return []string{}, []string{}, 0
	}

	// Pre-allocate result slices with better capacity estimation
	estimatedResults := len(urls) / 4 // Assume 25% URLs have keywords (more realistic)
	if estimatedResults < 10 {
		estimatedResults = 10
	}

	// Result collector with pre-allocated capacity
	type result struct {
		keyword string
		url     string
	}

	// Create buffered channels with backpressure control
	// Smaller buffer to prevent excessive memory usage
	bufferSize := pke.workers * 2
	if bufferSize > 50 {
		bufferSize = 50
	}
	urlChan := make(chan parser.URL, bufferSize)
	resultChan := make(chan *result, bufferSize)

	// Start worker goroutines with optimized processing
	var wg sync.WaitGroup
	var failedCount int64

	// Start workers - use configured worker count for CPU-bound task
	for i := 0; i < pke.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for url := range urlChan {
				// Check context cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Extract keywords with memory pool optimization
				urlKeywords := getStringSlice() // Get reusable slice
				extractedKeywords, err := pke.extractor.Extract(url.Address)
				if err != nil {
					putStringSlice(urlKeywords) // Return slice to pool
					failedCount++
					continue
				}

				// Copy extracted keywords to pooled slice
				urlKeywords = append(urlKeywords, extractedKeywords...)

				if len(urlKeywords) > 0 {
					primaryKeyword := primarySelector(urlKeywords)
					if primaryKeyword != "" {
						resultChan <- &result{
							keyword: primaryKeyword,
							url:     url.Address,
						}
					}
				}

				// Return slice to pool
				putStringSlice(urlKeywords)
			}
		}(i)
	}

	// Start result collector goroutine with optimized memory allocation
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

	// Feed URLs to workers with controlled rate to prevent memory spikes
	go func() {
		defer close(urlChan)
		for _, url := range urls {
			select {
			case urlChan <- url:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()

	// Close result channel and wait for collector
	close(resultChan)
	collectorWg.Wait()

	return keywords, urlList, int(failedCount)
}

// GetWorkerCount returns the configured number of workers
func (pke *ParallelKeywordExtractor) GetWorkerCount() int {
	return pke.workers
}

// SetWorkerCount updates the worker count (for dynamic adjustment)
func (pke *ParallelKeywordExtractor) SetWorkerCount(workers int) {
	if workers > 0 && workers <= 16 {
		pke.workers = workers
	}
}