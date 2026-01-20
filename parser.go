// parser.go - URL parsing and HTML link extraction
package main

import (
	"io"
	"net/url"
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
