package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"sitemap-go/pkg/monitor"
)

func main() {
	fmt.Println("=== Concurrent Sitemap Monitoring Performance Test ===\n")

	// Test configuration with a subset of game sites
	testSitemaps := []string{
		"https://1games.io/sitemap.xml",
		"https://geometrydash.io/sitemap.xml",
		"https://wordle2.io/sitemap.xml",
		"https://poki.com/en/sitemaps/index.xml",
		"https://kizi.com/sitemaps/kizi/en/sitemap_games.xml.gz",
		"https://playgama.com/sitemap-2.xml",
		"https://sprunki.org/sitemap.xml",
		"https://www.1001games.com/sitemap-games.xml",
	}

	// Set test environment variables
	os.Setenv("TREND_API_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_API_KEY", "test-key")
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	// Create monitor configuration
	config := monitor.MonitorConfig{
		SitemapURLs:     testSitemaps,
		TrendAPIBaseURL: "https://httpbin.org",
		BackendBaseURL:  "https://httpbin.org",
		BackendAPIKey:   "test-key",
		EncryptionKey:   "test-encryption-key",
		WorkerPoolSize:  8,
	}

	// Create sitemap monitor
	sitemapMonitor, err := monitor.NewSitemapMonitor(config)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	// Run performance test
	fmt.Printf("Testing %d sitemaps with 8 concurrent workers...\n", len(testSitemaps))
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Execute monitoring
	results, err := sitemapMonitor.MonitorSitemaps(ctx, config)
	if err != nil {
		log.Printf("Monitoring completed with errors: %v", err)
	}

	totalDuration := time.Since(startTime)

	// Analyze results
	successful := 0
	failed := 0
	totalKeywords := 0
	totalURLs := 0

	for _, result := range results {
		if result.Success {
			successful++
			totalKeywords += len(result.Keywords)
			if urlCount, ok := result.Metadata["url_count"].(int); ok {
				totalURLs += urlCount
			}
		} else {
			failed++
		}
	}

	// Print performance metrics
	fmt.Printf("\n=== Performance Results ===\n")
	fmt.Printf("Total Duration: %v\n", totalDuration)
	fmt.Printf("Sitemaps Processed: %d\n", len(results))
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success Rate: %.2f%%\n", float64(successful)/float64(len(results))*100)
	fmt.Printf("Total URLs Extracted: %d\n", totalURLs)
	fmt.Printf("Total Keywords: %d\n", totalKeywords)
	fmt.Printf("Average Processing Time: %v per sitemap\n", totalDuration/time.Duration(len(testSitemaps)))

	if totalURLs > 0 {
		fmt.Printf("URLs per Second: %.2f\n", float64(totalURLs)/totalDuration.Seconds())
	}

	if totalKeywords > 0 {
		fmt.Printf("Keywords per Second: %.2f\n", float64(totalKeywords)/totalDuration.Seconds())
	}

	// Print detailed results
	fmt.Printf("\n=== Detailed Results ===\n")
	for _, result := range results {
		status := "✓"
		if !result.Success {
			status = "✗"
		}
		
		keywordCount := len(result.Keywords)
		duration := "N/A"
		if processingDuration, ok := result.Metadata["processing_duration_ms"].(int64); ok {
			duration = fmt.Sprintf("%dms", processingDuration)
		}

		fmt.Printf("%s %s - Keywords: %d, Duration: %s\n", 
			status, result.SitemapURL, keywordCount, duration)
		
		if !result.Success && result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}
	}

	// Performance benchmarks
	fmt.Printf("\n=== Performance Benchmarks ===\n")
	
	targetURLsPerMinute := 10000 // PRD requirement
	actualURLsPerMinute := float64(totalURLs) / totalDuration.Minutes()
	
	fmt.Printf("Target: %d URLs/minute\n", targetURLsPerMinute)
	fmt.Printf("Actual: %.2f URLs/minute\n", actualURLsPerMinute)
	
	if actualURLsPerMinute >= float64(targetURLsPerMinute) {
		fmt.Printf("✓ Performance target ACHIEVED\n")
	} else {
		fmt.Printf("✗ Performance target not met (%.2f%% of target)\n", 
			actualURLsPerMinute/float64(targetURLsPerMinute)*100)
	}

	// Memory usage estimate
	fmt.Printf("\nMemory efficiency: Using concurrent processing with 8 workers\n")
	fmt.Printf("Concurrent processing prevents memory spikes from loading all sitemaps at once\n")

	// Save results to file
	if err := saveResults(results, "performance_test_results.json"); err != nil {
		log.Printf("Failed to save results: %v", err)
	} else {
		fmt.Printf("\nResults saved to performance_test_results.json\n")
	}
}

func saveResults(results []*monitor.MonitorResult, filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}