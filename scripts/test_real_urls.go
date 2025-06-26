package main

import (
	"fmt"
	"sitemap-go/pkg/api"
)

func main() {
	fmt.Println("=== 实际URL负载均衡测试 ===")
	
	// 使用真实的API地址
	realURLs := "https://ads.seokey.vip/api/keywords?keyword=,https://k2.seokey.vip/api/keywords?keyword="
	pool := api.NewEfficientURLPool(realURLs)
	
	fmt.Printf("解析到的URL数量: %d\n", pool.Size())
	fmt.Printf("URL列表:\n")
	for i, url := range pool.URLs() {
		fmt.Printf("  %d: %s\n", i+1, url)
	}
	
	fmt.Println("\n前10次API调用会发送到:")
	for i := 1; i <= 10; i++ {
		baseURL := pool.Next()
		fullURL := baseURL + "/keywords/batch"
		fmt.Printf("请求 %2d: %s\n", i, fullURL)
	}
	
	fmt.Println("\n负载分布统计 (100次调用):")
	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		url := pool.Next()
		counts[url]++
	}
	
	for url, count := range counts {
		fmt.Printf("  %s: %d次 (%.1f%%)\n", url, count, float64(count))
	}
}