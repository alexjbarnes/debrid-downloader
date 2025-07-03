// Package folder provides folder browsing functionality with security restrictions
package folder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Service provides secure folder browsing within a base directory
type Service struct {
	BasePath string
}

// DirectoryInfo represents information about a directory or file
type DirectoryInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// Breadcrumb represents a breadcrumb navigation item
type Breadcrumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// NewService creates a new folder service with the specified base path
func NewService(basePath string) *Service {
	return &Service{
		BasePath: filepath.Clean(basePath),
	}
}

// ListDirectories lists directories within the given path, restricted to the base path
func (fs *Service) ListDirectories(relativePath string) ([]DirectoryInfo, error) {
	fullPath, err := fs.ValidatePath(relativePath)
	if err != nil {
		return nil, fmt.Errorf("validate path error: %w", err)
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", fullPath, err)
	}

	var directories []DirectoryInfo

	// Add parent directory option if not at root
	if relativePath != "" && relativePath != "/" {
		parentPath := filepath.Dir(relativePath)
		if parentPath == "." {
			parentPath = "/"
		}
		directories = append(directories, DirectoryInfo{
			Name:  "..",
			Path:  parentPath,
			IsDir: true,
		})
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Create the relative path for this directory
			var itemPath string
			if relativePath == "" || relativePath == "/" {
				itemPath = "/" + entry.Name()
			} else {
				itemPath = relativePath + "/" + entry.Name()
			}

			directories = append(directories, DirectoryInfo{
				Name:  entry.Name(),
				Path:  itemPath,
				IsDir: true,
			})
		}
	}

	return directories, nil
}

// ValidatePath ensures the given relative path is safe and returns the full path
func (fs *Service) ValidatePath(relativePath string) (string, error) {
	// Handle empty path or root
	if relativePath == "" || relativePath == "/" {
		return fs.BasePath, nil
	}

	// Normalize the input path - remove leading slash to treat as relative
	cleanRelative := strings.TrimPrefix(relativePath, "/")

	// Join with base path and clean
	fullPath := filepath.Join(fs.BasePath, cleanRelative)
	fullPath = filepath.Clean(fullPath)

	// Clean both paths for comparison
	cleanBase := filepath.Clean(fs.BasePath)

	// Ensure the result is still within the base path
	// Check if the full path starts with the base path
	if !strings.HasPrefix(fullPath, cleanBase) {
		return "", fmt.Errorf("path outside of base directory: %s", relativePath)
	}

	// Additional check: ensure we're not going up with ..
	if fullPath != cleanBase && !strings.HasPrefix(fullPath, cleanBase+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path: %s", relativePath)
	}

	return fullPath, nil
}

// GetBreadcrumbs generates breadcrumb navigation for the given path
func (fs *Service) GetBreadcrumbs(relativePath string) []Breadcrumb {
	// Use the base path's directory name as the root name
	rootName := filepath.Base(fs.BasePath)
	if rootName == "" || rootName == "/" || rootName == "." {
		rootName = "Root"
	}

	breadcrumbs := []Breadcrumb{
		{Name: rootName, Path: "/"},
	}

	if relativePath == "" || relativePath == "/" {
		return breadcrumbs
	}

	// Split the path and build breadcrumbs
	cleanPath := strings.Trim(relativePath, "/")
	if cleanPath == "" {
		return breadcrumbs
	}

	parts := strings.Split(cleanPath, "/")
	currentPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}

		currentPath += "/" + part
		breadcrumbs = append(breadcrumbs, Breadcrumb{
			Name: part,
			Path: currentPath,
		})
	}

	return breadcrumbs
}

// CreateDirectory creates a new directory at the specified path (if it doesn't exist)
func (fs *Service) CreateDirectory(relativePath string) error {
	fullPath, err := fs.ValidatePath(relativePath)
	if err != nil {
		return err
	}

	// Check if directory already exists
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("directory already exists: %s", relativePath)
	}

	// Create the directory with appropriate permissions
	if err := os.MkdirAll(fullPath, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}
