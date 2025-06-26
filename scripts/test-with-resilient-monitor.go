package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
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
	fmt.Println("=== Testing All 57 Sitemaps with Resilient Monitor ===\n")

	// All 57 sitemaps
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

	// Set environment variables
	os.Setenv("TREND_API_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_BASE_URL", "https://httpbin.org")
	os.Setenv("BACKEND_API_KEY", "test-key")
	os.Setenv("ENCRYPTION_KEY", "test-encryption-key")

	// Create config
	config := monitor.MonitorConfig{
		SitemapURLs:     sitemaps,
		TrendAPIBaseURL: "https://httpbin.org",
		BackendBaseURL:  "https://httpbin.org",
		BackendAPIKey:   "test-key",
		EncryptionKey:   "test-encryption-key",
		WorkerPoolSize:  20, // Increased concurrency
	}

	// Create resilient monitor
	resilientMonitor, err := monitor.NewResilientSitemapMonitor(config)
	if err != nil {
		log.Fatalf("Failed to create resilient monitor: %v", err)
	}

	fmt.Printf("🚀 Testing %d sitemaps with Resilient Monitor (20 concurrent workers)...\n\n", len(sitemaps))
	
	startTime := time.Now()
	
	// Process all sitemaps with batch processing
	ctx := context.Background()
	results, err := resilientMonitor.BatchProcessSitemaps(ctx, sitemaps, 20)
	if err != nil {
		log.Fatalf("Batch processing failed: %v", err)
	}
	
	totalTime := time.Since(startTime)
	
	// Analyze results
	var successful []TestResult
	var failed []TestResult
	totalURLs := 0
	totalKeywords := 0
	
	for _, result := range results {
		testResult := TestResult{
			URL:     result.SitemapURL,
			Success: result.Success,
			Error:   result.Error,
		}
		
		if urlCount, ok := result.Metadata["url_count"].(int); ok {
			testResult.URLCount = urlCount
			totalURLs += urlCount
		}
		
		testResult.KeywordCount = len(result.Keywords)
		totalKeywords += testResult.KeywordCount
		
		if result.Success {
			successful = append(successful, testResult)
		} else {
			failed = append(failed, testResult)
		}
	}
	
	// Print summary
	fmt.Printf("\n✅ All tests completed in %v\n\n", totalTime.Round(time.Second))
	fmt.Printf("=== Summary ===\n")
	fmt.Printf("Total sitemaps: %d\n", len(sitemaps))
	fmt.Printf("✅ Successful: %d (%.1f%%)\n", len(successful), float64(len(successful))/float64(len(sitemaps))*100)
	fmt.Printf("❌ Failed: %d (%.1f%%)\n", len(failed), float64(len(failed))/float64(len(sitemaps))*100)
	fmt.Printf("Total URLs processed: %d\n", totalURLs)
	fmt.Printf("Total keywords extracted: %d\n", totalKeywords)
	if totalURLs > 0 {
		fmt.Printf("Average keywords/URL ratio: %.2f\n", float64(totalKeywords)/float64(totalURLs))
	}
	
	// Print failed sitemaps
	if len(failed) > 0 {
		fmt.Printf("\n=== Failed Sitemaps (%d) ===\n", len(failed))
		
		// Group by error type
		errorGroups := make(map[string][]TestResult)
		for _, result := range failed {
			errorKey := simplifyError(result.Error)
			errorGroups[errorKey] = append(errorGroups[errorKey], result)
		}
		
		// Sort error types by frequency
		type errorGroup struct {
			Type  string
			Count int
			Sites []TestResult
		}
		var sortedGroups []errorGroup
		for errorType, sites := range errorGroups {
			sortedGroups = append(sortedGroups, errorGroup{
				Type:  errorType,
				Count: len(sites),
				Sites: sites,
			})
		}
		sort.Slice(sortedGroups, func(i, j int) bool {
			return sortedGroups[i].Count > sortedGroups[j].Count
		})
		
		// Print each error group
		for _, group := range sortedGroups {
			fmt.Printf("\n🔴 %s (%d sites):\n", group.Type, group.Count)
			for _, result := range group.Sites {
				fmt.Printf("   - %s\n", result.URL)
			}
		}
		
		// List of sitemaps that still need fixes
		fmt.Printf("\n=== Sites Still Requiring Attention ===\n")
		for _, result := range failed {
			fmt.Printf("- %s\n", result.URL)
		}
	} else {
		fmt.Printf("\n🎉 All sitemaps processed successfully!\n")
	}
	
	// Show performance metrics
	fmt.Printf("\n=== Performance Metrics ===\n")
	fmt.Printf("Total processing time: %v\n", totalTime.Round(time.Second))
	fmt.Printf("Average time per sitemap: %v\n", (totalTime / time.Duration(len(sitemaps))).Round(time.Millisecond))
	fmt.Printf("Processing rate: %.1f sitemaps/minute\n", float64(len(sitemaps))/totalTime.Minutes())
}

func simplifyError(err string) string {
	if contains(err, "403") {
		return "HTTP 403 Forbidden (Access Denied)"
	}
	if contains(err, "404") {
		return "HTTP 404 Not Found"
	}
	if contains(err, "timeout") || contains(err, "deadline") {
		return "Timeout/Deadline Exceeded"
	}
	if contains(err, "XML") || contains(err, "xml syntax") {
		return "XML Parsing Error"
	}
	if contains(err, "encoding") || contains(err, "UTF") {
		return "Character Encoding Error"
	}
	if contains(err, "connection refused") {
		return "Connection Refused"
	}
	if contains(err, "no such host") {
		return "DNS Resolution Failed"
	}
	if contains(err, "all") && contains(err, "strategies failed") {
		return "All Parsing Strategies Failed"
	}
	if contains(err, "unsupported") && contains(err, "format") {
		return "Unsupported Sitemap Format"
	}
	
	// Return first 60 chars if no match
	if len(err) > 60 {
		return err[:60] + "..."
	}
	return err
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}