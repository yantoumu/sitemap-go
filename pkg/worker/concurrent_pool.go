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

// Task represents a unit of work to be processed
type Task interface {
	Execute(ctx context.Context) error
	GetID() string
	GetPriority() int // Lower number = higher priority
}

// SmartTask extends Task interface with adaptive timeout capabilities
type SmartTask interface {
	Task
	GetAdaptiveTimeout() time.Duration // Returns calculated timeout for this specific task
	EstimateComplexity() int           // Returns estimated complexity (e.g., number of URLs to process)
}

// Result represents the result of a task execution
type Result struct {
	TaskID    string
	Success   bool
	Error     error
	Data      interface{}
	Duration  time.Duration
	Timestamp time.Time
}

// ConcurrentPool implements a high-performance worker pool with adaptive timeout
type ConcurrentPool struct {
	workers     int
	taskQueue   chan Task
	resultQueue chan *Result
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	
	// Metrics
	totalTasks    uint64
	completedTasks uint64
	failedTasks   uint64
	activeWorkers int32
	
	// Configuration
	maxQueueSize     int
	batchSize        int
	adaptiveBatch    bool
	taskTimeout      time.Duration // Fallback timeout
	adaptiveTimeout  bool          // Enable adaptive timeout
	
	// Adaptive timeout calculator
	timeoutCalculator *SmartTimeoutCalculator
	
	log *logger.Logger
}

// PoolConfig holds configuration for the worker pool
type PoolConfig struct {
	Workers         int           `json:"workers"`
	MaxQueueSize    int           `json:"max_queue_size"`
	BatchSize       int           `json:"batch_size"`
	AdaptiveBatch   bool          `json:"adaptive_batch"`
	TaskTimeout     time.Duration `json:"task_timeout"`
	BufferSize      int           `json:"buffer_size"`
	AdaptiveTimeout bool          `json:"adaptive_timeout"`
	TimeoutConfig   TimeoutConfig `json:"timeout_config"`
}

// DefaultPoolConfig returns optimized default configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		Workers:       8, // Default 8 concurrent workers as requested
		MaxQueueSize:  10000,
		BatchSize:     10,
		AdaptiveBatch: true,
		TaskTimeout:   30 * time.Second, // Fallback timeout
		BufferSize:    1000,
		AdaptiveTimeout: true, // Enable adaptive timeout by default
		TimeoutConfig: TimeoutConfig{
			BaseTimeout:    2 * time.Minute,
			MaxTimeout:     15 * time.Minute,
			SizeMultiplier: 1.5,
		},
	}
}

// HighThroughputConfig returns configuration optimized for high throughput
func HighThroughputConfig() PoolConfig {
	numCPU := runtime.NumCPU()

	// Adaptive queue size based on available memory
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Calculate reasonable queue size based on available memory (max 10MB for queue)
	maxQueueSize := int(memStats.Sys / (1024 * 1024)) // 1 task per MB of system memory
	if maxQueueSize > 50000 {
		maxQueueSize = 50000 // Cap at original max
	}
	if maxQueueSize < 1000 {
		maxQueueSize = 1000 // Minimum reasonable size
	}

	bufferSize := maxQueueSize / 10 // Buffer is 10% of queue size
	if bufferSize > 5000 {
		bufferSize = 5000
	}
	if bufferSize < 100 {
		bufferSize = 100
	}

	return PoolConfig{
		Workers:       numCPU * 2, // 2x CPU cores for I/O bound tasks
		MaxQueueSize:  maxQueueSize,
		BatchSize:     20,
		AdaptiveBatch: true,
		TaskTimeout:   15 * time.Second,
		BufferSize:    bufferSize,
	}
}

// NewConcurrentPool creates a new high-performance worker pool
func NewConcurrentPool(config PoolConfig) *ConcurrentPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize adaptive timeout calculator if enabled
	var timeoutCalculator *SmartTimeoutCalculator
	if config.AdaptiveTimeout {
		timeoutCalculator = NewSmartTimeoutCalculator(config.TimeoutConfig)
	}
	
	return &ConcurrentPool{
		workers:           config.Workers,
		taskQueue:         make(chan Task, config.MaxQueueSize),
		resultQueue:       make(chan *Result, config.BufferSize),
		ctx:               ctx,
		cancel:            cancel,
		maxQueueSize:      config.MaxQueueSize,
		batchSize:         config.BatchSize,
		adaptiveBatch:     config.AdaptiveBatch,
		taskTimeout:       config.TaskTimeout,
		adaptiveTimeout:   config.AdaptiveTimeout,
		timeoutCalculator: timeoutCalculator,
		log:               logger.GetLogger().WithField("component", "concurrent_pool"),
	}
}

// Start begins processing tasks with the specified number of workers
func (p *ConcurrentPool) Start() error {
	p.log.WithField("workers", p.workers).Info("Starting concurrent worker pool")
	
	// Start worker goroutines
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	
	// Start result collector
	go p.resultCollector()
	
	p.log.Info("Worker pool started successfully")
	return nil
}

// Stop gracefully shuts down the worker pool
func (p *ConcurrentPool) Stop() error {
	p.log.Info("Stopping worker pool...")
	
	// Step 1: Cancel context to signal workers to stop
	p.cancel()
	
	// Step 2: Close task queue
	close(p.taskQueue)
	
	// Step 3: Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		p.log.Info("All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		p.log.Warn("Timeout waiting for workers to stop")
		<-done
	}
	
	// Step 4: Close result queue after workers are done
	close(p.resultQueue)
	p.log.Info("Worker pool stopped")
	return nil
}

// Submit adds a task to the worker queue (non-blocking)
func (p *ConcurrentPool) Submit(task Task) error {
	select {
	case p.taskQueue <- task:
		atomic.AddUint64(&p.totalTasks, 1)
		return nil
	default:
		return fmt.Errorf("task queue is full, cannot submit task %s", task.GetID())
	}
}

// SubmitWithTimeout adds a task with a timeout (blocking with timeout)
func (p *ConcurrentPool) SubmitWithTimeout(task Task, timeout time.Duration) error {
	select {
	case p.taskQueue <- task:
		atomic.AddUint64(&p.totalTasks, 1)
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout submitting task %s", task.GetID())
	case <-p.ctx.Done():
		return fmt.Errorf("pool is shutting down")
	}
}

// GetResultChannel returns the result channel for consuming results
func (p *ConcurrentPool) GetResultChannel() <-chan *Result {
	return p.resultQueue
}

// GetMetrics returns current pool metrics
func (p *ConcurrentPool) GetMetrics() PoolMetrics {
	return PoolMetrics{
		TotalTasks:     atomic.LoadUint64(&p.totalTasks),
		CompletedTasks: atomic.LoadUint64(&p.completedTasks),
		FailedTasks:    atomic.LoadUint64(&p.failedTasks),
		ActiveWorkers:  atomic.LoadInt32(&p.activeWorkers),
		QueueLength:    len(p.taskQueue),
		QueueCapacity:  cap(p.taskQueue),
	}
}

// worker is the main worker goroutine that processes tasks
func (p *ConcurrentPool) worker(id int) {
	defer p.wg.Done()
	
	p.log.WithField("worker_id", id).Debug("Worker started")
	atomic.AddInt32(&p.activeWorkers, 1)
	defer atomic.AddInt32(&p.activeWorkers, -1)
	
	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				p.log.WithField("worker_id", id).Debug("Worker stopping - task queue closed")
				return
			}
			
			p.processTask(id, task)
			
		case <-p.ctx.Done():
			p.log.WithField("worker_id", id).Debug("Worker stopping - context cancelled")
			return
		}
	}
}

// processTask executes a single task with timeout and error handling
func (p *ConcurrentPool) processTask(workerID int, task Task) {
	startTime := time.Now()
	taskID := task.GetID()
	
	// Remove per-task debug logs to reduce log noise
	// Task processing is tracked via metrics instead
	
	// Calculate adaptive timeout for this task
	timeout := p.calculateTaskTimeout(task)
	
	// Create task-specific context with calculated timeout
	taskCtx, cancel := context.WithTimeout(p.ctx, timeout)
	defer cancel()
	
	// Remove per-task timeout debug logs to reduce log noise
	
	// Execute task
	var err error
	var success bool
	
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("task panicked: %v", r)
				p.log.WithField("task_id", taskID).WithField("panic", r).Error("Task panicked")
			}
		}()
		
		err = task.Execute(taskCtx)
		success = err == nil
	}()
	
	duration := time.Since(startTime)
	
	// Update metrics
	if success {
		atomic.AddUint64(&p.completedTasks, 1)
	} else {
		atomic.AddUint64(&p.failedTasks, 1)
	}
	
	// Store task result data if available (for SitemapTask)
	var taskData interface{}
	if sitemapTask, ok := task.(interface{ GetResult() interface{} }); ok && success {
		// Extract result data from sitemap task
		taskData = sitemapTask.GetResult()
	}
	
	// Create result
	result := &Result{
		TaskID:    taskID,
		Success:   success,
		Error:     err,
		Data:      taskData,
		Duration:  duration,
		Timestamp: time.Now(),
	}
	
	// Send result (non-blocking with context check)
	select {
	case p.resultQueue <- result:
		// Result sent successfully
	case <-p.ctx.Done():
		// Pool shutting down, safely drop result
		// Removed debug logging for cleaner output
	default:
		// Result queue is full, log and continue
		p.log.WithField("task_id", taskID).Warn("Result queue full, dropping result")
	}
	
	// Remove per-task completion debug logs to reduce log noise
	// Task metrics are tracked via other mechanisms
}

// resultCollector handles result processing
func (p *ConcurrentPool) resultCollector() {
	// Removed debug logging for cleaner output

	for result := range p.resultQueue {
		// Only log errors, not debug info
		if result.Error != nil {
			p.log.WithField("task_id", result.TaskID).WithError(result.Error).Error("Task failed")
		}
	}

	// Removed debug logging for cleaner output
}

// PoolMetrics represents worker pool performance metrics
type PoolMetrics struct {
	TotalTasks     uint64 `json:"total_tasks"`
	CompletedTasks uint64 `json:"completed_tasks"`
	FailedTasks    uint64 `json:"failed_tasks"`
	ActiveWorkers  int32  `json:"active_workers"`
	QueueLength    int    `json:"queue_length"`
	QueueCapacity  int    `json:"queue_capacity"`
}

// GetSuccessRate calculates the success rate of completed tasks
func (m PoolMetrics) GetSuccessRate() float64 {
	total := m.CompletedTasks + m.FailedTasks
	if total == 0 {
		return 0
	}
	return float64(m.CompletedTasks) / float64(total)
}

// GetUtilization calculates queue utilization percentage
func (m PoolMetrics) GetUtilization() float64 {
	if m.QueueCapacity == 0 {
		return 0
	}
	return float64(m.QueueLength) / float64(m.QueueCapacity) * 100
}

// calculateTaskTimeout calculates appropriate timeout for a task
func (p *ConcurrentPool) calculateTaskTimeout(task Task) time.Duration {
	// If adaptive timeout is disabled, use fallback timeout
	if !p.adaptiveTimeout || p.timeoutCalculator == nil {
		return p.taskTimeout
	}
	
	// Check if task implements SmartTask interface
	if smartTask, ok := task.(SmartTask); ok {
		// Use task's own adaptive timeout calculation
		adaptiveTimeout := smartTask.GetAdaptiveTimeout()
		if adaptiveTimeout > 0 {
			p.log.WithFields(map[string]interface{}{
				"task_id": task.GetID(),
				"adaptive_timeout": adaptiveTimeout,
			}).Debug("Using task-specific adaptive timeout")
			return adaptiveTimeout
		}
	}
	
	// Fallback to default timeout
	return p.taskTimeout
}