package database

import (
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
