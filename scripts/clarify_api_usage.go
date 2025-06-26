package main

import (
	"fmt"
	"sitemap-go/pkg/api"
)

func main() {
	fmt.Println("=== API用途澄清 ===")
	
	// 1. Google Trends查询API (多个，负载均衡)
	trendsURLs := "https://ads.seokey.vip/api/keywords?keyword=,https://k2.seokey.vip/api/keywords?keyword="
	trendsPool := api.NewEfficientURLPool(trendsURLs)
	
	fmt.Println("1. Google Trends查询API (负载均衡):")
	fmt.Printf("   配置: %s\n", trendsURLs)
	fmt.Printf("   解析为 %d 个API端点\n", trendsPool.Size())
	
	fmt.Println("\n   模拟Google Trends查询请求:")
	keywords := []string{"game", "puzzle", "action"}
	for _, keyword := range keywords {
		baseURL := trendsPool.Next()
		// 实际请求会是 POST 到 /keywords/batch，这里只是演示负载均衡
		fmt.Printf("   查询 '%s': %s\n", keyword, baseURL)
	}
	
	// 2. 后端提交API (单个，无负载均衡)
	backendURL := "https://work.seokey.vip/api/v1/keyword-metrics/batch"
	
	fmt.Println("\n2. 后端提交API (单个端点):")
	fmt.Printf("   配置: %s\n", backendURL)
	fmt.Println("   用途: 提交所有处理后的关键词数据")
	fmt.Println("   频率: 批量提交，不需要负载均衡")
	
	fmt.Println("\n=== 完整工作流程 ===")
	fmt.Println("1. 解析网站地图 → 提取关键词")
	fmt.Println("2. 查询Google Trends ← 使用负载均衡的多个API")
	fmt.Println("   - ads.seokey.vip/api/keywords")
	fmt.Println("   - k2.seokey.vip/api/keywords")
	fmt.Println("3. 提交结果到后端 ← 单个API")
	fmt.Println("   - work.seokey.vip/api/v1/keyword-metrics/batch")
}