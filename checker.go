// checker.go - URL health checking
package main

import (
	"net/http"
	"sync"
)

// checkURL checks if a URL is accessible and returns status code
func checkURL(client *http.Client, targetURL string) (int, error) {
	resp, err := client.Get(targetURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// checkURLs checks multiple URLs in parallel without crawling
func checkURLs(client *http.Client, urls []string) []LinkResult {
	var results []LinkResult
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	for _, targetURL := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			status, err := checkURL(client, url)
			result := LinkResult{
				URL:       url,
				SourceURL: "",
				Status:    status,
				Error:     err,
				IsBroken:  err != nil || status >= 400,
			}

			resultsMu.Lock()
			results = append(results, result)
			resultsMu.Unlock()
		}(targetURL)
	}

	wg.Wait()
	return results
}
