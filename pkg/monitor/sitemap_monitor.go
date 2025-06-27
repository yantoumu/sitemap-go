package monitor

import (
	"context"
	"fmt"
	"net/url"
	"os"
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
	
	monitor := &SitemapMonitor{
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
		apiExecutor:        api.NewSequentialExecutor(), // Sequential execution, no forced delays
		log:                logger.GetLogger().WithField("component", "sitemap_monitor"),
		secureLog:          logger.GetSecurityLogger(),
	}

	// Configure atomic concurrency control for API client (inspired by 1.js)
	monitor.configureAtomicConcurrencyControl(config.TrendAPIBaseURL)

	return monitor, nil
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
		apiExecutor:        api.NewSequentialExecutor(), // Sequential execution, no forced delays
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
	// Step 1: Extract keywords from all sitemaps
	allKeywords, keywordToSpecificURLMap, sitemapResults, err := sm.extractAllKeywords(ctx, sitemapURLs, workers)
	if err != nil {
		return nil, fmt.Errorf("failed to extract keywords: %w", err)
	}
	
	// Step 2: Global keyword deduplication
	sm.log.WithField("total_keywords_before_dedup", len(allKeywords)).Info("Step 2: Starting global keyword deduplication")
	uniqueKeywords := sm.deduplicateKeywords(allKeywords)
	sm.log.WithFields(map[string]interface{}{
		"keywords_before_dedup": len(allKeywords),
		"keywords_after_dedup":  len(uniqueKeywords),
		"deduplication_ratio":   fmt.Sprintf("%.1f%%", float64(len(uniqueKeywords))/float64(len(allKeywords))*100),
	}).Info("Global deduplication completed")
	
	// Step 2.5: URL级别去重 - 直接过滤已处理的URL (修复根本问题)
	sm.log.WithField("total_keywords", len(uniqueKeywords)).Info("Step 2.5: Filtering already processed URLs")
	filteredKeywords, err := sm.filterUnprocessedKeywordURLs(ctx, uniqueKeywords, keywordToSpecificURLMap)
	if err != nil {
		sm.secureLog.SafeError("Failed to filter processed URLs", err, nil)
		filteredKeywords = uniqueKeywords // Fallback to process all
	}

	sm.log.WithFields(map[string]interface{}{
		"total_sitemaps":         len(sitemapURLs),
		"keywords_before_filter": len(uniqueKeywords),
		"keywords_after_filter":  len(filteredKeywords),
	}).Info("URL filtering completed")

	// Step 3: Query SEOKey API for keywords from unprocessed sitemaps only
	if len(filteredKeywords) > 0 {
		sm.log.WithField("filtered_keywords", len(filteredKeywords)).Info("Step 3: Starting SEOKey API queries for unprocessed sitemaps")
		
		// Create reverse mapping from keyword to sitemap URL for success tracking
		keywordToSitemapMap := make(map[string]string)
		for _, result := range sitemapResults {
			if result.Success {
				for _, keyword := range result.Keywords {
					formattedKeyword := sm.formatKeywordForAPI(keyword)
					keywordToSitemapMap[formattedKeyword] = result.SitemapURL
				}
			}
		}
		
		err = sm.queryAndSubmitKeywords(ctx, filteredKeywords, keywordToSpecificURLMap, keywordToSitemapMap)
		if err != nil {
			sm.secureLog.SafeError("Failed to query and submit keywords", err, nil)
		}
	} else {
		sm.log.Info("No keywords from unprocessed sitemaps found to query")
	}
	
	// Step 4: Sitemap processing completed
	// Note: Successful sitemaps are now saved in queryAndSubmitKeywords after API success
	
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
	}).Debug("Batch processing completed")
	
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
	
	// Wait for all tasks to complete with configurable timeout
	concurrencyConfig := sm.concurrencyManager.GetCurrentConfig()
	processingTimeout := concurrencyConfig.DownloadTimeout * time.Duration(len(filteredURLs)) // Scale with number of URLs
	if processingTimeout > 15*time.Minute {
		processingTimeout = 15 * time.Minute // Cap at 15 minutes
	}
	if processingTimeout < 2*time.Minute {
		processingTimeout = 2 * time.Minute // Minimum 2 minutes
	}
	timeout := time.After(processingTimeout)
	
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
	
	// Submit results to backend
	
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
	
	// Backend submission successful
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
// Uses improved algorithm considering both length and semantic importance
func (sm *SitemapMonitor) selectPrimaryKeyword(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	if len(keywords) == 1 {
		return keywords[0]
	}

	// Strategy: Score keywords based on multiple factors
	bestKeyword := keywords[0]
	bestScore := sm.scoreKeyword(bestKeyword)

	for _, keyword := range keywords[1:] {
		score := sm.scoreKeyword(keyword)
		if score > bestScore {
			bestScore = score
			bestKeyword = keyword
		}
	}

	sm.secureLog.SafeDebug("Selected primary keyword", map[string]interface{}{
		"all_keywords":     keywords,
		"selected_keyword": bestKeyword,
		"score":           bestScore,
	})

	return bestKeyword
}

// scoreKeyword calculates a score for keyword selection
// Higher score indicates better keyword quality
func (sm *SitemapMonitor) scoreKeyword(keyword string) float64 {
	if keyword == "" {
		return 0
	}

	score := float64(len(keyword)) // Base score from length

	// Bonus for game-related terms
	gameTerms := []string{"game", "play", "puzzle", "action", "adventure", "strategy", "arcade"}
	for _, term := range gameTerms {
		if strings.Contains(strings.ToLower(keyword), term) {
			score += 10 // Significant bonus for game terms
			break
		}
	}

	// Penalty for common generic terms
	genericTerms := []string{"index", "page", "home", "main", "default"}
	for _, term := range genericTerms {
		if strings.ToLower(keyword) == term {
			score -= 5 // Penalty for generic terms
			break
		}
	}

	// Bonus for keywords with meaningful separators (likely compound terms)
	if strings.Contains(keyword, "-") || strings.Contains(keyword, "_") {
		score += 3
	}

	return score
}

// getAPIEndpointForRateLimiting returns the API endpoint identifier for rate limiting
// Enables proper dual API utilization with independent rate limiters
func (sm *SitemapMonitor) getAPIEndpointForRateLimiting() string {
	// Check if using DualAPIClient
	if dualClient, ok := sm.apiClient.(interface{ GetCurrentAPIEndpoint() string }); ok {
		endpoint := dualClient.GetCurrentAPIEndpoint()
		if endpoint != "" {
			return endpoint
		}
	}

	// Fallback to generic identifier for single API clients
	return "default-api"
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
	actualWorkers := config.MainWorkers
	if len(sitemapURLs) < actualWorkers {
		actualWorkers = len(sitemapURLs) // Don't exceed sitemap count
	}
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
			
			// Direct execution without rate limiting for local operations
			// Rate limiting should only apply to API calls, not local keyword extraction
			keywords, urls, err = sm.extractKeywordsFromSitemap(ctx, url)
			
			responseTime := time.Since(startTime)
			success := err == nil
			
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
	sm.secureLog.InfoWithURL("Starting keyword extraction from sitemap", sitemapURL, nil)
	
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
	
	// Only log for large sitemaps to reduce noise
	if len(urls) > 1000 {
		sm.log.WithField("total_urls", len(urls)).Debug("Starting parallel keyword extraction for large sitemap")
	}
	
	// Use parallel extraction with configured worker count for optimal performance
	config := sm.concurrencyManager.GetCurrentConfig()
	parallelExtractor := NewParallelKeywordExtractorWithWorkers(config.ExtractWorkers)
	keywords, urlList, failedCount := parallelExtractor.ExtractFromURLs(ctx, urls, sm.selectPrimaryKeyword)
	
	// Log summary only if there are significant failures
	if failedCount > 10 {
		sm.secureLog.WarnWithURL("High keyword extraction failure rate", sitemapURL, map[string]interface{}{
			"failed_count": failedCount,
			"total_urls":   len(urls),
			"failure_rate": float64(failedCount) / float64(len(urls)),
		})
	}
	
	sm.secureLog.DebugWithURL("Keywords extracted from sitemap", sitemapURL, map[string]interface{}{
		"keyword_count": len(keywords),
		"url_count":     len(urlList),
	})
	
	return keywords, urlList, nil
}

// deduplicateKeywords removes duplicate keywords globally with intelligent similarity detection
func (sm *SitemapMonitor) deduplicateKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return keywords
	}

	// Pre-allocate with estimated capacity
	keywordSet := make(map[string]bool, len(keywords))
	normalizedSet := make(map[string]string, len(keywords)) // normalized -> original mapping
	var uniqueKeywords []string

	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}

		// Skip exact duplicates
		if keywordSet[keyword] {
			continue
		}

		// Normalize for similarity detection
		normalized := sm.normalizeForDeduplication(keyword)

		// Check if we already have a similar keyword
		if existingKeyword, exists := normalizedSet[normalized]; exists {
			// Keep the better quality keyword (higher score)
			if sm.scoreKeyword(keyword) > sm.scoreKeyword(existingKeyword) {
				// Replace the existing keyword with the better one
				normalizedSet[normalized] = keyword
				// Update in uniqueKeywords slice
				for i, existing := range uniqueKeywords {
					if existing == existingKeyword {
						uniqueKeywords[i] = keyword
						break
					}
				}
				keywordSet[existingKeyword] = false // Mark old as removed
				keywordSet[keyword] = true
			}
			// Skip adding the lower quality duplicate
			continue
		}

		// Add new unique keyword
		normalizedSet[normalized] = keyword
		keywordSet[keyword] = true
		uniqueKeywords = append(uniqueKeywords, keyword)
	}

	return uniqueKeywords
}

// normalizeForDeduplication creates a normalized form for similarity detection
func (sm *SitemapMonitor) normalizeForDeduplication(keyword string) string {
	// Convert to lowercase
	normalized := strings.ToLower(keyword)

	// Replace common separators with spaces
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, ".", " ")

	// Remove extra spaces
	normalized = strings.Join(strings.Fields(normalized), " ")

	return strings.TrimSpace(normalized)
}

// batchResult holds the result of a batch API query
type batchResult struct {
	batch     []string
	trendData *api.APIResponse
	err       error
}

// queryAndSubmitKeywords queries SEOKey API in batches and submits results to backend
// Uses concurrent batch processing with configurable workers and optimized batch size
func (sm *SitemapMonitor) queryAndSubmitKeywords(ctx context.Context, keywords []string, keywordToSpecificURLMap map[string]string, keywordToSitemapMap map[string]string) error {
	// Get current configuration for dynamic worker count
	config := sm.concurrencyManager.GetCurrentConfig()
	batchSize := 10 // Maximum batch size for SEOKey API (API limit: 10 keywords per request)
	concurrentWorkers := config.APIWorkers // Use configured worker count
	
	sm.log.WithField("total_keywords", len(keywords)).Info("🔍 Starting API keyword analysis")
	
	// Create batches
	var batches [][]string
	for i := 0; i < len(keywords); i += batchSize {
		end := i + batchSize
		if end > len(keywords) {
			end = len(keywords)
		}
		batches = append(batches, keywords[i:end])
	}
	
	// Channel for batch processing
	batchChan := make(chan []string, len(batches))
	resultChan := make(chan batchResult, len(batches))
	
	// Send batches to channel
	for _, batch := range batches {
		batchChan <- batch
	}
	close(batchChan)
	
	// Start concurrent workers
	var wg sync.WaitGroup
	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go sm.processBatchWorker(ctx, i, batchChan, resultChan, keywordToSpecificURLMap, &wg)
	}

	// Wait for all workers to complete - pass wg as parameter to avoid race condition
	go func(waitGroup *sync.WaitGroup, resChan chan<- batchResult) {
		defer func() {
			if r := recover(); r != nil {
				sm.log.WithField("panic", r).Error("Result channel closer panicked")
			}
		}()
		waitGroup.Wait()
		close(resChan)
	}(&wg, resultChan)
	
	// Collect results
	var allTrendData []api.Keyword
	var totalErrors int
	var successfulKeywords []string // Track keywords that were successfully queried
	
	for result := range resultChan {
		if result.err != nil {
			totalErrors++
			sm.secureLog.SafeError("Batch processing failed", result.err, map[string]interface{}{
				"batch_size": len(result.batch),
			})
			
			// Save failed keywords
			var failedKeywords []string
			for _, keyword := range result.batch {
				if keywordToSpecificURLMap[keyword] != "" {
					failedKeywords = append(failedKeywords, keyword)
				}
			}
			if len(failedKeywords) > 0 {
				if saveErr := sm.simpleTracker.SaveFailedKeywords(ctx, failedKeywords, "", "", result.err); saveErr != nil {
					sm.secureLog.SafeError("Failed to save failed keywords", saveErr, nil)
				}
			}
		} else if result.trendData != nil && len(result.trendData.Keywords) > 0 {
			allTrendData = append(allTrendData, result.trendData.Keywords...)
			successfulKeywords = append(successfulKeywords, result.batch...)
		}
	}
	
	sm.log.WithFields(map[string]interface{}{
		"successful_results": len(allTrendData),
		"failed_batches":    totalErrors,
		"total_batches":     len(batches),
	}).Info("Concurrent API queries completed")
	
	if len(allTrendData) == 0 {
		return fmt.Errorf("no successful trend data retrieved from any batch")
	}
	
	// SEOKey API queries completed
	
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
		// Submit deduplicated results to backend
		
		success := sm.submissionPool.Submit(allBackendData, func(err error) {
			if err != nil {
				sm.secureLog.SafeError("Backend submission failed for deduplicated data", err, nil)
			} else {
				// Backend submission successful
			}
		})
		
		if !success {
			sm.log.Warn("Failed to queue deduplicated data for backend submission")
		}
	}
	
	// ✅ FIX: Save URLs that had successful API queries (URL级别去重 + 防竞态条件)
	if len(successfulKeywords) > 0 {
		// Group successful URLs by sitemap for batch saving
		sitemapURLsMap := make(map[string][]string)
		for _, keyword := range successfulKeywords {
			if specificURL := keywordToSpecificURLMap[keyword]; specificURL != "" {
				if sitemapURL := keywordToSitemapMap[keyword]; sitemapURL != "" {
					sitemapURLsMap[sitemapURL] = append(sitemapURLsMap[sitemapURL], specificURL)
				}
			}
		}
		
		// Save URLs in batches (thread-safe with mutex)
		totalSavedURLs := 0
		for sitemapURL, urls := range sitemapURLsMap {
			if err := sm.simpleTracker.SaveProcessedURLs(ctx, urls, sitemapURL); err != nil {
				sm.secureLog.WarnWithURL("Failed to save successfully queried URLs", sitemapURL, map[string]interface{}{
					"error": err.Error(),
					"url_count": len(urls),
				})
			} else {
				totalSavedURLs += len(urls)
			}
		}
		
		sm.log.WithFields(map[string]interface{}{
			"successful_sitemaps": len(sitemapURLsMap),
			"successful_urls":     totalSavedURLs,
			"successful_keywords": len(successfulKeywords),
		}).Info("✅ Saved successfully queried URLs to avoid reprocessing (URL级别去重)")
	}
	
	return nil
}

// filterUnprocessedSitemaps returns sitemaps that haven't been processed yet
func (sm *SitemapMonitor) filterUnprocessedSitemaps(ctx context.Context, sitemapURLs []string) ([]string, error) {
	var unprocessedSitemaps []string

	for _, sitemapURL := range sitemapURLs {
		processed, err := sm.simpleTracker.IsURLProcessed(ctx, sitemapURL)
		if err != nil {
			sm.secureLog.WarnWithURL("Failed to check if URL is processed", sitemapURL, map[string]interface{}{
				"error": err.Error(),
			})
			// On error, assume not processed to avoid skipping
			unprocessedSitemaps = append(unprocessedSitemaps, sitemapURL)
			continue
		}

		if !processed {
			unprocessedSitemaps = append(unprocessedSitemaps, sitemapURL)
		} else {
			sm.secureLog.DebugWithURL("Skipping already processed sitemap", sitemapURL, nil)
		}
	}

	return unprocessedSitemaps, nil
}

// filterKeywordsForUnprocessedSitemaps filters keywords to only include those from unprocessed sitemaps
func (sm *SitemapMonitor) filterKeywordsForUnprocessedSitemaps(keywords []string, keywordToURLMap map[string]string, unprocessedSitemaps []string) []string {
	// Create a set of unprocessed sitemap URLs for fast lookup
	unprocessedSet := make(map[string]bool)
	for _, sitemapURL := range unprocessedSitemaps {
		unprocessedSet[sitemapURL] = true
	}

	var filteredKeywords []string
	for _, keyword := range keywords {
		if sitemapURL, exists := keywordToURLMap[keyword]; exists {
			if unprocessedSet[sitemapURL] {
				filteredKeywords = append(filteredKeywords, keyword)
			}
		}
	}

	return filteredKeywords
}

// filterUnprocessedKeywordURLs filters keywords to only include those whose associated URLs haven't been processed
// This implements URL-level deduplication to avoid reprocessing already handled URLs
func (sm *SitemapMonitor) filterUnprocessedKeywordURLs(ctx context.Context, keywords []string, keywordToSpecificURLMap map[string]string) ([]string, error) {
	if len(keywords) == 0 {
		return keywords, nil
	}

	// Collect all specific URLs that need to be checked
	urlsToCheck := make([]string, 0, len(keywords))
	keywordToURL := make(map[string]string)
	
	for _, keyword := range keywords {
		if specificURL, exists := keywordToSpecificURLMap[keyword]; exists && specificURL != "" {
			urlsToCheck = append(urlsToCheck, specificURL)
			keywordToURL[keyword] = specificURL
		}
	}

	if len(urlsToCheck) == 0 {
		// No URLs to check, return all keywords
		return keywords, nil
	}

	// Check which URLs have been processed using batch method for efficiency
	urlProcessingStatus, err := sm.simpleTracker.AreURLsProcessed(ctx, urlsToCheck)
	if err != nil {
		return nil, fmt.Errorf("failed to check URL processing status: %w", err)
	}

	// Filter keywords based on URL processing status
	var filteredKeywords []string
	for _, keyword := range keywords {
		specificURL, hasURL := keywordToURL[keyword]
		if !hasURL {
			// No specific URL mapping, include the keyword
			filteredKeywords = append(filteredKeywords, keyword)
			continue
		}

		isProcessed, exists := urlProcessingStatus[specificURL]
		if !exists || !isProcessed {
			// URL not processed or status unknown, include the keyword
			filteredKeywords = append(filteredKeywords, keyword)
		}
		// Skip keywords whose URLs have already been processed
	}

	sm.secureLog.SafeDebug("Filtered keywords based on URL processing status", map[string]interface{}{
		"total_keywords":     len(keywords),
		"urls_checked":       len(urlsToCheck),
		"filtered_keywords":  len(filteredKeywords),
		"deduplication_rate": fmt.Sprintf("%.1f%%", float64(len(filteredKeywords))/float64(len(keywords))*100),
	})

	return filteredKeywords, nil
}

// saveProcessedSitemaps function removed - now saving happens in queryAndSubmitKeywords after API success

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
	
	// Close all resources with panic protection in dependency order
	safeClose("worker pool", func() error {
		if sm.workerPool != nil {
			return sm.workerPool.Stop()
		}
		return nil
	})

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

// processBatchWorker processes batches of keywords concurrently
func (sm *SitemapMonitor) processBatchWorker(ctx context.Context, workerID int, batchChan <-chan []string, resultChan chan<- batchResult, keywordToSpecificURLMap map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()
	
	// Removed worker startup debug logging for cleaner output

	for batch := range batchChan {
		// Check context cancellation
		select {
		case <-ctx.Done():
			resultChan <- batchResult{batch: batch, err: ctx.Err()}
			return
		default:
		}

		// Removed batch processing debug logging for cleaner output
		
		// Query API with API-endpoint-aware rate limiting for optimal dual API utilization
		var trendData *api.APIResponse
		config := sm.concurrencyManager.GetCurrentConfig()

		// Get API endpoint for proper rate limiting
		apiEndpoint := sm.getAPIEndpointForRateLimiting()
		workerRateLimiter := sm.rateLimiterPool.GetOrCreateForAPI(apiEndpoint, config.APIRequestsPerSecond)

		err := workerRateLimiter.Execute(ctx, func() error {
			var queryErr error
			trendData, queryErr = sm.apiClient.Query(ctx, batch)
			return queryErr
		})
		
		// Send result
		resultChan <- batchResult{
			batch:     batch,
			trendData: trendData,
			err:       err,
		}
		
		if err != nil {
			sm.log.WithFields(map[string]interface{}{
				"worker_id":  workerID,
				"batch_size": len(batch),
				"error":      err.Error(),
			}).Warn("Batch query failed")
		}
		// Removed successful batch logging for cleaner output
	}

	// Removed worker completion debug logging for cleaner output
}

// Helper function to parse URL
func parseURL(urlStr string) (*url.URL, error) {
	return url.Parse(urlStr)
}

// ExportDataSummary exports data summary for GitHub Actions
func (sm *SitemapMonitor) ExportDataSummary(ctx context.Context, outputDir string) error {
	exporter := storage.NewDataExporter(sm.storage)
	return exporter.ExportReport(ctx, outputDir)
}

// configureAtomicConcurrencyControl sets up atomic concurrency control for API clients
// Inspired by 1.js distributed lock mechanism for precise concurrent request management
func (sm *SitemapMonitor) configureAtomicConcurrencyControl(apiURL string) {
	// Get current concurrency configuration
	config := sm.concurrencyManager.GetCurrentConfig()
	_ = config // Use config if needed for dynamic adjustment

	// Create atomic limiter for primary API endpoint using configuration
	primaryLimiter := sm.rateLimiterPool.GetOrCreateAtomicLimiter(
		apiURL, config.MaxConcurrentPerAPI, config.ConcurrencyTimeout)

	// Configure API client with atomic concurrency control
	if configurable, ok := sm.apiClient.(api.ConcurrencyConfigurable); ok {
		adapter := NewAtomicLimiterAdapter(primaryLimiter)
		configurable.SetConcurrencyLimiter(adapter)

		sm.log.WithFields(map[string]interface{}{
			"api_endpoint":     sm.maskAPIEndpoint(apiURL),
			"max_concurrent":   config.MaxConcurrentPerAPI,
			"acquire_timeout":  config.ConcurrencyTimeout,
		}).Info("Atomic concurrency control configured for API client")
	} else {
		sm.log.Warn("API client does not support concurrency configuration")
	}

	// If secondary API is configured, set up its concurrency control too
	if secondaryURL := os.Getenv("TRENDS_API_URL_SECONDARY"); secondaryURL != "" && secondaryURL != apiURL {
		_ = sm.rateLimiterPool.GetOrCreateAtomicLimiter(
			secondaryURL, config.MaxConcurrentPerAPI, config.ConcurrencyTimeout)

		sm.log.WithFields(map[string]interface{}{
			"secondary_api_endpoint": sm.maskAPIEndpoint(secondaryURL),
			"max_concurrent":         config.MaxConcurrentPerAPI,
			"acquire_timeout":        config.ConcurrencyTimeout,
		}).Info("Secondary API atomic concurrency control configured")
	}
}

// maskAPIEndpoint masks API endpoint for secure logging
func (sm *SitemapMonitor) maskAPIEndpoint(endpoint string) string {
	if len(endpoint) > 20 {
		return endpoint[:10] + "***" + endpoint[len(endpoint)-7:]
	}
	return "***"
}