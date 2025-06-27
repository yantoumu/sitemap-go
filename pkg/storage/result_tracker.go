package storage

import (
	"context"
	"fmt"
	"sort"
	"time"

	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/utils"
)

// QueryState represents the state of a URL query
type QueryState struct {
	URL        string    `json:"url"`
	Hash       string    `json:"hash"`
	Keywords   []string  `json:"keywords"`
	Queried    bool      `json:"queried"`
	Success    bool      `json:"success"`
	Timestamp  time.Time `json:"timestamp"`
	RetryCount int       `json:"retry_count"`
}

// FailedKeyword represents a failed keyword query
type FailedKeyword struct {
	Keyword     string    `json:"keyword"`
	SitemapURL  string    `json:"sitemap_url"`
	FailedAt    time.Time `json:"failed_at"`
	RetryCount  int       `json:"retry_count"`
	LastError   string    `json:"last_error"`
	NextRetryAt time.Time `json:"next_retry_at"`
}

// QueryTracker manages query state and failed keyword tracking
type QueryTracker struct {
	storage Storage
	log     *logger.Logger
}

// NewQueryTracker creates a new query tracker
func NewQueryTracker(storage Storage) *QueryTracker {
	return &QueryTracker{
		storage: storage,
		log:     logger.GetLogger().WithField("component", "query_tracker"),
	}
}

// SaveQueryStates saves URL query states with hash comparison
func (qt *QueryTracker) SaveQueryStates(ctx context.Context, domain string, states []QueryState) error {
	key := fmt.Sprintf("query_states:%s", domain)
	
	qt.log.WithFields(map[string]interface{}{
		"domain":      domain,
		"state_count": len(states),
	}).Debug("Saving query states")
	
	return qt.storage.Save(ctx, key, states)
}

// GetQueryStates retrieves URL query states for a domain
func (qt *QueryTracker) GetQueryStates(ctx context.Context, domain string) ([]QueryState, error) {
	key := fmt.Sprintf("query_states:%s", domain)
	
	var states []QueryState
	err := qt.storage.Load(ctx, key, &states)
	if err != nil {
		// Return empty slice if no states exist
		return []QueryState{}, nil
	}
	
	qt.log.WithFields(map[string]interface{}{
		"domain":      domain,
		"state_count": len(states),
	}).Debug("Retrieved query states")
	
	return states, nil
}

// CalculateURLKeywordHash generates a hash for URL and keywords combination
// This is different from utils.CalculateURLHash which only hashes URLs
func (qt *QueryTracker) CalculateURLKeywordHash(url string, keywords []string) string {
	// Sort keywords for consistent hash
	sortedKeywords := make([]string, len(keywords))
	copy(sortedKeywords, keywords)
	sort.Strings(sortedKeywords)
	
	// Create deterministic string and use unified hash utility
	data := fmt.Sprintf("%s:%v", url, sortedKeywords)
	return utils.CalculateURLHash(data) // Use unified hash utility
}

// CompareWithPrevious compares current URLs with previous query states
func (qt *QueryTracker) CompareWithPrevious(ctx context.Context, domain string, currentURLs map[string][]string) ([]string, []string, error) {
	previousStates, err := qt.GetQueryStates(ctx, domain)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get previous states: %w", err)
	}
	
	// Create hash maps for comparison
	previousHashes := make(map[string]QueryState)
	for _, state := range previousStates {
		previousHashes[state.Hash] = state
	}
	
	var newURLs []string      // URLs not queried before
	var changedURLs []string  // URLs with different keywords
	
	for url, keywords := range currentURLs {
		currentHash := qt.CalculateURLKeywordHash(url, keywords)
		
		if prevState, exists := previousHashes[currentHash]; exists {
			// URL with same keywords exists and was queried successfully
			if prevState.Queried && prevState.Success {
				continue // Skip already processed URLs
			}
			// Mark for retry if previous attempt failed
			changedURLs = append(changedURLs, url)
		} else {
			// New URL or URL with different keywords
			newURLs = append(newURLs, url)
		}
	}
	
	qt.log.WithFields(map[string]interface{}{
		"domain":      domain,
		"new_urls":    len(newURLs),
		"changed_urls": len(changedURLs),
		"previous_states": len(previousStates),
	}).Info("Compared URLs with previous states")
	
	return newURLs, changedURLs, nil
}

// SaveFailedKeywords saves keywords that failed API queries
func (qt *QueryTracker) SaveFailedKeywords(ctx context.Context, failedKeywords []FailedKeyword) error {
	key := "failed_keywords"
	
	// Load existing failed keywords
	var existing []FailedKeyword
	_ = qt.storage.Load(ctx, key, &existing) // Ignore error if file doesn't exist
	
	// Merge with new failed keywords
	keywordMap := make(map[string]FailedKeyword)
	
	// Add existing keywords
	for _, fk := range existing {
		keywordMap[fk.Keyword] = fk
	}
	
	// Update or add new failed keywords
	for _, fk := range failedKeywords {
		if existing, exists := keywordMap[fk.Keyword]; exists {
			// Update retry count and next retry time
			existing.RetryCount++
			existing.LastError = fk.LastError
			existing.FailedAt = fk.FailedAt
			existing.NextRetryAt = qt.calculateNextRetryTime(existing.RetryCount)
			keywordMap[fk.Keyword] = existing
		} else {
			// New failed keyword
			fk.RetryCount = 1
			fk.NextRetryAt = qt.calculateNextRetryTime(1)
			keywordMap[fk.Keyword] = fk
		}
	}
	
	// Convert back to slice - keep all failed keywords for next normal monitoring
	var allFailed []FailedKeyword
	for _, fk := range keywordMap {
		allFailed = append(allFailed, fk)
	}
	
	qt.log.WithFields(map[string]interface{}{
		"new_failed":   len(failedKeywords),
		"total_failed": len(allFailed),
	}).Info("Saved failed keywords")
	
	return qt.storage.Save(ctx, key, allFailed)
}

// GetRetryableKeywords gets keywords ready for retry
func (qt *QueryTracker) GetRetryableKeywords(ctx context.Context) ([]FailedKeyword, error) {
	key := "failed_keywords"
	
	var failed []FailedKeyword
	err := qt.storage.Load(ctx, key, &failed)
	if err != nil {
		return []FailedKeyword{}, nil // Return empty if no failed keywords
	}
	
	// Filter keywords ready for retry (only those under retry limit)
	var retryable []FailedKeyword
	now := time.Now()
	
	for _, fk := range failed {
		// Only retry if under limit and time has passed
		if now.After(fk.NextRetryAt) && fk.RetryCount <= 3 {
			retryable = append(retryable, fk)
		}
	}
	
	qt.log.WithFields(map[string]interface{}{
		"total_failed": len(failed),
		"retryable":    len(retryable),
	}).Info("Retrieved retryable keywords")
	
	return retryable, nil
}

// RemoveSuccessfulKeywords removes keywords that were successfully retried
func (qt *QueryTracker) RemoveSuccessfulKeywords(ctx context.Context, successfulKeywords []string) error {
	key := "failed_keywords"
	
	var failed []FailedKeyword
	err := qt.storage.Load(ctx, key, &failed)
	if err != nil {
		return nil // Nothing to remove
	}
	
	// Create set of successful keywords
	successSet := make(map[string]bool)
	for _, keyword := range successfulKeywords {
		successSet[keyword] = true
	}
	
	// Filter out successful keywords
	var remaining []FailedKeyword
	removedCount := 0
	
	for _, fk := range failed {
		if !successSet[fk.Keyword] {
			remaining = append(remaining, fk)
		} else {
			removedCount++
		}
	}
	
	qt.log.WithFields(map[string]interface{}{
		"removed":   removedCount,
		"remaining": len(remaining),
	}).Info("Removed successful keywords from failed list")
	
	return qt.storage.Save(ctx, key, remaining)
}

// calculateNextRetryTime calculates when to retry based on attempt count
func (qt *QueryTracker) calculateNextRetryTime(retryCount int) time.Time {
	// Exponential backoff: 5min, 15min, 1hour
	delays := []time.Duration{
		5 * time.Minute,   // First retry after 5 minutes
		15 * time.Minute,  // Second retry after 15 minutes  
		60 * time.Minute,  // Third retry after 1 hour
	}
	
	if retryCount <= len(delays) {
		return time.Now().Add(delays[retryCount-1])
	}
	
	// Default to 1 hour for any additional retries
	return time.Now().Add(60 * time.Minute)
}

// GetFailedKeywordsForSitemap gets failed keywords for a specific sitemap
func (qt *QueryTracker) GetFailedKeywordsForSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	key := "failed_keywords"
	
	var failed []FailedKeyword
	err := qt.storage.Load(ctx, key, &failed)
	if err != nil {
		return []string{}, nil // Return empty if no failed keywords
	}
	
	var sitemapFailedKeywords []string
	for _, fk := range failed {
		if fk.SitemapURL == sitemapURL {
			sitemapFailedKeywords = append(sitemapFailedKeywords, fk.Keyword)
		}
	}
	
	qt.log.WithFields(map[string]interface{}{
		"sitemap_url":      sitemapURL,
		"failed_keywords":  len(sitemapFailedKeywords),
	}).Debug("Retrieved failed keywords for sitemap")
	
	return sitemapFailedKeywords, nil
}