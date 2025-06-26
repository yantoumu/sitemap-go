package monitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sitemap-go/pkg/api"
	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/storage"
)

// RetryManager handles background retry of failed keyword queries
type RetryManager struct {
	apiClient    api.APIClient
	storage      storage.Storage
	queryTracker *storage.QueryTracker
	log          *logger.Logger
}

// NewRetryManager creates a new retry manager
func NewRetryManager(apiClient api.APIClient, storage storage.Storage, queryTracker *storage.QueryTracker) *RetryManager {
	return &RetryManager{
		apiClient:    apiClient,
		storage:      storage,
		queryTracker: queryTracker,
		log:          logger.GetLogger().WithField("component", "retry_manager"),
	}
}

// RetryFailedKeywords runs in background to retry failed API queries
func (rm *RetryManager) RetryFailedKeywords(ctx context.Context) {
	rm.log.Debug("Starting background retry of failed keywords")
	
	// Get keywords ready for retry
	retryableKeywords, err := rm.queryTracker.GetRetryableKeywords(ctx)
	if err != nil {
		rm.log.WithError(err).Error("Failed to get retryable keywords")
		return
	}
	
	if len(retryableKeywords) == 0 {
		rm.log.Debug("No keywords ready for retry")
		return
	}
	
	rm.log.WithField("retry_count", len(retryableKeywords)).Info("Retrying failed keywords")
	
	// Group keywords by sitemap for batch processing
	keywordsByMap := make(map[string][]string)
	failedKeywordMap := make(map[string]storage.FailedKeyword)
	
	for _, fk := range retryableKeywords {
		keywordsByMap[fk.SitemapURL] = append(keywordsByMap[fk.SitemapURL], fk.Keyword)
		failedKeywordMap[fk.Keyword] = fk
	}
	
	var successfulKeywords []string
	var stillFailedKeywords []storage.FailedKeyword
	
	// Retry keywords for each sitemap
	for sitemapURL, keywords := range keywordsByMap {
		rm.log.WithFields(map[string]interface{}{
			"sitemap_url":    sitemapURL,
			"keyword_count":  len(keywords),
		}).Debug("Retrying keywords for sitemap")
		
		// Query API for these keywords
		trendData, err := rm.apiClient.Query(ctx, keywords)
		if err != nil {
			rm.log.WithError(err).Warn("Retry failed for keywords")
			
			// Update failed keywords with new retry info
			for _, keyword := range keywords {
				fk := failedKeywordMap[keyword]
				fk.RetryCount++
				fk.LastError = err.Error()
				fk.FailedAt = time.Now()
				
				// Keep failed keywords even after retry limit for next normal query
				stillFailedKeywords = append(stillFailedKeywords, fk)
				
				if fk.RetryCount > 3 {
					rm.log.WithField("keyword", keyword).Info("Keyword exceeded retry limit, will try again in next normal monitoring")
				}
			}
		} else {
			rm.log.WithFields(map[string]interface{}{
				"sitemap_url":   sitemapURL,
				"keyword_count": len(keywords),
			}).Info("Keywords retry successful")
			
			successfulKeywords = append(successfulKeywords, keywords...)
			
			// Store successful trend data
			retryResultKey := fmt.Sprintf("retry_result_%s_%d", 
				strings.ReplaceAll(sitemapURL, "/", "_"), time.Now().Unix())
			if storeErr := rm.storage.Save(ctx, retryResultKey, trendData); storeErr != nil {
				rm.log.WithError(storeErr).Warn("Failed to store retry result")
			}
		}
	}
	
	// Update failed keywords list
	if len(stillFailedKeywords) > 0 {
		if err := rm.queryTracker.SaveFailedKeywords(ctx, stillFailedKeywords); err != nil {
			rm.log.WithError(err).Error("Failed to update failed keywords")
		}
	}
	
	// Remove successful keywords from failed list
	if len(successfulKeywords) > 0 {
		if err := rm.queryTracker.RemoveSuccessfulKeywords(ctx, successfulKeywords); err != nil {
			rm.log.WithError(err).Error("Failed to remove successful keywords")
		}
	}
	
	rm.log.WithFields(map[string]interface{}{
		"successful_retries": len(successfulKeywords),
		"still_failed":       len(stillFailedKeywords),
	}).Info("Background keyword retry completed")
}

// SaveFailedKeywords saves keywords that failed API queries
func (rm *RetryManager) SaveFailedKeywords(ctx context.Context, sitemapURL string, keywords []string, err error) error {
	if !rm.isRetryableError(err) {
		rm.log.WithError(err).Debug("Error is not retryable, not saving keywords")
		return nil
	}
	
	rm.log.WithError(err).WithFields(map[string]interface{}{
		"sitemap_url":    sitemapURL,
		"keyword_count":  len(keywords),
	}).Warn("API query failed with retryable error, saving for retry")
	
	// Create failed keywords
	failedKeywords := make([]storage.FailedKeyword, len(keywords))
	for i, keyword := range keywords {
		failedKeywords[i] = storage.FailedKeyword{
			Keyword:    keyword,
			SitemapURL: sitemapURL,
			FailedAt:   time.Now(),
			LastError:  err.Error(),
		}
	}
	
	// Save to tracker
	if saveErr := rm.queryTracker.SaveFailedKeywords(ctx, failedKeywords); saveErr != nil {
		rm.log.WithError(saveErr).Error("Failed to save failed keywords")
		return saveErr
	}
	
	rm.log.WithField("failed_keywords", len(keywords)).Info("Failed keywords saved for retry")
	return nil
}

// RemoveSuccessfulKeywords removes keywords that were successfully queried
func (rm *RetryManager) RemoveSuccessfulKeywords(ctx context.Context, keywords []string) {
	if len(keywords) == 0 {
		return
	}
	
	if err := rm.queryTracker.RemoveSuccessfulKeywords(ctx, keywords); err != nil {
		rm.log.WithError(err).Debug("Failed to remove successful keywords from retry list")
	}
}

// isRetryableError checks if an error should trigger a retry
func (rm *RetryManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := err.Error()
	
	// HTTP 500 Internal Server Error
	if strings.Contains(errorStr, "500") || 
	   strings.Contains(errorStr, "Internal Server Error") {
		return true
	}
	
	// HTTP 429 Too Many Requests (Rate Limiting)
	if strings.Contains(errorStr, "429") || 
	   strings.Contains(errorStr, "Too Many Requests") ||
	   strings.Contains(errorStr, "rate limit") {
		return true
	}
	
	// Network timeout errors
	if strings.Contains(errorStr, "timeout") ||
	   strings.Contains(errorStr, "context deadline exceeded") ||
	   strings.Contains(errorStr, "connection timeout") {
		return true
	}
	
	// API service unavailable
	if strings.Contains(errorStr, "503") ||
	   strings.Contains(errorStr, "Service Unavailable") {
		return true
	}
	
	// Network connection errors
	if strings.Contains(errorStr, "connection refused") ||
	   strings.Contains(errorStr, "no such host") ||
	   strings.Contains(errorStr, "network unreachable") {
		return true
	}
	
	return false
}