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
	fmt.Println("=== Testing Primary Keyword Selection ===\n")

	// Test RSS URL to see keyword selection
	testURL := "https://www.megaigry.ru/rss/"

	// Set test environment variables
	os.Setenv("TREND_API_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_API_KEY", "test-key")
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	config := monitor.MonitorConfig{
		SitemapURLs:     []string{testURL},
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

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	result, err := sitemapMonitor.ProcessSitemap(ctx, testURL)
	if err != nil {
		fmt.Printf("❌ Processing failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Processing successful!\n")
	fmt.Printf("URL: %s\n", result.SitemapURL)
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("URLs found: %d\n", getURLCount(result))
	fmt.Printf("Keywords extracted: %d\n", len(result.Keywords))
	fmt.Printf("Keywords: %v\n", result.Keywords)
	
	// Show ratio
	if urlCount := getURLCount(result); urlCount > 0 {
		ratio := float64(len(result.Keywords)) / float64(urlCount)
		fmt.Printf("Keyword/URL ratio: %.2f (should be 1.00)\n", ratio)
		
		if ratio == 1.0 {
			fmt.Printf("🎉 Perfect! Each URL now has exactly 1 primary keyword\n")
		} else {
			fmt.Printf("⚠️  Ratio is not 1:1\n")
		}
	}
	
	// Show sample keywords to verify quality
	if len(result.Keywords) > 0 {
		fmt.Printf("\nSample primary keywords (showing quality of selection):\n")
		for i, keyword := range result.Keywords[:min(10, len(result.Keywords))] {
			fmt.Printf("  %d. \"%s\"\n", i+1, keyword)
		}
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