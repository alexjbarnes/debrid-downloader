package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/config"
	"debrid-downloader/internal/database"

	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	cfg := &config.Config{
		ServerPort: "8080",
		LogLevel:   "info",
	}

	server := NewServer(db, client, cfg)
	require.NotNil(t, server)
	require.Equal(t, ":8080", server.server.Addr)
}

func TestServer_StartAndShutdown(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	cfg := &config.Config{
		ServerPort: "0", // Use random port
		LogLevel:   "info",
	}

	server := NewServer(db, client, cfg)

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
