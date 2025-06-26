package main

import (
	"context"
	"fmt"
	"time"
	
	"sitemap-go/pkg/api"
)

func main() {
	fmt.Println("=== Google Trends API控制机制演示 ===")
	fmt.Println("演示严格的1秒间隔控制")
	fmt.Println()
	
	// 创建Sequential Executor，1秒间隔
	executor := api.NewSequentialExecutor(1 * time.Second)
	
	// 模拟批次处理
	batches := [][]string{
		{"gaming", "mobile", "puzzle"},
		{"action", "strategy", "rpg"},
		{"casual", "arcade", "adventure"},
		{"sports", "racing", "simulation"},
	}
	
	fmt.Printf("准备处理 %d 个批次，每批次间隔1秒...\n", len(batches))
	fmt.Println()
	
	startTime := time.Now()
	
	for i, batch := range batches {
		batchStart := time.Now()
		
		err := executor.Execute(context.Background(), func() error {
			// 模拟API调用
			fmt.Printf("批次 %d: 查询关键词 %v\n", i+1, batch)
			fmt.Printf("  执行时间: %s\n", time.Since(startTime).Truncate(time.Millisecond))
			
			// 模拟API响应时间
			time.Sleep(100 * time.Millisecond)
			
			fmt.Printf("  批次完成，耗时: %s\n", time.Since(batchStart).Truncate(time.Millisecond))
			return nil
		})
		
		if err != nil {
			fmt.Printf("  ❌ 错误: %v\n", err)
		} else {
			fmt.Printf("  ✅ 成功\n")
		}
		
		fmt.Println()
	}
	
	totalTime := time.Since(startTime)
	fmt.Printf("总处理时间: %s\n", totalTime.Truncate(time.Millisecond))
	fmt.Printf("预期时间: %s (3秒间隔 + 执行时间)\n", 3*time.Second+400*time.Millisecond)
	
	if totalTime >= 3*time.Second {
		fmt.Println("✅ 间隔控制正常工作！")
	} else {
		fmt.Println("❌ 间隔控制异常！")
	}
	
	fmt.Println("\n=== 演示总结 ===")
	fmt.Println("✅ 每个API请求都等待上一个完成")
	fmt.Println("✅ 完成后强制延迟1秒")
	fmt.Println("✅ 严格串行执行，避免并发")
	fmt.Println("✅ 防止Google Trends API限流")
}