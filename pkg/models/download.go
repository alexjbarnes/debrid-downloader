// Package models defines the data structures used throughout the application
package models

import (
	"time"
)

// DownloadStatus represents the current status of a download
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusPaused      DownloadStatus = "paused"
)

// Download represents a file download record
type Download struct {
	ID              int64          `json:"id" db:"id"`
	OriginalURL     string         `json:"original_url" db:"original_url"`
	UnrestrictedURL string         `json:"unrestricted_url" db:"unrestricted_url"`
	Filename        string         `json:"filename" db:"filename"`
	Directory       string         `json:"directory" db:"directory"`
	Status          DownloadStatus `json:"status" db:"status"`
	Progress        float64        `json:"progress" db:"progress"`
	FileSize        int64          `json:"file_size" db:"file_size"`
	DownloadedBytes int64          `json:"downloaded_bytes" db:"downloaded_bytes"`
	DownloadSpeed   float64        `json:"download_speed" db:"download_speed"`
	ErrorMessage    string         `json:"error_message" db:"error_message"`
	RetryCount      int            `json:"retry_count" db:"retry_count"`
	CreatedAt       time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
	StartedAt       *time.Time     `json:"started_at" db:"started_at"`
	CompletedAt     *time.Time     `json:"completed_at" db:"completed_at"`
	PausedAt        *time.Time     `json:"paused_at" db:"paused_at"`
	TotalPausedTime int64          `json:"total_paused_time" db:"total_paused_time"` // Total paused time in seconds
}

// DirectoryMapping represents a learned directory suggestion
type DirectoryMapping struct {
	ID              int64     `json:"id" db:"id"`
	FilenamePattern string    `json:"filename_pattern" db:"filename_pattern"`
	OriginalURL     string    `json:"original_url" db:"original_url"`
	Directory       string    `json:"directory" db:"directory"`
	UseCount        int       `json:"use_count" db:"use_count"`
	LastUsed        time.Time `json:"last_used" db:"last_used"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}
