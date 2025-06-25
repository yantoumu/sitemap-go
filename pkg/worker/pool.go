package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"sitemap-go/pkg/logger"
)

// Task represents a unit of work to be executed
type Task struct {
	ID       string
	Fn       func(ctx context.Context) error
	Priority int
	Timeout  time.Duration
}

// Result represents the result of task execution
type Result struct {
	TaskID   string
	Error    error
	Duration time.Duration
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	MaxWorkers      int           `json:"max_workers"`
	QueueSize       int           `json:"queue_size"`
	WorkerTimeout   time.Duration `json:"worker_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	EnableMetrics   bool          `json:"enable_metrics"`
}

// DefaultWorkerPoolConfig returns optimized default configuration
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		MaxWorkers:      runtime.NumCPU() * 2,
		QueueSize:       1000,
		WorkerTimeout:   30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		EnableMetrics:   true,
	}
}

// HighThroughputWorkerPoolConfig returns config optimized for high throughput
func HighThroughputWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		MaxWorkers:      runtime.NumCPU() * 4,
		QueueSize:       5000,
		WorkerTimeout:   15 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		EnableMetrics:   true,
	}
}

// WorkerPool manages a pool of goroutines for concurrent task execution
type WorkerPool struct {
	config     WorkerPoolConfig
	taskQueue  chan Task
	resultChan chan Result
	workers    []*worker
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	log        *logger.Logger
	
	// Metrics
	metrics *PoolMetrics
	
	// State management
	started atomic.Bool
	stopped atomic.Bool
}

// NewWorkerPool creates a new worker pool with the given configuration
func NewWorkerPool(config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &WorkerPool{
		config:     config,
		taskQueue:  make(chan Task, config.QueueSize),
		resultChan: make(chan Result, config.QueueSize/2),
		workers:    make([]*worker, 0, config.MaxWorkers),
		ctx:        ctx,
		cancel:     cancel,
		log:        logger.GetLogger().WithField("component", "worker_pool"),
	}
	
	if config.EnableMetrics {
		pool.metrics = NewPoolMetrics()
	}
	
	return pool
}

// Start initializes and starts all workers
func (wp *WorkerPool) Start() error {
	if !wp.started.CompareAndSwap(false, true) {
		return fmt.Errorf("worker pool already started")
	}
	
	wp.log.WithField("max_workers", wp.config.MaxWorkers).Info("Starting worker pool")
	
	// Create and start workers
	for i := 0; i < wp.config.MaxWorkers; i++ {
		w := newWorker(i, wp.taskQueue, wp.resultChan, wp.config.WorkerTimeout, wp.log)
		wp.workers = append(wp.workers, w)
		
		wp.wg.Add(1)
		go func(worker *worker) {
			defer wp.wg.Done()
			worker.start(wp.ctx, wp.metrics)
		}(w)
	}
	
	// Start result processor if metrics are enabled
	if wp.config.EnableMetrics {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			wp.processResults()
		}()
	}
	
	wp.log.Info("Worker pool started successfully")
	return nil
}

// Submit adds a task to the worker pool queue
func (wp *WorkerPool) Submit(task Task) error {
	if wp.stopped.Load() {
		return fmt.Errorf("worker pool is stopped")
	}
	
	if !wp.started.Load() {
		return fmt.Errorf("worker pool not started")
	}
	
	// Set default timeout if not specified
	if task.Timeout == 0 {
		task.Timeout = wp.config.WorkerTimeout
	}
	
	select {
	case wp.taskQueue <- task:
		if wp.metrics != nil {
			wp.metrics.IncrementTasksSubmitted()
		}
		return nil
	default:
		if wp.metrics != nil {
			wp.metrics.IncrementTasksRejected()
		}
		return fmt.Errorf("task queue is full")
	}
}

// SubmitFunc is a convenience method to submit a function as a task
func (wp *WorkerPool) SubmitFunc(id string, fn func(ctx context.Context) error) error {
	return wp.Submit(Task{
		ID: id,
		Fn: fn,
	})
}

// SubmitWithPriority submits a task with specified priority
func (wp *WorkerPool) SubmitWithPriority(task Task, priority int) error {
	task.Priority = priority
	return wp.Submit(task)
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() error {
	if !wp.stopped.CompareAndSwap(false, true) {
		return nil // Already stopped
	}
	
	wp.log.Info("Stopping worker pool")
	
	// Signal all workers to stop
	wp.cancel()
	
	// Close task queue to prevent new submissions
	close(wp.taskQueue)
	
	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		wp.log.Info("Worker pool stopped gracefully")
	case <-time.After(wp.config.ShutdownTimeout):
		wp.log.Warn("Worker pool shutdown timeout exceeded")
	}
	
	// Close result channel
	close(wp.resultChan)
	
	return nil
}

// GetMetrics returns current pool metrics
func (wp *WorkerPool) GetMetrics() PoolMetrics {
	if wp.metrics == nil {
		return PoolMetrics{}
	}
	return *wp.metrics
}

// GetQueueSize returns current queue size
func (wp *WorkerPool) GetQueueSize() int {
	return len(wp.taskQueue)
}

// GetActiveWorkers returns number of active workers
func (wp *WorkerPool) GetActiveWorkers() int {
	activeCount := 0
	for _, w := range wp.workers {
		if w.isActive() {
			activeCount++
		}
	}
	return activeCount
}

// processResults processes task results for metrics collection
func (wp *WorkerPool) processResults() {
	for result := range wp.resultChan {
		if wp.metrics != nil {
			wp.metrics.RecordTaskResult(result)
		}
	}
}