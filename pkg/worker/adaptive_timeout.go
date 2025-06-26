package worker

import (
	"context"
	"strings"
	"time"

	"sitemap-go/pkg/logger"
)

// AdaptiveTimeout manages dynamic timeout calculation based on sitemap characteristics
type AdaptiveTimeout struct {
	baseTimeout    time.Duration
	maxTimeout     time.Duration
	sizeMultiplier float64
	log            *logger.Logger
}

// TimeoutConfig holds configuration for adaptive timeout
type TimeoutConfig struct {
	BaseTimeout    time.Duration `json:"base_timeout"`     // Base timeout for small sitemaps
	MaxTimeout     time.Duration `json:"max_timeout"`      // Maximum allowed timeout
	SizeMultiplier float64       `json:"size_multiplier"`  // Multiplier based on estimated size
}

// NewAdaptiveTimeout creates a new adaptive timeout manager
func NewAdaptiveTimeout(config TimeoutConfig) *AdaptiveTimeout {
	if config.BaseTimeout == 0 {
		config.BaseTimeout = 2 * time.Minute // More reasonable base timeout
	}
	if config.MaxTimeout == 0 {
		config.MaxTimeout = 15 * time.Minute // Allow up to 15 minutes for huge sitemaps
	}
	if config.SizeMultiplier == 0 {
		config.SizeMultiplier = 1.5
	}

	return &AdaptiveTimeout{
		baseTimeout:    config.BaseTimeout,
		maxTimeout:     config.MaxTimeout,
		sizeMultiplier: config.SizeMultiplier,
		log:            logger.GetLogger().WithField("component", "adaptive_timeout"),
	}
}

// CalculateTimeout calculates appropriate timeout based on sitemap URL characteristics
func (at *AdaptiveTimeout) CalculateTimeout(sitemapURL string) time.Duration {
	// Estimate sitemap complexity based on URL patterns
	complexity := at.estimateComplexity(sitemapURL)
	
	// Calculate timeout based on complexity
	timeout := time.Duration(float64(at.baseTimeout) * complexity)
	
	// Ensure timeout doesn't exceed maximum
	if timeout > at.maxTimeout {
		timeout = at.maxTimeout
	}
	
	// Ensure minimum timeout
	if timeout < at.baseTimeout {
		timeout = at.baseTimeout
	}
	
	at.log.WithFields(map[string]interface{}{
		"sitemap_url": sitemapURL,
		"complexity":  complexity,
		"timeout":     timeout,
	}).Debug("Calculated adaptive timeout")
	
	return timeout
}

// estimateComplexity estimates sitemap processing complexity based on URL patterns
func (at *AdaptiveTimeout) estimateComplexity(sitemapURL string) float64 {
	lowerURL := strings.ToLower(sitemapURL)
	complexity := 1.0 // Base complexity
	
	// Compressed files take longer to process
	if strings.Contains(lowerURL, ".gz") {
		complexity *= 1.8
		at.log.Debug("Increased complexity for compressed sitemap")
	}
	
	// Sitemap index files require recursive processing
	if strings.Contains(lowerURL, "index") {
		complexity *= 3.0
		at.log.Debug("Increased complexity for sitemap index")
	}
	
	// Large gaming sites typically have huge sitemaps
	largeSitePatterns := []string{
		"poki.com", "y8.com", "gamesgames.com", "miniplay.com",
		"1001games.com", "kizi.com", "friv.com", "agame.com",
	}
	
	for _, pattern := range largeSitePatterns {
		if strings.Contains(lowerURL, pattern) {
			complexity *= 2.5
			at.log.WithField("pattern", pattern).Debug("Increased complexity for large gaming site")
			break
		}
	}
	
	// Sites with numbered sitemaps suggest multiple parts
	numberedPatterns := []string{
		"sitemap-1", "sitemap-2", "sitemap-3", "sitemap_1", "sitemap_2",
		"games-1", "games-2", "part-1", "part-2",
	}
	
	for _, pattern := range numberedPatterns {
		if strings.Contains(lowerURL, pattern) {
			complexity *= 1.5
			at.log.WithField("pattern", pattern).Debug("Increased complexity for numbered sitemap")
			break
		}
	}
	
	// International sites might have encoding issues (slower processing)
	intlPatterns := []string{
		"/zh/", "/ru/", "/de/", "/fr/", "/es/", "/it/", "/pt/", "/ja/", "/ko/",
		".ru/", ".de/", ".fr/", ".cn/", ".jp/", ".kr/",
	}
	
	for _, pattern := range intlPatterns {
		if strings.Contains(lowerURL, pattern) {
			complexity *= 1.3
			at.log.WithField("pattern", pattern).Debug("Increased complexity for international site")
			break
		}
	}
	
	// RSS feeds are usually smaller but may have different processing needs
	if strings.Contains(lowerURL, "rss") || strings.Contains(lowerURL, "feed") {
		complexity *= 0.8
		at.log.Debug("Decreased complexity for RSS feed")
	}
	
	// TXT sitemaps are usually simpler
	if strings.Contains(lowerURL, ".txt") {
		complexity *= 0.6
		at.log.Debug("Decreased complexity for TXT sitemap")
	}
	
	return complexity
}

// CreateContextWithAdaptiveTimeout creates a context with calculated timeout
func (at *AdaptiveTimeout) CreateContextWithAdaptiveTimeout(parent context.Context, sitemapURL string) (context.Context, context.CancelFunc) {
	timeout := at.CalculateTimeout(sitemapURL)
	return context.WithTimeout(parent, timeout)
}

// ProgressiveTimeout implements a progressive timeout strategy
type ProgressiveTimeout struct {
	stages []TimeoutStage
	log    *logger.Logger
}

// TimeoutStage represents a stage in progressive timeout
type TimeoutStage struct {
	Duration time.Duration
	MaxItems int    // Maximum items to process in this stage
	Name     string // Stage name for logging
}

// NewProgressiveTimeout creates a new progressive timeout manager
func NewProgressiveTimeout() *ProgressiveTimeout {
	return &ProgressiveTimeout{
		stages: []TimeoutStage{
			{Duration: 30 * time.Second, MaxItems: 100, Name: "quick_scan"},
			{Duration: 2 * time.Minute, MaxItems: 1000, Name: "medium_scan"},
			{Duration: 5 * time.Minute, MaxItems: 10000, Name: "full_scan"},
			{Duration: 15 * time.Minute, MaxItems: -1, Name: "unlimited_scan"}, // -1 = no limit
		},
		log: logger.GetLogger().WithField("component", "progressive_timeout"),
	}
}

// GetTimeoutForStage returns timeout for processing a specific number of items
func (pt *ProgressiveTimeout) GetTimeoutForStage(estimatedItems int) time.Duration {
	for _, stage := range pt.stages {
		if stage.MaxItems == -1 || estimatedItems <= stage.MaxItems {
			pt.log.WithFields(map[string]interface{}{
				"estimated_items": estimatedItems,
				"stage":          stage.Name,
				"timeout":        stage.Duration,
			}).Debug("Selected timeout stage")
			return stage.Duration
		}
	}
	
	// Fallback to the last (unlimited) stage
	lastStage := pt.stages[len(pt.stages)-1]
	pt.log.WithFields(map[string]interface{}{
		"estimated_items": estimatedItems,
		"stage":          lastStage.Name,
		"timeout":        lastStage.Duration,
	}).Debug("Using fallback timeout stage")
	
	return lastStage.Duration
}

// SmartTimeoutCalculator combines multiple timeout strategies
type SmartTimeoutCalculator struct {
	adaptive    *AdaptiveTimeout
	progressive *ProgressiveTimeout
	log         *logger.Logger
}

// NewSmartTimeoutCalculator creates a comprehensive timeout calculator
func NewSmartTimeoutCalculator(config TimeoutConfig) *SmartTimeoutCalculator {
	return &SmartTimeoutCalculator{
		adaptive:    NewAdaptiveTimeout(config),
		progressive: NewProgressiveTimeout(),
		log:         logger.GetLogger().WithField("component", "smart_timeout"),
	}
}

// CalculateOptimalTimeout calculates the best timeout considering multiple factors
func (stc *SmartTimeoutCalculator) CalculateOptimalTimeout(sitemapURL string, estimatedSize int) time.Duration {
	// Get timeout from adaptive strategy
	adaptiveTimeout := stc.adaptive.CalculateTimeout(sitemapURL)
	
	// Get timeout from progressive strategy
	progressiveTimeout := stc.progressive.GetTimeoutForStage(estimatedSize)
	
	// Use the larger of the two timeouts for safety
	optimalTimeout := adaptiveTimeout
	if progressiveTimeout > adaptiveTimeout {
		optimalTimeout = progressiveTimeout
	}
	
	stc.log.WithFields(map[string]interface{}{
		"sitemap_url":        sitemapURL,
		"estimated_size":     estimatedSize,
		"adaptive_timeout":   adaptiveTimeout,
		"progressive_timeout": progressiveTimeout,
		"optimal_timeout":    optimalTimeout,
	}).Info("Calculated optimal timeout")
	
	return optimalTimeout
}

// CreateSmartContext creates a context with optimal timeout
func (stc *SmartTimeoutCalculator) CreateSmartContext(parent context.Context, sitemapURL string, estimatedSize int) (context.Context, context.CancelFunc) {
	timeout := stc.CalculateOptimalTimeout(sitemapURL, estimatedSize)
	return context.WithTimeout(parent, timeout)
}