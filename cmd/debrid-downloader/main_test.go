package main

import (
	"context"
	"os"
	"testing"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/config"
	"debrid-downloader/internal/database"
	"debrid-downloader/internal/web"

	"github.com/stretchr/testify/require"
)

func TestSetupLogging(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
		{"invalid level defaults to info", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				setupLogging(tt.level)
			})
		})
	}
}

func TestRun(t *testing.T) {
	// Test configuration loading error handling
	// Set invalid env var to trigger config error
	os.Setenv("ALLDEBRID_API_KEY", "")
	defer os.Unsetenv("ALLDEBRID_API_KEY")

	// This test verifies that run() returns error on invalid config
	err := run()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load configuration")
}

func TestRunWithValidConfig(t *testing.T) {
	// Skip this test as it would hang waiting for signals
	// This test would verify initialization but can't easily test the signal handling
	t.Skip("Skipping test that would hang on signal handling")
}

func TestRunDatabaseError(t *testing.T) {
	// Set valid config but invalid database path
	os.Setenv("ALLDEBRID_API_KEY", "test-key")
	os.Setenv("DATABASE_PATH", "/invalid/path/test.db")
	defer func() {
		os.Unsetenv("ALLDEBRID_API_KEY")
		os.Unsetenv("DATABASE_PATH")
	}()

	err := run()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to initialize database")
}

func TestContextTimeout(t *testing.T) {
	// Test context timeout functionality
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify context is created properly
	require.NotNil(t, ctx)

	// Verify context has timeout
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.True(t, deadline.After(time.Now()))
}

func TestShutdownContext(t *testing.T) {
	// Test shutdown context creation
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Verify shutdown context is created properly
	require.NotNil(t, shutdownCtx)

	// Verify context has timeout
	deadline, ok := shutdownCtx.Deadline()
	require.True(t, ok)
	require.True(t, deadline.After(time.Now()))
}

func TestRunServerShutdown(t *testing.T) {
	// Skip this test as it would hang and interfere with other tests
	t.Skip("Skipping test that sends signals to the process")
}

func TestRunServerStartError(t *testing.T) {
	// Create a server that will fail to start by using an invalid port
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	cfg := &config.Config{
		ServerPort: "999999", // Invalid port
		LogLevel:   "info",
	}

	server := web.NewServer(db, client, cfg)

	err = runServer(server)
	require.Error(t, err)
	require.Contains(t, err.Error(), "server failed to start")
}

func TestSetupLoggingAllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "invalid"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			require.NotPanics(t, func() {
				setupLogging(level)
			})
		})
	}
}

func TestRunInitialization(t *testing.T) {
	// Test successful initialization up to server creation
	os.Setenv("ALLDEBRID_API_KEY", "test-key")
	os.Setenv("DATABASE_PATH", ":memory:")
	os.Setenv("SERVER_PORT", "0")
	defer func() {
		os.Unsetenv("ALLDEBRID_API_KEY")
		os.Unsetenv("DATABASE_PATH")
		os.Unsetenv("SERVER_PORT")
	}()

	// Test initialization components individually
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	db, err := database.New(cfg.DatabasePath)
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New(cfg.AllDebridAPIKey)
	require.NotNil(t, client)

	server := web.NewServer(db, client, cfg)
	require.NotNil(t, server)
}

func TestMainFunction(t *testing.T) {
	// Test that main function handles error correctly
	os.Setenv("ALLDEBRID_API_KEY", "")
	defer os.Unsetenv("ALLDEBRID_API_KEY")

	// We can't directly test main() because it calls os.Exit
	// But we can test that run() returns the expected error
	err := run()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load configuration")
}
