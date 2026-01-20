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
	fileFlag := flag.String("file", "", "File containing URLs to check (one per line)")
	jsonFlag := flag.Bool("json", false, "Output results as JSON for CI/CD integration")
	quietFlag := flag.Bool("quiet", false, "Suppress output, only show errors (useful with -json)")
	timeoutFlag := flag.Duration("timeout", 10*time.Second, "HTTP request timeout (e.g., 10s, 30s, 1m)")
	flag.Parse()

	// get URLs from arguments or file
	var urls []string

	if *fileFlag != "" {
		// read URLs from file
		file, err := os.Open(*fileFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				urls = append(urls, line)
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// get URLs from command line arguments
		urls = flag.Args()
	}

	// validate we have at least one URL
	if len(urls) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-file <filename>] <url> [url...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s https://example.com                    # Crawl mode (single URL)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s https://github.com https://google.com  # Direct check mode (multiple URLs)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -file links.txt                        # Check URLs from file\n", os.Args[0])
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
