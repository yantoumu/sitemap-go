package main

import (
	"fmt"
	"sitemap-go/pkg/extractor"
)

func main() {
	fmt.Println("=== Testing Hyphenated Keywords ===\n")

	// Test the specific URL mentioned
	testURL := "https://aleessitah.itch.io/freeze-ta-flame-nivel-2"
	
	keywordExtractor := extractor.NewURLKeywordExtractor()
	
	fmt.Printf("Testing URL: %s\n", testURL)
	keywords, err := keywordExtractor.Extract(testURL)
	if err != nil {
		fmt.Printf("❌ Extraction failed: %v\n", err)
		return
	}
	
	fmt.Printf("Current extraction (%d keywords): %v\n", len(keywords), keywords)
	
	// Test more hyphenated game names
	testURLs := []string{
		"https://example.com/geometry-dash-world",
		"https://example.com/super-mario-bros",
		"https://example.com/call-of-duty-modern-warfare",
		"https://example.com/the-legend-of-zelda",
		"https://example.com/temple-run-2",
	}
	
	fmt.Println("\nTesting additional hyphenated game names:")
	for i, url := range testURLs {
		keywords, err := keywordExtractor.Extract(url)
		if err != nil {
			fmt.Printf("%d. ❌ %s - Failed: %v\n", i+1, url, err)
			continue
		}
		fmt.Printf("%d. %s\n", i+1, url)
		fmt.Printf("   Keywords: %v\n", keywords)
	}
	
	fmt.Println("\n=== Expected vs Current ===")
	fmt.Printf("URL: freeze-ta-flame-nivel-2\n")
	fmt.Printf("Expected: [\"freeze ta flame nivel 2\"] or similar\n")
	fmt.Printf("Current:  %v\n", keywords)
}