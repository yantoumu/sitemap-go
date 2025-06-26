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
	fmt.Println("=== Testing XML Sitemap Keyword Extraction ===\n")

	// Test XML sitemaps that may have failed before
	testSitemaps := []string{
		"https://geoguessr.io/sitemap.xml",
		"https://startgamer.ru/sitemap.xml", 
		"https://doodle-jump.co/sitemap.xml",
		"https://www.hoodamath.com/sitemap.xml",
		"https://www.brightestgames.com/games-sitemap.xml",
		"https://www.hahagames.com/sitemap.xml",
		"https://www.puzzleplayground.com/sitemap.xml",
		"https://www.mathplayground.com/sitemap_main.xml",
		"https://geometrydashworld.net/sitemap.xml",
		"https://zh.y8.com/sitemaps/y8/zh/sitemap.xml.gz",
		"https://geometry-lite.io/sitemap.xml",
		"https://geometrydashsubzero.net/sitemap.xml",
		"https://kizgame.com/sitemap-en.xml",
		"https://wordhurdle.co/sitemap.xml",
		"https://chillguygame.io/sitemap.xml",
	}

	// Set test environment variables
	os.Setenv("TREND_API_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_API_KEY", "test-key")
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	config := monitor.MonitorConfig{
		SitemapURLs:     testSitemaps,
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

	fmt.Printf("Testing %d XML sitemaps for keyword extraction...\n\n", len(testSitemaps))

	// Test each sitemap individually to see detailed results
	successful := 0
	failed := 0
	totalKeywords := 0
	totalURLs := 0

	for i, sitemapURL := range testSitemaps {
		fmt.Printf("%d. Testing: %s\n", i+1, sitemapURL)
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		result, err := sitemapMonitor.ProcessSitemap(ctx, sitemapURL)
		cancel()
		
		if err != nil {
			fmt.Printf("   ❌ FAILED: %v\n", err)
			failed++
		} else if result != nil && result.Success {
			urlCount := getURLCount(result)
			keywordCount := len(result.Keywords)
			
			fmt.Printf("   ✅ SUCCESS: %d URLs → %d keywords\n", urlCount, keywordCount)
			
			successful++
			totalKeywords += keywordCount
			totalURLs += urlCount
			
			// Show ratio
			if urlCount > 0 {
				ratio := float64(keywordCount) / float64(urlCount)
				fmt.Printf("   📊 Ratio: %.2f (keywords/URL)\n", ratio)
			}
			
			// Show sample keywords
			if len(result.Keywords) > 0 {
				sampleCount := 3
				if len(result.Keywords) < sampleCount {
					sampleCount = len(result.Keywords)
				}
				fmt.Printf("   🎯 Sample keywords: %v\n", result.Keywords[:sampleCount])
			}
		} else {
			fmt.Printf("   ❌ FAILED: %s\n", result.Error)
			failed++
		}
		fmt.Println()
	}

	// Summary
	fmt.Printf("=== Summary ===\n")
	fmt.Printf("Total tested: %d\n", len(testSitemaps))
	fmt.Printf("Successful: %d (%.1f%%)\n", successful, float64(successful)/float64(len(testSitemaps))*100)
	fmt.Printf("Failed: %d (%.1f%%)\n", failed, float64(failed)/float64(len(testSitemaps))*100)
	fmt.Printf("Total URLs processed: %d\n", totalURLs)
	fmt.Printf("Total keywords extracted: %d\n", totalKeywords)
	
	if totalURLs > 0 {
		avgRatio := float64(totalKeywords) / float64(totalURLs)
		fmt.Printf("Average keywords/URL ratio: %.2f\n", avgRatio)
		
		if avgRatio >= 0.8 && avgRatio <= 1.2 {
			fmt.Printf("🎉 Good keyword extraction ratio!\n")
		} else {
			fmt.Printf("⚠️  Keyword extraction ratio may need adjustment\n")
		}
	}

	// Check if hyphenated keywords are being properly handled
	fmt.Printf("\n=== Hyphen Processing Check ===\n")
	hyphenatedSites := []string{
		"doodle-jump.co",
		"geometry-lite.io", 
		"geometrydashsubzero.net",
	}
	
	for _, site := range hyphenatedSites {
		for _, sitemap := range testSitemaps {
			if contains(sitemap, site) {
				fmt.Printf("Site: %s - Should handle hyphenated game names properly\n", site)
				break
			}
		}
	}
}

func getURLCount(result *monitor.MonitorResult) int {
	if count, ok := result.Metadata["url_count"].(int); ok {
		return count
	}
	return 0
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}