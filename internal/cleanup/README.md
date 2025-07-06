# internal/cleanup Package Documentation

## Overview

The `internal/cleanup` package provides secure file cleanup functionality for the debrid-downloader application. It automatically removes non-video files (like text files, images, subtitles, and metadata) from extracted archives while preserving video content and maintaining strict security boundaries.

This package is designed to clean up extracted media archives by removing auxiliary files that are typically not needed after extraction, helping to save storage space and organize downloaded content.

**Last Updated:** 2025-07-06  
**Package Version:** Part of debrid-downloader main  
**Test Coverage:** Comprehensive with edge cases

## Features

- **Secure File Cleanup**: Safely removes non-video files with path validation
- **Intelligent File Classification**: Distinguishes between video files, cleanup targets, and unknown files
- **Conservative Approach**: Preserves files with unknown extensions to prevent accidental deletion
- **Empty Directory Cleanup**: Removes empty directories left after file cleanup
- **Audit Trail**: Logs all cleanup operations with detailed information
- **Statistics and Preview**: Provides cleanup statistics without performing actual deletions
- **Database Integration**: Updates database records to track deleted files
- **Path Security**: Prevents directory traversal attacks with strict path validation

## Quick Start

### Basic Usage

```go
package main

import (
    "log"
    "debrid-downloader/internal/cleanup"
    "debrid-downloader/internal/database"
)

func main() {
    // Initialize database
    db, err := database.New("debrid.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create cleanup service
    service := cleanup.NewService(db, "/downloads")

    // Clean up extracted files for a download
    err = service.CleanupExtractedFiles(downloadID)
    if err != nil {
        log.Printf("Cleanup failed: %v", err)
    }
}
```

### Preview Cleanup Operations

```go
// Get statistics before cleanup
stats, err := service.GetCleanupStats(downloadID)
if err != nil {
    log.Printf("Failed to get stats: %v", err)
    return
}

log.Printf("Would delete %d files (%d bytes)", stats.CleanupFiles, stats.CleanupSize)
log.Printf("Would keep %d video files", stats.VideoFiles)
```

## Architecture

### Core Components

```
Service
├── CleanupExtractedFiles()     # Main cleanup operation
├── CleanupEmptyDirectories()   # Remove empty directories
├── GetCleanupStats()          # Preview cleanup statistics
├── isPathSafe()               # Security validation
├── shouldCleanupFile()        # File classification
└── deleteFile()               # Safe file deletion
```

### File Classification Strategy

The package uses three categories of files:

1. **Video Files** (Keep): `.mp4`, `.mkv`, `.avi`, `.mov`, `.wmv`, `.flv`, `.webm`, etc.
2. **Cleanup Files** (Delete): `.txt`, `.nfo`, `.jpg`, `.srt`, `.sub`, `.log`, `.xml`, etc.
3. **Unknown Files** (Keep): Files with extensions not in either category

### Security Model

- **Path Validation**: All file paths must be within configured base directory
- **Subdirectory Restriction**: Prevents deletion of files directly in base directory
- **Absolute Path Resolution**: Converts relative paths to absolute for security checks
- **Conservative Deletion**: Only deletes files with explicitly defined cleanup extensions

## API Reference

### Service Creation

```go
func NewService(db *database.DB, baseDownloadPath string) *Service
```

Creates a new cleanup service instance with database connection and base path for security validation.

### Main Operations

#### CleanupExtractedFiles

```go
func (s *Service) CleanupExtractedFiles(downloadID int64) error
```

Performs cleanup of extracted files for a specific download:
- Retrieves extracted files from database
- Validates file paths for security
- Classifies files based on extension
- Deletes non-video files
- Updates database records
- Logs all operations

#### CleanupEmptyDirectories

```go
func (s *Service) CleanupEmptyDirectories(downloadID int64, rootPath string) error
```

Removes empty directories after file cleanup:
- Walks directory tree recursively
- Identifies empty directories
- Removes empty directories (preserves root)
- Validates path security

#### GetCleanupStats

```go
func (s *Service) GetCleanupStats(downloadID int64) (*CleanupStats, error)
```

Returns preview statistics without performing cleanup:
- Counts files by category
- Calculates total and cleanup sizes
- Identifies unsafe files
- Provides detailed breakdown

### Data Structures

#### CleanupStats

```go
type CleanupStats struct {
    TotalFiles   int   `json:"total_files"`
    VideoFiles   int   `json:"video_files"`
    CleanupFiles int   `json:"cleanup_files"`
    UnknownFiles int   `json:"unknown_files"`
    UnsafeFiles  int   `json:"unsafe_files"`
    TotalSize    int64 `json:"total_size"`
    CleanupSize  int64 `json:"cleanup_size"`
}
```

## Configuration

### File Extension Lists

The package defines two configurable extension lists:

#### Video Extensions (Preserved)
```go
var VideoExtensions = []string{
    ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v",
    ".mpg", ".mpeg", ".3gp", ".divx", ".xvid", ".asf", ".rm", ".rmvb",
    ".ts", ".mts", ".m2ts", ".ogv", ".ogg",
}
```

#### Cleanup Extensions (Deleted)
```go
var CleanupExtensions = []string{
    ".txt", ".nfo", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".srt",
    ".sub", ".idx", ".vtt", ".ass", ".ssa", ".smi", ".rt", ".sbv",
    ".dfxp", ".ttml", ".xml", ".log", ".diz", ".sfv",
}
```

### Dependencies

- `debrid-downloader/internal/database` - Database operations
- `debrid-downloader/pkg/models` - Data models
- `log/slog` - Structured logging
- Go standard library: `os`, `path/filepath`, `strings`, `time`

## Error Handling

The package implements comprehensive error handling:

### Error Types

1. **Database Errors**: Connection failures, query errors
2. **File System Errors**: Permission denied, file not found
3. **Security Errors**: Path traversal attempts, unsafe paths
4. **Validation Errors**: Invalid download IDs, missing records

### Error Handling Strategy

```go
// Graceful handling of missing files
if _, err := os.Stat(file.Path); os.IsNotExist(err) {
    s.logger.Debug("File already deleted", "file", file.Path)
    return s.markFileDeleted(file.ID)
}

// Aggregate errors for batch operations
var errors []string
for _, file := range files {
    if err := s.deleteFile(file); err != nil {
        errors = append(errors, fmt.Sprintf("%s: %s", file.Path, err.Error()))
    }
}
```

## Testing

### Test Coverage

The package includes comprehensive tests covering:

- **Unit Tests**: All public and private methods
- **Integration Tests**: Database operations with real SQLite
- **Security Tests**: Path validation and traversal prevention
- **Edge Cases**: Missing files, permission errors, empty directories
- **Error Scenarios**: Database failures, file system errors

### Running Tests

```bash
# Run all tests
just test

# Run with coverage
just coverage

# Run specific test
go test -v ./internal/cleanup -run TestService_CleanupExtractedFiles
```

### Test Structure

```go
func TestService_CleanupExtractedFiles(t *testing.T) {
    // Setup: Create temporary files and database records
    // Execute: Run cleanup operation
    // Verify: Check file deletions and database updates
}
```

### Mock Dependencies

Tests use real SQLite in-memory databases rather than mocks to ensure integration correctness:

```go
db, err := database.New(":memory:")
require.NoError(t, err)
defer db.Close()
```

## Usage Examples

### Complete Cleanup Workflow

```go
package main

import (
    "log/slog"
    "debrid-downloader/internal/cleanup"
    "debrid-downloader/internal/database"
)

func performCleanup(downloadID int64) error {
    // Initialize service
    db, err := database.New("debrid.db")
    if err != nil {
        return err
    }
    defer db.Close()

    service := cleanup.NewService(db, "/downloads")

    // Get preview statistics
    stats, err := service.GetCleanupStats(downloadID)
    if err != nil {
        return err
    }

    slog.Info("Cleanup preview",
        "download_id", downloadID,
        "total_files", stats.TotalFiles,
        "cleanup_files", stats.CleanupFiles,
        "cleanup_size", stats.CleanupSize)

    // Perform cleanup
    err = service.CleanupExtractedFiles(downloadID)
    if err != nil {
        return err
    }

    // Clean up empty directories
    downloadDir := "/downloads/some-movie"
    err = service.CleanupEmptyDirectories(downloadID, downloadDir)
    if err != nil {
        return err
    }

    slog.Info("Cleanup completed", "download_id", downloadID)
    return nil
}
```

### Custom Extension Lists

```go
// Modify extension lists if needed
func init() {
    // Add custom video extension
    cleanup.VideoExtensions = append(cleanup.VideoExtensions, ".custom")
    
    // Add custom cleanup extension
    cleanup.CleanupExtensions = append(cleanup.CleanupExtensions, ".temp")
}
```

### Batch Cleanup Operations

```go
func cleanupMultipleDownloads(downloadIDs []int64) error {
    service := cleanup.NewService(db, "/downloads")
    
    var allErrors []string
    for _, id := range downloadIDs {
        if err := service.CleanupExtractedFiles(id); err != nil {
            allErrors = append(allErrors, fmt.Sprintf("Download %d: %v", id, err))
        }
    }
    
    if len(allErrors) > 0 {
        return fmt.Errorf("cleanup errors: %v", allErrors)
    }
    
    return nil
}
```

## Security Considerations

### Path Validation

The package implements strict path validation to prevent security vulnerabilities:

```go
// Validates file is within base directory and not the base directory itself
func (s *Service) isPathSafe(filePath string) bool {
    absFilePath, err := filepath.Abs(filePath)
    if err != nil {
        return false
    }
    
    absBasePath, err := filepath.Abs(s.baseDownloadPath)
    if err != nil {
        return false
    }
    
    // Must be within base path but not the base path itself
    return strings.HasPrefix(absFilePath, absBasePath+string(os.PathSeparator)) && 
           absFilePath != absBasePath
}
```

### Conservative Deletion Policy

- Only deletes files with explicitly defined cleanup extensions
- Preserves all video files regardless of other criteria
- Keeps unknown file types to prevent accidental deletion
- Requires files to be within configured base directory

### Audit Trail

All cleanup operations are logged with detailed information:

```go
s.logger.Info("Deleting non-video file",
    "download_id", downloadID,
    "file", file.Path,
    "size", fileSize,
    "created_at", file.CreatedAt)
```

## Performance Considerations

### Batch Operations

The cleanup process handles multiple files efficiently:
- Processes files in a single database transaction
- Aggregates errors rather than failing on first error
- Logs progress for long-running operations

### Memory Usage

- Streams file lists rather than loading all into memory
- Uses minimal file system operations
- Efficient path validation with early returns

### Concurrency

The service is designed for sequential operation but can be made concurrent with proper synchronization:

```go
// Example concurrent cleanup (requires additional synchronization)
func (s *Service) concurrentCleanup(downloadIDs []int64) error {
    var wg sync.WaitGroup
    errorsChan := make(chan error, len(downloadIDs))
    
    for _, id := range downloadIDs {
        wg.Add(1)
        go func(downloadID int64) {
            defer wg.Done()
            if err := s.CleanupExtractedFiles(downloadID); err != nil {
                errorsChan <- err
            }
        }(id)
    }
    
    wg.Wait()
    close(errorsChan)
    
    // Handle errors...
}
```

## Contributing

When contributing to the cleanup package:

1. **Add Tests**: All new functionality must include comprehensive tests
2. **Security First**: Validate all path operations and file access
3. **Conservative Approach**: When in doubt, preserve files rather than delete
4. **Logging**: Add appropriate logging for debugging and audit trails
5. **Documentation**: Update this documentation for any API changes

### Test Requirements

- Unit tests for all public methods
- Security tests for path validation
- Error handling tests for edge cases
- Integration tests with real database operations

## License

This package is part of the debrid-downloader project and follows the same license terms.