package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"sitemap-go/pkg/monitor"
	"sitemap-go/pkg/parser"
)

func main() {
	fmt.Println("=== Testing RSS Parsing ===\n")

	// Test RSS URLs that failed in previous test
	rssURLs := []string{
		"https://itch.io/feed/new.xml",
		"https://www.megaigry.ru/rss/",
	}

	// Test factory first
	fmt.Println("1. Testing Parser Factory...")
	factory := parser.GetParserFactory()
	
	for _, testURL := range rssURLs {
		fmt.Printf("\nTesting URL: %s\n", testURL)
		
		// Test format detection logic (simulate monitor logic)
		format := determineFormat(testURL)
		fmt.Printf("Detected format: %s\n", format)
		
		// Get parser from factory
		sitemapParser := factory.GetParser(format)
		if sitemapParser == nil {
			fmt.Printf("❌ No parser found for format: %s\n", format)
			continue
		}
		fmt.Printf("✅ Parser found for format: %s\n", format)
		
		// Test direct parsing
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		fmt.Printf("Attempting to parse with %s parser...\n", format)
		urls, err := sitemapParser.Parse(ctx, testURL)
		if err != nil {
			fmt.Printf("❌ Parsing failed: %v\n", err)
		} else {
			fmt.Printf("✅ Parsing successful: %d URLs found\n", len(urls))
			if len(urls) > 0 {
				fmt.Printf("Sample URLs:\n")
				for i, url := range urls[:min(3, len(urls))] {
					fmt.Printf("  %d. %s\n", i+1, url.Address)
					if title, ok := url.Metadata["title"]; ok {
						fmt.Printf("     Title: %s\n", title)
					}
				}
			}
		}
	}

	// Test full monitor workflow
	fmt.Println("\n2. Testing Full Monitor Workflow...")
	
	// Set test environment variables
	os.Setenv("TREND_API_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_BASE_URL", "https://httpbin.org") 
	os.Setenv("BACKEND_API_KEY", "test-key")
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	config := monitor.MonitorConfig{
		SitemapURLs:     []string{rssURLs[0]}, // Test just first RSS URL
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := sitemapMonitor.ProcessSitemap(ctx, rssURLs[0])
	if err != nil {
		fmt.Printf("❌ Monitor processing failed: %v\n", err)
	} else {
		fmt.Printf("✅ Monitor processing successful:\n")
		fmt.Printf("  URL: %s\n", result.SitemapURL)
		fmt.Printf("  Success: %t\n", result.Success)
		fmt.Printf("  Keywords: %d\n", len(result.Keywords))
		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}
	}
}

// Replicate the monitor's format detection logic
func determineFormat(sitemapURL string) string {
	// Check RSS/Feed patterns first (higher priority than file extension)
	if contains(sitemapURL, "rss") || contains(sitemapURL, "feed") {
		return "rss"
	}
	
	// Check file extensions
	if contains(sitemapURL, ".xml.gz") {
		return "xml.gz"
	}
	if contains(sitemapURL, ".txt") {
		return "txt"
	}
	if contains(sitemapURL, ".xml") {
		return "xml"
	}
	
	// Default to XML
	return "xml"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}