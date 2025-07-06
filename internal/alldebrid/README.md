# AllDebrid API Client Package

## Overview

The `internal/alldebrid` package provides a comprehensive Go client for integrating with the AllDebrid API. AllDebrid is a premium link generator service that allows users to download files from various file hosting services at high speeds. This package implements the core functionality needed to interact with AllDebrid's RESTful API v4.

## Features

- **Link Unrestriction**: Convert premium links to direct download URLs
- **API Key Validation**: Verify API key authenticity and user access
- **Error Handling**: Comprehensive error handling with typed API errors
- **Context Support**: Full context support for request cancellation and timeouts
- **Interface-Based Design**: Clean interface separation for easy testing and mocking
- **Robust HTTP Client**: Configurable HTTP client with sensible defaults

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "debrid-downloader/internal/alldebrid"
)

func main() {
    // Create a new client
    client := alldebrid.New("your-api-key-here")
    
    // Create a context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Validate API key
    if err := client.CheckAPIKey(ctx); err != nil {
        fmt.Printf("API key validation failed: %v\n", err)
        return
    }
    
    // Unrestrict a link
    result, err := client.UnrestrictLink(ctx, "https://example.com/file.zip")
    if err != nil {
        fmt.Printf("Failed to unrestrict link: %v\n", err)
        return
    }
    
    fmt.Printf("Direct download URL: %s\n", result.UnrestrictedURL)
    fmt.Printf("Filename: %s\n", result.Filename)
    fmt.Printf("File size: %d bytes\n", result.FileSize)
}
```

### Using the Interface

```go
package main

import (
    "context"
    "debrid-downloader/internal/alldebrid"
)

type DownloadService struct {
    client alldebrid.AllDebridClient
}

func NewDownloadService(client alldebrid.AllDebridClient) *DownloadService {
    return &DownloadService{client: client}
}

func (s *DownloadService) ProcessLink(ctx context.Context, link string) error {
    result, err := s.client.UnrestrictLink(ctx, link)
    if err != nil {
        return fmt.Errorf("failed to unrestrict link: %w", err)
    }
    
    // Process the unrestricted result
    // ...
    
    return nil
}
```

## Architecture

### Core Components

The package is organized around several key components:

```
internal/alldebrid/
├── client.go          # Main client implementation
├── client_test.go     # Comprehensive test suite
├── mock.go            # Mock generation directive
└── mocks/
    └── mock_client.go # Generated mock for testing
```

### Interface Design

The package follows Go's interface-first design principle:

```go
// AllDebridClient defines the interface for AllDebrid operations
type AllDebridClient interface {
    UnrestrictLink(ctx context.Context, link string) (*UnrestrictResult, error)
    CheckAPIKey(ctx context.Context) error
}
```

This interface allows for:
- Easy testing with mocks
- Dependency injection
- Alternative implementations
- Clean separation of concerns

## API Reference

### Base URL

```
https://api.alldebrid.com/v4
```

### Authentication

All requests require an API key passed as a query parameter:

```
?apikey=YOUR_API_KEY&agent=debrid-downloader
```

### Client Configuration

```go
// Client represents an AllDebrid API client
type Client struct {
    apiKey     string        // API key for authentication
    baseURL    string        // Base URL for API requests
    httpClient *http.Client  // HTTP client with 30s timeout
}
```

### Data Types

#### UnrestrictResult

```go
type UnrestrictResult struct {
    UnrestrictedURL string `json:"link"`     // Direct download URL
    Filename        string `json:"filename"` // Original filename
    FileSize        int64  `json:"filesize"` // File size in bytes
}
```

#### APIResponse

```go
type APIResponse struct {
    Status string          `json:"status"`           // "success" or "error"
    Data   json.RawMessage `json:"data,omitempty"`   // Response data
    Error  *APIError       `json:"error,omitempty"`  // Error details
}
```

#### APIError

```go
type APIError struct {
    Message string      `json:"message"`          // Error message
    Code    interface{} `json:"code,omitempty"`   // Error code (string or int)
}
```

## Core Methods

### New(apiKey string) *Client

Creates a new AllDebrid client with the provided API key.

**Parameters:**
- `apiKey`: Your AllDebrid API key

**Returns:**
- `*Client`: Configured client instance

**Example:**
```go
client := alldebrid.New("your-api-key-here")
```

### UnrestrictLink(ctx context.Context, link string) (*UnrestrictResult, error)

Unrestricts a premium link to generate a direct download URL.

**Parameters:**
- `ctx`: Context for request cancellation and timeout
- `link`: The premium link to unrestrict

**Returns:**
- `*UnrestrictResult`: Contains the direct download URL, filename, and file size
- `error`: Error if the request fails

**API Endpoint:** `GET /v4/link/unlock`

**Example:**
```go
result, err := client.UnrestrictLink(ctx, "https://example.com/file.zip")
if err != nil {
    // Handle error
}
fmt.Printf("Download URL: %s\n", result.UnrestrictedURL)
```

### CheckAPIKey(ctx context.Context) error

Validates the API key by making a test request to the user endpoint.

**Parameters:**
- `ctx`: Context for request cancellation and timeout

**Returns:**
- `error`: Error if validation fails or API key is invalid

**API Endpoint:** `GET /v4/user`

**Example:**
```go
if err := client.CheckAPIKey(ctx); err != nil {
    fmt.Printf("API key is invalid: %v\n", err)
}
```

## Error Handling

The package implements comprehensive error handling with multiple layers:

### HTTP-Level Errors

```go
// HTTP request failures
if resp.StatusCode != http.StatusOK {
    return fmt.Errorf("API request failed with status %d", resp.StatusCode)
}
```

### API-Level Errors

```go
// API response errors
if apiResp.Status != "success" {
    if apiResp.Error != nil {
        return apiResp.Error  // Returns typed APIError
    }
    return fmt.Errorf("API returned status: %s", apiResp.Status)
}
```

### Error Types

The `APIError` type implements the `error` interface and provides detailed error information:

```go
func (e *APIError) Error() string {
    if e.Code != nil {
        return fmt.Sprintf("%s (code: %v)", e.Message, e.Code)
    }
    return e.Message
}
```

**Example Error Handling:**
```go
result, err := client.UnrestrictLink(ctx, invalidLink)
if err != nil {
    if apiErr, ok := err.(*alldebrid.APIError); ok {
        fmt.Printf("API Error: %s\n", apiErr.Message)
        if apiErr.Code != nil {
            fmt.Printf("Error Code: %v\n", apiErr.Code)
        }
    } else {
        fmt.Printf("Request Error: %v\n", err)
    }
}
```

## Testing

The package includes comprehensive testing infrastructure:

### Test Coverage

- **Client Creation**: Tests for proper client initialization
- **Successful Operations**: Tests for valid API responses
- **Error Scenarios**: Tests for various error conditions
- **Edge Cases**: Tests for malformed JSON, network failures, etc.

### Mock Generation

The package uses `mockgen` to generate mocks for testing:

```go
//go:generate mockgen -source=client.go -destination=mocks/mock_client.go -package=mocks
```

**Generate mocks:**
```bash
go generate ./internal/alldebrid/
```

### Using Mocks in Tests

```go
package main

import (
    "context"
    "testing"
    
    "debrid-downloader/internal/alldebrid"
    "debrid-downloader/internal/alldebrid/mocks"
    "go.uber.org/mock/gomock"
)

func TestDownloadService(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockClient := mocks.NewMockAllDebridClient(ctrl)
    service := NewDownloadService(mockClient)
    
    // Set up expectations
    mockClient.EXPECT().
        UnrestrictLink(gomock.Any(), "https://example.com/file.zip").
        Return(&alldebrid.UnrestrictResult{
            UnrestrictedURL: "https://direct.link/file.zip",
            Filename:        "file.zip",
            FileSize:        1024000,
        }, nil)
    
    // Test the service
    err := service.ProcessLink(context.Background(), "https://example.com/file.zip")
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
}
```

### Test Server Setup

The test suite uses `httptest.NewServer` to create mock HTTP servers:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status": "success", "data": {"link": "https://direct.link"}}`))
}))
defer server.Close()

client := alldebrid.New("test-key")
client.baseURL = server.URL  // Override for testing
```

## Configuration

### Environment Variables

The AllDebrid client is typically configured through environment variables:

```env
# Required: Your AllDebrid API key
ALLDEBRID_API_KEY=your_api_key_here

# Optional: Server configuration
SERVER_PORT=3000
DATABASE_PATH=debrid.db
BASE_DOWNLOADS_PATH=/downloads
LOG_LEVEL=info
```

### Configuration Structure

```go
type Config struct {
    AllDebridAPIKey   string `env:"ALLDEBRID_API_KEY,required"`
    ServerPort        string `env:"SERVER_PORT" envDefault:"8080"`
    LogLevel          string `env:"LOG_LEVEL" envDefault:"info"`
    DatabasePath      string `env:"DATABASE_PATH" envDefault:"debrid.db"`
    BaseDownloadsPath string `env:"BASE_DOWNLOADS_PATH" envDefault:"/downloads"`
}
```

## Integration Examples

### Web Application Integration

```go
package main

import (
    "debrid-downloader/internal/alldebrid"
    "debrid-downloader/internal/config"
    "debrid-downloader/internal/web"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }
    
    // Create AllDebrid client
    allDebridClient := alldebrid.New(cfg.AllDebridAPIKey)
    
    // Validate API key on startup
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := allDebridClient.CheckAPIKey(ctx); err != nil {
        log.Printf("Warning: AllDebrid API key validation failed: %v", err)
    }
    
    // Initialize web server with client
    server := web.NewServer(db, allDebridClient, cfg, downloadWorker)
    server.Start()
}
```

### Download Handler Integration

```go
func (h *Handlers) SubmitDownload(w http.ResponseWriter, r *http.Request) {
    // Parse form data
    url := r.FormValue("url")
    directory := r.FormValue("directory")
    
    // Unrestrict the URL using AllDebrid
    result, err := h.allDebridClient.UnrestrictLink(r.Context(), url)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to unrestrict URL: %s", err.Error()), 
                  http.StatusBadRequest)
        return
    }
    
    // Create download record
    download := &models.Download{
        OriginalURL:     url,
        UnrestrictedURL: result.UnrestrictedURL,
        Filename:        result.Filename,
        Directory:       directory,
        Status:          models.StatusPending,
        FileSize:        result.FileSize,
        CreatedAt:       time.Now(),
        UpdatedAt:       time.Now(),
    }
    
    // Save to database and queue for download
    if err := h.db.CreateDownload(download); err != nil {
        http.Error(w, "Failed to create download record", 
                  http.StatusInternalServerError)
        return
    }
    
    h.downloadWorker.QueueDownload(download.ID)
}
```

## Rate Limiting Considerations

### AllDebrid API Limits

- **Request Rate**: The API has built-in rate limiting
- **Concurrent Requests**: Limit concurrent API calls to avoid throttling
- **Retry Logic**: Implement exponential backoff for failed requests

### Best Practices

1. **Implement Request Throttling**:
```go
type ThrottledClient struct {
    client alldebrid.AllDebridClient
    limiter *rate.Limiter
}

func (t *ThrottledClient) UnrestrictLink(ctx context.Context, link string) (*alldebrid.UnrestrictResult, error) {
    if err := t.limiter.Wait(ctx); err != nil {
        return nil, err
    }
    return t.client.UnrestrictLink(ctx, link)
}
```

2. **Batch Operations**:
```go
func ProcessMultipleLinks(ctx context.Context, client alldebrid.AllDebridClient, links []string) ([]*alldebrid.UnrestrictResult, error) {
    results := make([]*alldebrid.UnrestrictResult, 0, len(links))
    
    for _, link := range links {
        result, err := client.UnrestrictLink(ctx, link)
        if err != nil {
            log.Printf("Failed to process link %s: %v", link, err)
            continue
        }
        results = append(results, result)
        
        // Small delay between requests
        time.Sleep(100 * time.Millisecond)
    }
    
    return results, nil
}
```

3. **Circuit Breaker Pattern**:
```go
type CircuitBreakerClient struct {
    client alldebrid.AllDebridClient
    breaker *circuit.Breaker
}

func (c *CircuitBreakerClient) UnrestrictLink(ctx context.Context, link string) (*alldebrid.UnrestrictResult, error) {
    result, err := c.breaker.Execute(func() (interface{}, error) {
        return c.client.UnrestrictLink(ctx, link)
    })
    
    if err != nil {
        return nil, err
    }
    
    return result.(*alldebrid.UnrestrictResult), nil
}
```

## Best Practices

### 1. Context Management

Always use context for timeout and cancellation:

```go
// Good: Use context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := client.UnrestrictLink(ctx, link)
```

### 2. Error Handling

Implement comprehensive error handling:

```go
result, err := client.UnrestrictLink(ctx, link)
if err != nil {
    if apiErr, ok := err.(*alldebrid.APIError); ok {
        // Handle API-specific errors
        log.Printf("API Error: %s (code: %v)", apiErr.Message, apiErr.Code)
    } else {
        // Handle network/HTTP errors
        log.Printf("Request Error: %v", err)
    }
    return err
}
```

### 3. Resource Management

Ensure proper cleanup of resources:

```go
func ProcessLinks(client alldebrid.AllDebridClient, links []string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel() // Always clean up
    
    for _, link := range links {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := processLink(ctx, client, link); err != nil {
                log.Printf("Failed to process link %s: %v", link, err)
            }
        }
    }
    
    return nil
}
```

### 4. Logging and Monitoring

Implement structured logging:

```go
func (c *Client) UnrestrictLink(ctx context.Context, link string) (*UnrestrictResult, error) {
    start := time.Now()
    
    result, err := c.makeRequest(ctx, link)
    
    // Log the operation
    fields := []slog.Attr{
        slog.String("operation", "unrestrict_link"),
        slog.String("link", link),
        slog.Duration("duration", time.Since(start)),
    }
    
    if err != nil {
        fields = append(fields, slog.String("error", err.Error()))
        slog.ErrorContext(ctx, "Failed to unrestrict link", fields...)
    } else {
        fields = append(fields, slog.String("filename", result.Filename))
        slog.InfoContext(ctx, "Successfully unrestricted link", fields...)
    }
    
    return result, err
}
```

## Troubleshooting

### Common Issues

1. **Invalid API Key**:
```
Error: Invalid API key (code: 401)
```
- Verify your API key is correct
- Check if your AllDebrid account is active
- Ensure the API key has proper permissions

2. **Rate Limiting**:
```
Error: Too many requests (code: 429)
```
- Implement request throttling
- Add delays between API calls
- Use exponential backoff for retries

3. **Network Timeouts**:
```
Error: context deadline exceeded
```
- Increase timeout duration
- Check network connectivity
- Consider implementing retries

4. **Invalid Links**:
```
Error: Link not supported
```
- Verify the link format is correct
- Check if the hosting service is supported
- Ensure the link is still active

### Debugging

Enable debug logging to troubleshoot issues:

```go
// Set log level to debug
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})))

// The client will log detailed information
client := alldebrid.New(apiKey)
result, err := client.UnrestrictLink(ctx, link)
```

## Security Considerations

### API Key Protection

1. **Environment Variables**: Store API keys in environment variables
2. **Secret Management**: Use secure secret management systems in production
3. **Key Rotation**: Regularly rotate API keys
4. **Access Control**: Limit API key access to necessary services only

### Request Security

1. **TLS**: All requests are made over HTTPS
2. **Input Validation**: Validate all input parameters
3. **Rate Limiting**: Implement client-side rate limiting
4. **Logging**: Avoid logging sensitive information

## Contributing

### Development Setup

1. **Install Dependencies**:
```bash
go mod tidy
```

2. **Generate Mocks**:
```bash
go generate ./internal/alldebrid/
```

3. **Run Tests**:
```bash
go test ./internal/alldebrid/...
```

4. **Run Linting**:
```bash
golangci-lint run ./internal/alldebrid/...
```

### Code Style

- Follow Go conventions and best practices
- Use meaningful variable and function names
- Write comprehensive tests for new functionality
- Document public APIs with clear examples
- Keep functions focused and single-purpose

## License

This package is part of the debrid-downloader project and follows the same license terms.