package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCrawl_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>Test Page</h1></body></html>`)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	var results []LinkResult
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	crawl(client, server.URL, "", server.URL, 0, visited, &results, &resultsMu, &wg)
	wg.Wait()

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", results[0].Status)
	}

	if results[0].IsBroken {
		t.Error("Expected link not to be broken")
	}
}

func TestCrawl_WithInternalLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
		</body></html>`)
	})
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>Page 1</h1></body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>Page 2</h1></body></html>`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	var results []LinkResult
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	crawl(client, server.URL, "", server.URL, 0, visited, &results, &resultsMu, &wg)
	wg.Wait()

	// Should crawl: root, page1, page2 = 3 pages
	if len(results) < 3 {
		t.Errorf("Expected at least 3 results, got %d", len(results))
	}
}

func TestCrawl_MaxDepth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Always link to /next to create infinite depth
		fmt.Fprint(w, `<html><body><a href="/next">Next</a></body></html>`)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	var results []LinkResult
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	crawl(client, server.URL, "", server.URL, 0, visited, &results, &resultsMu, &wg)
	wg.Wait()

	// Should respect maxDepth and not crawl infinitely
	// At depth 0, 1, 2 we crawl. At depth 3+ we stop.
	if len(results) > 10 {
		t.Errorf("Crawl depth not respected, got %d results (expected limited by maxDepth)", len(results))
	}
}

func TestCrawl_ExternalLinks(t *testing.T) {
	externalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>External</h1></body></html>`)
	}))
	defer externalServer.Close()

	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<html><body>
			<a href="%s">External Link</a>
		</body></html>`, externalServer.URL)
	}))
	defer mainServer.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	var results []LinkResult
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	crawl(client, mainServer.URL, "", mainServer.URL, 0, visited, &results, &resultsMu, &wg)
	wg.Wait()

	// Should check both the main page and the external link
	if len(results) < 2 {
		t.Errorf("Expected at least 2 results (main + external), got %d", len(results))
	}

	// Verify external link was checked but not crawled deeply
	foundExternal := false
	for _, result := range results {
		if result.URL == externalServer.URL {
			foundExternal = true
			if result.Status != http.StatusOK {
				t.Errorf("External link status = %d, want 200", result.Status)
			}
		}
	}

	if !foundExternal {
		t.Error("External link was not checked")
	}
}

func TestCrawl_BrokenLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body>
			<a href="/broken">Broken Link</a>
		</body></html>`)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	var results []LinkResult
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	crawl(client, server.URL, "", server.URL, 0, visited, &results, &resultsMu, &wg)
	wg.Wait()

	// Find the broken link result
	var brokenResult *LinkResult
	for i := range results {
		if results[i].URL == server.URL+"/broken" {
			brokenResult = &results[i]
			break
		}
	}

	if brokenResult == nil {
		t.Fatal("Broken link not found in results")
	}

	if !brokenResult.IsBroken {
		t.Error("Expected broken link to be marked as broken")
	}

	if brokenResult.Status != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", brokenResult.Status)
	}
}

func TestCrawl_DuplicateVisitPrevention(t *testing.T) {
	visitedURLs := make(map[string]int)
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		visitedURLs[r.URL.Path]++
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		// Create a circular reference
		fmt.Fprint(w, `<html><body><a href="/page1">Page</a></body></html>`)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	visited := &SafeUrlMap{visited: make(map[string]bool)}
	var results []LinkResult
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	crawl(client, server.URL, "", server.URL, 0, visited, &results, &resultsMu, &wg)
	wg.Wait()

	mu.Lock()
	totalVisits := 0
	for _, count := range visitedURLs {
		totalVisits += count
	}
	mu.Unlock()

	// Should visit each unique path only once
	// With root and /page1, that's 2 unique URLs
	if totalVisits > 2 {
		t.Errorf("Expected at most 2 unique URL visits, got %d total visits: %v", totalVisits, visitedURLs)
	}

	// Verify no URL was visited more than once
	mu.Lock()
	for path, count := range visitedURLs {
		if count > 1 {
			t.Errorf("Path %s was visited %d times, expected 1", path, count)
		}
	}
	mu.Unlock()
}

func BenchmarkCrawl(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
		</body></html>`)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	b.ResetTimer()
	for b.Loop() {
		visited := &SafeUrlMap{visited: make(map[string]bool)}
		var results []LinkResult
		var resultsMu sync.Mutex
		var wg sync.WaitGroup

		wg.Add(1)
		crawl(client, server.URL, "", server.URL, 0, visited, &results, &resultsMu, &wg)
		wg.Wait()
	}
}
