# internal/config Package

## Overview

The `internal/config` package provides centralized configuration management for the debrid-downloader application. It handles environment variable parsing, configuration validation, and provides a unified interface for accessing application settings.

**Key Features:**
- Environment variable-based configuration with .env file support
- Automatic validation of configuration values
- Default value handling for optional settings
- Type-safe configuration struct with proper error handling
- Path validation and sanitization for security

## Quick Start

```go
import "github.com/your-org/debrid-downloader/internal/config"

// Load configuration from environment variables
cfg, err := config.Load()
if err != nil {
    log.Fatal("Failed to load configuration:", err)
}

// Access configuration values
fmt.Printf("Server will run on port: %s\n", cfg.ServerPort)
fmt.Printf("Downloads path: %s\n", cfg.BaseDownloadsPath)
```

## Configuration Structure

### Config Type

```go
type Config struct {
    AllDebridAPIKey   string `env:"ALLDEBRID_API_KEY,required"`
    ServerPort        string `env:"SERVER_PORT" envDefault:"8080"`
    LogLevel          string `env:"LOG_LEVEL" envDefault:"info"`
    DatabasePath      string `env:"DATABASE_PATH" envDefault:"debrid.db"`
    BaseDownloadsPath string `env:"BASE_DOWNLOADS_PATH" envDefault:"/downloads"`
}
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ALLDEBRID_API_KEY` | Yes | - | API key for AllDebrid service authentication |
| `SERVER_PORT` | No | `8080` | Port number for the HTTP server |
| `LOG_LEVEL` | No | `info` | Logging level (debug, info, warn, error) |
| `DATABASE_PATH` | No | `debrid.db` | Path to SQLite database file |
| `BASE_DOWNLOADS_PATH` | No | `/downloads` | Base directory for file downloads |

## Environment Variable Handling

### Loading Priority

1. **Environment variables**: Direct system environment variables take highest priority
2. **.env file**: Variables from .env file in the working directory
3. **Default values**: Built-in defaults for optional settings

### .env File Support

The package automatically loads a `.env` file if present in the working directory. The file should contain key-value pairs:

```bash
# .env file example
ALLDEBRID_API_KEY=your_api_key_here
SERVER_PORT=3000
LOG_LEVEL=debug
DATABASE_PATH=./data/debrid.db
BASE_DOWNLOADS_PATH=/home/user/downloads
```

## Configuration Validation

### Validation Rules

The `Validate()` method ensures:

1. **Required fields**: `ALLDEBRID_API_KEY` must be present and non-empty
2. **Log level**: Must be one of: `debug`, `info`, `warn`, `error` (case-insensitive)
3. **Base downloads path**: Must be an absolute path and, if it exists, must be a directory
4. **Path sanitization**: Downloads path is cleaned using `filepath.Clean()`

### Validation Examples

```go
// Valid configuration
cfg := &Config{
    AllDebridAPIKey:   "ak-1234567890abcdef",
    ServerPort:        "8080",
    LogLevel:          "info",
    DatabasePath:      "debrid.db",
    BaseDownloadsPath: "/downloads",
}

// This will pass validation
err := cfg.Validate()
// err == nil

// Invalid configuration - missing API key
cfg := &Config{
    AllDebridAPIKey:   "", // Empty - will fail
    ServerPort:        "8080",
    LogLevel:          "info",
    BaseDownloadsPath: "/downloads",
}

// This will fail validation
err := cfg.Validate()
// err: "ALLDEBRID_API_KEY is required"
```

## Error Handling

### Error Types

The package returns descriptive errors for different failure scenarios:

1. **Environment parsing errors**: Issues with reading environment variables
2. **Validation errors**: Configuration values that don't meet requirements
3. **Path validation errors**: Invalid or inaccessible download paths

### Error Examples

```go
// Missing required API key
cfg, err := config.Load()
// err: "invalid configuration: ALLDEBRID_API_KEY is required"

// Invalid log level
os.Setenv("LOG_LEVEL", "invalid")
cfg, err := config.Load()
// err: "invalid configuration: invalid log level \"invalid\", must be one of: [debug info warn error]"

// Relative path (not absolute)
os.Setenv("BASE_DOWNLOADS_PATH", "downloads")
cfg, err := config.Load()
// err: "invalid configuration: BASE_DOWNLOADS_PATH must be an absolute path, got: downloads"
```

## API Reference

### Functions

#### Load() (*Config, error)

Loads configuration from environment variables and .env file.

**Returns:**
- `*Config`: Populated configuration struct
- `error`: Error if loading or validation fails

**Example:**
```go
cfg, err := config.Load()
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

### Methods

#### (c *Config) Validate() error

Validates the configuration struct and applies path sanitization.

**Returns:**
- `error`: Validation error or nil if valid

**Side effects:**
- Modifies `BaseDownloadsPath` by cleaning the path
- Performs case-insensitive log level validation

## Testing

### Test Coverage

The package includes comprehensive tests covering:

- **Load function**: Environment variable parsing and defaults
- **Validation**: All validation rules and edge cases
- **Error handling**: Proper error messages and types

### Running Tests

```bash
# Run all tests
go test ./internal/config

# Run tests with coverage
go test -cover ./internal/config

# Run tests with verbose output
go test -v ./internal/config
```

### Test Examples

```go
func TestLoad(t *testing.T) {
    // Test with valid configuration
    os.Setenv("ALLDEBRID_API_KEY", "test-key")
    cfg, err := Load()
    require.NoError(t, err)
    require.Equal(t, "test-key", cfg.AllDebridAPIKey)
    require.Equal(t, "8080", cfg.ServerPort) // Default value
}

func TestValidate(t *testing.T) {
    // Test validation failure
    cfg := &Config{
        AllDebridAPIKey:   "", // Empty - should fail
        LogLevel:          "info",
        BaseDownloadsPath: "/tmp",
    }
    err := cfg.Validate()
    require.Error(t, err)
    require.Contains(t, err.Error(), "ALLDEBRID_API_KEY is required")
}
```

## Usage Examples

### Basic Application Setup

```go
package main

import (
    "log"
    "github.com/your-org/debrid-downloader/internal/config"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatal("Configuration error:", err)
    }

    // Use configuration
    log.Printf("Starting server on port %s", cfg.ServerPort)
    log.Printf("Downloads will be saved to: %s", cfg.BaseDownloadsPath)
    log.Printf("Database path: %s", cfg.DatabasePath)
    log.Printf("Log level: %s", cfg.LogLevel)
}
```

### Configuration with Custom .env File

```go
// For testing or custom environments
func loadTestConfig() (*config.Config, error) {
    // Set up test environment
    os.Setenv("ALLDEBRID_API_KEY", "test-key")
    os.Setenv("LOG_LEVEL", "debug")
    os.Setenv("BASE_DOWNLOADS_PATH", "/tmp/test-downloads")
    
    return config.Load()
}
```

### Validation in Application Startup

```go
func validateEnvironment() error {
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("configuration validation failed: %w", err)
    }
    
    // Additional application-specific validation
    if cfg.ServerPort == "80" || cfg.ServerPort == "443" {
        return fmt.Errorf("server port %s requires root privileges", cfg.ServerPort)
    }
    
    return nil
}
```

## Best Practices

### Environment Management

1. **Use .env files for development**: Keep secrets out of version control
2. **Validate early**: Call `config.Load()` at application startup
3. **Handle errors gracefully**: Provide clear error messages for configuration issues
4. **Use absolute paths**: Always use absolute paths for file and directory settings

### Security Considerations

1. **API key protection**: Never log or expose API keys in error messages
2. **Path validation**: The package validates download paths to prevent directory traversal
3. **Environment isolation**: Use different .env files for different environments

### Testing

1. **Clear environment**: Use `os.Clearenv()` in tests to ensure isolation
2. **Test all scenarios**: Include tests for missing, invalid, and edge-case values
3. **Validate defaults**: Ensure default values are applied correctly

## Dependencies

- `github.com/caarlos0/env/v10`: Environment variable parsing
- `github.com/joho/godotenv`: .env file loading
- `github.com/stretchr/testify/require`: Test assertions

## Architecture Notes

The configuration package follows these design principles:

1. **Single responsibility**: Only handles configuration loading and validation
2. **Fail fast**: Validates configuration at startup to catch issues early
3. **Immutable**: Configuration is loaded once and not modified during runtime
4. **Secure defaults**: Uses secure default values where appropriate

---

*Last updated: 2025-07-06*  
*Package version: v1.0.0*  
*Test coverage: 100%*