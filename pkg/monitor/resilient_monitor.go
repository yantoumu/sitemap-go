package monitor

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"sitemap-go/pkg/api"
	"sitemap-go/pkg/extractor"
	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/storage"
)

// ResilientSitemapMonitor provides enhanced error recovery for sitemap monitoring
type ResilientSitemapMonitor struct {
	parserFactory    *parser.ResilientParserFactory
	keywordExtractor extractor.KeywordExtractor
	apiClient        api.APIClient
	storage          storage.Storage
	log              *logger.Logger
	config           MonitorConfig
	errorHistory     map[string][]error // Track errors per sitemap for intelligent retry
}

// NewResilientSitemapMonitor creates a monitor with enhanced error handling
func NewResilientSitemapMonitor(config MonitorConfig) (*ResilientSitemapMonitor, error) {
	// Initialize resilient parser factory
	parserFactory := parser.NewResilientParserFactory()
	
	// Initialize keyword extractor
	keywordExtractor := extractor.NewURLKeywordExtractor()
	
	// Create API client
	trendAPIClient := api.NewHTTPAPIClient(config.TrendAPIBaseURL, "")
	
	// Create storage service
	storageConfig := storage.StorageConfig{
		DataDir:     "./data",
		CacheSize:   100,
		EncryptData: true,
	}
	encryptionKey := config.EncryptionKey
	if encryptionKey == "" {
		encryptionKey = "default-sitemap-monitor-key"
	}
	storageService, err := storage.NewEncryptedFileStorage(storageConfig, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage service: %w", err)
	}
	
	return &ResilientSitemapMonitor{
		parserFactory:    parserFactory,
		keywordExtractor: keywordExtractor,
		apiClient:        trendAPIClient,
		storage:          storageService,
		log:              logger.GetLogger().WithField("component", "resilient_sitemap_monitor"),
		config:           config,
		errorHistory:     make(map[string][]error),
	}, nil
}

// ProcessSitemap handles a single sitemap with resilient error recovery
func (rm *ResilientSitemapMonitor) ProcessSitemap(ctx context.Context, sitemapURL string) (*MonitorResult, error) {
	rm.log.WithField("sitemap_url", sitemapURL).Info("Processing sitemap with resilient strategies")
	
	result := &MonitorResult{
		SitemapURL: sitemapURL,
		Timestamp:  time.Now(),
		Metadata:   make(map[string]interface{}),
	}
	
	// Get error history for this sitemap
	errorHistory := rm.errorHistory[sitemapURL]
	
	// Use resilient parser factory to parse sitemap
	urls, err := rm.parserFactory.Parse(ctx, sitemapURL)
	if err != nil {
		// Track error for future intelligent retry
		rm.errorHistory[sitemapURL] = append(errorHistory, err)
		
		result.Success = false
		result.Error = err.Error()
		rm.log.WithError(err).WithField("sitemap_url", sitemapURL).Error("Failed to parse sitemap")
		return result, err
	}
	
	// Extract keywords using enhanced extractor
	var keywords []string
	keywordMap := make(map[string]bool)
	
	for _, parsedURL := range urls {
		// First try to extract from the URL itself
		urlKeywords, err := rm.keywordExtractor.Extract(parsedURL.Address)
		if err != nil {
			rm.log.WithError(err).WithField("url", parsedURL.Address).Debug("Failed to extract keywords")
			continue
		}
		
		// Select primary keyword (longest) if multiple found
		if len(urlKeywords) > 0 {
			primaryKeyword := rm.selectPrimaryKeyword(urlKeywords)
			if !keywordMap[primaryKeyword] {
				keywordMap[primaryKeyword] = true
				keywords = append(keywords, primaryKeyword)
			}
		}
	}
	
	result.Keywords = keywords
	result.Metadata["url_count"] = len(urls)
	result.Metadata["keyword_count"] = len(keywords)
	result.Metadata["extraction_ratio"] = float64(len(keywords)) / float64(len(urls))
	
	// Query trend data (with error tolerance)
	if len(keywords) > 0 {
		trendData, err := rm.apiClient.Query(ctx, keywords)
		if err != nil {
			rm.log.WithError(err).Warn("Failed to query trend data, continuing without trends")
			result.Metadata["trend_error"] = err.Error()
		} else {
			result.TrendData = trendData
		}
	}
	
	// Store results
	if err := rm.storeResults(sitemapURL, result); err != nil {
		rm.log.WithError(err).Warn("Failed to store results")
		result.Metadata["storage_error"] = err.Error()
	}
	
	result.Success = true
	rm.log.WithFields(map[string]interface{}{
		"sitemap_url":    sitemapURL,
		"urls_found":     len(urls),
		"keywords_found": len(keywords),
		"success":        true,
	}).Info("Sitemap processing completed")
	
	return result, nil
}

// selectPrimaryKeyword selects the most relevant keyword
func (rm *ResilientSitemapMonitor) selectPrimaryKeyword(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	
	// Select the longest keyword as it's likely most specific
	longest := keywords[0]
	for _, kw := range keywords[1:] {
		if len(kw) > len(longest) {
			longest = kw
		}
	}
	
	return longest
}

// storeResults saves the monitoring results
func (rm *ResilientSitemapMonitor) storeResults(sitemapURL string, result *MonitorResult) error {
	key := rm.generateStorageKey(sitemapURL)
	return rm.storage.Save(context.Background(), key, result)
}

// generateStorageKey creates a storage key for a sitemap URL
func (rm *ResilientSitemapMonitor) generateStorageKey(sitemapURL string) string {
	// Parse URL to extract domain
	parsedURL, err := url.Parse(sitemapURL)
	if err != nil {
		return fmt.Sprintf("sitemap_%s_%d", strings.ReplaceAll(sitemapURL, "/", "_"), time.Now().Unix())
	}
	
	domain := strings.ReplaceAll(parsedURL.Host, ".", "_")
	path := strings.ReplaceAll(parsedURL.Path, "/", "_")
	
	return fmt.Sprintf("sitemap_%s%s_%d", domain, path, time.Now().Unix())
}

// GetErrorHistory returns the error history for a specific sitemap
func (rm *ResilientSitemapMonitor) GetErrorHistory(sitemapURL string) []error {
	return rm.errorHistory[sitemapURL]
}

// ClearErrorHistory clears the error history for a sitemap
func (rm *ResilientSitemapMonitor) ClearErrorHistory(sitemapURL string) {
	delete(rm.errorHistory, sitemapURL)
}

// BatchProcessSitemaps processes multiple sitemaps concurrently with resilient strategies
func (rm *ResilientSitemapMonitor) BatchProcessSitemaps(ctx context.Context, sitemapURLs []string, concurrency int) ([]*MonitorResult, error) {
	if concurrency <= 0 {
		concurrency = 8 // Default to 8 concurrent workers
	}
	
	rm.log.WithFields(map[string]interface{}{
		"sitemap_count": len(sitemapURLs),
		"concurrency":   concurrency,
	}).Info("Starting batch sitemap processing")
	
	// Create channels for work distribution
	workChan := make(chan string, len(sitemapURLs))
	resultChan := make(chan *MonitorResult, len(sitemapURLs))
	
	// Start workers
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	for i := 0; i < concurrency; i++ {
		go rm.sitemapWorker(workerCtx, i, workChan, resultChan)
	}
	
	// Send work
	for _, sitemapURL := range sitemapURLs {
		workChan <- sitemapURL
	}
	close(workChan)
	
	// Collect results
	results := make([]*MonitorResult, 0, len(sitemapURLs))
	for i := 0; i < len(sitemapURLs); i++ {
		result := <-resultChan
		results = append(results, result)
	}
	
	// Log summary
	successful := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			successful++
		} else {
			failed++
		}
	}
	
	rm.log.WithFields(map[string]interface{}{
		"total":      len(results),
		"successful": successful,
		"failed":     failed,
	}).Info("Batch processing completed")
	
	return results, nil
}

// sitemapWorker is a worker goroutine for processing sitemaps
func (rm *ResilientSitemapMonitor) sitemapWorker(ctx context.Context, id int, workChan <-chan string, resultChan chan<- *MonitorResult) {
	rm.log.WithField("worker_id", id).Debug("Worker started")
	
	for sitemapURL := range workChan {
		select {
		case <-ctx.Done():
			rm.log.WithField("worker_id", id).Debug("Worker cancelled")
			return
		default:
			result, err := rm.ProcessSitemap(ctx, sitemapURL)
			if err != nil {
				// Result already contains error information
				rm.log.WithError(err).WithFields(map[string]interface{}{
					"worker_id":    id,
					"sitemap_url": sitemapURL,
				}).Debug("Worker encountered error")
			}
			resultChan <- result
		}
	}
	
	rm.log.WithField("worker_id", id).Debug("Worker finished")
}