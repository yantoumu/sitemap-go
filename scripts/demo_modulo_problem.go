package main

import (
	"fmt"
	"math"
)

func main() {
	fmt.Println("=== Go语言模运算问题演示 ===")
	
	// 1. 正常情况
	fmt.Println("\n1. 正常情况（正数）:")
	for i := 0; i < 6; i++ {
		result := int64(i) % 3
		fmt.Printf("  %d %% 3 = %d (数组索引: urls[%d] ✅)\n", i, result, result)
	}
	
	// 2. 问题情况：负数模运算
	fmt.Println("\n2. 问题情况（负数）- 会导致程序崩溃:")
	negativeNumbers := []int64{-1, -2, -3, -4, -5}
	for _, n := range negativeNumbers {
		result := n % 3
		status := "❌ 数组越界崩溃"
		if result >= 0 {
			status = "✅ 安全"
		}
		fmt.Printf("  %d %% 3 = %d (数组索引: urls[%d] %s)\n", n, result, result, status)
	}
	
	// 3. 安全模运算修复
	fmt.Println("\n3. 安全模运算修复:")
	for _, n := range negativeNumbers {
		unsafeResult := n % 3
		safeResult := ((n % 3) + 3) % 3
		fmt.Printf("  %d: 不安全=%d, 安全=%d (urls[%d] ✅)\n", n, unsafeResult, safeResult, safeResult)
	}
	
	// 4. 整数溢出演示
	fmt.Println("\n4. 整数溢出场景:")
	fmt.Printf("  MaxInt64 = %d\n", math.MaxInt64)
	fmt.Printf("  MinInt64 = %d (溢出后的值)\n", math.MinInt64)
	
	overflowValue := math.MinInt64  // 模拟溢出后的负数
	unsafeIndex := overflowValue % 3
	safeIndex := ((overflowValue % 3) + 3) % 3
	
	fmt.Printf("  溢出后模运算: %d %% 3 = %d ❌\n", overflowValue, unsafeIndex)
	fmt.Printf("  安全模运算: %d → %d ✅\n", overflowValue, safeIndex)
}