package api

import (
	"sync"
	"testing"
)

func TestEfficientURLPool_Basic(t *testing.T) {
	pool := NewEfficientURLPool("https://api1.com,https://api2.com")
	
	if pool.Size() != 2 {
		t.Errorf("Expected size 2, got %d", pool.Size())
	}
	
	// Test round-robin
	expected := []string{"https://api1.com", "https://api2.com", "https://api1.com", "https://api2.com"}
	for i, expectedURL := range expected {
		url := pool.Next()
		if url != expectedURL {
			t.Errorf("Call %d: expected %s, got %s", i+1, expectedURL, url)
		}
	}
}

func TestEfficientURLPool_SingleURL(t *testing.T) {
	pool := NewEfficientURLPool("https://api.com")
	
	// Single URL should always return the same
	for i := 0; i < 10; i++ {
		url := pool.Next()
		if url != "https://api.com" {
			t.Errorf("Expected https://api.com, got %s", url)
		}
	}
}

func TestEfficientURLPool_Empty(t *testing.T) {
	pool := NewEfficientURLPool("")
	
	if !pool.IsEmpty() {
		t.Error("Expected empty pool")
	}
	
	if pool.Next() != "" {
		t.Error("Expected empty string from empty pool")
	}
}

func TestEfficientURLPool_ConcurrentSafety(t *testing.T) {
	pool := NewEfficientURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	const numGoroutines = 100
	const numCalls = 1000
	
	results := make([][]string, numGoroutines)
	var wg sync.WaitGroup
	
	// Launch concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index] = make([]string, numCalls)
			for j := 0; j < numCalls; j++ {
				results[index][j] = pool.Next()
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all results are valid
	validURLs := map[string]bool{
		"https://api1.com": true,
		"https://api2.com": true,
		"https://api3.com": true,
	}
	
	totalCalls := 0
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numCalls; j++ {
			url := results[i][j]
			if !validURLs[url] {
				t.Errorf("Invalid URL: %s", url)
			}
			totalCalls++
		}
	}
	
	t.Logf("Processed %d concurrent calls successfully", totalCalls)
}

func TestEfficientURLPool_LoadBalancing(t *testing.T) {
	pool := NewEfficientURLPool("https://api1.com,https://api2.com")
	
	counts := make(map[string]int)
	const numCalls = 10000
	
	for i := 0; i < numCalls; i++ {
		url := pool.Next()
		counts[url]++
	}
	
	// Check distribution (should be roughly equal)
	api1Count := counts["https://api1.com"]
	api2Count := counts["https://api2.com"]
	
	if api1Count == 0 || api2Count == 0 {
		t.Error("One API got zero calls")
	}
	
	// Allow 1% deviation
	diff := abs(api1Count - api2Count)
	if float64(diff)/float64(numCalls) > 0.01 {
		t.Errorf("Load balancing deviation too high: %d vs %d", api1Count, api2Count)
	}
	
	t.Logf("Load balancing: API1=%d, API2=%d (deviation: %.2f%%)", 
		api1Count, api2Count, float64(diff)*100/float64(numCalls))
}

func BenchmarkEfficientURLPool_SingleURL(b *testing.B) {
	pool := NewEfficientURLPool("https://api.com")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Next()
	}
}

func BenchmarkEfficientURLPool_MultipleURLs(b *testing.B) {
	pool := NewEfficientURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Next()
	}
}

func BenchmarkEfficientURLPool_ConcurrentAccess(b *testing.B) {
	pool := NewEfficientURLPool("https://api1.com,https://api2.com")
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.Next()
		}
	})
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}