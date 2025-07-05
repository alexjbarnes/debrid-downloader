# Development commands for debrid-downloader

# Default recipe to display available commands
default:
    @just --list

# Build the application
build: generate
    go build -o . ./cmd/debrid-downloader

# Run the application
run: generate
    go run ./cmd/debrid-downloader

# Run tests
test:
    go test -v -race ./...

# Calculate coverage foe whole repo
coverage:
    @just coverage-dir .

# Calculate test coverage for either the current directory or the passed in directory
coverage-dir *d: generate
    #!/usr/bin/env bash
    set -euo pipefail
    
    # Use the provided directory from Just parameter
    TARGET_DIR="{{d}}"
    
    # Default to current directory if empty
    if [ -z "$TARGET_DIR" ]; then
        TARGET_DIR="."
    fi
    
    # Ensure the directory exists
    if [ ! -d "$TARGET_DIR" ]; then
        echo "Error: Directory '$TARGET_DIR' does not exist"
        exit 1
    fi
    
    # Run tests with coverage - capture both per-package and profile output
    TEST_OUTPUT=$(go test -short -cover -coverprofile=coverage.out "./$TARGET_DIR/..." 2>&1)
    
    if [ ! -f coverage.out ]; then
        echo "No coverage data generated"
        echo "$TEST_OUTPUT"
        exit 1
    fi
    
    echo "Package Coverage for $TARGET_DIR (excluding mocks and generated files):"
    echo "======================================================="
    
    # Parse per-package coverage from the captured test output
    echo "$TEST_OUTPUT" | \
        grep -E "(ok|FAIL)" | \
        grep -v "mocks" | \
        grep -v "templates" | \
        grep "coverage:" | \
        sort | \
        awk '{
            package = $2
            gsub(/^debrid-downloader\//, "", package)  # Remove project prefix only
            # Handle special case for "coverage: [no statements]"
            if ($0 ~ /\[no statements\]/) {
                coverage = "[no statements]"
            } else {
                coverage = $(NF-2)
            }
            printf "%-40s %s\n", package, coverage
        }'
    
    echo ""
    echo "Overall Coverage (excluding mocks and generated files):"
    echo "======================================================="
    
    # Calculate total coverage excluding mocks and generated files
    grep -v "_templ.go" coverage.out | grep -v "/mocks/" | grep -v "/templates/" > coverage_filtered.out
    TOTAL_COV=$(go tool cover -func=coverage_filtered.out | grep "total:" | awk '{print $3}')
    
    if [ "$TARGET_DIR" = "." ]; then
        printf "%-40s %s\n" "TOTAL REPOSITORY:" "$TOTAL_COV"
    else
        printf "%-40s %s\n" "TOTAL:" "$TOTAL_COV"
    fi
    
    # Clean up
    rm -f coverage.out coverage_filtered.out

# generate all files
generate: templ mocks

# Format code
fmt:
    gofumpt -w .

# Run linters
lint:
    golangci-lint run
    staticcheck ./...
    go vet ./...

# Generate mocks
mocks:
    go generate ./...

# Clean build artifacts
clean:
    rm -rf debrid-downloader coverage.out coverage.html

# Download dependencies
mod:
    go mod download
    go mod tidy

# Generate templ templates
templ:
    templ generate

# Install development tools
install-tools:
    go install mvdan.cc/gofumpt@latest
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install honnef.co/go/tools/cmd/staticcheck@latest
    go install go.uber.org/mock/mockgen@latest
    go install github.com/a-h/templ/cmd/templ@latest
