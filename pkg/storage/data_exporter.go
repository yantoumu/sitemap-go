package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DataExporter exports encrypted data to readable JSON format for GitHub Actions
type DataExporter struct {
	tracker *SimpleTracker
	storage Storage
}

// NewDataExporter creates a new data exporter
func NewDataExporter(storage Storage) *DataExporter {
	return &DataExporter{
		tracker: NewSimpleTracker(storage),
		storage: storage,
	}
}

// ExportReport exports a summary report for GitHub Actions
func (de *DataExporter) ExportReport(ctx context.Context, outputDir string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Export processed URLs summary
	if err := de.exportProcessedURLs(ctx, outputDir); err != nil {
		return fmt.Errorf("failed to export processed URLs: %w", err)
	}
	
	// Export failed keywords summary
	if err := de.exportFailedKeywords(ctx, outputDir); err != nil {
		return fmt.Errorf("failed to export failed keywords: %w", err)
	}
	
	// Create overall summary
	if err := de.createSummary(ctx, outputDir); err != nil {
		return fmt.Errorf("failed to create summary: %w", err)
	}
	
	return nil
}

// exportProcessedURLs exports processed URLs summary
func (de *DataExporter) exportProcessedURLs(ctx context.Context, outputDir string) error {
	var records []URLHashRecord
	err := de.storage.Load(ctx, "processed_urls", &records)
	if err != nil {
		// If no data exists, create empty file
		records = []URLHashRecord{}
	}
	
	// Create summary
	summary := map[string]interface{}{
		"total_processed": len(records),
		"export_time":     time.Now().Format(time.RFC3339),
		"latest_10":       []URLHashRecord{},
	}
	
	// Get latest 10 records
	if len(records) > 10 {
		summary["latest_10"] = records[len(records)-10:]
	} else {
		summary["latest_10"] = records
	}
	
	// Save to file
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	
	filePath := filepath.Join(outputDir, "processed_urls_summary.json")
	return os.WriteFile(filePath, data, 0644)
}

// exportFailedKeywords exports failed keywords summary
func (de *DataExporter) exportFailedKeywords(ctx context.Context, outputDir string) error {
	var records []FailedKeywordRecord
	err := de.storage.Load(ctx, "failed_keywords", &records)
	if err != nil {
		records = []FailedKeywordRecord{}
	}
	
	// Group by sitemap
	bySitemap := make(map[string][]FailedKeywordRecord)
	for _, record := range records {
		bySitemap[record.SitemapURL] = append(bySitemap[record.SitemapURL], record)
	}
	
	// Create summary
	summary := map[string]interface{}{
		"total_failed":    len(records),
		"export_time":     time.Now().Format(time.RFC3339),
		"by_sitemap":      map[string]int{},
		"recent_failures": []FailedKeywordRecord{},
	}
	
	// Count by sitemap
	for sitemap, keywords := range bySitemap {
		summary["by_sitemap"].(map[string]int)[sitemap] = len(keywords)
	}
	
	// Get recent 20 failures
	if len(records) > 20 {
		summary["recent_failures"] = records[len(records)-20:]
	} else {
		summary["recent_failures"] = records
	}
	
	// Save to file
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	
	filePath := filepath.Join(outputDir, "failed_keywords_summary.json")
	return os.WriteFile(filePath, data, 0644)
}

// createSummary creates an overall summary file
func (de *DataExporter) createSummary(ctx context.Context, outputDir string) error {
	// Load all data
	var processedURLs []URLHashRecord
	var failedKeywords []FailedKeywordRecord
	
	_ = de.storage.Load(ctx, "processed_urls", &processedURLs)
	_ = de.storage.Load(ctx, "failed_keywords", &failedKeywords)
	
	// Create summary
	summary := map[string]interface{}{
		"report_time":        time.Now().Format(time.RFC3339),
		"total_processed":    len(processedURLs),
		"total_failed":       len(failedKeywords),
		"success_rate":       0.0,
		"data_directory":     "./data",
		"encrypted_files": map[string]string{
			"processed_urls":  "./data/pr/processed_urls.enc",
			"failed_keywords": "./data/fa/failed_keywords.enc",
		},
	}
	
	// Calculate success rate
	total := len(processedURLs) + len(failedKeywords)
	if total > 0 {
		summary["success_rate"] = float64(len(processedURLs)) / float64(total) * 100
	}
	
	// Save to file
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	
	filePath := filepath.Join(outputDir, "monitoring_summary.json")
	return os.WriteFile(filePath, data, 0644)
}