// crawler.go - Concurrent web crawling
package main

import (
	"net/http"
	"net/url"
	"sync"
)

// crawl recursively crawls a URL and its links with concurrency
func crawl(client *http.Client, targetURL, sourceURL, baseDomain string, depth int,
	visited *SafeUrlMap, results *[]LinkResult, resultsMu *sync.Mutex, wg *sync.WaitGroup) {

	defer wg.Done()

	// check if already visited
	if visited.Visit(targetURL) {
		return
	}

	// check the URL
	resp, err := client.Get(targetURL)

	result := LinkResult{
		URL:       targetURL,
		SourceURL: sourceURL,
	}

	if err != nil {
		result.Error = err
		result.IsBroken = true
		resultsMu.Lock()
		*results = append(*results, result)
		resultsMu.Unlock()
		return
	}
	defer resp.Body.Close()

	result.Status = resp.StatusCode
	if resp.StatusCode >= 400 {
		result.IsBroken = true
	}

	resultsMu.Lock()
	*results = append(*results, result)
	resultsMu.Unlock()

	// only follow links if same domain and within depth limit
	if !isSameDomain(targetURL, baseDomain) || depth >= maxDepth || resp.StatusCode >= 400 {
		return
	}

	// parse base URL for this page
	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return
	}

	// extract and crawl links
	links, err := extractLinks(resp.Body, baseURL)
	if err != nil {
		return
	}

	for _, link := range links {
		if isSameDomain(link, baseDomain) {
			// recursively crawl same-domain links in parallel
			wg.Add(1)
			go crawl(client, link, targetURL, baseDomain, depth+1, visited, results, resultsMu, wg)
		} else {
			// just check external links without following
			if !visited.Visit(link) {
				wg.Add(1)
				go func(extLink, srcURL string) {
					defer wg.Done()

					status, err := checkURL(client, extLink)
					extResult := LinkResult{
						URL:       extLink,
						SourceURL: srcURL,
						Status:    status,
						Error:     err,
						IsBroken:  err != nil || status >= 400,
					}

					resultsMu.Lock()
					*results = append(*results, extResult)
					resultsMu.Unlock()
				}(link, targetURL)
			}
		}
	}
}
