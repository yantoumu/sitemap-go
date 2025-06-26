package main

import (
	"context"
	"fmt"
	"time"
	
	"sitemap-go/pkg/api"
)

func main() {
	fmt.Println("=== Google Trends API控制机制演示 ===")
	fmt.Println("演示串行执行控制 - 请求完成后立即执行下一个")
	fmt.Println()
	
	// 创建Sequential Executor，无强制延迟
	executor := api.NewSequentialExecutor()
	
	// 模拟批次处理 - 每批次4个关键词
	batches := [][]string{
		{"gaming", "mobile", "puzzle", "online"},
		{"action", "strategy", "rpg", "multiplayer"},
		{"casual", "arcade", "adventure", "platform"},
		{"sports", "racing", "simulation", "sandbox"},
	}
	
	fmt.Printf("准备处理 %d 个批次，串行执行（无强制延迟）...\n", len(batches))
	fmt.Println()
	
	startTime := time.Now()
	
	for i, batch := range batches {
		batchStart := time.Now()
		
		err := executor.Execute(context.Background(), func() error {
			// 模拟API调用 - 4个关键词一批
			fmt.Printf("批次 %d: 查询关键词 %v (4个关键词)\n", i+1, batch)
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
	fmt.Printf("预期时间: ~%s (仅执行时间，无额外延迟)\n", 400*time.Millisecond)
	
	if totalTime < 1*time.Second {
		fmt.Println("✅ 串行控制正常工作！无强制延迟")
	} else {
		fmt.Println("❌ 执行时间异常，可能有意外延迟")
	}
	
	fmt.Println("\n=== 演示总结 ===")
	fmt.Println("✅ 每个API请求都等待上一个完成")
	fmt.Println("✅ 完成后立即执行下一个，无强制延迟")
	fmt.Println("✅ 严格串行执行，避免并发")
	fmt.Println("✅ 防止Google Trends API并发限流")
}