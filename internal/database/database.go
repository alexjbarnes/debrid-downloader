// Package database provides SQLite database operations for the application
package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"debrid-downloader/pkg/models"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*DB, error) {
	// Add connection parameters to help with concurrent access
	connString := dbPath
	if dbPath != ":memory:" {
		connString = dbPath + "?_busy_timeout=30000&_journal_mode=WAL&_synchronous=NORMAL"
	}
	
	conn, err := sql.Open("sqlite", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	conn.SetMaxOpenConns(1) // SQLite doesn't handle concurrent writes well
	conn.SetMaxIdleConns(1)
	
	db := &DB{conn: conn}

	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates the necessary tables
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS downloads (
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

	CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
	CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at);
	CREATE INDEX IF NOT EXISTS idx_downloads_group_id ON downloads(group_id);

	CREATE TABLE IF NOT EXISTS directory_mappings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename_pattern TEXT NOT NULL,
		original_url TEXT,
		directory TEXT NOT NULL,
		use_count INTEGER DEFAULT 1,
		last_used DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_directory_mappings_pattern ON directory_mappings(filename_pattern);
	CREATE INDEX IF NOT EXISTS idx_directory_mappings_use_count ON directory_mappings(use_count DESC);

	CREATE TABLE IF NOT EXISTS download_groups (
		id TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		total_downloads INTEGER NOT NULL,
		completed_downloads INTEGER DEFAULT 0,
		status TEXT NOT NULL,
		processing_error TEXT
	);

	CREATE TABLE IF NOT EXISTS extracted_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		download_id INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		deleted_at DATETIME,
		FOREIGN KEY (download_id) REFERENCES downloads(id)
	);

	CREATE INDEX IF NOT EXISTS idx_extracted_files_download_id ON extracted_files(download_id);
	CREATE INDEX IF NOT EXISTS idx_extracted_files_deleted_at ON extracted_files(deleted_at);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// CreateDownload creates a new download record
func (db *DB) CreateDownload(download *models.Download) error {
	query := `
	INSERT INTO downloads (
		original_url, unrestricted_url, filename, directory, status,
		progress, file_size, downloaded_bytes, download_speed,
		error_message, retry_count, created_at, updated_at,
		started_at, completed_at, paused_at, total_paused_time,
		group_id, is_archive, extracted_files
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query,
		download.OriginalURL, download.UnrestrictedURL, download.Filename,
		download.Directory, download.Status, download.Progress,
		download.FileSize, download.DownloadedBytes, download.DownloadSpeed,
		download.ErrorMessage, download.RetryCount, download.CreatedAt,
		download.UpdatedAt, download.StartedAt, download.CompletedAt,
		download.PausedAt, download.TotalPausedTime,
		download.GroupID, download.IsArchive, download.ExtractedFiles,
	)
	if err != nil {
		return fmt.Errorf("failed to create download: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	download.ID = id
	return nil
}

// GetDownload retrieves a download by ID
func (db *DB) GetDownload(id int64) (*models.Download, error) {
	query := `
	SELECT id, original_url, unrestricted_url, filename, directory, status,
		   progress, file_size, downloaded_bytes, download_speed,
		   error_message, retry_count, created_at, updated_at,
		   started_at, completed_at, paused_at, total_paused_time,
		   group_id, is_archive, extracted_files
	FROM downloads WHERE id = ?
	`

	var download models.Download
	err := db.conn.QueryRow(query, id).Scan(
		&download.ID, &download.OriginalURL, &download.UnrestrictedURL,
		&download.Filename, &download.Directory, &download.Status,
		&download.Progress, &download.FileSize, &download.DownloadedBytes,
		&download.DownloadSpeed, &download.ErrorMessage, &download.RetryCount,
		&download.CreatedAt, &download.UpdatedAt, &download.StartedAt,
		&download.CompletedAt, &download.PausedAt, &download.TotalPausedTime,
		&download.GroupID, &download.IsArchive, &download.ExtractedFiles,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("download not found")
		}
		return nil, fmt.Errorf("failed to get download: %w", err)
	}

	return &download, nil
}

// UpdateDownload updates an existing download record
func (db *DB) UpdateDownload(download *models.Download) error {
	query := `
	UPDATE downloads SET
		unrestricted_url = ?, status = ?, progress = ?, file_size = ?,
		downloaded_bytes = ?, download_speed = ?, error_message = ?,
		retry_count = ?, updated_at = ?, started_at = ?, completed_at = ?,
		paused_at = ?, total_paused_time = ?, group_id = ?, is_archive = ?,
		extracted_files = ?
	WHERE id = ?
	`

	_, err := db.conn.Exec(query,
		download.UnrestrictedURL, download.Status, download.Progress,
		download.FileSize, download.DownloadedBytes, download.DownloadSpeed,
		download.ErrorMessage, download.RetryCount, download.UpdatedAt,
		download.StartedAt, download.CompletedAt, download.PausedAt,
		download.TotalPausedTime, download.GroupID, download.IsArchive,
		download.ExtractedFiles, download.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update download: %w", err)
	}

	return nil
}

// ListDownloads retrieves downloads with pagination
func (db *DB) ListDownloads(limit, offset int) ([]*models.Download, error) {
	query := `
	SELECT id, original_url, unrestricted_url, filename, directory, status,
		   progress, file_size, downloaded_bytes, download_speed,
		   error_message, retry_count, created_at, updated_at,
		   started_at, completed_at, paused_at, total_paused_time,
		   group_id, is_archive, extracted_files
	FROM downloads 
	ORDER BY created_at DESC, id ASC 
	LIMIT ? OFFSET ?
	`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list downloads: %w", err)
	}
	defer rows.Close()

	var downloads []*models.Download
	for rows.Next() {
		var download models.Download
		err := rows.Scan(
			&download.ID, &download.OriginalURL, &download.UnrestrictedURL,
			&download.Filename, &download.Directory, &download.Status,
			&download.Progress, &download.FileSize, &download.DownloadedBytes,
			&download.DownloadSpeed, &download.ErrorMessage, &download.RetryCount,
			&download.CreatedAt, &download.UpdatedAt, &download.StartedAt,
			&download.CompletedAt, &download.PausedAt, &download.TotalPausedTime,
			&download.GroupID, &download.IsArchive, &download.ExtractedFiles,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download: %w", err)
		}
		downloads = append(downloads, &download)
	}

	return downloads, nil
}

// GetPendingDownloadsOldestFirst retrieves all pending downloads ordered by creation time (oldest first)
func (db *DB) GetPendingDownloadsOldestFirst() ([]*models.Download, error) {
	query := `
	SELECT id, original_url, unrestricted_url, filename, directory, status,
		   progress, file_size, downloaded_bytes, download_speed,
		   error_message, retry_count, created_at, updated_at,
		   started_at, completed_at, paused_at, total_paused_time,
		   group_id, is_archive, extracted_files
	FROM downloads 
	WHERE status = ?
	ORDER BY created_at ASC, id ASC
	`

	rows, err := db.conn.Query(query, models.StatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending downloads: %w", err)
	}
	defer rows.Close()

	var downloads []*models.Download
	for rows.Next() {
		var download models.Download
		err := rows.Scan(
			&download.ID, &download.OriginalURL, &download.UnrestrictedURL,
			&download.Filename, &download.Directory, &download.Status,
			&download.Progress, &download.FileSize, &download.DownloadedBytes,
			&download.DownloadSpeed, &download.ErrorMessage, &download.RetryCount,
			&download.CreatedAt, &download.UpdatedAt, &download.StartedAt,
			&download.CompletedAt, &download.PausedAt, &download.TotalPausedTime,
			&download.GroupID, &download.IsArchive, &download.ExtractedFiles,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download: %w", err)
		}
		downloads = append(downloads, &download)
	}

	return downloads, nil
}

// GetOrphanedDownloads retrieves downloads stuck in downloading state (orphaned by server restart)
func (db *DB) GetOrphanedDownloads() ([]*models.Download, error) {
	query := `
	SELECT id, original_url, unrestricted_url, filename, directory, status,
		   progress, file_size, downloaded_bytes, download_speed,
		   error_message, retry_count, created_at, updated_at,
		   started_at, completed_at, paused_at, total_paused_time,
		   group_id, is_archive, extracted_files
	FROM downloads 
	WHERE status = ?
	ORDER BY created_at ASC, id ASC
	`

	rows, err := db.conn.Query(query, models.StatusDownloading)
	if err != nil {
		return nil, fmt.Errorf("failed to get orphaned downloads: %w", err)
	}
	defer rows.Close()

	var downloads []*models.Download
	for rows.Next() {
		var download models.Download
		err := rows.Scan(
			&download.ID, &download.OriginalURL, &download.UnrestrictedURL,
			&download.Filename, &download.Directory, &download.Status,
			&download.Progress, &download.FileSize, &download.DownloadedBytes,
			&download.DownloadSpeed, &download.ErrorMessage, &download.RetryCount,
			&download.CreatedAt, &download.UpdatedAt, &download.StartedAt,
			&download.CompletedAt, &download.PausedAt, &download.TotalPausedTime,
			&download.GroupID, &download.IsArchive, &download.ExtractedFiles,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download: %w", err)
		}
		downloads = append(downloads, &download)
	}

	return downloads, nil
}

// SearchDownloads performs a fuzzy search on downloads with support for multiple status filters and custom sort order
func (db *DB) SearchDownloads(searchTerm string, statusFilters []string, sortOrder string, limit, offset int) ([]*models.Download, error) {
	query := `
	SELECT id, original_url, unrestricted_url, filename, directory, status,
		   progress, file_size, downloaded_bytes, download_speed,
		   error_message, retry_count, created_at, updated_at,
		   started_at, completed_at, paused_at, total_paused_time,
		   group_id, is_archive, extracted_files
	FROM downloads 
	WHERE 1=1`

	args := []interface{}{}

	// Add search term filter with fuzzy matching
	if searchTerm != "" {
		// Split search term into words for fuzzy matching
		words := strings.Fields(strings.ToLower(searchTerm))
		if len(words) > 0 {
			// Create conditions for each word - any word can match any field
			var conditions []string
			for _, word := range words {
				// Add multiple pattern variations for better fuzzy matching
				wordPattern := "%" + word + "%"
				var wordConditions []string

				// Exact partial match
				wordConditions = append(wordConditions, "(LOWER(filename) LIKE ? OR LOWER(original_url) LIKE ? OR LOWER(directory) LIKE ?)")
				args = append(args, wordPattern, wordPattern, wordPattern)

				// Match with word boundaries for more precise results
				if len(word) >= 3 {
					// Pattern that allows character separation (e.g., "example" matches "example_file")
					partialPattern := "%"
					for i, char := range word {
						if i > 0 {
							partialPattern += "_?"
						}
						partialPattern += string(char)
					}
					partialPattern += "%"
					wordConditions = append(wordConditions, "(LOWER(filename) LIKE ? OR LOWER(original_url) LIKE ? OR LOWER(directory) LIKE ?)")
					args = append(args, partialPattern, partialPattern, partialPattern)
				}

				// Add pattern for character substitution fuzzy matching (if word is 4+ chars)
				if len(word) >= 4 {
					// Create pattern with wildcard in middle for typo tolerance
					midPattern := "%" + word[:2] + "%" + word[len(word)-2:] + "%"
					wordConditions = append(wordConditions, "(LOWER(filename) LIKE ? OR LOWER(original_url) LIKE ? OR LOWER(directory) LIKE ?)")
					args = append(args, midPattern, midPattern, midPattern)
				}

				conditions = append(conditions, "("+strings.Join(wordConditions, " OR ")+")")
			}
			// Any word matching is enough (use OR instead of AND)
			query += ` AND (` + strings.Join(conditions, " OR ") + `)`
		}
	}

	// Add status filter - support multiple statuses
	// If no statuses provided, return no results
	if len(statusFilters) == 0 {
		query += ` AND 1=0`
	} else {
		placeholders := make([]string, len(statusFilters))
		for i, status := range statusFilters {
			placeholders[i] = "?"
			args = append(args, status)
		}
		query += ` AND status IN (` + strings.Join(placeholders, ",") + `)`
	}

	// Add sort order
	if sortOrder == "asc" {
		query += ` ORDER BY created_at ASC, id DESC`
	} else {
		query += ` ORDER BY created_at DESC, id ASC`
	}
	
	query += ` LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search downloads: %w", err)
	}
	defer rows.Close()

	var downloads []*models.Download
	for rows.Next() {
		var download models.Download
		err := rows.Scan(
			&download.ID, &download.OriginalURL, &download.UnrestrictedURL,
			&download.Filename, &download.Directory, &download.Status,
			&download.Progress, &download.FileSize, &download.DownloadedBytes,
			&download.DownloadSpeed, &download.ErrorMessage, &download.RetryCount,
			&download.CreatedAt, &download.UpdatedAt, &download.StartedAt,
			&download.CompletedAt, &download.PausedAt, &download.TotalPausedTime,
			&download.GroupID, &download.IsArchive, &download.ExtractedFiles,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download: %w", err)
		}
		downloads = append(downloads, &download)
	}

	return downloads, nil
}

// DeleteOldDownloads removes downloads older than the specified duration
func (db *DB) DeleteOldDownloads(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	// First get the downloads that will be deleted to clean up temp files
	selectQuery := `
		SELECT id, filename, directory FROM downloads 
		WHERE created_at < ? AND status IN ('failed', 'completed')
	`

	rows, err := db.conn.Query(selectQuery, cutoff)
	if err != nil {
		return fmt.Errorf("failed to query old downloads: %w", err)
	}
	defer rows.Close()

	// Collect downloads to clean up
	type downloadInfo struct {
		ID        int64
		Filename  string
		Directory string
	}
	var downloads []downloadInfo

	for rows.Next() {
		var dl downloadInfo
		if err := rows.Scan(&dl.ID, &dl.Filename, &dl.Directory); err != nil {
			continue // Skip on error
		}
		downloads = append(downloads, dl)
	}

	// Delete from database
	deleteQuery := "DELETE FROM downloads WHERE created_at < ? AND status IN ('failed', 'completed')"
	result, err := db.conn.Exec(deleteQuery, cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete old downloads: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	// Clean up any temporary files (best effort, don't fail on errors)
	for _, dl := range downloads {
		tempFilename := fmt.Sprintf("%s.%d.tmp", dl.Filename, dl.ID)
		tempPath := filepath.Join(dl.Directory, tempFilename)
		os.Remove(tempPath) // Ignore errors
	}

	if rowsAffected > 0 {
		slog.Info("Deleted old downloads", "count", rowsAffected, "cutoff", cutoff)
	}

	return nil
}

// DeleteDownload removes a single download record by ID
func (db *DB) DeleteDownload(id int64) error {
	query := "DELETE FROM downloads WHERE id = ?"

	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete download: %w", err)
	}

	return nil
}

// CreateDirectoryMapping creates a new directory mapping
func (db *DB) CreateDirectoryMapping(mapping *models.DirectoryMapping) error {
	query := `
	INSERT INTO directory_mappings (
		filename_pattern, original_url, directory, use_count, last_used, created_at
	) VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query,
		mapping.FilenamePattern, mapping.OriginalURL, mapping.Directory,
		mapping.UseCount, mapping.LastUsed, mapping.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create directory mapping: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	mapping.ID = id
	return nil
}

// GetDirectoryMappings retrieves all directory mappings ordered by use count
func (db *DB) GetDirectoryMappings() ([]*models.DirectoryMapping, error) {
	query := `
	SELECT id, filename_pattern, original_url, directory, use_count, last_used, created_at
	FROM directory_mappings 
	ORDER BY use_count DESC, last_used DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get directory mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*models.DirectoryMapping
	for rows.Next() {
		var mapping models.DirectoryMapping
		err := rows.Scan(
			&mapping.ID, &mapping.FilenamePattern, &mapping.OriginalURL,
			&mapping.Directory, &mapping.UseCount, &mapping.LastUsed, &mapping.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory mapping: %w", err)
		}
		mappings = append(mappings, &mapping)
	}

	return mappings, nil
}

// UpdateDirectoryMappingUsage updates the use count and last used time for a mapping
func (db *DB) UpdateDirectoryMappingUsage(id int64) error {
	query := `
	UPDATE directory_mappings 
	SET use_count = use_count + 1, last_used = ? 
	WHERE id = ?
	`

	_, err := db.conn.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update directory mapping usage: %w", err)
	}

	return nil
}

// GetDirectorySuggestionsForURL retrieves directory mappings that might match the given URL
func (db *DB) GetDirectorySuggestionsForURL(url string) ([]*models.DirectoryMapping, error) {
	query := `
	SELECT id, filename_pattern, original_url, directory, use_count, last_used, created_at
	FROM directory_mappings 
	ORDER BY use_count DESC, last_used DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get directory suggestions: %w", err)
	}
	defer rows.Close()

	var mappings []*models.DirectoryMapping
	for rows.Next() {
		var mapping models.DirectoryMapping
		err := rows.Scan(
			&mapping.ID, &mapping.FilenamePattern, &mapping.OriginalURL,
			&mapping.Directory, &mapping.UseCount, &mapping.LastUsed, &mapping.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory mapping: %w", err)
		}
		mappings = append(mappings, &mapping)
	}

	return mappings, nil
}

// CreateDownloadGroup creates a new download group record
func (db *DB) CreateDownloadGroup(group *models.DownloadGroup) error {
	query := `
	INSERT INTO download_groups (
		id, created_at, total_downloads, completed_downloads, status, processing_error
	) VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.conn.Exec(query,
		group.ID, group.CreatedAt, group.TotalDownloads,
		group.CompletedDownloads, group.Status, group.ProcessingError,
	)
	if err != nil {
		return fmt.Errorf("failed to create download group: %w", err)
	}

	return nil
}

// GetDownloadGroup retrieves a download group by ID
func (db *DB) GetDownloadGroup(id string) (*models.DownloadGroup, error) {
	query := `
	SELECT id, created_at, total_downloads, completed_downloads, status, processing_error
	FROM download_groups WHERE id = ?
	`

	var group models.DownloadGroup
	err := db.conn.QueryRow(query, id).Scan(
		&group.ID, &group.CreatedAt, &group.TotalDownloads,
		&group.CompletedDownloads, &group.Status, &group.ProcessingError,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("download group not found")
		}
		return nil, fmt.Errorf("failed to get download group: %w", err)
	}

	return &group, nil
}

// UpdateDownloadGroup updates an existing download group record
func (db *DB) UpdateDownloadGroup(group *models.DownloadGroup) error {
	query := `
	UPDATE download_groups SET
		completed_downloads = ?, status = ?, processing_error = ?
	WHERE id = ?
	`

	_, err := db.conn.Exec(query,
		group.CompletedDownloads, group.Status, group.ProcessingError, group.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update download group: %w", err)
	}

	return nil
}

// GetDownloadsByGroupID retrieves all downloads for a specific group
func (db *DB) GetDownloadsByGroupID(groupID string) ([]*models.Download, error) {
	query := `
	SELECT id, original_url, unrestricted_url, filename, directory, status,
		   progress, file_size, downloaded_bytes, download_speed,
		   error_message, retry_count, created_at, updated_at,
		   started_at, completed_at, paused_at, total_paused_time,
		   group_id, is_archive, extracted_files
	FROM downloads 
	WHERE group_id = ?
	ORDER BY created_at ASC, id ASC
	`

	rows, err := db.conn.Query(query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get downloads by group: %w", err)
	}
	defer rows.Close()

	var downloads []*models.Download
	for rows.Next() {
		var download models.Download
		err := rows.Scan(
			&download.ID, &download.OriginalURL, &download.UnrestrictedURL,
			&download.Filename, &download.Directory, &download.Status,
			&download.Progress, &download.FileSize, &download.DownloadedBytes,
			&download.DownloadSpeed, &download.ErrorMessage, &download.RetryCount,
			&download.CreatedAt, &download.UpdatedAt, &download.StartedAt,
			&download.CompletedAt, &download.PausedAt, &download.TotalPausedTime,
			&download.GroupID, &download.IsArchive, &download.ExtractedFiles,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download: %w", err)
		}
		downloads = append(downloads, &download)
	}

	return downloads, nil
}

// CreateExtractedFile creates a record for an extracted file
func (db *DB) CreateExtractedFile(file *models.ExtractedFile) error {
	query := `
	INSERT INTO extracted_files (
		download_id, file_path, created_at, deleted_at
	) VALUES (?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query,
		file.DownloadID, file.FilePath, file.CreatedAt, file.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create extracted file: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	file.ID = id
	return nil
}

// GetExtractedFilesByDownloadID retrieves all extracted files for a download
func (db *DB) GetExtractedFilesByDownloadID(downloadID int64) ([]*models.ExtractedFile, error) {
	query := `
	SELECT id, download_id, file_path, created_at, deleted_at
	FROM extracted_files 
	WHERE download_id = ? AND deleted_at IS NULL
	ORDER BY created_at ASC, id ASC
	`

	rows, err := db.conn.Query(query, downloadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get extracted files: %w", err)
	}
	defer rows.Close()

	var files []*models.ExtractedFile
	for rows.Next() {
		var file models.ExtractedFile
		err := rows.Scan(
			&file.ID, &file.DownloadID, &file.FilePath,
			&file.CreatedAt, &file.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan extracted file: %w", err)
		}
		files = append(files, &file)
	}

	return files, nil
}

// MarkExtractedFileDeleted marks an extracted file as deleted
func (db *DB) MarkExtractedFileDeleted(id int64, deletedAt time.Time) error {
	query := `
	UPDATE extracted_files SET deleted_at = ? WHERE id = ?
	`

	_, err := db.conn.Exec(query, deletedAt, id)
	if err != nil {
		return fmt.Errorf("failed to mark extracted file as deleted: %w", err)
	}

	return nil
}

// GetDownloadStats retrieves download statistics by status
func (db *DB) GetDownloadStats() (map[string]int, error) {
	query := `
	SELECT status, COUNT(*) as count
	FROM downloads
	GROUP BY status
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get download stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		err := rows.Scan(&status, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan download stats: %w", err)
		}
		stats[status] = count
	}

	return stats, nil
}
