# Agent Development Guidelines

## Version Control
- Always check for sensitive information before commiting code.

## Build/Test Commands
- `make build` - Build the binary to `bin/conflux`
- `make test` - Run all tests with `go test ./...`
- `make lint` - Run golangci-lint with configured linters
- `make clean` - Clean build artifacts
- `make run` or `make dev` - Run with example config (dev includes -verbose -dry-run)

## Code Style & Conventions
- **Module**: `conflux` (import paths start with `conflux/`)
- **Imports**: Use gci linter ordering (standard, default, conflux prefix)
- **Formatting**: Use `gofmt` for consistent formatting
- **Error Handling**: Wrap errors with `fmt.Errorf("context: %w", err)`
- **Naming**: Use Go conventions - exported PascalCase, unexported camelCase
- **Structs**: Use struct tags for YAML/JSON serialization
- **Logging**: Use custom logger from `conflux/pkg/logger` with levels (Info, Debug, Error, Fatal)
- **Config**: Use YAML config files with validation methods

## Project Structure
- `cmd/conflux/` - Main application entry point
- `internal/` - Private application code (config, confluence, markdown, sync)
- `pkg/` - Public reusable packages (logger, version)
- No test files exist currently - add `_test.go` files when writing tests

## Linting
Enabled linters: gci, gofmt, govet, misspell, staticcheck, unused, gosec, errcheck
