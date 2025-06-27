package parser

import (
	"fmt"
	"net/url"
	"strings"
)

// CommonURLValidator provides shared URL validation logic
type CommonURLValidator struct{}

// NewCommonURLValidator creates a new URL validator
func NewCommonURLValidator() *CommonURLValidator {
	return &CommonURLValidator{}
}

// ValidateURL performs comprehensive URL validation used across parsers
func (v *CommonURLValidator) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("empty URL")
	}
	
	if len(urlStr) > 2048 {
		return fmt.Errorf("URL too long (max 2048 characters)")
	}
	
	// Must start with http:// or https://
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return fmt.Errorf("invalid URL scheme, must be http or https")
	}
	
	// Check for invalid characters
	if strings.ContainsAny(urlStr, " \t\n\r") {
		return fmt.Errorf("URL contains invalid characters")
	}
	
	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}
	
	// Must have a host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL missing host")
	}
	
	return nil
}

// IsValidURL returns true if URL is valid, false otherwise
func (v *CommonURLValidator) IsValidURL(urlStr string) bool {
	return v.ValidateURL(urlStr) == nil
}

// CommonErrorClassifier provides shared error classification logic
type CommonErrorClassifier struct{}

// NewCommonErrorClassifier creates a new error classifier
func NewCommonErrorClassifier() *CommonErrorClassifier {
	return &CommonErrorClassifier{}
}

// IsRetryableError determines if an error should trigger a retry
func (c *CommonErrorClassifier) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := strings.ToLower(err.Error())
	
	// Non-retryable errors
	nonRetryablePatterns := []string{
		"invalid url",
		"unsupported protocol", 
		"no such host",
		"connection refused",
		"network unreachable",
		"url missing scheme",
		"url missing host",
	}
	
	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errorStr, pattern) {
			return false
		}
	}
	
	// Retryable errors
	retryablePatterns := []string{
		"timeout",
		"deadline exceeded",
		"http 429", // Rate limited
		"http 502", // Bad gateway
		"http 503", // Service unavailable
		"http 504", // Gateway timeout
		"connection reset",
		"temporary failure",
	}
	
	for _, pattern := range retryablePatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}
	
	// Default to non-retryable for unknown errors
	return false
}

// ClassifyError categorizes errors for better handling
func (c *CommonErrorClassifier) ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryNone
	}
	
	errorStr := strings.ToLower(err.Error())
	
	// HTTP errors
	if strings.Contains(errorStr, "http 4") || strings.Contains(errorStr, "http 5") {
		return ErrorCategoryHTTP
	}
	
	// Network errors
	if strings.Contains(errorStr, "timeout") || 
	   strings.Contains(errorStr, "connection") ||
	   strings.Contains(errorStr, "network") {
		return ErrorCategoryNetwork
	}
	
	// Parsing errors
	if strings.Contains(errorStr, "xml") || 
	   strings.Contains(errorStr, "encoding") ||
	   strings.Contains(errorStr, "parse") {
		return ErrorCategoryParsing
	}
	
	// Validation errors
	if strings.Contains(errorStr, "invalid") || 
	   strings.Contains(errorStr, "missing") {
		return ErrorCategoryValidation
	}
	
	return ErrorCategoryUnknown
}

// ErrorCategory represents different types of errors
type ErrorCategory int

const (
	ErrorCategoryNone ErrorCategory = iota
	ErrorCategoryHTTP
	ErrorCategoryNetwork
	ErrorCategoryParsing
	ErrorCategoryValidation
	ErrorCategoryUnknown
)

// String returns string representation of error category
func (ec ErrorCategory) String() string {
	switch ec {
	case ErrorCategoryNone:
		return "none"
	case ErrorCategoryHTTP:
		return "http"
	case ErrorCategoryNetwork:
		return "network"
	case ErrorCategoryParsing:
		return "parsing"
	case ErrorCategoryValidation:
		return "validation"
	case ErrorCategoryUnknown:
		return "unknown"
	default:
		return "invalid"
	}
}

// CommonHTTPErrorHandler provides shared HTTP error handling
type CommonHTTPErrorHandler struct{}

// NewCommonHTTPErrorHandler creates a new HTTP error handler
func NewCommonHTTPErrorHandler() *CommonHTTPErrorHandler {
	return &CommonHTTPErrorHandler{}
}

// HandleHTTPError creates appropriate error messages for HTTP status codes
func (h *CommonHTTPErrorHandler) HandleHTTPError(statusCode int, url string) error {
	switch {
	case statusCode >= 400 && statusCode < 500:
		return fmt.Errorf("client error HTTP %d for URL %s", statusCode, url)
	case statusCode >= 500 && statusCode < 600:
		return fmt.Errorf("server error HTTP %d for URL %s", statusCode, url)
	case statusCode >= 300 && statusCode < 400:
		return fmt.Errorf("redirect HTTP %d for URL %s", statusCode, url)
	default:
		return fmt.Errorf("unexpected HTTP %d for URL %s", statusCode, url)
	}
}

// IsTemporaryHTTPError checks if HTTP error is temporary and retryable
func (h *CommonHTTPErrorHandler) IsTemporaryHTTPError(statusCode int) bool {
	temporaryErrors := []int{429, 502, 503, 504}
	for _, code := range temporaryErrors {
		if statusCode == code {
			return true
		}
	}
	return false
}

// CommonParserUtils provides shared utility functions
type CommonParserUtils struct {
	urlValidator    *CommonURLValidator
	errorClassifier *CommonErrorClassifier
	httpHandler     *CommonHTTPErrorHandler
}

// NewCommonParserUtils creates a new parser utilities instance
func NewCommonParserUtils() *CommonParserUtils {
	return &CommonParserUtils{
		urlValidator:    NewCommonURLValidator(),
		errorClassifier: NewCommonErrorClassifier(),
		httpHandler:     NewCommonHTTPErrorHandler(),
	}
}

// URLValidator returns the URL validator
func (u *CommonParserUtils) URLValidator() *CommonURLValidator {
	return u.urlValidator
}

// ErrorClassifier returns the error classifier
func (u *CommonParserUtils) ErrorClassifier() *CommonErrorClassifier {
	return u.errorClassifier
}

// HTTPHandler returns the HTTP error handler
func (u *CommonParserUtils) HTTPHandler() *CommonHTTPErrorHandler {
	return u.httpHandler
}
