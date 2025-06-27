package storage

import (
	"context"
	"testing"

	"sitemap-go/pkg/utils"
)

func TestSimpleTracker_URLHashLogic(t *testing.T) {
	// Create in-memory storage for testing
	storage := NewMemoryStorage()
	tracker := NewSimpleTracker(storage)
	ctx := context.Background()

	// Test data
	sitemapURL1 := "https://example.com/sitemap.xml"
	sitemapURL2 := "https://another.com/sitemap.xml"
	keywords1 := []string{"keyword1", "keyword2"}
	keywords2 := []string{"keyword3", "keyword4"}

	// Test 1: Initially, no URLs should be processed
	processed, err := tracker.IsURLProcessed(ctx, sitemapURL1)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if processed {
		t.Error("Expected URL to not be processed initially")
	}

	// Test 2: Save processed URL
	err = tracker.SaveProcessedURL(ctx, sitemapURL1, keywords1)
	if err != nil {
		t.Fatalf("Expected no error saving URL, got: %v", err)
	}

	// Test 3: URL should now be marked as processed
	processed, err = tracker.IsURLProcessed(ctx, sitemapURL1)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !processed {
		t.Error("Expected URL to be processed after saving")
	}

	// Test 4: Same URL with different keywords should still be considered processed
	// This is the key fix - URL hash should only depend on URL, not keywords
	processed, err = tracker.IsURLProcessed(ctx, sitemapURL1)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !processed {
		t.Error("Expected URL to be processed regardless of keywords (URL-only hashing)")
	}

	// Test 5: Different URL should not be processed
	processed, err = tracker.IsURLProcessed(ctx, sitemapURL2)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if processed {
		t.Error("Expected different URL to not be processed")
	}

	// Test 6: Save second URL
	err = tracker.SaveProcessedURL(ctx, sitemapURL2, keywords2)
	if err != nil {
		t.Fatalf("Expected no error saving second URL, got: %v", err)
	}

	// Test 7: Both URLs should now be processed
	processed, err = tracker.IsURLProcessed(ctx, sitemapURL1)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !processed {
		t.Error("Expected first URL to still be processed")
	}

	processed, err = tracker.IsURLProcessed(ctx, sitemapURL2)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !processed {
		t.Error("Expected second URL to be processed")
	}
}

func TestSimpleTracker_HashConsistency(t *testing.T) {
	sitemapURL := "https://example.com/sitemap.xml"

	// Calculate hash for same URL (should be identical)
	hash1 := utils.CalculateURLHash(sitemapURL)
	hash2 := utils.CalculateURLHash(sitemapURL)

	// Hashes should be identical (URL-only hashing)
	if hash1 != hash2 {
		t.Errorf("Expected same hash for same URL, got %s and %s", hash1, hash2)
	}

	// Hash should be deterministic
	differentURL := "https://different.com/sitemap.xml"
	hash3 := utils.CalculateURLHash(differentURL)

	if hash1 == hash3 {
		t.Error("Expected different hashes for different URLs")
	}
}

func TestSimpleTracker_DuplicateSaveHandling(t *testing.T) {
	storage := NewMemoryStorage()
	tracker := NewSimpleTracker(storage)
	ctx := context.Background()

	sitemapURL := "https://example.com/sitemap.xml"
	keywords := []string{"keyword1", "keyword2"}

	// Save URL first time
	err := tracker.SaveProcessedURL(ctx, sitemapURL, keywords)
	if err != nil {
		t.Fatalf("Expected no error on first save, got: %v", err)
	}

	// Save same URL again - should not cause error
	err = tracker.SaveProcessedURL(ctx, sitemapURL, keywords)
	if err != nil {
		t.Fatalf("Expected no error on duplicate save, got: %v", err)
	}

	// Verify URL is still marked as processed
	processed, err := tracker.IsURLProcessed(ctx, sitemapURL)
	if err != nil {
		t.Fatalf("Expected no error checking processed status, got: %v", err)
	}
	if !processed {
		t.Error("Expected URL to remain processed after duplicate save")
	}
}
