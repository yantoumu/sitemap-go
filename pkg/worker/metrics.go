package worker

import (
	"sync/atomic"
	"time"
)

// PoolMetrics tracks worker pool performance metrics
type PoolMetrics struct {
	TasksSubmitted  atomic.Uint64
	TasksCompleted  atomic.Uint64
	TasksFailed     atomic.Uint64
	TasksRejected   atomic.Uint64
	
	TotalDuration   atomic.Uint64 // in nanoseconds
	MinDuration     atomic.Uint64 // in nanoseconds
	MaxDuration     atomic.Uint64 // in nanoseconds
	
	StartTime       time.Time
}

// NewPoolMetrics creates a new metrics instance
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{
		StartTime:   time.Now(),
		MinDuration: atomic.Uint64{},
		MaxDuration: atomic.Uint64{},
	}
}

// IncrementTasksSubmitted increments the submitted tasks counter
func (pm *PoolMetrics) IncrementTasksSubmitted() {
	pm.TasksSubmitted.Add(1)
}

// IncrementTasksCompleted increments the completed tasks counter
func (pm *PoolMetrics) IncrementTasksCompleted() {
	pm.TasksCompleted.Add(1)
}

// IncrementTasksFailed increments the failed tasks counter
func (pm *PoolMetrics) IncrementTasksFailed() {
	pm.TasksFailed.Add(1)
}

// IncrementTasksRejected increments the rejected tasks counter
func (pm *PoolMetrics) IncrementTasksRejected() {
	pm.TasksRejected.Add(1)
}

// RecordTaskDuration records task execution duration
func (pm *PoolMetrics) RecordTaskDuration(duration time.Duration) {
	nanos := uint64(duration.Nanoseconds())
	
	// Update total duration
	pm.TotalDuration.Add(nanos)
	
	// Update min duration
	for {
		current := pm.MinDuration.Load()
		if current == 0 || nanos < current {
			if pm.MinDuration.CompareAndSwap(current, nanos) {
				break
			}
		} else {
			break
		}
	}
	
	// Update max duration
	for {
		current := pm.MaxDuration.Load()
		if nanos > current {
			if pm.MaxDuration.CompareAndSwap(current, nanos) {
				break
			}
		} else {
			break
		}
	}
}

// RecordTaskResult records the result of task execution
func (pm *PoolMetrics) RecordTaskResult(result Result) {
	if result.Error != nil {
		pm.IncrementTasksFailed()
	} else {
		pm.IncrementTasksCompleted()
	}
	pm.RecordTaskDuration(result.Duration)
}

// GetSnapshot returns a snapshot of current metrics
func (pm *PoolMetrics) GetSnapshot() MetricsSnapshot {
	submitted := pm.TasksSubmitted.Load()
	completed := pm.TasksCompleted.Load()
	failed := pm.TasksFailed.Load()
	rejected := pm.TasksRejected.Load()
	totalDuration := pm.TotalDuration.Load()
	minDuration := pm.MinDuration.Load()
	maxDuration := pm.MaxDuration.Load()
	
	var avgDuration time.Duration
	if completed > 0 {
		avgDuration = time.Duration(totalDuration / completed)
	}
	
	var successRate float64
	if submitted > 0 {
		successRate = float64(completed) / float64(submitted)
	}
	
	var throughput float64
	uptime := time.Since(pm.StartTime)
	if uptime > 0 {
		throughput = float64(completed) / uptime.Seconds()
	}
	
	return MetricsSnapshot{
		TasksSubmitted:  submitted,
		TasksCompleted:  completed,
		TasksFailed:     failed,
		TasksRejected:   rejected,
		SuccessRate:     successRate,
		Throughput:      throughput,
		AverageDuration: avgDuration,
		MinDuration:     time.Duration(minDuration),
		MaxDuration:     time.Duration(maxDuration),
		Uptime:          uptime,
	}
}

// Reset clears all metrics
func (pm *PoolMetrics) Reset() {
	pm.TasksSubmitted.Store(0)
	pm.TasksCompleted.Store(0)
	pm.TasksFailed.Store(0)
	pm.TasksRejected.Store(0)
	pm.TotalDuration.Store(0)
	pm.MinDuration.Store(0)
	pm.MaxDuration.Store(0)
	pm.StartTime = time.Now()
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	TasksSubmitted  uint64        `json:"tasks_submitted"`
	TasksCompleted  uint64        `json:"tasks_completed"`
	TasksFailed     uint64        `json:"tasks_failed"`
	TasksRejected   uint64        `json:"tasks_rejected"`
	SuccessRate     float64       `json:"success_rate"`
	Throughput      float64       `json:"throughput_per_second"`
	AverageDuration time.Duration `json:"average_duration"`
	MinDuration     time.Duration `json:"min_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	Uptime          time.Duration `json:"uptime"`
}