# Database Package Documentation

## Overview

The `internal/database` package provides SQLite database operations for the debrid-downloader application. It handles all database interactions including downloads tracking, directory mappings for intelligent suggestions, download groups, and extracted file management.

**Key Features:**
- SQLite database with optimized connection settings
- Automatic schema initialization and migrations
- Comprehensive CRUD operations for downloads and metadata
- Intelligent directory mapping with usage tracking
- Search functionality with fuzzy matching
- Download grouping and archive extraction tracking
- Connection pooling and transaction management
- Cleanup operations for old downloads

## Table of Contents

1. [Database Schema](#database-schema)
2. [Core Types](#core-types)
3. [Database Operations](#database-operations)
4. [Connection Management](#connection-management)
5. [Error Handling](#error-handling)
6. [Testing](#testing)
7. [Usage Examples](#usage-examples)
8. [Best Practices](#best-practices)

## Database Schema

The database consists of four main tables:

### downloads
Tracks individual file downloads with comprehensive metadata:

```sql
CREATE TABLE downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_url TEXT NOT NULL,
    unrestricted_url TEXT,
    filename TEXT NOT NULL,
    directory TEXT NOT NULL,
    status TEXT NOT NULL,
    progress REAL DEFAULT 0.0,
    file_size INTEGER DEFAULT 0,
    downloaded_bytes INTEGER DEFAULT 0,
    download_speed REAL DEFAULT 0.0,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    started_at DATETIME,
    completed_at DATETIME,
    paused_at DATETIME,
    total_paused_time INTEGER DEFAULT 0,
    group_id TEXT,
    is_archive BOOLEAN DEFAULT FALSE,
    extracted_files TEXT
);
```

**Indexes:**
- `idx_downloads_status` on `status`
- `idx_downloads_created_at` on `created_at`
- `idx_downloads_group_id` on `group_id`

### directory_mappings
Machine learning-like system for intelligent directory suggestions:

```sql
CREATE TABLE directory_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename_pattern TEXT NOT NULL,
    original_url TEXT,
    directory TEXT NOT NULL,
    use_count INTEGER DEFAULT 1,
    last_used DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);
```

**Indexes:**
- `idx_directory_mappings_pattern` on `filename_pattern`
- `idx_directory_mappings_use_count` on `use_count DESC`

### download_groups
Manages related downloads as a group:

```sql
CREATE TABLE download_groups (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL,
    total_downloads INTEGER NOT NULL,
    completed_downloads INTEGER DEFAULT 0,
    status TEXT NOT NULL,
    processing_error TEXT
);
```

### extracted_files
Tracks files extracted from archives:

```sql
CREATE TABLE extracted_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    download_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    deleted_at DATETIME,
    FOREIGN KEY (download_id) REFERENCES downloads(id)
);
```

**Indexes:**
- `idx_extracted_files_download_id` on `download_id`
- `idx_extracted_files_deleted_at` on `deleted_at`

## Core Types

### DB
The main database wrapper struct:

```go
type DB struct {
    conn *sql.DB
}
```

**Methods:**
- `New(dbPath string) (*DB, error)` - Creates new database connection
- `Close() error` - Closes database connection
- `initSchema() error` - Initializes database schema

## Database Operations

### Download Operations

#### CreateDownload
Creates a new download record:

```go
func (db *DB) CreateDownload(download *models.Download) error
```

**Parameters:**
- `download`: Download struct with all fields populated
- Automatically sets the ID field after insertion

#### GetDownload
Retrieves a single download by ID:

```go
func (db *DB) GetDownload(id int64) (*models.Download, error)
```

**Returns:**
- Download struct or error if not found

#### UpdateDownload
Updates an existing download record:

```go
func (db *DB) UpdateDownload(download *models.Download) error
```

**Note:** Updates all fields except ID, original_url, filename, directory, and created_at

#### ListDownloads
Retrieves downloads with pagination:

```go
func (db *DB) ListDownloads(limit, offset int) ([]*models.Download, error)
```

**Parameters:**
- `limit`: Maximum number of records to return
- `offset`: Number of records to skip

**Returns:** Downloads ordered by `created_at DESC, id ASC`

#### SearchDownloads
Advanced search with fuzzy matching:

```go
func (db *DB) SearchDownloads(searchTerm, statusFilter string, limit, offset int) ([]*models.Download, error)
```

**Features:**
- Fuzzy matching across filename, original_url, and directory
- Multiple search patterns for typo tolerance
- Status filtering
- Pagination support

**Search Patterns:**
- Exact word matching
- Partial word matching (for words ≥3 chars)
- Character substitution fuzzy matching (for words ≥4 chars)

#### DeleteDownload
Removes a single download record:

```go
func (db *DB) DeleteDownload(id int64) error
```

#### DeleteOldDownloads
Bulk deletion of old downloads with cleanup:

```go
func (db *DB) DeleteOldDownloads(olderThan time.Duration) error
```

**Features:**
- Only deletes completed or failed downloads
- Automatically cleans up temporary files
- Logs deletion count

### Directory Mapping Operations

#### CreateDirectoryMapping
Creates a new directory mapping:

```go
func (db *DB) CreateDirectoryMapping(mapping *models.DirectoryMapping) error
```

#### GetDirectoryMappings
Retrieves all mappings ordered by usage:

```go
func (db *DB) GetDirectoryMappings() ([]*models.DirectoryMapping, error)
```

**Returns:** Mappings ordered by `use_count DESC, last_used DESC`

#### UpdateDirectoryMappingUsage
Increments usage count and updates last used time:

```go
func (db *DB) UpdateDirectoryMappingUsage(id int64) error
```

#### GetDirectorySuggestionsForURL
Retrieves directory suggestions for a given URL:

```go
func (db *DB) GetDirectorySuggestionsForURL(url string) ([]*models.DirectoryMapping, error)
```

**Note:** Currently returns all mappings ordered by usage for client-side filtering

### Download Group Operations

#### CreateDownloadGroup
Creates a new download group:

```go
func (db *DB) CreateDownloadGroup(group *models.DownloadGroup) error
```

#### GetDownloadGroup
Retrieves a download group by ID:

```go
func (db *DB) GetDownloadGroup(id string) (*models.DownloadGroup, error)
```

#### UpdateDownloadGroup
Updates group status and completion count:

```go
func (db *DB) UpdateDownloadGroup(group *models.DownloadGroup) error
```

#### GetDownloadsByGroupID
Retrieves all downloads in a group:

```go
func (db *DB) GetDownloadsByGroupID(groupID string) ([]*models.Download, error)
```

**Returns:** Downloads ordered by `created_at ASC, id ASC`

### Extracted File Operations

#### CreateExtractedFile
Records an extracted file:

```go
func (db *DB) CreateExtractedFile(file *models.ExtractedFile) error
```

#### GetExtractedFilesByDownloadID
Retrieves non-deleted extracted files:

```go
func (db *DB) GetExtractedFilesByDownloadID(downloadID int64) ([]*models.ExtractedFile, error)
```

#### MarkExtractedFileDeleted
Soft-deletes an extracted file:

```go
func (db *DB) MarkExtractedFileDeleted(id int64, deletedAt time.Time) error
```

## Connection Management

### Connection Settings
The database uses optimized SQLite settings for concurrent access:

```go
// Connection string includes:
// - _busy_timeout=30000 (30 second timeout)
// - _journal_mode=WAL (Write-Ahead Logging)
// - _synchronous=NORMAL (balanced performance/safety)
```

### Connection Pool
```go
conn.SetMaxOpenConns(1) // SQLite doesn't handle concurrent writes well
conn.SetMaxIdleConns(1)
```

**Note:** Limited to single connection due to SQLite's limited concurrent write capability

### Database Paths
- Use `:memory:` for in-memory testing databases
- File paths automatically get connection parameters appended
- Schema initialization happens automatically on connection

## Error Handling

### Error Types
- **Connection Errors**: Database file access, network issues
- **Schema Errors**: Table creation, index creation failures
- **Query Errors**: SQL syntax, constraint violations
- **Not Found Errors**: Specific error for missing records

### Error Handling Patterns
```go
// Not found errors
if err == sql.ErrNoRows {
    return nil, fmt.Errorf("download not found")
}

// Wrapped errors with context
return fmt.Errorf("failed to create download: %w", err)
```

### Best Practices
- Always wrap errors with context
- Use specific error messages for common cases
- Log but don't fail on cleanup operations
- Handle `sql.ErrNoRows` explicitly

## Testing

### Test Database Strategy
All tests use in-memory databases:

```go
db, err := New(":memory:")
```

### Test Coverage
The test suite covers:
- **Connection Management**: Valid/invalid paths, connection lifecycle
- **CRUD Operations**: Create, read, update, delete for all entities
- **Search Functionality**: Fuzzy matching, filtering, pagination
- **Error Cases**: Missing records, closed connections, invalid operations
- **Edge Cases**: Pagination boundaries, empty results, bulk operations

### Test Patterns
```go
func TestDB_Operation(t *testing.T) {
    db, err := New(":memory:")
    require.NoError(t, err)
    defer db.Close()
    
    // Test implementation
}
```

### Key Test Categories
1. **Happy Path Tests**: Normal operations with valid data
2. **Error Path Tests**: Invalid inputs, missing records
3. **Edge Case Tests**: Boundary conditions, empty states
4. **Integration Tests**: Multi-table operations, transactions

## Usage Examples

### Basic Download Lifecycle
```go
// Create database connection
db, err := New("/path/to/database.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Create a new download
download := &models.Download{
    OriginalURL:     "https://example.com/file.zip",
    UnrestrictedURL: "https://alldebrid.com/file.zip",
    Filename:        "file.zip",
    Directory:       "/downloads",
    Status:          models.StatusPending,
    CreatedAt:       time.Now(),
    UpdatedAt:       time.Now(),
}

err = db.CreateDownload(download)
if err != nil {
    log.Fatal(err)
}

// Update download progress
download.Status = models.StatusDownloading
download.Progress = 50.0
download.DownloadedBytes = 512000
download.UpdatedAt = time.Now()

err = db.UpdateDownload(download)
if err != nil {
    log.Fatal(err)
}
```

### Directory Learning System
```go
// Create directory mapping
mapping := &models.DirectoryMapping{
    FilenamePattern: "*.mp4",
    OriginalURL:     "https://example.com/movie.mp4",
    Directory:       "/downloads/movies",
    UseCount:        1,
    LastUsed:        time.Now(),
    CreatedAt:       time.Now(),
}

err = db.CreateDirectoryMapping(mapping)
if err != nil {
    log.Fatal(err)
}

// Update usage when pattern is reused
err = db.UpdateDirectoryMappingUsage(mapping.ID)
if err != nil {
    log.Fatal(err)
}
```

### Search and Pagination
```go
// Search for downloads with fuzzy matching
results, err := db.SearchDownloads("movie", "completed", 10, 0)
if err != nil {
    log.Fatal(err)
}

// Paginate through all downloads
page := 0
limit := 20
for {
    downloads, err := db.ListDownloads(limit, page*limit)
    if err != nil {
        log.Fatal(err)
    }
    
    if len(downloads) == 0 {
        break // No more results
    }
    
    // Process downloads
    for _, download := range downloads {
        fmt.Printf("Download: %s (%s)\n", download.Filename, download.Status)
    }
    
    page++
}
```

### Download Groups and Archives
```go
// Create download group
group := &models.DownloadGroup{
    ID:                 "group-123",
    CreatedAt:          time.Now(),
    TotalDownloads:     3,
    CompletedDownloads: 0,
    Status:             models.GroupStatusDownloading,
}

err = db.CreateDownloadGroup(group)
if err != nil {
    log.Fatal(err)
}

// Track extracted files
extractedFile := &models.ExtractedFile{
    DownloadID: download.ID,
    FilePath:   "/downloads/extracted/file1.txt",
    CreatedAt:  time.Now(),
}

err = db.CreateExtractedFile(extractedFile)
if err != nil {
    log.Fatal(err)
}
```

### Cleanup Operations
```go
// Delete downloads older than 30 days
err = db.DeleteOldDownloads(30 * 24 * time.Hour)
if err != nil {
    log.Fatal(err)
}

// Mark extracted file as deleted
err = db.MarkExtractedFileDeleted(extractedFile.ID, time.Now())
if err != nil {
    log.Fatal(err)
}
```

## Best Practices

### Database Connection
1. **Always close connections**: Use `defer db.Close()` immediately after successful connection
2. **Use connection pooling**: The package handles this automatically
3. **Handle initialization errors**: Check for schema creation failures

### Error Handling
1. **Wrap errors with context**: Always provide meaningful error messages
2. **Handle `sql.ErrNoRows`**: Distinguish between errors and missing records
3. **Don't fail on cleanup**: Log cleanup errors but continue execution

### Performance Optimization
1. **Use indexes**: All queries are optimized with appropriate indexes
2. **Batch operations**: Use transactions for multiple related operations
3. **Limit result sets**: Always use pagination for large datasets
4. **Use prepared statements**: The package handles this internally

### Data Consistency
1. **Update timestamps**: Always update `updated_at` when modifying records
2. **Use transactions**: For operations that must succeed or fail together
3. **Validate input**: Check required fields before database operations
4. **Use soft deletes**: For extracted files to maintain audit trail

### Testing
1. **Use in-memory databases**: For fast, isolated tests
2. **Clean up resources**: Always close test databases
3. **Test error paths**: Verify error handling works correctly
4. **Test edge cases**: Empty results, boundary conditions

### Schema Evolution
1. **Version control schema**: Track changes to database structure
2. **Backup before changes**: Always backup before schema modifications
3. **Test migrations**: Verify upgrade/downgrade procedures work
4. **Document changes**: Keep changelog of schema modifications

---

**Last Updated:** 2025-07-06  
**Version:** 1.0.0  
**File:** `/root/repos/debrid-downloader/internal/database/README.md`