package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDataPersistenceIntegrity 测试数据持久化和恢复的完整性
func TestDataPersistenceIntegrity(t *testing.T) {
	// 创建临时测试目录
	tempDir := filepath.Join(os.TempDir(), "test_data_persistence")
	defer os.RemoveAll(tempDir)

	encryptionKey := "test-key-for-persistence-testing"
	ctx := context.Background()

	// 创建存储服务
	config := StorageConfig{
		DataDir:     tempDir,
		CacheSize:   10,
		EncryptData: true,
	}
	storage, err := NewEncryptedFileStorage(config, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// 创建SimpleTracker
	tracker := NewSimpleTracker(storage)

	// 测试数据保存
	testURLs := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
	}
	sitemapURL := "https://example.com/sitemap.xml"

	// 保存URL数据
	err = tracker.SaveProcessedURLs(ctx, testURLs, sitemapURL)
	if err != nil {
		t.Fatalf("Failed to save URLs: %v", err)
	}

	// 测试失败关键词保存
	failedKeywords := []string{"keyword1", "keyword2"}
	testError := fmt.Errorf("test error")
	err = tracker.SaveFailedKeywords(ctx, failedKeywords, sitemapURL, sitemapURL, testError)
	if err != nil {
		t.Fatalf("Failed to save failed keywords: %v", err)
	}

	// 验证文件结构
	expectedFiles := []string{
		"pr/processed_urls.enc",
		"fa/failed_keywords.enc",
	}

	for _, expectedFile := range expectedFiles {
		fullPath := filepath.Join(tempDir, expectedFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", fullPath)
		}
	}

	// 创建新的存储实例（模拟程序重启）
	storage2, err := NewEncryptedFileStorage(config, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to create second storage instance: %v", err)
	}
	tracker2 := NewSimpleTracker(storage2)

	// 测试数据恢复
	for _, url := range testURLs {
		processed, err := tracker2.IsURLProcessed(ctx, url)
		if err != nil {
			t.Errorf("Failed to check URL %s: %v", url, err)
		}
		if !processed {
			t.Errorf("URL %s should be marked as processed", url)
		}
	}

	// 测试未处理URL
	unprocessedURL := "https://example.com/new-page"
	processed, err := tracker2.IsURLProcessed(ctx, unprocessedURL)
	if err != nil {
		t.Errorf("Failed to check unprocessed URL: %v", err)
	}
	if processed {
		t.Errorf("URL %s should not be marked as processed", unprocessedURL)
	}

	// 验证失败关键词恢复
	var failedRecords []FailedKeywordRecord
	err = storage2.Load(ctx, "failed_keywords", &failedRecords)
	if err != nil {
		t.Errorf("Failed to load failed keywords: %v", err)
	}

	if len(failedRecords) < len(failedKeywords) {
		t.Errorf("Expected at least %d failed keyword records, got %d", len(failedKeywords), len(failedRecords))
	}
}

// TestEncryptionKeyConsistency 测试加密密钥一致性
func TestEncryptionKeyConsistency(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_encryption_consistency")
	defer os.RemoveAll(tempDir)

	config := StorageConfig{
		DataDir:     tempDir,
		CacheSize:   10,
		EncryptData: true,
	}

	key1 := "consistent-test-key"
	key2 := "consistent-test-key" // 相同密钥
	key3 := "different-test-key"  // 不同密钥

	ctx := context.Background()
	testData := map[string]interface{}{
		"test": "data",
		"number": 42,
	}

	// 用第一个密钥保存数据
	storage1, err := NewEncryptedFileStorage(config, key1)
	if err != nil {
		t.Fatalf("Failed to create storage1: %v", err)
	}

	err = storage1.Save(ctx, "test_key", testData)
	if err != nil {
		t.Fatalf("Failed to save data with key1: %v", err)
	}

	// 用相同密钥读取数据（应该成功）
	storage2, err := NewEncryptedFileStorage(config, key2)
	if err != nil {
		t.Fatalf("Failed to create storage2: %v", err)
	}

	var loadedData map[string]interface{}
	err = storage2.Load(ctx, "test_key", &loadedData)
	if err != nil {
		t.Errorf("Failed to load data with same key: %v", err)
	}

	// 用不同密钥读取数据（应该失败）
	storage3, err := NewEncryptedFileStorage(config, key3)
	if err != nil {
		t.Fatalf("Failed to create storage3: %v", err)
	}

	var loadedData2 map[string]interface{}
	err = storage3.Load(ctx, "test_key", &loadedData2)
	if err == nil {
		t.Error("Expected error when loading data with different key, but succeeded")
	}
}

// TestBatchURLProcessing 测试批量URL处理性能
func TestBatchURLProcessing(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_batch_processing")
	defer os.RemoveAll(tempDir)

	config := StorageConfig{
		DataDir:     tempDir,
		CacheSize:   100,
		EncryptData: true,
	}
	storage, err := NewEncryptedFileStorage(config, "test-key")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	tracker := NewSimpleTracker(storage)

	ctx := context.Background()

	// 生成大量测试URL
	urls := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		urls[i] = fmt.Sprintf("https://example.com/page%d", i)
	}

	start := time.Now()

	// 批量保存
	err = tracker.SaveProcessedURLs(ctx, urls, "https://example.com/sitemap.xml")
	if err != nil {
		t.Fatalf("Failed to save batch URLs: %v", err)
	}

	saveTime := time.Since(start)
	t.Logf("Batch save time for 1000 URLs: %v", saveTime)

	// 批量检查
	start = time.Now()
	results, err := tracker.AreURLsProcessed(ctx, urls)
	if err != nil {
		t.Fatalf("Failed to check batch URLs: %v", err)
	}

	checkTime := time.Since(start)
	t.Logf("Batch check time for 1000 URLs: %v", checkTime)

	// 验证结果
	for url, processed := range results {
		if !processed {
			t.Errorf("URL %s should be marked as processed", url)
		}
	}

	// 性能断言（根据实际需求调整）
	if saveTime > 5*time.Second {
		t.Errorf("Batch save took too long: %v", saveTime)
	}
	if checkTime > 1*time.Second {
		t.Errorf("Batch check took too long: %v", checkTime)
	}
}