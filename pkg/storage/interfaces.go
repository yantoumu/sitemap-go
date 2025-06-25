package storage

import "context"

type Site struct {
	Domain      string   `json:"domain"`
	SitemapURLs []string `json:"sitemap_urls"`
	LastChecked string   `json:"last_checked"`
	URLCount    int      `json:"url_count"`
}

type StorageConfig struct {
	DataDir     string `json:"data_dir"`
	CacheSize   int    `json:"cache_size"`
	EncryptData bool   `json:"encrypt_data"`
}

type Storage interface {
	Save(ctx context.Context, key string, data interface{}) error
	Load(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

type Cache interface {
	Set(key string, value interface{}) error
	Get(key string) (interface{}, bool)
	Delete(key string) error
	Clear() error
}