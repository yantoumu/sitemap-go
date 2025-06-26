package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sitemap-go/pkg/monitor"
)

func main() {
	// Focus on previously failed sitemaps
	failedSitemaps := []string{
		"https://www.brightestgames.com/games-sitemap.xml",
		"https://www.puzzleplayground.com/sitemap.xml",
		"https://kizgame.com/sitemap-en.xml",
		"https://wordle2.io/sitemap.xml",
		"https://www.play-games.com/sitemap.xml",
		"https://superkidgames.com/sitemap.xml",
		"https://sprunki.org/sitemap.xml",
		// Add more from test results
		"https://www.gamearter.com/sitemap",
		"https://lagged.com/sitemap.txt",
	}

	os.Setenv("TREND_API_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_API_KEY", "test-key")
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	config := monitor.MonitorConfig{
		SitemapURLs:     failedSitemaps,
		TrendAPIBaseURL: "https://httpbin.org",
		BackendBaseURL:  "https://httpbin.org",
		BackendAPIKey:   "test-key",
		EncryptionKey:   "test-encryption-key",
		WorkerPoolSize:  8,
	}

	sitemapMonitor, err := monitor.NewSitemapMonitor(config)
	if err != nil {
		fmt.Printf("Failed to create monitor: %v\n", err)
		return
	}

	fmt.Printf("=== Testing %d Failed Sitemaps ===\n\n", len(failedSitemaps))

	for i, sitemapURL := range failedSitemaps {
		fmt.Printf("%d. Testing: %s\n", i+1, sitemapURL)
		
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := sitemapMonitor.ProcessSitemap(ctx, sitemapURL)
		cancel()
		
		if err != nil {
			fmt.Printf("   ❌ FAILED: %v\n", err)
		} else if result != nil && result.Success {
			urlCount := getURLCount(result)
			fmt.Printf("   ✅ SUCCESS: %d URLs → %d keywords\n", urlCount, len(result.Keywords))
		} else {
			fmt.Printf("   ❌ FAILED: %s\n", result.Error)
		}
		fmt.Println()
	}
}

func getURLCount(result *monitor.MonitorResult) int {
	if count, ok := result.Metadata["url_count"].(int); ok {
		return count
	}
	return 0
}