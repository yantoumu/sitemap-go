package extractor

import (
	"context"
	"runtime"
	"testing"
	"time"
	
	"sitemap-go/pkg/parser"
)

// BenchmarkKeywordExtraction benchmarks the optimized keyword extraction
func BenchmarkKeywordExtraction(b *testing.B) {
	extractor := NewURLKeywordExtractor()
	
	testURLs := []string{
		"https://example.com/games/action/super-mario-bros",
		"https://example.com/puzzle/tetris-classic-game",
		"https://example.com/racing/need-for-speed-online",
		"https://example.com/strategy/chess-master-3d",
		"https://example.com/arcade/pac-man-championship",
		"https://example.com/sports/fifa-soccer-2024",
		"https://example.com/adventure/zelda-breath-of-wild",
		"https://example.com/shooter/call-of-duty-mobile",
		"https://example.com/platform/sonic-the-hedgehog",
		"https://example.com/rpg/final-fantasy-online",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		for _, url := range testURLs {
			_, err := extractor.Extract(url)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkParallelExtraction benchmarks the parallel keyword extraction
func BenchmarkParallelExtraction(b *testing.B) {
	// Create test URLs
	testURLs := make([]parser.URL, 100)
	for i := 0; i < 100; i++ {
		testURLs[i] = parser.URL{
			Address: "https://example.com/games/action/super-mario-bros-" + string(rune('a'+i%26)),
		}
	}
	
	// Test different worker counts
	workerCounts := []int{1, 2, 4, 8, runtime.NumCPU()}
	
	for _, workers := range workerCounts {
		b.Run("Workers"+string(rune('0'+workers)), func(b *testing.B) {
			extractor := NewParallelKeywordExtractorWithWorkers(workers)
			ctx := context.Background()
			
			primarySelector := func(keywords []string) string {
				if len(keywords) > 0 {
					return keywords[0]
				}
				return ""
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, _, _ = extractor.ExtractFromURLs(ctx, testURLs, primarySelector)
			}
		})
	}
}

// BenchmarkStringPooling benchmarks the string pooling optimization
func BenchmarkStringPooling(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			slice := getKeywordSlice()
			slice = append(slice, "test", "keyword", "extraction")
			putKeywordSlice(slice)
		}
	})
	
	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			slice := make([]string, 0, 8)
			slice = append(slice, "test", "keyword", "extraction")
			_ = slice // Simulate usage
		}
	})
}

// TestKeywordExtractionAccuracy tests that optimization doesn't affect accuracy
func TestKeywordExtractionAccuracy(t *testing.T) {
	extractor := NewURLKeywordExtractor()
	
	testCases := []struct {
		url      string
		expected []string
	}{
		{
			url:      "https://example.com/games/action/super-mario-bros",
			expected: []string{"action", "super", "mario", "bros"},
		},
		{
			url:      "https://example.com/puzzle/tetris-classic",
			expected: []string{"puzzle", "tetris", "classic"},
		},
		{
			url:      "https://example.com/racing/need-for-speed",
			expected: []string{"racing", "need", "speed"},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			keywords, err := extractor.Extract(tc.url)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}
			
			// Check that expected keywords are present
			keywordMap := make(map[string]bool)
			for _, kw := range keywords {
				keywordMap[kw] = true
			}
			
			for _, expected := range tc.expected {
				if !keywordMap[expected] {
					t.Errorf("Expected keyword %q not found in %v", expected, keywords)
				}
			}
		})
	}
}

// TestConcurrencyConfig tests the concurrency configuration
func TestConcurrencyConfig(t *testing.T) {
	// Test default configuration
	extractor := NewParallelKeywordExtractor()
	expectedWorkers := runtime.NumCPU()
	if expectedWorkers > 8 {
		expectedWorkers = 8
	}
	
	if extractor.GetWorkerCount() != expectedWorkers {
		t.Errorf("Expected %d workers, got %d", expectedWorkers, extractor.GetWorkerCount())
	}
	
	// Test custom configuration
	customExtractor := NewParallelKeywordExtractorWithWorkers(4)
	if customExtractor.GetWorkerCount() != 4 {
		t.Errorf("Expected 4 workers, got %d", customExtractor.GetWorkerCount())
	}
	
	// Test worker count adjustment
	customExtractor.SetWorkerCount(6)
	if customExtractor.GetWorkerCount() != 6 {
		t.Errorf("Expected 6 workers after adjustment, got %d", customExtractor.GetWorkerCount())
	}
}

// TestMemoryUsage tests memory usage optimization
func TestMemoryUsage(t *testing.T) {
	// This test verifies that memory pools are working correctly
	// by checking that we can get and put objects without panics
	
	// Test string builder pool
	sb := getStringBuilder()
	sb.WriteString("test")
	putStringBuilder(sb)
	
	// Test keyword slice pool
	slice := getKeywordSlice()
	slice = append(slice, "test")
	putKeywordSlice(slice)
	
	// Test that pools work under concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			
			for j := 0; j < 100; j++ {
				sb := getStringBuilder()
				sb.WriteString("concurrent test")
				putStringBuilder(sb)
				
				slice := getKeywordSlice()
				slice = append(slice, "concurrent", "test")
				putKeywordSlice(slice)
			}
		}()
	}
	
	// Wait for all goroutines to complete
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Test timed out - possible deadlock in pools")
		}
	}
}
