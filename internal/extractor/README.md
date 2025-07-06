# Extractor Package

## Overview

The `internal/extractor` package provides secure archive extraction functionality for ZIP and RAR files in the debrid-downloader application. It offers a clean interface for extracting compressed archives while implementing security measures to prevent directory traversal attacks and handle corrupted or password-protected files gracefully.

## Features

### Supported Archive Formats

- **ZIP Files** (`.zip`)
  - Standard ZIP archives using Go's built-in `archive/zip` package
  - Handles compressed and uncompressed files
  - Supports nested directory structures (flattened during extraction)

- **RAR Files** (`.rar`)
  - Single-volume RAR archives
  - Multi-part RAR archives (`.part01.rar`, `.part001.rar`, `.part1.rar`)
  - Automatic multi-volume detection and processing
  - Uses `github.com/nwaples/rardecode` library

### Security Features

- **Path Traversal Protection**: Validates file paths to prevent extraction outside the destination directory
- **Filename Sanitization**: Removes dangerous path components like `..` and absolute paths
- **Flattened Extraction**: Extracts all files to a single directory level, preventing directory structure attacks
- **Password Detection**: Gracefully handles password-protected archives with appropriate error messages

### Extraction Strategy

All archives are extracted using a **flattened approach**:
- Files from subdirectories are extracted directly to the destination directory
- Original directory structure is discarded for security
- File naming conflicts are handled naturally by the filesystem

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/your-org/debrid-downloader/internal/extractor"
)

func main() {
    // Create extractor service
    service := extractor.NewService()
    
    // Check if file is a supported archive
    if service.IsArchive("example.zip") {
        // Extract archive
        files, err := service.Extract("example.zip", "/path/to/destination")
        if err != nil {
            log.Fatal(err)
        }
        
        fmt.Printf("Extracted %d files:\n", len(files))
        for _, file := range files {
            fmt.Printf("  - %s\n", file)
        }
    }
}
```

## Architecture

### Interface Definition

```go
type Extractor interface {
    Extract(archivePath, destPath string) ([]string, error)
    IsArchive(filename string) bool
}
```

### Core Components

#### Service
The main service struct that implements the `Extractor` interface:

```go
type Service struct {
    logger *slog.Logger
}
```

#### Key Methods

- `NewService()`: Creates a new extractor service instance
- `Extract(archivePath, destPath string)`: Extracts an archive to the specified destination
- `IsArchive(filename string)`: Determines if a file is a supported archive format

### Internal Implementation

The package uses format-specific extraction methods:

- `extractZip()`: Handles ZIP file extraction using Go's standard library
- `extractRar()`: Handles RAR file extraction using the rardecode library
- `extractZipFile()`: Extracts individual files from ZIP archives
- `extractRarFile()`: Extracts individual files from RAR archives

## API Reference

### Core Interface

#### `Extract(archivePath, destPath string) ([]string, error)`

Extracts an archive file to the specified destination directory.

**Parameters:**
- `archivePath`: Path to the archive file to extract
- `destPath`: Destination directory for extracted files

**Returns:**
- `[]string`: List of extracted file paths
- `error`: Extraction error, if any

**Error Conditions:**
- Archive file not found or unreadable
- Destination directory creation fails
- Unsupported archive format
- Corrupted archive data
- Password-protected archives (RAR only)

#### `IsArchive(filename string) bool`

Determines if a file is a supported archive format based on its filename.

**Parameters:**
- `filename`: Name of the file to check

**Returns:**
- `bool`: True if the file is a supported archive format

**Supported Patterns:**
- `.zip` files (case-insensitive)
- `.rar` files (case-insensitive)
- Multi-part RAR files (first part only): `.part1.rar`, `.part01.rar`, `.part001.rar`

### Archive Format Support

#### ZIP Files
- **Library**: Go standard library `archive/zip`
- **Features**: Full ZIP specification support
- **Limitations**: None significant for typical use cases

#### RAR Files
- **Library**: `github.com/nwaples/rardecode`
- **Features**: 
  - RAR 2.x and 3.x support
  - Multi-volume archives
  - Automatic volume detection
- **Limitations**: 
  - Password-protected archives are detected and skipped
  - RAR 5.x may have limited support

### Multi-Part Archive Handling

The extractor intelligently handles multi-part RAR archives:

1. **Detection**: Identifies multi-part archives by filename patterns
2. **Volume Discovery**: Automatically locates related archive parts
3. **Extraction**: Uses the first part as entry point for the entire archive
4. **Logging**: Provides detailed logs about volume usage

## Error Handling

The package implements comprehensive error handling:

### Error Types

1. **File System Errors**
   - Archive file not found
   - Destination directory creation failures
   - Permission issues

2. **Archive Format Errors**
   - Unsupported archive types
   - Corrupted archive data
   - Invalid archive structure

3. **Security Errors**
   - Path traversal attempts
   - Dangerous filename patterns

4. **Content Errors**
   - Password-protected archives
   - Extraction failures for individual files

### Error Handling Strategy

- **Graceful Degradation**: Individual file extraction failures don't stop the entire process
- **Detailed Logging**: Comprehensive logging for debugging and monitoring
- **Security First**: Dangerous operations are blocked with clear error messages
- **User-Friendly Messages**: Error messages are descriptive and actionable

## Testing

### Test Coverage

The package includes comprehensive tests covering:

- **Unit Tests**: Individual method functionality
- **Integration Tests**: End-to-end extraction scenarios
- **Security Tests**: Path traversal and filename sanitization
- **Error Handling Tests**: Various failure scenarios
- **Edge Cases**: Empty archives, corrupted files, permission issues

### Test Structure

```
extractor_test.go           # Main test file with comprehensive coverage
mock.go                     # Mock generation directive
mocks/mock_extractor.go     # Generated mock implementation
```

### Key Test Scenarios

1. **Format Detection Tests**
   - Various archive formats and extensions
   - Case sensitivity handling
   - Multi-part archive detection

2. **Extraction Tests**
   - Valid ZIP and RAR files
   - Flattened extraction verification
   - File content integrity

3. **Security Tests**
   - Path traversal prevention
   - Dangerous filename handling
   - Destination directory validation

4. **Error Handling Tests**
   - Corrupted archives
   - Missing files
   - Permission errors
   - Password-protected archives

### Running Tests

```bash
# Run all tests
go test ./internal/extractor

# Run with coverage
go test -cover ./internal/extractor

# Run with race detection
go test -race ./internal/extractor
```

## Mock Implementation

The package provides mock implementations for testing:

### Generated Mock

```go
// MockExtractor provides a mock implementation of the Extractor interface
type MockExtractor struct {
    ctrl     *gomock.Controller
    recorder *MockExtractorMockRecorder
}
```

### Usage Example

```go
func TestWithMockExtractor(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockExtractor := mocks.NewMockExtractor(ctrl)
    mockExtractor.EXPECT().
        IsArchive("test.zip").
        Return(true)
    
    mockExtractor.EXPECT().
        Extract("test.zip", "/dest").
        Return([]string{"/dest/file1.txt", "/dest/file2.txt"}, nil)
    
    // Use mock in tests
    result, err := mockExtractor.Extract("test.zip", "/dest")
    assert.NoError(t, err)
    assert.Len(t, result, 2)
}
```

## Performance Considerations

### Memory Usage

- **Streaming Extraction**: Files are extracted one at a time to minimize memory usage
- **No Full Load**: Archives are not loaded entirely into memory
- **Efficient I/O**: Uses `io.Copy` for optimal data transfer

### I/O Optimization

- **Directory Creation**: Destination directories are created only once
- **File Permissions**: Original file permissions are preserved when possible
- **Concurrent Safety**: Service instances are safe for concurrent use

### Large Archive Handling

- **Streaming Processing**: Handles large archives efficiently
- **Progress Logging**: Provides progress information for monitoring
- **Error Recovery**: Individual file failures don't affect other files

## Integration Patterns

### Dependency Injection

```go
type DownloadService struct {
    extractor extractor.Extractor
}

func NewDownloadService(ext extractor.Extractor) *DownloadService {
    return &DownloadService{
        extractor: ext,
    }
}
```

### Error Handling Integration

```go
files, err := service.Extract(archivePath, destPath)
if err != nil {
    if strings.Contains(err.Error(), "password-protected") {
        // Handle password-protected archives
        logger.Warn("Archive is password-protected, skipping", "file", archivePath)
        return nil
    }
    return fmt.Errorf("extraction failed: %w", err)
}
```

### Logging Integration

The extractor provides structured logging using Go's `slog` package:

```go
service.logger.Info("Extracting archive", 
    "archive", archivePath, 
    "dest", destPath,
    "type", "zip")
```

## Configuration

### Environment Variables

The extractor service doesn't require specific configuration but integrates with the application's logging configuration:

- `LOG_LEVEL`: Controls logging verbosity (debug, info, warn, error)

### Customization Options

While the current implementation uses default settings, the architecture allows for future extensibility:

- Custom logger injection
- Configurable extraction strategies
- Plugin-based format support

## Security Considerations

### Path Traversal Prevention

The extractor implements multiple layers of protection:

1. **Filename Validation**: Checks for dangerous patterns (`..`, absolute paths)
2. **Path Sanitization**: Uses `filepath.Base()` to extract only the filename
3. **Destination Validation**: Ensures all extracted files are within the destination directory

### Safe Defaults

- **Flattened Extraction**: Eliminates directory traversal possibilities
- **Permission Preservation**: Maintains original file permissions when safe
- **Error Logging**: Security events are logged for monitoring

### Best Practices

1. **Validate Input**: Always check archives with `IsArchive()` before extraction
2. **Secure Destinations**: Use dedicated extraction directories with appropriate permissions
3. **Monitor Logs**: Watch for security-related warnings in the logs
4. **Regular Updates**: Keep dependencies updated for security patches

## Contributing

### Code Style

- Follow Go best practices and idioms
- Use structured logging with `slog`
- Include comprehensive test coverage
- Document public APIs with clear examples

### Testing Requirements

- All new features must include tests
- Security-related code requires specific security tests
- Performance-critical code should include benchmarks
- Mock implementations must be updated for interface changes

### Dependencies

- Minimize external dependencies
- Use well-maintained libraries with good security track records
- Keep dependencies updated for security and compatibility

## License

This package is part of the debrid-downloader project and follows the same license terms as the main project.