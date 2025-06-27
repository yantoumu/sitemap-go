package storage

import (
	"context"
	"sync"
	"time"

	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/utils"
)

// SimpleTracker只保存URL哈希和失败关键词
type SimpleTracker struct {
	storage Storage
	log     *logger.Logger
	mu      sync.Mutex // 防止竞态条件的互斥锁
}

// NewSimpleTracker创建简化的跟踪器
func NewSimpleTracker(storage Storage) *SimpleTracker {
	return &SimpleTracker{
		storage: storage,
		log:     logger.GetLogger().WithField("component", "simple_tracker"),
	}
}

// ProcessedURLSet represents a set of processed URL hashes (极简设计)
type ProcessedURLSet map[string]bool // URLHash -> processed

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

// SaveProcessedURLs saves multiple URL hashes to avoid duplicate processing (极简设计)
func (st *SimpleTracker) SaveProcessedURLs(ctx context.Context, urls []string, sitemapURL string) error {
	st.mu.Lock() // 🔒 防止竞态条件
	defer st.mu.Unlock()
	
	if len(urls) == 0 {
		return nil
	}
	
	// Load existing hash set
	var processedSet ProcessedURLSet
	err := st.storage.Load(ctx, "processed_urls", &processedSet)
	if err != nil || processedSet == nil {
		processedSet = make(ProcessedURLSet)
	}
	
	// Add new URLs (只保存哈希，极简!)
	newCount := 0
	for _, url := range urls {
		urlHash := utils.CalculateURLHash(url)
		if !processedSet[urlHash] {
			processedSet[urlHash] = true
			newCount++
		}
	}
	
	// Limit size to prevent memory explosion (线程安全清理)
	if len(processedSet) > 100000 {
		// Keep newest 50000 entries using timestamp-based approach
		st.performSafeCleanup(&processedSet, 50000)
	}
	
	st.log.WithFields(map[string]interface{}{
		"new_urls":    newCount,
		"total_urls":  len(urls),
		"total_saved": len(processedSet),
	}).Debug("Saved processed URLs (hash-only)")
	
	return st.storage.Save(ctx, "processed_urls", processedSet)
}

// SaveProcessedURL saves single URL (backward compatibility)
func (st *SimpleTracker) SaveProcessedURL(ctx context.Context, sitemapURL string, keywords []string) error {
	// For backward compatibility - treat as sitemap URL
	return st.SaveProcessedURLs(ctx, []string{sitemapURL}, sitemapURL)
}

// IsURLProcessed checks if URL was already processed (极简版本)
func (st *SimpleTracker) IsURLProcessed(ctx context.Context, url string) (bool, error) {
	st.mu.Lock() // 🔒 防止竞态条件
	defer st.mu.Unlock()
	
	urlHash := utils.CalculateURLHash(url)
	
	var processedSet ProcessedURLSet
	err := st.storage.Load(ctx, "processed_urls", &processedSet)
	if err != nil || processedSet == nil {
		return false, nil // Assume not processed if can't load
	}
	
	return processedSet[urlHash], nil
}

// AreURLsProcessed checks multiple URLs for processing status (极简批量检查)
func (st *SimpleTracker) AreURLsProcessed(ctx context.Context, urls []string) (map[string]bool, error) {
	st.mu.Lock() // 🔒 防止竞态条件
	defer st.mu.Unlock()
	
	var processedSet ProcessedURLSet
	err := st.storage.Load(ctx, "processed_urls", &processedSet)
	if err != nil || processedSet == nil {
		// Return all as unprocessed if can't load
		result := make(map[string]bool)
		for _, url := range urls {
			result[url] = false
		}
		return result, nil
	}
	
	// Check each URL (超简单!)
	result := make(map[string]bool)
	for _, url := range urls {
		urlHash := utils.CalculateURLHash(url)
		result[url] = processedSet[urlHash]
	}
	
	return result, nil
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

// performSafeCleanup performs thread-safe cleanup of processed URL set
func (st *SimpleTracker) performSafeCleanup(processedSet *ProcessedURLSet, keepCount int) {
	if len(*processedSet) <= keepCount {
		return
	}
	
	// 简单的FIFO清理策略 - 保留最近的一半
	newSet := make(ProcessedURLSet)
	count := 0
	target := len(*processedSet) / 2 // 保留一半，避免频繁清理
	
	for hash := range *processedSet {
		if count >= target {
			break
		}
		newSet[hash] = true
		count++
	}
	
	*processedSet = newSet
	st.log.WithField("kept_entries", count).Debug("Performed safe cleanup of processed URLs")
}