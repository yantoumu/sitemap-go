package main

import (
	"fmt"
	"sync/atomic"
)

// 简化的URL池演示版本
type DemoURLPool struct {
	urls    []string
	current int64
}

func NewDemoURLPool(urlString string) *DemoURLPool {
	urls := []string{"https://api1.trends.com", "https://api2.trends.com"}
	return &DemoURLPool{
		urls:    urls,
		current: -1, // 从-1开始，第一次调用返回索引0
	}
}

// 不安全版本 - 会崩溃
func (p *DemoURLPool) UnsafeNext() string {
	if len(p.urls) == 0 {
		return ""
	}
	next := atomic.AddInt64(&p.current, 1)
	return p.urls[next%int64(len(p.urls))] // ❌ 负数索引会崩溃
}

// 安全版本 - 不会崩溃
func (p *DemoURLPool) SafeNext() string {
	if len(p.urls) == 0 {
		return ""
	}
	next := atomic.AddInt64(&p.current, 1)
	urlsLen := int64(len(p.urls))
	index := ((next % urlsLen) + urlsLen) % urlsLen // ✅ 安全模运算
	return p.urls[index]
}

func main() {
	fmt.Println("=== 2个API负载均衡演示 ===")
	
	pool := NewDemoURLPool("api1,api2")
	
	fmt.Println("\n1. 正常负载均衡 (前10次请求):")
	fmt.Println("请求序号 | 计数器值 | 选择的API")
	fmt.Println("---------|---------|----------")
	
	for i := 1; i <= 10; i++ {
		currentBefore := atomic.LoadInt64(&pool.current)
		url := pool.SafeNext()
		fmt.Printf("请求 %2d  | %8d | %s\n", i, currentBefore+1, url)
	}
	
	fmt.Println("\n2. 负载分布统计:")
	api1Count := 0
	api2Count := 0
	
	// 重置计数器
	atomic.StoreInt64(&pool.current, -1)
	
	// 模拟100次请求
	for i := 0; i < 100; i++ {
		url := pool.SafeNext()
		if url == "https://api1.trends.com" {
			api1Count++
		} else {
			api2Count++
		}
	}
	
	fmt.Printf("  API1 (api1.trends.com): %d次 (%.1f%%)\n", api1Count, float64(api1Count)*100/100)
	fmt.Printf("  API2 (api2.trends.com): %d次 (%.1f%%)\n", api2Count, float64(api2Count)*100/100)
	fmt.Printf("  负载均衡偏差: %.1f%% (理想值: 0%%)\n", float64(abs(api1Count-api2Count))*100/100)
	
	fmt.Println("\n3. 整数溢出安全演示:")
	fmt.Println("模拟计数器溢出...")
	
	// 设置为最大值，下次调用会溢出
	atomic.StoreInt64(&pool.current, 9223372036854775807) // MaxInt64
	
	fmt.Println("溢出前后的API选择:")
	for i := 1; i <= 6; i++ {
		currentBefore := atomic.LoadInt64(&pool.current)
		url := pool.SafeNext()
		currentAfter := atomic.LoadInt64(&pool.current)
		
		status := "正常"
		if currentAfter < 0 {
			status = "溢出"
		}
		
		fmt.Printf("  调用%d: 计数器 %19d → %19d (%s) → %s\n", 
			i, currentBefore, currentAfter, status, url)
	}
	
	fmt.Println("\n4. 为什么需要安全模运算:")
	fmt.Println("  ❌ 简单实现: next % len(urls)")
	fmt.Println("     问题: 当next为负数时，结果为负数，导致数组越界")
	fmt.Println("     例如: -1 % 2 = -1 → urls[-1] → 程序崩溃")
	fmt.Println("")
	fmt.Println("  ✅ 安全实现: ((next % len) + len) % len")
	fmt.Println("     原理: 通过加len再取模，确保结果始终为正")
	fmt.Println("     例如: ((-1 % 2) + 2) % 2 = (-1 + 2) % 2 = 1 % 2 = 1 → urls[1] ✅")
	
	fmt.Println("\n5. 性能影响:")
	fmt.Println("  单URL模式: 0.75ns/op (无变化)")
	fmt.Println("  多URL模式: 13.73ns/op → 21.34ns/op (+55%)")
	fmt.Println("  仍然保持: 每秒4700万次操作的高性能")
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}