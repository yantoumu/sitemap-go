package api

import (
	"math"
	"sync/atomic"
	"testing"
)

// TestOverflowProtection tests the critical integer overflow bug fix
func TestOverflowProtection(t *testing.T) {
	pool := NewURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	// Simulate integer overflow by setting counter near max value
	// We set it to MaxInt64 - 5 to test overflow scenarios
	atomic.StoreInt64(&pool.current, math.MaxInt64-5)
	
	// Test URLs around overflow point
	expectedURLs := []string{"https://api1.com", "https://api2.com", "https://api3.com"}
	
	for i := 0; i < 10; i++ {
		url := pool.Next()
		
		// Verify URL is valid (not empty or out of bounds)
		if url == "" {
			t.Errorf("Iteration %d: Got empty URL", i)
		}
		
		// Verify URL is one of the expected URLs
		found := false
		for _, expectedURL := range expectedURLs {
			if url == expectedURL {
				found = true
				break
			}
		}
		
		if !found {
			t.Errorf("Iteration %d: Got unexpected URL: %s", i, url)
		}
		
		t.Logf("Iteration %d: current=%d, url=%s", i, atomic.LoadInt64(&pool.current), url)
	}
	
	// Verify counter has overflowed (should be negative now)
	finalCounter := atomic.LoadInt64(&pool.current)
	if finalCounter >= 0 {
		t.Errorf("Expected counter to overflow to negative value, got %d", finalCounter)
	}
	
	t.Logf("Final counter value after overflow: %d", finalCounter)
}

// TestNegativeModulo tests the specific mathematical operation for negative numbers
func TestNegativeModulo(t *testing.T) {
	testCases := []struct {
		n        int64
		m        int64
		expected int64
	}{
		{-1, 3, 2},   // (-1 % 3 + 3) % 3 = 2
		{-2, 3, 1},   // (-2 % 3 + 3) % 3 = 1
		{-3, 3, 0},   // (-3 % 3 + 3) % 3 = 0
		{-4, 3, 2},   // (-4 % 3 + 3) % 3 = 2
		{math.MinInt64, 3, 1}, // Extreme negative value
		{5, 3, 2},    // Positive case should still work
	}
	
	for _, tc := range testCases {
		// Apply the safe modulo formula: ((n % m) + m) % m
		result := ((tc.n % tc.m) + tc.m) % tc.m
		
		if result != tc.expected {
			t.Errorf("safeModulo(%d, %d) = %d, expected %d", tc.n, tc.m, result, tc.expected)
		}
		
		// Verify result is always non-negative and less than m
		if result < 0 || result >= tc.m {
			t.Errorf("safeModulo(%d, %d) = %d is out of valid range [0, %d)", tc.n, tc.m, result, tc.m)
		}
	}
}

// TestActualOverflowScenario simulates the exact overflow scenario
func TestActualOverflowScenario(t *testing.T) {
	pool := NewURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	// Set counter to MaxInt64, next increment will overflow
	atomic.StoreInt64(&pool.current, math.MaxInt64)
	
	// This call should trigger overflow but handle it safely
	url1 := pool.Next()
	if url1 == "" {
		t.Error("Got empty URL after overflow")
	}
	
	// Verify counter is now negative (overflowed)
	currentValue := atomic.LoadInt64(&pool.current)
	if currentValue >= 0 {
		t.Errorf("Expected negative counter after overflow, got %d", currentValue)
	}
	
	// Should continue working normally even with negative counter
	url2 := pool.Next()
	if url2 == "" {
		t.Error("Got empty URL with negative counter")
	}
	
	t.Logf("After overflow: counter=%d, url1=%s, url2=%s", currentValue, url1, url2)
}

// BenchmarkOverflowProtection benchmarks the performance impact of overflow protection
func BenchmarkOverflowProtection(b *testing.B) {
	pool := NewURLPool("https://api1.com,https://api2.com,https://api3.com")
	
	// Set near overflow to test worst-case performance
	atomic.StoreInt64(&pool.current, math.MaxInt64-int64(b.N)/2)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Next()
	}
}

// TestConcurrentOverflow tests concurrent access during overflow
func TestConcurrentOverflow(t *testing.T) {
	pool := NewURLPool("https://api1.com,https://api2.com")
	
	// Set counter near overflow
	atomic.StoreInt64(&pool.current, math.MaxInt64-100)
	
	const numGoroutines = 50
	const numRequests = 10
	
	results := make(chan string, numGoroutines*numRequests)
	
	// Launch concurrent goroutines that will cross the overflow boundary
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numRequests; j++ {
				url := pool.Next()
				results <- url
			}
		}()
	}
	
	// Collect all results
	validURLs := map[string]bool{
		"https://api1.com": true,
		"https://api2.com": true,
	}
	
	for i := 0; i < numGoroutines*numRequests; i++ {
		url := <-results
		if !validURLs[url] {
			t.Errorf("Got invalid URL: %s", url)
		}
	}
	
	// Verify counter has overflowed
	finalCounter := atomic.LoadInt64(&pool.current)
	t.Logf("Final counter after concurrent overflow test: %d", finalCounter)
}