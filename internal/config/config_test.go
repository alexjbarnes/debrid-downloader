package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid config",
			envVars: map[string]string{
				"ALLDEBRID_API_KEY":   "test-key",
				"SERVER_PORT":         "8080",
				"LOG_LEVEL":           "info",
				"BASE_DOWNLOADS_PATH": "/downloads",
			},
			wantErr: false,
		},
		{
			name: "missing required API key",
			envVars: map[string]string{
				"SERVER_PORT": "8080",
				"LOG_LEVEL":   "info",
			},
			wantErr: true,
		},
		{
			name: "defaults applied",
			envVars: map[string]string{
				"ALLDEBRID_API_KEY": "test-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			cfg, err := Load()

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Verify required fields
			if apiKey, exists := tt.envVars["ALLDEBRID_API_KEY"]; exists {
				require.Equal(t, apiKey, cfg.AllDebridAPIKey)
			}

			// Verify defaults
			if _, exists := tt.envVars["SERVER_PORT"]; !exists {
				require.Equal(t, "8080", cfg.ServerPort)
			}

			if _, exists := tt.envVars["LOG_LEVEL"]; !exists {
				require.Equal(t, "info", cfg.LogLevel)
			}

			if _, exists := tt.envVars["BASE_DOWNLOADS_PATH"]; !exists {
				require.Equal(t, "/downloads", cfg.BaseDownloadsPath)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				AllDebridAPIKey:   "test-key",
				ServerPort:        "8080",
				LogLevel:          "info",
				BaseDownloadsPath: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "empty API key",
			config: Config{
				AllDebridAPIKey:   "",
				ServerPort:        "8080",
				LogLevel:          "info",
				BaseDownloadsPath: "/tmp",
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: Config{
				AllDebridAPIKey:   "test-key",
				ServerPort:        "8080",
				LogLevel:          "invalid",
				BaseDownloadsPath: "/tmp",
			},
			wantErr: true,
		},
		{
			name: "relative downloads path",
			config: Config{
				AllDebridAPIKey:   "test-key",
				ServerPort:        "8080",
				LogLevel:          "info",
				BaseDownloadsPath: "downloads",
			},
			wantErr: true,
		},
		{
			name: "empty downloads path",
			config: Config{
				AllDebridAPIKey:   "test-key",
				ServerPort:        "8080",
				LogLevel:          "info",
				BaseDownloadsPath: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
