# Package models

## Overview

The `pkg/models` package defines the core data structures used throughout the debrid-downloader application. It provides type-safe models for download management, directory mapping intelligence, and archive extraction tracking with comprehensive JSON serialization and database mapping support.

**Key Features:**
- Comprehensive download lifecycle management with status tracking
- Machine learning-like directory suggestion system
- Multi-file download grouping and processing
- Archive extraction file tracking
- Full JSON serialization/deserialization
- Database-optimized field mapping with proper indexing

## Data Models

### Download Model

The `Download` struct represents a complete file download record with detailed progress tracking, status management, and metadata storage.

```go
type Download struct {
    ID              int64          `json:"id" db:"id"`
    OriginalURL     string         `json:"original_url" db:"original_url"`
    UnrestrictedURL string         `json:"unrestricted_url" db:"unrestricted_url"`
    Filename        string         `json:"filename" db:"filename"`
    Directory       string         `json:"directory" db:"directory"`
    Status          DownloadStatus `json:"status" db:"status"`
    Progress        float64        `json:"progress" db:"progress"`
    FileSize        int64          `json:"file_size" db:"file_size"`
    DownloadedBytes int64          `json:"downloaded_bytes" db:"downloaded_bytes"`
    DownloadSpeed   float64        `json:"download_speed" db:"download_speed"`
    ErrorMessage    string         `json:"error_message" db:"error_message"`
    RetryCount      int            `json:"retry_count" db:"retry_count"`
    CreatedAt       time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
    StartedAt       *time.Time     `json:"started_at" db:"started_at"`
    CompletedAt     *time.Time     `json:"completed_at" db:"completed_at"`
    PausedAt        *time.Time     `json:"paused_at" db:"paused_at"`
    TotalPausedTime int64          `json:"total_paused_time" db:"total_paused_time"`
    GroupID         string         `json:"group_id" db:"group_id"`
    IsArchive       bool           `json:"is_archive" db:"is_archive"`
    ExtractedFiles  string         `json:"extracted_files" db:"extracted_files"`
}
```

**Field Descriptions:**
- `ID`: Auto-incrementing primary key
- `OriginalURL`: The original URL submitted by the user
- `UnrestrictedURL`: The unrestricted URL from the debrid service
- `Filename`: The target filename for the download
- `Directory`: Target directory path for the download
- `Status`: Current download status (see Status Lifecycle)
- `Progress`: Download progress percentage (0.0-100.0)
- `FileSize`: Total file size in bytes
- `DownloadedBytes`: Bytes downloaded so far
- `DownloadSpeed`: Current download speed in bytes/second
- `ErrorMessage`: Error message if download failed
- `RetryCount`: Number of retry attempts
- `CreatedAt`: Record creation timestamp
- `UpdatedAt`: Last update timestamp
- `StartedAt`: Download start timestamp (nullable)
- `CompletedAt`: Download completion timestamp (nullable)
- `PausedAt`: Download pause timestamp (nullable)
- `TotalPausedTime`: Total time spent paused (seconds)
- `GroupID`: Group identifier for multi-file downloads
- `IsArchive`: Whether the file is an archive requiring extraction
- `ExtractedFiles`: JSON array of extracted file paths

### DirectoryMapping Model

The `DirectoryMapping` struct implements a machine learning-like system that learns from user behavior to suggest appropriate directories for downloads.

```go
type DirectoryMapping struct {
    ID              int64     `json:"id" db:"id"`
    FilenamePattern string    `json:"filename_pattern" db:"filename_pattern"`
    OriginalURL     string    `json:"original_url" db:"original_url"`
    Directory       string    `json:"directory" db:"directory"`
    UseCount        int       `json:"use_count" db:"use_count"`
    LastUsed        time.Time `json:"last_used" db:"last_used"`
    CreatedAt       time.Time `json:"created_at" db:"created_at"`
}
```

**Field Descriptions:**
- `ID`: Auto-incrementing primary key
- `FilenamePattern`: Pattern extracted from filename for matching
- `OriginalURL`: The original URL that created this mapping
- `Directory`: The directory path chosen by the user
- `UseCount`: Number of times this mapping has been used
- `LastUsed`: Last time this mapping was used
- `CreatedAt`: Mapping creation timestamp

### DownloadGroup Model

The `DownloadGroup` struct manages groups of related downloads, typically for multi-file archives or batch downloads.

```go
type DownloadGroup struct {
    ID                 string              `json:"id" db:"id"`
    CreatedAt          time.Time           `json:"created_at" db:"created_at"`
    TotalDownloads     int                 `json:"total_downloads" db:"total_downloads"`
    CompletedDownloads int                 `json:"completed_downloads" db:"completed_downloads"`
    Status             DownloadGroupStatus `json:"status" db:"status"`
    ProcessingError    string              `json:"processing_error" db:"processing_error"`
}
```

### ExtractedFile Model

The `ExtractedFile` struct tracks files extracted from archive downloads, supporting soft deletion.

```go
type ExtractedFile struct {
    ID         int64      `json:"id" db:"id"`
    DownloadID int64      `json:"download_id" db:"download_id"`
    FilePath   string     `json:"file_path" db:"file_path"`
    CreatedAt  time.Time  `json:"created_at" db:"created_at"`
    DeletedAt  *time.Time `json:"deleted_at" db:"deleted_at"`
}
```

## Status Lifecycle

### Download Status

The `DownloadStatus` type defines the possible states of a download:

```go
type DownloadStatus string

const (
    StatusPending     DownloadStatus = "pending"
    StatusDownloading DownloadStatus = "downloading"
    StatusCompleted   DownloadStatus = "completed"
    StatusFailed      DownloadStatus = "failed"
    StatusPaused      DownloadStatus = "paused"
)
```

**State Transitions:**
```
pending → downloading → completed
pending → downloading → failed
pending → downloading → paused → downloading
pending → failed (if URL unrestricting fails)
```

### Download Group Status

The `DownloadGroupStatus` type defines the possible states of a download group:

```go
type DownloadGroupStatus string

const (
    GroupStatusDownloading DownloadGroupStatus = "downloading"
    GroupStatusProcessing  DownloadGroupStatus = "processing"
    GroupStatusCompleted   DownloadGroupStatus = "completed"
    GroupStatusFailed      DownloadGroupStatus = "failed"
)
```

**State Transitions:**
```
downloading → processing → completed
downloading → processing → failed
downloading → failed (if individual downloads fail)
```

## Database Mapping

The models are designed for optimal database performance with proper indexing:

### Downloads Table
- Primary key: `id` (auto-increment)
- Indexes: `status`, `created_at`, `group_id`
- Foreign key: `group_id` references `download_groups(id)`

### Directory Mappings Table
- Primary key: `id` (auto-increment)
- Indexes: `filename_pattern`, `use_count DESC`
- Optimized for pattern matching and popularity sorting

### Download Groups Table
- Primary key: `id` (string UUID)
- Referenced by: `downloads.group_id`

### Extracted Files Table
- Primary key: `id` (auto-increment)
- Foreign key: `download_id` references `downloads(id)`
- Indexes: `download_id`, `deleted_at`
- Supports soft deletion with `deleted_at` timestamp

## JSON Serialization

All models support full JSON serialization/deserialization with proper field mapping:

```go
// Example JSON output for a Download
{
    "id": 1,
    "original_url": "https://example.com/file.zip",
    "unrestricted_url": "https://debrid.com/file.zip",
    "filename": "file.zip",
    "directory": "/downloads",
    "status": "completed",
    "progress": 100.0,
    "file_size": 1024000,
    "downloaded_bytes": 1024000,
    "download_speed": 1500.5,
    "error_message": "",
    "retry_count": 0,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:35:00Z",
    "started_at": "2024-01-15T10:30:15Z",
    "completed_at": "2024-01-15T10:35:00Z",
    "paused_at": null,
    "total_paused_time": 0,
    "group_id": "",
    "is_archive": false,
    "extracted_files": "[]"
}
```

## Usage Examples

### Creating a New Download

```go
download := &models.Download{
    OriginalURL:     "https://example.com/file.zip",
    UnrestrictedURL: "https://debrid.com/file.zip",
    Filename:        "file.zip",
    Directory:       "/downloads",
    Status:          models.StatusPending,
    Progress:        0.0,
    FileSize:        1024000,
    DownloadedBytes: 0,
    DownloadSpeed:   0.0,
    ErrorMessage:    "",
    RetryCount:      0,
    CreatedAt:       time.Now(),
    UpdatedAt:       time.Now(),
    IsArchive:       false,
    ExtractedFiles:  "[]",
}
```

### Updating Download Progress

```go
download.Status = models.StatusDownloading
download.Progress = 45.5
download.DownloadedBytes = 465920
download.DownloadSpeed = 1500.0
download.UpdatedAt = time.Now()
if download.StartedAt == nil {
    now := time.Now()
    download.StartedAt = &now
}
```

### Creating a Directory Mapping

```go
mapping := &models.DirectoryMapping{
    FilenamePattern: "*.zip",
    OriginalURL:     "https://example.com/archive.zip",
    Directory:       "/downloads/archives",
    UseCount:        1,
    LastUsed:        time.Now(),
    CreatedAt:       time.Now(),
}
```

### Managing Download Groups

```go
group := &models.DownloadGroup{
    ID:                 "group-" + uuid.New().String(),
    CreatedAt:          time.Now(),
    TotalDownloads:     3,
    CompletedDownloads: 0,
    Status:             models.GroupStatusDownloading,
    ProcessingError:    "",
}
```

## Testing Approach

The models package uses comprehensive testing strategies:

### Structure Validation
- Tests ensure all struct fields are properly initialized
- Validates zero-value behavior
- Confirms proper field types and constraints

### JSON Serialization Testing
- Round-trip serialization/deserialization testing
- Ensures all fields are preserved correctly
- Tests nullable fields and edge cases

### Constants Testing
- Validates all status constants have expected string values
- Ensures type safety with strongly-typed enums
- Tests string conversion behavior

### Coverage Areas
- Struct field initialization and access
- JSON marshaling/unmarshaling
- Status constant validation
- Zero value behavior
- Type safety and constraints

## Model Relationships

### Download → DirectoryMapping
- Downloads can create new directory mappings based on user choices
- Mappings are used to suggest directories for future downloads

### Download → DownloadGroup
- Downloads can belong to a group via `group_id`
- Groups track collective progress and status

### Download → ExtractedFile
- Archives can have multiple extracted files
- Extracted files maintain reference to parent download

### Validation Rules

1. **Download Model:**
   - `OriginalURL` must be a valid URL
   - `Filename` cannot be empty
   - `Directory` must be a valid path
   - `Progress` must be between 0.0 and 100.0
   - `FileSize` and `DownloadedBytes` must be non-negative
   - `Status` must be one of the defined constants

2. **DirectoryMapping Model:**
   - `FilenamePattern` cannot be empty
   - `Directory` must be a valid path
   - `UseCount` must be positive
   - `LastUsed` must not be in the future

3. **DownloadGroup Model:**
   - `ID` must be unique
   - `TotalDownloads` must be positive
   - `CompletedDownloads` must not exceed `TotalDownloads`

## Best Practices

1. **Status Management:**
   - Always update `UpdatedAt` when changing status
   - Set appropriate timestamp fields (`StartedAt`, `CompletedAt`, `PausedAt`)
   - Use atomic operations for status transitions

2. **Progress Tracking:**
   - Update `Progress` and `DownloadedBytes` together
   - Calculate `DownloadSpeed` based on recent measurements
   - Reset speed to 0 when paused or completed

3. **Error Handling:**
   - Store detailed error messages in `ErrorMessage`
   - Increment `RetryCount` on failure
   - Consider exponential backoff for retries

4. **Directory Mapping:**
   - Update `UseCount` and `LastUsed` when mappings are used
   - Clean up unused mappings periodically
   - Use fuzzy matching for pattern suggestions

5. **Group Management:**
   - Update group status based on individual download statuses
   - Handle partial failures gracefully
   - Maintain accurate progress counters

## Dependencies

The models package has minimal dependencies:
- `time` - Standard library for timestamp handling
- `json` - Implicit for JSON serialization via struct tags
- `database/sql` - Implicit for database mapping via struct tags

## Performance Considerations

1. **Database Indexes:**
   - Status-based queries are optimized with `idx_downloads_status`
   - Time-based queries use `idx_downloads_created_at`
   - Group operations use `idx_downloads_group_id`

2. **Memory Usage:**
   - Models are designed to be lightweight
   - Nullable timestamps use pointers to save memory
   - JSON strings are used for complex data (extracted files)

3. **Concurrent Access:**
   - Models are safe for concurrent read access
   - Write operations should be synchronized at the application level
   - Database operations handle concurrent access with proper locking

---

**Last Updated:** 2024-01-15  
**Version:** 1.0.0  
**Package Path:** `/root/repos/debrid-downloader/pkg/models/`