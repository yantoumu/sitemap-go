package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"sitemap-go/pkg/monitor"
)

func main() {
	// Read the test results
	data, err := os.ReadFile("full_sitemap_test_results.json")
	if err != nil {
		fmt.Printf("Error reading results file: %v\n", err)
		return
	}

	var results []*monitor.MonitorResult
	if err := json.Unmarshal(data, &results); err != nil {
		fmt.Printf("Error parsing results: %v\n", err)
		return
	}

	fmt.Println("=== SITEMAP PARSING TEST RESULTS ===\n")

	// Separate successful and failed results
	successful := []*monitor.MonitorResult{}
	failed := []*monitor.MonitorResult{}

	for _, result := range results {
		if result.Success {
			successful = append(successful, result)
		} else {
			failed = append(failed, result)
		}
	}

	// Print summary
	fmt.Printf("Total Tested: %d\n", len(results))
	fmt.Printf("✅ Successful: %d (%.1f%%)\n", len(successful), float64(len(successful))/float64(len(results))*100)
	fmt.Printf("❌ Failed: %d (%.1f%%)\n\n", len(failed), float64(len(failed))/float64(len(results))*100)

	// Sort successful by keyword count
	sort.Slice(successful, func(i, j int) bool {
		return len(successful[i].Keywords) > len(successful[j].Keywords)
	})

	// Print successful results
	fmt.Printf("=== ✅ SUCCESSFUL SITEMAPS (%d) ===\n", len(successful))
	totalKeywords := 0
	totalURLs := 0
	
	for i, result := range successful {
		urlCount := 0
		if count, ok := result.Metadata["url_count"].(float64); ok {
			urlCount = int(count)
		}
		totalURLs += urlCount
		totalKeywords += len(result.Keywords)

		fmt.Printf("%d. %s\n", i+1, result.SitemapURL)
		fmt.Printf("   Keywords: %d, URLs: %d\n", len(result.Keywords), urlCount)
		
		// Show top keywords
		if len(result.Keywords) > 0 {
			topKeywords := result.Keywords
			if len(topKeywords) > 5 {
				topKeywords = topKeywords[:5]
			}
			fmt.Printf("   Top keywords: %v\n", topKeywords)
		}
		fmt.Println()
	}

	fmt.Printf("Total Keywords Extracted: %d\n", totalKeywords)
	fmt.Printf("Total URLs Processed: %d\n\n", totalURLs)

	// Analyze failures by error type
	fmt.Printf("=== ❌ FAILED SITEMAPS (%d) ===\n", len(failed))
	
	errorGroups := make(map[string][]*monitor.MonitorResult)
	for _, result := range failed {
		errorType := categorizeError(result.Error)
		errorGroups[errorType] = append(errorGroups[errorType], result)
	}

	// Print grouped errors
	for errorType, results := range errorGroups {
		fmt.Printf("\n%s (%d failures):\n", errorType, len(results))
		for _, result := range results {
			fmt.Printf("  • %s\n", result.SitemapURL)
			if len(result.Error) > 100 {
				fmt.Printf("    Error: %s...\n", result.Error[:100])
			} else {
				fmt.Printf("    Error: %s\n", result.Error)
			}
		}
	}

	// Generate list of failed URLs for debugging
	fmt.Printf("\n=== FAILED URLs FOR DEBUGGING ===\n")
	for _, result := range failed {
		fmt.Printf("\"%s\", // %s\n", result.SitemapURL, categorizeError(result.Error))
	}

	// Print parsing format analysis
	fmt.Printf("\n=== FORMAT ANALYSIS ===\n")
	formatStats := make(map[string]int)
	formatSuccess := make(map[string]int)
	
	for _, result := range results {
		format := detectFormat(result.SitemapURL)
		formatStats[format]++
		if result.Success {
			formatSuccess[format]++
		}
	}

	for format, total := range formatStats {
		success := formatSuccess[format]
		fmt.Printf("%s: %d/%d successful (%.1f%%)\n", 
			format, success, total, float64(success)/float64(total)*100)
	}
}

func categorizeError(errorMsg string) string {
	if errorMsg == "" {
		return "Unknown Error"
	}

	errorLower := strings.ToLower(errorMsg)
	
	switch {
	case strings.Contains(errorLower, "403") || strings.Contains(errorLower, "forbidden"):
		return "🚫 Access Forbidden (403)"
	case strings.Contains(errorLower, "404") || strings.Contains(errorLower, "not found"):
		return "🔍 Not Found (404)"
	case strings.Contains(errorLower, "timeout") || strings.Contains(errorLower, "deadline"):
		return "⏱️ Timeout"
	case strings.Contains(errorLower, "connection") || strings.Contains(errorLower, "network"):
		return "🌐 Network Error"
	case strings.Contains(errorLower, "invalid character entity") || strings.Contains(errorLower, "no semicolon"):
		return "🔤 XML Encoding Error"
	case strings.Contains(errorLower, "expected element type") || strings.Contains(errorLower, "rss"):
		return "📰 RSS Format (not XML sitemap)"
	case strings.Contains(errorLower, "illegal character code"):
		return "💢 Illegal Character in XML"
	case strings.Contains(errorLower, "parse") || strings.Contains(errorLower, "xml") || strings.Contains(errorLower, "syntax"):
		return "📝 XML Parsing Error"
	case strings.Contains(errorLower, "502") || strings.Contains(errorLower, "503") || strings.Contains(errorLower, "500"):
		return "🖥️ Server Error (5xx)"
	default:
		return "❓ Other Error"
	}
}

func detectFormat(url string) string {
	urlLower := strings.ToLower(url)
	switch {
	case strings.Contains(urlLower, ".xml.gz"):
		return "XML.GZ (Compressed)"
	case strings.Contains(urlLower, ".txt"):
		return "TXT (Text)"
	case strings.Contains(urlLower, "rss") || strings.Contains(urlLower, "feed"):
		return "RSS (Feed)"
	case strings.Contains(urlLower, ".xml"):
		return "XML (Standard)"
	default:
		return "Unknown Format"
	}
}