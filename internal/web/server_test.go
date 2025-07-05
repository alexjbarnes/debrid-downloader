package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/config"
	"debrid-downloader/internal/database"
	"debrid-downloader/internal/downloader"

	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	cfg := &config.Config{
		ServerPort: "8080",
		LogLevel:   "info",
	}

	server := NewServer(db, client, cfg, worker)
	require.NotNil(t, server)
	require.Equal(t, ":8080", server.server.Addr)
}

func TestServer_StartAndShutdown(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	cfg := &config.Config{
		ServerPort: "0", // Use random port
		LogLevel:   "info",
	}

	server := NewServer(db, client, cfg, worker)

	// Test that we can start and shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown server
	err = server.Shutdown(ctx)
	require.NoError(t, err)

	// Check if start returned an error (should be http.ErrServerClosed)
	select {
	case err := <-errChan:
		require.Equal(t, http.ErrServerClosed, err)
	case <-time.After(time.Second):
		t.Fatal("Server did not shutdown within timeout")
	}
}

func TestGetLocalIP(t *testing.T) {
	// Test getLocalIP function
	ip := getLocalIP()
	require.NotEmpty(t, ip)
	// Should return either "localhost" or a valid IP address
	require.True(t, ip == "localhost" || 
		len(ip) >= 7, // Minimum IP length
		"Expected localhost or valid IP, got: %s", ip)
}

func TestIsInRange172(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "valid 172.16 range",
			ip:       "172.16.0.1",
			expected: true,
		},
		{
			name:     "valid 172.31 range",
			ip:       "172.31.255.255",
			expected: true,
		},
		{
			name:     "valid 172.20 range",
			ip:       "172.20.1.1",
			expected: true,
		},
		{
			name:     "invalid 172.15 range",
			ip:       "172.15.0.1",
			expected: false,
		},
		{
			name:     "invalid 172.32 range",
			ip:       "172.32.0.1",
			expected: false,
		},
		{
			name:     "not 172 range",
			ip:       "192.168.1.1",
			expected: false,
		},
		{
			name:     "invalid IP format",
			ip:       "172",
			expected: false,
		},
		{
			name:     "invalid second octet",
			ip:       "172.abc.0.1",
			expected: false,
		},
		{
			name:     "empty IP",
			ip:       "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInRange172(tt.ip)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestServer_Components(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	cfg := &config.Config{
		ServerPort: "3000",
		LogLevel:   "debug",
	}

	server := NewServer(db, client, cfg, worker)
	
	// Test server configuration
	require.NotNil(t, server.server)
	require.Equal(t, ":3000", server.server.Addr)
	require.Equal(t, 15*time.Second, server.server.ReadTimeout)
	require.Equal(t, 15*time.Second, server.server.WriteTimeout)
	require.Equal(t, 60*time.Second, server.server.IdleTimeout)
	
	// Test handlers are set
	require.NotNil(t, server.handlers)
	require.NotNil(t, server.logger)
}

func TestServer_ShutdownTimeout(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	cfg := &config.Config{
		ServerPort: "0",
		LogLevel:   "info",
	}

	server := NewServer(db, client, cfg, worker)

	// Test shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	
	// Shutdown should handle timeout gracefully
	err = server.Shutdown(ctx)
	// Context might be cancelled but shouldn't panic
	require.True(t, err == nil || err == context.DeadlineExceeded)
}
