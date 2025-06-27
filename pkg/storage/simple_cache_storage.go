package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SimpleCacheStorage 极简存储实现，无加密，无复杂抽象
type SimpleCacheStorage struct {
	dataDir string
}

// URLHashSet 存储URL哈希的简单集合
type URLHashSet map[string]bool

// 注意：FailedKeyword已在result_tracker.go中定义，这里直接使用

// NewSimpleCacheStorage 创建简单缓存存储
func NewSimpleCacheStorage(dataDir string) *SimpleCacheStorage {
	os.MkdirAll(dataDir, 0755)
	return &SimpleCacheStorage{dataDir: dataDir}
}

// SaveURLHashes 保存URL哈希集合（简单文本格式）
func (s *SimpleCacheStorage) SaveURLHashes(ctx context.Context, hashes []string) error {
	hashFile := filepath.Join(s.dataDir, "url_hashes.txt")
	content := strings.Join(hashes, "\n")
	return os.WriteFile(hashFile, []byte(content), 0644)
}

// LoadURLHashes 加载URL哈希集合
func (s *SimpleCacheStorage) LoadURLHashes(ctx context.Context) ([]string, error) {
	hashFile := filepath.Join(s.dataDir, "url_hashes.txt")
	data, err := os.ReadFile(hashFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // 文件不存在返回空列表
		}
		return nil, err
	}
	
	content := strings.TrimSpace(string(data))
	if content == "" {
		return []string{}, nil
	}
	
	return strings.Split(content, "\n"), nil
}

// SaveFailedKeywords 保存失败关键词（简单JSON格式）
func (s *SimpleCacheStorage) SaveFailedKeywords(ctx context.Context, keywords []FailedKeyword) error {
	failedFile := filepath.Join(s.dataDir, "failed_keywords.json")
	data, err := json.MarshalIndent(keywords, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(failedFile, data, 0644)
}

// LoadFailedKeywords 加载失败关键词
func (s *SimpleCacheStorage) LoadFailedKeywords(ctx context.Context) ([]FailedKeyword, error) {
	failedFile := filepath.Join(s.dataDir, "failed_keywords.json")
	data, err := os.ReadFile(failedFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []FailedKeyword{}, nil // 文件不存在返回空列表
		}
		return nil, err
	}
	
	var keywords []FailedKeyword
	err = json.Unmarshal(data, &keywords)
	return keywords, err
}

// IsURLProcessed 检查URL是否已处理（通过哈希）
func (s *SimpleCacheStorage) IsURLProcessed(ctx context.Context, urlHash string) (bool, error) {
	hashes, err := s.LoadURLHashes(ctx)
	if err != nil {
		return false, err
	}
	
	for _, hash := range hashes {
		if hash == urlHash {
			return true, nil
		}
	}
	return false, nil
}

// AddProcessedURL 添加已处理的URL哈希
func (s *SimpleCacheStorage) AddProcessedURL(ctx context.Context, urlHash string) error {
	hashes, err := s.LoadURLHashes(ctx)
	if err != nil {
		return err
	}
	
	// 检查是否已存在
	for _, hash := range hashes {
		if hash == urlHash {
			return nil // 已存在，无需重复添加
		}
	}
	
	// 添加新哈希
	hashes = append(hashes, urlHash)
	return s.SaveURLHashes(ctx, hashes)
}

// GetStats 获取存储统计信息
func (s *SimpleCacheStorage) GetStats(ctx context.Context) (map[string]interface{}, error) {
	hashes, _ := s.LoadURLHashes(ctx)
	keywords, _ := s.LoadFailedKeywords(ctx)
	
	return map[string]interface{}{
		"processed_urls": len(hashes),
		"failed_keywords": len(keywords),
		"data_dir": s.dataDir,
	}, nil
}