package monitor

import (
	"fmt"
	"net/url"
	"strings"
)

// MonitorConfigBuilder implements Builder pattern for creating SitemapMonitor
// Follows Single Responsibility Principle - only handles configuration building
type MonitorConfigBuilder struct {
	trendsAPIURL  string
	backendURL    string
	backendAPIKey string
	batchSize     int
	workers       int
	encryptionKey string
	errors        []error
}

// NewMonitorConfigBuilder creates a new configuration builder
func NewMonitorConfigBuilder() *MonitorConfigBuilder {
	return &MonitorConfigBuilder{
		batchSize: 4,   // Default batch size: 4 keywords per request
		workers:   8,   // Default worker count
		errors:    make([]error, 0),
	}
}

// WithTrendsAPI sets the Google Trends API URL(s) with validation
// Supports both single URL and comma-separated multiple URLs for load balancing
func (b *MonitorConfigBuilder) WithTrendsAPI(apiURL string) *MonitorConfigBuilder {
	if apiURL == "" {
		b.errors = append(b.errors, fmt.Errorf("trends API URL cannot be empty"))
		return b
	}
	
	// Parse and validate each URL (supports comma-separated URLs)
	urls := strings.Split(apiURL, ",")
	for i, rawURL := range urls {
		cleanURL := strings.TrimSpace(rawURL)
		if cleanURL == "" {
			continue
		}
		
		// Validate URL format
		if _, err := url.Parse(cleanURL); err != nil {
			b.errors = append(b.errors, fmt.Errorf("invalid trends API URL #%d (%s): %w", i+1, cleanURL, err))
			return b
		}
	}
	
	b.trendsAPIURL = apiURL // Store original string for URL pool creation
	return b
}

// WithBackend sets the backend configuration with validation
func (b *MonitorConfigBuilder) WithBackend(backendURL, apiKey string) *MonitorConfigBuilder {
	if backendURL == "" {
		b.errors = append(b.errors, fmt.Errorf("backend URL cannot be empty"))
		return b
	}
	
	if apiKey == "" {
		b.errors = append(b.errors, fmt.Errorf("backend API key cannot be empty"))
		return b
	}
	
	// Validate URL format
	if _, err := url.Parse(backendURL); err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid backend URL: %w", err))
		return b
	}
	
	b.backendURL = backendURL
	b.backendAPIKey = apiKey
	return b
}

// WithBatchSize sets the batch size with validation
func (b *MonitorConfigBuilder) WithBatchSize(size int) *MonitorConfigBuilder {
	if size <= 0 {
		b.errors = append(b.errors, fmt.Errorf("batch size must be positive, got: %d", size))
		return b
	}
	
	if size > 1000 {
		b.errors = append(b.errors, fmt.Errorf("batch size too large (max 1000), got: %d", size))
		return b
	}
	
	b.batchSize = size
	return b
}

// WithWorkers sets the worker count with validation
func (b *MonitorConfigBuilder) WithWorkers(count int) *MonitorConfigBuilder {
	if count <= 0 {
		b.errors = append(b.errors, fmt.Errorf("worker count must be positive, got: %d", count))
		return b
	}
	
	if count > 50 {
		b.errors = append(b.errors, fmt.Errorf("worker count too high (max 50), got: %d", count))
		return b
	}
	
	b.workers = count
	return b
}

// WithEncryptionKey sets the encryption key for securing stored data
func (b *MonitorConfigBuilder) WithEncryptionKey(key string) *MonitorConfigBuilder {
	if key == "" {
		b.errors = append(b.errors, fmt.Errorf("encryption key cannot be empty"))
		return b
	}
	
	if len(key) < 16 {
		b.errors = append(b.errors, fmt.Errorf("encryption key must be at least 16 characters long, got: %d", len(key)))
		return b
	}
	
	// Warn about weak keys
	if key == "default-sitemap-monitor-key" || key == "test-encryption-key" {
		b.errors = append(b.errors, fmt.Errorf("using default encryption key is insecure - generate a strong random key"))
		return b
	}
	
	b.encryptionKey = key
	return b
}

// Validate checks all configuration and returns any validation errors
func (b *MonitorConfigBuilder) Validate() error {
	if len(b.errors) == 0 {
		return nil
	}
	
	// Aggregate all errors into a single error message
	var errorMessages []string
	for _, err := range b.errors {
		errorMessages = append(errorMessages, err.Error())
	}
	
	return fmt.Errorf("configuration validation failed: %s", strings.Join(errorMessages, "; "))
}

// Build creates a SitemapMonitor with validated configuration
// Returns error instead of panic - follows proper error handling
func (b *MonitorConfigBuilder) Build() (*SitemapMonitor, error) {
	// Validate configuration first
	if err := b.Validate(); err != nil {
		return nil, err
	}
	
	// All validation passed, create the monitor
	return b.createMonitor()
}

// createMonitor internal method to create the actual monitor
// Separated for better testability and Single Responsibility
func (b *MonitorConfigBuilder) createMonitor() (*SitemapMonitor, error) {
	// Create configuration
	config := MonitorConfig{
		TrendAPIBaseURL:         b.trendsAPIURL,
		WorkerPoolSize:          b.workers,
		EncryptionKey:          b.encryptionKey,
		EnableBackendSubmission: true,
	}
	
	// Check if TRENDS_API_URL contains multiple URLs (comma-separated)
	if strings.Contains(b.trendsAPIURL, ",") {
		// Split into primary and secondary for dual API mode
		urls := strings.Split(b.trendsAPIURL, ",")
		primaryURL := strings.TrimSpace(urls[0])
		secondaryURL := strings.TrimSpace(urls[1])
		
		// Use dual API monitor for load balancing
		return NewMonitorWithDualAPI(config, b.backendURL, b.backendAPIKey, b.batchSize, primaryURL, secondaryURL)
	}
	
	// Use the safe internal constructor
	monitor, err := createSitemapMonitorInternal(config, b.backendURL, b.backendAPIKey, b.batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create sitemap monitor: %w", err)
	}
	
	return monitor, nil
}

// BuildForTesting creates a monitor suitable for testing (no backend requirements)
func (b *MonitorConfigBuilder) BuildForTesting() (*SitemapMonitor, error) {
	// For testing, we don't require backend configuration
	// But we still need a valid trends API URL
	if b.trendsAPIURL == "" {
		return nil, fmt.Errorf("trends API URL is required even for testing")
	}
	
	config := MonitorConfig{
		TrendAPIBaseURL:         b.trendsAPIURL,
		WorkerPoolSize:          b.workers,
		EncryptionKey:          "test-encryption-key",
		EnableBackendSubmission: false, // Disable for testing
	}
	
	monitor, err := createSitemapMonitorInternal(config, "http://test-backend.com", "test-key", b.batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create test monitor: %w", err)
	}
	
	return monitor, nil
}

// HasErrors returns true if there are any validation errors
func (b *MonitorConfigBuilder) HasErrors() bool {
	return len(b.errors) > 0
}

// GetErrors returns all validation errors
func (b *MonitorConfigBuilder) GetErrors() []error {
	return b.errors
}