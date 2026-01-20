// parser.go - URL parsing and HTML link extraction
package main

import (
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// isSameDomain checks if two URLs have the same domain
func isSameDomain(url1, url2 string) bool {
	u1, err1 := url.Parse(url1)
	u2, err2 := url.Parse(url2)
	if err1 != nil || err2 != nil {
		return false
	}
	return u1.Host == u2.Host
}

// extractLinks extracts all links from HTML
func extractLinks(body io.Reader, baseURL *url.URL) ([]string, error) {
	var links []string
	tokenizer := html.NewTokenizer(body)

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				return links, nil
			}
			return links, err

		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						link := attr.Val

						// skip empty, anchors, and non-http links
						if link == "" || link == "#" || strings.HasPrefix(link, "#") ||
							strings.HasPrefix(link, "javascript:") ||
							strings.HasPrefix(link, "mailto:") {
							continue
						}

						// resolve relative URLs
						parsedLink, err := url.Parse(link)
						if err != nil {
							continue
						}
						absoluteURL := baseURL.ResolveReference(parsedLink)
						links = append(links, absoluteURL.String())
						break
					}
				}
			}
		}
	}
}

// extractMarkdownLinks extracts URLs from Markdown content
// Supports: [text](url) and bare URLs (http://... or https://...)
// URLs are returned in order of appearance in the document
func extractMarkdownLinks(content string) []string {
	var urls []string
	seen := make(map[string]bool)

	// Track positions of all URL occurrences
	type urlMatch struct {
		url string
		pos int
	}
	var allMatches []urlMatch

	// Find Markdown link syntax: [text](url)
	markdownLinkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdMatches := markdownLinkRe.FindAllStringSubmatchIndex(content, -1)
	for _, match := range mdMatches {
		if len(match) >= 6 {
			link := strings.TrimSpace(content[match[4]:match[5]])
			if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
				allMatches = append(allMatches, urlMatch{url: link, pos: match[0]})
			}
		}
	}

	// Find bare URLs (not inside markdown links)
	// First, create a mask of positions to skip (markdown link regions)
	skipRanges := make([][2]int, len(mdMatches))
	for i, match := range mdMatches {
		skipRanges[i] = [2]int{match[0], match[1]}
	}

	bareURLRe := regexp.MustCompile(`https?://[^\s<>"{}|\\^\[\]` + "`" + `()]+`)
	bareMatches := bareURLRe.FindAllStringIndex(content, -1)
	for _, match := range bareMatches {
		// Check if this bare URL is inside a markdown link
		skip := false
		for _, skipRange := range skipRanges {
			if match[0] >= skipRange[0] && match[1] <= skipRange[1] {
				skip = true
				break
			}
		}
		if !skip {
			link := strings.TrimSpace(content[match[0]:match[1]])
			allMatches = append(allMatches, urlMatch{url: link, pos: match[0]})
		}
	}

	// Sort by position to maintain document order
	for i := 0; i < len(allMatches); i++ {
		for j := i + 1; j < len(allMatches); j++ {
			if allMatches[i].pos > allMatches[j].pos {
				allMatches[i], allMatches[j] = allMatches[j], allMatches[i]
			}
		}
	}

	// Build result list, removing duplicates while preserving order
	for _, match := range allMatches {
		if !seen[match.url] {
			urls = append(urls, match.url)
			seen[match.url] = true
		}
	}

	return urls
}
