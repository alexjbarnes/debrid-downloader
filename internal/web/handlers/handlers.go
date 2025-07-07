// Package handlers provides HTTP handlers for the web interface
package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/database"
	"debrid-downloader/internal/downloader"
	"debrid-downloader/internal/folder"
	"debrid-downloader/internal/web/templates"
	"debrid-downloader/pkg/models"
	"github.com/google/uuid"
)

// Handlers contains all HTTP handlers and their dependencies
type Handlers struct {
	db              *database.DB
	allDebridClient alldebrid.AllDebridClient
	folderService   *folder.Service
	downloadWorker  *downloader.Worker
	logger          *slog.Logger
}

// NewHandlers creates a new handlers instance
func NewHandlers(db *database.DB, client alldebrid.AllDebridClient, basePath string, worker *downloader.Worker) *Handlers {
	return &Handlers{
		db:              db,
		allDebridClient: client,
		folderService:   folder.NewService(basePath),
		downloadWorker:  worker,
		logger:          slog.Default(),
	}
}

// Home handles the home page (download form and history)
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Get filename from query parameter for directory suggestions
	filename := r.URL.Query().Get("filename")

	// Get directory suggestions based on filename
	suggestedDir := h.getDirectorySuggestions(filename)

	// Start with empty downloads list - user must select statuses to see results
	var downloads []*models.Download

	component := templates.Base("Debrid Downloader", templates.Home(downloads, suggestedDir))
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render home template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// CurrentDownloads handles HTMX requests for current downloads
func (h *Handlers) CurrentDownloads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Get current downloads (pending, downloading, paused)
	downloads, err := h.db.ListDownloads(10, 0)
	if err != nil {
		h.logger.Error("Failed to get current downloads", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Filter for active downloads only
	var activeDownloads []*models.Download
	for _, download := range downloads {
		if download.Status == models.StatusPending ||
			download.Status == models.StatusDownloading ||
			download.Status == models.StatusPaused {
			activeDownloads = append(activeDownloads, download)
		}
	}

	// Use the wrapper template that includes polling interval update
	component := templates.CurrentDownloadsWithPolling(activeDownloads, len(activeDownloads))
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render current downloads", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// SubmitDownload handles download form submission
func (h *Handlers) SubmitDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		component := templates.DownloadResult(false, "Failed to parse form data")
		if err := component.Render(r.Context(), w); err != nil {
			h.logger.Error("Failed to render component", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	directory := r.FormValue("directory")
	if directory == "" {
		w.WriteHeader(http.StatusBadRequest)
		component := templates.DownloadResult(false, "Directory is required")
		if err := component.Render(r.Context(), w); err != nil {
			h.logger.Error("Failed to render component", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	// Parse URLs - check if it's single or multi-URL submission
	var urls []string

	// Check for multi-URL submission
	multiURLs := r.FormValue("urls")
	if multiURLs != "" {
		// Parse multiple URLs
		urls = h.parseMultipleURLs(multiURLs)
		if len(urls) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			component := templates.DownloadResult(false, "No valid URLs found")
			if err := component.Render(r.Context(), w); err != nil {
				h.logger.Error("Failed to render component", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			return
		}
	} else {
		// Single URL submission
		singleURL := r.FormValue("url")
		if singleURL == "" {
			w.WriteHeader(http.StatusBadRequest)
			component := templates.DownloadResult(false, "URL is required")
			if err := component.Render(r.Context(), w); err != nil {
				h.logger.Error("Failed to render component", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			return
		}
		urls = []string{singleURL}
	}

	var groupID string
	var downloads []*models.Download

	// If multiple URLs, create a group
	if len(urls) > 1 {
		groupID = uuid.New().String()

		// Create download group record
		group := &models.DownloadGroup{
			ID:                 groupID,
			CreatedAt:          time.Now(),
			TotalDownloads:     len(urls),
			CompletedDownloads: 0,
			Status:             models.GroupStatusDownloading,
		}

		if err := h.db.CreateDownloadGroup(group); err != nil {
			h.logger.Error("Failed to create download group", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			component := templates.DownloadResult(false, "Failed to create download group")
			if err := component.Render(r.Context(), w); err != nil {
				h.logger.Error("Failed to render component", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	// Process each URL
	for i, url := range urls {
		// Unrestrict the URL using AllDebrid
		result, err := h.allDebridClient.UnrestrictLink(r.Context(), url)
		if err != nil {
			h.logger.Error("Failed to unrestrict URL", "error", err, "url", url, "group_id", groupID)
			// For multi-URL, continue with other URLs; for single URL, return error
			if len(urls) == 1 {
				w.WriteHeader(http.StatusBadRequest)
				component := templates.DownloadResult(false, fmt.Sprintf("Failed to unrestrict URL: %s", err.Error()))
				if err := component.Render(r.Context(), w); err != nil {
					h.logger.Error("Failed to render component", "error", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				return
			}
			continue // Skip this URL but continue with others
		}

		// Ensure unique filename by checking for existing files
		uniqueFilename := h.ensureUniqueFilename(result.Filename, directory)

		// Detect if this is an archive file
		isArchive := h.isArchiveFile(result.Filename)

		// Create download record
		download := &models.Download{
			OriginalURL:     url,
			UnrestrictedURL: result.UnrestrictedURL,
			Filename:        uniqueFilename,
			Directory:       directory,
			Status:          models.StatusPending,
			Progress:        0.0,
			FileSize:        result.FileSize,
			DownloadedBytes: 0,
			DownloadSpeed:   0.0,
			RetryCount:      0,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
			GroupID:         groupID,
			IsArchive:       isArchive,
			ExtractedFiles:  "",
		}

		if err := h.db.CreateDownload(download); err != nil {
			h.logger.Error("Failed to create download record", "error", err, "url", url, "group_id", groupID)
			if len(urls) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				component := templates.DownloadResult(false, "Failed to create download record")
				if err := component.Render(r.Context(), w); err != nil {
					h.logger.Error("Failed to render component", "error", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				return
			}
			continue // Skip this URL but continue with others
		}

		downloads = append(downloads, download)

		// Create or update directory mapping for future suggestions
		if err := h.createOrUpdateDirectoryMapping(result.Filename, url, directory); err != nil {
			h.logger.Warn("Failed to update directory mapping", "error", err, "filename", result.Filename, "url", url, "directory", directory)
		}

		// Queue the download for processing
		h.downloadWorker.QueueDownload(download.ID)

		h.logger.Info("Download submitted", "url", url, "directory", directory, "filename", result.Filename, "download_id", download.ID, "group_id", groupID, "is_archive", isArchive, "position", i+1, "total", len(urls))
	}

	// Check if any downloads were created
	if len(downloads) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		component := templates.DownloadResult(false, "No downloads could be created")
		if err := component.Render(r.Context(), w); err != nil {
			h.logger.Error("Failed to render component", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	// Create success message
	var successMessage string
	if len(downloads) == 1 {
		successMessage = "Download added to queue successfully"
	} else {
		successMessage = fmt.Sprintf("%d downloads added to queue successfully", len(downloads))
		if groupID != "" {
			successMessage += fmt.Sprintf(" (Group: %s)", groupID[:8]) // Show first 8 chars of group ID
		}
	}

	// Get updated downloads for the list
	allDownloads, err := h.db.ListDownloads(50, 0)
	if err != nil {
		h.logger.Error("Failed to get downloads for refresh", "error", err)
		// Still return success, but without the refresh
		component := templates.DownloadResult(true, successMessage)
		if err := component.Render(r.Context(), w); err != nil {
			h.logger.Error("Failed to render component", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	// Count active downloads for polling logic
	activeCount := 0
	for _, download := range allDownloads {
		if download.Status == models.StatusPending ||
			download.Status == models.StatusDownloading ||
			download.Status == models.StatusPaused {
			activeCount++
		}
	}

	// Get directory suggestions for form reset
	suggestedDir := h.getDirectorySuggestions("")

	// Send empty result div to clear any previous messages
	if _, err := w.Write([]byte(`<div id="result" class="mt-6"></div>`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	// Send success button state as out-of-band swap
	successButton := templates.SubmitButton("success")
	if err := successButton.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render success button", "error", err)
	}

	// Send out-of-band swap to reset the single URL input
	if _, err := w.Write([]byte(`<input type="url" id="url-single" name="url" required placeholder="https://example.com/file.zip" class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 transition-colors" hx-post="/api/directory-suggestion" hx-trigger="keyup changed delay:500ms, paste delay:500ms" hx-target="#directory-suggestion-response" hx-include="this" hx-indicator="#directory-suggestion-indicator" hx-swap-oob="true" value="">`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	// Send out-of-band swap to reset the multi URL textarea
	if _, err := w.Write([]byte(`<textarea id="url-multi" name="urls" rows="6" placeholder="Enter multiple URLs (one per line or space-separated):&#10;https://example.com/file1.zip&#10;https://example.com/file2.zip&#10;https://example.com/file3.zip" class="hidden w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 transition-colors resize-vertical" hx-post="/api/directory-suggestion" hx-trigger="keyup changed delay:500ms, paste delay:500ms" hx-target="#directory-suggestion-response" hx-include="this" hx-indicator="#directory-suggestion-indicator" hx-swap-oob="true"></textarea>`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	// Send out-of-band swap to reset the multifile-mode checkbox
	if _, err := w.Write([]byte(`<input type="checkbox" id="multifile-mode" name="multifile-mode" onchange="toggleMultiFileMode()" class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700" hx-swap-oob="true">`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	// Send out-of-band swap to reset the URL label to single-file mode
	if _, err := w.Write([]byte(`<span id="url-label" hx-swap-oob="true">File URL</span>`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	// Send out-of-band swap to reset the directory fields
	if _, err := w.Write([]byte(`<input type="hidden" id="directory" name="directory" value="`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
	if _, err := w.Write([]byte(suggestedDir)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
	if _, err := w.Write([]byte(`" hx-swap-oob="true">`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	if _, err := w.Write([]byte(`<span id="selected-directory" hx-swap-oob="true">`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
	if _, err := w.Write([]byte(suggestedDir)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
	if _, err := w.Write([]byte(`</span>`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	// Send out-of-band swap to update downloads list
	if _, err := w.Write([]byte(`<div id="downloads-list" class="space-y-4" hx-post="/downloads/search" hx-trigger="load, refresh" hx-include="#search-form" hx-swap="innerHTML" hx-swap-oob="true">`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
	downloadsComponent := templates.DownloadsList(allDownloads)
	if err := downloadsComponent.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render downloads list", "error", err)
		return
	}
	if _, err := w.Write([]byte(`</div>`)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}

	// Also send updated polling trigger
	pollingComponent := templates.DynamicPollingTrigger("polling-trigger", "/downloads/search", "#downloads-list", activeCount)
	if err := pollingComponent.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render polling trigger", "error", err)
		return
	}
}

// getDirectorySuggestions returns directory suggestions based on filename fuzzy matching
func (h *Handlers) getDirectorySuggestions(filename string) string {
	// Use the configured base path as default
	basePath := h.folderService.BasePath

	// Get directory mappings from database
	mappings, err := h.db.GetDirectoryMappings()
	if err != nil {
		h.logger.Error("Failed to get directory mappings", "error", err)
		return basePath
	}

	// If no mappings exist, just return base path
	if len(mappings) == 0 {
		return basePath
	}

	// Score directories based on filename matching
	type dirScore struct {
		directory string
		score     int
		useCount  int
	}

	var scores []dirScore
	filename = strings.ToLower(filename)

	// Check against existing mappings
	for _, mapping := range mappings {
		score := fuzzyMatch(filename, strings.ToLower(mapping.FilenamePattern))
		if score > 0 {
			scores = append(scores, dirScore{
				directory: mapping.Directory,
				score:     score,
				useCount:  mapping.UseCount,
			})
		}
	}

	// Sort by score (descending) then by use count (descending)
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].useCount > scores[j].useCount
		}
		return scores[i].score > scores[j].score
	})

	// Return best match or base path
	if len(scores) > 0 {
		return scores[0].directory
	}

	return basePath
}

// getDirectorySuggestionsForURL returns directory suggestions based on URL fuzzy matching
func (h *Handlers) getDirectorySuggestionsForURL(url string) string {
	// Use the configured base path as default
	basePath := h.folderService.BasePath

	// Get directory mappings from database
	mappings, err := h.db.GetDirectorySuggestionsForURL(url)
	if err != nil {
		h.logger.Error("Failed to get directory suggestions for URL", "error", err, "url", url)
		return basePath
	}

	// If no mappings exist, try to suggest based on URL analysis
	if len(mappings) == 0 {
		return h.getSmartDirectorySuggestion(url, basePath)
	}

	// Score directories based on URL similarity
	type dirScore struct {
		directory string
		score     int
		useCount  int
	}

	var scores []dirScore
	url = strings.ToLower(url)

	// Check against existing mappings
	for _, mapping := range mappings {
		var score int

		// First try URL matching if we have a stored URL
		if mapping.OriginalURL != "" {
			urlScore := fuzzyMatchURL(url, strings.ToLower(mapping.OriginalURL))
			if urlScore > 0 {
				score = urlScore
			}
		}

		// If no URL match, fall back to filename matching
		if score == 0 && mapping.FilenamePattern != "" {
			// Extract filename from URL for comparison
			urlFilename := extractFilenameFromURL(url)
			if urlFilename != "" {
				score = fuzzyMatch(urlFilename, strings.ToLower(mapping.FilenamePattern))
			}
		}

		if score > 0 {
			scores = append(scores, dirScore{
				directory: mapping.Directory,
				score:     score,
				useCount:  mapping.UseCount,
			})
		}
	}

	// Sort by score (descending) then by use count (descending)
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].useCount > scores[j].useCount
		}
		return scores[i].score > scores[j].score
	})

	// Return best match or base path
	if len(scores) > 0 {
		return scores[0].directory
	}

	return basePath
}

// fuzzyMatch returns a score (0-100) for how well filename matches pattern
func fuzzyMatch(filename, pattern string) int {
	if pattern == "" {
		return 0
	}

	// Exact substring match gets highest score
	if strings.Contains(filename, pattern) {
		return 100
	}

	// Check file extension match
	filenameExt := strings.ToLower(filepath.Ext(filename))
	patternExt := strings.ToLower(filepath.Ext(pattern))
	if filenameExt != "" && filenameExt == patternExt {
		return 80
	}

	// Check for partial word matches
	filenameWords := strings.FieldsFunc(filename, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})
	patternWords := strings.FieldsFunc(pattern, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})

	matches := 0
	for _, fw := range filenameWords {
		for _, pw := range patternWords {
			if strings.Contains(fw, pw) || strings.Contains(pw, fw) {
				matches++
				break
			}
		}
	}

	if len(patternWords) > 0 {
		return (matches * 60) / len(patternWords)
	}

	return 0
}

// contentBasedScore returns a score based on content type detection from filename
func (h *Handlers) contentBasedScore(filename, directory string) int {
	filename = strings.ToLower(filename)
	directory = strings.ToLower(directory)

	// Video extensions and keywords
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".m4v"}
	videoKeywords := []string{"movie", "film", "season", "episode", "s01", "s02", "s03", "s04", "s05"}

	// Audio extensions and keywords
	audioExts := []string{".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a"}
	audioKeywords := []string{"album", "song", "music", "audio", "track"}

	// Software extensions and keywords
	softwareExts := []string{".exe", ".msi", ".dmg", ".pkg", ".deb", ".rpm", ".zip", ".tar.gz"}
	softwareKeywords := []string{"setup", "installer", "software", "program", "app"}

	ext := filepath.Ext(filename)

	// Check for video content
	for _, videoExt := range videoExts {
		if ext == videoExt {
			if strings.Contains(directory, "movie") {
				return 90
			}
			if strings.Contains(directory, "tv") {
				return 85
			}
			return 70
		}
	}

	// Check for audio content
	for _, audioExt := range audioExts {
		if ext == audioExt && strings.Contains(directory, "music") {
			return 90
		}
	}

	// Check for software content
	for _, softwareExt := range softwareExts {
		if ext == softwareExt && strings.Contains(directory, "software") {
			return 90
		}
	}

	// Check keywords
	for _, keyword := range videoKeywords {
		if strings.Contains(filename, keyword) {
			if strings.Contains(directory, "movie") || strings.Contains(directory, "tv") {
				return 70
			}
		}
	}

	for _, keyword := range audioKeywords {
		if strings.Contains(filename, keyword) && strings.Contains(directory, "music") {
			return 70
		}
	}

	for _, keyword := range softwareKeywords {
		if strings.Contains(filename, keyword) && strings.Contains(directory, "software") {
			return 70
		}
	}

	// Default score for base downloads directory
	if directory == h.folderService.BasePath {
		return 50
	}

	return 0
}

// createOrUpdateDirectoryMapping creates or updates a directory mapping based on filename pattern and URL
func (h *Handlers) createOrUpdateDirectoryMapping(filename, url, directory string) error {
	// Extract pattern from filename (e.g., extension or keywords)
	pattern := extractPattern(filename)
	if pattern == "" {
		return nil // No meaningful pattern to store
	}

	// Check if a mapping already exists for this pattern and directory
	mappings, err := h.db.GetDirectoryMappings()
	if err != nil {
		return err
	}

	for _, mapping := range mappings {
		if mapping.FilenamePattern == pattern && mapping.Directory == directory {
			// Update existing mapping
			return h.db.UpdateDirectoryMappingUsage(mapping.ID)
		}
	}

	// Create new mapping
	mapping := &models.DirectoryMapping{
		FilenamePattern: pattern,
		OriginalURL:     url,
		Directory:       directory,
		UseCount:        1,
		LastUsed:        time.Now(),
		CreatedAt:       time.Now(),
	}

	return h.db.CreateDirectoryMapping(mapping)
}

// extractPattern extracts a meaningful pattern from filename for directory mapping
func extractPattern(filename string) string {
	filename = strings.ToLower(filename)

	// Get file extension
	ext := filepath.Ext(filename)
	if ext != "" {
		return ext
	}

	// Extract meaningful keywords
	keywords := []string{"movie", "film", "season", "episode", "music", "album", "software", "setup", "installer"}

	for _, keyword := range keywords {
		if strings.Contains(filename, keyword) {
			return keyword
		}
	}

	// Look for season/episode patterns
	if strings.Contains(filename, "s0") && strings.Contains(filename, "e0") {
		return "tv_show"
	}

	return ""
}

// fuzzyMatchURL returns a score (0-100) for how well two URLs match
func fuzzyMatchURL(url1, url2 string) int {
	if url1 == "" || url2 == "" {
		return 0
	}

	// Exact match gets highest score
	if url1 == url2 {
		return 100
	}

	// Extract and compare domain names
	domain1 := extractDomain(url1)
	domain2 := extractDomain(url2)

	if domain1 != "" && domain2 != "" && domain1 == domain2 {
		// Same domain gets a good score
		score := 60

		// Boost score for path similarity
		path1 := extractPath(url1)
		path2 := extractPath(url2)

		if path1 != "" && path2 != "" {
			pathScore := fuzzyMatch(path1, path2)
			score += (pathScore * 40) / 100 // Add up to 40 more points for path similarity
		}

		return score
	}

	// Compare filenames extracted from URLs
	filename1 := extractFilenameFromURL(url1)
	filename2 := extractFilenameFromURL(url2)

	if filename1 != "" && filename2 != "" {
		return fuzzyMatch(filename1, filename2)
	}

	return 0
}

// extractDomain extracts the domain from a URL
func extractDomain(url string) string {
	// Remove protocol
	if strings.HasPrefix(url, "http://") {
		url = url[7:]
	} else if strings.HasPrefix(url, "https://") {
		url = url[8:]
	}

	// Extract domain (everything before first slash)
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		domain := parts[0]
		// Remove port if present
		if colonIndex := strings.Index(domain, ":"); colonIndex != -1 {
			domain = domain[:colonIndex]
		}
		return strings.ToLower(domain)
	}

	return ""
}

// extractPath extracts the path from a URL
func extractPath(url string) string {
	// Remove protocol
	if strings.HasPrefix(url, "http://") {
		url = url[7:]
	} else if strings.HasPrefix(url, "https://") {
		url = url[8:]
	}

	// Find first slash to get path
	slashIndex := strings.Index(url, "/")
	if slashIndex != -1 && slashIndex < len(url)-1 {
		path := url[slashIndex+1:]
		// Remove query parameters
		if questionIndex := strings.Index(path, "?"); questionIndex != -1 {
			path = path[:questionIndex]
		}
		return strings.ToLower(path)
	}

	return ""
}

// extractFilenameFromURL extracts filename from URL
func extractFilenameFromURL(url string) string {
	// Remove query parameters first
	if questionIndex := strings.Index(url, "?"); questionIndex != -1 {
		url = url[:questionIndex]
	}

	// Extract filename (everything after last slash)
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		if filename != "" {
			return strings.ToLower(filename)
		}
	}

	return ""
}

// BrowseFolders handles API requests for folder browsing
func (h *Handlers) BrowseFolders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	directories, err := h.folderService.ListDirectories(path)
	if err != nil {
		h.logger.Error("Failed to list directories", "error", err, "path", path, "basePath", h.folderService.BasePath)
		errorMsg := fmt.Sprintf(`{"error": "Failed to list directories: %s"}`, err.Error())
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	breadcrumbs := h.folderService.GetBreadcrumbs(path)

	response := struct {
		Directories []folder.DirectoryInfo `json:"directories"`
		Breadcrumbs []folder.Breadcrumb    `json:"breadcrumbs"`
		CurrentPath string                 `json:"current_path"`
		BasePath    string                 `json:"base_path"`
	}{
		Directories: directories,
		Breadcrumbs: breadcrumbs,
		CurrentPath: path,
		BasePath:    h.folderService.BasePath,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode folder response", "error", err)
		http.Error(w, `{"error": "Failed to encode response"}`, http.StatusInternalServerError)
		return
	}
}

// CreateFolder handles API requests for creating new folders
func (h *Handlers) CreateFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode create folder request", "error", err)
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, `{"error": "Folder name is required"}`, http.StatusBadRequest)
		return
	}

	// Construct the new folder path
	newFolderPath := req.Path
	if newFolderPath == "" || newFolderPath == "/" {
		newFolderPath = "/" + req.Name
	} else {
		newFolderPath = req.Path + "/" + req.Name
	}

	if err := h.folderService.CreateDirectory(newFolderPath); err != nil {
		h.logger.Error("Failed to create directory", "error", err, "path", newFolderPath, "basePath", h.folderService.BasePath)
		errorMsg := fmt.Sprintf(`{"error": "Failed to create directory: %s"}`, err.Error())
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	response := struct {
		Success bool   `json:"success"`
		Path    string `json:"path"`
	}{
		Success: true,
		Path:    newFolderPath,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode create folder response", "error", err)
		http.Error(w, `{"error": "Failed to encode response"}`, http.StatusInternalServerError)
		return
	}
}

// Settings handles the settings page
func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	component := templates.Base("Settings", templates.Settings())
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render settings template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// SearchDownloads handles HTMX requests for search functionality
func (h *Handlers) SearchDownloads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := r.ParseForm(); err != nil {
		h.logger.Error("Failed to parse search form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	searchTerm := r.FormValue("search")
	statusFilters := r.Form["status"] // Get array of selected statuses
	sortOrder := r.FormValue("sort")
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Get filtered downloads from database
	downloads, err := h.db.SearchDownloads(searchTerm, statusFilters, sortOrder, 50, 0)
	if err != nil {
		h.logger.Error("Failed to search downloads", "error", err, "search", searchTerm, "status", statusFilters, "sort", sortOrder)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Count active downloads for polling logic
	activeCount := 0
	for _, download := range downloads {
		if download.Status == models.StatusPending ||
			download.Status == models.StatusDownloading ||
			download.Status == models.StatusPaused {
			activeCount++
		}
	}

	// Use the wrapper template that includes polling interval update
	component := templates.DownloadsListWithPolling(downloads, activeCount)
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render downloads list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// RetryDownload handles retrying a failed download
func (h *Handlers) RetryDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Extract download ID from URL path parameter
	idStr := r.PathValue("id")
	downloadID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid download ID in retry request", "id", idStr, "error", err)
		http.Error(w, "Invalid download ID", http.StatusBadRequest)
		return
	}

	// Get download from database
	download, err := h.db.GetDownload(downloadID)
	if err != nil {
		h.logger.Error("Failed to get download for retry", "download_id", downloadID, "error", err)
		http.Error(w, "Download not found", http.StatusNotFound)
		return
	}

	// Check if download can be retried
	if download.Status != models.StatusFailed {
		h.logger.Warn("Attempted to retry non-failed download", "download_id", downloadID, "status", download.Status)
		http.Error(w, "Download is not in failed state", http.StatusBadRequest)
		return
	}

	if download.RetryCount >= 5 {
		h.logger.Warn("Attempted to retry download that has exceeded retry limit", "download_id", downloadID, "retry_count", download.RetryCount)
		http.Error(w, "Download has exceeded retry limit", http.StatusBadRequest)
		return
	}

	// Reset download status and queue it
	download.Status = models.StatusPending
	download.ErrorMessage = ""
	download.UpdatedAt = time.Now()

	if err := h.db.UpdateDownload(download); err != nil {
		h.logger.Error("Failed to update download for retry", "download_id", downloadID, "error", err)
		http.Error(w, "Failed to update download", http.StatusInternalServerError)
		return
	}

	// Queue the download for processing
	h.downloadWorker.QueueDownload(downloadID)

	h.logger.Info("Download queued for retry", "download_id", downloadID, "retry_count", download.RetryCount)

	// Render the updated download item
	component := templates.DownloadItem(download)
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render retry download result", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// GetDirectorySuggestion handles HTMX requests for directory suggestions based on URL
func (h *Handlers) GetDirectorySuggestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	// Try both query parameter and form data
	var url string
	if r.Method == "POST" {
		if err := r.ParseForm(); err == nil {
			// Check for single URL
			url = r.FormValue("url")

			// If no single URL, check for multi-URLs and use the first one
			if url == "" {
				multiURLs := r.FormValue("urls")
				if multiURLs != "" {
					urls := h.parseMultipleURLs(multiURLs)
					if len(urls) > 0 {
						url = urls[0] // Use first URL for suggestion
					}
				}
			}
		}
	}

	// Try query parameter if no form data
	if url == "" {
		url = r.URL.Query().Get("url")
	}

	if url == "" {
		// Return base path if no URL provided
		if _, err := w.Write([]byte(h.folderService.BasePath)); err != nil {
			h.logger.Error("Failed to write response", "error", err)
		}
		return
	}

	// Get directory suggestion based on URL fuzzy matching
	suggestedDir := h.getDirectorySuggestionsForURL(url)
	if _, err := w.Write([]byte(suggestedDir)); err != nil {
		h.logger.Error("Failed to write response", "error", err)
	}
}

// CreateTestFailedDownload creates a test failed download for testing retry functionality
func (h *Handlers) CreateTestFailedDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Create a test failed download
	download := &models.Download{
		OriginalURL:     "https://example.com/test-failed-file.zip",
		UnrestrictedURL: "https://example.com/test-failed-file.zip",
		Filename:        "test-failed-file.zip",
		Directory:       h.folderService.BasePath,
		Status:          models.StatusFailed,
		Progress:        0.0,
		FileSize:        1024000,
		DownloadedBytes: 0,
		DownloadSpeed:   0.0,
		ErrorMessage:    "Test failed download for retry testing",
		RetryCount:      2,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := h.db.CreateDownload(download); err != nil {
		h.logger.Error("Failed to create test download", "error", err)
		http.Error(w, `{"error": "Failed to create test download"}`, http.StatusInternalServerError)
		return
	}

	h.logger.Info("Test failed download created", "download_id", download.ID)

	response := struct {
		Success    bool   `json:"success"`
		DownloadID int64  `json:"download_id"`
		Message    string `json:"message"`
	}{
		Success:    true,
		DownloadID: download.ID,
		Message:    "Test failed download created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
		http.Error(w, `{"error": "Failed to encode response"}`, http.StatusInternalServerError)
		return
	}
}

// PauseDownload handles pausing an active download
func (h *Handlers) PauseDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Extract download ID from URL path parameter
	idStr := r.PathValue("id")
	downloadID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid download ID in pause request", "id", idStr, "error", err)
		http.Error(w, "Invalid download ID", http.StatusBadRequest)
		return
	}

	// Pause the download
	if err := h.downloadWorker.PauseCurrentDownload(); err != nil {
		h.logger.Error("Failed to pause download", "download_id", downloadID, "error", err)
		http.Error(w, "Failed to pause download", http.StatusInternalServerError)
		return
	}

	// Get updated download from database
	download, err := h.db.GetDownload(downloadID)
	if err != nil {
		h.logger.Error("Failed to get download after pause", "download_id", downloadID, "error", err)
		http.Error(w, "Download not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Download paused", "download_id", downloadID)

	// Render the updated download item
	component := templates.DownloadItem(download)
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render paused download", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// ResumeDownload handles resuming a paused download
func (h *Handlers) ResumeDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Extract download ID from URL path parameter
	idStr := r.PathValue("id")
	downloadID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid download ID in resume request", "id", idStr, "error", err)
		http.Error(w, "Invalid download ID", http.StatusBadRequest)
		return
	}

	// Resume the download
	if err := h.downloadWorker.ResumeDownload(downloadID); err != nil {
		h.logger.Error("Failed to resume download", "download_id", downloadID, "error", err)
		http.Error(w, "Failed to resume download", http.StatusInternalServerError)
		return
	}

	// Get updated download from database
	download, err := h.db.GetDownload(downloadID)
	if err != nil {
		h.logger.Error("Failed to get download after resume", "download_id", downloadID, "error", err)
		http.Error(w, "Download not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Download resumed", "download_id", downloadID)

	// Render the updated download item
	component := templates.DownloadItem(download)
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("Failed to render resumed download", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// DeleteDownload handles deleting a download record (keeps the file)
func (h *Handlers) DeleteDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Extract download ID from URL path parameter
	idStr := r.PathValue("id")
	downloadID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid download ID in delete request", "id", idStr, "error", err)
		http.Error(w, "Invalid download ID", http.StatusBadRequest)
		return
	}

	// Get download info before deleting for cleanup
	download, err := h.db.GetDownload(downloadID)
	if err != nil {
		h.logger.Error("Failed to get download for deletion", "download_id", downloadID, "error", err)
		http.Error(w, "Download not found", http.StatusNotFound)
		return
	}

	// Check if this was an active download that needs to trigger queue processing
	wasActive := download.Status == models.StatusPending || 
		download.Status == models.StatusDownloading || 
		download.Status == models.StatusPaused
	
	h.logger.Info("Checking download status before deletion", 
		"download_id", downloadID,
		"status", download.Status,
		"was_active", wasActive)

	// If this is the currently downloading item, cancel it first
	if download.Status == models.StatusDownloading {
		wasCanceled := h.downloadWorker.CancelCurrentDownloadIfMatches(downloadID)
		h.logger.Info("Attempted to cancel current download", 
			"download_id", downloadID, 
			"was_canceled", wasCanceled)
		
		// Give the worker a moment to process the cancellation
		if wasCanceled {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Delete from database (this will remove the history record)
	if err := h.db.DeleteDownload(downloadID); err != nil {
		h.logger.Error("Failed to delete download", "download_id", downloadID, "error", err)
		http.Error(w, "Failed to delete download", http.StatusInternalServerError)
		return
	}

	// Clean up temporary file if it exists (but keep final file)
	tempFilename := fmt.Sprintf("%s.%d.tmp", download.Filename, download.ID)
	tempPath := filepath.Join(download.Directory, tempFilename)
	if _, err := os.Stat(tempPath); err == nil {
		if removeErr := os.Remove(tempPath); removeErr != nil {
			h.logger.Warn("Failed to clean up temporary file", "temp_path", tempPath, "error", removeErr)
		}
	}

	h.logger.Info("Download deleted from history", "download_id", downloadID, "filename", download.Filename)

	// If this was an active download, check for and queue next pending download
	if wasActive {
		h.logger.Info("Active download was canceled, checking for next pending download", 
			"canceled_id", downloadID, 
			"canceled_status", download.Status)
		// Add a small delay to ensure the worker has finished processing the cancellation
		time.Sleep(200 * time.Millisecond)
		h.queueNextPendingDownload()
	}

	// Return empty response to remove the item from DOM
	w.WriteHeader(http.StatusOK)
}

// ensureUniqueFilename checks if a file exists and generates a unique filename if needed
func (h *Handlers) ensureUniqueFilename(filename, directory string) string {
	originalName := filename
	counter := 1

	for {
		// Check if file exists at the target location
		fullPath := filepath.Join(directory, filename)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// File doesn't exist, we can use this name
			break
		}

		// File exists, generate a new name
		ext := filepath.Ext(originalName)
		nameWithoutExt := strings.TrimSuffix(originalName, ext)
		filename = fmt.Sprintf("%s(%d)%s", nameWithoutExt, counter, ext)
		counter++

		// Safety check to prevent infinite loop
		if counter > 1000 {
			h.logger.Warn("Too many filename conflicts, using timestamp", "original", originalName, "directory", directory)
			timestamp := time.Now().Unix()
			filename = fmt.Sprintf("%s_%d%s", nameWithoutExt, timestamp, ext)
			break
		}
	}

	if filename != originalName {
		h.logger.Info("Generated unique filename", "original", originalName, "unique", filename, "directory", directory)
	}

	return filename
}

// parseMultipleURLs parses a string containing multiple URLs separated by whitespace or newlines
func (h *Handlers) parseMultipleURLs(input string) []string {
	var urls []string

	// Split on both newlines and spaces, then filter for HTTP URLs
	lines := strings.Fields(strings.ReplaceAll(input, "\n", " "))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && (strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://")) {
			urls = append(urls, line)
		}
	}

	return urls
}

// isArchiveFile checks if a filename is an archive that should be extracted
func (h *Handlers) isArchiveFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	lowerFilename := strings.ToLower(filename)
	
	// For RAR files, be more selective about multi-part archives
	if ext == ".rar" {
		// If it's a multi-part RAR, only mark the first part as an archive
		if strings.Contains(lowerFilename, ".part") {
			return strings.Contains(lowerFilename, ".part1.rar") || 
				   strings.Contains(lowerFilename, ".part01.rar") || 
				   strings.Contains(lowerFilename, ".part001.rar")
		}
		// Single RAR files are archives
		return true
	}
	
	// Other archive formats
	archiveExts := []string{".zip", ".7z", ".tar", ".gz", ".bz2", ".xz"}
	for _, archiveExt := range archiveExts {
		if ext == archiveExt {
			return true
		}
	}

	// Check for compound extensions like .tar.gz
	if strings.HasSuffix(lowerFilename, ".tar.gz") ||
		strings.HasSuffix(lowerFilename, ".tar.bz2") ||
		strings.HasSuffix(lowerFilename, ".tar.xz") {
		return true
	}

	return false
}

// getSmartDirectorySuggestion analyzes URL to suggest appropriate directory
func (h *Handlers) getSmartDirectorySuggestion(url, basePath string) string {
	// Extract filename from URL
	filename := extractFilenameFromURL(url)

	if filename == "" {
		return basePath
	}

	// Convert to lowercase for analysis
	filename = strings.ToLower(filename)
	url = strings.ToLower(url)

	// Extract domain for additional analysis
	domain := extractDomain(url)

	// Define category mappings
	categories := map[string][]string{
		"Movies": {
			".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v",
			"movie", "film", "cinema", "dvdrip", "bluray", "hdtv", "webrip",
		},
		"TV Shows": {
			"season", "episode", "s01", "s02", "s03", "s04", "s05", "e01", "e02",
			"series", "tv", "show", "hdtv", "webrip",
		},
		"Music": {
			".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a", ".wma",
			"album", "music", "song", "track", "artist", "band",
		},
		"Software": {
			".exe", ".msi", ".dmg", ".pkg", ".deb", ".rpm", ".appimage",
			"software", "program", "app", "installer", "setup", "portable",
		},
		"Games": {
			"game", "steam", "gog", "origin", "epic", "uplay", "crack", "repack",
			".iso", "setup.exe",
		},
		"Books": {
			".pdf", ".epub", ".mobi", ".azw", ".azw3", ".djvu",
			"book", "ebook", "novel", "manual", "guide",
		},
		"Archives": {
			".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz",
		},
	}

	// Score each category
	bestCategory := ""
	bestScore := 0

	for category, keywords := range categories {
		score := 0
		for _, keyword := range keywords {
			if strings.Contains(filename, keyword) || strings.Contains(url, keyword) {
				if strings.HasPrefix(keyword, ".") {
					// File extension gets higher score
					score += 10
				} else {
					// Keyword gets regular score
					score += 5
				}
			}
		}

		// Domain-specific scoring
		if domain != "" {
			switch {
			case strings.Contains(domain, "torrent") && category == "Movies":
				score += 3
			case strings.Contains(domain, "music") && category == "Music":
				score += 3
			case strings.Contains(domain, "software") && category == "Software":
				score += 3
			case strings.Contains(domain, "game") && category == "Games":
				score += 3
			}
		}

		if score > bestScore {
			bestScore = score
			bestCategory = category
		}
	}

	// Return suggested directory path
	if bestCategory != "" && bestScore >= 5 {
		return filepath.Join(basePath, bestCategory)
	}

	// Fallback to base path
	return basePath
}

// queueNextPendingDownload checks for pending downloads and queues the next one
func (h *Handlers) queueNextPendingDownload() {
	h.logger.Debug("Checking for next pending download to queue")
	
	// Get pending downloads ordered by creation time (oldest first)
	pendingDownloads, err := h.db.GetPendingDownloadsOldestFirst()
	if err != nil {
		h.logger.Error("Failed to get pending downloads for queue check", "error", err)
		return
	}

	h.logger.Debug("Found pending downloads", "count", len(pendingDownloads))

	// Queue the first (oldest) pending download
	if len(pendingDownloads) > 0 {
		download := pendingDownloads[0]
		h.downloadWorker.QueueDownload(download.ID)
		h.logger.Info("Queued next pending download", 
			"download_id", download.ID, 
			"filename", download.Filename,
			"created_at", download.CreatedAt)
	} else {
		h.logger.Info("No pending downloads found to queue")
	}
}
