package storage

import (
	"context"
	"time"

	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/utils"
)

// SimpleTracker只保存URL哈希和失败关键词
type SimpleTracker struct {
	storage Storage
	log     *logger.Logger
}

// NewSimpleTracker创建简化的跟踪器
func NewSimpleTracker(storage Storage) *SimpleTracker {
	return &SimpleTracker{
		storage: storage,
		log:     logger.GetLogger().WithField("component", "simple_tracker"),
	}
}

// URLHashRecord represents a processed URL hash record
type URLHashRecord struct {
	URLHash     string    `json:"url_hash"`
	SitemapURL  string    `json:"sitemap_url"`
	ProcessedAt time.Time `json:"processed_at"`
}

// FailedKeywordRecord represents a failed keyword for retry
type FailedKeywordRecord struct {
	Keyword     string    `json:"keyword"`
	SourceURL   string    `json:"source_url"`
	SitemapURL  string    `json:"sitemap_url"`
	FailedAt    time.Time `json:"failed_at"`
	RetryCount  int       `json:"retry_count"`
	LastError   string    `json:"last_error"`
	NextRetryAt time.Time `json:"next_retry_at"`
}

// SaveProcessedURL saves URL hash to avoid duplicate processing
func (st *SimpleTracker) SaveProcessedURL(ctx context.Context, sitemapURL string, keywords []string) error {
	urlHash := utils.CalculateURLHash(sitemapURL)

	record := URLHashRecord{
		URLHash:     urlHash,
		SitemapURL:  sitemapURL,
		ProcessedAt: time.Now(),
	}
	
	// Load existing hashes
	var existingHashes []URLHashRecord
	_ = st.storage.Load(ctx, "processed_urls", &existingHashes)
	
	// Check if already exists
	for _, existing := range existingHashes {
		if existing.URLHash == urlHash {
			st.log.WithField("url_hash", urlHash).Debug("URL already processed, skipping")
			return nil
		}
	}
	
	// Add new hash
	existingHashes = append(existingHashes, record)
	
	// Keep only recent hashes (last 10000)
	if len(existingHashes) > 10000 {
		existingHashes = existingHashes[len(existingHashes)-10000:]
	}
	
	st.log.WithField("url_hash", urlHash).Debug("Saving processed URL hash")
	return st.storage.Save(ctx, "processed_urls", existingHashes)
}

// IsURLProcessed checks if URL was already processed
func (st *SimpleTracker) IsURLProcessed(ctx context.Context, sitemapURL string) (bool, error) {
	urlHash := utils.CalculateURLHash(sitemapURL)
	
	var existingHashes []URLHashRecord
	err := st.storage.Load(ctx, "processed_urls", &existingHashes)
	if err != nil {
		return false, nil // Assume not processed if can't load
	}
	
	for _, existing := range existingHashes {
		if existing.URLHash == urlHash {
			st.log.WithField("url_hash", urlHash).Debug("URL already processed")
			return true, nil
		}
	}
	
	return false, nil
}

// SaveFailedKeywords saves failed keywords for retry
func (st *SimpleTracker) SaveFailedKeywords(ctx context.Context, keywords []string, sourceURL, sitemapURL string, err error) error {
	if len(keywords) == 0 {
		return nil
	}
	
	// Load existing failed keywords
	var existingFailed []FailedKeywordRecord
	_ = st.storage.Load(ctx, "failed_keywords", &existingFailed)
	
	failedMap := make(map[string]FailedKeywordRecord)
	for _, failed := range existingFailed {
		failedMap[failed.Keyword] = failed
	}
	
	// Add or update failed keywords
	now := time.Now()
	for _, keyword := range keywords {
		if existing, exists := failedMap[keyword]; exists {
			// Update existing record
			existing.RetryCount++
			existing.LastError = err.Error()
			existing.FailedAt = now
			existing.NextRetryAt = st.calculateNextRetryTime(existing.RetryCount)
			failedMap[keyword] = existing
		} else {
			// Create new record
			failedMap[keyword] = FailedKeywordRecord{
				Keyword:     keyword,
				SourceURL:   sourceURL,
				SitemapURL:  sitemapURL,
				FailedAt:    now,
				RetryCount:  1,
				LastError:   err.Error(),
				NextRetryAt: st.calculateNextRetryTime(1),
			}
		}
	}
	
	// Convert back to slice
	var updatedFailed []FailedKeywordRecord
	for _, failed := range failedMap {
		updatedFailed = append(updatedFailed, failed)
	}
	
	st.log.WithField("failed_keywords", len(keywords)).Debug("Saved failed keywords for retry")
	return st.storage.Save(ctx, "failed_keywords", updatedFailed)
}

// GetRetryableKeywords gets keywords ready for retry
func (st *SimpleTracker) GetRetryableKeywords(ctx context.Context) ([]FailedKeywordRecord, error) {
	var failedKeywords []FailedKeywordRecord
	err := st.storage.Load(ctx, "failed_keywords", &failedKeywords)
	if err != nil {
		return []FailedKeywordRecord{}, nil
	}
	
	var retryable []FailedKeywordRecord
	now := time.Now()
	
	for _, failed := range failedKeywords {
		if now.After(failed.NextRetryAt) {
			retryable = append(retryable, failed)
		}
	}
	
	st.log.WithFields(map[string]interface{}{
		"total_failed": len(failedKeywords),
		"retryable":    len(retryable),
	}).Info("Retrieved retryable keywords")
	
	return retryable, nil
}

// RemoveSuccessfulKeywords removes keywords that were successfully processed
func (st *SimpleTracker) RemoveSuccessfulKeywords(ctx context.Context, successfulKeywords []string) error {
	if len(successfulKeywords) == 0 {
		return nil
	}
	
	var failedKeywords []FailedKeywordRecord
	err := st.storage.Load(ctx, "failed_keywords", &failedKeywords)
	if err != nil {
		return nil
	}
	
	successSet := make(map[string]bool)
	for _, keyword := range successfulKeywords {
		successSet[keyword] = true
	}
	
	var remaining []FailedKeywordRecord
	removedCount := 0
	
	for _, failed := range failedKeywords {
		if !successSet[failed.Keyword] {
			remaining = append(remaining, failed)
		} else {
			removedCount++
		}
	}
	
	st.log.WithFields(map[string]interface{}{
		"removed":   removedCount,
		"remaining": len(remaining),
	}).Info("Removed successful keywords from failed list")
	
	return st.storage.Save(ctx, "failed_keywords", remaining)
}



// calculateNextRetryTime calculates when to retry based on attempt count
func (st *SimpleTracker) calculateNextRetryTime(retryCount int) time.Time {
	delays := []time.Duration{
		5 * time.Minute,   // 5分钟
		15 * time.Minute,  // 15分钟
		60 * time.Minute,  // 1小时
		4 * time.Hour,     // 4小时
		24 * time.Hour,    // 24小时
	}
	
	if retryCount <= len(delays) {
		return time.Now().Add(delays[retryCount-1])
	}
	
	// Default to 24 hours for retries beyond limit
	return time.Now().Add(24 * time.Hour)
}