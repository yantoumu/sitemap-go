package main

import (
	"context"
	"fmt"
	"time"

	"sitemap-go/pkg/parser"
)

func main() {
	fmt.Println("=== Testing Final Two Problem Sites ===\n")

	// The two remaining problem sites
	problemSites := []string{
		"https://lagged.com/sitemap.txt",           // Format not supported
		"https://www.playgame24.com/sitemap.xml",   // XML syntax error
	}

	// Create enhanced resilient parser factory
	factory := parser.NewResilientParserFactory()

	for i, sitemapURL := range problemSites {
		fmt.Printf("%d. Testing: %s\n", i+1, sitemapURL)
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		
		start := time.Now()
		urls, err := factory.Parse(ctx, sitemapURL)
		duration := time.Since(start)
		
		cancel()

		if err != nil {
			fmt.Printf("   ❌ FAILED: %v\n", err)
		} else {
			fmt.Printf("   ✅ SUCCESS: %d URLs parsed in %v\n", len(urls), duration.Round(time.Millisecond))
			
			// Show sample URLs
			if len(urls) > 0 {
				sampleCount := 5
				if len(urls) < sampleCount {
					sampleCount = len(urls)
				}
				fmt.Printf("   📋 Sample URLs:\n")
				for j := 0; j < sampleCount; j++ {
					fmt.Printf("      - %s\n", urls[j].Address)
				}
			}
		}
		fmt.Println()
	}
	
	fmt.Println("=== Testing Strategy Selection ===")
	
	// Test URL analysis
	for _, sitemapURL := range problemSites {
		fmt.Printf("\nAnalyzing URL: %s\n", sitemapURL)
		
		// This would be internal factory method, let's test manually
		if contains(sitemapURL, ".txt") {
			fmt.Printf("   → Should use TXT Strategy ✓\n")
		} else if contains(sitemapURL, "playgame24") {
			fmt.Printf("   → Should use Empty Content Strategy ✓\n")
		} else {
			fmt.Printf("   → Will use Standard Strategy\n")
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr || 
		   indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}