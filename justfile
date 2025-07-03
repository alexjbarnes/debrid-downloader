# Development commands for debrid-downloader

# Default recipe to display available commands
default:
    @just --list

# Build the application
build:
    templ generate
    go build -o bin/debrid-downloader ./cmd/debrid-downloader

# Run the application
run:
    go run ./cmd/debrid-downloader

# Run tests with coverage
test:
    go test -v -race -coverprofile=coverage.out ./...

# Run tests with coverage and open coverage report
test-coverage:
    go test -v -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report generated: coverage.html"

# Check test coverage percentage
coverage-check:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $3}'

# Get test coverage excluding generated files (mocks, templates, etc.)
# Default: analyze entire repo
coverage:
    #!/usr/bin/env bash
    set -euo pipefail
    TARGET_PATH="./..."
    echo "Running coverage analysis on: $TARGET_PATH"
    echo "=============================================="
    
    # Run tests with coverage
    go test -coverprofile=coverage.out "$TARGET_PATH" 2>/dev/null
    
    # Get overall coverage first
    TOTAL_COVERAGE=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}')
    
    # Show detailed coverage by file (excluding generated files)
    echo "Coverage by file (excluding generated files):"
    echo "============================================="
    go tool cover -func=coverage.out | \
        grep -v "_test.go" | \
        grep -v "mock_" | \
        grep -v "_templ.go" | \
        grep -v "/mocks/" | \
        grep -v "/templates/" | \
        grep "\.go:" | \
        awk '{ 
            gsub(/^.*\//, "", $1)  # Remove path prefix
            printf "%-50s %s\n", $1, $3
        }' | head -20
    
    echo ""
    echo "Package-level coverage:"
    echo "======================"
    go test -cover "$TARGET_PATH" 2>/dev/null | \
        grep -v "coverage: 0.0%" | \
        grep -v "/mocks" | \
        grep -v "/templates" | \
        grep "coverage:" | \
        awk '{
            # Extract package name and coverage
            package = $2
            gsub(/^.*\//, "", package)  # Remove path prefix
            coverage = $(NF-2)
            printf "%-40s %s\n", package, coverage
        }'
    
    echo ""
    printf "%-40s %s\n" "TOTAL COVERAGE:" "$TOTAL_COVERAGE"
    
    # Clean up
    rm -f coverage.out
    
    echo ""
    echo "Notes:"
    echo "- Excludes: test files, mocks, templ files, generated code"
    echo "- Use 'just coverage-dir' to analyze current directory only"

# Get test coverage for current directory only (where just was invoked from)
coverage-dir:
    #!/usr/bin/env bash
    set -euo pipefail
    
    # Use just's built-in invocation_directory to get where just was called from
    INVOCATION_DIR="{{invocation_directory()}}"
    JUSTFILE_DIR="{{justfile_directory()}}"
    
    if [[ "$INVOCATION_DIR" == "$JUSTFILE_DIR" ]]; then
        TARGET_PATH="./..."
    else
        # Calculate relative path from justfile dir to invocation dir
        REL_PATH="${INVOCATION_DIR#$JUSTFILE_DIR/}"
        TARGET_PATH="./$REL_PATH"
    fi
    
    echo "Running coverage analysis on: $TARGET_PATH"
    echo "Invoked from: {{invocation_directory()}}"
    echo "=============================================="
    
    # Run tests with coverage
    go test -coverprofile=coverage.out "$TARGET_PATH" 2>/dev/null
    
    # Get overall coverage first
    TOTAL_COVERAGE=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}')
    
    # Show detailed coverage by file (excluding generated files)
    echo "Coverage by file (excluding generated files):"
    echo "============================================="
    go tool cover -func=coverage.out | \
        grep -v "_test.go" | \
        grep -v "mock_" | \
        grep -v "_templ.go" | \
        grep -v "/mocks/" | \
        grep -v "/templates/" | \
        grep "\.go:" | \
        awk '{ 
            gsub(/^.*\//, "", $1)  # Remove path prefix
            printf "%-50s %s\n", $1, $3
        }' | head -20
    
    echo ""
    echo "Package-level coverage:"
    echo "======================"
    go test -cover "$TARGET_PATH" 2>/dev/null | \
        grep -v "coverage: 0.0%" | \
        grep -v "/mocks" | \
        grep -v "/templates" | \
        grep "coverage:" | \
        awk '{
            # Extract package name and coverage
            package = $2
            gsub(/^.*\//, "", package)  # Remove path prefix
            coverage = $(NF-2)
            printf "%-40s %s\n", package, coverage
        }'
    
    echo ""
    printf "%-40s %s\n" "TOTAL COVERAGE:" "$TOTAL_COVERAGE"
    
    # Clean up
    rm -f coverage.out
    
    echo ""
    echo "Notes:"
    echo "- Excludes: test files, mocks, templ files, generated code"
    echo "- Use 'just coverage-dir' to analyze only the directory you're in"

# Format code
fmt:
    gofumpt -w .

# Run linters
lint:
    golangci-lint run
    staticcheck ./...

# Run go vet
vet:
    go vet ./...

# Generate mocks
mocks:
    go generate ./...

# Run all quality checks
check: templ-generate fmt vet lint test

# Clean build artifacts
clean:
    rm -rf bin/ coverage.out coverage.html

# Download dependencies
deps:
    go mod download
    go mod tidy

# Generate templ templates
templ-generate:
    templ generate

# Install development tools
install-tools:
    go install mvdan.cc/gofumpt@latest
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install honnef.co/go/tools/cmd/staticcheck@latest
    go install go.uber.org/mock/mockgen@latest
    go install github.com/a-h/templ/cmd/templ@latest