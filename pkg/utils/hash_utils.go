package utils

import (
	"crypto/md5"
	"fmt"
)

// URLHasher provides consistent URL hashing across the application
// Follows Single Responsibility Principle - only handles URL hashing
type URLHasher struct{}

// NewURLHasher creates a new URL hasher instance
func NewURLHasher() *URLHasher {
	return &URLHasher{}
}

// CalculateURLHash generates a consistent MD5 hash for any URL
// This is the single source of truth for URL hashing in the application
func (h *URLHasher) CalculateURLHash(url string) string {
	if url == "" {
		return ""
	}
	
	hash := md5.Sum([]byte(url))
	return fmt.Sprintf("%x", hash)
}

// CalculateURLHashShort generates a short version of URL hash (first 8 characters)
// Useful for logging and display purposes
func (h *URLHasher) CalculateURLHashShort(url string) string {
	fullHash := h.CalculateURLHash(url)
	if len(fullHash) >= 8 {
		return fullHash[:8]
	}
	return fullHash
}

// Global instance for convenience
var globalHasher = NewURLHasher()

// CalculateURLHash is a convenience function that uses the global hasher
func CalculateURLHash(url string) string {
	return globalHasher.CalculateURLHash(url)
}

// CalculateURLHashShort is a convenience function that uses the global hasher
func CalculateURLHashShort(url string) string {
	return globalHasher.CalculateURLHashShort(url)
}
