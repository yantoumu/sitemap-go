package service

import (
	"context"
	"sitemap-go/pkg/api"
	"sitemap-go/pkg/parser"
	"sitemap-go/pkg/extractor"
)

type CollectorService interface {
	CollectSitemaps(ctx context.Context, domains []string) ([]parser.URL, error)
	ExtractKeywords(ctx context.Context, urls []parser.URL) ([]extractor.Keyword, error)
}

type ProcessorService interface {
	ProcessKeywords(ctx context.Context, keywords []extractor.Keyword) (*api.APIResponse, error)
	DetectUpdates(ctx context.Context, oldURLs, newURLs []parser.URL) ([]parser.URL, error)
}

type StorageService interface {
	SaveURLs(ctx context.Context, urls []parser.URL) error
	LoadURLs(ctx context.Context, domain string) ([]parser.URL, error)
	SaveKeywords(ctx context.Context, keywords []extractor.Keyword) error
	LoadKeywords(ctx context.Context, domain string) ([]extractor.Keyword, error)
}

type NotificationService interface {
	NotifyUpdates(ctx context.Context, updates []parser.URL) error
	NotifyErrors(ctx context.Context, errors []error) error
}

type MonitorService interface {
	HealthCheck(ctx context.Context) error
	GetMetrics(ctx context.Context) (map[string]interface{}, error)
	StartMonitoring(ctx context.Context) error
	StopMonitoring() error
}