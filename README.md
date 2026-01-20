# Link Checker

A fast, minimal link checker for websites and Markdown files.

## Features

- Check websites and Markdown files
- JSON output for CI/CD integration

## Install

```bash
go install github.com/yourname/linkchecker@latest
```

## Usage

```bash
linkchecker https://example.com
linkchecker post.md
linkchecker docs/*.md
linkchecker urls.txt
```

## CI usage

```bash
linkchecker -json -quiet urls.txt
```

Exits with status code `1` if any broken links are found.

## Testing

```bash
go test ./...
```

## License

MIT
