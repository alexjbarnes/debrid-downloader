package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/config"
	"debrid-downloader/internal/database"
	"debrid-downloader/internal/downloader"
	"debrid-downloader/internal/web"
	"debrid-downloader/pkg/models"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Setup structured logging
	setupLogging(cfg.LogLevel)

	slog.Info("Starting Debrid Downloader", "version", "1.0.0")

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Failed to close database", "error", err)
		}
	}()

	// Initialize AllDebrid client
	allDebridClient := alldebrid.New(cfg.AllDebridAPIKey)

	// Validate API key (warn but don't exit if validation fails during development)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := allDebridClient.CheckAPIKey(ctx); err != nil {
		slog.Warn("AllDebrid API key validation failed - continuing anyway", "error", err)
		slog.Warn("Please ensure your AllDebrid API key is valid for full functionality")
	} else {
		slog.Info("AllDebrid API key validated successfully")
	}
	cancel()

	// Initialize download worker
	downloadWorker := downloader.NewWorker(db, cfg.BaseDownloadsPath)

	// Initialize web server with download worker
	server := web.NewServer(db, allDebridClient, cfg, downloadWorker)

	return runServer(server, downloadWorker, db)
}

func runServer(server *web.Server, downloadWorker *downloader.Worker, db *database.DB) error {
	// Create main context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start download worker in goroutine
	go downloadWorker.Start(ctx)

	// Reset orphaned downloads from previous session
	if err := resetOrphanedDownloads(db); err != nil {
		slog.Error("Failed to reset orphaned downloads", "error", err)
	}

	// Queue any pending downloads from previous session
	if err := queuePendingDownloads(db, downloadWorker); err != nil {
		slog.Error("Failed to queue pending downloads", "error", err)
	}

	// Start history cleanup routine (runs daily)
	go startHistoryCleanup(ctx, db)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed to start: %w", err)
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig.String())
	}

	// Cancel context to stop download worker
	cancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server gracefully: %w", err)
	}

	slog.Info("Server shutdown complete")
	return nil
}

// setupLogging configures structured logging based on the log level
func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// startHistoryCleanup runs a goroutine that cleans up old downloads periodically
func startHistoryCleanup(ctx context.Context, db *database.DB) {
	ticker := time.NewTicker(24 * time.Hour) // Run daily
	defer ticker.Stop()

	// Run cleanup immediately on startup
	cleanupOldDownloads(db)

	for {
		select {
		case <-ctx.Done():
			slog.Info("History cleanup routine shutting down")
			return
		case <-ticker.C:
			cleanupOldDownloads(db)
		}
	}
}

// cleanupOldDownloads removes downloads older than 60 days
func cleanupOldDownloads(db *database.DB) {
	retention := 60 * 24 * time.Hour // 60 days

	slog.Info("Running history cleanup", "retention_days", 60)

	if err := db.DeleteOldDownloads(retention); err != nil {
		slog.Error("Failed to cleanup old downloads", "error", err)
		return
	}

	slog.Info("History cleanup completed")
}

// resetOrphanedDownloads finds downloads stuck in downloading state and resets them to pending
func resetOrphanedDownloads(db *database.DB) error {
	// Get downloads stuck in downloading state (orphaned by server restart)
	orphanedDownloads, err := db.GetOrphanedDownloads()
	if err != nil {
		return fmt.Errorf("failed to get orphaned downloads: %w", err)
	}

	for _, download := range orphanedDownloads {
		// Clean up temporary file if it exists
		tempFilename := fmt.Sprintf("%s.%d.tmp", download.Filename, download.ID)
		tempPath := filepath.Join(download.Directory, tempFilename)
		if _, err := os.Stat(tempPath); err == nil {
			if removeErr := os.Remove(tempPath); removeErr != nil {
				slog.Warn("Failed to clean up orphaned temporary file", 
					"temp_path", tempPath, 
					"download_id", download.ID, 
					"error", removeErr)
			} else {
				slog.Info("Cleaned up orphaned temporary file", 
					"temp_path", tempPath, 
					"download_id", download.ID)
			}
		}

		// Reset download to pending state
		download.Status = models.StatusPending
		download.Progress = 0.0
		download.DownloadedBytes = 0
		download.DownloadSpeed = 0.0
		download.ErrorMessage = ""
		download.UpdatedAt = time.Now()
		download.StartedAt = nil

		if err := db.UpdateDownload(download); err != nil {
			slog.Error("Failed to reset orphaned download", 
				"download_id", download.ID, 
				"error", err)
			continue
		}

		slog.Info("Reset orphaned download to pending state", 
			"download_id", download.ID, 
			"filename", download.Filename)
	}

	if len(orphanedDownloads) > 0 {
		slog.Info("Reset orphaned downloads from previous session", "count", len(orphanedDownloads))
	}

	return nil
}

// queuePendingDownloads looks for any pending downloads from previous session and queues them
func queuePendingDownloads(db *database.DB, worker *downloader.Worker) error {
	// Get pending downloads ordered by creation time (oldest first)
	pendingDownloads, err := db.GetPendingDownloadsOldestFirst()
	if err != nil {
		return fmt.Errorf("failed to get pending downloads: %w", err)
	}

	for _, download := range pendingDownloads {
		worker.QueueDownload(download.ID)
		slog.Info("Queued pending download from previous session", 
			"download_id", download.ID, 
			"filename", download.Filename,
			"created_at", download.CreatedAt)
	}

	if len(pendingDownloads) > 0 {
		slog.Info("Queued pending downloads from previous session", "count", len(pendingDownloads))
	}

	return nil
}
