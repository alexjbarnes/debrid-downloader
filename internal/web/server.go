// Package web provides the HTTP server and routing
package web

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/config"
	"debrid-downloader/internal/database"
	"debrid-downloader/internal/downloader"
	"debrid-downloader/internal/web/handlers"
)

// Server represents the HTTP server
type Server struct {
	server   *http.Server
	handlers *handlers.Handlers
	logger   *slog.Logger
}

// NewServer creates a new HTTP server
func NewServer(db *database.DB, client alldebrid.AllDebridClient, cfg *config.Config, worker *downloader.Worker) *Server {
	handlers := handlers.NewHandlers(db, client, cfg.BaseDownloadsPath, worker)

	mux := http.NewServeMux()

	// Routes
	mux.HandleFunc("GET /", handlers.Home)
	mux.HandleFunc("GET /settings", handlers.Settings)

	// HTMX partial endpoints
	mux.HandleFunc("GET /downloads/current", handlers.CurrentDownloads)
	mux.HandleFunc("POST /download", handlers.SubmitDownload)
	mux.HandleFunc("POST /downloads/search", handlers.SearchDownloads)
	mux.HandleFunc("POST /downloads/progress", handlers.UpdateDownloadProgress)
	mux.HandleFunc("POST /downloads/{id}/retry", handlers.RetryDownload)
	mux.HandleFunc("POST /downloads/{id}/pause", handlers.PauseDownload)
	mux.HandleFunc("POST /downloads/{id}/resume", handlers.ResumeDownload)
	mux.HandleFunc("DELETE /downloads/{id}", handlers.DeleteDownload)
	mux.HandleFunc("GET /api/stats", handlers.GetDownloadStats)
	mux.HandleFunc("GET /api/directory-suggestion", handlers.GetDirectorySuggestion)
	mux.HandleFunc("POST /api/directory-suggestion", handlers.GetDirectorySuggestion)
	mux.HandleFunc("POST /api/test/failed-download", handlers.CreateTestFailedDownload)

	// Folder browsing API endpoints
	mux.HandleFunc("GET /api/folders", handlers.BrowseFolders)
	mux.HandleFunc("POST /api/folders", handlers.CreateFolder)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		server:   server,
		handlers: handlers,
		logger:   slog.Default(),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	localIP := getLocalIP()
	port := strings.TrimPrefix(s.server.Addr, ":")

	s.logger.Info("Starting HTTP server",
		"addr", s.server.Addr,
		"local_ip", localIP,
		"port", port,
		"url", fmt.Sprintf("http://%s:%s", localIP, port))

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	return s.server.Shutdown(ctx)
}

// getLocalIP returns the local network IP address (192.168.0.* range)
func getLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "localhost"
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Check for IPv4 private network ranges
			if ip.To4() != nil {
				ipStr := ip.String()
				// Check for 192.168.0.* range specifically
				if strings.HasPrefix(ipStr, "192.168.") {
					return ipStr
				}
				// Fallback to other private ranges (10.*, 172.16-31.*)
				if strings.HasPrefix(ipStr, "10.") ||
					(strings.HasPrefix(ipStr, "172.") && isInRange172(ipStr)) {
					return ipStr
				}
			}
		}
	}

	return "localhost"
}

// isInRange172 checks if IP is in 172.16.0.0/12 range (172.16.0.0 - 172.31.255.255)
func isInRange172(ipStr string) bool {
	parts := strings.Split(ipStr, ".")
	if len(parts) < 2 {
		return false
	}

	if parts[0] != "172" {
		return false
	}

	// Parse second octet
	var secondOctet int
	if _, err := fmt.Sscanf(parts[1], "%d", &secondOctet); err != nil {
		return false
	}

	return secondOctet >= 16 && secondOctet <= 31
}
