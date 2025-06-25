package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"sitemap-go/pkg/logger"
)

// worker represents a single worker goroutine
type worker struct {
	id          int
	taskQueue   <-chan Task
	resultChan  chan<- Result
	timeout     time.Duration
	log         *logger.Logger
	active      atomic.Bool
	tasksProcessed atomic.Uint64
}

// newWorker creates a new worker instance
func newWorker(id int, taskQueue <-chan Task, resultChan chan<- Result, timeout time.Duration, log *logger.Logger) *worker {
	return &worker{
		id:         id,
		taskQueue:  taskQueue,
		resultChan: resultChan,
		timeout:    timeout,
		log:        log.WithField("worker_id", id),
	}
}

// start begins the worker's execution loop
func (w *worker) start(ctx context.Context, metrics *PoolMetrics) {
	w.log.Debug("Worker started")
	defer w.log.Debug("Worker stopped")
	
	for {
		select {
		case task, ok := <-w.taskQueue:
			if !ok {
				// Task queue closed, worker should exit
				return
			}
			
			w.processTask(ctx, task, metrics)
			
		case <-ctx.Done():
			// Context cancelled, worker should exit
			return
		}
	}
}

// processTask executes a single task with timeout and error handling
func (w *worker) processTask(ctx context.Context, task Task, metrics *PoolMetrics) {
	w.active.Store(true)
	defer w.active.Store(false)
	
	start := time.Now()
	
	w.log.WithField("task_id", task.ID).Debug("Processing task")
	
	// Create task context with timeout
	taskTimeout := task.Timeout
	if taskTimeout == 0 {
		taskTimeout = w.timeout
	}
	
	taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()
	
	// Execute task with panic recovery
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				w.log.WithFields(map[string]interface{}{
					"task_id": task.ID,
					"panic":   r,
				}).Error("Task panicked")
				err = &PanicError{Value: r}
			}
		}()
		
		err = task.Fn(taskCtx)
	}()
	
	duration := time.Since(start)
	w.tasksProcessed.Add(1)
	
	// Create result
	result := Result{
		TaskID:   task.ID,
		Error:    err,
		Duration: duration,
	}
	
	// Send result to result channel (non-blocking)
	select {
	case w.resultChan <- result:
		// Result sent successfully
	default:
		// Result channel full, log warning
		w.log.WithField("task_id", task.ID).Warn("Result channel full, dropping result")
	}
	
	// Update metrics if available
	if metrics != nil {
		if err != nil {
			metrics.IncrementTasksFailed()
		} else {
			metrics.IncrementTasksCompleted()
		}
		metrics.RecordTaskDuration(duration)
	}
	
	// Log task completion
	logFields := map[string]interface{}{
		"task_id":  task.ID,
		"duration": duration,
	}
	
	if err != nil {
		logFields["error"] = err.Error()
		w.log.WithFields(logFields).Warn("Task completed with error")
	} else {
		w.log.WithFields(logFields).Debug("Task completed successfully")
	}
}

// isActive returns true if the worker is currently processing a task
func (w *worker) isActive() bool {
	return w.active.Load()
}

// getTasksProcessed returns the total number of tasks processed by this worker
func (w *worker) getTasksProcessed() uint64 {
	return w.tasksProcessed.Load()
}

// PanicError wraps a panic value as an error
type PanicError struct {
	Value interface{}
}

func (pe *PanicError) Error() string {
	return fmt.Sprintf("panic: %v", pe.Value)
}