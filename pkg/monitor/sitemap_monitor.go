package monitor

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"sitemap-go/pkg/api"
	"sitemap-go/pkg/backend"
	"sitemap-go/pkg/extractor"
	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/storage"
	"sitemap-go/pkg/worker"
)

// SitemapMonitor orchestrates the sitemap monitoring workflow
type SitemapMonitor struct {
	parserFactory      parser.ParserFactory
	keywordExtractor   extractor.KeywordExtractor
	apiClient          api.APIClient
	storage            storage.Storage
	workerPool         *worker.ConcurrentPool
	simpleTracker      *storage.SimpleTracker  // Simplified URL hash and failed keyword tracking
	submissionPool     *backend.SubmissionPool // Non-blocking backend submission
	retryProcessor     *SimpleRetryProcessor   // Simple startup retry processor
	dataConverter      *backend.DataConverter  // Data format converter
	concurrencyManager *AdaptiveConcurrencyManager // Dynamic concurrency control
	rateLimiter        *RateLimitedExecutor    // Rate limiting for requests
	rateLimiterPool    *RateLimiterPool        // Pool for managing rate limiters (Resource Pool pattern)
	apiExecutor        *api.SequentialExecutor // Sequential API execution with 1s interval
	log                *logger.Logger
	secureLog          *logger.SecurityLogger  // Security-aware logger for sensitive data
}

// MonitorConfig holds configuration for sitemap monitoring
type MonitorConfig struct {
	SitemapURLs       []string              `json:"sitemap_urls"`
	TrendAPIBaseURL   string                `json:"trend_api_base_url"`
	BackendConfig     backend.BackendConfig `json:"backend_config"`
	EncryptionKey     string                `json:"encryption_key"`
	WorkerPoolSize    int                   `json:"worker_pool_size"`
	EnableBackendSubmission bool            `json:"enable_backend_submission"`
}

// MonitorResult represents the result of monitoring a sitemap
type MonitorResult struct {
	SitemapURL string                 `json:"sitemap_url"`
	Keywords   []string               `json:"keywords"`
	TrendData  *api.APIResponse       `json:"trend_data,omitempty"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NewSitemapMonitor creates a new sitemap monitor
func NewSitemapMonitor(cfg interface{}) (*SitemapMonitor, error) {
	// Create a default MonitorConfig for backward compatibility
	config := MonitorConfig{
		TrendAPIBaseURL: "", // Will be set from environment variable
		WorkerPoolSize:  8,
		EncryptionKey:   "default-sitemap-monitor-key",
		EnableBackendSubmission: false,
	}
	// Initialize components
	parserFactory := parser.GetParserFactory()
	keywordExtractor := extractor.NewURLKeywordExtractor()
	
	// Create API client for trend data (no API key required)
	trendAPIClient := api.NewHTTPAPIClient(config.TrendAPIBaseURL, "")
	
	// Create storage service
	storageConfig := storage.StorageConfig{
		DataDir:     "./data",
		CacheSize:   100,
		EncryptData: true,
	}
	encryptionKey := config.EncryptionKey
	if encryptionKey == "" {
		return nil, fmt.Errorf("encryption key is required for secure data storage - set ENCRYPTION_KEY environment variable")
	}
	var storageService storage.Storage
	storageService, err := storage.NewEncryptedFileStorage(storageConfig, encryptionKey)
	if err != nil {
		// Use a simple storage fallback if encrypted storage fails
		storageService = storage.NewMemoryStorage()
	}
	
	// Create high-performance worker pool with 8 concurrent workers
	poolConfig := worker.DefaultPoolConfig()
	poolConfig.Workers = 8 // As requested: 8 concurrent workers
	workerPool := worker.NewConcurrentPool(poolConfig)
	
	// Create simplified tracker for URL hashes and failed keywords
	simpleTracker := storage.NewSimpleTracker(storageService)
	
	// Create backend client with default configuration
	backendConfig := backend.BackendConfig{
		BaseURL:    "https://api.example.com", // Will be configured via command line
		APIKey:     "", // Will be configured via command line
		BatchSize:  300,
		EnableGzip: true,
		Timeout:    60 * time.Second,
	}
	backendClient, err := backend.NewBackendClient(backendConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend client: %w", err)
	}
	dataConverter := backend.NewDataConverter()
	
	// Create non-blocking submission pool
	submissionPool := backend.NewSubmissionPool(backendClient, 3) // 3 worker threads
	
	// Create simple retry processor for failed keywords (startup only)
	retryProcessor := NewSimpleRetryProcessor(trendAPIClient, simpleTracker, submissionPool, dataConverter)
	
	// Create adaptive concurrency manager with optimized settings
	concurrencyConfig := DefaultConcurrencyConfig()
	concurrencyManager := NewAdaptiveConcurrencyManager(concurrencyConfig)
	
	// Create rate limiter for sitemap requests
	rateLimiter := NewRateLimitedExecutor(concurrencyConfig.SitemapRequestsPerSecond)
	
	// Create rate limiter pool for resource management
	rateLimiterPool := NewRateLimiterPool()
	
	return &SitemapMonitor{
		parserFactory:      parserFactory,
		keywordExtractor:   keywordExtractor,
		apiClient:          trendAPIClient,
		storage:            storageService,
		workerPool:         workerPool,
		simpleTracker:      simpleTracker,
		submissionPool:     submissionPool,
		retryProcessor:     retryProcessor,
		dataConverter:      dataConverter,
		concurrencyManager: concurrencyManager,
		rateLimiter:        rateLimiter,
		rateLimiterPool:    rateLimiterPool,
		apiExecutor:        api.NewSequentialExecutor(1 * time.Second), // 1 second minimum interval
		log:                logger.GetLogger().WithField("component", "sitemap_monitor"),
		secureLog:          logger.GetSecurityLogger(),
	}, nil
}

// NewSitemapMonitorWithBackend creates a new sitemap monitor with backend configuration
// DEPRECATED: Use MonitorConfigBuilder instead for better error handling
func NewSitemapMonitorWithBackend(backendURL, apiKey string, batchSize int) *SitemapMonitor {
	// For backward compatibility, we can't panic. Return a monitor that will fail gracefully.
	builder := NewMonitorConfigBuilder().
		WithBackend(backendURL, apiKey).
		WithBatchSize(batchSize).
		WithTrendsAPI("http://placeholder-will-fail.com") // Will fail at runtime, but no panic
	
	monitor, err := builder.Build()
	if err != nil {
		// Log the error but don't panic - let it fail at runtime
		logger.GetLogger().WithError(err).Error("Failed to create monitor with deprecated method")
		return nil
	}
	return monitor
}

// NewSitemapMonitorWithConfig creates a new sitemap monitor with full configuration
// DEPRECATED: Use MonitorConfigBuilder for better error handling and validation
func NewSitemapMonitorWithConfig(backendURL, apiKey string, batchSize int, trendsAPIURL string) *SitemapMonitor {
	builder := NewMonitorConfigBuilder().
		WithTrendsAPI(trendsAPIURL).
		WithBackend(backendURL, apiKey).
		WithBatchSize(batchSize)
	
	monitor, err := builder.Build()
	if err != nil {
		// For backward compatibility, we have to panic here since the original signature doesn't return error
		// But this is clearly marked as deprecated
		panic(fmt.Sprintf("NewSitemapMonitorWithConfig failed: %v. Use MonitorConfigBuilder instead.", err))
	}
	return monitor
}

// createSitemapMonitorInternal is the internal safe constructor used by the builder
func createSitemapMonitorInternal(config MonitorConfig, backendURL, apiKey string, batchSize int) (*SitemapMonitor, error) {
	// Initialize components
	parserFactory := parser.GetParserFactory()
	keywordExtractor := extractor.NewURLKeywordExtractor()
	
	// Create API client for trend data
	trendAPIClient := api.NewHTTPAPIClient(config.TrendAPIBaseURL, "")
	
	// Create storage service
	storageConfig := storage.StorageConfig{
		DataDir:     "./data",
		CacheSize:   100,
		EncryptData: true,
	}
	var storageService storage.Storage
	storageService, err := storage.NewEncryptedFileStorage(storageConfig, config.EncryptionKey)
	if err != nil {
		storageService = storage.NewMemoryStorage()
	}
	
	// Create high-performance worker pool
	poolConfig := worker.DefaultPoolConfig()
	poolConfig.Workers = 8
	workerPool := worker.NewConcurrentPool(poolConfig)
	
	// Create simplified tracker
	simpleTracker := storage.NewSimpleTracker(storageService)
	
	// Create backend client with provided configuration
	backendConfig := backend.BackendConfig{
		BaseURL:    backendURL,
		APIKey:     apiKey,
		BatchSize:  batchSize,
		EnableGzip: true,
		Timeout:    60 * time.Second,
	}
	backendClient, err := backend.NewBackendClient(backendConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend client: %w", err)
	}
	dataConverter := backend.NewDataConverter()
	
	// Create non-blocking submission pool
	submissionPool := backend.NewSubmissionPool(backendClient, 3)
	
	// Create retry service
	retryProcessor := NewSimpleRetryProcessor(trendAPIClient, simpleTracker, submissionPool, dataConverter)
	
	// Create adaptive concurrency manager with optimized settings
	concurrencyConfig := DefaultConcurrencyConfig()
	concurrencyManager := NewAdaptiveConcurrencyManager(concurrencyConfig)
	
	// Create rate limiter for sitemap requests
	rateLimiter := NewRateLimitedExecutor(concurrencyConfig.SitemapRequestsPerSecond)
	
	// Create rate limiter pool for resource management
	rateLimiterPool := NewRateLimiterPool()
	
	return &SitemapMonitor{
		parserFactory:      parserFactory,
		keywordExtractor:   keywordExtractor,
		apiClient:          trendAPIClient,
		storage:            storageService,
		workerPool:         workerPool,
		simpleTracker:      simpleTracker,
		submissionPool:     submissionPool,
		retryProcessor:     retryProcessor,
		dataConverter:      dataConverter,
		concurrencyManager: concurrencyManager,
		rateLimiter:        rateLimiter,
		rateLimiterPool:    rateLimiterPool,
		apiExecutor:        api.NewSequentialExecutor(1 * time.Second), // 1 second minimum interval
		log:                logger.GetLogger().WithField("component", "sitemap_monitor"),
		secureLog:          logger.GetSecurityLogger(),
	}, nil
}



// ProcessSitemaps processes multiple sitemaps with global keyword deduplication
func (sm *SitemapMonitor) ProcessSitemaps(ctx context.Context, sitemapURLs []string, workers int) ([]*MonitorResult, error) {
	if workers <= 0 {
		workers = 8
	}
	
	sm.secureLog.SafeInfo("Starting batch sitemap processing with global keyword deduplication", map[string]interface{}{
		"sitemap_count": len(sitemapURLs),
		"workers":       workers,
	})
	
	// Start background services
	sm.submissionPool.Start(ctx)
	
	// Process failed keywords at startup (non-blocking)
	sm.retryProcessor.ProcessFailedKeywordsAtStartup(ctx)
	
	// Note: Submission pool will be stopped in Close() method
	
	// Step 1: Extract all keywords from all sitemaps first
	sm.log.Info("Step 1: Extracting keywords from all sitemaps")
	allKeywords, keywordToSpecificURLMap, sitemapResults, err := sm.extractAllKeywords(ctx, sitemapURLs, workers)
	if err != nil {
		return nil, fmt.Errorf("failed to extract keywords: %w", err)
	}
	
	// Step 2: Global keyword deduplication
	sm.log.WithField("total_keywords_before_dedup", len(allKeywords)).Info("Step 2: Global keyword deduplication")
	uniqueKeywords := sm.deduplicateKeywords(allKeywords)
	sm.log.WithFields(map[string]interface{}{
		"keywords_before_dedup": len(allKeywords),
		"keywords_after_dedup":  len(uniqueKeywords),
		"deduplication_ratio":   fmt.Sprintf("%.1f%%", float64(len(uniqueKeywords))/float64(len(allKeywords))*100),
	}).Info("Global deduplication completed")
	
	// Step 3: Query Google Trends API for unique keywords
	if len(uniqueKeywords) > 0 {
		sm.log.WithField("unique_keywords", len(uniqueKeywords)).Info("Step 3: Querying Google Trends for unique keywords")
		err = sm.queryAndSubmitKeywords(ctx, uniqueKeywords, keywordToSpecificURLMap)
		if err != nil {
			sm.secureLog.SafeError("Failed to query and submit keywords", err, nil)
		}
	}
	
	// Step 4: Update sitemap results and save URL hashes
	sm.log.Info("Step 4: Saving URL hashes for processed sitemaps")
	sm.saveProcessedSitemaps(ctx, sitemapResults)
	
	// Count success/failure for summary
	successCount := 0
	for _, result := range sitemapResults {
		if result.Success {
			successCount++
		}
	}
	
	sm.log.WithFields(map[string]interface{}{
		"total_sitemaps":        len(sitemapResults),
		"success_count":         successCount,
		"failure_count":         len(sitemapResults) - successCount,
		"unique_keywords_queried": len(uniqueKeywords),
	}).Info("Batch processing completed with global deduplication")
	
	return sitemapResults, nil
}

// MonitorSitemaps executes the complete monitoring workflow with 8 concurrent workers
func (sm *SitemapMonitor) MonitorSitemaps(ctx context.Context, config MonitorConfig) ([]*MonitorResult, error) {
	sm.log.WithField("sitemap_count", len(config.SitemapURLs)).Info("Starting concurrent sitemap monitoring")
	
	// Start the worker pool
	if err := sm.workerPool.Start(); err != nil {
		return nil, fmt.Errorf("failed to start worker pool: %w", err)
	}
	defer sm.workerPool.Stop()
	
	// Apply URL filters before processing
	filteredURLs := sm.applyURLFilters(config.SitemapURLs)
	sm.log.WithFields(map[string]interface{}{
		"original_count": len(config.SitemapURLs),
		"filtered_count": len(filteredURLs),
	}).Info("Applied URL filters")
	
	// Submit tasks to worker pool (non-blocking) and keep references
	tasks := make(map[string]*SitemapTask)
	for _, sitemapURL := range filteredURLs {
		task := NewSitemapTask(sitemapURL, config, sm)
		tasks[sitemapURL] = task
		if err := sm.workerPool.Submit(task); err != nil {
			sm.secureLog.ErrorWithURL("Failed to submit task to worker pool", sitemapURL, err, nil)
		}
	}
	
	// Collect results from worker pool
	results := make([]*MonitorResult, 0, len(filteredURLs))
	resultMap := make(map[string]*MonitorResult)
	var mu sync.Mutex
	
	// Process results as they come in
	resultChannel := sm.workerPool.GetResultChannel()
	processedResults := 0
	
	// Wait for all tasks to complete with timeout
	timeout := time.After(10 * time.Minute) // Configurable timeout
	
	for processedResults < len(filteredURLs) {
		select {
		case result := <-resultChannel:
			mu.Lock()
			processedResults++
			
			if result.Success {
				// Extract result data directly from the worker result
				if taskResult, ok := result.Data.(*MonitorResult); ok {
					resultMap[taskResult.SitemapURL] = taskResult
				} else {
					// Find the task URL for fallback
					for sitemapURL, task := range tasks {
						if task.GetID() == result.TaskID {
							if taskData := task.GetResult(); taskData != nil {
								if monitorResult, ok := taskData.(*MonitorResult); ok {
									resultMap[sitemapURL] = monitorResult
								}
							} else {
								// Create success result without data
								resultMap[sitemapURL] = &MonitorResult{
									SitemapURL: sitemapURL,
									Success:    true,
									Timestamp:  result.Timestamp,
								}
							}
							break
						}
					}
				}
			} else {
				// Find the task URL for error result
				for sitemapURL, task := range tasks {
					if task.GetID() == result.TaskID {
						errorResult := &MonitorResult{
							SitemapURL: sitemapURL,
							Success:    false,
							Error:      result.Error.Error(),
							Timestamp:  result.Timestamp,
						}
						resultMap[sitemapURL] = errorResult
						break
					}
				}
			}
			mu.Unlock()
			
		case <-timeout:
			sm.log.Warn("Timeout waiting for sitemap processing to complete")
			goto collectResults
		case <-ctx.Done():
			sm.log.Warn("Context cancelled during monitoring")
			goto collectResults
		}
	}
	
collectResults:
	// Collect all results
	mu.Lock()
	for sitemapURL := range resultMap {
		if result, exists := resultMap[sitemapURL]; exists {
			results = append(results, result)
		}
	}
	
	// Create results for any missing URLs (failed to process)
	for _, sitemapURL := range filteredURLs {
		if _, exists := resultMap[sitemapURL]; !exists {
			results = append(results, &MonitorResult{
				SitemapURL: sitemapURL,
				Success:    false,
				Error:      "task timeout or failed to process",
				Timestamp:  time.Now(),
			})
		}
	}
	mu.Unlock()
	
	// Log pool metrics
	metrics := sm.workerPool.GetMetrics()
	sm.log.WithFields(map[string]interface{}{
		"total_tasks":     metrics.TotalTasks,
		"completed_tasks": metrics.CompletedTasks,
		"failed_tasks":    metrics.FailedTasks,
		"success_rate":    metrics.GetSuccessRate(),
		"total_results":   len(results),
	}).Info("Concurrent sitemap monitoring completed")
	
	return results, nil
}

// ProcessSitemap is deprecated - use ProcessSitemaps with global deduplication instead
func (sm *SitemapMonitor) ProcessSitemap(ctx context.Context, sitemapURL string) (*MonitorResult, error) {
	// For backward compatibility, process as single sitemap batch
	results, err := sm.ProcessSitemaps(ctx, []string{sitemapURL}, 1)
	if err != nil {
		return nil, err
	}
	if len(results) > 0 {
		return results[0], nil
	}
	return &MonitorResult{
		SitemapURL: sitemapURL,
		Success:    false,
		Error:      "no results returned",
		Timestamp:  time.Now(),
	}, nil
}

// SubmitToBackend submits monitoring results to the backend API
func (sm *SitemapMonitor) SubmitToBackend(ctx context.Context, results []*MonitorResult, backendConfig BackendConfig) error {
	if len(results) == 0 {
		return nil
	}
	
	sm.log.WithField("result_count", len(results)).Info("Submitting results to backend")
	
	// Create backend API client
	backendClient := api.NewHTTPAPIClient(backendConfig.BaseURL, backendConfig.APIKey)
	
	// Prepare submission data (for future use)
	_ = map[string]interface{}{
		"timestamp": time.Now(),
		"results":   results,
		"metadata": map[string]interface{}{
			"total_sitemaps": len(results),
			"successful":     countSuccessful(results),
			"failed":         len(results) - countSuccessful(results),
		},
	}
	
	// Convert to keywords for API submission (adapting to existing API interface)
	allKeywords := make([]string, 0)
	for _, result := range results {
		if result.Success {
			allKeywords = append(allKeywords, result.Keywords...)
		}
	}
	
	// Submit to backend
	_, err := backendClient.Query(ctx, allKeywords)
	if err != nil {
		sm.log.WithError(err).Error("Failed to submit results to backend")
		return fmt.Errorf("backend submission failed: %w", err)
	}
	
	sm.log.Info("Results submitted to backend successfully")
	return nil
}

// BackendConfig holds backend API configuration
type BackendConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}


// Helper function to count successful results
func countSuccessful(results []*MonitorResult) int {
	count := 0
	for _, result := range results {
		if result.Success {
			count++
		}
	}
	return count
}

// determineFormat determines the sitemap format from URL
func (sm *SitemapMonitor) determineFormat(sitemapURL string) string {
	// Check RSS/Feed patterns first (higher priority than file extension)
	if strings.Contains(sitemapURL, "rss") || strings.Contains(sitemapURL, "feed") {
		return "rss"
	}
	
	// Check file extensions
	if strings.Contains(sitemapURL, ".xml.gz") {
		return "xml.gz"
	}
	if strings.Contains(sitemapURL, ".txt") {
		return "txt"
	}
	if strings.Contains(sitemapURL, ".xml") {
		return "xml"
	}
	
	// Default to XML
	return "xml"
}

// applyURLFilters applies filtering rules to sitemap URLs
func (sm *SitemapMonitor) applyURLFilters(sitemapURLs []string) []string {
	// Create URL filters based on PRD requirements
	pathFilter := parser.NewPathFilter("exclude_admin", []string{
		"/admin/", "/wp-admin/", "/dashboard/", "/login/", "/private/",
		"/test/", "/staging/", "/dev/", "/debug/",
	})
	
	extensionFilter := parser.NewExtensionFilter("exclude_media", []string{
		".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico",
		".pdf", ".doc", ".docx", ".zip", ".tar", ".gz",
		".mp3", ".mp4", ".avi", ".mov", ".css", ".js",
	})
	
	filtered := make([]string, 0, len(sitemapURLs))
	
	for _, sitemapURL := range sitemapURLs {
		// Parse URL for filtering
		parsedURL, err := parseURL(sitemapURL)
		if err != nil {
			sm.secureLog.DebugWithURL("Failed to parse URL for filtering", sitemapURL, map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}
		
		// Apply filters
		if pathFilter.ShouldExclude(parsedURL) {
			sm.secureLog.DebugWithURL("URL excluded by path filter", sitemapURL, nil)
			continue
		}
		
		if extensionFilter.ShouldExclude(parsedURL) {
			sm.secureLog.DebugWithURL("URL excluded by extension filter", sitemapURL, nil)
			continue
		}
		
		// Additional game-specific filtering
		if sm.shouldExcludeGameURL(sitemapURL) {
			sm.secureLog.DebugWithURL("URL excluded by game-specific filter", sitemapURL, nil)
			continue
		}
		
		filtered = append(filtered, sitemapURL)
	}
	
	return filtered
}

// shouldExcludeGameURL applies game-specific filtering rules
func (sm *SitemapMonitor) shouldExcludeGameURL(sitemapURL string) bool {
	lowerURL := strings.ToLower(sitemapURL)
	
	// Exclude non-game related sitemaps
	excludePatterns := []string{
		"privacy", "terms", "contact", "about", "help",
		"support", "blog", "news", "legal", "cookies",
		"ads", "advertisement", "tracking", "analytics",
	}
	
	for _, pattern := range excludePatterns {
		if strings.Contains(lowerURL, pattern) {
			return true
		}
	}
	
	return false
}

// selectPrimaryKeyword selects the most representative keyword from a list
func (sm *SitemapMonitor) selectPrimaryKeyword(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	if len(keywords) == 1 {
		return keywords[0]
	}
	
	// Strategy: Select the longest keyword as it's usually most descriptive
	// For game URLs, longer keywords often contain the actual game name
	longest := keywords[0]
	for _, keyword := range keywords[1:] {
		if len(keyword) > len(longest) {
			longest = keyword
		}
	}
	
	sm.secureLog.SafeDebug("Selected primary keyword", map[string]interface{}{
		"all_keywords":     keywords,
		"selected_keyword": longest,
	})
	
	return longest
}

// saveFailedKeywords saves keywords that failed API query for retry
func (sm *SitemapMonitor) saveFailedKeywords(ctx context.Context, keywords []string, keywordURLMap map[string]string, sitemapURL string, err error) {
	sm.secureLog.WarnWithURL("API query failed, saving keywords for retry", sitemapURL, map[string]interface{}{
		"keyword_count": len(keywords),
		"error": err.Error(),
	})
	
	for _, keyword := range keywords {
		sourceURL := keywordURLMap[keyword]
		if sourceURL == "" {
			sourceURL = sitemapURL // Fallback to sitemap URL
		}
		
		if saveErr := sm.simpleTracker.SaveFailedKeywords(ctx, []string{keyword}, sourceURL, sitemapURL, err); saveErr != nil {
			sm.secureLog.SafeError("Failed to save failed keyword", saveErr, nil)
		}
	}
}

// ExtractAllKeywords extracts keywords from all sitemaps with optimized concurrency (exported for testing)
func (sm *SitemapMonitor) ExtractAllKeywords(ctx context.Context, sitemapURLs []string, workers int) ([]string, map[string]string, []*MonitorResult, error) {
	return sm.extractAllKeywords(ctx, sitemapURLs, workers)
}

// extractAllKeywords extracts keywords from all sitemaps with optimized concurrency
func (sm *SitemapMonitor) extractAllKeywords(ctx context.Context, sitemapURLs []string, workers int) ([]string, map[string]string, []*MonitorResult, error) {
	// Use adaptive concurrency settings instead of fixed workers count
	config := sm.concurrencyManager.GetCurrentConfig()
	actualWorkers := min(config.MainWorkers, len(sitemapURLs)) // Don't exceed sitemap count
	type extractResult struct {
		sitemapURL string
		keywords   []string
		urls       []string
		success    bool
		error      string
	}
	
	results := make([]extractResult, 0, len(sitemapURLs))
	resultsChan := make(chan extractResult, len(sitemapURLs))
	semaphore := make(chan struct{}, actualWorkers) // Use adaptive worker count
	
	var wg sync.WaitGroup
	
	sm.secureLog.SafeInfo("Starting optimized concurrent sitemap processing", map[string]interface{}{
		"requested_workers": workers,
		"actual_workers":    actualWorkers,
		"sitemap_count":     len(sitemapURLs),
	})
	
	// Extract keywords from each sitemap with rate limiting and performance tracking
	for _, sitemapURL := range sitemapURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Rate limiting: use sitemap-specific rate limiter (more permissive)
			startTime := time.Now()
			var keywords []string
			var urls []string
			var err error
			
			// Use shared rate limiter from pool (Resource Pool pattern - prevents leakage)
			config := sm.concurrencyManager.GetCurrentConfig()
			sitemapLimiter := sm.rateLimiterPool.GetOrCreate(config.SitemapRequestsPerSecond)
			
			rateLimitErr := sitemapLimiter.Execute(ctx, func() error {
				keywords, urls, err = sm.extractKeywordsFromSitemap(ctx, url)
				return err
			})
			
			responseTime := time.Since(startTime)
			success := err == nil && rateLimitErr == nil
			
			// Update performance metrics for adaptive adjustment
			sm.concurrencyManager.UpdateMetrics(responseTime, success)
			
			result := extractResult{
				sitemapURL: url,
				keywords:   keywords,
				urls:       urls,
				success:    success,
			}
			if err != nil {
				result.error = err.Error()
			} else if rateLimitErr != nil {
				result.error = rateLimitErr.Error()
			}
			
			resultsChan <- result
		}(sitemapURL)
	}
	
	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		results = append(results, result)
	}
	
	// Aggregate all keywords and build mapping to specific URLs
	var allKeywords []string
	keywordToSpecificURLMap := make(map[string]string) // Maps keyword to specific URL (not sitemap)
	sitemapResults := make([]*MonitorResult, len(results))
	
	for i, result := range results {
		sitemapResults[i] = &MonitorResult{
			SitemapURL: result.sitemapURL,
			Keywords:   result.keywords,
			Success:    result.success,
			Error:      result.error,
			Timestamp:  time.Now(),
			Metadata:   make(map[string]interface{}),
		}
		
		if result.success {
			// Build keyword to specific URL mapping (1:1 correspondence)
			for j, keyword := range result.keywords {
				// Format keyword for API query ("action-games" → "action games")
				formattedKeyword := sm.formatKeywordForAPI(keyword)
				allKeywords = append(allKeywords, formattedKeyword)
				
				// Map formatted keyword to specific URL
				if j < len(result.urls) {
					keywordToSpecificURLMap[formattedKeyword] = result.urls[j]
				}
			}
			sitemapResults[i].Metadata["url_count"] = len(result.urls)
		}
	}
	
	return allKeywords, keywordToSpecificURLMap, sitemapResults, nil
}

// formatKeywordForAPI formats keywords for Google Trends API query
func (sm *SitemapMonitor) formatKeywordForAPI(keyword string) string {
	// Convert hyphens to spaces for API query
	formatted := strings.ReplaceAll(keyword, "-", " ")
	
	// Clean up multiple spaces
	formatted = strings.TrimSpace(formatted)
	formatted = regexp.MustCompile(`\s+`).ReplaceAllString(formatted, " ")
	
	// Convert to lowercase for consistency
	formatted = strings.ToLower(formatted)
	
	return formatted
}

// extractKeywordsFromSitemap extracts keywords from a single sitemap
func (sm *SitemapMonitor) extractKeywordsFromSitemap(ctx context.Context, sitemapURL string) ([]string, []string, error) {
	sm.secureLog.DebugWithURL("Extracting keywords from sitemap", sitemapURL, nil)
	
	// Parse sitemap
	format := sm.determineFormat(sitemapURL)
	sitemapParser := sm.parserFactory.GetParser(format)
	if sitemapParser == nil {
		return nil, nil, fmt.Errorf("no parser available for format: %s", format)
	}
	
	urls, err := sitemapParser.Parse(ctx, sitemapURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse sitemap: %w", err)
	}
	
	sm.log.WithField("url_count", len(urls)).Debug("Sitemap parsed, extracting keywords")
	
	// Extract keywords from URLs
	var keywords []string
	var urlList []string
	
	for _, url := range urls {
		urlKeywords, err := sm.keywordExtractor.Extract(url.Address)
		if err != nil {
			sm.secureLog.WarnWithURL("Failed to extract keywords from URL", url.Address, map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}
		
		if len(urlKeywords) > 0 {
			primaryKeyword := sm.selectPrimaryKeyword(urlKeywords)
			keywords = append(keywords, primaryKeyword)
			urlList = append(urlList, url.Address)
		}
	}
	
	sm.secureLog.DebugWithURL("Keywords extracted from sitemap", sitemapURL, map[string]interface{}{
		"keyword_count": len(keywords),
		"url_count":     len(urlList),
	})
	
	return keywords, urlList, nil
}

// deduplicateKeywords removes duplicate keywords globally
func (sm *SitemapMonitor) deduplicateKeywords(keywords []string) []string {
	keywordSet := make(map[string]bool)
	var uniqueKeywords []string
	
	for _, keyword := range keywords {
		if !keywordSet[keyword] {
			keywordSet[keyword] = true
			uniqueKeywords = append(uniqueKeywords, keyword)
		}
	}
	
	return uniqueKeywords
}

// queryAndSubmitKeywords queries Google Trends in batches and submits results to backend
func (sm *SitemapMonitor) queryAndSubmitKeywords(ctx context.Context, keywords []string, keywordToSpecificURLMap map[string]string) error {
	sm.log.WithField("keyword_count", len(keywords)).Info("Querying Google Trends API for deduplicated keywords")
	
	// Split keywords into batches of 8 (Google Trends limit is 10, we use 8 for safety)
	const batchSize = 8
	var allTrendData []api.Keyword
	
	for i := 0; i < len(keywords); i += batchSize {
		end := i + batchSize
		if end > len(keywords) {
			end = len(keywords)
		}
		
		batch := keywords[i:end]
		sm.log.WithField("batch_size", len(batch)).WithField("batch_start", i).Debug("Processing keyword batch")
		
		// Query Google Trends API for this batch with 1-second interval control
		var trendData *api.APIResponse
		err := sm.apiExecutor.Execute(ctx, func() error {
			var queryErr error
			trendData, queryErr = sm.apiClient.Query(ctx, batch)
			return queryErr
		})
		if err != nil {
			sm.secureLog.SafeError("Google Trends API batch query failed", err, map[string]interface{}{
				"batch_size": len(batch),
				"batch_start": i,
			})
			
			// Save this batch as failed and continue with next batch
			for _, keyword := range batch {
				specificURL := keywordToSpecificURLMap[keyword]
				if specificURL != "" {
					if saveErr := sm.simpleTracker.SaveFailedKeywords(ctx, []string{keyword}, specificURL, "", err); saveErr != nil {
						sm.secureLog.SafeError("Failed to save failed keyword", saveErr, nil)
					}
				}
			}
			continue // Continue with next batch for retryable errors
		}
		
		// Collect successful results
		if trendData != nil && len(trendData.Keywords) > 0 {
			allTrendData = append(allTrendData, trendData.Keywords...)
			sm.log.WithField("batch_success_count", len(trendData.Keywords)).Debug("Batch query successful")
		}
	}
	
	if len(allTrendData) == 0 {
		return fmt.Errorf("no successful trend data retrieved from any batch")
	}
	
	sm.log.WithField("successful_keywords", len(allTrendData)).Info("Google Trends batch queries successful")
	
	// Convert to backend format and submit
	var allBackendData []backend.KeywordMetricsData
	
	for _, keyword := range allTrendData {
		metrics := sm.dataConverter.ConvertKeywordMetrics(keyword)
		
		// Get specific URL for this keyword
		specificURL := keywordToSpecificURLMap[keyword.Word]
		if specificURL != "" {
			allBackendData = append(allBackendData, backend.KeywordMetricsData{
				Keyword: keyword.Word,
				URL:     specificURL, // Use specific game page URL
				Metrics: metrics,
			})
		}
	}
	
	// Submit to backend (non-blocking)
	if len(allBackendData) > 0 {
		sm.log.WithField("backend_data_count", len(allBackendData)).Info("Submitting deduplicated results to backend")
		
		success := sm.submissionPool.Submit(allBackendData, func(err error) {
			if err != nil {
				sm.secureLog.SafeError("Backend submission failed for deduplicated data", err, nil)
			} else {
				sm.log.WithField("data_count", len(allBackendData)).Info("Backend submission successful for deduplicated data")
			}
		})
		
		if !success {
			sm.log.Warn("Failed to queue deduplicated data for backend submission")
		}
	}
	
	return nil
}

// saveProcessedSitemaps saves URL hashes for all processed sitemaps
func (sm *SitemapMonitor) saveProcessedSitemaps(ctx context.Context, results []*MonitorResult) {
	for _, result := range results {
		if result.Success && len(result.Keywords) > 0 {
			if err := sm.simpleTracker.SaveProcessedURL(ctx, result.SitemapURL, result.Keywords); err != nil {
				sm.secureLog.WarnWithURL("Failed to save processed URL hash", result.SitemapURL, map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// Close properly closes all resources (implements proper cleanup)
func (sm *SitemapMonitor) Close() error {
	var errors []error
	
	// Helper function to safely execute cleanup operations
	safeClose := func(name string, fn func() error) {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic during %s close: %v", name, r)
				errors = append(errors, err)
				sm.log.WithError(err).Error("Resource cleanup panic recovered")
			}
		}()
		
		if err := fn(); err != nil {
			errors = append(errors, fmt.Errorf("%s close failed: %w", name, err))
			sm.log.WithError(err).Warn("Failed to close " + name)
		}
	}
	
	// Helper function for operations that don't return errors
	safeCloseNoError := func(name string, fn func()) {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic during %s close: %v", name, r)
				errors = append(errors, err)
				sm.log.WithError(err).Error("Resource cleanup panic recovered")
			}
		}()
		fn()
	}
	
	// Close all resources with panic protection
	safeClose("rate limiter pool", func() error {
		return sm.rateLimiterPool.Close()
	})
	
	safeCloseNoError("main rate limiter", func() {
		sm.rateLimiter.Close()
	})
	
	safeCloseNoError("submission pool", func() {
		sm.submissionPool.Stop()
	})
	
	// Note: Simple retry processor doesn't need cleanup as it's fire-and-forget
	
	// Return combined error if any occurred
	if len(errors) > 0 {
		var errorMsgs []string
		for _, err := range errors {
			errorMsgs = append(errorMsgs, err.Error())
		}
		return fmt.Errorf("multiple cleanup errors: %s", strings.Join(errorMsgs, "; "))
	}
	
	return nil
}

// Helper function to parse URL
func parseURL(urlStr string) (*url.URL, error) {
	return url.Parse(urlStr)
}