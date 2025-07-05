package folder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	basePath := "/test/base/path"
	service := NewService(basePath)
	
	require.NotNil(t, service)
	require.Equal(t, filepath.Clean(basePath), service.BasePath)
}

func TestService_ValidatePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "validate_path_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(tempDir)

	tests := []struct {
		name         string
		relativePath string
		expectError  bool
		errorContains string
	}{
		{
			name:         "empty path",
			relativePath: "",
			expectError:  false,
		},
		{
			name:         "root path",
			relativePath: "/",
			expectError:  false,
		},
		{
			name:         "valid relative path",
			relativePath: "subdir",
			expectError:  false,
		},
		{
			name:         "valid relative path with leading slash",
			relativePath: "/subdir",
			expectError:  false,
		},
		{
			name:         "path traversal attack - parent directory",
			relativePath: "../",
			expectError:  true,
			errorContains: "path outside of base directory",
		},
		{
			name:         "path traversal attack - multiple levels",
			relativePath: "../../etc/passwd",
			expectError:  true,
			errorContains: "path outside of base directory",
		},
		{
			name:         "path traversal attack - mixed valid and invalid",
			relativePath: "valid/../../../etc",
			expectError:  true,
			errorContains: "path outside of base directory",
		},
		{
			name:         "absolute path attack",
			relativePath: "/etc/passwd",
			expectError:  false, // This actually becomes a valid relative path within tempDir
		},
		{
			name:         "path equal to base",
			relativePath: "",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath, err := service.ValidatePath(tt.relativePath)
			
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorContains)
				require.Empty(t, fullPath)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, fullPath)
				// Ensure the path is within the base directory
				require.True(t, filepath.HasPrefix(fullPath, service.BasePath))
			}
		})
	}
}

func TestService_ListDirectories(t *testing.T) {
	// Create temp directory structure
	tempDir, err := os.MkdirTemp("", "list_directories_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	testDirs := []string{
		"subdir1",
		"subdir2",
		"deep/nested/dir",
	}
	for _, dir := range testDirs {
		err = os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		require.NoError(t, err)
	}

	// Create test files (should be ignored)
	testFiles := []string{
		"file1.txt",
		"file2.txt",
		"subdir1/file3.txt",
	}
	for _, file := range testFiles {
		err = os.WriteFile(filepath.Join(tempDir, file), []byte("test"), 0644)
		require.NoError(t, err)
	}

	service := NewService(tempDir)

	t.Run("list root directory", func(t *testing.T) {
		dirs, err := service.ListDirectories("")
		require.NoError(t, err)
		
		// Should have subdir1, subdir2, and deep directories (no files)
		require.Len(t, dirs, 3)
		
		dirNames := make([]string, len(dirs))
		for i, dir := range dirs {
			dirNames[i] = dir.Name
			require.True(t, dir.IsDir)
		}
		require.Contains(t, dirNames, "subdir1")
		require.Contains(t, dirNames, "subdir2")
		require.Contains(t, dirNames, "deep")
	})

	t.Run("list subdirectory with parent link", func(t *testing.T) {
		dirs, err := service.ListDirectories("/subdir1")
		require.NoError(t, err)
		
		// Should have parent directory ".." link
		require.Greater(t, len(dirs), 0)
		require.Equal(t, "..", dirs[0].Name)
		require.Equal(t, "/", dirs[0].Path)
		require.True(t, dirs[0].IsDir)
	})

	t.Run("list nested directory", func(t *testing.T) {
		dirs, err := service.ListDirectories("/deep/nested")
		require.NoError(t, err)
		
		// Should have parent directory ".." link and "dir" subdirectory
		require.Len(t, dirs, 2)
		require.Equal(t, "..", dirs[0].Name)
		require.Equal(t, "/deep", dirs[0].Path)
		require.Equal(t, "dir", dirs[1].Name)
		require.Equal(t, "/deep/nested/dir", dirs[1].Path)
	})

	t.Run("list non-existent directory", func(t *testing.T) {
		_, err := service.ListDirectories("/nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read directory")
	})

	t.Run("list with invalid path", func(t *testing.T) {
		_, err := service.ListDirectories("../../../etc")
		require.Error(t, err)
		require.Contains(t, err.Error(), "validate path error")
	})

	t.Run("list directory with parent path edge case", func(t *testing.T) {
		// Create a subdirectory at root level to test the parent path logic
		err := os.Mkdir(filepath.Join(tempDir, "rootsub"), 0755)
		require.NoError(t, err)
		
		dirs, err := service.ListDirectories("rootsub")
		require.NoError(t, err)
		
		// Should have parent directory ".." with path "/"
		require.Greater(t, len(dirs), 0)
		require.Equal(t, "..", dirs[0].Name)
		require.Equal(t, "/", dirs[0].Path)
	})
}

func TestService_GetBreadcrumbs(t *testing.T) {
	service := NewService("/downloads")

	tests := []struct {
		name         string
		relativePath string
		expected     []Breadcrumb
	}{
		{
			name:         "root path",
			relativePath: "",
			expected: []Breadcrumb{
				{Name: "downloads", Path: "/"},
			},
		},
		{
			name:         "root path with slash",
			relativePath: "/",
			expected: []Breadcrumb{
				{Name: "downloads", Path: "/"},
			},
		},
		{
			name:         "single level path",
			relativePath: "/movies",
			expected: []Breadcrumb{
				{Name: "downloads", Path: "/"},
				{Name: "movies", Path: "/movies"},
			},
		},
		{
			name:         "multi level path",
			relativePath: "/movies/action/2023",
			expected: []Breadcrumb{
				{Name: "downloads", Path: "/"},
				{Name: "movies", Path: "/movies"},
				{Name: "action", Path: "/movies/action"},
				{Name: "2023", Path: "/movies/action/2023"},
			},
		},
		{
			name:         "path without leading slash",
			relativePath: "tv/series",
			expected: []Breadcrumb{
				{Name: "downloads", Path: "/"},
				{Name: "tv", Path: "/tv"},
				{Name: "series", Path: "/tv/series"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			breadcrumbs := service.GetBreadcrumbs(tt.relativePath)
			require.Equal(t, tt.expected, breadcrumbs)
		})
	}
}

func TestService_GetBreadcrumbsWithDifferentBasePaths(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		expected string
	}{
		{
			name:     "normal path",
			basePath: "/home/user/downloads",
			expected: "downloads",
		},
		{
			name:     "root path",
			basePath: "/",
			expected: "Root",
		},
		{
			name:     "dot path",
			basePath: ".",
			expected: "Root",
		},
		{
			name:     "empty path",
			basePath: "",
			expected: "Root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.basePath)
			breadcrumbs := service.GetBreadcrumbs("")
			require.Len(t, breadcrumbs, 1)
			require.Equal(t, tt.expected, breadcrumbs[0].Name)
			require.Equal(t, "/", breadcrumbs[0].Path)
		})
	}
}

func TestService_CreateDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "create_directory_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(tempDir)

	t.Run("create new directory", func(t *testing.T) {
		err := service.CreateDirectory("/newdir")
		require.NoError(t, err)
		
		// Check directory was created
		fullPath := filepath.Join(tempDir, "newdir")
		stat, err := os.Stat(fullPath)
		require.NoError(t, err)
		require.True(t, stat.IsDir())
	})

	t.Run("create nested directory", func(t *testing.T) {
		err := service.CreateDirectory("/deep/nested/newdir")
		require.NoError(t, err)
		
		// Check directory was created
		fullPath := filepath.Join(tempDir, "deep/nested/newdir")
		stat, err := os.Stat(fullPath)
		require.NoError(t, err)
		require.True(t, stat.IsDir())
	})

	t.Run("create directory that already exists", func(t *testing.T) {
		// Create directory first
		err := service.CreateDirectory("/existing")
		require.NoError(t, err)
		
		// Try to create again
		err = service.CreateDirectory("/existing")
		require.Error(t, err)
		require.Contains(t, err.Error(), "directory already exists")
	})

	t.Run("create directory with invalid path", func(t *testing.T) {
		err := service.CreateDirectory("../../../etc/malicious")
		require.Error(t, err)
		require.Contains(t, err.Error(), "path outside of base directory")
	})

}

func TestDirectoryInfo_Struct(t *testing.T) {
	info := DirectoryInfo{
		Name:  "testdir",
		Path:  "/test/path",
		IsDir: true,
	}
	
	require.Equal(t, "testdir", info.Name)
	require.Equal(t, "/test/path", info.Path)
	require.True(t, info.IsDir)
}

func TestBreadcrumb_Struct(t *testing.T) {
	breadcrumb := Breadcrumb{
		Name: "Home",
		Path: "/home",
	}
	
	require.Equal(t, "Home", breadcrumb.Name)
	require.Equal(t, "/home", breadcrumb.Path)
}

func TestService_EdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "edge_cases_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	service := NewService(tempDir)

	t.Run("path with multiple slashes", func(t *testing.T) {
		fullPath, err := service.ValidatePath("///subdir///")
		require.NoError(t, err)
		expected := filepath.Join(tempDir, "subdir")
		require.Equal(t, expected, fullPath)
	})

	t.Run("path with dots", func(t *testing.T) {
		fullPath, err := service.ValidatePath("./subdir/./")
		require.NoError(t, err)
		expected := filepath.Join(tempDir, "subdir")
		require.Equal(t, expected, fullPath)
	})

	t.Run("breadcrumbs with empty parts", func(t *testing.T) {
		breadcrumbs := service.GetBreadcrumbs("//movies//action//")
		require.Len(t, breadcrumbs, 3)
		require.Equal(t, "movies", breadcrumbs[1].Name)
		require.Equal(t, "action", breadcrumbs[2].Name)
	})

	t.Run("path validation edge case", func(t *testing.T) {
		// Create a service with a base path that ends with separator
		serviceWithSep := NewService(tempDir + string(filepath.Separator))
		
		// Test path that would trigger the second condition in ValidatePath
		fullPath, err := serviceWithSep.ValidatePath("")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean(tempDir+string(filepath.Separator)), fullPath)
	})

	t.Run("breadcrumbs with edge cases", func(t *testing.T) {
		// Test GetBreadcrumbs with edge cases to improve coverage
		breadcrumbs := service.GetBreadcrumbs("/")
		require.Len(t, breadcrumbs, 1)
		
		// Test with path that has empty components after cleaning
		breadcrumbs = service.GetBreadcrumbs("///")
		require.Len(t, breadcrumbs, 1)
		
		// Test with single empty component
		breadcrumbs = service.GetBreadcrumbs("/a//b/")
		require.Len(t, breadcrumbs, 3)
		require.Equal(t, "a", breadcrumbs[1].Name)
		require.Equal(t, "b", breadcrumbs[2].Name)
	})
}

func TestService_ListDirectoriesParentPathHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "parent_path_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create nested directory structure
	err = os.MkdirAll(filepath.Join(tempDir, "level1/level2"), 0755)
	require.NoError(t, err)

	service := NewService(tempDir)

	t.Run("parent path from root", func(t *testing.T) {
		dirs, err := service.ListDirectories("/")
		require.NoError(t, err)
		
		// Should not have parent directory link at root
		for _, dir := range dirs {
			require.NotEqual(t, "..", dir.Name)
		}
	})

	t.Run("parent path from level 1", func(t *testing.T) {
		dirs, err := service.ListDirectories("/level1")
		require.NoError(t, err)
		
		// Should have parent directory pointing to root
		require.Greater(t, len(dirs), 0)
		require.Equal(t, "..", dirs[0].Name)
		require.Equal(t, "/", dirs[0].Path)
	})

	t.Run("parent path from level 2", func(t *testing.T) {
		dirs, err := service.ListDirectories("/level1/level2")
		require.NoError(t, err)
		
		// Should have parent directory pointing to level1
		require.Greater(t, len(dirs), 0)
		require.Equal(t, "..", dirs[0].Name)
		require.Equal(t, "/level1", dirs[0].Path)
	})
}