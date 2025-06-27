package backend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sitemap-go/pkg/logger"
)

// batchWork represents a batch submission task
type batchWork struct {
	data         []KeywordMetricsData
	batchNum     int
	totalBatches int
}

// batchResult represents the result of a batch submission
type batchResult struct {
	batchNum int
	err      error
}

// ConcurrentSubmitter handles concurrent batch submission with proper error handling
type ConcurrentSubmitter struct {
	client BackendClient
	log    *logger.Logger
}

// NewConcurrentSubmitter creates a new concurrent submitter
func NewConcurrentSubmitter(client BackendClient) *ConcurrentSubmitter {
	return &ConcurrentSubmitter{
		client: client,
		log:    logger.GetLogger().WithField("component", "concurrent_submitter"),
	}
}

// SubmitBatchesConcurrently submits batches with controlled concurrency
func (cs *ConcurrentSubmitter) SubmitBatchesConcurrently(data []KeywordMetricsData, batchSize int) error {
	if len(data) == 0 {
		cs.log.Debug("No data to submit")
		return nil
	}

	totalBatches := (len(data) + batchSize - 1) / batchSize
	
	cs.log.WithFields(map[string]interface{}{
		"total_keywords": len(data),
		"batch_size":     batchSize,
		"total_batches":  totalBatches,
	}).Info("Starting concurrent batch submission")

	// Use controlled concurrency for better performance
	maxConcurrency := 3 // Limit concurrent requests to avoid overwhelming backend
	if totalBatches < maxConcurrency {
		maxConcurrency = totalBatches
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create channels for work distribution and result collection
	batchChan := make(chan batchWork, totalBatches)
	resultChan := make(chan batchResult, totalBatches)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxConcurrency; i++ {
		wg.Add(1)
		go cs.batchWorker(ctx, &wg, batchChan, resultChan)
	}

	// Send work to workers
	go func() {
		defer close(batchChan)
		for i := 0; i < len(data); i += batchSize {
			end := i + batchSize
			if end > len(data) {
				end = len(data)
			}

			select {
			case batchChan <- batchWork{
				data:         data[i:end],
				batchNum:     i/batchSize + 1,
				totalBatches: totalBatches,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	successCount := 0
	failureCount := 0
	var firstError error

	for result := range resultChan {
		if result.err != nil {
			failureCount++
			if firstError == nil {
				firstError = result.err
			}
			cs.log.WithError(result.err).WithField("batch_number", result.batchNum).Error("Batch submission failed")
		} else {
			successCount++
			cs.log.WithField("batch_number", result.batchNum).Debug("Batch submitted successfully")
		}

		// Progress logging
		completed := successCount + failureCount
		shouldLog := completed == totalBatches || // Always log final batch
			(totalBatches > 50 && completed%25 == 0) || // Every 25th batch for large datasets
			(totalBatches <= 50 && totalBatches > 10 && completed%10 == 0) // Every 10th batch for medium datasets

		if shouldLog {
			cs.log.WithField("progress", fmt.Sprintf("%d/%d batches", completed, totalBatches)).Info("Backend submission progress")
		}
	}

	cs.log.WithFields(map[string]interface{}{
		"total_batches":      totalBatches,
		"successful_batches": successCount,
		"failed_batches":     failureCount,
		"success_rate":       fmt.Sprintf("%.1f%%", float64(successCount)/float64(totalBatches)*100),
		"concurrency":        maxConcurrency,
	}).Info("Concurrent batch submission completed")

	if failureCount > 0 {
		return fmt.Errorf("failed to submit %d out of %d batches (first error: %v)", failureCount, totalBatches, firstError)
	}

	return nil
}

// batchWorker processes batch submission tasks concurrently
func (cs *ConcurrentSubmitter) batchWorker(ctx context.Context, wg *sync.WaitGroup, batchChan <-chan batchWork, resultChan chan<- batchResult) {
	defer wg.Done()

	for {
		select {
		case work, ok := <-batchChan:
			if !ok {
				return // Channel closed, worker should exit
			}

			// Submit the batch
			_, err := cs.client.SubmitBatch(KeywordMetricsBatch(work.data))
			
			// Send result
			select {
			case resultChan <- batchResult{
				batchNum: work.batchNum,
				err:      err,
			}:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
