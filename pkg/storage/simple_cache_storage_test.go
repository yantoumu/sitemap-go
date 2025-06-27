package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleCacheStorage(t *testing.T) {
	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "test_simple_cache")
	defer os.RemoveAll(tempDir)
	
	storage := NewSimpleCacheStorage(tempDir)
	ctx := context.Background()
	
	// 测试URL哈希存储和检索
	testHashes := []string{"hash1", "hash2", "hash3"}
	
	// 保存哈希
	err := storage.SaveURLHashes(ctx, testHashes)
	if err != nil {
		t.Fatalf("Failed to save hashes: %v", err)
	}
	
	// 加载哈希
	loadedHashes, err := storage.LoadURLHashes(ctx)
	if err != nil {
		t.Fatalf("Failed to load hashes: %v", err)
	}
	
	// 验证数据
	if len(loadedHashes) != len(testHashes) {
		t.Errorf("Expected %d hashes, got %d", len(testHashes), len(loadedHashes))
	}
	
	// 测试URL处理检查
	processed, err := storage.IsURLProcessed(ctx, "hash1")
	if err != nil {
		t.Fatalf("Failed to check URL: %v", err)
	}
	if !processed {
		t.Error("Expected hash1 to be processed")
	}
	
	// 测试失败关键词
	failedKeywords := []FailedKeyword{
		{Keyword: "test1", SitemapURL: "sitemap1", LastError: "error1"},
		{Keyword: "test2", SitemapURL: "sitemap2", LastError: "error2"},
	}
	
	err = storage.SaveFailedKeywords(ctx, failedKeywords)
	if err != nil {
		t.Fatalf("Failed to save failed keywords: %v", err)
	}
	
	loadedKeywords, err := storage.LoadFailedKeywords(ctx)
	if err != nil {
		t.Fatalf("Failed to load failed keywords: %v", err)
	}
	
	if len(loadedKeywords) != len(failedKeywords) {
		t.Errorf("Expected %d failed keywords, got %d", len(failedKeywords), len(loadedKeywords))
	}
	
	// 测试统计信息
	stats, err := storage.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	
	if stats["processed_urls"] != 3 {
		t.Errorf("Expected 3 processed URLs, got %v", stats["processed_urls"])
	}
	if stats["failed_keywords"] != 2 {
		t.Errorf("Expected 2 failed keywords, got %v", stats["failed_keywords"])
	}
}

// 测试空文件情况
func TestSimpleCacheStorageEmpty(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "test_simple_cache_empty")
	defer os.RemoveAll(tempDir)
	
	storage := NewSimpleCacheStorage(tempDir)
	ctx := context.Background()
	
	// 测试加载不存在的文件
	hashes, err := storage.LoadURLHashes(ctx)
	if err != nil {
		t.Fatalf("Failed to load non-existent hashes: %v", err)
	}
	if len(hashes) != 0 {
		t.Errorf("Expected empty hashes, got %d", len(hashes))
	}
	
	keywords, err := storage.LoadFailedKeywords(ctx)
	if err != nil {
		t.Fatalf("Failed to load non-existent keywords: %v", err)
	}
	if len(keywords) != 0 {
		t.Errorf("Expected empty keywords, got %d", len(keywords))
	}
}