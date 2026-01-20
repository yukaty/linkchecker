package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
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

func main() {
	// check command line arguments
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <url>\n", os.Args[0])
		os.Exit(1)
	}

	startURL := os.Args[1]

	// parse base URL
	baseURL, err := url.Parse(startURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid URL: %v\n", err)
		os.Exit(1)
	}

	// create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	fmt.Printf("Checking: %s\n", startURL)

	// get the page
	resp, err := client.Get(startURL)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// check status of start URL
	if resp.StatusCode >= 400 {
		fmt.Printf("✗ [%d] BROKEN\n", resp.StatusCode)
		os.Exit(1)
	}
	fmt.Printf("✓ [%d] OK\n", resp.StatusCode)

	// extract links
	links, err := extractLinks(resp.Body, baseURL)
	if err != nil {
		fmt.Printf("Error extracting links: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d links. Checking...\n\n", len(links))

	// check each link
	brokenCount := 0
	for _, link := range links {
		status, err := checkURL(client, link)
		if err != nil {
			fmt.Printf("✗ [error] %s - %v\n", link, err)
			brokenCount++
		} else if status >= 400 {
			fmt.Printf("✗ [%d] %s\n", status, link)
			brokenCount++
		} else {
			fmt.Printf("✓ [%d] %s\n", status, link)
		}
	}

	// summary
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Summary: %d checked, %d broken\n", len(links), brokenCount)

	if brokenCount > 0 {
		os.Exit(1)
	}
}
