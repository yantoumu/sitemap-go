package logger

import (
	"fmt"
	"sync"
	"time"
)

// ProgressReporter provides simple progress reporting functionality
type ProgressReporter struct {
	mu          sync.RWMutex
	total       int
	current     int
	description string
	startTime   time.Time
	lastUpdate  time.Time
	logger      *Logger
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(total int, description string) *ProgressReporter {
	return &ProgressReporter{
		total:       total,
		current:     0,
		description: description,
		startTime:   time.Now(),
		lastUpdate:  time.Now(),
		logger:      GetLogger().WithField("component", "progress"),
	}
}

// Update increments the progress counter and optionally reports progress
func (pr *ProgressReporter) Update(increment int) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	
	pr.current += increment
	now := time.Now()
	
	// Report progress every 5 seconds or when complete
	if now.Sub(pr.lastUpdate) >= 5*time.Second || pr.current >= pr.total {
		pr.reportProgress()
		pr.lastUpdate = now
	}
}

// SetCurrent sets the current progress value
func (pr *ProgressReporter) SetCurrent(current int) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	
	pr.current = current
	now := time.Now()
	
	// Report progress every 5 seconds or when complete
	if now.Sub(pr.lastUpdate) >= 5*time.Second || pr.current >= pr.total {
		pr.reportProgress()
		pr.lastUpdate = now
	}
}

// Complete marks the progress as complete and reports final status
func (pr *ProgressReporter) Complete() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	
	pr.current = pr.total
	pr.reportProgress()
}

// reportProgress logs the current progress (must be called with lock held)
func (pr *ProgressReporter) reportProgress() {
	percentage := float64(pr.current) / float64(pr.total) * 100
	elapsed := time.Since(pr.startTime)
	
	// Estimate remaining time
	var eta string
	if pr.current > 0 && pr.current < pr.total {
		avgTimePerItem := elapsed / time.Duration(pr.current)
		remaining := time.Duration(pr.total-pr.current) * avgTimePerItem
		eta = fmt.Sprintf(" (ETA: %s)", remaining.Round(time.Second))
	}
	
	pr.logger.WithFields(map[string]interface{}{
		"progress":    fmt.Sprintf("%.1f%%", percentage),
		"current":     pr.current,
		"total":       pr.total,
		"elapsed":     elapsed.Round(time.Second).String(),
		"description": pr.description,
	}).Info(fmt.Sprintf("%s: %d/%d (%.1f%%)%s", pr.description, pr.current, pr.total, percentage, eta))
}

// GetProgress returns current progress information
func (pr *ProgressReporter) GetProgress() (current, total int, percentage float64) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	
	return pr.current, pr.total, float64(pr.current) / float64(pr.total) * 100
}

// SimpleProgressTracker provides basic progress tracking for multiple operations
type SimpleProgressTracker struct {
	operations map[string]*ProgressReporter
	mu         sync.RWMutex
	logger     *Logger
}

// NewSimpleProgressTracker creates a new simple progress tracker
func NewSimpleProgressTracker() *SimpleProgressTracker {
	return &SimpleProgressTracker{
		operations: make(map[string]*ProgressReporter),
		logger:     GetLogger().WithField("component", "progress_tracker"),
	}
}

// StartOperation starts tracking a new operation
func (spt *SimpleProgressTracker) StartOperation(name string, total int, description string) {
	spt.mu.Lock()
	defer spt.mu.Unlock()
	
	spt.operations[name] = NewProgressReporter(total, description)
	spt.logger.WithFields(map[string]interface{}{
		"operation": name,
		"total":     total,
	}).Info(fmt.Sprintf("Started: %s", description))
}

// UpdateOperation updates progress for an operation
func (spt *SimpleProgressTracker) UpdateOperation(name string, increment int) {
	spt.mu.RLock()
	reporter, exists := spt.operations[name]
	spt.mu.RUnlock()
	
	if exists {
		reporter.Update(increment)
	}
}

// CompleteOperation marks an operation as complete
func (spt *SimpleProgressTracker) CompleteOperation(name string) {
	spt.mu.RLock()
	reporter, exists := spt.operations[name]
	spt.mu.RUnlock()
	
	if exists {
		reporter.Complete()
		spt.logger.WithField("operation", name).Info(fmt.Sprintf("Completed: %s", reporter.description))
	}
}

// GetOperationProgress returns progress for a specific operation
func (spt *SimpleProgressTracker) GetOperationProgress(name string) (current, total int, percentage float64, exists bool) {
	spt.mu.RLock()
	defer spt.mu.RUnlock()
	
	if reporter, ok := spt.operations[name]; ok {
		current, total, percentage = reporter.GetProgress()
		return current, total, percentage, true
	}
	return 0, 0, 0, false
}

// GetOverallProgress returns overall progress across all operations
func (spt *SimpleProgressTracker) GetOverallProgress() (totalCurrent, totalExpected int, overallPercentage float64) {
	spt.mu.RLock()
	defer spt.mu.RUnlock()
	
	for _, reporter := range spt.operations {
		current, total, _ := reporter.GetProgress()
		totalCurrent += current
		totalExpected += total
	}
	
	if totalExpected > 0 {
		overallPercentage = float64(totalCurrent) / float64(totalExpected) * 100
	}
	
	return totalCurrent, totalExpected, overallPercentage
}
