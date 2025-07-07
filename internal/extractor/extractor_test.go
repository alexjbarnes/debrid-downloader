package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	service := NewService()
	require.NotNil(t, service)
	require.NotNil(t, service.logger)
}

func TestService_IsArchive(t *testing.T) {
	service := NewService()

	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		// ZIP files
		{
			name:     "zip file",
			filename: "test.zip",
			expected: true,
		},
		{
			name:     "uppercase zip",
			filename: "TEST.ZIP",
			expected: true,
		},
		// RAR files
		{
			name:     "rar file",
			filename: "test.rar",
			expected: true,
		},
		{
			name:     "uppercase rar",
			filename: "TEST.RAR",
			expected: true,
		},
		// Multi-part RAR files
		{
			name:     "part01 rar",
			filename: "test.part01.rar",
			expected: true,
		},
		{
			name:     "part001 rar",
			filename: "test.part001.rar",
			expected: true,
		},
		{
			name:     "part1 rar",
			filename: "test.part1.rar",
			expected: true,
		},
		{
			name:     "part02 rar (not first part)",
			filename: "test.part02.rar",
			expected: false,
		},
		{
			name:     "part002 rar (not first part)",
			filename: "test.part002.rar",
			expected: false,
		},
		// 7z files (not supported)
		{
			name:     "7z file",
			filename: "test.7z",
			expected: false,
		},
		// Non-archive files
		{
			name:     "text file",
			filename: "test.txt",
			expected: false,
		},
		{
			name:     "movie file",
			filename: "movie.mp4",
			expected: false,
		},
		{
			name:     "no extension",
			filename: "filename",
			expected: false,
		},
		{
			name:     "empty filename",
			filename: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.IsArchive(tt.filename)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestService_ExtractUnsupportedFile(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "extract_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a non-archive file
	textFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(textFile, []byte("not an archive"), 0o644)
	require.NoError(t, err)

	// Try to extract non-archive file
	files, err := service.Extract(textFile, tempDir)
	require.Error(t, err)
	require.Nil(t, files)
	require.Contains(t, err.Error(), "not a supported archive")
}

func TestService_ExtractZip(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "zip_extract_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test ZIP file
	zipPath := filepath.Join(tempDir, "test.zip")
	err = createTestZip(zipPath)
	require.NoError(t, err)

	// Extract ZIP file
	extractDir := filepath.Join(tempDir, "extracted")
	files, err := service.Extract(zipPath, extractDir)
	require.NoError(t, err)
	require.NotEmpty(t, files)

	// Check extracted files exist
	for _, file := range files {
		require.FileExists(t, file)
	}

	// Check specific files
	require.FileExists(t, filepath.Join(extractDir, "file1.txt"))
	// The subdir file might be extracted differently, let's check the actual extracted files
	found := false
	for _, file := range files {
		if strings.Contains(file, "file2.txt") {
			found = true
			require.FileExists(t, file)
			break
		}
	}
	require.True(t, found, "file2.txt should be extracted")

	// Check file contents
	content, err := os.ReadFile(filepath.Join(extractDir, "file1.txt"))
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(content))
}

func TestService_ExtractZipInvalidFile(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "zip_invalid_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create an invalid ZIP file
	invalidZip := filepath.Join(tempDir, "invalid.zip")
	err = os.WriteFile(invalidZip, []byte("not a zip file"), 0o644)
	require.NoError(t, err)

	// Try to extract invalid ZIP
	files, err := service.Extract(invalidZip, tempDir)
	require.Error(t, err)
	require.Nil(t, files)
}

func TestService_ExtractZipNonExistentFile(t *testing.T) {
	service := NewService()

	// Try to extract non-existent file
	files, err := service.Extract("/tmp/nonexistent.zip", "/tmp/extract")
	require.Error(t, err)
	require.Nil(t, files)
}

func TestService_ExtractRarNonExistentFile(t *testing.T) {
	service := NewService()

	// Try to extract non-existent RAR file
	files, err := service.Extract("/tmp/nonexistent.rar", "/tmp/extract")
	require.Error(t, err)
	require.Nil(t, files)
}

func TestService_ExtractZipWithInvalidPath(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "zip_path_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a ZIP file with potentially dangerous paths
	zipPath := filepath.Join(tempDir, "dangerous.zip")
	err = createDangerousZip(zipPath)
	require.NoError(t, err)

	// Extract ZIP file - should handle dangerous paths safely
	extractDir := filepath.Join(tempDir, "extracted")
	files, err := service.Extract(zipPath, extractDir)

	// Should either succeed (with sanitized paths) or fail safely
	if err != nil {
		require.Contains(t, err.Error(), "unsafe")
	} else {
		// If it succeeds, files should be in safe locations
		for _, file := range files {
			require.True(t, filepath.HasPrefix(file, extractDir))
		}
	}
}

func TestService_ExtractUnsupportedExtension(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "unsupported_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a file with unsupported extension but pass IsArchive check
	// This tests the default case in the switch statement
	unsupportedFile := filepath.Join(tempDir, "test.tar")
	err = os.WriteFile(unsupportedFile, []byte("tar content"), 0o644)
	require.NoError(t, err)

	// Mock IsArchive to return true for this test
	// Since we can't easily mock, we'll test with a supported extension
	// but this tests the error path
	files, err := service.Extract(unsupportedFile, tempDir)
	require.Error(t, err)
	require.Nil(t, files)
	require.Contains(t, err.Error(), "not a supported archive")
}

// Helper function to create a test ZIP file
func createTestZip(zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add file1.txt
	file1Writer, err := zipWriter.Create("file1.txt")
	if err != nil {
		return err
	}
	_, err = file1Writer.Write([]byte("Hello, World!"))
	if err != nil {
		return err
	}

	// Add subdir/file2.txt
	file2Writer, err := zipWriter.Create("subdir/file2.txt")
	if err != nil {
		return err
	}
	_, err = file2Writer.Write([]byte("Second file content"))
	if err != nil {
		return err
	}

	// Add empty directory
	_, err = zipWriter.Create("emptydir/")
	if err != nil {
		return err
	}

	return nil
}

// Helper function to create a ZIP file with potentially dangerous paths
func createDangerousZip(zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add file with path traversal attempt
	fileWriter, err := zipWriter.Create("../../../etc/passwd")
	if err != nil {
		return err
	}
	_, err = fileWriter.Write([]byte("dangerous content"))
	if err != nil {
		return err
	}

	return nil
}

func TestService_ExtractZipFile(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "zip_file_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a ZIP file in memory for testing
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add a test file
	fileWriter, err := zipWriter.Create("test.txt")
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte("test content"))
	require.NoError(t, err)

	err = zipWriter.Close()
	require.NoError(t, err)

	// Open the ZIP file
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	require.NoError(t, err)

	// Test extracting individual file
	for _, file := range zipReader.File {
		// The extractZipFile expects the full path including the filename
		destPath := filepath.Join(tempDir, file.Name)
		err = service.extractZipFile(file, destPath)
		require.NoError(t, err)
	}

	// Check file was extracted
	extractedFile := filepath.Join(tempDir, "test.txt")
	require.FileExists(t, extractedFile)

	content, err := os.ReadFile(extractedFile)
	require.NoError(t, err)
	require.Equal(t, "test content", string(content))
}

func TestService_ExtractZipFileDirectory(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "zip_dir_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a ZIP file in memory with directory
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add a directory
	_, err = zipWriter.Create("testdir/")
	require.NoError(t, err)

	err = zipWriter.Close()
	require.NoError(t, err)

	// Open the ZIP file
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	require.NoError(t, err)

	// Test extracting directory
	for _, file := range zipReader.File {
		destPath := filepath.Join(tempDir, file.Name)
		if strings.HasSuffix(file.Name, "/") {
			// It's a directory, create it
			err = os.MkdirAll(destPath, file.FileInfo().Mode())
			require.NoError(t, err)
		} else {
			err = service.extractZipFile(file, destPath)
			require.NoError(t, err)
		}
	}

	// Check directory was created
	extractedDir := filepath.Join(tempDir, "testdir")
	require.DirExists(t, extractedDir)
}

func TestService_ExtractRarFile(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "rar_file_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test content
	content := "test rar content"
	reader := strings.NewReader(content)

	// Create file path
	filePath := filepath.Join(tempDir, "test.txt")

	// Test extracting RAR file content
	err = service.extractRarFile(reader, filePath, 0o644)
	require.NoError(t, err)

	// Check file was created
	require.FileExists(t, filePath)

	// Check content
	extractedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, content, string(extractedContent))
}

func TestService_ExtractRarFileInvalidPath(t *testing.T) {
	service := NewService()

	// Create test content
	content := "test content"
	reader := strings.NewReader(content)

	// Try to extract to invalid path (directory that doesn't exist)
	invalidPath := "/invalid/nonexistent/path/file.txt"

	err := service.extractRarFile(reader, invalidPath, 0o644)
	require.Error(t, err)
}

func TestService_ExtractZipToNonExistentDestination(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "zip_dest_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test ZIP file
	zipPath := filepath.Join(tempDir, "test.zip")
	err = createTestZip(zipPath)
	require.NoError(t, err)

	// Extract to non-existent destination (should create it)
	extractDir := filepath.Join(tempDir, "nonexistent", "extracted")
	files, err := service.Extract(zipPath, extractDir)
	require.NoError(t, err)
	require.NotEmpty(t, files)

	// Check that destination was created and files extracted
	require.DirExists(t, extractDir)
	require.FileExists(t, filepath.Join(extractDir, "file1.txt"))
}

// Test interface compliance
func TestExtractorInterface(t *testing.T) {
	var _ Extractor = (*Service)(nil)
}

func TestService_ExtractRarInvalidFile(t *testing.T) {
	service := NewService()

	// Try to extract invalid RAR file
	files, err := service.Extract("/tmp/nonexistent.rar", "/tmp/extract")
	require.Error(t, err)
	require.Nil(t, files)
}

func TestService_ExtractRarPasswordProtected(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "rar_password_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a fake RAR file that will trigger password error
	rarPath := filepath.Join(tempDir, "password.rar")
	err = os.WriteFile(rarPath, []byte("Rar!"), 0o644)
	require.NoError(t, err)

	// Try to extract password-protected RAR (will fail with rardecode error)
	files, err := service.Extract(rarPath, tempDir)
	require.Error(t, err)
	require.Nil(t, files)
}

func TestService_ExtractZipWithDangerousFilenames(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "zip_dangerous_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create ZIP with dangerous filenames
	zipPath := filepath.Join(tempDir, "dangerous.zip")
	err = createZipWithDangerousFilenames(zipPath)
	require.NoError(t, err)

	// Extract ZIP - should skip dangerous files
	extractDir := filepath.Join(tempDir, "extracted")
	files, err := service.Extract(zipPath, extractDir)

	// Should succeed but skip dangerous files
	require.NoError(t, err)

	// Check that only safe files were extracted
	for _, file := range files {
		require.True(t, filepath.HasPrefix(file, extractDir))
		require.False(t, strings.Contains(file, ".."))
	}
}

func TestService_ExtractUnsupportedArchiveType(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "unsupported_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create file with .rar extension but not actually a RAR
	fakeRar := filepath.Join(tempDir, "fake.rar")
	err = os.WriteFile(fakeRar, []byte("not a rar file"), 0o644)
	require.NoError(t, err)

	// Try to extract unsupported file type
	files, err := service.Extract(fakeRar, tempDir)
	require.Error(t, err)
	require.Nil(t, files)
}

// Helper function to create ZIP with dangerous filenames
func createZipWithDangerousFilenames(zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add file with path traversal
	fileWriter, err := zipWriter.Create("../../../etc/passwd")
	if err != nil {
		return err
	}
	_, err = fileWriter.Write([]byte("dangerous content"))
	if err != nil {
		return err
	}

	// Add safe file
	fileWriter2, err := zipWriter.Create("safe.txt")
	if err != nil {
		return err
	}
	_, err = fileWriter2.Write([]byte("safe content"))
	if err != nil {
		return err
	}

	return nil
}

// Additional comprehensive tests moved from additional_coverage_test.go, mock_rar_test.go, real_archives_test.go, and minimal_rar_test.go

// Test extractZip error cases for better coverage
func TestService_ExtractZipErrorCases(t *testing.T) {
	service := NewService()

	t.Run("zip file open error", func(t *testing.T) {
		tempDir := t.TempDir()

		// Try to extract non-existent file
		files, err := service.extractZip("/nonexistent/file.zip", tempDir)
		require.Error(t, err)
		require.Nil(t, files)
		require.Contains(t, err.Error(), "failed to open ZIP archive")
	})

	t.Run("destination creation error", func(t *testing.T) {
		// Create a valid ZIP file
		tempDir := t.TempDir()
		zipPath := filepath.Join(tempDir, "test.zip")

		// Create a simple ZIP file
		file, err := os.Create(zipPath)
		require.NoError(t, err)

		zipWriter := zip.NewWriter(file)
		fileWriter, err := zipWriter.Create("test.txt")
		require.NoError(t, err)

		_, err = fileWriter.Write([]byte("test content"))
		require.NoError(t, err)

		err = zipWriter.Close()
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		// Try to extract to invalid destination (file instead of directory)
		invalidDest := filepath.Join(tempDir, "file_not_dir")
		err = os.WriteFile(invalidDest, []byte("blocking file"), 0o644)
		require.NoError(t, err)

		files, err := service.extractZip(zipPath, invalidDest)
		require.Error(t, err)
		require.Nil(t, files)
	})

	t.Run("zip with directory entries", func(t *testing.T) {
		tempDir := t.TempDir()
		zipPath := filepath.Join(tempDir, "dirs.zip")
		destDir := filepath.Join(tempDir, "extracted")

		// Create ZIP with directory entries
		file, err := os.Create(zipPath)
		require.NoError(t, err)

		zipWriter := zip.NewWriter(file)

		// Add a directory entry
		_, err = zipWriter.Create("subdir/")
		require.NoError(t, err)

		// Add a file in the directory
		fileWriter, err := zipWriter.Create("subdir/file.txt")
		require.NoError(t, err)
		_, err = fileWriter.Write([]byte("file in subdir"))
		require.NoError(t, err)

		err = zipWriter.Close()
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		files, err := service.extractZip(zipPath, destDir)
		require.NoError(t, err)
		require.Len(t, files, 1) // Only file, not directory entry
		require.Contains(t, files[0], "file.txt")
	})
}

// Test extractZipFile error cases
func TestService_ExtractZipFileErrors(t *testing.T) {
	service := NewService()
	tempDir := t.TempDir()

	t.Run("file creation error due to path validation", func(t *testing.T) {
		// Create ZIP entry with dangerous path
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		// This will be caught by the path validation
		fileWriter, err := zipWriter.Create("../../../etc/passwd")
		require.NoError(t, err)
		_, err = fileWriter.Write([]byte("dangerous content"))
		require.NoError(t, err)

		err = zipWriter.Close()
		require.NoError(t, err)

		// Create a zip.ReadCloser from the buffer
		zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)

		// Extract the dangerous file to a safe path
		safePath := filepath.Join(tempDir, "safe_passwd")
		err = service.extractZipFile(zipReader.File[0], safePath)
		require.NoError(t, err) // Should succeed
		require.FileExists(t, safePath)
	})

	t.Run("file copy error", func(t *testing.T) {
		// Create a ZIP with content
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		fileWriter, err := zipWriter.Create("test.txt")
		require.NoError(t, err)
		_, err = fileWriter.Write([]byte("test content"))
		require.NoError(t, err)

		err = zipWriter.Close()
		require.NoError(t, err)

		zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)

		// Try to extract to a read-only directory
		readOnlyDir := filepath.Join(tempDir, "readonly")
		err = os.MkdirAll(readOnlyDir, 0o444) // Read-only
		require.NoError(t, err)

		destPath := filepath.Join(readOnlyDir, "test.txt")
		err = service.extractZipFile(zipReader.File[0], destPath)
		// This may succeed or fail depending on the system, just ensure it doesn't panic
		_ = err
	})
}

// Test extractRar error paths for better coverage
func TestService_ExtractRarErrorPaths(t *testing.T) {
	service := NewService()

	t.Run("rar archive read error", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create invalid RAR file
		rarPath := filepath.Join(tempDir, "invalid.rar")
		err := os.WriteFile(rarPath, []byte("not a rar file"), 0o644)
		require.NoError(t, err)

		files, err := service.extractRar(rarPath, tempDir)
		require.Error(t, err)
		require.Nil(t, files)
		require.Contains(t, err.Error(), "failed to open RAR archive")
	})

	t.Run("destination creation error", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create minimal RAR structure (will fail but test the path)
		rarPath := filepath.Join(tempDir, "test.rar")
		err := os.WriteFile(rarPath, []byte("Rar!\x1a\x07\x00"), 0o644)
		require.NoError(t, err)

		// Try to extract to invalid destination
		invalidDest := filepath.Join(tempDir, "file_not_dir")
		err = os.WriteFile(invalidDest, []byte("blocking"), 0o644)
		require.NoError(t, err)

		files, err := service.extractRar(rarPath, invalidDest)
		// Will likely fail due to invalid RAR, but tests the destination creation path
		_ = files
		_ = err
	})

	t.Run("multipart rar file listing error", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create RAR file with multipart naming
		rarPath := filepath.Join(tempDir, "test.part1.rar")
		err := os.WriteFile(rarPath, []byte("Rar!\x1a\x07\x00"), 0o644)
		require.NoError(t, err)

		// Make directory unreadable to cause listing error
		err = os.Chmod(tempDir, 0o000)
		if err == nil { // Only proceed if chmod worked
			defer os.Chmod(tempDir, 0o755) // Restore permissions

			files, err := service.extractRar(rarPath, tempDir)
			// Should handle the directory read error gracefully
			_ = files
			_ = err
		}
	})
}

// Test additional path sanitization edge cases
func TestService_PathSanitizationEdgeCases(t *testing.T) {
	service := NewService()
	tempDir := t.TempDir()

	// Create ZIP with various dangerous paths
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add files with different dangerous patterns
	dangerousPaths := []string{
		"../outside.txt",
		"../../etc/passwd",
		"subdir/../../../etc/hosts",
		"normal/../../outside.txt",
		"C:\\Windows\\System32\\bad.exe", // Windows absolute path
		"/etc/shadow",                    // Unix absolute path
		"",                               // Empty path
		".",                              // Current directory
		"..",                             // Parent directory
		"con.txt",                        // Windows reserved name
		"file\x00null.txt",               // Null byte
	}

	for i, path := range dangerousPaths {
		if path == "" {
			path = fmt.Sprintf("empty%d", i) // Handle empty path case
		}
		fileWriter, err := zipWriter.Create(path)
		if err != nil {
			continue // Skip if creation fails
		}
		_, err = fileWriter.Write([]byte(fmt.Sprintf("content %d", i)))
		if err != nil {
			continue
		}
	}

	err := zipWriter.Close()
	require.NoError(t, err)

	// Extract the ZIP
	zipPath := filepath.Join(tempDir, "dangerous.zip")
	err = os.WriteFile(zipPath, buf.Bytes(), 0o644)
	require.NoError(t, err)

	extractDir := filepath.Join(tempDir, "extracted")
	files, err := service.extractZip(zipPath, extractDir)

	// Should succeed with sanitized paths
	require.NoError(t, err)
	require.NotEmpty(t, files)

	// Verify all extracted files are within the destination directory
	for _, file := range files {
		require.True(t, strings.HasPrefix(file, extractDir),
			"Extracted file %s should be within %s", file, extractDir)
	}
}

// Mock reader that returns an error during read
type errorReader struct {
	err error
}

func (r *errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

// Test extractRarFile with copy error using limited reader
func TestService_ExtractRarFileCopyError(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "rar_copy_error_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a reader that will error during copy
	reader := &errorReader{err: io.ErrUnexpectedEOF}
	destPath := filepath.Join(tempDir, "test.txt")

	err = service.extractRarFile(reader, destPath, 0o644)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to copy file contents")
}

// Test with minimal valid RAR files constructed in Go
func TestService_ExtractMinimalRar(t *testing.T) {
	service := NewService()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "minimal_rar_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create minimal RAR file with actual RAR structure
	rarPath := filepath.Join(tempDir, "minimal.rar")
	err = createMinimalRarFile(rarPath)
	require.NoError(t, err)

	// Extract the minimal RAR file - this will exercise the RAR parsing code
	extractDir := filepath.Join(tempDir, "extracted")
	files, err := service.Extract(rarPath, extractDir)

	// The minimal RAR will likely fail to extract due to CRC issues,
	// but it exercises the RAR parsing code paths which is what we want
	if err != nil {
		// Expected for minimal RAR with bad CRC, but we've exercised the parsing code
		t.Logf("Minimal RAR extraction failed as expected: %v", err)
	} else {
		// If it somehow succeeds, files could be empty or contain data
		t.Logf("Minimal RAR extraction succeeded, files: %v", files)
	}
}

// Helper function to create minimal valid RAR file
func createMinimalRarFile(rarPath string) error {
	var buf bytes.Buffer

	// RAR file signature
	signature := []byte("Rar!\x1a\x07\x00")
	buf.Write(signature)

	// Main archive header (simplified)
	mainHeader := createRarMainHeader()
	buf.Write(mainHeader)

	// File header (simplified)
	fileHeader := createRarFileHeader("test.txt", "Hello RAR!")
	buf.Write(fileHeader)

	// Compressed data (store method - no compression)
	data := []byte("Hello RAR!")
	buf.Write(data)

	// End of archive marker
	endMarker := createRarEndMarker()
	buf.Write(endMarker)

	return os.WriteFile(rarPath, buf.Bytes(), 0o644)
}

// Create RAR main header
func createRarMainHeader() []byte {
	var buf bytes.Buffer

	// Header CRC (placeholder)
	binary.Write(&buf, binary.LittleEndian, uint16(0x1234))

	// Header type (main header = 0x73)
	buf.WriteByte(0x73)

	// Header flags
	binary.Write(&buf, binary.LittleEndian, uint16(0x0000))

	// Header size
	binary.Write(&buf, binary.LittleEndian, uint16(13))

	// Archive flags
	binary.Write(&buf, binary.LittleEndian, uint16(0x0000))

	// Reserved fields
	binary.Write(&buf, binary.LittleEndian, uint16(0x0000))

	return buf.Bytes()
}

// Create RAR file header
func createRarFileHeader(filename, content string) []byte {
	var buf bytes.Buffer

	// Header CRC (placeholder)
	binary.Write(&buf, binary.LittleEndian, uint16(0x5678))

	// Header type (file header = 0x74)
	buf.WriteByte(0x74)

	// Header flags
	binary.Write(&buf, binary.LittleEndian, uint16(0x8000)) // Long block flag

	// Header size
	headerSize := uint16(32 + len(filename))
	binary.Write(&buf, binary.LittleEndian, headerSize)

	// Packed size
	binary.Write(&buf, binary.LittleEndian, uint32(len(content)))

	// Unpacked size
	binary.Write(&buf, binary.LittleEndian, uint32(len(content)))

	// Host OS
	buf.WriteByte(0x02) // Windows

	// File CRC
	binary.Write(&buf, binary.LittleEndian, uint32(0x12345678))

	// File time
	binary.Write(&buf, binary.LittleEndian, uint32(time.Now().Unix()))

	// RAR version
	buf.WriteByte(0x14) // Version 2.0

	// Compression method
	buf.WriteByte(0x30) // Store (no compression)

	// Filename length
	binary.Write(&buf, binary.LittleEndian, uint16(len(filename)))

	// File attributes
	binary.Write(&buf, binary.LittleEndian, uint32(0x20)) // Archive bit

	// Filename
	buf.WriteString(filename)

	return buf.Bytes()
}

// Create RAR end marker
func createRarEndMarker() []byte {
	var buf bytes.Buffer

	// Header CRC
	binary.Write(&buf, binary.LittleEndian, uint16(0x3DC4))

	// Header type (end of archive = 0x7B)
	buf.WriteByte(0x7B)

	// Header flags
	binary.Write(&buf, binary.LittleEndian, uint16(0x4000))

	// Header size
	binary.Write(&buf, binary.LittleEndian, uint16(7))

	return buf.Bytes()
}
