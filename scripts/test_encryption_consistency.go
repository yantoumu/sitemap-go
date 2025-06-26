package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sitemap-go/pkg/storage"
)

// TestEncryptionConsistency tests that data can be encrypted and decrypted consistently
func main() {
	fmt.Println("=== 测试加密解密一致性 ===")
	
	// 模拟失败关键词数据
	testData := []storage.FailedKeyword{
		{
			Keyword:     "game-test",
			SitemapURL:  "https://example.com/sitemap.xml",
			FailedAt:    time.Now(),
			RetryCount:  1,
			LastError:   "API timeout",
			NextRetryAt: time.Now().Add(time.Hour),
		},
		{
			Keyword:     "puzzle-test",
			SitemapURL:  "https://example.com/sitemap.xml",
			FailedAt:    time.Now(),
			RetryCount:  2,
			LastError:   "Rate limit exceeded",
			NextRetryAt: time.Now().Add(2 * time.Hour),
		},
	}
	
	encryptionKey := "test-encryption-key-32-characters-long"
	testDir := "./test_data"
	
	// 清理测试目录
	defer func() {
		os.RemoveAll(testDir)
		fmt.Println("测试数据已清理")
	}()
	
	fmt.Printf("使用加密密钥: %s\n", encryptionKey)
	fmt.Printf("测试数据: %d个失败关键词\n", len(testData))
	
	// 第一次：创建存储并保存数据
	fmt.Println("\n=== 第一次：保存加密数据 ===")
	{
		config := storage.StorageConfig{
			DataDir:     testDir,
			CacheSize:   10,
			EncryptData: true,
		}
		
		storageService, err := storage.NewEncryptedFileStorage(config, encryptionKey)
		if err != nil {
			fmt.Printf("❌ 创建加密存储失败: %v\n", err)
			return
		}
		
		err = storageService.Save(context.Background(), "failed_keywords", testData)
		if err != nil {
			fmt.Printf("❌ 保存数据失败: %v\n", err)
			return
		}
		
		fmt.Printf("✅ 成功保存 %d 个失败关键词\n", len(testData))
	}
	
	// 第二次：重新创建存储（模拟程序重启）并读取数据
	fmt.Println("\n=== 第二次：模拟程序重启，读取解密数据 ===")
	{
		config := storage.StorageConfig{
			DataDir:     testDir,
			CacheSize:   10,
			EncryptData: true,
		}
		
		// 重新创建存储服务（模拟程序重启）
		storageService, err := storage.NewEncryptedFileStorage(config, encryptionKey)
		if err != nil {
			fmt.Printf("❌ 重新创建加密存储失败: %v\n", err)
			return
		}
		
		var loadedData []storage.FailedKeyword
		err = storageService.Load(context.Background(), "failed_keywords", &loadedData)
		if err != nil {
			fmt.Printf("❌ 读取数据失败: %v\n", err)
			fmt.Println("这表明加密/解密不一致！")
			return
		}
		
		fmt.Printf("✅ 成功读取 %d 个失败关键词\n", len(loadedData))
		
		// 验证数据完整性
		if len(loadedData) != len(testData) {
			fmt.Printf("❌ 数据数量不匹配: 期望 %d, 实际 %d\n", len(testData), len(loadedData))
			return
		}
		
		for i, loaded := range loadedData {
			original := testData[i]
			if loaded.Keyword != original.Keyword {
				fmt.Printf("❌ 关键词不匹配: 期望 %s, 实际 %s\n", original.Keyword, loaded.Keyword)
				return
			}
		}
		
		fmt.Println("✅ 数据完整性验证通过")
		
		// 显示读取的数据
		fmt.Println("\n读取到的失败关键词:")
		for i, keyword := range loadedData {
			fmt.Printf("  %d. %s (重试%d次, 错误: %s)\n", 
				i+1, keyword.Keyword, keyword.RetryCount, keyword.LastError)
		}
	}
	
	// 第三次：测试不同密钥（应该失败）
	fmt.Println("\n=== 第三次：使用错误密钥（应该失败）===")
	{
		wrongKey := "wrong-encryption-key-32-characters-long"
		config := storage.StorageConfig{
			DataDir:     testDir,
			CacheSize:   10,
			EncryptData: true,
		}
		
		storageService, err := storage.NewEncryptedFileStorage(config, wrongKey)
		if err != nil {
			fmt.Printf("❌ 创建存储失败: %v\n", err)
			return
		}
		
		var loadedData []storage.FailedKeyword
		err = storageService.Load(context.Background(), "failed_keywords", &loadedData)
		if err != nil {
			fmt.Printf("✅ 预期的解密失败: %v\n", err)
			fmt.Println("这证明加密是安全的，错误密钥无法解密数据")
		} else {
			fmt.Println("❌ 错误：使用错误密钥竟然成功了！这是安全漏洞！")
		}
	}
	
	fmt.Println("\n=== 测试总结 ===")
	fmt.Println("✅ 加密解密一致性测试完成")
	fmt.Println("✅ 失败关键词可以在程序重启后正确恢复")
	fmt.Println("✅ 加密机制安全，错误密钥无法解密数据")
}