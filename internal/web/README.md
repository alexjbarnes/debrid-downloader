# internal/web Package Documentation

## Overview

The `internal/web` package provides a comprehensive HTTP server implementation for the debrid-downloader application. It implements a modern web interface with real-time updates, intelligent directory suggestions, and dark/light mode theming using HTMX, Templ templates, and Tailwind CSS.

**Key Features:**
- HTTP server with graceful shutdown
- HTMX-powered dynamic content updates
- Type-safe HTML templating with Templ
- Real-time download progress tracking
- Intelligent directory suggestion system
- Dark/light mode theme switching
- Secure folder browsing with path validation
- RESTful API endpoints for folder management

**Last Updated:** 2025-07-06  
**Go Version:** 1.24.4  
**Dependencies:** HTMX 2.0.4, Tailwind CSS (CDN)

## Architecture

### Package Structure

```
internal/web/
├── server.go                 # HTTP server implementation
├── server_test.go           # Server tests
├── handlers/
│   ├── handlers.go          # HTTP handlers implementation
│   └── handlers_test.go     # Handler tests
├── templates/
│   ├── base.templ           # Base HTML template
│   ├── home.templ           # Home page template
│   ├── settings.templ       # Settings page template
│   ├── partials.templ       # Reusable template components
│   └── *.templ_go          # Generated Go code from templates
└── static/                  # Static assets (if any)
```

### Dependencies

- **Database:** SQLite for downloads and directory mappings
- **AllDebrid Client:** Interface for unrestricting URLs
- **Folder Service:** Secure directory browsing
- **Download Worker:** Background download processing
- **Logging:** Structured logging with slog

## Server Configuration

### Server Creation

```go
func NewServer(db *database.DB, client alldebrid.AllDebridClient, cfg *config.Config, worker *downloader.Worker) *Server
```

The server is configured with:
- **Read Timeout:** 15 seconds
- **Write Timeout:** 15 seconds  
- **Idle Timeout:** 60 seconds
- **Port:** Configurable via `SERVER_PORT` environment variable

### Server Lifecycle

```go
// Start the server
server := NewServer(db, client, cfg, worker)
go func() {
    if err := server.Start(); err != nil && err != http.ErrServerClosed {
        log.Fatal("Server failed to start:", err)
    }
}()

// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := server.Shutdown(ctx); err != nil {
    log.Error("Server shutdown error:", err)
}
```

## Route Structure

### Main Routes

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET` | `/` | `handlers.Home` | Home page with download form and history |
| `GET` | `/settings` | `handlers.Settings` | Settings page with theme controls |

### HTMX Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET` | `/downloads/current` | `handlers.CurrentDownloads` | Current downloads with polling |
| `POST` | `/download` | `handlers.SubmitDownload` | Submit new download |
| `POST` | `/downloads/search` | `handlers.SearchDownloads` | Search/filter downloads |
| `POST` | `/downloads/{id}/retry` | `handlers.RetryDownload` | Retry failed download |
| `POST` | `/downloads/{id}/pause` | `handlers.PauseDownload` | Pause active download |
| `POST` | `/downloads/{id}/resume` | `handlers.ResumeDownload` | Resume paused download |
| `DELETE` | `/downloads/{id}` | `handlers.DeleteDownload` | Delete download record |

### API Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET/POST` | `/api/directory-suggestion` | `handlers.GetDirectorySuggestion` | Directory suggestions |
| `GET` | `/api/folders` | `handlers.BrowseFolders` | Browse filesystem folders |
| `POST` | `/api/folders` | `handlers.CreateFolder` | Create new folder |
| `POST` | `/api/test/failed-download` | `handlers.CreateTestFailedDownload` | Testing endpoint |

## Handler Implementation

### Core Handlers Structure

```go
type Handlers struct {
    db              *database.DB
    allDebridClient alldebrid.AllDebridClient
    folderService   *folder.Service
    downloadWorker  *downloader.Worker
    logger          *slog.Logger
}
```

### Key Handler Functions

#### Download Submission

```go
func (h *Handlers) SubmitDownload(w http.ResponseWriter, r *http.Request)
```

**Features:**
- Single and multi-URL support
- URL unrestriction via AllDebrid API
- Unique filename generation
- Archive detection
- Directory mapping learning
- Download group management
- Real-time form updates via HTMX

#### Directory Suggestions

```go
func (h *Handlers) GetDirectorySuggestion(w http.ResponseWriter, r *http.Request)
```

**Algorithm:**
1. Extract filename/URL from request
2. Fuzzy match against stored patterns
3. Score based on filename similarity and usage count
4. Return top suggestion with alternatives

#### Folder Management

```go
func (h *Handlers) BrowseFolders(w http.ResponseWriter, r *http.Request)
func (h *Handlers) CreateFolder(w http.ResponseWriter, r *http.Request)
```

**Security Features:**
- Path validation prevents directory traversal
- Sandboxed to base downloads directory
- Proper error handling for invalid paths

## HTMX Integration

### Dynamic Content Updates

The application uses HTMX for seamless user interactions:

```html
<!-- Auto-polling for download updates -->
<div hx-get="/downloads/current" 
     hx-trigger="every 2s" 
     hx-swap="innerHTML">
</div>

<!-- Form submission with indicators -->
<form hx-post="/download" 
      hx-target="#result" 
      hx-indicator="#submit-button">
</form>
```

### Real-time Features

1. **Download Progress:** Auto-updates every 2 seconds when active downloads exist
2. **Form Validation:** Real-time directory suggestions as user types
3. **Status Updates:** Instant feedback on download operations
4. **Dynamic Polling:** Polling interval adjusts based on activity

### Out-of-Band Swaps

The server uses HTMX out-of-band swaps for complex UI updates:

```go
// Reset form fields after successful submission
w.Write([]byte(`<input type="url" ... hx-swap-oob="true" value="">`))
w.Write([]byte(`<div id="downloads-list" ... hx-swap-oob="true">`))
```

## Template System

### Templ Templates

The application uses [Templ](https://templ.guide/) for type-safe HTML generation:

```go
// Base template with content injection
templ Base(title string, content templ.Component) {
    <!DOCTYPE html>
    <html>
        <head>
            <title>{ title }</title>
        </head>
        <body>
            @content
        </body>
    </html>
}

// Component composition
templ Home(downloads []*models.Download, suggestedDir string, recentDirs []string) {
    <div class="space-y-6">
        <!-- Download form -->
        <!-- Download history -->
    </div>
}
```

### Template Components

**Core Templates:**
- `base.templ`: Base HTML structure with theme system
- `home.templ`: Main download interface
- `settings.templ`: Settings page
- `partials.templ`: Reusable components

**Key Components:**
- `DownloadItem`: Individual download display
- `StatusBadge`: Download status indicator
- `ProgressBar`: Download progress visualization
- `DirectoryPicker`: Folder selection modal

### Template Compilation

Templates must be compiled to Go code:

```bash
# Generate templates
just templ-generate

# Or manually
templ generate
```

## Theme System

### Theme Detection

The application implements intelligent theme detection:

```javascript
// Auto-detect device preference
const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;

// Respect user override
const userSetTheme = localStorage.getItem('userSetTheme');
const savedTheme = localStorage.getItem('theme');

// Apply theme
const theme = (savedTheme && userSetTheme) ? savedTheme : (prefersDark ? 'dark' : 'light');
```

### Theme Switching

```javascript
// Global theme setter
window.setTheme = function(theme) {
    document.documentElement.classList.toggle('dark', theme === 'dark');
    localStorage.setItem('theme', theme);
    localStorage.setItem('userSetTheme', 'true');
};
```

### CSS Framework

- **Tailwind CSS:** Utility-first CSS framework
- **Dark Mode:** Class-based dark mode implementation
- **Custom Scrollbars:** Themed scrollbar styles
- **HTMX Indicators:** Loading state animations

## Error Handling

### HTTP Error Responses

```go
// Structured error handling
func (h *Handlers) handleError(w http.ResponseWriter, err error, statusCode int) {
    h.logger.Error("Handler error", "error", err)
    http.Error(w, "Internal server error", statusCode)
}
```

### Client-Side Error Handling

```javascript
// HTMX error handling
document.addEventListener('htmx:responseError', function(event) {
    console.error('HTMX Error:', event.detail);
    // Show user-friendly error message
});
```

### Logging

The package uses structured logging with slog:

```go
h.logger.Info("Download submitted", 
    "url", url, 
    "directory", directory, 
    "filename", filename,
    "download_id", download.ID)

h.logger.Error("Failed to create download", 
    "error", err, 
    "url", url)
```

## Testing

### Test Structure

```go
func TestHandlers_SubmitDownload(t *testing.T) {
    // Setup test database
    db, err := database.New(":memory:")
    require.NoError(t, err)
    defer db.Close()
    
    // Create mock client
    ctrl := gomock.NewController(t)
    mockClient := mocks.NewMockAllDebridClient(ctrl)
    
    // Test handler
    handlers := NewHandlers(db, mockClient, "/tmp/test", worker)
    
    // Execute test
    req := httptest.NewRequest("POST", "/download", body)
    w := httptest.NewRecorder()
    handlers.SubmitDownload(w, req)
    
    // Verify results
    require.Equal(t, http.StatusOK, w.Code)
}
```

### Test Coverage

- **Server Tests:** Server lifecycle and configuration
- **Handler Tests:** HTTP handler functionality
- **Integration Tests:** End-to-end request/response flow
- **Mock Tests:** External service integration

### Running Tests

```bash
# Run all tests
just test

# Run with coverage
just coverage

# Run specific test
go test -v ./internal/web/handlers -run TestHandlers_SubmitDownload
```

## Usage Examples

### Basic Server Setup

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "debrid-downloader/internal/web"
    "debrid-downloader/internal/database"
    "debrid-downloader/internal/config"
    "debrid-downloader/internal/alldebrid"
    "debrid-downloader/internal/downloader"
)

func main() {
    // Initialize dependencies
    cfg := config.Load()
    db, err := database.New(cfg.DatabasePath)
    if err != nil {
        log.Fatal("Database error:", err)
    }
    defer db.Close()
    
    client := alldebrid.New(cfg.AllDebridAPIKey)
    worker := downloader.NewWorker(db, cfg.BaseDownloadsPath)
    
    // Create and start server
    server := web.NewServer(db, client, cfg, worker)
    
    go func() {
        if err := server.Start(); err != nil && err != http.ErrServerClosed {
            log.Fatal("Server error:", err)
        }
    }()
    
    // Wait for shutdown signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatal("Server shutdown error:", err)
    }
}
```

### Custom Handler Integration

```go
// Add custom handler
func (s *Server) AddCustomHandler(pattern string, handler http.HandlerFunc) {
    s.server.Handler.(*http.ServeMux).HandleFunc(pattern, handler)
}

// Custom middleware
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        duration := time.Since(start)
        log.Printf("%s %s %v", r.Method, r.URL.Path, duration)
    })
}
```

## Best Practices

### Security

1. **Input Validation:** All user inputs are validated and sanitized
2. **Path Traversal Prevention:** Folder service validates all paths
3. **CSRF Protection:** HTMX provides CSRF token handling
4. **Content Security:** Proper Content-Type headers set

### Performance

1. **Template Compilation:** Templates pre-compiled to Go code
2. **Static Assets:** CDN delivery for CSS/JS frameworks
3. **Efficient Polling:** Dynamic polling based on activity
4. **Database Optimization:** Indexed queries and connection pooling

### Maintainability

1. **Type Safety:** Templ templates provide compile-time safety
2. **Structured Logging:** Consistent logging with context
3. **Error Handling:** Comprehensive error handling and recovery
4. **Testing:** High test coverage with mocks

### User Experience

1. **Progressive Enhancement:** Works without JavaScript
2. **Responsive Design:** Mobile-friendly interface
3. **Real-time Updates:** Instant feedback on operations
4. **Accessibility:** Proper ARIA labels and keyboard navigation

## Integration Patterns

### Database Integration

```go
// Handler with database operations
func (h *Handlers) customHandler(w http.ResponseWriter, r *http.Request) {
    // Query database
    downloads, err := h.db.ListDownloads(50, 0)
    if err != nil {
        h.handleError(w, err, http.StatusInternalServerError)
        return
    }
    
    // Render template
    component := templates.CustomTemplate(downloads)
    if err := component.Render(r.Context(), w); err != nil {
        h.handleError(w, err, http.StatusInternalServerError)
        return
    }
}
```

### External Service Integration

```go
// Handler with external API calls
func (h *Handlers) apiHandler(w http.ResponseWriter, r *http.Request) {
    // Call external service
    result, err := h.allDebridClient.UnrestrictLink(r.Context(), url)
    if err != nil {
        h.logger.Error("API call failed", "error", err)
        http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
        return
    }
    
    // Process result
    // ...
}
```

### Background Processing

```go
// Handler triggering background work
func (h *Handlers) processHandler(w http.ResponseWriter, r *http.Request) {
    // Create work item
    download := &models.Download{
        // ... initialize fields
    }
    
    if err := h.db.CreateDownload(download); err != nil {
        h.handleError(w, err, http.StatusInternalServerError)
        return
    }
    
    // Queue for background processing
    h.downloadWorker.QueueDownload(download.ID)
    
    // Return immediate response
    w.WriteHeader(http.StatusAccepted)
}
```

## Configuration

### Environment Variables

```bash
# Server configuration
SERVER_PORT=3000

# Database configuration
DATABASE_PATH=debrid.db

# AllDebrid API
ALLDEBRID_API_KEY=your_api_key_here

# Download configuration
BASE_DOWNLOADS_PATH=/downloads

# Logging
LOG_LEVEL=info
```

### Runtime Configuration

```go
type Config struct {
    ServerPort        string
    DatabasePath      string
    AllDebridAPIKey   string
    BaseDownloadsPath string
    LogLevel          string
}
```

## Troubleshooting

### Common Issues

1. **Template Compilation Errors**
   ```bash
   # Regenerate templates
   just templ-generate
   ```

2. **HTMX Not Working**
   - Check browser console for JavaScript errors
   - Verify HTMX script is loaded
   - Ensure proper HTMX attributes

3. **Theme Not Switching**
   - Check localStorage for theme settings
   - Verify JavaScript theme functions are loaded
   - Clear browser cache

4. **Database Errors**
   - Check database file permissions
   - Verify database schema is up to date
   - Check available disk space

### Debug Mode

```go
// Enable debug logging
cfg.LogLevel = "debug"

// Add debug handlers
if cfg.LogLevel == "debug" {
    mux.HandleFunc("/debug/pprof/", pprof.Index)
    mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
    mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
}
```

## API Documentation

### Folder API

#### Browse Folders
```
GET /api/folders?path={path}
```

**Response:**
```json
{
  "directories": [
    {
      "name": "Documents",
      "path": "/Documents",
      "is_directory": true
    }
  ],
  "breadcrumbs": [
    {
      "name": "Home",
      "path": "/"
    }
  ],
  "current_path": "/Documents",
  "base_path": "/downloads"
}
```

#### Create Folder
```
POST /api/folders
Content-Type: application/json

{
  "path": "/Documents",
  "name": "New Folder"
}
```

**Response:**
```json
{
  "success": true,
  "path": "/Documents/New Folder"
}
```

### Directory Suggestion API

#### Get Directory Suggestion
```
GET /api/directory-suggestion?url={url}
POST /api/directory-suggestion
Content-Type: application/x-www-form-urlencoded

url=https://example.com/file.zip
```

**Response:**
```
/downloads/Movies
```

---

This documentation provides a comprehensive guide to the internal/web package, covering all aspects from basic usage to advanced integration patterns. The package is designed to be maintainable, secure, and user-friendly while providing a modern web interface for the debrid-downloader application.