# Internal Downloader Package Documentation

## Overview

The `internal/downloader` package provides a comprehensive download queue and worker system for the debrid-downloader application. It implements a robust download manager with support for parallel downloads, progress tracking, resume capability, automatic retry logic, and post-download archive processing.

**Key Features:**
- Sequential download processing with queue management
- Real-time progress tracking with wget-style speed calculation
- Resume capability for interrupted downloads
- Exponential backoff retry mechanism
- Archive extraction and cleanup
- Download group management and processing
- Comprehensive error handling and recovery

## Architecture

### Core Components

#### 1. Worker (`worker.go`)
The main download worker that processes downloads sequentially from a queue.

**Key Responsibilities:**
- Download queue management
- File downloading with progress tracking
- Resume interrupted downloads
- Retry failed downloads with exponential backoff
- Archive extraction and cleanup
- Download group coordination

#### 2. Interfaces (`interfaces.go`)
Defines clean abstractions for external dependencies to enable testing and modularity.

**Interfaces:**
- `DatabaseInterface`: Database operations for downloads and groups
- `CleanupInterface`: File and directory cleanup operations
- `ExtractorInterface`: Archive extraction operations

#### 3. Speed History (`worker.go`)
Implements wget-style download speed calculation using a ring buffer for smoothed speed reporting.

**Features:**
- Ring buffer with configurable sample size (default: 20 samples)
- Minimum sample duration filtering (150ms)
- Smoothed speed calculation over multiple samples
- Stall detection capabilities

### Data Flow

```
Download Request → Queue → Worker → Download → Progress Updates → Completion → Archive Processing
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "debrid-downloader/internal/database"
    "debrid-downloader/internal/downloader"
)

func main() {
    // Initialize database
    db, err := database.New("downloads.db")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Create worker
    worker := downloader.NewWorker(db, "/downloads")

    // Start worker in background
    ctx := context.Background()
    go worker.Start(ctx)

    // Queue a download
    downloadID := int64(123)
    worker.QueueDownload(downloadID)
}
```

### Worker Lifecycle Management

```go
// Create worker with base download path
worker := downloader.NewWorker(db, "/downloads")

// Start processing with context for graceful shutdown
ctx, cancel := context.WithCancel(context.Background())
go worker.Start(ctx)

// Queue downloads
worker.QueueDownload(downloadID)

// Pause current download
err := worker.PauseCurrentDownload()

// Resume paused download
err = worker.ResumeDownload(downloadID)

// Get current download status
current := worker.GetCurrentDownload()

// Graceful shutdown
cancel()
```

## Features

### Download State Management

The package manages downloads through a comprehensive state lifecycle:

```
pending → downloading → completed/failed/paused
```

**State Transitions:**
- `pending`: Download queued for processing
- `downloading`: Currently being downloaded
- `completed`: Successfully downloaded
- `failed`: Download failed after all retries
- `paused`: Download paused by user

### Progress Tracking

Real-time progress tracking with multiple metrics:

```go
type ProgressMetrics struct {
    DownloadedBytes int64   // Bytes downloaded
    TotalBytes      int64   // Total file size
    Progress        float64 // Percentage complete
    DownloadSpeed   float64 // Current speed (bytes/sec)
    EstimatedTime   time.Duration // ETA
}
```

### Speed Calculation

Implements wget-style speed smoothing using a ring buffer:

```go
// Constants for speed calculation
const (
    SPEED_HISTORY_SIZE  = 20   // Number of samples in ring buffer
    SAMPLE_MIN_DURATION = 0.15 // Minimum 150ms between samples
    STALL_THRESHOLD     = 5.0  // Seconds before considering stalled
)

// Usage
speedHistory := NewSpeedHistory()
speedHistory.AddSample(bytesRead, duration)
currentSpeed := speedHistory.CalculateSpeed(recentBytes, recentTime)
```

### Retry Logic

Exponential backoff retry mechanism with configurable attempts:

```go
// Retry configuration
maxRetries := 5
for attempt := 0; attempt <= maxRetries; attempt++ {
    // Exponential backoff: 2^attempt seconds
    backoffDuration := time.Duration(1<<uint(attempt)) * time.Second
    
    // Retry download
    err := downloadFile(ctx, download)
    if err == nil {
        return // Success
    }
    
    // Wait before next attempt
    time.Sleep(backoffDuration)
}
```

### Resume Capability

Supports HTTP range requests for resuming interrupted downloads:

```go
// Check for partial download
if stat, err := os.Stat(tempPath); err == nil {
    resumeFrom := stat.Size()
    if resumeFrom > 0 {
        req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
    }
}
```

### Archive Processing

Automatic extraction and cleanup of downloaded archives:

**Supported Archive Types:**
- ZIP files
- RAR files (including multi-part)
- 7z files
- TAR files (with compression)

**Processing Features:**
- Multi-part RAR handling
- Extraction to same directory
- Original archive deletion after extraction
- Non-video file cleanup
- Empty directory cleanup

## API Reference

### Worker

#### Constructor

```go
func NewWorker(db *database.DB, baseDownloadPath string) *Worker
```

Creates a new download worker with the specified database and base download path.

#### Methods

```go
// Start processing downloads
func (w *Worker) Start(ctx context.Context)

// Queue a download for processing
func (w *Worker) QueueDownload(downloadID int64)

// Get current download information
func (w *Worker) GetCurrentDownload() *models.Download

// Pause current download
func (w *Worker) PauseCurrentDownload() error

// Resume a paused download
func (w *Worker) ResumeDownload(downloadID int64) error
```

### SpeedHistory

#### Constructor

```go
func NewSpeedHistory() *SpeedHistory
```

Creates a new speed history tracker with default configuration.

#### Methods

```go
// Add a speed sample
func (sh *SpeedHistory) AddSample(bytes int64, duration float64)

// Calculate current smoothed speed
func (sh *SpeedHistory) CalculateSpeed(recentBytes int64, recentTime float64) float64
```

### Interfaces

#### DatabaseInterface

```go
type DatabaseInterface interface {
    // Download operations
    GetDownload(id int64) (*models.Download, error)
    UpdateDownload(download *models.Download) error
    
    // Download group operations
    GetDownloadGroup(id string) (*models.DownloadGroup, error)
    GetDownloadsByGroupID(groupID string) ([]*models.Download, error)
    UpdateDownloadGroup(group *models.DownloadGroup) error
    
    // Extracted file operations
    CreateExtractedFile(file *models.ExtractedFile) error
}
```

#### CleanupInterface

```go
type CleanupInterface interface {
    CleanupExtractedFiles(downloadID int64) error
    CleanupEmptyDirectories(downloadID int64, directory string) error
}
```

#### ExtractorInterface

```go
type ExtractorInterface interface {
    Extract(archivePath, destPath string) ([]string, error)
    IsArchive(filename string) bool
}
```

## Error Handling

### Common Error Scenarios

1. **Network Errors**: Automatic retry with exponential backoff
2. **File System Errors**: Graceful handling with cleanup
3. **Database Errors**: Logged warnings, operations continue
4. **Archive Errors**: Logged warnings, extraction continues for other files
5. **Context Cancellation**: Graceful shutdown and cleanup

### Error Recovery

```go
// Network error recovery
for attempt := 0; attempt <= maxRetries; attempt++ {
    err := downloadFile(ctx, download)
    if err == nil {
        return // Success
    }
    
    // Log and retry with backoff
    log.Warn("Download failed, retrying", "attempt", attempt, "error", err)
    time.Sleep(backoffDuration)
}
```

## Testing

### Mock Implementations

The package includes comprehensive mocks for all interfaces:

```go
// Generated mocks in mocks/mock_interfaces.go
type MockDatabaseInterface struct { ... }
type MockCleanupInterface struct { ... }
type MockExtractorInterface struct { ... }
```

### Test Coverage

The package includes extensive tests covering:

- **Unit Tests**: Individual component functionality
- **Integration Tests**: End-to-end download scenarios
- **Error Handling**: Various failure scenarios
- **Edge Cases**: Network timeouts, file system errors, cancellation
- **Performance Tests**: Speed calculation accuracy

### Running Tests

```bash
# Run all tests
go test ./internal/downloader

# Run with coverage
go test -cover ./internal/downloader

# Run with race detection
go test -race ./internal/downloader

# Run specific test
go test -run TestWorker_ProcessDownload ./internal/downloader
```

## Configuration

### Environment Variables

- `BASE_DOWNLOADS_PATH`: Base directory for downloads (default: `/downloads`)
- `MAX_RETRIES`: Maximum retry attempts (default: 5)
- `QUEUE_SIZE`: Download queue buffer size (default: 100)

### Tuning Parameters

```go
// Speed calculation constants
const (
    SPEED_HISTORY_SIZE  = 20   // Ring buffer size
    SAMPLE_MIN_DURATION = 0.15 // Minimum sample duration
    STALL_THRESHOLD     = 5.0  // Stall detection threshold
)

// Download configuration
const (
    PROGRESS_UPDATE_INTERVAL = 500 * time.Millisecond
    HTTP_TIMEOUT            = 1 * time.Hour
    BUFFER_SIZE             = 32 * 1024 // 32KB
)
```

## Concurrency and Thread Safety

### Thread Safety

- **Worker State**: Protected by RWMutex for concurrent access
- **Queue Operations**: Channel-based thread-safe operations
- **Database Updates**: Atomic operations with proper error handling

### Concurrency Model

```go
// Single worker processes downloads sequentially
// Multiple workers can be created for parallel processing
worker1 := NewWorker(db, "/downloads/worker1")
worker2 := NewWorker(db, "/downloads/worker2")

go worker1.Start(ctx)
go worker2.Start(ctx)
```

## Integration Examples

### With Web Handler

```go
func downloadHandler(w http.ResponseWriter, r *http.Request) {
    // Create download record
    download := &models.Download{
        OriginalURL:     originalURL,
        UnrestrictedURL: unrestrictedURL,
        Filename:        filename,
        Directory:       downloadDir,
        Status:          models.StatusPending,
    }
    
    // Save to database
    err := db.CreateDownload(download)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Queue for processing
    worker.QueueDownload(download.ID)
    
    // Return success response
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Download queued",
        "id":      download.ID,
    })
}
```

### With Progress Monitoring

```go
func monitorProgress(worker *downloader.Worker) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            current := worker.GetCurrentDownload()
            if current != nil {
                log.Info("Download progress",
                    "id", current.ID,
                    "filename", current.Filename,
                    "progress", current.Progress,
                    "speed", current.DownloadSpeed)
            }
        }
    }
}
```

## Performance Considerations

### Memory Usage

- **Ring Buffer**: Fixed size (20 samples × 16 bytes = 320 bytes)
- **Download Buffer**: 32KB per active download
- **Queue**: Configurable buffer size (default: 100 download IDs)

### Disk I/O

- **Temporary Files**: Downloads use `.tmp` extension during transfer
- **Atomic Rename**: Final file move is atomic operation
- **Progress Updates**: Limited to 500ms intervals to reduce I/O

### Network Optimization

- **Connection Reuse**: HTTP client with connection pooling
- **Resume Support**: Range requests for interrupted downloads
- **Timeout Configuration**: 1-hour timeout for large files

## Troubleshooting

### Common Issues

1. **Queue Full**: Increase queue buffer size or process downloads faster
2. **Disk Space**: Monitor available space in download directory
3. **Network Timeouts**: Adjust HTTP timeout configuration
4. **Permission Errors**: Ensure write permissions on download directory

### Debug Information

```go
// Enable debug logging
log.SetLevel(log.DebugLevel)

// Monitor queue status
log.Debug("Queue status", "size", len(worker.queue), "capacity", cap(worker.queue))

// Track download metrics
log.Debug("Download metrics",
    "downloaded", download.DownloadedBytes,
    "total", download.FileSize,
    "speed", download.DownloadSpeed,
    "progress", download.Progress)
```

## Contributing

### Code Style

- Follow Go conventions and `gofmt` formatting
- Add comprehensive tests for new features
- Update documentation for API changes
- Use meaningful commit messages

### Testing Requirements

- All new code must have tests
- Maintain >90% test coverage
- Include both unit and integration tests
- Test error scenarios and edge cases

---

*Last Updated: 2025-01-06*
*Package Version: v1.0.0*
*Go Version: 1.24.4*