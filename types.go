// types.go - Data structures for link checking
package main

import "sync"

const maxDepth = 2 // maximum crawl depth

// LinkResult stores the result of checking a link
type LinkResult struct {
	URL       string
	Status    int
	Error     error
	IsBroken  bool
	SourceURL string
}

// JSONOutput represents the machine-readable output format for CI/CD integration
type JSONOutput struct {
	Summary JSONSummary  `json:"summary"`
	Results []JSONResult `json:"results"`
}

// JSONSummary contains aggregate statistics
type JSONSummary struct {
	Total   int `json:"total"`
	Broken  int `json:"broken"`
	Success int `json:"success"`
}

// JSONResult represents a single link check result
type JSONResult struct {
	URL       string  `json:"url"`
	Status    int     `json:"status"`
	Error     *string `json:"error,omitempty"`
	Broken    bool    `json:"broken"`
	SourceURL string  `json:"source,omitempty"`
}

// SafeUrlMap provides thread-safe access to visited URLs
type SafeUrlMap struct {
	visited map[string]bool
	mu      sync.Mutex
}

// Visit marks a URL as visited, returns true if already visited
func (s *SafeUrlMap) Visit(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.visited[url] {
		return true
	}
	s.visited[url] = true
	return false
}
