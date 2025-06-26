package backend

import (
	"context"
	"sync"
	"time"

	"sitemap-go/pkg/logger"
)

// SubmissionTask represents a single submission task
type SubmissionTask struct {
	Data     []KeywordMetricsData
	Callback func(error) // Optional callback for completion notification
}

// SubmissionPool manages non-blocking background submission
type SubmissionPool struct {
	client       BackendClient
	taskChannel  chan SubmissionTask
	workerCount  int
	stopChannel  chan struct{}
	wg           sync.WaitGroup
	log          *logger.Logger
	
	// Statistics
	totalTasks     int64
	completedTasks int64
	failedTasks    int64
	mu             sync.RWMutex
}

// NewSubmissionPool creates a new submission pool
func NewSubmissionPool(client BackendClient, workerCount int) *SubmissionPool {
	if workerCount <= 0 {
		workerCount = 3 // Default worker count
	}
	
	return &SubmissionPool{
		client:      client,
		taskChannel: make(chan SubmissionTask, 100), // Buffer for 100 tasks
		workerCount: workerCount,
		stopChannel: make(chan struct{}),
		log:         logger.GetLogger().WithField("component", "submission_pool"),
	}
}

// Start starts the submission workers
func (sp *SubmissionPool) Start(ctx context.Context) {
	sp.log.WithField("worker_count", sp.workerCount).Info("Starting submission pool")
	
	for i := 0; i < sp.workerCount; i++ {
		sp.wg.Add(1)
		go sp.worker(ctx, i)
	}
}

// Stop gracefully stops the submission pool
func (sp *SubmissionPool) Stop() {
	sp.log.Info("Stopping submission pool")
	close(sp.stopChannel)
	sp.wg.Wait()
	sp.log.Info("Submission pool stopped")
}

// Submit submits data for background processing (non-blocking)
func (sp *SubmissionPool) Submit(data []KeywordMetricsData, callback func(error)) bool {
	task := SubmissionTask{
		Data:     data,
		Callback: callback,
	}
	
	sp.mu.Lock()
	sp.totalTasks++
	sp.mu.Unlock()
	
	select {
	case sp.taskChannel <- task:
		sp.log.WithField("data_count", len(data)).Debug("Task queued for submission")
		return true
	case <-sp.stopChannel:
		sp.log.Warn("Submission pool is stopping, task rejected")
		return false
	default:
		sp.log.Warn("Submission queue is full, task rejected")
		return false
	}
}

// worker processes submission tasks
func (sp *SubmissionPool) worker(ctx context.Context, workerID int) {
	defer sp.wg.Done()
	
	workerLog := sp.log.WithField("worker_id", workerID)
	workerLog.Debug("Submission worker started")
	
	for {
		select {
		case task := <-sp.taskChannel:
			workerLog.WithField("data_count", len(task.Data)).Debug("Processing submission task")
			
			err := sp.processTask(ctx, task)
			
			sp.mu.Lock()
			if err != nil {
				sp.failedTasks++
				workerLog.WithError(err).Error("Submission task failed")
			} else {
				sp.completedTasks++
				workerLog.Debug("Submission task completed successfully")
			}
			sp.mu.Unlock()
			
			// Call callback if provided
			if task.Callback != nil {
				task.Callback(err)
			}
			
		case <-sp.stopChannel:
			workerLog.Debug("Worker stopping")
			return
		case <-ctx.Done():
			workerLog.Debug("Worker context cancelled")
			return
		}
	}
}

// processTask processes a single submission task
func (sp *SubmissionPool) processTask(ctx context.Context, task SubmissionTask) error {
	if len(task.Data) == 0 {
		return nil
	}
	
	startTime := time.Now()
	err := sp.client.SubmitBatches(task.Data)
	duration := time.Since(startTime)
	
	sp.log.WithFields(map[string]interface{}{
		"data_count": len(task.Data),
		"duration":   duration.String(),
		"success":    err == nil,
	}).Info("Submission task processed")
	
	return err
}

// GetStats returns submission statistics
func (sp *SubmissionPool) GetStats() (total, completed, failed int64) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.totalTasks, sp.completedTasks, sp.failedTasks
}

// GetSuccessRate returns the success rate as a percentage
func (sp *SubmissionPool) GetSuccessRate() float64 {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	
	if sp.totalTasks == 0 {
		return 0
	}
	
	return float64(sp.completedTasks) / float64(sp.totalTasks) * 100
}