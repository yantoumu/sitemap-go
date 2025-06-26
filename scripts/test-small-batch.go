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
	fmt.Println("=== Testing Small Batch of Sitemaps ===\n")

	// Test a small batch of sitemaps that we know work
	testSitemaps := []string{
		"https://kiz10.com/sitemap-games.xml",
		"https://www.freegames.com/sitemap/games_1.xml", 
		"https://www.hoodamath.com/sitemap.xml",
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

	// Run test with shorter timeout 
	fmt.Printf("Testing %d sitemaps with result collection fix...\n", len(testSitemaps))
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
	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Duration: %v\n", totalDuration)
	fmt.Printf("Total results returned: %d\n", len(results))

	successful := 0
	failed := 0
	totalKeywords := 0

	for i, result := range results {
		fmt.Printf("\n%d. %s\n", i+1, result.SitemapURL)
		if result.Success {
			successful++
			totalKeywords += len(result.Keywords)
			urlCount := 0
			if count, ok := result.Metadata["url_count"].(int); ok {
				urlCount = count
			}
			fmt.Printf("   ✅ SUCCESS: %d keywords, %d URLs\n", len(result.Keywords), urlCount)
			if len(result.Keywords) > 0 {
				fmt.Printf("   Sample keywords: %v\n", result.Keywords[:min(5, len(result.Keywords))])
			}
		} else {
			failed++
			fmt.Printf("   ❌ FAILED: %s\n", result.Error)
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Total Keywords: %d\n", totalKeywords)

	if successful == len(testSitemaps) {
		fmt.Printf("🎉 All test sitemaps processed successfully!\n")
	} else {
		fmt.Printf("⚠️  Some sitemaps failed to process\n")
	}

	// Save results
	if err := saveResults(results, "small_batch_results.json"); err != nil {
		log.Printf("Failed to save results: %v", err)
	} else {
		fmt.Printf("Results saved to small_batch_results.json\n")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func saveResults(results []*monitor.MonitorResult, filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}