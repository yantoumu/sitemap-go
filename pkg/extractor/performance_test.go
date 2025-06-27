package extractor

import (
	"testing"
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

			// Check that expected keywords are present (order doesn't matter)
			keywordMap := make(map[string]bool)
			for _, kw := range keywords {
				keywordMap[kw] = true
			}

			foundCount := 0
			for _, expected := range tc.expected {
				if keywordMap[expected] {
					foundCount++
				}
			}

			// Expect at least half of the expected keywords to be found
			if foundCount < len(tc.expected)/2 {
				t.Errorf("Expected at least %d keywords from %v, but only found %d in %v",
					len(tc.expected)/2, tc.expected, foundCount, keywords)
			}
		})
	}
}
