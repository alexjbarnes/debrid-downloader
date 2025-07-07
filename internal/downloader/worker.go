// Package downloader implements the download queue and worker functionality
package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"debrid-downloader/internal/cleanup"
	"debrid-downloader/internal/database"
	"debrid-downloader/internal/extractor"
	"debrid-downloader/pkg/models"
)

// SpeedHistory implements wget-style speed smoothing using a ring buffer
type SpeedHistory struct {
	samples    []SpeedSample
	pos        int
	size       int
	totalBytes int64
	totalTime  float64
}

type SpeedSample struct {
	bytes int64
	time  float64
}

const (
	SPEED_HISTORY_SIZE  = 20   // Number of samples in ring buffer (wget uses 20)
	SAMPLE_MIN_DURATION = 0.15 // Minimum 150ms between samples (wget default)
	STALL_THRESHOLD     = 5.0  // Seconds before considering stalled
)

// NewSpeedHistory creates a new speed history tracker
func NewSpeedHistory() *SpeedHistory {
	return &SpeedHistory{
		samples: make([]SpeedSample, SPEED_HISTORY_SIZE),
		pos:     0,
		size:    0,
	}
}

// AddSample adds a new speed sample to the ring buffer
func (sh *SpeedHistory) AddSample(bytes int64, duration float64) {
	if duration < SAMPLE_MIN_DURATION {
		return // Don't add samples that are too short
	}

	// Remove the oldest sample if we're at capacity
	if sh.size == SPEED_HISTORY_SIZE {
		oldSample := sh.samples[sh.pos]
		sh.totalBytes -= oldSample.bytes
		sh.totalTime -= oldSample.time
	} else {
		sh.size++
	}

	// Add new sample
	sh.samples[sh.pos] = SpeedSample{
		bytes: bytes,
		time:  duration,
	}
	sh.totalBytes += bytes
	sh.totalTime += duration

	// Move to next position in ring
	sh.pos = (sh.pos + 1) % SPEED_HISTORY_SIZE
}

// CalculateSpeed returns the smoothed download speed in bytes per second
func (sh *SpeedHistory) CalculateSpeed(recentBytes int64, recentTime float64) float64 {
	if sh.size == 0 && recentTime <= 0 {
		return 0
	}

	totalBytes := sh.totalBytes + recentBytes
	totalTime := sh.totalTime + recentTime

	if totalTime <= 0 {
		return 0
	}

	return float64(totalBytes) / totalTime
}

// Worker manages the download queue and processes downloads sequentially
type Worker struct {
	db        *database.DB
	logger    *slog.Logger
	queue     chan int64 // Channel for download IDs
	extractor *extractor.Service
	cleanup   *cleanup.Service
	mu        sync.RWMutex

	// Current download state
	currentDownload *models.Download
	cancel          context.CancelFunc
	paused          bool
}

// NewWorker creates a new download worker
func NewWorker(db *database.DB, baseDownloadPath string) *Worker {
	return &Worker{
		db:        db,
		logger:    slog.Default(),
		queue:     make(chan int64, 100), // Buffer for up to 100 queued downloads
		extractor: extractor.NewService(),
		cleanup:   cleanup.NewService(db, baseDownloadPath),
	}
}

// Start begins processing the download queue
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("Starting download worker")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Download worker shutting down")
			return
		case downloadID := <-w.queue:
			w.processDownload(ctx, downloadID)
		}
	}
}

// QueueDownload adds a download to the processing queue
func (w *Worker) QueueDownload(downloadID int64) {
	select {
	case w.queue <- downloadID:
		w.logger.Info("Download queued", "download_id", downloadID)
	default:
		w.logger.Error("Download queue is full", "download_id", downloadID)
	}
}

// GetCurrentDownload returns information about the currently processing download
func (w *Worker) GetCurrentDownload() *models.Download {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentDownload
}

// PauseCurrentDownload pauses the current download
func (w *Worker) PauseCurrentDownload() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentDownload == nil {
		return fmt.Errorf("no download currently in progress")
	}

	w.paused = true
	if w.cancel != nil {
		w.cancel()
	}

	// Update database status to paused and record pause time
	w.currentDownload.Status = models.StatusPaused
	now := time.Now()
	w.currentDownload.UpdatedAt = now
	w.currentDownload.PausedAt = &now

	if err := w.db.UpdateDownload(w.currentDownload); err != nil {
		w.logger.Error("Failed to update paused download status", "error", err)
		return err
	}

	w.logger.Info("Download paused", "download_id", w.currentDownload.ID)
	return nil
}

// ResumeDownload resumes a paused download by re-queuing it
func (w *Worker) ResumeDownload(downloadID int64) error {
	download, err := w.db.GetDownload(downloadID)
	if err != nil {
		return fmt.Errorf("failed to get download: %w", err)
	}

	if download.Status != models.StatusPaused {
		return fmt.Errorf("download is not paused")
	}

	// Calculate paused duration if we have a pause timestamp
	if download.PausedAt != nil {
		pausedDuration := time.Since(*download.PausedAt)
		download.TotalPausedTime += int64(pausedDuration.Seconds())
		download.PausedAt = nil // Clear pause timestamp
	}

	// Update status back to pending and queue it
	download.Status = models.StatusPending
	download.UpdatedAt = time.Now()

	if err := w.db.UpdateDownload(download); err != nil {
		return fmt.Errorf("failed to update download status: %w", err)
	}

	w.QueueDownload(downloadID)
	w.logger.Info("Download resumed", "download_id", downloadID)
	return nil
}

// CancelCurrentDownloadIfMatches cancels the current download if it matches the given ID
func (w *Worker) CancelCurrentDownloadIfMatches(downloadID int64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentDownload != nil && w.currentDownload.ID == downloadID {
		w.logger.Info("Canceling current download", "download_id", downloadID)
		if w.cancel != nil {
			w.cancel()
		}
		return true
	}
	return false
}

// processDownload handles the actual downloading of a file
func (w *Worker) processDownload(ctx context.Context, downloadID int64) {
	// Get download details from database
	download, err := w.db.GetDownload(downloadID)
	if err != nil {
		w.logger.Error("Failed to get download", "download_id", downloadID, "error", err)
		return
	}

	// Set as current download
	w.mu.Lock()
	w.currentDownload = download
	w.paused = false
	w.mu.Unlock()

	// Defer cleanup
	defer func() {
		w.mu.Lock()
		w.currentDownload = nil
		w.cancel = nil
		w.mu.Unlock()
	}()

	// Start download with retry logic
	maxRetries := 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2^attempt seconds
			backoffDuration := time.Duration(1<<uint(attempt)) * time.Second
			w.logger.Info("Retrying download after backoff",
				"download_id", downloadID,
				"attempt", attempt,
				"backoff", backoffDuration)

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoffDuration):
			}

			// Check if the download was deleted during the backoff period
			if _, err := w.db.GetDownload(downloadID); err != nil {
				w.logger.Info("Download was deleted during retry backoff, stopping processing",
					"download_id", downloadID)
				return
			}
		}

		// Create cancellable context for this download attempt
		downloadCtx, cancel := context.WithCancel(ctx)
		w.mu.Lock()
		w.cancel = cancel
		w.mu.Unlock()

		err := w.downloadFile(downloadCtx, download)
		cancel()

		if err == nil {
			// Success!
			w.logger.Info("Download completed successfully", "download_id", downloadID)

			// Check if this download is part of a group and handle group completion
			if download.GroupID != "" {
				w.checkGroupCompletion(download.GroupID)
			}

			return
		}

		// Check if we were paused
		w.mu.RLock()
		isPaused := w.paused
		w.mu.RUnlock()

		if isPaused {
			w.logger.Info("Download was paused", "download_id", downloadID)
			return
		}

		// Update retry count
		download.RetryCount = attempt + 1
		download.ErrorMessage = err.Error()
		download.UpdatedAt = time.Now()

		if attempt < maxRetries {
			download.Status = models.StatusPending
			w.logger.Warn("Download attempt failed, will retry",
				"download_id", downloadID,
				"attempt", attempt+1,
				"error", err)
		} else {
			download.Status = models.StatusFailed
			completedAt := time.Now()
			download.CompletedAt = &completedAt
			w.logger.Error("Download failed after all retries",
				"download_id", downloadID,
				"error", err)
		}

		if updateErr := w.db.UpdateDownload(download); updateErr != nil {
			w.logger.Error("Failed to update download after attempt",
				"download_id", downloadID,
				"error", updateErr)
		}

		// If we've exhausted retries, clean up temporary file and stop
		if attempt >= maxRetries {
			// Clean up temporary file for this download
			tempFilename := fmt.Sprintf("%s.%d.tmp", download.Filename, download.ID)
			tempPath := filepath.Join(download.Directory, tempFilename)
			if _, err := os.Stat(tempPath); err == nil {
				if removeErr := os.Remove(tempPath); removeErr != nil {
					w.logger.Warn("Failed to clean up temporary file", "temp_path", tempPath, "error", removeErr)
				} else {
					w.logger.Info("Cleaned up temporary file after failed download", "temp_path", tempPath)
				}
			}
			break
		}
	}
}

// downloadFile performs the actual file download with progress tracking
func (w *Worker) downloadFile(ctx context.Context, download *models.Download) error {
	// Update status to downloading
	download.Status = models.StatusDownloading
	download.UpdatedAt = time.Now()

	if err := w.db.UpdateDownload(download); err != nil {
		return fmt.Errorf("failed to update download status: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", download.UnrestrictedURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Use unique temporary filename during download to prevent conflicts
	tempFilename := fmt.Sprintf("%s.%d.tmp", download.Filename, download.ID)
	tempPath := filepath.Join(download.Directory, tempFilename)
	finalPath := filepath.Join(download.Directory, download.Filename)

	// Check if we have partial download and can resume
	var resumeFrom int64
	if stat, err := os.Stat(tempPath); err == nil {
		resumeFrom = stat.Size()
		if resumeFrom > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
			w.logger.Info("Resuming download from byte", "download_id", download.ID, "resume_from", resumeFrom)
		}
	}

	// Make the request with longer timeout for large file downloads
	client := &http.Client{
		Timeout: 1 * time.Hour, // Allow up to 1 hour for downloads
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Get content length for progress tracking
	contentLength := resp.ContentLength
	if contentLength > 0 && download.FileSize == 0 {
		download.FileSize = contentLength + resumeFrom
		download.UpdatedAt = time.Now()
		if err := w.db.UpdateDownload(download); err != nil {
			w.logger.Warn("Failed to update file size", "error", err)
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(download.Directory, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open temporary destination file
	var file *os.File
	if resumeFrom > 0 {
		file, err = os.OpenFile(tempPath, os.O_APPEND|os.O_WRONLY, 0o644)
	} else {
		file, err = os.Create(tempPath)
	}

	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Set start time right before actual download begins (only for new downloads)
	if download.StartedAt == nil {
		startedAt := time.Now()
		download.StartedAt = &startedAt
	}

	// Download with progress tracking
	err = w.copyWithProgress(ctx, file, resp.Body, download, resumeFrom)
	if err != nil {
		return err
	}

	// Close the file before renaming
	file.Close()

	// Move temporary file to final location on successful completion
	if err := os.Rename(tempPath, finalPath); err != nil {
		w.logger.Error("Failed to rename completed download", "temp_path", tempPath, "final_path", finalPath, "error", err)
		return fmt.Errorf("failed to move completed file: %w", err)
	}

	w.logger.Info("Download completed and moved to final location", "download_id", download.ID, "final_path", finalPath)
	return nil
}

// copyWithProgress copies data while tracking progress and updating the database
func (w *Worker) copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, download *models.Download, resumeFrom int64) error {
	buffer := make([]byte, 32*1024) // 32KB buffer
	var totalRead int64 = resumeFrom

	// Initialize wget-style speed tracking
	speedHistory := NewSpeedHistory()
	lastUpdate := time.Now()
	lastSampleTime := lastUpdate
	lastSampleBytes := resumeFrom

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := src.Read(buffer)
		if n > 0 {
			_, writeErr := dst.Write(buffer[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write to file: %w", writeErr)
			}

			totalRead += int64(n)

			// Update progress every 500ms for smooth progress viewing
			now := time.Now()
			if now.Sub(lastUpdate) >= 500*time.Millisecond {
				// Add speed sample to history if enough time has passed
				timeSinceSample := now.Sub(lastSampleTime).Seconds()
				if timeSinceSample >= SAMPLE_MIN_DURATION {
					bytesSinceSample := totalRead - lastSampleBytes
					speedHistory.AddSample(bytesSinceSample, timeSinceSample)
					lastSampleTime = now
					lastSampleBytes = totalRead
				}

				// Calculate smoothed speed using wget-style algorithm
				recentTime := now.Sub(lastSampleTime).Seconds()
				recentBytes := totalRead - lastSampleBytes
				speed := speedHistory.CalculateSpeed(recentBytes, recentTime)

				// Calculate progress percentage
				var progress float64
				if download.FileSize > 0 {
					progress = float64(totalRead) / float64(download.FileSize) * 100
				}

				// Update download record
				download.DownloadedBytes = totalRead
				download.Progress = progress
				download.DownloadSpeed = speed
				download.UpdatedAt = now

				if updateErr := w.db.UpdateDownload(download); updateErr != nil {
					w.logger.Warn("Failed to update download progress", "error", updateErr)
				}

				w.logger.Debug("Download progress",
					"download_id", download.ID,
					"progress", fmt.Sprintf("%.1f%%", progress),
					"speed", fmt.Sprintf("%.1f KB/s", speed/1024))

				lastUpdate = now
			}
		}

		if err != nil {
			if err == io.EOF {
				// Download completed successfully
				download.Status = models.StatusCompleted
				download.Progress = 100.0
				download.DownloadedBytes = totalRead

				// Calculate overall download speed for completed download
				if download.StartedAt != nil {
					totalDuration := time.Since(*download.StartedAt).Seconds()
					// Subtract paused time from total duration for accurate speed calculation
					activeDuration := totalDuration - float64(download.TotalPausedTime)
					if activeDuration > 0 {
						// Use total downloaded bytes for accurate average speed calculation
						download.DownloadSpeed = float64(download.DownloadedBytes) / activeDuration
					}
				}

				completedAt := time.Now()
				download.CompletedAt = &completedAt
				download.UpdatedAt = completedAt

				if updateErr := w.db.UpdateDownload(download); updateErr != nil {
					w.logger.Error("Failed to update completed download", "error", updateErr)
				}

				return nil
			}
			return fmt.Errorf("failed to read from response: %w", err)
		}
	}
}

// checkGroupCompletion checks if all downloads in a group are complete and triggers post-processing
func (w *Worker) checkGroupCompletion(groupID string) {
	// Get the group from database
	group, err := w.db.GetDownloadGroup(groupID)
	if err != nil {
		w.logger.Error("Failed to get download group", "group_id", groupID, "error", err)
		return
	}

	// Get all downloads in this group
	downloads, err := w.db.GetDownloadsByGroupID(groupID)
	if err != nil {
		w.logger.Error("Failed to get downloads by group ID", "group_id", groupID, "error", err)
		return
	}

	// Count completed downloads
	completedCount := 0
	for _, download := range downloads {
		if download.Status == models.StatusCompleted {
			completedCount++
		}
	}

	// Update group progress
	group.CompletedDownloads = completedCount
	if err := w.db.UpdateDownloadGroup(group); err != nil {
		w.logger.Error("Failed to update download group progress", "group_id", groupID, "error", err)
		return
	}

	w.logger.Info("Group progress updated", "group_id", groupID, "completed", completedCount, "total", group.TotalDownloads)

	// If all downloads are complete, start post-processing
	if completedCount >= group.TotalDownloads {
		w.logger.Info("All downloads in group completed, starting post-processing", "group_id", groupID)

		// Update group status to processing
		group.Status = models.GroupStatusProcessing
		if err := w.db.UpdateDownloadGroup(group); err != nil {
			w.logger.Error("Failed to update group status to processing", "group_id", groupID, "error", err)
			return
		}

		// Process the group asynchronously to avoid blocking the download worker
		go w.processGroup(groupID)
	}
}

// processGroup handles post-download processing for a completed group
func (w *Worker) processGroup(groupID string) {
	w.logger.Info("Starting group post-processing", "group_id", groupID)

	// Get all completed downloads in this group
	downloads, err := w.db.GetDownloadsByGroupID(groupID)
	if err != nil {
		w.logger.Error("Failed to get downloads for group processing", "group_id", groupID, "error", err)
		w.markGroupFailed(groupID, fmt.Sprintf("Failed to get downloads: %s", err.Error()))
		return
	}

	// Filter for completed downloads
	var completedDownloads []*models.Download
	allCompleted := true
	statusCount := make(map[models.DownloadStatus]int)

	for _, download := range downloads {
		statusCount[download.Status]++
		if download.Status == models.StatusCompleted {
			completedDownloads = append(completedDownloads, download)
		} else if download.Status != models.StatusFailed {
			// If any download is still pending/downloading, don't process yet
			allCompleted = false
		}
	}

	w.logger.Info("Group status summary", "group_id", groupID, "total", len(downloads), "status_counts", statusCount)

	if !allCompleted {
		w.logger.Info("Not all downloads in group are completed yet", "group_id", groupID, "completed", len(completedDownloads), "total", len(downloads))

		// Log details of incomplete downloads
		for _, download := range downloads {
			if download.Status != models.StatusCompleted && download.Status != models.StatusFailed {
				w.logger.Info("Incomplete download", "download_id", download.ID, "filename", download.Filename, "status", download.Status, "progress", download.Progress)
			}
		}
		return
	}

	// Now filter for archives that should be processed
	var archiveDownloads []*models.Download
	processedMultiparts := make(map[string]bool)

	for _, download := range completedDownloads {
		if download.IsArchive {
			filename := strings.ToLower(download.Filename)

			// Check if it's a multi-part RAR
			if strings.HasSuffix(filename, ".rar") && strings.Contains(filename, ".part") {
				// Extract the base name (everything before .partX.rar)
				baseName := filename
				if idx := strings.Index(filename, ".part"); idx > 0 {
					baseName = filename[:idx]
				}

				// Only process the first part
				if strings.Contains(filename, ".part1.rar") ||
					strings.Contains(filename, ".part01.rar") ||
					strings.Contains(filename, ".part001.rar") {
					if !processedMultiparts[baseName] {
						archiveDownloads = append(archiveDownloads, download)
						processedMultiparts[baseName] = true
						w.logger.Info("Adding multi-part RAR for processing", "filename", download.Filename)
					}
				} else {
					w.logger.Info("Skipping non-first part of multi-part RAR", "filename", download.Filename)
				}
			} else if w.extractor.IsArchive(download.Filename) {
				// For non-multipart archives or other archive types
				archiveDownloads = append(archiveDownloads, download)
				w.logger.Info("Adding archive for processing", "filename", download.Filename)
			}
		}
	}

	if len(archiveDownloads) == 0 {
		w.logger.Info("No archive files to process in group", "group_id", groupID)
		w.markGroupCompleted(groupID)
		return
	}

	w.logger.Info("Processing archives in group", "group_id", groupID, "archive_count", len(archiveDownloads))

	// Process each archive download
	successCount := 0
	for _, download := range archiveDownloads {
		if err := w.processArchive(download); err != nil {
			w.logger.Error("Failed to process archive", "download_id", download.ID, "filename", download.Filename, "error", err)
			// Continue with other archives even if one fails
		} else {
			successCount++
		}
	}

	if successCount > 0 {
		w.logger.Info("Archive processing completed", "group_id", groupID, "successful", successCount, "total", len(archiveDownloads))
		w.markGroupCompleted(groupID)
	} else {
		w.logger.Error("No archives could be processed", "group_id", groupID)
		w.markGroupFailed(groupID, "Failed to process any archive files")
	}
}

// markGroupCompleted marks a group as successfully completed
func (w *Worker) markGroupCompleted(groupID string) {
	group, err := w.db.GetDownloadGroup(groupID)
	if err != nil {
		w.logger.Error("Failed to get group for completion", "group_id", groupID, "error", err)
		return
	}

	group.Status = models.GroupStatusCompleted
	if err := w.db.UpdateDownloadGroup(group); err != nil {
		w.logger.Error("Failed to mark group as completed", "group_id", groupID, "error", err)
		return
	}

	w.logger.Info("Group processing completed successfully", "group_id", groupID)
}

// markGroupFailed marks a group as failed with an error message
func (w *Worker) markGroupFailed(groupID string, errorMessage string) {
	group, err := w.db.GetDownloadGroup(groupID)
	if err != nil {
		w.logger.Error("Failed to get group for failure marking", "group_id", groupID, "error", err)
		return
	}

	group.Status = models.GroupStatusFailed
	group.ProcessingError = errorMessage
	if err := w.db.UpdateDownloadGroup(group); err != nil {
		w.logger.Error("Failed to mark group as failed", "group_id", groupID, "error", err)
		return
	}

	w.logger.Error("Group processing failed", "group_id", groupID, "error", errorMessage)
}

// processArchive extracts an archive file and tracks the extracted files
func (w *Worker) processArchive(download *models.Download) error {
	archivePath := filepath.Join(download.Directory, download.Filename)

	// Check if archive file exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("archive file not found: %s", archivePath)
	}

	w.logger.Info("Processing archive", "download_id", download.ID, "archive", archivePath)

	// Extract archive to the same directory
	extractedFiles, err := w.extractor.Extract(archivePath, download.Directory)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	if len(extractedFiles) == 0 {
		return fmt.Errorf("no files were extracted from archive")
	}

	w.logger.Info("Archive extracted successfully", "download_id", download.ID, "extracted_files", len(extractedFiles))

	// Store extracted files in database for tracking
	if err := w.storeExtractedFiles(download.ID, extractedFiles); err != nil {
		w.logger.Warn("Failed to store extracted files list", "download_id", download.ID, "error", err)
		// Don't return error here as extraction was successful
	}

	// Update download record with extracted files list
	extractedFilesJSON, err := json.Marshal(extractedFiles)
	if err != nil {
		w.logger.Warn("Failed to marshal extracted files list", "download_id", download.ID, "error", err)
	} else {
		download.ExtractedFiles = string(extractedFilesJSON)
		download.UpdatedAt = time.Now()
		if err := w.db.UpdateDownload(download); err != nil {
			w.logger.Warn("Failed to update download with extracted files", "download_id", download.ID, "error", err)
		}
	}

	// Delete the original archive file after successful extraction
	if err := w.deleteArchiveFiles(download); err != nil {
		w.logger.Warn("Failed to delete archive files", "error", err)
		// Don't return error here as extraction was successful
	}

	// Clean up non-video files from the extracted files
	if err := w.cleanup.CleanupExtractedFiles(download.ID); err != nil {
		w.logger.Warn("File cleanup completed with errors", "download_id", download.ID, "error", err)
		// Don't return error here as extraction was successful
	} else {
		w.logger.Info("File cleanup completed successfully", "download_id", download.ID)
	}

	// Optionally clean up empty directories
	if err := w.cleanup.CleanupEmptyDirectories(download.ID, download.Directory); err != nil {
		w.logger.Warn("Directory cleanup failed", "download_id", download.ID, "error", err)
		// Don't return error here as the main operation was successful
	}

	return nil
}

// deleteArchiveFiles deletes all parts of an archive (handles multi-part archives)
func (w *Worker) deleteArchiveFiles(download *models.Download) error {
	archivePath := filepath.Join(download.Directory, download.Filename)

	// Delete the main archive file
	if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
		w.logger.Warn("Failed to delete archive file", "archive", archivePath, "error", err)
	} else {
		w.logger.Info("Archive file deleted", "archive", archivePath)
	}

	// If this is part of a group, delete all other archive parts in the group
	if download.GroupID != "" {
		downloads, err := w.db.GetDownloadsByGroupID(download.GroupID)
		if err != nil {
			return fmt.Errorf("failed to get downloads for archive cleanup: %w", err)
		}

		for _, groupDownload := range downloads {
			// Skip the file we already deleted
			if groupDownload.ID == download.ID {
				continue
			}

			// Check if it's a RAR part file (including parts that aren't marked as archives)
			filename := strings.ToLower(groupDownload.Filename)
			if strings.HasSuffix(filename, ".rar") {
				partPath := filepath.Join(groupDownload.Directory, groupDownload.Filename)
				if err := os.Remove(partPath); err != nil && !os.IsNotExist(err) {
					w.logger.Warn("Failed to delete archive part", "archive", partPath, "error", err)
				} else {
					w.logger.Info("Archive part deleted", "archive", partPath)
				}
			}
		}
	}

	return nil
}

// storeExtractedFiles stores a list of extracted files in the database for cleanup tracking
func (w *Worker) storeExtractedFiles(downloadID int64, filePaths []string) error {
	now := time.Now()

	for _, filePath := range filePaths {
		extractedFile := &models.ExtractedFile{
			DownloadID: downloadID,
			FilePath:   filePath,
			CreatedAt:  now,
		}

		if err := w.db.CreateExtractedFile(extractedFile); err != nil {
			w.logger.Warn("Failed to store extracted file record", "download_id", downloadID, "file", filePath, "error", err)
			// Continue with other files
		}
	}

	return nil
}
