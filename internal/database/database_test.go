package database

import (
	"fmt"
	"testing"
	"time"

	"debrid-downloader/pkg/models"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		dbPath  string
		wantErr bool
	}{
		{
			name:    "in-memory database",
			dbPath:  ":memory:",
			wantErr: false,
		},
		{
			name:    "temporary file database",
			dbPath:  "/tmp/test.db",
			wantErr: false,
		},
		{
			name:    "invalid database path",
			dbPath:  "/invalid/nonexistent/path/test.db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := New(tt.dbPath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, db)

			// Test that we can close the database
			err = db.Close()
			require.NoError(t, err)
		})
	}
}

func TestDB_CreateDownload(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	download := &models.Download{
		OriginalURL:     "https://example.com/file.zip",
		UnrestrictedURL: "https://alldebrid.com/file.zip",
		Filename:        "file.zip",
		Directory:       "/downloads",
		Status:          models.StatusPending,
		Progress:        0.0,
		FileSize:        1024000,
		DownloadedBytes: 0,
		DownloadSpeed:   0.0,
		RetryCount:      0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err = db.CreateDownload(download)
	require.NoError(t, err)
	require.NotZero(t, download.ID)
}

func TestDB_GetDownload(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download first
	download := &models.Download{
		OriginalURL:     "https://example.com/file.zip",
		UnrestrictedURL: "https://alldebrid.com/file.zip",
		Filename:        "file.zip",
		Directory:       "/downloads",
		Status:          models.StatusPending,
		Progress:        0.0,
		FileSize:        1024000,
		DownloadedBytes: 0,
		DownloadSpeed:   0.0,
		RetryCount:      0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Retrieve the download
	retrieved, err := db.GetDownload(download.ID)
	require.NoError(t, err)
	require.Equal(t, download.ID, retrieved.ID)
	require.Equal(t, download.OriginalURL, retrieved.OriginalURL)
	require.Equal(t, download.Filename, retrieved.Filename)
}

func TestDB_ListDownloads(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create multiple downloads
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/file1.zip",
			Filename:    "file1.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file2.zip",
			Filename:    "file2.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// List all downloads
	retrieved, err := db.ListDownloads(10, 0)
	require.NoError(t, err)
	require.Len(t, retrieved, 2)
}

func TestDB_CreateDirectoryMapping(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	mapping := &models.DirectoryMapping{
		FilenamePattern: "*.zip",
		Directory:       "/downloads/archives",
		UseCount:        1,
		LastUsed:        time.Now(),
		CreatedAt:       time.Now(),
	}

	err = db.CreateDirectoryMapping(mapping)
	require.NoError(t, err)
	require.NotZero(t, mapping.ID)
}

func TestDB_GetDirectoryMappings(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create test mappings
	mappings := []*models.DirectoryMapping{
		{
			FilenamePattern: "movie",
			Directory:       "/downloads/movies",
			UseCount:        5,
			LastUsed:        time.Now(),
			CreatedAt:       time.Now(),
		},
		{
			FilenamePattern: "music",
			Directory:       "/downloads/music",
			UseCount:        3,
			LastUsed:        time.Now(),
			CreatedAt:       time.Now(),
		},
	}

	for _, mapping := range mappings {
		err = db.CreateDirectoryMapping(mapping)
		require.NoError(t, err)
	}

	// Retrieve mappings
	retrieved, err := db.GetDirectoryMappings()
	require.NoError(t, err)
	require.Len(t, retrieved, 2)
}

func TestDB_UpdateDownload(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download first
	download := &models.Download{
		OriginalURL:     "https://example.com/file.zip",
		UnrestrictedURL: "https://alldebrid.com/file.zip",
		Filename:        "file.zip",
		Directory:       "/downloads",
		Status:          models.StatusPending,
		Progress:        0.0,
		FileSize:        1024000,
		DownloadedBytes: 0,
		DownloadSpeed:   0.0,
		RetryCount:      0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Update the download
	download.Status = models.StatusCompleted
	download.Progress = 100.0
	download.DownloadedBytes = 1024000
	download.DownloadSpeed = 500.0

	err = db.UpdateDownload(download)
	require.NoError(t, err)

	// Verify the update
	retrieved, err := db.GetDownload(download.ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusCompleted, retrieved.Status)
	require.Equal(t, 100.0, retrieved.Progress)
	require.Equal(t, int64(1024000), retrieved.DownloadedBytes)
	require.Equal(t, 500.0, retrieved.DownloadSpeed)
}

func TestDB_DeleteOldDownloads(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create old and new downloads
	oldTime := time.Now().AddDate(0, 0, -70) // 70 days ago
	newTime := time.Now().AddDate(0, 0, -30) // 30 days ago

	oldDownload := &models.Download{
		OriginalURL: "https://example.com/old.zip",
		Filename:    "old.zip",
		Directory:   "/downloads",
		Status:      models.StatusCompleted,
		CreatedAt:   oldTime,
		UpdatedAt:   oldTime,
	}

	newDownload := &models.Download{
		OriginalURL: "https://example.com/new.zip",
		Filename:    "new.zip",
		Directory:   "/downloads",
		Status:      models.StatusCompleted,
		CreatedAt:   newTime,
		UpdatedAt:   newTime,
	}

	err = db.CreateDownload(oldDownload)
	require.NoError(t, err)
	err = db.CreateDownload(newDownload)
	require.NoError(t, err)

	// Delete old downloads (older than 60 days)
	err = db.DeleteOldDownloads(60 * 24 * time.Hour)
	require.NoError(t, err)

	// Verify only the new download remains
	downloads, err := db.ListDownloads(10, 0)
	require.NoError(t, err)
	require.Len(t, downloads, 1)
	require.Equal(t, "new.zip", downloads[0].Filename)
}

func TestDB_UpdateDirectoryMappingUsage(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a directory mapping
	mapping := &models.DirectoryMapping{
		FilenamePattern: "*.zip",
		Directory:       "/downloads/archives",
		UseCount:        1,
		LastUsed:        time.Now().AddDate(0, 0, -1), // 1 day ago
		CreatedAt:       time.Now(),
	}

	err = db.CreateDirectoryMapping(mapping)
	require.NoError(t, err)

	// Update usage
	err = db.UpdateDirectoryMappingUsage(mapping.ID)
	require.NoError(t, err)

	// Verify the update
	mappings, err := db.GetDirectoryMappings()
	require.NoError(t, err)
	require.Len(t, mappings, 1)
	updated := mappings[0]
	require.Equal(t, 2, updated.UseCount)
	require.True(t, updated.LastUsed.After(mapping.LastUsed))
}

func TestDB_SearchDownloads(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create test downloads
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/movie.mp4",
			Filename:    "action_movie.mp4",
			Directory:   "/downloads/movies",
			Status:      models.StatusCompleted,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/song.mp3",
			Filename:    "music_track.mp3",
			Directory:   "/downloads/music",
			Status:      models.StatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/document.pdf",
			Filename:    "important_doc.pdf",
			Directory:   "/downloads/docs",
			Status:      models.StatusFailed,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test search by filename with specific status
	results, err := db.SearchDownloads("movie", []string{"completed"}, "desc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "action_movie.mp4", results[0].Filename)

	// Test search by status
	results, err = db.SearchDownloads("", []string{"pending"}, "desc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, models.StatusPending, results[0].Status)

	// Test search by both filename and status
	results, err = db.SearchDownloads("music", []string{"pending"}, "desc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "music_track.mp3", results[0].Filename)

	// Test search with no results (empty status filter)
	results, err = db.SearchDownloads("movie", []string{}, "desc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 0)

	// Test pagination with all statuses
	results, err = db.SearchDownloads("", []string{"completed", "pending", "failed"}, "desc", 2, 0)
	require.NoError(t, err)
	require.Len(t, results, 2)

	results, err = db.SearchDownloads("", []string{"completed", "pending", "failed"}, "desc", 2, 2)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestDB_DeleteDownload(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download
	download := &models.Download{
		OriginalURL: "https://example.com/file.zip",
		Filename:    "file.zip",
		Directory:   "/downloads",
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Delete the download
	err = db.DeleteDownload(download.ID)
	require.NoError(t, err)

	// Verify it's deleted
	_, err = db.GetDownload(download.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "download not found")
}

func TestDB_GetDirectorySuggestionsForURL(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create directory mappings
	mappings := []*models.DirectoryMapping{
		{
			FilenamePattern: ".mp4",
			OriginalURL:     "https://example.com/movies/action.mp4",
			Directory:       "/downloads/movies",
			UseCount:        5,
			LastUsed:        time.Now(),
			CreatedAt:       time.Now(),
		},
		{
			FilenamePattern: ".mp3",
			OriginalURL:     "https://music.com/songs/track.mp3",
			Directory:       "/downloads/music",
			UseCount:        3,
			LastUsed:        time.Now(),
			CreatedAt:       time.Now(),
		},
	}

	for _, mapping := range mappings {
		err = db.CreateDirectoryMapping(mapping)
		require.NoError(t, err)
	}

	// Test URL matching - this function returns all mappings ordered by use_count
	suggestions, err := db.GetDirectorySuggestionsForURL("https://example.com/movies/thriller.mp4")
	require.NoError(t, err)
	require.Len(t, suggestions, 2)
	require.Equal(t, "/downloads/movies", suggestions[0].Directory) // First because use_count is higher (5 vs 3)

	// Test with different URL - still returns all mappings
	suggestions, err = db.GetDirectorySuggestionsForURL("https://unknown.com/file.txt")
	require.NoError(t, err)
	require.Len(t, suggestions, 2)
}

func TestDB_CreateDownloadGroup(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	group := &models.DownloadGroup{
		ID:                 "test-group-id",
		CreatedAt:          time.Now(),
		TotalDownloads:     3,
		CompletedDownloads: 1,
		Status:             models.GroupStatusDownloading,
	}

	err = db.CreateDownloadGroup(group)
	require.NoError(t, err)
}

func TestDB_GetDownloadGroup(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download group
	group := &models.DownloadGroup{
		ID:                 "test-group-id",
		CreatedAt:          time.Now(),
		TotalDownloads:     3,
		CompletedDownloads: 1,
		Status:             models.GroupStatusDownloading,
	}

	err = db.CreateDownloadGroup(group)
	require.NoError(t, err)

	// Retrieve the group
	retrieved, err := db.GetDownloadGroup(group.ID)
	require.NoError(t, err)
	require.Equal(t, group.ID, retrieved.ID)
	require.Equal(t, group.TotalDownloads, retrieved.TotalDownloads)
	require.Equal(t, group.CompletedDownloads, retrieved.CompletedDownloads)
	require.Equal(t, group.Status, retrieved.Status)

	// Test non-existent group
	_, err = db.GetDownloadGroup("non-existent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "download group not found")
}

func TestDB_UpdateDownloadGroup(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download group
	group := &models.DownloadGroup{
		ID:                 "test-group-id",
		CreatedAt:          time.Now(),
		TotalDownloads:     3,
		CompletedDownloads: 1,
		Status:             models.GroupStatusDownloading,
	}

	err = db.CreateDownloadGroup(group)
	require.NoError(t, err)

	// Update the group
	group.CompletedDownloads = 3
	group.Status = models.GroupStatusCompleted

	err = db.UpdateDownloadGroup(group)
	require.NoError(t, err)

	// Verify the update
	retrieved, err := db.GetDownloadGroup(group.ID)
	require.NoError(t, err)
	require.Equal(t, 3, retrieved.CompletedDownloads)
	require.Equal(t, models.GroupStatusCompleted, retrieved.Status)
}

func TestDB_GetDownloadsByGroupID(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	groupID := "test-group-id"

	// Create downloads with the same group ID
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/file1.zip",
			Filename:    "file1.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			GroupID:     groupID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file2.zip",
			Filename:    "file2.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			GroupID:     groupID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file3.zip",
			Filename:    "file3.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			GroupID:     "different-group",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Get downloads by group ID
	groupDownloads, err := db.GetDownloadsByGroupID(groupID)
	require.NoError(t, err)
	require.Len(t, groupDownloads, 2)

	// Verify both downloads belong to the correct group
	for _, download := range groupDownloads {
		require.Equal(t, groupID, download.GroupID)
	}
}

func TestDB_CreateExtractedFile(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download first
	download := &models.Download{
		OriginalURL: "https://example.com/archive.zip",
		Filename:    "archive.zip",
		Directory:   "/downloads",
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Create extracted file
	extractedFile := &models.ExtractedFile{
		DownloadID: download.ID,
		FilePath:   "/downloads/extracted/file1.txt",
		CreatedAt:  time.Now(),
	}

	err = db.CreateExtractedFile(extractedFile)
	require.NoError(t, err)
	require.NotZero(t, extractedFile.ID)
}

func TestDB_GetExtractedFilesByDownloadID(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download first
	download := &models.Download{
		OriginalURL: "https://example.com/archive.zip",
		Filename:    "archive.zip",
		Directory:   "/downloads",
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Create extracted files
	extractedFiles := []*models.ExtractedFile{
		{
			DownloadID: download.ID,
			FilePath:   "/downloads/extracted/file1.txt",
			CreatedAt:  time.Now(),
		},
		{
			DownloadID: download.ID,
			FilePath:   "/downloads/extracted/file2.txt",
			CreatedAt:  time.Now(),
		},
	}

	for _, file := range extractedFiles {
		err = db.CreateExtractedFile(file)
		require.NoError(t, err)
	}

	// Get extracted files by download ID
	files, err := db.GetExtractedFilesByDownloadID(download.ID)
	require.NoError(t, err)
	require.Len(t, files, 2)

	// Verify file paths
	filePaths := make([]string, len(files))
	for i, file := range files {
		filePaths[i] = file.FilePath
	}
	require.Contains(t, filePaths, "/downloads/extracted/file1.txt")
	require.Contains(t, filePaths, "/downloads/extracted/file2.txt")
}

func TestDB_MarkExtractedFileDeleted(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a download first
	download := &models.Download{
		OriginalURL: "https://example.com/archive.zip",
		Filename:    "archive.zip",
		Directory:   "/downloads",
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Create extracted file
	extractedFile := &models.ExtractedFile{
		DownloadID: download.ID,
		FilePath:   "/downloads/extracted/file1.txt",
		CreatedAt:  time.Now(),
	}

	err = db.CreateExtractedFile(extractedFile)
	require.NoError(t, err)

	// Mark file as deleted
	deletedAt := time.Now()
	err = db.MarkExtractedFileDeleted(extractedFile.ID, deletedAt)
	require.NoError(t, err)

	// Verify file is no longer returned (filtered out by WHERE deleted_at IS NULL)
	files, err := db.GetExtractedFilesByDownloadID(download.ID)
	require.NoError(t, err)
	require.Len(t, files, 0) // Deleted files are filtered out
}

func TestDB_ErrorCases(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Test GetDownload with non-existent ID
	_, err = db.GetDownload(99999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "download not found")

	// Test UpdateDirectoryMappingUsage with non-existent ID
	err = db.UpdateDirectoryMappingUsage(99999)
	require.NoError(t, err) // This should not error even if no rows affected

	// Test DeleteDownload with non-existent ID
	err = db.DeleteDownload(99999)
	require.NoError(t, err) // This should not error even if no rows affected
}

func TestDB_SearchDownloadsErrorCases(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Test with empty search term and no status filter
	results, err := db.SearchDownloads("", []string{}, "desc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 0)

	// Test with only status filter
	results, err = db.SearchDownloads("", []string{"completed"}, "desc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 0)
}

func TestDB_ListDownloadsWithPagination(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create multiple downloads
	for i := 0; i < 5; i++ {
		download := &models.Download{
			OriginalURL: fmt.Sprintf("https://example.com/file%d.zip", i),
			Filename:    fmt.Sprintf("file%d.zip", i),
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			CreatedAt:   time.Now().Add(time.Duration(i) * time.Minute),
			UpdatedAt:   time.Now(),
		}
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test first page
	downloads, err := db.ListDownloads(3, 0)
	require.NoError(t, err)
	require.Len(t, downloads, 3)

	// Test second page
	downloads, err = db.ListDownloads(3, 3)
	require.NoError(t, err)
	require.Len(t, downloads, 2)

	// Test beyond available records
	downloads, err = db.ListDownloads(3, 10)
	require.NoError(t, err)
	require.Len(t, downloads, 0)
}

func TestDB_InvalidDatabaseOperations(t *testing.T) {
	// Test with closed database
	db, err := New(":memory:")
	require.NoError(t, err)
	
	// Close the database
	err = db.Close()
	require.NoError(t, err)

	// Now try operations on closed database
	download := &models.Download{
		OriginalURL: "https://example.com/file.zip",
		Filename:    "file.zip",
		Directory:   "/downloads",
		Status:      models.StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// These should all fail with database closed errors
	err = db.CreateDownload(download)
	require.Error(t, err)

	_, err = db.GetDownload(1)
	require.Error(t, err)

	_, err = db.ListDownloads(10, 0)
	require.Error(t, err)

	err = db.UpdateDownload(download)
	require.Error(t, err)

	_, err = db.SearchDownloads("test", []string{}, "desc", 10, 0)
	require.Error(t, err)

	err = db.DeleteDownload(1)
	require.Error(t, err)

	err = db.DeleteOldDownloads(time.Hour)
	require.Error(t, err)
}

// Tests for new methods added in recent commits

func TestDB_GetDownloadStats(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Test empty database
	stats, err := db.GetDownloadStats()
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, 0, stats["pending"])
	require.Equal(t, 0, stats["downloading"])
	require.Equal(t, 0, stats["completed"])
	require.Equal(t, 0, stats["failed"])
	require.Equal(t, 0, stats["paused"])

	// Create downloads with different statuses
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/file1.zip",
			Filename:    "file1.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file2.zip",
			Filename:    "file2.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file3.zip",
			Filename:    "file3.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file4.zip",
			Filename:    "file4.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file5.zip",
			Filename:    "file5.zip",
			Directory:   "/downloads",
			Status:      models.StatusFailed,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file6.zip",
			Filename:    "file6.zip",
			Directory:   "/downloads",
			Status:      models.StatusPaused,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test statistics calculation
	stats, err = db.GetDownloadStats()
	require.NoError(t, err)
	require.Equal(t, 2, stats["pending"])
	require.Equal(t, 1, stats["downloading"])
	require.Equal(t, 1, stats["completed"])
	require.Equal(t, 1, stats["failed"])
	require.Equal(t, 1, stats["paused"])
}

func TestDB_GetOrphanedDownloads(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Test empty database
	orphaned, err := db.GetOrphanedDownloads()
	require.NoError(t, err)
	require.Len(t, orphaned, 0)

	// Create downloads with different statuses
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/file1.zip",
			Filename:    "file1.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			CreatedAt:   time.Now().Add(-2 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file2.zip",
			Filename:    "file2.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			CreatedAt:   time.Now().Add(-1 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file3.zip",
			Filename:    "file3.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file4.zip",
			Filename:    "file4.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test orphaned downloads retrieval
	orphaned, err = db.GetOrphanedDownloads()
	require.NoError(t, err)
	require.Len(t, orphaned, 2)

	// Verify only downloading status downloads are returned
	for _, download := range orphaned {
		require.Equal(t, models.StatusDownloading, download.Status)
	}

	// Verify ordering by creation time (oldest first)
	require.Equal(t, "file1.zip", orphaned[0].Filename) // Older download should be first
	require.Equal(t, "file2.zip", orphaned[1].Filename)
}

func TestDB_GetPendingDownloadsOldestFirst(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Test empty database
	pending, err := db.GetPendingDownloadsOldestFirst()
	require.NoError(t, err)
	require.Len(t, pending, 0)

	// Create downloads with different statuses and creation times
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/file1.zip",
			Filename:    "file1.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   time.Now().Add(-3 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file2.zip",
			Filename:    "file2.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			CreatedAt:   time.Now().Add(-2 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file3.zip",
			Filename:    "file3.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   time.Now().Add(-1 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalURL: "https://example.com/file4.zip",
			Filename:    "file4.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test pending downloads retrieval
	pending, err = db.GetPendingDownloadsOldestFirst()
	require.NoError(t, err)
	require.Len(t, pending, 2)

	// Verify only pending status downloads are returned
	for _, download := range pending {
		require.Equal(t, models.StatusPending, download.Status)
	}

	// Verify ordering by creation time (oldest first)
	require.Equal(t, "file1.zip", pending[0].Filename) // Oldest download should be first
	require.Equal(t, "file3.zip", pending[1].Filename)
}

func TestDB_StatusBasedSortingInListDownloads(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create downloads with different statuses (creation time should not affect priority sorting)
	baseTime := time.Now()
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/completed.zip",
			Filename:    "completed.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			CreatedAt:   baseTime.Add(-4 * time.Hour), // Oldest, but should be last
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/downloading.zip",
			Filename:    "downloading.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			CreatedAt:   baseTime.Add(-3 * time.Hour), // Should be first
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/failed.zip",
			Filename:    "failed.zip",
			Directory:   "/downloads",
			Status:      models.StatusFailed,
			CreatedAt:   baseTime.Add(-2 * time.Hour), // Should be last
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/pending.zip",
			Filename:    "pending.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   baseTime.Add(-1 * time.Hour), // Should be second
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/paused.zip",
			Filename:    "paused.zip",
			Directory:   "/downloads",
			Status:      models.StatusPaused,
			CreatedAt:   baseTime, // Newest, but should be third
			UpdatedAt:   baseTime,
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test status-based priority sorting
	results, err := db.ListDownloads(10, 0)
	require.NoError(t, err)
	require.Len(t, results, 5)

	// Verify status priority order: downloading(1) → pending(2) → paused(3) → others(4)
	require.Equal(t, models.StatusDownloading, results[0].Status)
	require.Equal(t, "downloading.zip", results[0].Filename)

	require.Equal(t, models.StatusPending, results[1].Status)
	require.Equal(t, "pending.zip", results[1].Filename)

	require.Equal(t, models.StatusPaused, results[2].Status)
	require.Equal(t, "paused.zip", results[2].Filename)

	// Failed and completed should be last (in creation time order within same priority)
	require.True(t, results[3].Status == models.StatusCompleted || results[3].Status == models.StatusFailed)
	require.True(t, results[4].Status == models.StatusCompleted || results[4].Status == models.StatusFailed)
}

func TestDB_StatusBasedSortingInSearchDownloads(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create downloads with different statuses
	baseTime := time.Now()
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/test_completed.zip",
			Filename:    "test_completed.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			CreatedAt:   baseTime.Add(-3 * time.Hour),
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/test_downloading.zip",
			Filename:    "test_downloading.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			CreatedAt:   baseTime.Add(-2 * time.Hour),
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/test_pending.zip",
			Filename:    "test_pending.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			CreatedAt:   baseTime.Add(-1 * time.Hour),
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/test_paused.zip",
			Filename:    "test_paused.zip",
			Directory:   "/downloads",
			Status:      models.StatusPaused,
			CreatedAt:   baseTime,
			UpdatedAt:   baseTime,
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test status-based priority sorting with search (desc order)
	results, err := db.SearchDownloads("test", []string{"pending", "downloading", "completed", "paused"}, "desc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 4)

	// Verify status priority order is maintained
	require.Equal(t, models.StatusDownloading, results[0].Status)
	require.Equal(t, models.StatusPending, results[1].Status)
	require.Equal(t, models.StatusPaused, results[2].Status)
	require.Equal(t, models.StatusCompleted, results[3].Status)

	// Test with asc order
	results, err = db.SearchDownloads("test", []string{"pending", "downloading", "completed", "paused"}, "asc", 10, 0)
	require.NoError(t, err)
	require.Len(t, results, 4)

	// Status priority should still be maintained even with asc time order
	require.Equal(t, models.StatusDownloading, results[0].Status)
	require.Equal(t, models.StatusPending, results[1].Status)
	require.Equal(t, models.StatusPaused, results[2].Status)
	require.Equal(t, models.StatusCompleted, results[3].Status)
}

func TestDB_StatusBasedSortingInGetDownloadsByGroupID(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	groupID := "test-group"
	baseTime := time.Now()

	// Create downloads with different statuses in the same group
	downloads := []*models.Download{
		{
			OriginalURL: "https://example.com/group_completed.zip",
			Filename:    "group_completed.zip",
			Directory:   "/downloads",
			Status:      models.StatusCompleted,
			GroupID:     groupID,
			CreatedAt:   baseTime.Add(-3 * time.Hour),
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/group_downloading.zip",
			Filename:    "group_downloading.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			GroupID:     groupID,
			CreatedAt:   baseTime.Add(-2 * time.Hour),
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/group_pending.zip",
			Filename:    "group_pending.zip",
			Directory:   "/downloads",
			Status:      models.StatusPending,
			GroupID:     groupID,
			CreatedAt:   baseTime.Add(-1 * time.Hour),
			UpdatedAt:   baseTime,
		},
		{
			OriginalURL: "https://example.com/different_group.zip",
			Filename:    "different_group.zip",
			Directory:   "/downloads",
			Status:      models.StatusDownloading,
			GroupID:     "different-group",
			CreatedAt:   baseTime,
			UpdatedAt:   baseTime,
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	// Test status-based priority sorting within group
	results, err := db.GetDownloadsByGroupID(groupID)
	require.NoError(t, err)
	require.Len(t, results, 3)

	// Verify status priority order is maintained within the group
	require.Equal(t, models.StatusDownloading, results[0].Status)
	require.Equal(t, "group_downloading.zip", results[0].Filename)

	require.Equal(t, models.StatusPending, results[1].Status)
	require.Equal(t, "group_pending.zip", results[1].Filename)

	require.Equal(t, models.StatusCompleted, results[2].Status)
	require.Equal(t, "group_completed.zip", results[2].Filename)

	// Verify all results belong to the correct group
	for _, download := range results {
		require.Equal(t, groupID, download.GroupID)
	}
}
