# Link Checker

A fast, minimal link checker for validating websites and detecting broken links before deployment.

## Features

- **Crawl Mode**: Recursively check internal links with depth limits
- **Direct Mode**: Check one or more URLs
- **Batch Input**: Read URLs from a file
- **JSON Output**: Structured output for CI/CD integration

## Usage

### Install

```bash
go install github.com/yourname/linkchecker@latest
```

### Check a website

```bash
linkchecker https://example.com
```

### Check multiple URLs from a file

```bash
linkchecker -file urls.txt
```

### JSON output (for CI)

```bash
linkchecker -json -file urls.txt > results.json
```

Exit code `1` if broken links found.

### Flags

```bash
❯ linkchecker -h
Usage of linkchecker:
  -file string
        File containing URLs to check (one per line)
  -json
        Output results as JSON for CI/CD integration
  -quiet
        Suppress output, only show errors
  -timeout duration
        HTTP request timeout (e.g., 10s, 30s, 1m) (default 10s)
```

## Testing

```bash
go test ./...
go test -race ./...  # Race condition detection
go test -bench=.     # Benchmark performance
```

## License

MIT
