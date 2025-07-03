// Package config handles application configuration from environment variables
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config represents the application configuration
type Config struct {
	AllDebridAPIKey   string `env:"ALLDEBRID_API_KEY,required"`
	ServerPort        string `env:"SERVER_PORT" envDefault:"8080"`
	LogLevel          string `env:"LOG_LEVEL" envDefault:"info"`
	DatabasePath      string `env:"DATABASE_PATH" envDefault:"debrid.db"`
	BaseDownloadsPath string `env:"BASE_DOWNLOADS_PATH" envDefault:"/downloads"`
}

// Load loads configuration from environment variables and .env file
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.AllDebridAPIKey == "" {
		return fmt.Errorf("ALLDEBRID_API_KEY is required")
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	logLevel := strings.ToLower(c.LogLevel)
	isValidLevel := false
	for _, level := range validLogLevels {
		if logLevel == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		return fmt.Errorf("invalid log level %q, must be one of: %v", c.LogLevel, validLogLevels)
	}

	// Validate base downloads path
	if c.BaseDownloadsPath == "" {
		return fmt.Errorf("BASE_DOWNLOADS_PATH cannot be empty")
	}

	// Clean and validate the path
	cleanPath := filepath.Clean(c.BaseDownloadsPath)
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("BASE_DOWNLOADS_PATH must be an absolute path, got: %s", c.BaseDownloadsPath)
	}

	// Check if path exists and is a directory (only if it exists)
	if info, err := os.Stat(cleanPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("BASE_DOWNLOADS_PATH must be a directory, got file: %s", cleanPath)
		}
	}

	// Update the config with cleaned path
	c.BaseDownloadsPath = cleanPath

	return nil
}
