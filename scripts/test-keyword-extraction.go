package main

import (
	"fmt"
	"sitemap-go/pkg/extractor"
)

func main() {
	fmt.Println("=== Testing Keyword Extraction ===\n")

	// Test the specific URL mentioned
	testURL := "https://www.megaigry.ru/online-game/roblox-steal-a-brainrot/"
	
	keywordExtractor := extractor.NewURLKeywordExtractor()
	
	fmt.Printf("Testing URL: %s\n", testURL)
	keywords, err := keywordExtractor.Extract(testURL)
	if err != nil {
		fmt.Printf("❌ Extraction failed: %v\n", err)
		return
	}
	
	fmt.Printf("Extracted keywords (%d): %v\n", len(keywords), keywords)
	
	// Test a few more URLs
	testURLs := []string{
		"https://www.megaigry.ru/online-game/geometry-vibes-3d/",
		"https://basake.itch.io/not-a-rage-game-for-sure-you-can-try",
		"https://siegelord.itch.io/gula",
	}
	
	fmt.Println("\nTesting additional URLs:")
	totalKeywords := 0
	for i, url := range testURLs {
		keywords, err := keywordExtractor.Extract(url)
		if err != nil {
			fmt.Printf("%d. ❌ %s - Failed: %v\n", i+1, url, err)
			continue
		}
		totalKeywords += len(keywords)
		fmt.Printf("%d. %s\n", i+1, url)
		fmt.Printf("   Keywords (%d): %v\n", len(keywords), keywords)
	}
	
	fmt.Printf("\nSummary: %d URLs produced %d total keywords\n", len(testURLs), totalKeywords)
	fmt.Printf("Average: %.2f keywords per URL\n", float64(totalKeywords)/float64(len(testURLs)))
}