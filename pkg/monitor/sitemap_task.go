package monitor

import (
	"context"
	"fmt"
	"time"

	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/worker"
)

// SitemapTask implements the worker.SmartTask interface for sitemap processing
type SitemapTask struct {
	SitemapURL        string
	Config            MonitorConfig
	Monitor           *SitemapMonitor
	Priority          int
	timeoutCalculator *worker.SmartTimeoutCalculator
	result            *MonitorResult // Store result for retrieval
	log               *logger.Logger
}

// NewSitemapTask creates a new sitemap processing task
func NewSitemapTask(sitemapURL string, config MonitorConfig, monitor *SitemapMonitor) *SitemapTask {
	// Create timeout calculator for this task
	timeoutConfig := worker.TimeoutConfig{
		BaseTimeout:    2 * time.Minute,
		MaxTimeout:     15 * time.Minute,
		SizeMultiplier: 1.5,
	}
	timeoutCalculator := worker.NewSmartTimeoutCalculator(timeoutConfig)
	
	return &SitemapTask{
		SitemapURL:        sitemapURL,
		Config:            config,
		Monitor:           monitor,
		Priority:          1, // Default priority
		timeoutCalculator: timeoutCalculator,
		log:               logger.GetLogger().WithField("component", "sitemap_task"),
	}
}

// Execute implements the worker.Task interface
func (st *SitemapTask) Execute(ctx context.Context) error {
	st.log.WithField("sitemap_url", st.SitemapURL).Info("Executing sitemap task")
	
	startTime := time.Now()
	
	// Process the sitemap
	result, err := st.Monitor.ProcessSitemap(ctx, st.SitemapURL)
	if err != nil {
		st.log.WithError(err).WithField("sitemap_url", st.SitemapURL).Error("Failed to process sitemap")
		return fmt.Errorf("sitemap processing failed for %s: %w", st.SitemapURL, err)
	}
	
	// Store the result in the task for later retrieval
	st.result = result
	
	if result != nil {
		duration := time.Since(startTime)
		st.log.WithFields(map[string]interface{}{
			"sitemap_url":   st.SitemapURL,
			"keywords_found": len(result.Keywords),
			"success":       result.Success,
			"duration":      duration,
		}).Info("Sitemap task completed successfully")
	}
	
	return nil
}

// GetID implements the worker.Task interface
func (st *SitemapTask) GetID() string {
	return fmt.Sprintf("sitemap_%s_%d", st.SitemapURL, time.Now().Unix())
}

// GetPriority implements the worker.Task interface
func (st *SitemapTask) GetPriority() int {
	return st.Priority
}

// SetPriority sets the task priority
func (st *SitemapTask) SetPriority(priority int) {
	st.Priority = priority
}

// GetAdaptiveTimeout implements worker.SmartTask interface
func (st *SitemapTask) GetAdaptiveTimeout() time.Duration {
	if st.timeoutCalculator == nil {
		return 2 * time.Minute // Fallback timeout
	}
	
	// Calculate optimal timeout based on sitemap URL and estimated complexity
	estimatedSize := st.EstimateComplexity()
	return st.timeoutCalculator.CalculateOptimalTimeout(st.SitemapURL, estimatedSize)
}

// EstimateComplexity implements worker.SmartTask interface
func (st *SitemapTask) EstimateComplexity() int {
	// Estimate sitemap complexity based on URL patterns
	estimatedSize := 1000 // Base estimate
	
	// Adjust based on known site patterns
	siteEstimates := map[string]int{
		"poki.com":        50000,  // Large gaming site
		"kizi.com":        30000,  // Large gaming site
		"1001games.com":   40000,  // Large gaming site
		"y8.com":          60000,  // Very large gaming site
		"gamesgames.com":  25000,  // Large gaming site
		"miniplay.com":    20000,  // Medium-large gaming site
		"friv.com":        15000,  // Medium gaming site
		"agame.com":       10000,  // Medium gaming site
	}
	
	// Check for known sites
	for domain, size := range siteEstimates {
		if contains(st.SitemapURL, domain) {
			estimatedSize = size
			break
		}
	}
	
	// Adjust for compressed files
	if contains(st.SitemapURL, ".gz") {
		estimatedSize = int(float64(estimatedSize) * 1.5) // Compressed files are usually larger
	}
	
	// Adjust for sitemap index files
	if contains(st.SitemapURL, "index") {
		estimatedSize = int(float64(estimatedSize) * 2.0) // Index files reference multiple sitemaps
	}
	
	st.log.WithFields(map[string]interface{}{
		"sitemap_url":    st.SitemapURL,
		"estimated_size": estimatedSize,
	}).Debug("Estimated sitemap complexity")
	
	return estimatedSize
}

// GetResult returns the processing result as interface{} for worker pool compatibility
func (st *SitemapTask) GetResult() interface{} {
	return st.result
}

// GetMonitorResult returns the processing result as MonitorResult
func (st *SitemapTask) GetMonitorResult() *MonitorResult {
	return st.result
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
			containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BatchSitemapTask handles multiple sitemaps as a single task
type BatchSitemapTask struct {
	SitemapURLs []string
	Config      MonitorConfig
	Monitor     *SitemapMonitor
	Priority    int
	Results     []*MonitorResult
	log         *logger.Logger
}

// NewBatchSitemapTask creates a new batch sitemap processing task
func NewBatchSitemapTask(sitemapURLs []string, config MonitorConfig, monitor *SitemapMonitor) *BatchSitemapTask {
	return &BatchSitemapTask{
		SitemapURLs: sitemapURLs,
		Config:      config,
		Monitor:     monitor,
		Priority:    1,
		Results:     make([]*MonitorResult, 0, len(sitemapURLs)),
		log:         logger.GetLogger().WithField("component", "batch_sitemap_task"),
	}
}

// Execute implements the worker.Task interface for batch processing
func (bst *BatchSitemapTask) Execute(ctx context.Context) error {
	bst.log.WithField("sitemap_count", len(bst.SitemapURLs)).Info("Executing batch sitemap task")
	
	results := make([]*MonitorResult, 0, len(bst.SitemapURLs))
	
	for _, sitemapURL := range bst.SitemapURLs {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		result, err := bst.Monitor.ProcessSitemap(ctx, sitemapURL)
		if err != nil {
			bst.log.WithError(err).WithField("sitemap_url", sitemapURL).Error("Failed to process sitemap in batch")
			// Create error result
			result = &MonitorResult{
				SitemapURL: sitemapURL,
				Success:    false,
				Error:      err.Error(),
				Timestamp:  time.Now(),
			}
		}
		
		if result != nil {
			results = append(results, result)
		}
	}
	
	bst.Results = results
	
	bst.log.WithFields(map[string]interface{}{
		"total_sitemaps": len(bst.SitemapURLs),
		"successful":     bst.countSuccessful(),
		"failed":         len(results) - bst.countSuccessful(),
	}).Info("Batch sitemap task completed")
	
	return nil
}

// GetID implements the worker.Task interface
func (bst *BatchSitemapTask) GetID() string {
	return fmt.Sprintf("batch_sitemap_%d_%d", len(bst.SitemapURLs), time.Now().Unix())
}

// GetPriority implements the worker.Task interface
func (bst *BatchSitemapTask) GetPriority() int {
	return bst.Priority
}

// SetPriority sets the batch task priority
func (bst *BatchSitemapTask) SetPriority(priority int) {
	bst.Priority = priority
}

// GetResults returns the batch processing results
func (bst *BatchSitemapTask) GetResults() []*MonitorResult {
	return bst.Results
}

// countSuccessful counts successful results in the batch
func (bst *BatchSitemapTask) countSuccessful() int {
	count := 0
	for _, result := range bst.Results {
		if result.Success {
			count++
		}
	}
	return count
}