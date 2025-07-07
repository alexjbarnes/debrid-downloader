package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"debrid-downloader/internal/database"
	"debrid-downloader/pkg/models"

	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db, "/tmp/test")
	require.NotNil(t, service)
	require.Equal(t, db, service.db)
	require.Equal(t, "/tmp/test", service.baseDownloadPath)
	require.NotNil(t, service.logger)
}

func TestService_CleanupExtractedFiles(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "cleanup_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(db, tempDir)

	// Create a download record
	download := &models.Download{
		OriginalURL: "https://example.com/archive.zip",
		Filename:    "archive.zip",
		Directory:   tempDir,
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Create test files
	videoFile := filepath.Join(tempDir, "movie.mp4")
	textFile := filepath.Join(tempDir, "readme.txt")
	unknownFile := filepath.Join(tempDir, "unknown.xyz")

	err = os.WriteFile(videoFile, []byte("video content"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(textFile, []byte("text content"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(unknownFile, []byte("unknown content"), 0o644)
	require.NoError(t, err)

	// Create extracted file records
	extractedFiles := []*models.ExtractedFile{
		{
			DownloadID: download.ID,
			FilePath:   videoFile,
			CreatedAt:  time.Now(),
		},
		{
			DownloadID: download.ID,
			FilePath:   textFile,
			CreatedAt:  time.Now(),
		},
		{
			DownloadID: download.ID,
			FilePath:   unknownFile,
			CreatedAt:  time.Now(),
		},
	}

	for _, file := range extractedFiles {
		err = db.CreateExtractedFile(file)
		require.NoError(t, err)
	}

	// Run cleanup
	err = service.CleanupExtractedFiles(download.ID)
	require.NoError(t, err)

	// Check results
	require.FileExists(t, videoFile)   // Video files should be kept
	require.NoFileExists(t, textFile)  // Text files should be deleted
	require.FileExists(t, unknownFile) // Unknown extensions should be kept
}

func TestService_CleanupExtractedFilesNoFiles(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db, "/tmp/test")

	// Test with non-existent download ID
	err = service.CleanupExtractedFiles(999)
	require.NoError(t, err)
}

func TestService_CleanupExtractedFilesDatabaseError(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	db.Close() // Close to trigger error

	service := NewService(db, "/tmp/test")

	err = service.CleanupExtractedFiles(1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get extracted files")
}

func TestService_IsPathSafe(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "path_safe_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(nil, tempDir)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "safe subdirectory",
			path:     filepath.Join(tempDir, "subdir", "file.txt"),
			expected: true,
		},
		{
			name:     "base directory itself",
			path:     tempDir,
			expected: false,
		},
		{
			name:     "file in base directory",
			path:     filepath.Join(tempDir, "file.txt"),
			expected: true,
		},
		{
			name:     "outside base directory",
			path:     "/etc/passwd",
			expected: false,
		},
		{
			name:     "relative path outside",
			path:     "../../../etc/passwd",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isPathSafe(tt.path)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestService_ShouldCleanupFile(t *testing.T) {
	service := NewService(nil, "/tmp/test")

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		// Video files (should keep)
		{
			name:     "mp4 video",
			filePath: "/path/to/movie.mp4",
			expected: false,
		},
		{
			name:     "mkv video",
			filePath: "/path/to/movie.mkv",
			expected: false,
		},
		{
			name:     "avi video",
			filePath: "/path/to/movie.avi",
			expected: false,
		},
		// Cleanup files (should delete)
		{
			name:     "text file",
			filePath: "/path/to/readme.txt",
			expected: true,
		},
		{
			name:     "nfo file",
			filePath: "/path/to/movie.nfo",
			expected: true,
		},
		{
			name:     "jpg image",
			filePath: "/path/to/poster.jpg",
			expected: true,
		},
		{
			name:     "srt subtitle",
			filePath: "/path/to/movie.srt",
			expected: true,
		},
		// Unknown files (should keep)
		{
			name:     "unknown extension",
			filePath: "/path/to/file.xyz",
			expected: false,
		},
		{
			name:     "no extension",
			filePath: "/path/to/file",
			expected: false,
		},
		// Case insensitive
		{
			name:     "uppercase video",
			filePath: "/path/to/MOVIE.MP4",
			expected: false,
		},
		{
			name:     "uppercase cleanup",
			filePath: "/path/to/README.TXT",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldCleanupFile(tt.filePath)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestService_DeleteFile(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	tempDir, err := os.MkdirTemp("", "delete_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(db, tempDir)

	// Create a download record
	download := &models.Download{
		OriginalURL: "https://example.com/archive.zip",
		Filename:    "archive.zip",
		Directory:   tempDir,
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	// Create extracted file record
	extractedFile := &models.ExtractedFile{
		DownloadID: download.ID,
		FilePath:   testFile,
		CreatedAt:  time.Now(),
	}
	err = db.CreateExtractedFile(extractedFile)
	require.NoError(t, err)

	// Delete the file
	err = service.deleteFile(extractedFile, download.ID)
	require.NoError(t, err)

	// Check file is deleted
	require.NoFileExists(t, testFile)
}

func TestService_DeleteFileNonExistent(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db, "/tmp/test")

	// Create extracted file record for non-existent file
	extractedFile := &models.ExtractedFile{
		ID:         1,
		DownloadID: 1,
		FilePath:   "/tmp/test/nonexistent.txt",
		CreatedAt:  time.Now(),
	}

	// Should handle non-existent file gracefully
	err = service.deleteFile(extractedFile, 1)
	require.NoError(t, err)
}

func TestService_GetFileSize(t *testing.T) {
	service := NewService(nil, "/tmp/test")

	// Test with non-existent file
	size := service.getFileSize("/tmp/nonexistent.txt")
	require.Equal(t, int64(0), size)

	// Test with real file
	tempFile, err := os.CreateTemp("", "size_test")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	content := "test content"
	_, err = tempFile.WriteString(content)
	require.NoError(t, err)
	tempFile.Close()

	size = service.getFileSize(tempFile.Name())
	require.Equal(t, int64(len(content)), size)
}

func TestService_CleanupEmptyDirectories(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create base directory within /tmp to ensure it's safe
	baseDir := "/tmp/test_cleanup"
	err = os.MkdirAll(baseDir, 0o755)
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	tempDir, err := os.MkdirTemp(baseDir, "empty_dir_test")
	require.NoError(t, err)

	service := NewService(db, baseDir)

	// Create directory structure within tempDir
	subDir1 := filepath.Join(tempDir, "subdir1")
	subDir2 := filepath.Join(tempDir, "subdir2")
	subDir3 := filepath.Join(tempDir, "subdir3")

	err = os.MkdirAll(subDir1, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(subDir2, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(subDir3, 0o755)
	require.NoError(t, err)

	// Add a file to subdir2 to make it non-empty
	testFile := filepath.Join(subDir2, "keepme.txt")
	err = os.WriteFile(testFile, []byte("content"), 0o644)
	require.NoError(t, err)

	// Run cleanup on tempDir (not baseDir)
	err = service.CleanupEmptyDirectories(1, tempDir)
	require.NoError(t, err)

	// Check results
	require.NoDirExists(t, subDir1) // Empty, should be removed
	require.DirExists(t, subDir2)   // Has file, should remain
	require.NoDirExists(t, subDir3) // Empty, should be removed
	require.DirExists(t, tempDir)   // Root should remain
}

func TestService_CleanupEmptyDirectoriesUnsafePath(t *testing.T) {
	service := NewService(nil, "/tmp/test")

	err := service.CleanupEmptyDirectories(1, "/etc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsafe path")
}

func TestService_IsDirectoryEmpty(t *testing.T) {
	service := NewService(nil, "/tmp/test")

	// Test with non-existent directory
	empty := service.isDirectoryEmpty("/tmp/nonexistent")
	require.False(t, empty)

	// Test with empty directory
	tempDir, err := os.MkdirTemp("", "empty_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	empty = service.isDirectoryEmpty(tempDir)
	require.True(t, empty)

	// Test with non-empty directory
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0o644)
	require.NoError(t, err)

	empty = service.isDirectoryEmpty(tempDir)
	require.False(t, empty)
}

func TestService_GetCleanupStats(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	tempDir, err := os.MkdirTemp("", "stats_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(db, tempDir)

	// Create a download record
	download := &models.Download{
		OriginalURL: "https://example.com/archive.zip",
		Filename:    "archive.zip",
		Directory:   tempDir,
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Create test files
	videoFile := filepath.Join(tempDir, "movie.mp4")
	textFile := filepath.Join(tempDir, "readme.txt")
	unknownFile := filepath.Join(tempDir, "unknown.xyz")
	unsafeFile := "/etc/passwd"

	err = os.WriteFile(videoFile, []byte("video content"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(textFile, []byte("text content"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(unknownFile, []byte("unknown content"), 0o644)
	require.NoError(t, err)

	// Create extracted file records
	extractedFiles := []*models.ExtractedFile{
		{
			DownloadID: download.ID,
			FilePath:   videoFile,
			CreatedAt:  time.Now(),
		},
		{
			DownloadID: download.ID,
			FilePath:   textFile,
			CreatedAt:  time.Now(),
		},
		{
			DownloadID: download.ID,
			FilePath:   unknownFile,
			CreatedAt:  time.Now(),
		},
		{
			DownloadID: download.ID,
			FilePath:   unsafeFile,
			CreatedAt:  time.Now(),
		},
	}

	for _, file := range extractedFiles {
		err = db.CreateExtractedFile(file)
		require.NoError(t, err)
	}

	// Get stats
	stats, err := service.GetCleanupStats(download.ID)
	require.NoError(t, err)
	require.NotNil(t, stats)

	require.Equal(t, 4, stats.TotalFiles)
	require.Equal(t, 1, stats.VideoFiles)
	require.Equal(t, 1, stats.CleanupFiles)
	require.Equal(t, 1, stats.UnknownFiles)
	require.Equal(t, 1, stats.UnsafeFiles)
	require.Greater(t, stats.TotalSize, int64(0))
	require.Greater(t, stats.CleanupSize, int64(0))
}

func TestService_GetCleanupStatsNoFiles(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	service := NewService(db, "/tmp/test")

	stats, err := service.GetCleanupStats(999)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, 0, stats.TotalFiles)
}

func TestService_GetCleanupStatsDatabaseError(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	db.Close() // Close to trigger error

	service := NewService(db, "/tmp/test")

	stats, err := service.GetCleanupStats(1)
	require.Error(t, err)
	require.Nil(t, stats)
	require.Contains(t, err.Error(), "failed to get extracted files")
}

func TestVideoExtensions(t *testing.T) {
	// Test that common video extensions are included
	expectedExtensions := []string{".mp4", ".mkv", ".avi", ".mov"}
	for _, ext := range expectedExtensions {
		require.Contains(t, VideoExtensions, ext)
	}
}

func TestCleanupExtensions(t *testing.T) {
	// Test that common cleanup extensions are included
	expectedExtensions := []string{".txt", ".nfo", ".jpg", ".srt"}
	for _, ext := range expectedExtensions {
		require.Contains(t, CleanupExtensions, ext)
	}
}

func TestCleanupStatsStruct(t *testing.T) {
	stats := &CleanupStats{
		TotalFiles:   10,
		VideoFiles:   5,
		CleanupFiles: 3,
		UnknownFiles: 1,
		UnsafeFiles:  1,
		TotalSize:    1024,
		CleanupSize:  512,
	}

	require.Equal(t, 10, stats.TotalFiles)
	require.Equal(t, 5, stats.VideoFiles)
	require.Equal(t, 3, stats.CleanupFiles)
	require.Equal(t, 1, stats.UnknownFiles)
	require.Equal(t, 1, stats.UnsafeFiles)
	require.Equal(t, int64(1024), stats.TotalSize)
	require.Equal(t, int64(512), stats.CleanupSize)
}

func TestService_CleanupExtractedFilesWithUnsafeFiles(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	tempDir, err := os.MkdirTemp("", "unsafe_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(db, tempDir)

	// Create a download record
	download := &models.Download{
		OriginalURL: "https://example.com/archive.zip",
		Filename:    "archive.zip",
		Directory:   tempDir,
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = db.CreateDownload(download)
	require.NoError(t, err)

	// Create extracted file record with unsafe path
	extractedFile := &models.ExtractedFile{
		DownloadID: download.ID,
		FilePath:   "/etc/passwd", // Unsafe path
		CreatedAt:  time.Now(),
	}
	err = db.CreateExtractedFile(extractedFile)
	require.NoError(t, err)

	// Run cleanup - should skip unsafe files
	err = service.CleanupExtractedFiles(download.ID)
	require.NoError(t, err)

	// /etc/passwd should still exist (we didn't actually try to delete it)
	require.FileExists(t, "/etc/passwd")
}
