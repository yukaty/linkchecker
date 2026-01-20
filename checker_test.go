package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckURL(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus int
		wantErr    bool
	}{
		{
			name:       "successful request",
			statusCode: http.StatusOK,
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantStatus: http.StatusNotFound,
			wantErr:    false,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantStatus: http.StatusInternalServerError,
			wantErr:    false,
		},
		{
			name:       "redirect",
			statusCode: http.StatusMovedPermanently,
			wantStatus: http.StatusMovedPermanently,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := &http.Client{Timeout: 5 * time.Second}
			got, err := checkURL(client, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("checkURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.wantStatus {
				t.Errorf("checkURL() = %v, want %v", got, tt.wantStatus)
			}
		})
	}
}

func TestCheckURL_InvalidURL(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	_, err := checkURL(client, "://invalid-url")

	if err == nil {
		t.Error("checkURL() expected error for invalid URL, got nil")
	}
}

func TestCheckURL_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Client with very short timeout
	client := &http.Client{Timeout: 10 * time.Millisecond}
	_, err := checkURL(client, server.URL)

	if err == nil {
		t.Error("checkURL() expected timeout error, got nil")
	}
}

func TestCheckURLs(t *testing.T) {
	// Create test servers with different status codes
	server200 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server200.Close()

	server404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server404.Close()

	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server500.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	urls := []string{server200.URL, server404.URL, server500.URL}

	results := checkURLs(client, urls)

	if len(results) != 3 {
		t.Fatalf("checkURLs() returned %d results, want 3", len(results))
	}

	// Verify each result
	statusMap := make(map[string]int)
	for _, result := range results {
		statusMap[result.URL] = result.Status
	}

	if statusMap[server200.URL] != http.StatusOK {
		t.Errorf("Expected status 200 for %s, got %d", server200.URL, statusMap[server200.URL])
	}

	if statusMap[server404.URL] != http.StatusNotFound {
		t.Errorf("Expected status 404 for %s, got %d", server404.URL, statusMap[server404.URL])
	}

	if statusMap[server500.URL] != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for %s, got %d", server500.URL, statusMap[server500.URL])
	}
}

func TestCheckURLs_BrokenLinks(t *testing.T) {
	server404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server404.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	urls := []string{server404.URL, "http://invalid-domain-that-does-not-exist-12345.com"}

	results := checkURLs(client, urls)

	brokenCount := 0
	for _, result := range results {
		if result.IsBroken {
			brokenCount++
		}
	}

	if brokenCount != 2 {
		t.Errorf("Expected 2 broken links, got %d", brokenCount)
	}
}

func TestCheckURLs_EmptyList(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	results := checkURLs(client, []string{})

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty URL list, got %d", len(results))
	}
}

func BenchmarkCheckURL(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	b.ResetTimer()
	for b.Loop() {
		checkURL(client, server.URL)
	}
}

func BenchmarkCheckURLs(b *testing.B) {
	servers := make([]*httptest.Server, 5)
	urls := make([]string, 5)

	for i := range servers {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		urls[i] = servers[i].URL
	}

	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	client := &http.Client{Timeout: 5 * time.Second}

	b.ResetTimer()
	for b.Loop() {
		checkURLs(client, urls)
	}
}
