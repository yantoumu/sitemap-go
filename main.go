package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"sitemap-go/pkg/logger"
	"sitemap-go/pkg/monitor"
)

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns environment variable as int or default
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBoolOrDefault returns environment variable as bool or default
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func main() {
	// Global panic recovery to prevent application crash
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🚨 CRITICAL ERROR: Application panic recovered: %v\n", r)
			fmt.Printf("The application encountered an unexpected error but has been safely recovered.\n")
			fmt.Printf("Please check the logs for more details and report this issue.\n")
			os.Exit(1)
		}
	}()
	
	// Environment variable defaults (GitHub Actions friendly)
	defaultSitemaps := getEnvOrDefault("SITEMAP_URLS", "")
	defaultWorkers := getEnvIntOrDefault("SITEMAP_WORKERS", 15)
	defaultDebug := getEnvBoolOrDefault("DEBUG", false)
	defaultBackendURL := getEnvOrDefault("BACKEND_URL", "")
	defaultBackendAPIKey := getEnvOrDefault("BACKEND_API_KEY", "")
	defaultBatchSize := getEnvIntOrDefault("BATCH_SIZE", 8)
	defaultTrendsAPIURL := getEnvOrDefault("TRENDS_API_URL", "")
	defaultEncryptionKey := getEnvOrDefault("ENCRYPTION_KEY", "")
	
	// Additional environment variables for advanced configuration
	defaultAPIWorkers := getEnvIntOrDefault("API_WORKERS", 4)
	defaultAPIRateLimit := getEnvOrDefault("API_RATE_LIMIT", "1.0")
	defaultSitemapRateLimit := getEnvOrDefault("SITEMAP_RATE_LIMIT", "30.0")
	defaultMaxURLs := getEnvIntOrDefault("MAX_URLS_PER_SITEMAP", 100000)
	
	// Command line flags (override environment variables)
	var (
		sitemapURLs   = flag.String("sitemaps", defaultSitemaps, "Comma-separated sitemap URLs (env: SITEMAP_URLS)")
		workers       = flag.Int("workers", defaultWorkers, "Number of concurrent sitemap workers (env: SITEMAP_WORKERS)")
		debug         = flag.Bool("debug", defaultDebug, "Enable debug logging (env: DEBUG)")
		help          = flag.Bool("help", false, "Show help message")
		backendURL    = flag.String("backend-url", defaultBackendURL, "Backend API URL for submitting results (env: BACKEND_URL)")
		backendAPIKey = flag.String("backend-api-key", defaultBackendAPIKey, "Backend API key (env: BACKEND_API_KEY)")
		batchSize     = flag.Int("batch-size", defaultBatchSize, "Keywords per API request batch (env: BATCH_SIZE)")
		trendsAPIURL  = flag.String("trends-api-url", defaultTrendsAPIURL, "Google Trends API URL (env: TRENDS_API_URL)")
		encryptionKey = flag.String("encryption-key", defaultEncryptionKey, "Encryption key for storing sensitive data (env: ENCRYPTION_KEY)")
		
		// Advanced configuration flags
		apiWorkers      = flag.Int("api-workers", defaultAPIWorkers, "Number of API query workers (env: API_WORKERS)")
		apiRateLimit    = flag.String("api-rate-limit", defaultAPIRateLimit, "API requests per second (env: API_RATE_LIMIT)")
		sitemapRateLimit = flag.String("sitemap-rate-limit", defaultSitemapRateLimit, "Sitemap requests per second (env: SITEMAP_RATE_LIMIT)")
		maxURLs         = flag.Int("max-urls", defaultMaxURLs, "Maximum URLs per sitemap (env: MAX_URLS_PER_SITEMAP)")
	)
	
	flag.Parse()
	
	if *help {
		printUsage()
		return
	}
	
	// Validate required parameters
	if *backendURL == "" {
		fmt.Println("ERROR: Backend URL is required for monitoring to be meaningful.")
		fmt.Println("Use -backend-url flag or BACKEND_URL environment variable.")
		fmt.Println("")
		printUsage()
		os.Exit(1)
	}
	
	if *backendAPIKey == "" {
		fmt.Println("ERROR: Backend API key is required for authentication.")
		fmt.Println("Use -backend-api-key flag or BACKEND_API_KEY environment variable.")
		fmt.Println("⚠️  SECURITY WARNING: Never hardcode API keys in source code!")
		fmt.Println("")
		printUsage()
		os.Exit(1)
	}
	
	if *trendsAPIURL == "" {
		fmt.Println("ERROR: Google Trends API URL is required.")
		fmt.Println("Use -trends-api-url flag or TRENDS_API_URL environment variable.")
		fmt.Println("")
		printUsage()
		os.Exit(1)
	}
	
	if *encryptionKey == "" {
		fmt.Println("ERROR: Encryption key is required for securing stored data.")
		fmt.Println("Use -encryption-key flag or ENCRYPTION_KEY environment variable.")
		fmt.Println("⚠️  SECURITY WARNING: Use a strong, randomly generated key (32+ characters)!")
		fmt.Println("Example: openssl rand -base64 32")
		fmt.Println("")
		printUsage()
		os.Exit(1)
	}
	
	if len(*encryptionKey) < 16 {
		fmt.Println("ERROR: Encryption key must be at least 16 characters long.")
		fmt.Println("⚠️  SECURITY WARNING: Use a strong, randomly generated key (32+ characters)!")
		fmt.Println("Example: openssl rand -base64 32")
		fmt.Println("")
		printUsage()
		os.Exit(1)
	}
	
	// Log configuration source for debugging
	log := logger.GetLogger().WithField("component", "main")
	secureLog := logger.GetSecurityLogger()
	secureLog.SafeInfo("Configuration loaded", map[string]interface{}{
		"sitemap_workers":     *workers,
		"api_workers":         *apiWorkers,
		"api_rate_limit":      *apiRateLimit,
		"sitemap_rate_limit":  *sitemapRateLimit,
		"batch_size":          *batchSize,
		"max_urls":            *maxURLs,
		"backend_url_set":     *backendURL != "",
		"config_source":       "env_vars_and_flags",
	})
	
	if *debug {
		log.Info("Debug logging enabled")
	}
	
	log.Info("Starting Sitemap Content Monitor Script")
	
	// Default sitemap URLs for game sites monitoring
	defaultSitemapList := []string{
		"https://poki.com/sitemap.xml",
		"https://www.y8.com/sitemap.xml",
		"https://www.crazygames.com/sitemap.xml",
		"https://www.friv.com/sitemap.xml",
		"https://www.silvergames.com/sitemap.xml",
		// Add more default URLs as needed
	}
	
	var urls []string
	if *sitemapURLs != "" {
		urls = strings.Split(*sitemapURLs, ",")
		for i, url := range urls {
			urls[i] = strings.TrimSpace(url)
		}
	} else {
		urls = defaultSitemapList
	}
	
	secureLog.SafeInfo("Sitemap configuration loaded", map[string]interface{}{
		"sitemap_count": len(urls),
		"workers":       *workers,
	})
	
	// Create sitemap monitor with backend configuration using builder pattern
	sitemapMonitor, createErr := monitor.NewMonitorConfigBuilder().
		WithTrendsAPI(*trendsAPIURL).
		WithBackend(*backendURL, *backendAPIKey).
		WithBatchSize(*batchSize).
		WithWorkers(*workers).
		WithEncryptionKey(*encryptionKey).
		Build()
	if createErr != nil {
		log.WithError(createErr).Fatal("Failed to create sitemap monitor")
	}
	defer func() {
		if err := sitemapMonitor.Close(); err != nil {
			log.WithError(err).Warn("Failed to close sitemap monitor cleanly")
		}
	}()
	
	// Backend submission is always enabled now
	secureLog.SafeInfo("Backend submission configured", map[string]interface{}{
		"backend_url":     secureLog.MaskAPIEndpoint(*backendURL),
		"backend_api_key": "api-key#" + secureLog.GenerateHash(*backendAPIKey)[:8],
		"batch_size":      *batchSize,
	})
	
	// Run monitoring with panic recovery and timeout control
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // Prevent infinite hang
	defer cancel()
	startTime := time.Now()
	
	log.Info("Starting sitemap monitoring...")
	
	var results []*monitor.MonitorResult
	var err error
	
	// Protected monitoring execution
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("monitoring panic recovered: %v", r)
				log.WithField("panic", r).Error("Panic during sitemap monitoring")
			}
		}()
		results, err = sitemapMonitor.ProcessSitemaps(ctx, urls, *workers)
	}()
	
	if err != nil {
		log.WithError(err).Fatal("Monitoring failed")
		// Note: log.Fatal() already calls os.Exit(1), no need for explicit os.Exit(1)
	}
	
	duration := time.Since(startTime)
	
	// Summary report
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}
	
	log.WithFields(map[string]interface{}{
		"total_sites":   len(results),
		"success_count": successCount,
		"failure_count": len(results) - successCount,
		"duration":      duration.String(),
	}).Info("Monitoring completed")
	
	fmt.Printf("\n=== Sitemap Monitoring Results ===\n")
	fmt.Printf("Total Sites: %d\n", len(results))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", len(results)-successCount)
	fmt.Printf("Duration: %s\n", duration.String())
	fmt.Printf("Success Rate: %.1f%%\n", float64(successCount)/float64(len(results))*100)
	
	// Show individual results
	fmt.Printf("\n=== Individual Results ===\n")
	for _, result := range results {
		status := "✅ SUCCESS"
		if !result.Success {
			status = "❌ FAILED"
		}
		
		// Mask URLs in console output
		maskedURL := secureLog.MaskSitemapURL(result.SitemapURL)
		fmt.Printf("%s %s - Keywords: %d\n", 
			status, maskedURL, len(result.Keywords))
		
		if !result.Success && result.Error != "" {
			fmt.Printf("   Error: %s\n", result.Error)
		}
	}
	
	fmt.Printf("\nResults have been saved to local storage for future reference.\n")
}

func printUsage() {
	fmt.Println("Sitemap-Go Content Monitoring Script")
	fmt.Println("")
	fmt.Println("USAGE:")
	fmt.Println("    ./sitemap-go -backend-url <URL> [OPTIONS]")
	fmt.Println("    ./sitemap-go  # Uses environment variables")
	fmt.Println("")
	fmt.Println("REQUIRED:")
	fmt.Println("    -backend-url string    Backend API URL (env: BACKEND_URL)")
	fmt.Println("")
	fmt.Println("BASIC OPTIONS:")
	fmt.Println("    -sitemaps string       Comma-separated sitemap URLs (env: SITEMAP_URLS)")
	fmt.Println("    -workers int           Sitemap workers (default: 15, env: SITEMAP_WORKERS)")
	fmt.Println("    -debug                 Enable debug logging (env: DEBUG)")
	fmt.Println("    -backend-api-key string Backend API key (env: BACKEND_API_KEY)")
	fmt.Println("    -batch-size int        Keywords per batch (default: 8, env: BATCH_SIZE)")
	fmt.Println("")
	fmt.Println("ADVANCED OPTIONS:")
	fmt.Println("    -api-workers int       API query workers (default: 4, env: API_WORKERS)")
	fmt.Println("    -api-rate-limit string API requests/sec (default: 1.0, env: API_RATE_LIMIT)")
	fmt.Println("    -sitemap-rate-limit string Sitemap requests/sec (default: 30.0, env: SITEMAP_RATE_LIMIT)")
	fmt.Println("    -max-urls int          Max URLs per sitemap (default: 100000, env: MAX_URLS_PER_SITEMAP)")
	fmt.Println("    -help                  Show this help message")
	fmt.Println("")
	fmt.Println("ENVIRONMENT VARIABLES (GitHub Actions friendly):")
	fmt.Println("    BACKEND_URL            Backend API URL (required)")
	fmt.Println("    BACKEND_API_KEY        Backend API key")
	fmt.Println("    SITEMAP_URLS           Comma-separated sitemap URLs")
	fmt.Println("    SITEMAP_WORKERS        Number of sitemap workers (15)")
	fmt.Println("    API_WORKERS            Number of API workers (4)")
	fmt.Println("    API_RATE_LIMIT         API requests per second (1.0)")
	fmt.Println("    SITEMAP_RATE_LIMIT     Sitemap requests per second (30.0)")
	fmt.Println("    BATCH_SIZE             Keywords per API batch (8)")
	fmt.Println("    MAX_URLS_PER_SITEMAP   Max URLs per sitemap (100000)")
	fmt.Println("    DEBUG                  Enable debug logging (false)")
	fmt.Println("")
	fmt.Println("EXAMPLES:")
	fmt.Println("    # Command line usage")
	fmt.Println("    ./sitemap-go -backend-url \"https://api.example.com\"")
	fmt.Println("    ./sitemap-go -backend-url \"https://api.example.com\" -workers 20 -api-workers 3")
	fmt.Println("")
	fmt.Println("    # Environment variables (GitHub Actions)")
	fmt.Println("    export BACKEND_URL=\"https://api.example.com\"")
	fmt.Println("    export SITEMAP_WORKERS=20")
	fmt.Println("    export API_WORKERS=1")
	fmt.Println("    ./sitemap-go")
	fmt.Println("")
	fmt.Println("    # GitHub Actions workflow")
	fmt.Println("    env:")
	fmt.Println("      BACKEND_URL: ${{ secrets.BACKEND_API_URL }}")
	fmt.Println("      BACKEND_API_KEY: ${{ secrets.BACKEND_API_KEY }}")
	fmt.Println("      SITEMAP_WORKERS: 15")
	fmt.Println("      API_WORKERS: 4")
	fmt.Println("")
	fmt.Println("PERFORMANCE OPTIMIZED:")
	fmt.Println("- Fast sitemap processing: 15 concurrent workers")
	fmt.Println("- Controlled API queries: 4 workers, 1 req/sec (avoid rate limits)")
	fmt.Println("- Global keyword deduplication")
	fmt.Println("- Non-blocking backend submission")
	fmt.Println("- Automatic retry for failed keywords")
}