package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"sitemap-go/pkg/extractor"
	"sitemap-go/pkg/parser"
)

type TestResult struct {
	URL           string
	Success       bool
	URLCount      int
	KeywordCount  int
	Error         string
	Duration      time.Duration
	ParserUsed    string
	AttemptsCount int
}

func main() {
	fmt.Println("=== Final Comprehensive Test of All 57 Sitemaps ===")
	fmt.Println("=== Using Resilient Parser Factory with 20 Concurrent Workers ===\n")

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

	// Create resilient parser factory
	factory := parser.NewResilientParserFactory()
	keywordExtractor := extractor.NewURLKeywordExtractor()

	// Results storage
	results := make([]TestResult, len(sitemaps))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Progress tracking
	completed := 0
	startTime := time.Now()

	// Create worker pool
	workChan := make(chan int, len(sitemaps))
	for i := range sitemaps {
		workChan <- i
	}
	close(workChan)

	// Start workers
	numWorkers := 20
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for index := range workChan {
				sitemapURL := sitemaps[index]
				testStart := time.Now()
				result := TestResult{URL: sitemapURL}

				// Parse sitemap with resilient factory
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				urls, err := factory.Parse(ctx, sitemapURL)
				cancel()

				result.Duration = time.Since(testStart)

				if err != nil {
					result.Success = false
					result.Error = err.Error()
				} else {
					result.Success = true
					result.URLCount = len(urls)
					
					// Extract keywords
					keywordMap := make(map[string]bool)
					for _, url := range urls {
						keywords, _ := keywordExtractor.Extract(url.Address)
						if len(keywords) > 0 {
							// Select primary keyword (longest)
							longest := keywords[0]
							for _, kw := range keywords[1:] {
								if len(kw) > len(longest) {
									longest = kw
								}
							}
							keywordMap[longest] = true
						}
					}
					
					result.KeywordCount = len(keywordMap)
				}

				// Store result
				mu.Lock()
				results[index] = result
				completed++
				progress := float64(completed) / float64(len(sitemaps)) * 100
				fmt.Printf("\r⏳ Progress: %d/%d (%.1f%%) - Elapsed: %v", 
					completed, len(sitemaps), progress, time.Since(startTime).Round(time.Second))
				mu.Unlock()
			}
		}(w)
	}

	// Wait for completion
	wg.Wait()
	totalTime := time.Since(startTime)
	
	fmt.Printf("\n\n✅ All tests completed in %v\n\n", totalTime.Round(time.Second))

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

	// Print comprehensive summary
	fmt.Printf("=== FINAL SUMMARY ===\n")
	fmt.Printf("Total sitemaps tested: %d\n", len(sitemaps))
	fmt.Printf("✅ Successful: %d (%.1f%%)\n", len(successful), float64(len(successful))/float64(len(sitemaps))*100)
	fmt.Printf("❌ Failed: %d (%.1f%%)\n", len(failed), float64(len(failed))/float64(len(sitemaps))*100)
	fmt.Printf("Total URLs processed: %d\n", totalURLs)
	fmt.Printf("Total keywords extracted: %d\n", totalKeywords)
	if totalURLs > 0 {
		fmt.Printf("Average keywords/URL ratio: %.2f (Target: 1.00)\n", float64(totalKeywords)/float64(totalURLs))
	}

	// Performance metrics
	fmt.Printf("\n=== PERFORMANCE METRICS ===\n")
	fmt.Printf("Total processing time: %v\n", totalTime.Round(time.Second))
	fmt.Printf("Average time per sitemap: %v\n", (totalTime / time.Duration(len(sitemaps))).Round(time.Millisecond))
	fmt.Printf("Processing rate: %.1f sitemaps/minute\n", float64(len(sitemaps))/totalTime.Minutes())
	fmt.Printf("Concurrent workers: %d\n", numWorkers)

	// Show failed sites if any
	var errorGroups map[string][]string
	if len(failed) > 0 {
		fmt.Printf("\n=== FAILED SITEMAPS (%d) ===\n", len(failed))
		
		// Group by error type
		errorGroups = make(map[string][]string)
		for _, result := range failed {
			errorKey := simplifyError(result.Error)
			errorGroups[errorKey] = append(errorGroups[errorKey], result.URL)
		}
		
		for errorType, urls := range errorGroups {
			fmt.Printf("\n%s (%d sites):\n", errorType, len(urls))
			for _, url := range urls {
				fmt.Printf("  - %s\n", url)
			}
		}
	}

	// Show top performing sites
	if len(successful) > 0 {
		fmt.Printf("\n=== TOP 10 SITES BY URL COUNT ===\n")
		sort.Slice(successful, func(i, j int) bool {
			return successful[i].URLCount > successful[j].URLCount
		})
		
		for i := 0; i < 10 && i < len(successful); i++ {
			result := successful[i]
			ratio := float64(result.KeywordCount) / float64(result.URLCount)
			fmt.Printf("%d. %s\n", i+1, extractDomain(result.URL))
			fmt.Printf("   URLs: %d | Keywords: %d | Ratio: %.2f | Time: %v\n", 
				result.URLCount, result.KeywordCount, ratio, result.Duration.Round(time.Millisecond))
		}
	}

	// Final verdict
	fmt.Printf("\n=== FINAL VERDICT ===\n")
	successRate := float64(len(successful)) / float64(len(sitemaps)) * 100
	if successRate >= 95 {
		fmt.Printf("🎉 EXCELLENT: %.1f%% success rate - The resilient parser solution is highly effective!\n", successRate)
	} else if successRate >= 85 {
		fmt.Printf("✅ GOOD: %.1f%% success rate - Most sitemaps are being processed successfully.\n", successRate)
	} else if successRate >= 70 {
		fmt.Printf("⚠️  FAIR: %.1f%% success rate - Some improvements still needed.\n", successRate)
	} else {
		fmt.Printf("❌ NEEDS WORK: %.1f%% success rate - Significant issues remain.\n", successRate)
	}

	if len(failed) > 0 {
		fmt.Printf("\nRemaining issues to address:\n")
		for errorType := range errorGroups {
			fmt.Printf("- %s\n", errorType)
		}
	}
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
	if contains(err, "all") && contains(err, "strategies failed") {
		return "All Strategies Failed"
	}
	if contains(err, "unsupported") {
		return "Unsupported Format"
	}
	
	if len(err) > 50 {
		return err[:50] + "..."
	}
	return err
}

func extractDomain(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return url
}

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}