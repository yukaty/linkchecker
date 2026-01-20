package main

import (
	"sync"
	"testing"
)

func TestSafeUrlMap_Visit(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantNew  bool
		wantSeen bool
	}{
		{
			name:     "first visit returns false",
			url:      "https://example.com",
			wantNew:  false,
			wantSeen: false,
		},
		{
			name:     "second visit returns true",
			url:      "https://example.com",
			wantNew:  true,
			wantSeen: true,
		},
		{
			name:     "different URL returns false",
			url:      "https://different.com",
			wantNew:  false,
			wantSeen: false,
		},
	}

	visited := &SafeUrlMap{visited: make(map[string]bool)}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visited.Visit(tt.url)
			if got != tt.wantSeen {
				t.Errorf("Visit(%q) = %v, want %v", tt.url, got, tt.wantSeen)
			}
		})
	}
}

func TestSafeUrlMap_ConcurrentAccess(t *testing.T) {
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	const goroutines = 100
	const urlsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Spawn multiple goroutines trying to visit the same URLs
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range urlsPerGoroutine {
				url := "https://example.com/page-" + string(rune('0'+j))
				visited.Visit(url)
			}
		}(i)
	}

	wg.Wait()

	// Verify exactly urlsPerGoroutine URLs were stored
	visited.mu.Lock()
	count := len(visited.visited)
	visited.mu.Unlock()

	if count != urlsPerGoroutine {
		t.Errorf("Expected %d unique URLs, got %d", urlsPerGoroutine, count)
	}
}

func TestSafeUrlMap_RaceCondition(t *testing.T) {
	// This test is designed to catch race conditions when run with -race flag
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	const workers = 50
	url := "https://example.com"

	var wg sync.WaitGroup
	wg.Add(workers)

	// Multiple goroutines visiting the same URL simultaneously
	for range workers {
		go func() {
			defer wg.Done()
			visited.Visit(url)
		}()
	}

	wg.Wait()

	// Exactly one goroutine should have seen it as "not visited"
	// All others should have seen it as "already visited"
	visited.mu.Lock()
	if !visited.visited[url] {
		t.Error("URL should be marked as visited")
	}
	visited.mu.Unlock()
}

func BenchmarkSafeUrlMap_Visit(b *testing.B) {
	visited := &SafeUrlMap{visited: make(map[string]bool)}

	b.ResetTimer()
	i := 0
	for b.Loop() {
		url := "https://example.com/page-" + string(rune('0'+(i%10)))
		visited.Visit(url)
		i++
	}
}

func BenchmarkSafeUrlMap_VisitParallel(b *testing.B) {
	visited := &SafeUrlMap{visited: make(map[string]bool)}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			url := "https://example.com/page-" + string(rune('0'+(i%10)))
			visited.Visit(url)
			i++
		}
	})
}
