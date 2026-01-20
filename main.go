// main.go - CLI entry point
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	// define flags
	jsonFlag := flag.Bool("json", false, "Output results as JSON for CI/CD integration")
	quietFlag := flag.Bool("quiet", false, "Suppress output, only show errors (useful with -json)")
	timeoutFlag := flag.Duration("timeout", 10*time.Second, "HTTP request timeout (e.g., 10s, 30s, 1m)")
	flag.Parse()

	// get arguments
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <url|file> [url|file...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nArguments:\n")
		fmt.Fprintf(os.Stderr, "  url               Direct URL (http:// or https://)\n")
		fmt.Fprintf(os.Stderr, "  file.md           Markdown file (extracts links)\n")
		fmt.Fprintf(os.Stderr, "  file.txt          URL list file (one URL per line)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s https://example.com                    # Crawl mode (single URL)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s https://github.com https://google.com  # Direct check mode (multiple URLs)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s post.md                                # Check links in Markdown file\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s docs/*.md                              # Check links in multiple Markdown files\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s urls.txt                               # Check URLs from text file\n", os.Args[0])
		os.Exit(1)
	}

	// process arguments and collect URLs
	var urls []string
	for _, arg := range args {
		switch {
		case strings.HasSuffix(arg, ".md"):
			// Markdown file - extract links
			content, err := os.ReadFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading Markdown file %s: %v\n", arg, err)
				os.Exit(1)
			}
			extractedURLs := extractMarkdownLinks(string(content))
			if len(extractedURLs) == 0 {
				fmt.Fprintf(os.Stderr, "Warning: No URLs found in %s\n", arg)
			}
			urls = append(urls, extractedURLs...)

		case strings.HasSuffix(arg, ".txt"):
			// Text file - read URLs line by line
			file, err := os.Open(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", arg, err)
				os.Exit(1)
			}
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					urls = append(urls, line)
				}
			}
			file.Close()
			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", arg, err)
				os.Exit(1)
			}

		case strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://"):
			// Direct URL
			urls = append(urls, arg)

		default:
			fmt.Fprintf(os.Stderr, "Error: Invalid argument '%s'\n", arg)
			fmt.Fprintf(os.Stderr, "Expected: URL (http://...), Markdown file (.md), or URL list (.txt)\n")
			os.Exit(1)
		}
	}

	// validate we have at least one URL
	if len(urls) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No URLs to check\n")
		os.Exit(1)
	}

	// create HTTP client with configurable timeout
	client := &http.Client{
		Timeout: *timeoutFlag,
	}

	var results []LinkResult

	// mode detection
	if len(urls) == 1 {
		// single URL - crawl mode
		startURL := urls[0]
		if !*quietFlag {
			fmt.Printf("🔍 Crawling: %s (depth: %d)\n\n", startURL, maxDepth)
		}

		visited := &SafeUrlMap{visited: make(map[string]bool)}
		var resultsMu sync.Mutex
		var wg sync.WaitGroup

		wg.Add(1)
		go crawl(client, startURL, "", startURL, 0, visited, &results, &resultsMu, &wg)
		wg.Wait()
	} else {
		// multiple URLs - direct check mode
		if !*quietFlag {
			fmt.Printf("🔍 Checking %d URLs...\n\n", len(urls))
		}
		results = checkURLs(client, urls)
	}

	// display results
	brokenCount := 0
	for _, result := range results {
		if result.IsBroken {
			brokenCount++
		}
	}

	if *jsonFlag {
		// JSON output for CI/CD integration
		outputJSON(results, brokenCount)
	} else {
		// Human-readable output
		outputHuman(results, brokenCount, *quietFlag)
	}

	if brokenCount > 0 {
		os.Exit(1)
	}
}

// outputJSON outputs results in JSON format for CI/CD integration
func outputJSON(results []LinkResult, brokenCount int) {
	jsonResults := make([]JSONResult, len(results))
	for i, result := range results {
		var errStr *string
		if result.Error != nil {
			s := result.Error.Error()
			errStr = &s
		}

		jsonResults[i] = JSONResult{
			URL:       result.URL,
			Status:    result.Status,
			Error:     errStr,
			Broken:    result.IsBroken,
			SourceURL: result.SourceURL,
		}
	}

	output := JSONOutput{
		Summary: JSONSummary{
			Total:   len(results),
			Broken:  brokenCount,
			Success: len(results) - brokenCount,
		},
		Results: jsonResults,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

// outputHuman outputs results in human-readable format
func outputHuman(results []LinkResult, brokenCount int, quiet bool) {
	if !quiet {
		fmt.Println("Results:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	for _, result := range results {
		if result.IsBroken {
			if result.Error != nil {
				fmt.Printf("✗ [error] %s\n", result.URL)
				if result.SourceURL != "" {
					fmt.Printf("  └─ Source: %s\n", result.SourceURL)
				}
				fmt.Printf("  └─ Error: %v\n", result.Error)
			} else {
				fmt.Printf("✗ [%d] %s\n", result.Status, result.URL)
				if result.SourceURL != "" {
					fmt.Printf("  └─ Source: %s\n", result.SourceURL)
				}
			}
			fmt.Println()
		} else if !quiet {
			fmt.Printf("✓ [%d] %s\n", result.Status, result.URL)
		}
	}

	if !quiet {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Summary: %d checked, %d broken\n", len(results), brokenCount)
	}
}
