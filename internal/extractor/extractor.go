// Package extractor provides archive extraction functionality for ZIP and RAR files
package extractor

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode"
)

// Extractor interface defines methods for extracting archive files
type Extractor interface {
	Extract(archivePath, destPath string) ([]string, error)
	IsArchive(filename string) bool
}

// Service provides archive extraction services
type Service struct {
	logger *slog.Logger
}

// NewService creates a new extractor service
func NewService() *Service {
	return &Service{
		logger: slog.Default(),
	}
}

// Extract extracts an archive file to the specified destination
func (s *Service) Extract(archivePath, destPath string) ([]string, error) {
	filename := filepath.Base(archivePath)
	ext := strings.ToLower(filepath.Ext(archivePath))

	// Double-check that we should extract this file
	if !s.IsArchive(filename) {
		return nil, fmt.Errorf("file is not a supported archive or is not the first part of a multi-part archive: %s", filename)
	}

	switch ext {
	case ".zip":
		return s.extractZip(archivePath, destPath)
	case ".rar":
		s.logger.Info("Extracting RAR archive", "file", filename, "multipart", strings.Contains(strings.ToLower(filename), ".part"))
		return s.extractRar(archivePath, destPath)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", ext)
	}
}

// IsArchive checks if a file is a supported archive format
func (s *Service) IsArchive(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	lowerFilename := strings.ToLower(filename)
	
	// Check for ZIP files
	if ext == ".zip" {
		return true
	}
	
	// Check for RAR files - only first part or single RAR
	if ext == ".rar" {
		// Skip if it's a part file that's not the first part
		if strings.Contains(lowerFilename, ".part") && !strings.Contains(lowerFilename, ".part1.rar") && !strings.Contains(lowerFilename, ".part01.rar") && !strings.Contains(lowerFilename, ".part001.rar") {
			return false
		}
		return true
	}

	return false
}

// extractZip extracts a ZIP archive using Go's built-in archive/zip package
func (s *Service) extractZip(archivePath, destPath string) ([]string, error) {
	s.logger.Info("Extracting ZIP archive", "archive", archivePath, "dest", destPath)

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP archive: %w", err)
	}
	defer reader.Close()

	var extractedFiles []string

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, file := range reader.File {
		// Skip directories since we're flattening
		if file.FileInfo().IsDir() {
			continue
		}

		// Get just the filename without any path
		filename := filepath.Base(file.Name)
		
		// Validate filename to prevent directory traversal
		if strings.Contains(filename, "..") || strings.ContainsRune(filename, os.PathSeparator) {
			s.logger.Warn("Skipping file with potentially dangerous name", "file", file.Name)
			continue
		}

		// Extract directly to destination directory (flattened)
		fullPath := filepath.Join(destPath, filename)

		// Extract file
		if err := s.extractZipFile(file, fullPath); err != nil {
			s.logger.Warn("Failed to extract file", "file", file.Name, "error", err)
			continue
		}

		extractedFiles = append(extractedFiles, fullPath)
		s.logger.Debug("Extracted file (flattened)", "original", file.Name, "extracted_to", fullPath)
	}

	s.logger.Info("ZIP extraction completed", "archive", archivePath, "extracted_files", len(extractedFiles))
	return extractedFiles, nil
}

// extractZipFile extracts a single file from a ZIP archive
func (s *Service) extractZipFile(file *zip.File, destPath string) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in archive: %w", err)
	}
	defer reader.Close()

	writer, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer writer.Close()

	_, err = io.Copy(writer, reader)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// extractRar extracts a RAR archive using the rardecode library
func (s *Service) extractRar(archivePath, destPath string) ([]string, error) {
	s.logger.Info("Extracting RAR archive", "archive", archivePath, "dest", destPath)

	// Check if this is a multi-part archive and if all parts exist
	dir := filepath.Dir(archivePath)
	base := filepath.Base(archivePath)
	isMultipart := strings.Contains(strings.ToLower(base), ".part")
	
	if isMultipart {
		s.logger.Info("Detected multi-part RAR archive", "file", base)
		// Log what RAR files exist in the directory
		files, _ := os.ReadDir(dir)
		var rarFiles []string
		for _, f := range files {
			if strings.HasSuffix(strings.ToLower(f.Name()), ".rar") {
				rarFiles = append(rarFiles, f.Name())
			}
		}
		s.logger.Info("RAR files in directory", "files", rarFiles)
	}

	// Use OpenReader for multi-part archive support
	rarReader, err := rardecode.OpenReader(archivePath, "")
	if err != nil {
		// Check if it's a password-protected archive
		if strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "encrypted") {
			s.logger.Warn("RAR archive is password-protected, skipping extraction", "archive", archivePath)
			return nil, fmt.Errorf("RAR archive is password-protected")
		}
		return nil, fmt.Errorf("failed to open RAR archive: %w", err)
	}
	defer rarReader.Close()

	var extractedFiles []string

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	for {
		header, err := rarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.logger.Warn("Error reading RAR header", "error", err)
			break
		}

		// Skip directories since we're flattening
		if header.IsDir {
			continue
		}

		// Get just the filename without any path
		filename := filepath.Base(header.Name)
		
		// Validate filename to prevent directory traversal
		if strings.Contains(filename, "..") || strings.ContainsRune(filename, os.PathSeparator) {
			s.logger.Warn("Skipping file with potentially dangerous name", "file", header.Name)
			continue
		}

		// Extract directly to destination directory (flattened)
		fullPath := filepath.Join(destPath, filename)

		// Extract file
		if err := s.extractRarFile(rarReader, fullPath, header.Mode()); err != nil {
			s.logger.Warn("Failed to extract file", "file", header.Name, "error", err)
			continue
		}

		extractedFiles = append(extractedFiles, fullPath)
		s.logger.Debug("Extracted file (flattened)", "original", header.Name, "extracted_to", fullPath)
	}

	// Log which volumes were used
	if volumes := rarReader.Volumes(); len(volumes) > 1 {
		s.logger.Info("Multi-part RAR extraction used volumes", "volumes", volumes, "count", len(volumes))
	}

	s.logger.Info("RAR extraction completed", "archive", archivePath, "extracted_files", len(extractedFiles))
	return extractedFiles, nil
}

// extractRarFile extracts a single file from a RAR archive
func (s *Service) extractRarFile(reader io.Reader, destPath string, mode os.FileMode) error {
	writer, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer writer.Close()

	_, err = io.Copy(writer, reader)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}
