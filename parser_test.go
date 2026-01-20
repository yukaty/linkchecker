package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestIsSameDomain(t *testing.T) {
	tests := []struct {
		name string
		url1 string
		url2 string
		want bool
	}{
		{
			name: "same domain exact match",
			url1: "https://example.com/page1",
			url2: "https://example.com/page2",
			want: true,
		},
		{
			name: "same domain different paths",
			url1: "https://example.com/a/b/c",
			url2: "https://example.com/x/y/z",
			want: true,
		},
		{
			name: "different domains",
			url1: "https://example.com",
			url2: "https://different.com",
			want: false,
		},
		{
			name: "different subdomains",
			url1: "https://www.example.com",
			url2: "https://api.example.com",
			want: false,
		},
		{
			name: "same domain different schemes",
			url1: "http://example.com",
			url2: "https://example.com",
			want: true,
		},
		{
			name: "same domain different ports",
			url1: "https://example.com:8080",
			url2: "https://example.com:9090",
			want: false,
		},
		{
			name: "invalid url1",
			url1: "://invalid",
			url2: "https://example.com",
			want: false,
		},
		{
			name: "invalid url2",
			url1: "https://example.com",
			url2: "://invalid",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSameDomain(tt.url1, tt.url2)
			if got != tt.want {
				t.Errorf("isSameDomain(%q, %q) = %v, want %v", tt.url1, tt.url2, got, tt.want)
			}
		})
	}
}

func TestExtractLinks(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		baseURL  string
		wantURLs []string
		wantErr  bool
	}{
		{
			name: "absolute URLs",
			html: `<html><body>
				<a href="https://example.com/page1">Link 1</a>
				<a href="https://example.com/page2">Link 2</a>
			</body></html>`,
			baseURL:  "https://example.com",
			wantURLs: []string{"https://example.com/page1", "https://example.com/page2"},
			wantErr:  false,
		},
		{
			name: "relative URLs",
			html: `<html><body>
				<a href="/about">About</a>
				<a href="contact">Contact</a>
			</body></html>`,
			baseURL:  "https://example.com",
			wantURLs: []string{"https://example.com/about", "https://example.com/contact"},
			wantErr:  false,
		},
		{
			name: "skip anchors and javascript",
			html: `<html><body>
				<a href="#">Anchor</a>
				<a href="#section">Section</a>
				<a href="javascript:void(0)">JS</a>
				<a href="mailto:test@example.com">Email</a>
				<a href="/valid">Valid</a>
			</body></html>`,
			baseURL:  "https://example.com",
			wantURLs: []string{"https://example.com/valid"},
			wantErr:  false,
		},
		{
			name: "empty href",
			html: `<html><body>
				<a href="">Empty</a>
				<a>No href</a>
				<a href="/page">Valid</a>
			</body></html>`,
			baseURL:  "https://example.com",
			wantURLs: []string{"https://example.com/page"},
			wantErr:  false,
		},
		{
			name: "mixed case and query params",
			html: `<html><body>
				<a href="/search?q=test">Search</a>
				<a href="/page#anchor">Page with anchor</a>
			</body></html>`,
			baseURL:  "https://example.com",
			wantURLs: []string{"https://example.com/search?q=test", "https://example.com/page#anchor"},
			wantErr:  false,
		},
		{
			name:     "no links",
			html:     `<html><body><p>No links here</p></body></html>`,
			baseURL:  "https://example.com",
			wantURLs: []string{},
			wantErr:  false,
		},
		{
			name: "relative path resolution",
			html: `<html><body>
				<a href="../parent">Parent</a>
				<a href="./current">Current</a>
			</body></html>`,
			baseURL:  "https://example.com/dir/page",
			wantURLs: []string{"https://example.com/parent", "https://example.com/dir/current"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, err := url.Parse(tt.baseURL)
			if err != nil {
				t.Fatalf("Failed to parse base URL: %v", err)
			}

			reader := strings.NewReader(tt.html)
			got, err := extractLinks(reader, baseURL)

			if (err != nil) != tt.wantErr {
				t.Errorf("extractLinks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.wantURLs) {
				t.Errorf("extractLinks() got %d links, want %d\nGot: %v\nWant: %v",
					len(got), len(tt.wantURLs), got, tt.wantURLs)
				return
			}

			for i, wantURL := range tt.wantURLs {
				if got[i] != wantURL {
					t.Errorf("extractLinks()[%d] = %q, want %q", i, got[i], wantURL)
				}
			}
		})
	}
}

func BenchmarkIsSameDomain(b *testing.B) {
	url1 := "https://example.com/page1"
	url2 := "https://example.com/page2"

	b.ResetTimer()
	for b.Loop() {
		isSameDomain(url1, url2)
	}
}

func BenchmarkExtractLinks(b *testing.B) {
	html := `<html><body>
		<a href="https://example.com/page1">Link 1</a>
		<a href="https://example.com/page2">Link 2</a>
		<a href="/relative">Relative</a>
		<a href="#anchor">Anchor</a>
		<a href="javascript:void(0)">JS</a>
	</body></html>`

	baseURL, _ := url.Parse("https://example.com")

	b.ResetTimer()
	for b.Loop() {
		reader := strings.NewReader(html)
		extractLinks(reader, baseURL)
	}
}
