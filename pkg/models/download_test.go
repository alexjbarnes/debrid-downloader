package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDownloadStatus_Constants(t *testing.T) {
	// Test that status constants have expected values
	require.Equal(t, DownloadStatus("pending"), StatusPending)
	require.Equal(t, DownloadStatus("downloading"), StatusDownloading)
	require.Equal(t, DownloadStatus("completed"), StatusCompleted)
	require.Equal(t, DownloadStatus("failed"), StatusFailed)
	require.Equal(t, DownloadStatus("paused"), StatusPaused)
}

func TestDownloadGroupStatus_Constants(t *testing.T) {
	// Test that group status constants have expected values
	require.Equal(t, DownloadGroupStatus("downloading"), GroupStatusDownloading)
	require.Equal(t, DownloadGroupStatus("processing"), GroupStatusProcessing)
	require.Equal(t, DownloadGroupStatus("completed"), GroupStatusCompleted)
	require.Equal(t, DownloadGroupStatus("failed"), GroupStatusFailed)
}

func TestDownload_Struct(t *testing.T) {
	now := time.Now()
	
	download := &Download{
		ID:              1,
		OriginalURL:     "https://example.com/file.zip",
		UnrestrictedURL: "https://debrid.com/file.zip",
		Filename:        "file.zip",
		Directory:       "/downloads",
		Status:          StatusPending,
		Progress:        0.0,
		FileSize:        1024000,
		DownloadedBytes: 0,
		DownloadSpeed:   0.0,
		ErrorMessage:    "",
		RetryCount:      0,
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       nil,
		CompletedAt:     nil,
		PausedAt:        nil,
		TotalPausedTime: 0,
		GroupID:         "group-123",
		IsArchive:       true,
		ExtractedFiles:  "[]",
	}

	// Test struct fields
	require.Equal(t, int64(1), download.ID)
	require.Equal(t, "https://example.com/file.zip", download.OriginalURL)
	require.Equal(t, "https://debrid.com/file.zip", download.UnrestrictedURL)
	require.Equal(t, "file.zip", download.Filename)
	require.Equal(t, "/downloads", download.Directory)
	require.Equal(t, StatusPending, download.Status)
	require.Equal(t, 0.0, download.Progress)
	require.Equal(t, int64(1024000), download.FileSize)
	require.Equal(t, int64(0), download.DownloadedBytes)
	require.Equal(t, 0.0, download.DownloadSpeed)
	require.Equal(t, "", download.ErrorMessage)
	require.Equal(t, 0, download.RetryCount)
	require.Equal(t, now, download.CreatedAt)
	require.Equal(t, now, download.UpdatedAt)
	require.Nil(t, download.StartedAt)
	require.Nil(t, download.CompletedAt)
	require.Nil(t, download.PausedAt)
	require.Equal(t, int64(0), download.TotalPausedTime)
	require.Equal(t, "group-123", download.GroupID)
	require.True(t, download.IsArchive)
	require.Equal(t, "[]", download.ExtractedFiles)
}

func TestDownload_JSONSerialization(t *testing.T) {
	now := time.Now()
	download := &Download{
		ID:              1,
		OriginalURL:     "https://example.com/file.zip",
		UnrestrictedURL: "https://debrid.com/file.zip",
		Filename:        "file.zip",
		Directory:       "/downloads",
		Status:          StatusCompleted,
		Progress:        100.0,
		FileSize:        1024000,
		DownloadedBytes: 1024000,
		DownloadSpeed:   1500.5,
		ErrorMessage:    "",
		RetryCount:      0,
		CreatedAt:       now,
		UpdatedAt:       now,
		StartedAt:       &now,
		CompletedAt:     &now,
		PausedAt:        nil,
		TotalPausedTime: 0,
		GroupID:         "",
		IsArchive:       false,
		ExtractedFiles:  "[]",
	}

	// Test JSON marshaling
	data, err := json.Marshal(download)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled Download
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify key fields
	require.Equal(t, download.ID, unmarshaled.ID)
	require.Equal(t, download.OriginalURL, unmarshaled.OriginalURL)
	require.Equal(t, download.Filename, unmarshaled.Filename)
	require.Equal(t, download.Status, unmarshaled.Status)
	require.Equal(t, download.Progress, unmarshaled.Progress)
	require.Equal(t, download.FileSize, unmarshaled.FileSize)
	require.Equal(t, download.IsArchive, unmarshaled.IsArchive)
}

func TestDirectoryMapping_Struct(t *testing.T) {
	now := time.Now()
	
	mapping := &DirectoryMapping{
		ID:              1,
		FilenamePattern: "*.zip",
		OriginalURL:     "https://example.com/archive.zip",
		Directory:       "/downloads/archives",
		UseCount:        5,
		LastUsed:        now,
		CreatedAt:       now,
	}

	// Test struct fields
	require.Equal(t, int64(1), mapping.ID)
	require.Equal(t, "*.zip", mapping.FilenamePattern)
	require.Equal(t, "https://example.com/archive.zip", mapping.OriginalURL)
	require.Equal(t, "/downloads/archives", mapping.Directory)
	require.Equal(t, 5, mapping.UseCount)
	require.Equal(t, now, mapping.LastUsed)
	require.Equal(t, now, mapping.CreatedAt)
}

func TestDirectoryMapping_JSONSerialization(t *testing.T) {
	now := time.Now()
	mapping := &DirectoryMapping{
		ID:              1,
		FilenamePattern: "movie",
		OriginalURL:     "https://example.com/movie.mp4",
		Directory:       "/downloads/movies",
		UseCount:        10,
		LastUsed:        now,
		CreatedAt:       now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(mapping)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled DirectoryMapping
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify fields
	require.Equal(t, mapping.ID, unmarshaled.ID)
	require.Equal(t, mapping.FilenamePattern, unmarshaled.FilenamePattern)
	require.Equal(t, mapping.OriginalURL, unmarshaled.OriginalURL)
	require.Equal(t, mapping.Directory, unmarshaled.Directory)
	require.Equal(t, mapping.UseCount, unmarshaled.UseCount)
}

func TestDownloadGroup_Struct(t *testing.T) {
	now := time.Now()
	
	group := &DownloadGroup{
		ID:                 "group-123",
		CreatedAt:          now,
		TotalDownloads:     3,
		CompletedDownloads: 2,
		Status:             GroupStatusDownloading,
		ProcessingError:    "",
	}

	// Test struct fields
	require.Equal(t, "group-123", group.ID)
	require.Equal(t, now, group.CreatedAt)
	require.Equal(t, 3, group.TotalDownloads)
	require.Equal(t, 2, group.CompletedDownloads)
	require.Equal(t, GroupStatusDownloading, group.Status)
	require.Equal(t, "", group.ProcessingError)
}

func TestDownloadGroup_JSONSerialization(t *testing.T) {
	now := time.Now()
	group := &DownloadGroup{
		ID:                 "test-group",
		CreatedAt:          now,
		TotalDownloads:     5,
		CompletedDownloads: 5,
		Status:             GroupStatusCompleted,
		ProcessingError:    "",
	}

	// Test JSON marshaling
	data, err := json.Marshal(group)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled DownloadGroup
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify fields
	require.Equal(t, group.ID, unmarshaled.ID)
	require.Equal(t, group.TotalDownloads, unmarshaled.TotalDownloads)
	require.Equal(t, group.CompletedDownloads, unmarshaled.CompletedDownloads)
	require.Equal(t, group.Status, unmarshaled.Status)
	require.Equal(t, group.ProcessingError, unmarshaled.ProcessingError)
}

func TestExtractedFile_Struct(t *testing.T) {
	now := time.Now()
	
	file := &ExtractedFile{
		ID:         1,
		DownloadID: 123,
		FilePath:   "/downloads/extracted/file.txt",
		CreatedAt:  now,
		DeletedAt:  nil,
	}

	// Test struct fields
	require.Equal(t, int64(1), file.ID)
	require.Equal(t, int64(123), file.DownloadID)
	require.Equal(t, "/downloads/extracted/file.txt", file.FilePath)
	require.Equal(t, now, file.CreatedAt)
	require.Nil(t, file.DeletedAt)

	// Test with deleted file
	file.DeletedAt = &now
	require.NotNil(t, file.DeletedAt)
	require.Equal(t, now, *file.DeletedAt)
}

func TestExtractedFile_JSONSerialization(t *testing.T) {
	now := time.Now()
	file := &ExtractedFile{
		ID:         1,
		DownloadID: 456,
		FilePath:   "/downloads/extracted/document.pdf",
		CreatedAt:  now,
		DeletedAt:  &now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(file)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Test JSON unmarshaling
	var unmarshaled ExtractedFile
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify fields
	require.Equal(t, file.ID, unmarshaled.ID)
	require.Equal(t, file.DownloadID, unmarshaled.DownloadID)
	require.Equal(t, file.FilePath, unmarshaled.FilePath)
	require.NotNil(t, unmarshaled.DeletedAt)
}

func TestDownloadStatus_StringValues(t *testing.T) {
	// Test that status values are strings
	require.Equal(t, "pending", string(StatusPending))
	require.Equal(t, "downloading", string(StatusDownloading))
	require.Equal(t, "completed", string(StatusCompleted))
	require.Equal(t, "failed", string(StatusFailed))
	require.Equal(t, "paused", string(StatusPaused))
}

func TestDownloadGroupStatus_StringValues(t *testing.T) {
	// Test that group status values are strings
	require.Equal(t, "downloading", string(GroupStatusDownloading))
	require.Equal(t, "processing", string(GroupStatusProcessing))
	require.Equal(t, "completed", string(GroupStatusCompleted))
	require.Equal(t, "failed", string(GroupStatusFailed))
}

func TestDownload_ZeroValues(t *testing.T) {
	// Test zero values
	var download Download
	require.Equal(t, int64(0), download.ID)
	require.Equal(t, "", download.OriginalURL)
	require.Equal(t, "", download.Filename)
	require.Equal(t, DownloadStatus(""), download.Status)
	require.Equal(t, 0.0, download.Progress)
	require.Equal(t, int64(0), download.FileSize)
	require.False(t, download.IsArchive)
}

func TestDownloadGroup_ZeroValues(t *testing.T) {
	// Test zero values
	var group DownloadGroup
	require.Equal(t, "", group.ID)
	require.Equal(t, 0, group.TotalDownloads)
	require.Equal(t, 0, group.CompletedDownloads)
	require.Equal(t, DownloadGroupStatus(""), group.Status)
	require.Equal(t, "", group.ProcessingError)
}