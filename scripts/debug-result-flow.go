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
	fmt.Println("=== Debug Result Flow ===\n")

	// Test just one sitemap
	testSitemaps := []string{
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

	// Test the direct ProcessSitemap method first
	fmt.Println("Testing direct ProcessSitemap method...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := sitemapMonitor.ProcessSitemap(ctx, testSitemaps[0])
	if err != nil {
		fmt.Printf("Direct processing failed: %v\n", err)
	} else if result != nil {
		fmt.Printf("Direct processing succeeded:\n")
		fmt.Printf("  URL: %s\n", result.SitemapURL)
		fmt.Printf("  Success: %t\n", result.Success)
		fmt.Printf("  Keywords: %d\n", len(result.Keywords))
		if urlCount, ok := result.Metadata["url_count"].(int); ok {
			fmt.Printf("  URL count: %d\n", urlCount)
		}
		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}
	}

	fmt.Println("\nNow testing full MonitorSitemaps method...")

	// Now test the full workflow
	results, err := sitemapMonitor.MonitorSitemaps(ctx, config)
	if err != nil {
		log.Printf("Monitoring completed with errors: %v", err)
	}

	fmt.Printf("Results returned: %d\n", len(results))
	for i, result := range results {
		fmt.Printf("Result %d:\n", i+1)
		fmt.Printf("  URL: %s\n", result.SitemapURL)
		fmt.Printf("  Success: %t\n", result.Success)
		fmt.Printf("  Keywords: %d\n", len(result.Keywords))
		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}
	}
}