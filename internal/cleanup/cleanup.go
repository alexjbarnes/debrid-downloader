// Package cleanup provides file cleanup functionality for removing non-video files after extraction
package cleanup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"debrid-downloader/internal/database"
	"debrid-downloader/pkg/models"
)

// VideoExtensions defines file extensions that should be kept (video files)
var VideoExtensions = []string{
	".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg",
	".3gp", ".divx", ".xvid", ".asf", ".rm", ".rmvb", ".ts", ".mts", ".m2ts", ".ogv", ".ogg",
}

// CleanupExtensions defines file extensions that should be removed (non-video files)
var CleanupExtensions = []string{
	".txt", ".nfo", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".srt", ".sub", ".idx", ".vtt",
	".ass", ".ssa", ".smi", ".rt", ".sbv", ".dfxp", ".ttml", ".xml", ".log", ".diz", ".sfv",
}

// Service provides file cleanup services
type Service struct {
	db               *database.DB
	logger           *slog.Logger
	baseDownloadPath string
}

// NewService creates a new cleanup service
func NewService(db *database.DB, baseDownloadPath string) *Service {
	return &Service{
		db:               db,
		logger:           slog.Default(),
		baseDownloadPath: baseDownloadPath,
	}
}

// CleanupExtractedFiles safely removes non-video files from extracted archives
func (s *Service) CleanupExtractedFiles(downloadID int64) error {
	s.logger.Info("Starting cleanup for extracted files", "download_id", downloadID)

	// Get list of extracted files for this download
	extractedFiles, err := s.db.GetExtractedFilesByDownloadID(downloadID)
	if err != nil {
		return fmt.Errorf("failed to get extracted files: %w", err)
	}

	if len(extractedFiles) == 0 {
		s.logger.Info("No extracted files found for cleanup", "download_id", downloadID)
		return nil
	}

	s.logger.Info("Found extracted files for cleanup", "download_id", downloadID, "file_count", len(extractedFiles))

	var deletedFiles []string
	var errors []string

	for _, extractedFile := range extractedFiles {
		// Validate file path is within allowed directories
		if !s.isPathSafe(extractedFile.FilePath) {
			s.logger.Warn("Skipping file outside safe path", "file", extractedFile.FilePath)
			continue
		}

		// Check if file should be cleaned up
		if s.shouldCleanupFile(extractedFile.FilePath) {
			if err := s.deleteFile(extractedFile, downloadID); err != nil {
				s.logger.Warn("Failed to delete file", "file", extractedFile.FilePath, "error", err)
				errors = append(errors, fmt.Sprintf("%s: %s", extractedFile.FilePath, err.Error()))
			} else {
				deletedFiles = append(deletedFiles, extractedFile.FilePath)
				s.logger.Info("Deleted non-video file", "file", extractedFile.FilePath)
			}
		} else {
			s.logger.Debug("Keeping video file", "file", extractedFile.FilePath)
		}
	}

	s.logger.Info("Cleanup completed",
		"download_id", downloadID,
		"deleted_files", len(deletedFiles),
		"errors", len(errors),
		"total_processed", len(extractedFiles))

	if len(errors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors: %v", len(errors), errors)
	}

	return nil
}

// isPathSafe checks if a file path is within the allowed base download directory
func (s *Service) isPathSafe(filePath string) bool {
	// Get absolute paths for comparison
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		s.logger.Warn("Failed to get absolute path for file", "file", filePath, "error", err)
		return false
	}

	absBasePath, err := filepath.Abs(s.baseDownloadPath)
	if err != nil {
		s.logger.Warn("Failed to get absolute path for base directory", "base", s.baseDownloadPath, "error", err)
		return false
	}

	// Ensure the file is within a subdirectory of the base download path
	// We don't allow deletion directly in the base path, only in subdirectories
	return strings.HasPrefix(absFilePath, absBasePath+string(os.PathSeparator)) && absFilePath != absBasePath
}

// shouldCleanupFile determines if a file should be deleted based on its extension
func (s *Service) shouldCleanupFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	// First check if it's a video file (keep these)
	for _, videoExt := range VideoExtensions {
		if ext == videoExt {
			return false // Keep video files
		}
	}

	// Then check if it's in the cleanup list (delete these)
	for _, cleanupExt := range CleanupExtensions {
		if ext == cleanupExt {
			return true // Delete these files
		}
	}

	// For unknown extensions, be conservative and keep them
	s.logger.Debug("Unknown file extension, keeping file", "file", filePath, "extension", ext)
	return false
}

// deleteFile safely deletes a file and updates the database record
func (s *Service) deleteFile(extractedFile *models.ExtractedFile, downloadID int64) error {
	// Check if file exists
	if _, err := os.Stat(extractedFile.FilePath); os.IsNotExist(err) {
		s.logger.Debug("File already deleted or doesn't exist", "file", extractedFile.FilePath)
		// Still mark as deleted in database
		return s.markFileDeleted(extractedFile.ID)
	}

	// Log the deletion for audit purposes
	s.logger.Info("Deleting non-video file",
		"download_id", downloadID,
		"file", extractedFile.FilePath,
		"size", s.getFileSize(extractedFile.FilePath),
		"created_at", extractedFile.CreatedAt)

	// Delete the file
	if err := os.Remove(extractedFile.FilePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Mark as deleted in database
	return s.markFileDeleted(extractedFile.ID)
}

// markFileDeleted updates the database to mark a file as deleted
func (s *Service) markFileDeleted(extractedFileID int64) error {
	deletedAt := time.Now()
	return s.db.MarkExtractedFileDeleted(extractedFileID, deletedAt)
}

// getFileSize returns the size of a file in bytes, or 0 if it can't be determined
func (s *Service) getFileSize(filePath string) int64 {
	if stat, err := os.Stat(filePath); err == nil {
		return stat.Size()
	}
	return 0
}

// CleanupEmptyDirectories removes empty directories that may be left after file cleanup
func (s *Service) CleanupEmptyDirectories(downloadID int64, rootPath string) error {
	if !s.isPathSafe(rootPath) {
		return fmt.Errorf("unsafe path for directory cleanup: %s", rootPath)
	}

	s.logger.Info("Cleaning up empty directories", "download_id", downloadID, "root_path", rootPath)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == rootPath {
			return nil
		}

		// Only process directories
		if !info.IsDir() {
			return nil
		}

		// Check if directory is empty
		if s.isDirectoryEmpty(path) {
			s.logger.Info("Removing empty directory", "download_id", downloadID, "directory", path)
			if removeErr := os.Remove(path); removeErr != nil {
				s.logger.Warn("Failed to remove empty directory", "directory", path, "error", removeErr)
			}
		}

		return nil
	})

	return err
}

// isDirectoryEmpty checks if a directory contains no files or subdirectories
func (s *Service) isDirectoryEmpty(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		s.logger.Warn("Failed to read directory", "directory", dirPath, "error", err)
		return false
	}
	return len(entries) == 0
}

// GetCleanupStats returns statistics about what would be cleaned up without actually doing it
func (s *Service) GetCleanupStats(downloadID int64) (*CleanupStats, error) {
	extractedFiles, err := s.db.GetExtractedFilesByDownloadID(downloadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get extracted files: %w", err)
	}

	stats := &CleanupStats{
		TotalFiles:   len(extractedFiles),
		VideoFiles:   0,
		CleanupFiles: 0,
		UnknownFiles: 0,
		UnsafeFiles:  0,
		TotalSize:    0,
		CleanupSize:  0,
	}

	for _, extractedFile := range extractedFiles {
		fileSize := s.getFileSize(extractedFile.FilePath)
		stats.TotalSize += fileSize

		if !s.isPathSafe(extractedFile.FilePath) {
			stats.UnsafeFiles++
			continue
		}

		if s.shouldCleanupFile(extractedFile.FilePath) {
			stats.CleanupFiles++
			stats.CleanupSize += fileSize
		} else {
			ext := strings.ToLower(filepath.Ext(extractedFile.FilePath))
			isVideo := false
			for _, videoExt := range VideoExtensions {
				if ext == videoExt {
					stats.VideoFiles++
					isVideo = true
					break
				}
			}
			if !isVideo {
				stats.UnknownFiles++
			}
		}
	}

	return stats, nil
}

// CleanupStats provides statistics about cleanup operations
type CleanupStats struct {
	TotalFiles   int   `json:"total_files"`
	VideoFiles   int   `json:"video_files"`
	CleanupFiles int   `json:"cleanup_files"`
	UnknownFiles int   `json:"unknown_files"`
	UnsafeFiles  int   `json:"unsafe_files"`
	TotalSize    int64 `json:"total_size"`
	CleanupSize  int64 `json:"cleanup_size"`
}
