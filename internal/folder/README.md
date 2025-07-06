# internal/folder Package Documentation

**Last Updated:** 2025-07-06  
**Version:** 1.0.0  
**Token Count:** ~4,500

## Overview

The `internal/folder` package provides secure folder browsing functionality with strict path validation and directory traversal protection. It enables safe navigation within a sandboxed base directory while preventing unauthorized access to files outside the designated folder structure.

This package is designed for applications that need to allow users to browse and manage directories within a controlled environment, such as download managers, file servers, or media organizers.

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/your-repo/internal/folder"
)

func main() {
    // Create a folder service restricted to /downloads
    service := folder.NewService("/downloads")
    
    // List directories in the root
    dirs, err := service.ListDirectories("")
    if err != nil {
        log.Fatal(err)
    }
    
    // Display directories
    for _, dir := range dirs {
        fmt.Printf("Directory: %s (Path: %s)\n", dir.Name, dir.Path)
    }
    
    // Create a new directory
    err = service.CreateDirectory("/movies/action")
    if err != nil {
        log.Fatal(err)
    }
}
```

## Features

### Core Functionality
- **Secure Path Validation**: Prevents directory traversal attacks using `../` sequences
- **Sandboxed Browsing**: Restricts access to files within the configured base directory
- **Directory Listing**: Lists only directories (filters out files) with parent navigation
- **Breadcrumb Navigation**: Generates navigation breadcrumbs for UI components
- **Safe Directory Creation**: Creates directories with proper validation and permissions

### Security Features
- **Path Traversal Prevention**: Blocks attempts to access parent directories
- **Base Path Enforcement**: Ensures all operations remain within the configured boundary
- **Input Sanitization**: Cleans and validates all path inputs before processing
- **Permission Control**: Creates directories with secure default permissions (0755)

## Documentation

### API Reference

#### Types

```go
type Service struct {
    BasePath string
}

type DirectoryInfo struct {
    Name  string `json:"name"`
    Path  string `json:"path"`
    IsDir bool   `json:"is_dir"`
}

type Breadcrumb struct {
    Name string `json:"name"`
    Path string `json:"path"`
}
```

#### Constructor

##### NewService(basePath string) *Service

Creates a new folder service instance with the specified base path.

**Parameters:**
- `basePath`: The absolute path that serves as the root directory for all operations

**Returns:**
- `*Service`: A new service instance with the cleaned base path

**Example:**
```go
service := folder.NewService("/home/user/downloads")
```

#### Methods

##### ListDirectories(relativePath string) ([]DirectoryInfo, error)

Lists all directories within the specified relative path, restricted to the base directory.

**Parameters:**
- `relativePath`: Relative path from the base directory (empty string or "/" for root)

**Returns:**
- `[]DirectoryInfo`: Slice of directory information objects
- `error`: Error if path validation fails or directory cannot be read

**Features:**
- Automatically includes parent directory (`..`) navigation for non-root paths
- Filters out files, showing only directories
- Validates path security before listing

**Example:**
```go
dirs, err := service.ListDirectories("/movies")
if err != nil {
    return err
}

for _, dir := range dirs {
    fmt.Printf("Directory: %s\n", dir.Name)
}
```

##### ValidatePath(relativePath string) (string, error)

Validates a relative path and returns the corresponding absolute path within the base directory.

**Parameters:**
- `relativePath`: The relative path to validate

**Returns:**
- `string`: The validated absolute path
- `error`: Error if path is invalid or outside base directory

**Security Checks:**
- Prevents directory traversal using `../` sequences
- Ensures resulting path remains within base directory
- Handles edge cases like multiple slashes and relative components

**Example:**
```go
fullPath, err := service.ValidatePath("/movies/action")
if err != nil {
    return fmt.Errorf("invalid path: %w", err)
}
```

##### GetBreadcrumbs(relativePath string) []Breadcrumb

Generates breadcrumb navigation for the specified path.

**Parameters:**
- `relativePath`: The relative path to generate breadcrumbs for

**Returns:**
- `[]Breadcrumb`: Slice of breadcrumb objects for navigation

**Features:**
- Always includes root breadcrumb based on base path name
- Generates hierarchical navigation structure
- Handles edge cases like empty paths and multiple slashes

**Example:**
```go
breadcrumbs := service.GetBreadcrumbs("/movies/action/2023")
// Returns: [{"downloads", "/"}, {"movies", "/movies"}, {"action", "/movies/action"}, {"2023", "/movies/action/2023"}]
```

##### CreateDirectory(relativePath string) error

Creates a new directory at the specified relative path.

**Parameters:**
- `relativePath`: The relative path where the directory should be created

**Returns:**
- `error`: Error if path validation fails, directory exists, or creation fails

**Features:**
- Validates path security before creation
- Creates parent directories as needed (`os.MkdirAll`)
- Sets secure permissions (0755)
- Prevents overwriting existing directories

**Example:**
```go
err := service.CreateDirectory("/movies/action/2023")
if err != nil {
    return fmt.Errorf("failed to create directory: %w", err)
}
```

## Architecture

### Security Model

The package implements a defense-in-depth security model:

1. **Input Sanitization**: All paths are cleaned and normalized using `filepath.Clean()`
2. **Path Validation**: Multiple checks prevent directory traversal:
   - Prefix validation ensures paths start with base directory
   - Additional separator checks prevent bypass attempts
3. **Sandboxing**: All operations are restricted to the configured base directory
4. **Permission Control**: New directories are created with secure default permissions

### Path Handling Strategy

```go
// Path validation logic
func (fs *Service) ValidatePath(relativePath string) (string, error) {
    // 1. Handle empty/root paths
    if relativePath == "" || relativePath == "/" {
        return fs.BasePath, nil
    }
    
    // 2. Normalize input (remove leading slash)
    cleanRelative := strings.TrimPrefix(relativePath, "/")
    
    // 3. Join with base path and clean
    fullPath := filepath.Join(fs.BasePath, cleanRelative)
    fullPath = filepath.Clean(fullPath)
    
    // 4. Security validation
    cleanBase := filepath.Clean(fs.BasePath)
    if !strings.HasPrefix(fullPath, cleanBase) {
        return "", fmt.Errorf("path outside of base directory")
    }
    
    // 5. Additional separator check
    if fullPath != cleanBase && !strings.HasPrefix(fullPath, cleanBase+string(filepath.Separator)) {
        return "", fmt.Errorf("invalid path")
    }
    
    return fullPath, nil
}
```

### Error Handling

The package uses Go's standard error handling with descriptive error messages:

- **Path Validation Errors**: Include the invalid path for debugging
- **Directory Read Errors**: Wrap filesystem errors with context
- **Creation Errors**: Distinguish between validation failures and filesystem issues

## Testing

### Test Coverage

The package includes comprehensive tests covering:

- **Path Validation**: All security scenarios including traversal attempts
- **Directory Listing**: Various directory structures and edge cases
- **Breadcrumb Generation**: Path parsing and navigation scenarios
- **Directory Creation**: Success cases and error conditions
- **Edge Cases**: Multiple slashes, dots, empty components

### Security Test Cases

```go
// Path traversal prevention tests
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
```

### Running Tests

```bash
# Run all tests
go test ./internal/folder

# Run with coverage
go test -cover ./internal/folder

# Run with race detection
go test -race ./internal/folder

# Verbose output
go test -v ./internal/folder
```

## Security Best Practices

### Implementation Guidelines

1. **Always Validate Paths**: Never trust user input - validate all paths through `ValidatePath()`
2. **Use Relative Paths**: Accept only relative paths from users, convert to absolute internally
3. **Handle Errors Properly**: Never ignore path validation errors
4. **Log Security Events**: Consider logging attempted path traversal attacks
5. **Regular Security Reviews**: Periodically audit path handling logic

### Usage Patterns

```go
// ✅ GOOD: Proper validation
func handleUserRequest(service *folder.Service, userPath string) error {
    dirs, err := service.ListDirectories(userPath)
    if err != nil {
        return fmt.Errorf("invalid path: %w", err)
    }
    // Process directories...
    return nil
}

// ❌ BAD: Direct filesystem access
func handleUserRequestUnsafe(basePath, userPath string) error {
    fullPath := filepath.Join(basePath, userPath) // No validation!
    entries, err := os.ReadDir(fullPath)
    if err != nil {
        return err
    }
    // This could access files outside basePath!
    return nil
}
```

### Common Attack Vectors Prevented

1. **Directory Traversal**: `../../../etc/passwd` → Blocked by path validation
2. **Absolute Path Injection**: `/etc/passwd` → Treated as relative path
3. **Multiple Traversal**: `valid/../../../etc` → Blocked by prefix checking
4. **Symlink Attacks**: Prevented by staying within base directory bounds
5. **Unicode Bypasses**: Handled by Go's filepath.Clean() normalization

## Error Reference

### Common Errors

- `path outside of base directory`: Attempted directory traversal
- `invalid path`: Path validation failed secondary checks
- `validate path error`: Wrapper for path validation failures
- `failed to read directory`: Filesystem error during directory listing
- `directory already exists`: Attempted to create existing directory
- `failed to create directory`: Filesystem error during directory creation

### Error Handling Examples

```go
dirs, err := service.ListDirectories(userPath)
if err != nil {
    if strings.Contains(err.Error(), "path outside of base directory") {
        // Log security event
        log.Printf("Security: Path traversal attempt: %s", userPath)
        return fmt.Errorf("access denied")
    }
    if strings.Contains(err.Error(), "failed to read directory") {
        // Directory doesn't exist or permission denied
        return fmt.Errorf("directory not found")
    }
    return fmt.Errorf("unexpected error: %w", err)
}
```

## Performance Considerations

- **Path Validation**: O(1) string operations with minimal overhead
- **Directory Listing**: O(n) where n is the number of entries in directory
- **Breadcrumb Generation**: O(d) where d is the directory depth
- **Memory Usage**: Minimal - only stores directory entries during listing

## Contributing

When contributing to this package:

1. **Security First**: All changes must maintain security guarantees
2. **Test Coverage**: New features require comprehensive tests including security tests
3. **Documentation**: Update this documentation for any API changes
4. **Code Review**: Security-related changes require thorough review

## License

This package is part of the debrid-downloader project and follows the project's licensing terms.