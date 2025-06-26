package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"sitemap-go/pkg/monitor"
)

func main() {
	fmt.Println("=== Testing Previously Failed RSS URLs ===\n")

	// Test RSS URLs that failed in the original test
	failedRSSURLs := []string{
		"https://itch.io/feed/new.xml",
		"https://www.megaigry.ru/rss/",
	}

	// Set test environment variables
	os.Setenv("TREND_API_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_API_KEY", "test-key")
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	config := monitor.MonitorConfig{
		SitemapURLs:     failedRSSURLs,
		TrendAPIBaseURL: "https://httpbin.org",
		BackendBaseURL:  "https://httpbin.org",
		BackendAPIKey:   "test-key",
		EncryptionKey:   "test-encryption-key",
		WorkerPoolSize:  8,
	}

	sitemapMonitor, err := monitor.NewSitemapMonitor(config)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	fmt.Printf("Testing %d previously failed RSS URLs...\n", len(failedRSSURLs))
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	results, err := sitemapMonitor.MonitorSitemaps(ctx, config)
	if err != nil {
		log.Printf("Monitoring completed with errors: %v", err)
	}

	totalDuration := time.Since(startTime)
	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Duration: %v\n", totalDuration)
	fmt.Printf("Total results: %d\n", len(results))

	successful := 0
	totalKeywords := 0
	totalURLs := 0

	for i, result := range results {
		fmt.Printf("\n%d. %s\n", i+1, result.SitemapURL)
		if result.Success {
			successful++
			totalKeywords += len(result.Keywords)
			if urlCount, ok := result.Metadata["url_count"].(int); ok {
				totalURLs += urlCount
			}
			fmt.Printf("   ✅ SUCCESS: %d keywords, %d URLs\n", len(result.Keywords), getURLCount(result))
			if len(result.Keywords) > 0 {
				fmt.Printf("   Sample keywords: %v\n", result.Keywords[:min(5, len(result.Keywords))])
			}
		} else {
			fmt.Printf("   ❌ FAILED: %s\n", result.Error)
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Successful: %d/%d (%.1f%%)\n", successful, len(results), float64(successful)/float64(len(results))*100)
	fmt.Printf("Total Keywords: %d\n", totalKeywords)
	fmt.Printf("Total URLs: %d\n", totalURLs)

	if successful == len(failedRSSURLs) {
		fmt.Printf("🎉 All previously failed RSS URLs now work!\n")
	} else {
		fmt.Printf("⚠️  Some RSS URLs still failing\n")
	}
}

func getURLCount(result *monitor.MonitorResult) int {
	if count, ok := result.Metadata["url_count"].(int); ok {
		return count
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}