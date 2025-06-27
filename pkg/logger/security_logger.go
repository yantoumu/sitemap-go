package logger

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"sitemap-go/pkg/utils"
)

// SecurityLogger provides methods to safely log sensitive information
type SecurityLogger struct {
	*Logger
}

// NewSecurityLogger creates a new security-aware logger
func NewSecurityLogger() *SecurityLogger {
	return &SecurityLogger{
		Logger: GetLogger(),
	}
}

// MaskURL masks sensitive parts of URLs for logging
func (sl *SecurityLogger) MaskURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	
	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, just mask the whole thing
		return sl.generateURLHash(rawURL)
	}
	
	// Keep only the domain for identification
	domain := parsedURL.Host
	if domain == "" {
		return sl.generateURLHash(rawURL)
	}
	
	// Create a safe representation: domain + hash
	hash := sl.generateURLHash(rawURL)
	return fmt.Sprintf("%s#%s", domain, hash[:8])
}

// MaskSitemapURL specifically masks sitemap URLs
func (sl *SecurityLogger) MaskSitemapURL(sitemapURL string) string {
	if sitemapURL == "" {
		return ""
	}
	
	// Extract domain and create hash
	parsedURL, err := url.Parse(sitemapURL)
	if err != nil {
		return sl.generateURLHash(sitemapURL)
	}
	
	domain := parsedURL.Host
	hash := sl.generateURLHash(sitemapURL)
	
	// Return format: domain/sitemap#hash
	return fmt.Sprintf("%s/sitemap#%s", domain, hash[:8])
}

// MaskAPIEndpoint masks API endpoints
func (sl *SecurityLogger) MaskAPIEndpoint(apiURL string) string {
	if apiURL == "" {
		return ""
	}
	
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return "api-endpoint#" + sl.generateURLHash(apiURL)[:8]
	}
	
	// Keep domain but mask path
	domain := parsedURL.Host
	if domain == "" {
		return "api-endpoint#" + sl.generateURLHash(apiURL)[:8]
	}
	
	return fmt.Sprintf("%s/api#%s", domain, sl.generateURLHash(apiURL)[:8])
}

// MaskKeywords masks sensitive keywords if needed
func (sl *SecurityLogger) MaskKeywords(keywords []string) interface{} {
	if len(keywords) == 0 {
		return "no_keywords"
	}
	
	// Just return count and sample for security
	if len(keywords) <= 3 {
		return fmt.Sprintf("keywords_count=%d", len(keywords))
	}
	
	return fmt.Sprintf("keywords_count=%d,sample=[%s,%s,...]", 
		len(keywords), keywords[0], keywords[1])
}

// MaskSensitiveData masks various types of sensitive data in a map
func (sl *SecurityLogger) MaskSensitiveData(data map[string]interface{}) map[string]interface{} {
	masked := make(map[string]interface{})
	
	for key, value := range data {
		lowerKey := strings.ToLower(key)
		
		switch {
		case strings.Contains(lowerKey, "url") && strings.Contains(lowerKey, "sitemap"):
			if str, ok := value.(string); ok {
				masked[key] = sl.MaskSitemapURL(str)
			} else {
				masked[key] = value
			}
		case strings.Contains(lowerKey, "url"):
			if str, ok := value.(string); ok {
				masked[key] = sl.MaskURL(str)
			} else {
				masked[key] = value
			}
		case strings.Contains(lowerKey, "api") && strings.Contains(lowerKey, "url"):
			if str, ok := value.(string); ok {
				masked[key] = sl.MaskAPIEndpoint(str)
			} else {
				masked[key] = value
			}
		case strings.Contains(lowerKey, "keyword"):
			if keywords, ok := value.([]string); ok {
				masked[key] = sl.MaskKeywords(keywords)
			} else {
				masked[key] = value
			}
		case strings.Contains(lowerKey, "backend") && strings.Contains(lowerKey, "url"):
			masked[key] = "backend-api#" + sl.generateHash(fmt.Sprintf("%v", value))[:8]
		default:
			masked[key] = value
		}
	}
	
	return masked
}

// Safe logging methods
func (sl *SecurityLogger) InfoWithURL(msg string, url string, extraFields map[string]interface{}) {
	fields := map[string]interface{}{
		"url": sl.MaskURL(url),
	}
	
	if extraFields != nil {
		maskedExtra := sl.MaskSensitiveData(extraFields)
		for k, v := range maskedExtra {
			fields[k] = v
		}
	}
	
	sl.Logger.WithFields(fields).Info(msg)
}

func (sl *SecurityLogger) InfoWithSitemap(msg string, sitemapURL string, extraFields map[string]interface{}) {
	fields := map[string]interface{}{
		"sitemap": sl.MaskSitemapURL(sitemapURL),
	}
	
	if extraFields != nil {
		maskedExtra := sl.MaskSensitiveData(extraFields)
		for k, v := range maskedExtra {
			fields[k] = v
		}
	}
	
	sl.Logger.WithFields(fields).Info(msg)
}

func (sl *SecurityLogger) ErrorWithURL(msg string, url string, err error, extraFields map[string]interface{}) {
	fields := map[string]interface{}{
		"url":   sl.MaskURL(url),
		"error": err.Error(),
	}
	
	if extraFields != nil {
		maskedExtra := sl.MaskSensitiveData(extraFields)
		for k, v := range maskedExtra {
			fields[k] = v
		}
	}
	
	sl.Logger.WithFields(fields).Error(msg)
}

func (sl *SecurityLogger) WarnWithURL(msg string, url string, extraFields map[string]interface{}) {
	fields := map[string]interface{}{
		"url": sl.MaskURL(url),
	}
	
	if extraFields != nil {
		maskedExtra := sl.MaskSensitiveData(extraFields)
		for k, v := range maskedExtra {
			fields[k] = v
		}
	}
	
	sl.Logger.WithFields(fields).Warn(msg)
}

func (sl *SecurityLogger) DebugWithURL(msg string, url string, extraFields map[string]interface{}) {
	fields := map[string]interface{}{
		"url": sl.MaskURL(url),
	}
	
	if extraFields != nil {
		maskedExtra := sl.MaskSensitiveData(extraFields)
		for k, v := range maskedExtra {
			fields[k] = v
		}
	}
	
	sl.Logger.WithFields(fields).Debug(msg)
}

// Helper functions
func (sl *SecurityLogger) generateURLHash(url string) string {
	return utils.CalculateURLHash(url)
}

func (sl *SecurityLogger) generateHash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8]) // 只取前8字节，保持兼容性
}

// GenerateHash exports hash function for main package  
func (sl *SecurityLogger) GenerateHash(data string) string {
	return sl.generateHash(data) // 使用安全的SHA256哈希
}

// MaskLogMessage masks sensitive information in log messages
func (sl *SecurityLogger) MaskLogMessage(message string) string {
	// Remove URLs from log messages
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	masked := urlRegex.ReplaceAllStringFunc(message, func(url string) string {
		return sl.MaskURL(url)
	})
	
	// Remove API keys (common patterns)
	apiKeyRegex := regexp.MustCompile(`(?i)(key|token|secret)[=:]\s*[a-zA-Z0-9]+`)
	masked = apiKeyRegex.ReplaceAllString(masked, "${1}=***")
	
	return masked
}

// SafeInfo logs info with automatic sensitive data masking
func (sl *SecurityLogger) SafeInfo(msg string, fields map[string]interface{}) {
	if fields != nil {
		maskedFields := sl.MaskSensitiveData(fields)
		sl.Logger.WithFields(maskedFields).Info(sl.MaskLogMessage(msg))
	} else {
		sl.Logger.Info(sl.MaskLogMessage(msg))
	}
}

// SafeError logs error with automatic sensitive data masking
func (sl *SecurityLogger) SafeError(msg string, err error, fields map[string]interface{}) {
	maskedFields := map[string]interface{}{
		"error": err.Error(),
	}
	
	if fields != nil {
		masked := sl.MaskSensitiveData(fields)
		for k, v := range masked {
			maskedFields[k] = v
		}
	}
	
	sl.Logger.WithFields(maskedFields).Error(sl.MaskLogMessage(msg))
}

// SafeWarn logs warning with automatic sensitive data masking
func (sl *SecurityLogger) SafeWarn(msg string, fields map[string]interface{}) {
	if fields != nil {
		maskedFields := sl.MaskSensitiveData(fields)
		sl.Logger.WithFields(maskedFields).Warn(sl.MaskLogMessage(msg))
	} else {
		sl.Logger.Warn(sl.MaskLogMessage(msg))
	}
}

// SafeDebug logs debug with automatic sensitive data masking
func (sl *SecurityLogger) SafeDebug(msg string, fields map[string]interface{}) {
	if fields != nil {
		maskedFields := sl.MaskSensitiveData(fields)
		sl.Logger.WithFields(maskedFields).Debug(sl.MaskLogMessage(msg))
	} else {
		sl.Logger.Debug(sl.MaskLogMessage(msg))
	}
}

// GetSecurityLogger returns a singleton security logger
var securityLoggerInstance *SecurityLogger

func GetSecurityLogger() *SecurityLogger {
	if securityLoggerInstance == nil {
		securityLoggerInstance = NewSecurityLogger()
	}
	return securityLoggerInstance
}