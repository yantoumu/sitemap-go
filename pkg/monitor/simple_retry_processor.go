package monitor

import (
	"context"
	"strings"
	"time"

	"sitemap-go/pkg/api"
	"sitemap-go/pkg/backend"
	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/storage"
)

// SimpleRetryProcessor handles failed keyword retry at startup only
type SimpleRetryProcessor struct {
	apiClient       api.APIClient
	simpleTracker   *storage.SimpleTracker
	submissionPool  *backend.SubmissionPool
	dataConverter   *backend.DataConverter
	log             *logger.Logger
	secureLog       *logger.SecurityLogger
}

// NewSimpleRetryProcessor creates a new simple retry processor
func NewSimpleRetryProcessor(
	apiClient api.APIClient,
	simpleTracker *storage.SimpleTracker,
	submissionPool *backend.SubmissionPool,
	dataConverter *backend.DataConverter,
) *SimpleRetryProcessor {
	return &SimpleRetryProcessor{
		apiClient:     apiClient,
		simpleTracker: simpleTracker,
		submissionPool: submissionPool,
		dataConverter: dataConverter,
		log:           logger.GetLogger().WithField("component", "simple_retry"),
		secureLog:     logger.GetSecurityLogger(),
	}
}

// ProcessFailedKeywordsAtStartup processes failed keywords once at startup in a separate goroutine
func (srp *SimpleRetryProcessor) ProcessFailedKeywordsAtStartup(ctx context.Context) {
	// Check if there are any failed keywords to process
	failedKeywordRecords, err := srp.simpleTracker.GetRetryableKeywords(ctx)
	if err != nil {
		srp.log.WithError(err).Error("Failed to get retryable keywords")
		return
	}

	if len(failedKeywordRecords) == 0 {
		srp.log.Debug("No failed keywords to retry at startup")
		return
	}

	srp.log.WithField("failed_count", len(failedKeywordRecords)).Info("Found failed keywords, starting retry goroutine")

	// Launch retry processing in separate goroutine (non-blocking)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				srp.log.WithField("panic", r).Error("Panic recovered in retry goroutine")
			}
		}()

		srp.processFailedKeywords(ctx, failedKeywordRecords)
	}()
}

// processFailedKeywords processes the failed keywords with deduplication
func (srp *SimpleRetryProcessor) processFailedKeywords(ctx context.Context, failedKeywordRecords []storage.FailedKeywordRecord) {
	startTime := time.Now()
	srp.log.WithField("keyword_count", len(failedKeywordRecords)).Info("Starting failed keyword retry processing")

	// Step 1: Extract keywords and remove duplicates
	keywords := make([]string, 0, len(failedKeywordRecords))
	for _, record := range failedKeywordRecords {
		keywords = append(keywords, record.Keyword)
	}
	
	uniqueKeywords := srp.deduplicateKeywords(keywords)
	if len(uniqueKeywords) == 0 {
		srp.log.Info("All failed keywords were duplicates or already processed successfully")
		return
	}

	srp.log.WithFields(map[string]interface{}{
		"original_count": len(failedKeywordRecords),
		"unique_count":   len(uniqueKeywords),
	}).Info("Deduplicated failed keywords")

	// Step 2: Process keywords in batches of 8 (Google Trends API limit)
	batchSize := 8
	totalBatches := (len(uniqueKeywords) + batchSize - 1) / batchSize
	successCount := 0
	failedCount := 0

	for i := 0; i < len(uniqueKeywords); i += batchSize {
		end := i + batchSize
		if end > len(uniqueKeywords) {
			end = len(uniqueKeywords)
		}
		
		batch := uniqueKeywords[i:end]
		batchNum := (i / batchSize) + 1
		
		srp.log.WithFields(map[string]interface{}{
			"batch_num":   batchNum,
			"total_batches": totalBatches,
			"batch_size":  len(batch),
		}).Debug("Processing failed keyword batch")

		// Query Google Trends API
		response, err := srp.apiClient.Query(ctx, batch)
		if err != nil {
			srp.log.WithError(err).WithField("batch", batchNum).Error("Failed to query batch")
			failedCount += len(batch)
			continue
		}

		// Convert response to backend format and submit
		if response != nil && len(response.Keywords) > 0 {
			// Create monitor result to use existing conversion logic
			monitorResult := &storage.MonitorResult{
				SitemapURL: "retry-processing",
				Keywords:   batch,
				TrendData:  response,
				Success:    true,
				Timestamp:  time.Now(),
			}
			
			backendData, err := srp.dataConverter.ConvertMonitorResults([]*storage.MonitorResult{monitorResult})
			if err != nil {
				srp.log.WithError(err).WithField("batch", batchNum).Error("Failed to convert retry data")
				failedCount += len(batch)
				continue
			}
			
			// Submit to backend via submission pool (non-blocking)
			submitted := srp.submissionPool.Submit(backendData, func(submitErr error) {
				if submitErr != nil {
					srp.log.WithError(submitErr).WithField("batch", batchNum).Error("Failed to submit retry batch")
				} else {
					srp.log.WithField("batch", batchNum).Debug("Successfully submitted retry batch")
				}
			})

			if submitted {
				successCount += len(batch)
				srp.log.WithField("batch", batchNum).Debug("Successfully queued retry batch for submission")
			} else {
				failedCount += len(batch)
				srp.log.WithField("batch", batchNum).Warn("Failed to queue batch for submission")
			}
		} else {
			failedCount += len(batch)
			srp.log.WithField("batch", batchNum).Warn("Empty response from API")
		}

		// Small delay between batches to avoid overwhelming the API
		select {
		case <-ctx.Done():
			srp.log.Info("Context cancelled, stopping retry processing")
			return
		case <-time.After(500 * time.Millisecond):
			// Continue to next batch
		}
	}

	duration := time.Since(startTime)
	srp.log.WithFields(map[string]interface{}{
		"total_keywords":    len(uniqueKeywords),
		"successful_retries": successCount,
		"failed_retries":    failedCount,
		"duration":          duration.String(),
		"success_rate":      float64(successCount)/float64(len(uniqueKeywords))*100,
	}).Info("Failed keyword retry processing completed")
}

// deduplicateKeywords removes duplicates and checks against successful keywords
func (srp *SimpleRetryProcessor) deduplicateKeywords(keywords []string) []string {
	// Create a set to track unique keywords
	seen := make(map[string]bool)
	var unique []string

	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}

		// Skip if already seen in this batch
		if seen[keyword] {
			continue
		}

		// TODO: Add check against successful keywords in storage
		// This would require a method to check if keyword was already successfully processed
		// For now, we rely on the SimpleTracker to manage this

		seen[keyword] = true
		unique = append(unique, keyword)
	}

	return unique
}

// Note: Currently there's no method to remove successful keywords from failed list
// This would need to be implemented in SimpleTracker if needed in the future

// GetStatus returns whether retry processing is available
func (srp *SimpleRetryProcessor) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"type":        "simple_startup_retry",
		"description": "Processes failed keywords once at startup in background goroutine",
		"blocking":    false,
	}
}