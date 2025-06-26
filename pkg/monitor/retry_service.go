package monitor

import (
	"context"
	"time"

	"sitemap-go/pkg/api"
	"sitemap-go/pkg/backend"
	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/storage"
)

// RetryService handles failed keyword retry and submission
type RetryService struct {
	apiClient       api.APIClient
	simpleTracker   *storage.SimpleTracker
	submissionPool  *backend.SubmissionPool
	dataConverter   *backend.DataConverter
	log             *logger.Logger
	stopChannel     chan struct{}
	retryInterval   time.Duration
}

// NewRetryService creates a new retry service
func NewRetryService(
	apiClient api.APIClient,
	simpleTracker *storage.SimpleTracker,
	submissionPool *backend.SubmissionPool,
	dataConverter *backend.DataConverter,
) *RetryService {
	return &RetryService{
		apiClient:     apiClient,
		simpleTracker: simpleTracker,
		submissionPool: submissionPool,
		dataConverter: dataConverter,
		log:           logger.GetLogger().WithField("component", "retry_service"),
		stopChannel:   make(chan struct{}),
		retryInterval: 10 * time.Minute, // Check every 10 minutes
	}
}

// Start starts the retry service in background
func (rs *RetryService) Start(ctx context.Context) {
	rs.log.Info("Starting retry service for failed keywords")
	go rs.retryLoop(ctx)
}

// Stop stops the retry service
func (rs *RetryService) Stop() {
	rs.log.Info("Stopping retry service")
	close(rs.stopChannel)
}

// retryLoop continuously checks and retries failed keywords
func (rs *RetryService) retryLoop(ctx context.Context) {
	ticker := time.NewTicker(rs.retryInterval)
	defer ticker.Stop()
	
	// Initial retry on startup
	rs.processRetryableKeywords(ctx)
	
	for {
		select {
		case <-ticker.C:
			rs.processRetryableKeywords(ctx)
		case <-rs.stopChannel:
			rs.log.Debug("Retry loop stopping")
			return
		case <-ctx.Done():
			rs.log.Debug("Retry loop context cancelled")
			return
		}
	}
}

// processRetryableKeywords processes keywords ready for retry
func (rs *RetryService) processRetryableKeywords(ctx context.Context) {
	retryableKeywords, err := rs.simpleTracker.GetRetryableKeywords(ctx)
	if err != nil {
		rs.log.WithError(err).Error("Failed to get retryable keywords")
		return
	}
	
	if len(retryableKeywords) == 0 {
		rs.log.Debug("No keywords ready for retry")
		return
	}
	
	rs.log.WithField("retryable_count", len(retryableKeywords)).Info("Processing retryable keywords")
	
	// Group keywords by sitemap for batch processing
	keywordsBySitemap := make(map[string][]storage.FailedKeywordRecord)
	for _, keyword := range retryableKeywords {
		keywordsBySitemap[keyword.SitemapURL] = append(keywordsBySitemap[keyword.SitemapURL], keyword)
	}
	
	var successfulKeywords []string
	var stillFailedKeywords []storage.FailedKeywordRecord
	
	// Process each sitemap group
	for sitemapURL, keywords := range keywordsBySitemap {
		rs.log.WithFields(map[string]interface{}{
			"sitemap_url":    sitemapURL,
			"keyword_count":  len(keywords),
		}).Debug("Retrying keywords for sitemap")
		
		// Extract keyword strings for API query
		keywordStrings := make([]string, len(keywords))
		for i, kw := range keywords {
			keywordStrings[i] = kw.Keyword
		}
		
		// Query Google Trends API
		trendData, err := rs.apiClient.Query(ctx, keywordStrings)
		if err != nil {
			rs.log.WithError(err).WithField("sitemap_url", sitemapURL).Warn("Retry API query failed")
			
			// Update failed keywords with new retry info
			for _, keyword := range keywords {
				keyword.RetryCount++
				keyword.LastError = err.Error()
				keyword.FailedAt = time.Now()
				stillFailedKeywords = append(stillFailedKeywords, keyword)
			}
			continue
		}
		
		rs.log.WithFields(map[string]interface{}{
			"sitemap_url":   sitemapURL,
			"keyword_count": len(keywords),
		}).Info("Retry API query successful")
		
		// Convert successful data to backend format
		backendData := rs.convertToBackendFormat(trendData, keywords)
		
		// Submit to backend via non-blocking pool
		if len(backendData) > 0 {
			success := rs.submissionPool.Submit(backendData, func(submitErr error) {
				if submitErr != nil {
					rs.log.WithError(submitErr).Error("Backend submission failed for retry data")
				} else {
					rs.log.WithField("data_count", len(backendData)).Info("Retry data submitted successfully")
				}
			})
			
			if !success {
				rs.log.Warn("Failed to queue retry data for submission")
			}
		}
		
		// Mark keywords as successful
		for _, keyword := range keywords {
			successfulKeywords = append(successfulKeywords, keyword.Keyword)
		}
	}
	
	// Update failed keywords storage
	if len(stillFailedKeywords) > 0 {
		// Save updated failed keywords
		if err := rs.saveUpdatedFailedKeywords(ctx, stillFailedKeywords); err != nil {
			rs.log.WithError(err).Error("Failed to update failed keywords")
		}
	}
	
	// Remove successful keywords
	if len(successfulKeywords) > 0 {
		if err := rs.simpleTracker.RemoveSuccessfulKeywords(ctx, successfulKeywords); err != nil {
			rs.log.WithError(err).Error("Failed to remove successful keywords")
		}
	}
	
	rs.log.WithFields(map[string]interface{}{
		"successful_retries": len(successfulKeywords),
		"still_failed":       len(stillFailedKeywords),
	}).Info("Retry processing completed")
}

// convertToBackendFormat converts API response to backend submission format
func (rs *RetryService) convertToBackendFormat(trendData *api.APIResponse, keywords []storage.FailedKeywordRecord) []backend.KeywordMetricsData {
	var backendData []backend.KeywordMetricsData
	
	// Create a map for keyword lookup
	keywordMap := make(map[string]storage.FailedKeywordRecord)
	for _, kw := range keywords {
		keywordMap[kw.Keyword] = kw
	}
	
	// Convert each trend keyword
	for _, trendKeyword := range trendData.Keywords {
		if keywordRecord, exists := keywordMap[trendKeyword.Word]; exists {
			metrics := rs.dataConverter.ConvertKeywordMetrics(trendKeyword)
			
			backendData = append(backendData, backend.KeywordMetricsData{
				Keyword: trendKeyword.Word,
				URL:     keywordRecord.SourceURL,
				Metrics: metrics,
			})
		}
	}
	
	return backendData
}

// saveUpdatedFailedKeywords saves updated failed keywords back to storage
func (rs *RetryService) saveUpdatedFailedKeywords(ctx context.Context, failedKeywords []storage.FailedKeywordRecord) error {
	// This is a simplified approach - in a real implementation, you'd want to
	// merge with existing failed keywords more carefully
	return nil // Placeholder - implement based on storage interface
}