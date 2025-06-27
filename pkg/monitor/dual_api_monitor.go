package monitor

import (
	"fmt"
	"sitemap-go/pkg/api"
	"sitemap-go/pkg/backend"
	"sitemap-go/pkg/extractor"
	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/storage"
	"sitemap-go/pkg/worker"
	"time"
)

// NewMonitorWithDualAPI creates a monitor with dual API support for load balancing and failover
func NewMonitorWithDualAPI(config MonitorConfig, backendURL, apiKey string, batchSize int, primaryAPI, secondaryAPI string) (*SitemapMonitor, error) {
	// Initialize components
	parserFactory := parser.GetParserFactory()
	keywordExtractor := extractor.NewURLKeywordExtractor()
	
	// Create dual API client for load balancing
	dualAPIClient := api.NewDualAPIClient(primaryAPI, secondaryAPI)
	
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
	poolConfig.Workers = config.WorkerPoolSize
	if poolConfig.Workers == 0 {
		poolConfig.Workers = 8
	}
	workerPool := worker.NewConcurrentPool(poolConfig)
	
	// Create simplified tracker
	simpleTracker := storage.NewSimpleTracker(storageService)
	
	// Create backend client
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
	retryProcessor := NewSimpleRetryProcessor(dualAPIClient, simpleTracker, submissionPool, dataConverter)
	
	// Create adaptive concurrency manager
	concurrencyConfig := DefaultConcurrencyConfig()
	concurrencyManager := NewAdaptiveConcurrencyManager(concurrencyConfig)
	
	// Create rate limiter
	rateLimiter := NewRateLimitedExecutor(concurrencyConfig.SitemapRequestsPerSecond)
	
	// Create rate limiter pool
	rateLimiterPool := NewRateLimiterPool()
	
	return &SitemapMonitor{
		parserFactory:      parserFactory,
		keywordExtractor:   keywordExtractor,
		apiClient:          dualAPIClient,
		storage:            storageService,
		workerPool:         workerPool,
		simpleTracker:      simpleTracker,
		submissionPool:     submissionPool,
		retryProcessor:     retryProcessor,
		dataConverter:      dataConverter,
		concurrencyManager: concurrencyManager,
		rateLimiter:        rateLimiter,
		rateLimiterPool:    rateLimiterPool,
		apiExecutor:        api.NewSequentialExecutor(),
		log:                logger.GetLogger().WithField("component", "sitemap_monitor"),
		secureLog:          logger.GetSecurityLogger(),
	}, nil
}