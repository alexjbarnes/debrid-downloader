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
