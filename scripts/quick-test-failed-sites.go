package main

import (
	"context"
	"fmt"
	"time"

	"sitemap-go/pkg/parser"
)

func main() {
	fmt.Println("=== Quick Test of Failed Sites with Resilient Parser ===\n")

	// Previously failed sites
	failedSites := []string{
		"https://www.brightestgames.com/games-sitemap.xml",
		"https://www.puzzleplayground.com/sitemap.xml",
		"https://kizgame.com/sitemap-en.xml",
		"https://wordle2.io/sitemap.xml",
		"https://www.play-games.com/sitemap.xml",
		"https://superkidgames.com/sitemap.xml",
		"https://sprunki.org/sitemap.xml",
	}

	// Create resilient parser factory
	factory := parser.NewResilientParserFactory()

	for i, sitemapURL := range failedSites {
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
		}
		fmt.Println()
	}
}