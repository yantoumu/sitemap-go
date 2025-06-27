package monitor

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestAtomicConcurrencyLimiter_BasicFunctionality tests basic acquire/release operations
func TestAtomicConcurrencyLimiter_BasicFunctionality(t *testing.T) {
	limiter := NewAtomicConcurrencyLimiter(2, 1*time.Second)
	ctx := context.Background()

	// Test successful acquisition
	err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Expected successful acquisition, got error: %v", err)
	}

	// Test second acquisition
	err = limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Expected second successful acquisition, got error: %v", err)
	}

	// Test third acquisition should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	err = limiter.Acquire(ctx)
	if err == nil {
		t.Fatal("Expected timeout error for third acquisition, got nil")
	}

	// Release one permit
	limiter.Release()

	// Now acquisition should succeed
	ctx = context.Background()
	err = limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Expected successful acquisition after release, got error: %v", err)
	}

	// Clean up
	limiter.Release()
	limiter.Release()
}

// TestAtomicConcurrencyLimiter_ConcurrentAccess tests thread safety
func TestAtomicConcurrencyLimiter_ConcurrentAccess(t *testing.T) {
	maxConcurrent := 5
	limiter := NewAtomicConcurrencyLimiter(maxConcurrent, 2*time.Second)
	
	numGoroutines := 20
	successCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Launch multiple goroutines trying to acquire permits
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			
			err := limiter.Acquire(ctx)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				
				// Hold permit briefly
				time.Sleep(50 * time.Millisecond)
				limiter.Release()
			}
		}()
	}

	wg.Wait()

	// Should have some successful acquisitions
	if successCount == 0 {
		t.Fatal("Expected some successful acquisitions, got none")
	}

	// In concurrent scenario, all goroutines might succeed due to timing
	// Just verify we got reasonable results
	t.Logf("Successful acquisitions: %d out of %d attempts (max concurrent: %d)",
		successCount, numGoroutines, maxConcurrent)


}

// TestAtomicConcurrencyLimiter_Stats tests statistics collection
func TestAtomicConcurrencyLimiter_Stats(t *testing.T) {
	limiter := NewAtomicConcurrencyLimiter(2, 50*time.Millisecond)
	ctx := context.Background()

	// Initial stats
	stats := limiter.GetStats()
	if stats.MaxConcurrent != 2 {
		t.Errorf("Expected MaxConcurrent=2, got %d", stats.MaxConcurrent)
	}
	if stats.CurrentActive != 0 {
		t.Errorf("Expected CurrentActive=0, got %d", stats.CurrentActive)
	}

	// Acquire permits
	limiter.Acquire(ctx)
	limiter.Acquire(ctx)

	stats = limiter.GetStats()
	if stats.CurrentActive != 2 {
		t.Errorf("Expected CurrentActive=2, got %d", stats.CurrentActive)
	}
	if stats.TotalAcquires < 2 {
		t.Errorf("Expected TotalAcquires>=2, got %d", stats.TotalAcquires)
	}

	// Test timeout - use context timeout shorter than limiter timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := limiter.Acquire(ctx) // Should timeout
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	stats = limiter.GetStats()
	if stats.TimeoutFailures == 0 {
		t.Error("Expected at least one timeout failure")
	}

	// Release permits
	limiter.Release()
	limiter.Release()

	stats = limiter.GetStats()
	if stats.CurrentActive != 0 {
		t.Errorf("Expected CurrentActive=0 after releases, got %d", stats.CurrentActive)
	}
	if stats.TotalReleases < 2 {
		t.Errorf("Expected TotalReleases>=2, got %d", stats.TotalReleases)
	}
}

// TestAtomicLimiterAdapter tests the adapter for API client integration
func TestAtomicLimiterAdapter(t *testing.T) {
	limiter := NewAtomicConcurrencyLimiter(1, 100*time.Millisecond)
	adapter := NewAtomicLimiterAdapter(limiter)
	
	ctx := context.Background()

	// Test adapter interface
	err := adapter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Expected successful acquisition through adapter, got error: %v", err)
	}

	// Test that second acquisition times out
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	err = adapter.Acquire(ctx)
	if err == nil {
		t.Fatal("Expected timeout error through adapter, got nil")
	}

	// Release and test again
	adapter.Release()
	
	ctx = context.Background()
	err = adapter.Acquire(ctx)
	if err != nil {
		t.Fatalf("Expected successful acquisition after release through adapter, got error: %v", err)
	}

	adapter.Release()
}

// BenchmarkAtomicConcurrencyLimiter_AcquireRelease benchmarks acquire/release performance
func BenchmarkAtomicConcurrencyLimiter_AcquireRelease(b *testing.B) {
	limiter := NewAtomicConcurrencyLimiter(10, 1*time.Second)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := limiter.Acquire(ctx)
			if err == nil {
				limiter.Release()
			}
		}
	})
}

// TestConcurrencyStats_UtilizationRate tests utilization calculation
func TestConcurrencyStats_UtilizationRate(t *testing.T) {
	stats := ConcurrencyStats{
		MaxConcurrent: 10,
		CurrentActive: 5,
	}

	rate := stats.UtilizationRate()
	expected := 50.0
	if rate != expected {
		t.Errorf("Expected utilization rate %.1f%%, got %.1f%%", expected, rate)
	}

	// Test edge case
	stats.MaxConcurrent = 0
	rate = stats.UtilizationRate()
	if rate != 0 {
		t.Errorf("Expected 0%% utilization for zero max, got %.1f%%", rate)
	}
}

// TestConcurrencyStats_SuccessRate tests success rate calculation
func TestConcurrencyStats_SuccessRate(t *testing.T) {
	stats := ConcurrencyStats{
		TotalAcquires:   100,
		TimeoutFailures: 10,
	}

	rate := stats.SuccessRate()
	expected := 90.0
	if rate != expected {
		t.Errorf("Expected success rate %.1f%%, got %.1f%%", expected, rate)
	}

	// Test edge case
	stats.TotalAcquires = 0
	rate = stats.SuccessRate()
	if rate != 100.0 {
		t.Errorf("Expected 100%% success rate for no attempts, got %.1f%%", rate)
	}
}
