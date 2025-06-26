package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"sitemap-go/pkg/monitor"
	"sitemap-go/pkg/logger"
)

func main() {
	fmt.Println("🚀 Starting Sitemap Keyword Deduplication Test")
	fmt.Println(strings.Repeat("=", 60))
	
	// Create security logger
	secureLog := logger.GetSecurityLogger()
	
	// 测试用的sitemap URL列表
	testSitemaps := []string{
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
	
	fmt.Printf("📋 Testing %d sitemap URLs\n", len(testSitemaps))
	fmt.Println()
	
	// 创建监控器实例 (测试模式不需要API调用)
	sitemapMonitor, err := monitor.NewMonitorConfigBuilder().
		WithTrendsAPI("http://test-trends-api.com").
		BuildForTesting()
	if err != nil {
		fmt.Printf("❌ Error creating sitemap monitor: %v\n", err)
		os.Exit(1)
	}
	
	// 设置上下文和超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	startTime := time.Now()
	
	// 步骤1: 提取所有关键词
	fmt.Println("🔍 Step 1: Extracting keywords from all sitemaps...")
	allKeywords, keywordToURLMap, results, err := sitemapMonitor.ExtractAllKeywords(ctx, testSitemaps, 20)
	if err != nil {
		fmt.Printf("❌ Error extracting keywords: %v\n", err)
		os.Exit(1)
	}
	
	// 统计成功和失败的sitemap
	successCount := 0
	failedSitemaps := []string{}
	failureReasons := make(map[string]string)
	errorCategories := make(map[string]int)
	
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			maskedURL := secureLog.MaskSitemapURL(result.SitemapURL)
			failedSitemaps = append(failedSitemaps, maskedURL)
			failureReasons[maskedURL] = result.Error
			
			// 分类错误类型
			errorType := categorizeError(result.Error)
			errorCategories[errorType]++
		}
	}
	
	fmt.Printf("✅ Successfully processed: %d/%d sitemaps\n", successCount, len(testSitemaps))
	if len(failedSitemaps) > 0 {
		fmt.Printf("❌ Failed sitemaps: %d\n", len(failedSitemaps))
		
		// 显示错误分类统计
		fmt.Printf("\n📊 Error Categories:\n")
		for errorType, count := range errorCategories {
			fmt.Printf("   - %s: %d\n", errorType, count)
		}
		
		// 显示具体失败原因
		fmt.Printf("\n🔍 Detailed Failures:\n")
		for i, failed := range failedSitemaps {
			fmt.Printf("   %d. %s\n", i+1, failed)
			fmt.Printf("      Error: %s\n", failureReasons[failed])
		}
	}
	fmt.Printf("📊 Total keywords before deduplication: %d\n", len(allKeywords))
	fmt.Println()
	
	// 步骤2: 全局去重
	fmt.Println("🔄 Step 2: Performing global keyword deduplication...")
	uniqueKeywords := deduplicateKeywords(allKeywords)
	
	deduplicationRatio := float64(len(uniqueKeywords)) / float64(len(allKeywords)) * 100
	savedRequests := len(allKeywords) - len(uniqueKeywords)
	
	fmt.Printf("📈 Deduplication Results:\n")
	fmt.Printf("   - Before: %d keywords\n", len(allKeywords))
	fmt.Printf("   - After:  %d unique keywords\n", len(uniqueKeywords))
	fmt.Printf("   - Saved:  %d duplicate requests (%.1f%% reduction)\n", savedRequests, 100-deduplicationRatio)
	fmt.Printf("   - Efficiency: %.1f%% unique keywords\n", deduplicationRatio)
	fmt.Println()
	
	// 步骤3: 分析关键词分布
	fmt.Println("📊 Step 3: Analyzing keyword distribution...")
	keywordFrequency := make(map[string]int)
	for _, keyword := range allKeywords {
		keywordFrequency[keyword]++
	}
	
	// 找出最常见的关键词
	var freqList []KeywordFreq
	for keyword, freq := range keywordFrequency {
		freqList = append(freqList, KeywordFreq{keyword, freq})
	}
	sort.Slice(freqList, func(i, j int) bool {
		return freqList[i].frequency > freqList[j].frequency
	})
	
	fmt.Printf("🏆 Top 20 most common keywords:\n")
	for i, kf := range freqList[:min(20, len(freqList))] {
		fmt.Printf("   %2d. %-25s (%d occurrences)\n", i+1, kf.keyword, kf.frequency)
	}
	fmt.Println()
	
	// 步骤4: 保存结果到文件
	fmt.Println("💾 Step 4: Saving results to files...")
	
	// 保存所有关键词（去重前）
	if err := saveKeywordsToFile("all_keywords.txt", allKeywords); err != nil {
		fmt.Printf("❌ Error saving all keywords: %v\n", err)
	} else {
		fmt.Printf("✅ Saved all keywords to: all_keywords.txt\n")
	}
	
	// 保存去重后的关键词
	if err := saveKeywordsToFile("unique_keywords.txt", uniqueKeywords); err != nil {
		fmt.Printf("❌ Error saving unique keywords: %v\n", err)
	} else {
		fmt.Printf("✅ Saved unique keywords to: unique_keywords.txt\n")
	}
	
	// 保存关键词映射分析
	if err := saveAnalysisToFile("keyword_analysis.txt", allKeywords, uniqueKeywords, keywordToURLMap, freqList); err != nil {
		fmt.Printf("❌ Error saving analysis: %v\n", err)
	} else {
		fmt.Printf("✅ Saved analysis to: keyword_analysis.txt\n")
	}
	
	// 保存失败的sitemap列表和详细错误分析
	if len(failedSitemaps) > 0 {
		if err := saveFailedSitemapsToFile("failed_sitemaps.txt", failedSitemaps, failureReasons, errorCategories); err != nil {
			fmt.Printf("❌ Error saving failed sitemaps: %v\n", err)
		} else {
			fmt.Printf("✅ Saved failed sitemaps to: failed_sitemaps.txt\n")
		}
	}
	
	// 总结
	duration := time.Since(startTime)
	fmt.Println()
	fmt.Println("🎉 Test Completed Successfully!")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("⏱️  Total processing time: %v\n", duration)
	fmt.Printf("🌐 Processed sitemaps: %d/%d (%.1f%% success rate)\n", 
		successCount, len(testSitemaps), float64(successCount)/float64(len(testSitemaps))*100)
	fmt.Printf("📝 Total keywords extracted: %d\n", len(allKeywords))
	fmt.Printf("🎯 Unique keywords after deduplication: %d\n", len(uniqueKeywords))
	fmt.Printf("💰 API requests saved: %d (%.1f%% efficiency gain)\n", 
		savedRequests, float64(savedRequests)/float64(len(allKeywords))*100)
	
	if len(keywordToURLMap) > 0 {
		fmt.Printf("🔗 Keywords with URL mappings: %d\n", len(keywordToURLMap))
	}
}

// deduplicateKeywords removes duplicate keywords
func deduplicateKeywords(keywords []string) []string {
	keywordSet := make(map[string]bool)
	var uniqueKeywords []string
	
	for _, keyword := range keywords {
		if !keywordSet[keyword] {
			keywordSet[keyword] = true
			uniqueKeywords = append(uniqueKeywords, keyword)
		}
	}
	
	return uniqueKeywords
}

// saveKeywordsToFile saves keywords to a text file
func saveKeywordsToFile(filename string, keywords []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Sort keywords for better readability
	sortedKeywords := make([]string, len(keywords))
	copy(sortedKeywords, keywords)
	sort.Strings(sortedKeywords)
	
	for _, keyword := range sortedKeywords {
		if _, err := file.WriteString(keyword + "\n"); err != nil {
			return err
		}
	}
	
	return nil
}

// KeywordFreq represents a keyword with its frequency
type KeywordFreq struct {
	keyword   string
	frequency int
}

// saveAnalysisToFile saves detailed analysis to a file
func saveAnalysisToFile(filename string, allKeywords, uniqueKeywords []string, keywordToURLMap map[string]string, freqList []KeywordFreq) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	fmt.Fprintf(file, "Sitemap Keyword Deduplication Analysis\n")
	fmt.Fprintf(file, "======================================\n\n")
	fmt.Fprintf(file, "Generated at: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	
	fmt.Fprintf(file, "Summary:\n")
	fmt.Fprintf(file, "- Total keywords (before dedup): %d\n", len(allKeywords))
	fmt.Fprintf(file, "- Unique keywords (after dedup):  %d\n", len(uniqueKeywords))
	fmt.Fprintf(file, "- Duplicates removed: %d\n", len(allKeywords)-len(uniqueKeywords))
	fmt.Fprintf(file, "- Deduplication efficiency: %.1f%%\n\n", float64(len(uniqueKeywords))/float64(len(allKeywords))*100)
	
	fmt.Fprintf(file, "Top 50 Most Common Keywords:\n")
	fmt.Fprintf(file, "----------------------------\n")
	for i, kf := range freqList[:min(50, len(freqList))] {
		fmt.Fprintf(file, "%3d. %-30s (%d occurrences)\n", i+1, kf.keyword, kf.frequency)
	}
	
	if len(keywordToURLMap) > 0 {
		fmt.Fprintf(file, "\n\nKeyword to URL Mappings (sample):\n")
		fmt.Fprintf(file, "--------------------------------\n")
		count := 0
		for keyword, url := range keywordToURLMap {
			if count >= 20 {
				fmt.Fprintf(file, "... and %d more mappings\n", len(keywordToURLMap)-20)
				break
			}
			fmt.Fprintf(file, "%-25s -> %s\n", keyword, url)
			count++
		}
	}
	
	return nil
}

// saveFailedSitemapsToFile saves failed sitemap URLs to a file with detailed error analysis
func saveFailedSitemapsToFile(filename string, failedSitemaps []string, failureReasons map[string]string, errorCategories map[string]int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	fmt.Fprintf(file, "Failed Sitemap Analysis Report\n")
	fmt.Fprintf(file, "==============================\n\n")
	fmt.Fprintf(file, "Generated at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Total Failed: %d sitemaps\n\n", len(failedSitemaps))
	
	// 错误分类统计
	fmt.Fprintf(file, "Error Categories Summary:\n")
	fmt.Fprintf(file, "------------------------\n")
	for errorType, count := range errorCategories {
		fmt.Fprintf(file, "- %s: %d\n", errorType, count)
	}
	fmt.Fprintf(file, "\n")
	
	// 详细的失败列表
	fmt.Fprintf(file, "Detailed Failure List:\n")
	fmt.Fprintf(file, "---------------------\n\n")
	
	for i, url := range failedSitemaps {
		fmt.Fprintf(file, "%d. %s\n", i+1, url)
		fmt.Fprintf(file, "   Error Type: %s\n", categorizeError(failureReasons[url]))
		fmt.Fprintf(file, "   Error Details: %s\n\n", failureReasons[url])
	}
	
	// 错误分析和建议
	fmt.Fprintf(file, "\nError Analysis and Recommendations:\n")
	fmt.Fprintf(file, "==================================\n\n")
	
	for errorType, count := range errorCategories {
		fmt.Fprintf(file, "%s (%d occurrences):\n", errorType, count)
		fmt.Fprintf(file, "%s\n\n", getErrorRecommendation(errorType))
	}
	
	return nil
}

// categorizeError categorizes error messages into types
func categorizeError(errMsg string) string {
	if errMsg == "" {
		return "Unknown Error"
	}
	
	errLower := strings.ToLower(errMsg)
	
	// 网络连接错误
	if strings.Contains(errLower, "timeout") || strings.Contains(errLower, "deadline exceeded") {
		return "Network Timeout"
	}
	if strings.Contains(errLower, "connection refused") || strings.Contains(errLower, "no such host") {
		return "Connection Failed"
	}
	if strings.Contains(errLower, "404") || strings.Contains(errLower, "not found") {
		return "404 Not Found"
	}
	if strings.Contains(errLower, "403") || strings.Contains(errLower, "forbidden") {
		return "403 Forbidden"
	}
	if strings.Contains(errLower, "500") || strings.Contains(errLower, "internal server") {
		return "500 Server Error"
	}
	if strings.Contains(errLower, "502") || strings.Contains(errLower, "bad gateway") {
		return "502 Bad Gateway"
	}
	if strings.Contains(errLower, "503") || strings.Contains(errLower, "service unavailable") {
		return "503 Service Unavailable"
	}
	
	// SSL/TLS错误
	if strings.Contains(errLower, "certificate") || strings.Contains(errLower, "tls") || strings.Contains(errLower, "ssl") {
		return "SSL/TLS Error"
	}
	
	// 解析错误
	if strings.Contains(errLower, "xml") || strings.Contains(errLower, "parse") || strings.Contains(errLower, "unmarshal") {
		return "Parse Error"
	}
	if strings.Contains(errLower, "unsupported") || strings.Contains(errLower, "format") {
		return "Unsupported Format"
	}
	
	// 重定向错误
	if strings.Contains(errLower, "redirect") || strings.Contains(errLower, "301") || strings.Contains(errLower, "302") {
		return "Too Many Redirects"
	}
	
	// 限流错误
	if strings.Contains(errLower, "429") || strings.Contains(errLower, "rate limit") || strings.Contains(errLower, "too many requests") {
		return "Rate Limited"
	}
	
	// DNS错误
	if strings.Contains(errLower, "dns") || strings.Contains(errLower, "lookup") {
		return "DNS Resolution Failed"
	}
	
	// 空响应
	if strings.Contains(errLower, "empty") || strings.Contains(errLower, "no urls") {
		return "Empty Response"
	}
	
	return "Other Error"
}

// getErrorRecommendation provides recommendations for each error type
func getErrorRecommendation(errorType string) string {
	recommendations := map[string]string{
		"Network Timeout": `- The server took too long to respond
- Possible causes: Slow server, large sitemap, network issues
- Recommendations: 
  * Increase timeout setting
  * Try accessing during off-peak hours
  * Check if the sitemap is exceptionally large`,
		
		"Connection Failed": `- Unable to establish connection to the server
- Possible causes: Server down, DNS issues, firewall blocking
- Recommendations:
  * Verify the URL is correct
  * Check if the website is accessible
  * Try using a VPN if region-blocked`,
		
		"404 Not Found": `- The sitemap URL doesn't exist
- Possible causes: Wrong URL, sitemap moved, website restructured
- Recommendations:
  * Check robots.txt for correct sitemap location
  * Visit the website's homepage to find sitemap link
  * Try common sitemap paths: /sitemap.xml, /sitemap_index.xml`,
		
		"403 Forbidden": `- Access to the sitemap is restricted
- Possible causes: IP blocking, authentication required, bot protection
- Recommendations:
  * Check if User-Agent is required
  * Verify if the sitemap requires authentication
  * Contact website owner for access`,
		
		"500 Server Error": `- The server encountered an internal error
- Possible causes: Server misconfiguration, backend issues
- Recommendations:
  * Wait and retry later
  * Contact website administrator
  * Check website status page`,
		
		"SSL/TLS Error": `- SSL certificate or HTTPS connection issue
- Possible causes: Expired certificate, self-signed cert, TLS version mismatch
- Recommendations:
  * Check certificate validity
  * Try with different TLS settings
  * Report to website owner`,
		
		"Parse Error": `- Failed to parse sitemap content
- Possible causes: Invalid XML, corrupted data, wrong format
- Recommendations:
  * Verify sitemap is valid XML
  * Check for special characters or encoding issues
  * Try downloading manually to inspect`,
		
		"Unsupported Format": `- Sitemap format not recognized
- Possible causes: Custom format, wrong content type
- Recommendations:
  * Check if it's RSS/Atom instead of sitemap
  * Verify file extension matches content
  * May need custom parser`,
		
		"Rate Limited": `- Too many requests, server limiting access
- Possible causes: Aggressive crawling, API limits
- Recommendations:
  * Reduce request frequency
  * Add delays between requests
  * Check robots.txt for crawl-delay`,
		
		"DNS Resolution Failed": `- Cannot resolve domain name
- Possible causes: Invalid domain, DNS server issues
- Recommendations:
  * Verify domain spelling
  * Check if domain is still active
  * Try different DNS servers`,
		
		"Empty Response": `- Sitemap exists but contains no URLs
- Possible causes: Dynamic sitemap generation failed, empty file
- Recommendations:
  * Check if sitemap requires parameters
  * Verify sitemap is properly generated
  * Try alternative sitemap paths`,
	}
	
	if rec, exists := recommendations[errorType]; exists {
		return rec
	}
	return "- Error not categorized\n- Check error details for more information"
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}