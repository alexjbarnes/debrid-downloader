package downloader

import (
	"debrid-downloader/pkg/models"
)

// DatabaseInterface defines the database operations used by the downloader worker
//
//go:generate mockgen -source=interfaces.go -destination=mocks/mock_interfaces.go -package=mocks
type DatabaseInterface interface {
	// Download operations
	GetDownload(id int64) (*models.Download, error)
	UpdateDownload(download *models.Download) error

	// Download group operations
	GetDownloadGroup(id string) (*models.DownloadGroup, error)
	GetDownloadsByGroupID(groupID string) ([]*models.Download, error)
	UpdateDownloadGroup(group *models.DownloadGroup) error

	// Extracted file operations
	CreateExtractedFile(file *models.ExtractedFile) error
}

// CleanupInterface defines the cleanup operations used by the downloader worker
type CleanupInterface interface {
	CleanupExtractedFiles(downloadID int64) error
	CleanupEmptyDirectories(downloadID int64, directory string) error
}

// ExtractorInterface defines the archive extraction operations
type ExtractorInterface interface {
	Extract(archivePath, destPath string) ([]string, error)
	IsArchive(filename string) bool
}
