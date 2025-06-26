package api

import (
	"sync"
	"testing"
)

func TestURLPool_SingleURL(t *testing.T) {
	pool := NewURLPool("https://api.example.com")
	
	if pool.Size() != 1 {
		t.Errorf("Expected size 1, got %d", pool.Size())
	}
	
	// Single URL should always return the same URL
	for i := 0; i < 10; i++ {
		url := pool.Next()
		if url != "https://api.example.com" {
			t.Errorf("Expected https://api.example.com, got %s", url)
		}
	}
}

func TestURLPool_MultipleURLs(t *testing.T) {
	pool := NewURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	if pool.Size() != 3 {
		t.Errorf("Expected size 3, got %d", pool.Size())
	}
	
	// Test round-robin behavior
	expected := []string{"https://api1.com", "https://api2.com", "https://api3.com"}
	for i := 0; i < 6; i++ { // Test two full cycles
		url := pool.Next()
		expectedURL := expected[i%3]
		if url != expectedURL {
			t.Errorf("At iteration %d, expected %s, got %s", i, expectedURL, url)
		}
	}
}

func TestURLPool_WithWhitespace(t *testing.T) {
	pool := NewURLPool(" https://api1.com , https://api2.com , https://api3.com ")
	
	if pool.Size() != 3 {
		t.Errorf("Expected size 3, got %d", pool.Size())
	}
	
	urls := pool.URLs()
	expected := []string{"https://api1.com", "https://api2.com", "https://api3.com"}
	
	for i, url := range urls {
		if url != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], url)
		}
	}
}

func TestURLPool_EmptyString(t *testing.T) {
	pool := NewURLPool("")
	
	if !pool.IsEmpty() {
		t.Error("Expected empty pool")
	}
	
	if pool.Next() != "" {
		t.Error("Expected empty string from empty pool")
	}
}

func TestURLPool_ThreadSafety(t *testing.T) {
	pool := NewURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	var wg sync.WaitGroup
	results := make([]string, 100)
	
	// Run 100 concurrent goroutines
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index] = pool.Next()
		}(i)
	}
	
	wg.Wait()
	
	// Verify all results are valid URLs
	validURLs := map[string]bool{
		"https://api1.com": true,
		"https://api2.com": true,
		"https://api3.com": true,
	}
	
	for i, url := range results {
		if !validURLs[url] {
			t.Errorf("Invalid URL at index %d: %s", i, url)
		}
	}
}

func BenchmarkURLPool_SingleURL(b *testing.B) {
	pool := NewURLPool("https://api.example.com")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Next()
	}
}

func BenchmarkURLPool_MultipleURLs(b *testing.B) {
	pool := NewURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Next()
	}
}