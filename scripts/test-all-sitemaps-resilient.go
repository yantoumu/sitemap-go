package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"sitemap-go/pkg/monitor"
)

type TestResult struct {
	URL          string
	Success      bool
	URLCount     int
	KeywordCount int
	Error        string
	Duration     time.Duration
}

func main() {
	fmt.Println("=== Testing All 57 Sitemaps with High Concurrency ===\n")

	// All 57 sitemaps to test
	sitemaps := []string{
		"https://1games.io/sitemap.xml",
		"https://azgames.io/sitemap.xml",
		"https://baldigames.com/sitemap.xml",
		"https://game-game.com/sitemap.xml",
		"https://geometry-free.com/sitemap.xml",
		"https://geometrydash.io/sitemap.xml",
		"https://googledoodlegames.net/sitemap.xml",
		"https://html5.gamedistribution.com/sitemap.xml",
		"https://itch.io/feed/new.xml",
		"https://kiz10.com/sitemap-games.xml",
		"https://kizi.com/sitemaps/kizi/en/sitemap_games.xml.gz",
		"https://lagged.com/sitemap.txt",
		"https://nointernetgame.com/game-sitemap.xml",
		"https://playgama.com/sitemap-2.xml",
		"https://playtropolis.com/sitemap.games.xml",
		"https://pokerogue.io/sitemap.xml",
		"https://poki.com/en/sitemaps/index.xml",
		"https://ssgames.site/sitemap.xml",
		"https://wordle2.io/sitemap.xml",
		"https://www.1001games.com/sitemap-games.xml",
		"https://www.1001jeux.fr/sitemap-games.xml",
		"https://www.freegames.com/sitemap/games_1.xml",
		"https://www.gamearter.com/sitemap",
		"https://www.minigiochi.com/sitemap-games-3.xml",
		"https://www.onlinegames.io/sitemap.xml",
		"https://www.play-games.com/sitemap.xml",
		"https://www.playgame24.com/sitemap.xml",
		"https://www.twoplayergames.org/sitemap-games.xml",
		"https://keygames.com/games-sitemap.xml",
		"https://www.snokido.com/sitemaps/games.xml",
		"https://www.miniplay.com/sitemap-games-3.xml",
		"https://sprunki.org/sitemap.xml",
		"https://geometrygame.org/sitemap.xml",
		"https://kiz10.com/sitemap-games-2.xml",
		"https://sprunkigo.com/en/sitemap.xml",
		"https://sprunki.com/sitemap.xml",
		"https://www.sprunky.org/sitemap.xml",
		"https://www.megaigry.ru/rss/",
		"https://superkidgames.com/sitemap.xml",
		"https://www.gamesgames.com/sitemaps/gamesgames/en/sitemap_games.xml.gz",
		"https://www.spel.nl/sitemaps/agame/nl/sitemap_games.xml.gz",
		"https://www.girlsgogames.it/sitemaps/girlsgogames/it/sitemap_games.xml.gz",
		"https://www.games.co.id/sitemaps/agame/id/sitemap_games.xml.gz",
		"https://www.newgrounds.com/sitemaps/art/sitemap.94.xml",
		"https://www.topigre.net/sitemap.xml",
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

	// Create monitor with high concurrency
	config := monitor.MonitorConfig{
		SitemapURLs:     sitemaps,
		TrendAPIBaseURL: "https://httpbin.org",
		BackendBaseURL:  "https://httpbin.org",
		BackendAPIKey:   "test-key",
		EncryptionKey:   "test-encryption-key",
		WorkerPoolSize:  57, // One worker per sitemap for maximum concurrency
	}

	sitemapMonitor, err := monitor.NewSitemapMonitor(config)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	fmt.Printf("🚀 Testing %d sitemaps with %d concurrent workers...\n\n", len(sitemaps), config.WorkerPoolSize)

	// Results storage
	results := make([]TestResult, len(sitemaps))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Progress tracking
	completed := 0
	startTime := time.Now()

	// Test each sitemap concurrently
	for i, sitemapURL := range sitemaps {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()

			testStart := time.Now()
			result := TestResult{URL: url}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Process sitemap
			monitorResult, err := sitemapMonitor.ProcessSitemap(ctx, url)
			result.Duration = time.Since(testStart)

			if err != nil {
				result.Success = false
				result.Error = err.Error()
			} else if monitorResult != nil {
				if monitorResult.Success {
					result.Success = true
					result.URLCount = getURLCount(monitorResult)
					result.KeywordCount = len(monitorResult.Keywords)
				} else {
					result.Success = false
					result.Error = monitorResult.Error
				}
			} else {
				result.Success = false
				result.Error = "nil result"
			}

			// Store result
			mu.Lock()
			results[index] = result
			completed++
			progress := float64(completed) / float64(len(sitemaps)) * 100
			fmt.Printf("\r⏳ Progress: %d/%d (%.1f%%) - Elapsed: %v", 
				completed, len(sitemaps), progress, time.Since(startTime).Round(time.Second))
			mu.Unlock()
		}(i, sitemapURL)
	}

	// Wait for all tests to complete
	wg.Wait()
	fmt.Printf("\n\n✅ All tests completed in %v\n\n", time.Since(startTime).Round(time.Second))

	// Analyze results
	var successful []TestResult
	var failed []TestResult
	totalURLs := 0
	totalKeywords := 0

	for _, result := range results {
		if result.Success {
			successful = append(successful, result)
			totalURLs += result.URLCount
			totalKeywords += result.KeywordCount
		} else {
			failed = append(failed, result)
		}
	}

	// Sort failed by error type
	sort.Slice(failed, func(i, j int) bool {
		return failed[i].Error < failed[j].Error
	})

	// Print summary
	fmt.Printf("=== Summary ===\n")
	fmt.Printf("Total sitemaps: %d\n", len(sitemaps))
	fmt.Printf("✅ Successful: %d (%.1f%%)\n", len(successful), float64(len(successful))/float64(len(sitemaps))*100)
	fmt.Printf("❌ Failed: %d (%.1f%%)\n", len(failed), float64(len(failed))/float64(len(sitemaps))*100)
	fmt.Printf("Total URLs processed: %d\n", totalURLs)
	fmt.Printf("Total keywords extracted: %d\n", totalKeywords)
	if totalURLs > 0 {
		fmt.Printf("Average keywords/URL ratio: %.2f\n", float64(totalKeywords)/float64(totalURLs))
	}

	// Print failed sitemaps grouped by error
	if len(failed) > 0 {
		fmt.Printf("\n=== Failed Sitemaps (%d) ===\n", len(failed))
		
		// Group by error type
		errorGroups := make(map[string][]TestResult)
		for _, result := range failed {
			// Simplify error for grouping
			errorKey := simplifyError(result.Error)
			errorGroups[errorKey] = append(errorGroups[errorKey], result)
		}

		// Print each error group
		for errorType, group := range errorGroups {
			fmt.Printf("\n🔴 %s (%d sites):\n", errorType, len(group))
			for _, result := range group {
				fmt.Printf("   - %s\n", result.URL)
				if len(result.Error) < 100 {
					fmt.Printf("     Error: %s\n", result.Error)
				} else {
					fmt.Printf("     Error: %s...\n", result.Error[:100])
				}
			}
		}
	}

	// Print success details
	if len(successful) > 0 {
		fmt.Printf("\n=== Top Performing Sitemaps ===\n")
		// Sort by URL count
		sort.Slice(successful, func(i, j int) bool {
			return successful[i].URLCount > successful[j].URLCount
		})
		
		// Show top 10
		for i := 0; i < 10 && i < len(successful); i++ {
			result := successful[i]
			ratio := float64(result.KeywordCount) / float64(result.URLCount)
			fmt.Printf("%d. %s\n", i+1, result.URL)
			fmt.Printf("   URLs: %d, Keywords: %d, Ratio: %.2f, Time: %v\n", 
				result.URLCount, result.KeywordCount, ratio, result.Duration.Round(time.Millisecond))
		}
	}

	// Final recommendations
	fmt.Printf("\n=== Recommendations ===\n")
	if len(failed) > 0 {
		fmt.Printf("• %d sitemaps need further investigation\n", len(failed))
		
		// Count error types
		http403Count := 0
		xmlErrorCount := 0
		encodingCount := 0
		timeoutCount := 0
		
		for _, result := range failed {
			if contains(result.Error, "403") {
				http403Count++
			} else if contains(result.Error, "XML") || contains(result.Error, "xml") {
				xmlErrorCount++
			} else if contains(result.Error, "encoding") || contains(result.Error, "UTF") {
				encodingCount++
			} else if contains(result.Error, "timeout") || contains(result.Error, "deadline") {
				timeoutCount++
			}
		}
		
		if http403Count > 0 {
			fmt.Printf("• %d sites have HTTP 403 (access denied) - may need proxy or different headers\n", http403Count)
		}
		if xmlErrorCount > 0 {
			fmt.Printf("• %d sites have XML parsing errors - may need custom parsers\n", xmlErrorCount)
		}
		if encodingCount > 0 {
			fmt.Printf("• %d sites have encoding issues - may need charset detection\n", encodingCount)
		}
		if timeoutCount > 0 {
			fmt.Printf("• %d sites timed out - may need longer timeout or smaller batch size\n", timeoutCount)
		}
	} else {
		fmt.Printf("• All sitemaps processed successfully! 🎉\n")
	}
}

func getURLCount(result *monitor.MonitorResult) int {
	if count, ok := result.Metadata["url_count"].(int); ok {
		return count
	}
	return 0
}

func simplifyError(err string) string {
	if contains(err, "403") {
		return "HTTP 403 Forbidden"
	}
	if contains(err, "404") {
		return "HTTP 404 Not Found"
	}
	if contains(err, "timeout") || contains(err, "deadline") {
		return "Timeout"
	}
	if contains(err, "XML") || contains(err, "xml syntax") {
		return "XML Parsing Error"
	}
	if contains(err, "encoding") || contains(err, "UTF") {
		return "Encoding Error"
	}
	if contains(err, "connection refused") {
		return "Connection Refused"
	}
	if contains(err, "no such host") {
		return "DNS Resolution Failed"
	}
	if contains(err, "format") {
		return "Unsupported Format"
	}
	
	// Return first 50 chars if no match
	if len(err) > 50 {
		return err[:50] + "..."
	}
	return err
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

