package monitor

import (
	"context"
	"sync"
	"time"

	"sitemap-go/pkg/logger"
)

// ConcurrencyConfig defines performance-optimized concurrency parameters
type ConcurrencyConfig struct {
	// Sitemap抓取层 (高并发，快速完成I/O)
	MainWorkers int `json:"main_workers"`
	
	// XML解析层 (CPU密集，充分利用多核)
	ParseWorkers int `json:"parse_workers"`
	
	// 关键词提取层 (轻量级，高并发)
	ExtractWorkers int `json:"extract_workers"`
	
	// API查询控制 (真正的瓶颈，需要严格控制)
	APIWorkers        int     `json:"api_workers"`         // Google Trends API worker数量
	APIRequestsPerSecond float64 `json:"api_requests_per_second"` // API请求频率控制
	
	// Sitemap抓取控制 (可以更激进)
	SitemapRequestsPerSecond float64 `json:"sitemap_requests_per_second"` // Sitemap抓取频率
	
	// Timeout settings
	DownloadTimeout time.Duration `json:"download_timeout"`
	ParseTimeout    time.Duration `json:"parse_timeout"`
	APITimeout      time.Duration `json:"api_timeout"`
	
	// Memory management
	MaxURLsPerSitemap int `json:"max_urls_per_sitemap"`
	BufferSize        int `json:"buffer_size"`
}

// DefaultConcurrencyConfig returns performance-optimized settings
func DefaultConcurrencyConfig() ConcurrencyConfig {
	return ConcurrencyConfig{
		// Sitemap处理层：高并发快速完成
		MainWorkers:       15,  // 🚀 15个sitemap抓取worker (I/O密集，可以高并发)
		ParseWorkers:      10,  // 🚀 10个XML解析worker (CPU密集，充分利用多核)  
		ExtractWorkers:    8,   // 🚀 8个关键词提取worker (轻量级，快速处理)
		
		// API查询层：严格控制，避免限流
		APIWorkers:        2,   // 🎯 仅2个API查询worker (Google Trends有严格限制)
		APIRequestsPerSecond: 1.0, // 🎯 每秒1个API请求 (防止429限流)
		
		// Sitemap抓取频率：可以更激进
		SitemapRequestsPerSecond: 30.0, // 🚀 每秒30个sitemap请求 (普通HTTP请求)
		
		// 超时配置：针对不同操作优化
		DownloadTimeout:   10 * time.Second, // Sitemap下载应该很快
		ParseTimeout:      3 * time.Second,  // XML解析应该很快
		APITimeout:        60 * time.Second, // API查询可能较慢，给足时间
		
		// 容量配置
		MaxURLsPerSitemap: 100000, // 支持大型sitemap
		BufferSize:        500,     // 大缓冲区支持高并发sitemap处理
	}
}

// AdaptiveConcurrencyManager manages dynamic concurrency adjustment
type AdaptiveConcurrencyManager struct {
	config        ConcurrencyConfig
	mu            sync.RWMutex
	errorRate     float64
	responseTime  time.Duration
	log           *logger.Logger
	
	// Metrics for adjustment
	totalRequests    int64
	failedRequests   int64
	lastAdjustment   time.Time
}

// NewAdaptiveConcurrencyManager creates a new adaptive concurrency manager
func NewAdaptiveConcurrencyManager(config ConcurrencyConfig) *AdaptiveConcurrencyManager {
	return &AdaptiveConcurrencyManager{
		config:         config,
		log:            logger.GetLogger().WithField("component", "concurrency_manager"),
		lastAdjustment: time.Now(),
	}
}

// GetCurrentConfig returns current concurrency configuration
func (acm *AdaptiveConcurrencyManager) GetCurrentConfig() ConcurrencyConfig {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	return acm.config
}

// UpdateMetrics updates performance metrics for adaptive adjustment
func (acm *AdaptiveConcurrencyManager) UpdateMetrics(responseTime time.Duration, success bool) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	
	acm.totalRequests++
	if !success {
		acm.failedRequests++
	}
	
	// Update running average of response time
	acm.responseTime = (acm.responseTime + responseTime) / 2
	acm.errorRate = float64(acm.failedRequests) / float64(acm.totalRequests)
	
	// Adjust concurrency if needed (每分钟最多调整一次)
	if time.Since(acm.lastAdjustment) > time.Minute {
		acm.adjustConcurrency()
		acm.lastAdjustment = time.Now()
	}
}

// adjustConcurrency dynamically adjusts concurrency based on performance
func (acm *AdaptiveConcurrencyManager) adjustConcurrency() {
	originalSitemapWorkers := acm.config.MainWorkers
	originalAPIWorkers := acm.config.APIWorkers
	
	// 分别调整Sitemap抓取和API查询的并发数
	
	// Sitemap抓取层调整：更激进，因为主要是I/O等待
	if acm.errorRate > 0.1 || acm.responseTime > 5*time.Second {
		// 减少sitemap并发
		acm.config.MainWorkers = max(5, acm.config.MainWorkers-2)
		acm.config.ParseWorkers = max(3, acm.config.ParseWorkers-1)
		acm.config.SitemapRequestsPerSecond = acm.config.SitemapRequestsPerSecond * 0.8
		
		acm.log.WithFields(map[string]interface{}{
			"error_rate":        acm.errorRate,
			"response_time":     acm.responseTime,
			"old_sitemap_workers": originalSitemapWorkers,
			"new_sitemap_workers": acm.config.MainWorkers,
		}).Warn("Reducing sitemap concurrency due to high error rate")
		
	} else if acm.errorRate < 0.02 && acm.responseTime < 2*time.Second {
		// 增加sitemap并发
		acm.config.MainWorkers = min(20, acm.config.MainWorkers+2) // 最多20个sitemap worker
		acm.config.ParseWorkers = min(15, acm.config.ParseWorkers+1) // 最多15个解析worker
		acm.config.SitemapRequestsPerSecond = minFloat(50.0, acm.config.SitemapRequestsPerSecond*1.2)
		
		acm.log.WithFields(map[string]interface{}{
			"error_rate":        acm.errorRate,
			"response_time":     acm.responseTime,
			"old_sitemap_workers": originalSitemapWorkers,
			"new_sitemap_workers": acm.config.MainWorkers,
		}).Info("Increasing sitemap concurrency due to good performance")
	}
	
	// API查询层调整：保守，防止被限流
	if acm.errorRate > 0.2 { // API错误率阈值更高
		// 减少API并发（更保守）
		acm.config.APIWorkers = max(1, acm.config.APIWorkers-1)
		acm.config.APIRequestsPerSecond = maxFloat(0.5, acm.config.APIRequestsPerSecond*0.7)
		
		acm.log.WithFields(map[string]interface{}{
			"error_rate":     acm.errorRate,
			"old_api_workers": originalAPIWorkers,
			"new_api_workers": acm.config.APIWorkers,
			"new_api_rate":   acm.config.APIRequestsPerSecond,
		}).Warn("Reducing API concurrency due to high error rate")
		
	} else if acm.errorRate < 0.01 && acm.responseTime < 10*time.Second {
		// 谨慎增加API并发
		acm.config.APIWorkers = min(3, acm.config.APIWorkers+1) // 最多3个API worker
		acm.config.APIRequestsPerSecond = minFloat(2.0, acm.config.APIRequestsPerSecond*1.1)
		
		acm.log.WithFields(map[string]interface{}{
			"error_rate":     acm.errorRate,
			"old_api_workers": originalAPIWorkers,
			"new_api_workers": acm.config.APIWorkers,
			"new_api_rate":   acm.config.APIRequestsPerSecond,
		}).Info("Carefully increasing API concurrency")
	}
}

// RateLimitedExecutor provides rate-limited execution
type RateLimitedExecutor struct {
	limiter *time.Ticker
	tokens  chan struct{}
}

// NewRateLimitedExecutor creates a rate-limited executor
func NewRateLimitedExecutor(requestsPerSecond float64) *RateLimitedExecutor {
	interval := time.Duration(float64(time.Second) / requestsPerSecond)
	return &RateLimitedExecutor{
		limiter: time.NewTicker(interval),
		tokens:  make(chan struct{}, 1),
	}
}

// Execute executes function with rate limiting
func (rle *RateLimitedExecutor) Execute(ctx context.Context, fn func() error) error {
	select {
	case <-rle.limiter.C:
		return fn()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops the rate limiter
func (rle *RateLimitedExecutor) Close() {
	if rle.limiter != nil {
		rle.limiter.Stop()
	}
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}