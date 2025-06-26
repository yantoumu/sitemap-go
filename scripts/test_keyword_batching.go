package main

import (
	"fmt"
	"sitemap-go/pkg/api"
)

func main() {
	fmt.Println("=== 关键词分批处理测试 ===")
	
	// 模拟URL池配置
	trendsURLs := "https://ads.seokey.vip/api/keywords?keyword=,https://k2.seokey.vip/api/keywords?keyword="
	pool := api.NewEfficientURLPool(trendsURLs)
	
	fmt.Printf("配置的API端点数量: %d\n", pool.Size())
	fmt.Printf("API端点列表:\n")
	for i, url := range pool.URLs() {
		fmt.Printf("  %d: %s\n", i+1, url)
	}
	
	// 模拟关键词列表（比8个多）
	keywords := []string{
		"game", "puzzle", "action", "adventure", "strategy",
		"racing", "sports", "fighting", "simulation", "rpg",
		"platform", "shooter", "arcade", "casual", "mmo",
	}
	
	fmt.Printf("\n总关键词数: %d\n", len(keywords))
	fmt.Printf("关键词列表: %v\n", keywords)
	
	// 演示分批处理
	const batchSize = 8
	fmt.Printf("\n=== 分批处理演示 (每批%d个) ===\n", batchSize)
	
	batchCount := 0
	for i := 0; i < len(keywords); i += batchSize {
		end := i + batchSize
		if end > len(keywords) {
			end = len(keywords)
		}
		
		batch := keywords[i:end]
		batchCount++
		
		// 获取负载均衡的API端点
		baseURL := pool.Next()
		
		// 构建完整的请求URL
		keywordParam := fmt.Sprintf("%s%s", baseURL, joinKeywords(batch))
		
		fmt.Printf("批次 %d (关键词 %d-%d): %d个关键词\n", batchCount, i+1, end, len(batch))
		fmt.Printf("  关键词: %v\n", batch)
		fmt.Printf("  请求URL: %s\n", keywordParam)
		fmt.Printf("  API端点: %s\n", baseURL)
		fmt.Println()
	}
	
	fmt.Printf("总批次数: %d\n", batchCount)
	fmt.Printf("API负载分布:\n")
	
	// 统计负载分布
	apiCounts := make(map[string]int)
	for i := 0; i < batchCount; i++ {
		url := pool.Next()
		apiCounts[url]++
	}
	
	for api, count := range apiCounts {
		percentage := float64(count) * 100 / float64(batchCount)
		fmt.Printf("  %s: %d次 (%.1f%%)\n", api, count, percentage)
	}
}

func joinKeywords(keywords []string) string {
	result := ""
	for i, keyword := range keywords {
		if i > 0 {
			result += ","
		}
		result += keyword
	}
	return result
}