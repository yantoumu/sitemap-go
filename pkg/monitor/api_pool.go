package monitor

import (
	"context"
	"sync"
	"time"

	"sitemap-go/pkg/api"
	"sitemap-go/pkg/logger"
)

// APIWorkerPool manages Google Trends API queries with strict rate limiting
type APIWorkerPool struct {
	apiClient        api.APIClient
	workers          int
	requestsPerSecond float64
	rateLimiter      *RateLimitedExecutor
	taskQueue        chan APITask
	resultQueue      chan APIResult
	wg               sync.WaitGroup
	log              *logger.Logger
	ctx              context.Context
	cancel           context.CancelFunc
}

// APITask represents a Google Trends API query task
type APITask struct {
	Keywords []string
	TaskID   string
}

// APIResult represents the result of an API query
type APIResult struct {
	TaskID    string
	Response  *api.APIResponse
	Error     error
	Duration  time.Duration
}

// NewAPIWorkerPool creates a new API worker pool for Google Trends queries
func NewAPIWorkerPool(apiClient api.APIClient, workers int, requestsPerSecond float64) *APIWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &APIWorkerPool{
		apiClient:        apiClient,
		workers:          workers,
		requestsPerSecond: requestsPerSecond,
		rateLimiter:      NewRateLimitedExecutor(requestsPerSecond),
		taskQueue:        make(chan APITask, 100), // 缓冲队列
		resultQueue:      make(chan APIResult, 100),
		log:              logger.GetLogger().WithField("component", "api_worker_pool"),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start starts the API worker pool
func (pool *APIWorkerPool) Start() {
	pool.log.WithFields(map[string]interface{}{
		"workers":             pool.workers,
		"requests_per_second": pool.requestsPerSecond,
	}).Info("Starting API worker pool for Google Trends queries")
	
	// Start worker goroutines
	for i := 0; i < pool.workers; i++ {
		pool.wg.Add(1)
		go pool.worker(i)
	}
}

// Stop gracefully stops the API worker pool
func (pool *APIWorkerPool) Stop() {
	pool.log.Info("Stopping API worker pool")
	close(pool.taskQueue)
	pool.wg.Wait()
	pool.cancel()
	pool.rateLimiter.Close()
}

// SubmitTask submits a task to the API worker pool
func (pool *APIWorkerPool) SubmitTask(keywords []string, taskID string) bool {
	select {
	case pool.taskQueue <- APITask{Keywords: keywords, TaskID: taskID}:
		pool.log.WithFields(map[string]interface{}{
			"task_id":      taskID,
			"keyword_count": len(keywords),
		}).Debug("Task submitted to API worker pool")
		return true
	case <-pool.ctx.Done():
		return false
	default:
		pool.log.Warn("API worker pool task queue is full")
		return false
	}
}

// GetResultChannel returns the result channel for reading API results
func (pool *APIWorkerPool) GetResultChannel() <-chan APIResult {
	return pool.resultQueue
}

// worker is the main worker function for processing API tasks
func (pool *APIWorkerPool) worker(workerID int) {
	defer pool.wg.Done()
	
	pool.log.WithField("worker_id", workerID).Debug("API worker started")
	
	for task := range pool.taskQueue {
		pool.processTask(workerID, task)
	}
	
	pool.log.WithField("worker_id", workerID).Debug("API worker stopped")
}

// processTask processes a single API task with rate limiting
func (pool *APIWorkerPool) processTask(workerID int, task APITask) {
	startTime := time.Now()
	
	pool.log.WithFields(map[string]interface{}{
		"worker_id":      workerID,
		"task_id":        task.TaskID,
		"keyword_count":  len(task.Keywords),
	}).Debug("Processing API task")
	
	var response *api.APIResponse
	var err error
	
	// Apply rate limiting for Google Trends API
	rateLimitErr := pool.rateLimiter.Execute(pool.ctx, func() error {
		response, err = pool.apiClient.Query(pool.ctx, task.Keywords)
		return err
	})
	
	duration := time.Since(startTime)
	
	// Determine final error
	finalErr := err
	if rateLimitErr != nil {
		finalErr = rateLimitErr
	}
	
	// Log result
	if finalErr != nil {
		pool.log.WithFields(map[string]interface{}{
			"worker_id":      workerID,
			"task_id":        task.TaskID,
			"duration":       duration,
			"error":          finalErr.Error(),
		}).Warn("API task failed")
	} else {
		pool.log.WithFields(map[string]interface{}{
			"worker_id":          workerID,
			"task_id":            task.TaskID,
			"duration":           duration,
			"successful_keywords": len(response.Keywords),
		}).Info("API task completed successfully")
	}
	
	// Send result
	result := APIResult{
		TaskID:   task.TaskID,
		Response: response,
		Error:    finalErr,
		Duration: duration,
	}
	
	select {
	case pool.resultQueue <- result:
		// Result sent successfully
	case <-pool.ctx.Done():
		// Pool is shutting down
		return
	}
}

// UpdateRateLimit updates the rate limiting for the API worker pool
func (pool *APIWorkerPool) UpdateRateLimit(requestsPerSecond float64) {
	if requestsPerSecond != pool.requestsPerSecond {
		pool.log.WithFields(map[string]interface{}{
			"old_rate": pool.requestsPerSecond,
			"new_rate": requestsPerSecond,
		}).Info("Updating API rate limit")
		
		// Close old rate limiter and create new one
		pool.rateLimiter.Close()
		pool.rateLimiter = NewRateLimitedExecutor(requestsPerSecond)
		pool.requestsPerSecond = requestsPerSecond
	}
}

// GetStats returns current statistics of the API worker pool
func (pool *APIWorkerPool) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"workers":             pool.workers,
		"requests_per_second": pool.requestsPerSecond,
		"pending_tasks":       len(pool.taskQueue),
		"pending_results":     len(pool.resultQueue),
		"active":              pool.ctx.Err() == nil,
	}
}