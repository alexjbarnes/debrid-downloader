package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/database"
	"debrid-downloader/internal/downloader"
	"debrid-downloader/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateDisplayItems tests the core grouping logic
func TestCreateDisplayItems(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	handlers := NewHandlers(db, client, "/tmp/test", worker)

	tests := []struct {
		name      string
		downloads []*models.Download
		expected  []testDisplayItem
	}{
		{
			name:      "empty downloads",
			downloads: []*models.Download{},
			expected:  []testDisplayItem{},
		},
		{
			name: "single downloads only",
			downloads: []*models.Download{
				createTestDownload(1, "", models.StatusCompleted, 100.0, time.Now().Add(-1*time.Hour)),
				createTestDownload(2, "", models.StatusDownloading, 50.0, time.Now().Add(-30*time.Minute)),
			},
			expected: []testDisplayItem{
				{IsGroup: false, DownloadID: 1, Status: models.StatusCompleted, IsExpanded: false},
				{IsGroup: false, DownloadID: 2, Status: models.StatusDownloading, IsExpanded: true},
			},
		},
		{
			name: "grouped downloads only",
			downloads: []*models.Download{
				createTestDownload(1, "group1", models.StatusCompleted, 100.0, time.Now().Add(-1*time.Hour)),
				createTestDownload(2, "group1", models.StatusDownloading, 50.0, time.Now().Add(-30*time.Minute)),
				createTestDownload(3, "group1", models.StatusPending, 0.0, time.Now().Add(-15*time.Minute)),
			},
			expected: []testDisplayItem{
				{
					IsGroup:               true,
					GroupID:               "group1",
					DownloadCount:         3,
					HighestPriorityStatus: models.StatusDownloading,
					HasActiveDownloads:    true,
					IsExpanded:            true,
					StatusCounts: map[models.DownloadStatus]int{
						models.StatusCompleted:   1,
						models.StatusDownloading: 1,
						models.StatusPending:     1,
					},
				},
			},
		},
		{
			name: "mixed single and grouped downloads",
			downloads: []*models.Download{
				createTestDownload(1, "", models.StatusCompleted, 100.0, time.Now().Add(-2*time.Hour)),
				createTestDownload(2, "group1", models.StatusCompleted, 100.0, time.Now().Add(-1*time.Hour)),
				createTestDownload(3, "group1", models.StatusDownloading, 50.0, time.Now().Add(-30*time.Minute)),
				createTestDownload(4, "", models.StatusPending, 0.0, time.Now().Add(-15*time.Minute)),
			},
			expected: []testDisplayItem{
				{IsGroup: false, DownloadID: 1, Status: models.StatusCompleted, IsExpanded: false},
				{
					IsGroup:               true,
					GroupID:               "group1",
					DownloadCount:         2,
					HighestPriorityStatus: models.StatusDownloading,
					HasActiveDownloads:    true,
					IsExpanded:            true,
					StatusCounts: map[models.DownloadStatus]int{
						models.StatusCompleted:   1,
						models.StatusDownloading: 1,
					},
				},
				{IsGroup: false, DownloadID: 4, Status: models.StatusPending, IsExpanded: false},
			},
		},
		{
			name: "multiple groups",
			downloads: []*models.Download{
				createTestDownload(1, "group1", models.StatusCompleted, 100.0, time.Now().Add(-2*time.Hour)),
				createTestDownload(2, "group1", models.StatusCompleted, 100.0, time.Now().Add(-1*time.Hour)),
				createTestDownload(3, "group2", models.StatusPending, 0.0, time.Now().Add(-30*time.Minute)),
				createTestDownload(4, "group2", models.StatusDownloading, 25.0, time.Now().Add(-15*time.Minute)),
			},
			expected: []testDisplayItem{
				{
					IsGroup:               true,
					GroupID:               "group1",
					DownloadCount:         2,
					HighestPriorityStatus: models.StatusCompleted,
					HasActiveDownloads:    false,
					IsExpanded:            false,
					StatusCounts: map[models.DownloadStatus]int{
						models.StatusCompleted: 2,
					},
				},
				{
					IsGroup:               true,
					GroupID:               "group2",
					DownloadCount:         2,
					HighestPriorityStatus: models.StatusDownloading,
					HasActiveDownloads:    true,
					IsExpanded:            true,
					StatusCounts: map[models.DownloadStatus]int{
						models.StatusPending:     1,
						models.StatusDownloading: 1,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handlers.createDisplayItems(tt.downloads)
			assert.Equal(t, len(tt.expected), len(result), "Number of display items should match")

			for i, expectedItem := range tt.expected {
				if i >= len(result) {
					t.Errorf("Missing display item at index %d", i)
					continue
				}

				actualItem := result[i]
				
				assert.Equal(t, expectedItem.IsGroup, actualItem.IsGroup, "IsGroup should match at index %d", i)

				if expectedItem.IsGroup {
					// Test group properties
					assert.Equal(t, expectedItem.GroupID, actualItem.GroupID, "GroupID should match at index %d", i)
					assert.Equal(t, expectedItem.DownloadCount, len(actualItem.Downloads), "Download count should match at index %d", i)
					assert.Equal(t, expectedItem.HighestPriorityStatus, actualItem.HighestPriorityStatus, "Priority status should match at index %d", i)
					assert.Equal(t, expectedItem.HasActiveDownloads, actualItem.HasActiveDownloads, "Active downloads flag should match at index %d", i)
					assert.Equal(t, expectedItem.IsExpanded, actualItem.IsExpanded, "Group expand state should match at index %d", i)
					
					// Test status counts
					for status, expectedCount := range expectedItem.StatusCounts {
						actualCount, exists := actualItem.StatusCounts[status]
						assert.True(t, exists, "Status %s should exist in counts at index %d", status, i)
						assert.Equal(t, expectedCount, actualCount, "Status count for %s should match at index %d", status, i)
					}
				} else {
					// Test single download properties
					assert.NotNil(t, actualItem.Download, "Download should not be nil at index %d", i)
					assert.Equal(t, expectedItem.DownloadID, actualItem.Download.ID, "Download ID should match at index %d", i)
					assert.Equal(t, expectedItem.Status, actualItem.Download.Status, "Download status should match at index %d", i)
					assert.Equal(t, expectedItem.IsExpanded, actualItem.Download.IsExpanded, "Download expand state should match at index %d", i)
				}
			}
		})
	}
}

// TestExpandStateManagement tests the expand/collapse state logic
func TestExpandStateManagement(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	handlers := NewHandlers(db, client, "/tmp/test", worker)

	t.Run("download expand state defaults", func(t *testing.T) {
		tests := []struct {
			status   models.DownloadStatus
			expected bool
		}{
			{models.StatusDownloading, true},
			{models.StatusPending, false},
			{models.StatusPaused, false},
			{models.StatusCompleted, false},
			{models.StatusFailed, false},
		}

		for _, tt := range tests {
			actual := handlers.isDownloadExpanded(123, tt.status)
			assert.Equal(t, tt.expected, actual, "Default expand state for status %s", tt.status)
		}
	})

	t.Run("group expand state defaults", func(t *testing.T) {
		// Default behavior - groups with active downloads are expanded
		assert.True(t, handlers.isGroupExpanded("group1", true))
		assert.False(t, handlers.isGroupExpanded("group2", false))
	})

	t.Run("download expand state persistence", func(t *testing.T) {
		downloadID := int64(456)

		// Set expanded state
		handlers.setDownloadExpanded(downloadID, true)
		assert.True(t, handlers.isDownloadExpanded(downloadID, models.StatusCompleted))

		// Set collapsed state
		handlers.setDownloadExpanded(downloadID, false)
		assert.False(t, handlers.isDownloadExpanded(downloadID, models.StatusDownloading))
	})

	t.Run("group expand state persistence", func(t *testing.T) {
		groupID := "test-group"

		// Set expanded state
		handlers.setGroupExpanded(groupID, true)
		assert.True(t, handlers.isGroupExpanded(groupID, false))

		// Set collapsed state
		handlers.setGroupExpanded(groupID, false)
		assert.False(t, handlers.isGroupExpanded(groupID, true))
	})
}

// TestToggleDownload tests the toggle download endpoint
func TestToggleDownload(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	handlers := NewHandlers(db, client, "/tmp/test", worker)

	// Create test download in database
	download := &models.Download{
		Filename:    "test-file.zip",
		OriginalURL: "https://example.com/test.zip",
		Directory:   "/tmp/test",
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
	}
	
	err = db.CreateDownload(download)
	require.NoError(t, err)

	t.Run("toggle existing download", func(t *testing.T) {
		// Create request
		req := httptest.NewRequest("POST", fmt.Sprintf("/downloads/%d/toggle", download.ID), nil)
		req.SetPathValue("id", strconv.FormatInt(download.ID, 10))
		w := httptest.NewRecorder()

		// Initially collapsed (completed status)
		assert.False(t, handlers.isDownloadExpanded(download.ID, download.Status))

		// First toggle - should expand
		handlers.ToggleDownload(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, handlers.isDownloadExpanded(download.ID, download.Status))

		// Second toggle - should collapse
		w = httptest.NewRecorder()
		handlers.ToggleDownload(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, handlers.isDownloadExpanded(download.ID, download.Status))
	})

	t.Run("toggle non-existent download", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/downloads/99999/toggle", nil)
		req.SetPathValue("id", "99999")
		w := httptest.NewRecorder()

		handlers.ToggleDownload(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid download ID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/downloads/invalid/toggle", nil)
		req.SetPathValue("id", "invalid")
		w := httptest.NewRecorder()

		handlers.ToggleDownload(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestToggleGroup tests the toggle group endpoint
func TestToggleGroup(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	handlers := NewHandlers(db, client, "/tmp/test", worker)

	// Create test downloads in a group
	groupID := "test-group-123"
	downloads := []*models.Download{
		{
			Filename:    "file1.zip",
			OriginalURL: "https://example.com/file1.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusCompleted,
			GroupID:     groupID,
			CreatedAt:   time.Now().Add(-1 * time.Hour),
		},
		{
			Filename:    "file2.zip",
			OriginalURL: "https://example.com/file2.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusDownloading,
			GroupID:     groupID,
			Progress:    75.5,
			CreatedAt:   time.Now().Add(-30 * time.Minute),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	t.Run("toggle existing group", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/groups/%s/toggle", groupID), nil)
		req.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		// Initially expanded (has active download)
		assert.True(t, handlers.isGroupExpanded(groupID, true))

		// First toggle - should collapse
		handlers.ToggleGroup(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, handlers.isGroupExpanded(groupID, true))

		// Second toggle - should expand
		w = httptest.NewRecorder()
		handlers.ToggleGroup(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, handlers.isGroupExpanded(groupID, true))
	})

	t.Run("toggle non-existent group", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/groups/non-existent/toggle", nil)
		req.SetPathValue("id", "non-existent")
		w := httptest.NewRecorder()

		handlers.ToggleGroup(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("empty group ID", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/groups//toggle", nil)
		req.SetPathValue("id", "")
		w := httptest.NewRecorder()

		handlers.ToggleGroup(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestUpdateDownloadProgress tests the progress update endpoint
func TestUpdateDownloadProgress(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	handlers := NewHandlers(db, client, "/tmp/test", worker)

	// Create test downloads
	downloads := []*models.Download{
		{
			Filename:    "active1.zip",
			OriginalURL: "https://example.com/active1.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusDownloading,
			Progress:    45.0,
			CreatedAt:   time.Now().Add(-1 * time.Hour),
		},
		{
			Filename:    "completed.zip",
			OriginalURL: "https://example.com/completed.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusCompleted,
			Progress:    100.0,
			CreatedAt:   time.Now().Add(-30 * time.Minute),
		},
		{
			Filename:    "pending.zip",
			OriginalURL: "https://example.com/pending.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusPending,
			Progress:    0.0,
			CreatedAt:   time.Now().Add(-15 * time.Minute),
		},
	}

	for _, download := range downloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	t.Run("progress update with active downloads", func(t *testing.T) {
		form := url.Values{}
		form.Add("status", "downloading")
		form.Add("status", "pending")
		form.Add("sort", "desc")

		req := httptest.NewRequest("POST", "/downloads/progress", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handlers.UpdateDownloadProgress(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Response should contain OOB updates for active downloads
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "hx-swap-oob")
		assert.Contains(t, responseBody, "active1.zip")
		// Should not contain completed download
		assert.NotContains(t, responseBody, "completed.zip")
	})

	t.Run("progress update with no active downloads", func(t *testing.T) {
		form := url.Values{}
		form.Add("status", "completed")
		form.Add("sort", "desc")

		req := httptest.NewRequest("POST", "/downloads/progress", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handlers.UpdateDownloadProgress(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Response should be minimal for no active downloads
		responseBody := w.Body.String()
		assert.Equal(t, "", strings.TrimSpace(responseBody))
	})

	t.Run("progress update with malformed form", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/downloads/progress", strings.NewReader("invalid%form"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handlers.UpdateDownloadProgress(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Helper functions for testing

type testDisplayItem struct {
	IsGroup               bool
	DownloadID            int64
	GroupID               string
	DownloadCount         int
	Status                models.DownloadStatus
	HighestPriorityStatus models.DownloadStatus
	HasActiveDownloads    bool
	IsExpanded            bool
	StatusCounts          map[models.DownloadStatus]int
}

func createTestDownload(id int64, groupID string, status models.DownloadStatus, progress float64, createdAt time.Time) *models.Download {
	return &models.Download{
		ID:          id,
		Filename:    fmt.Sprintf("test-file-%d.zip", id),
		OriginalURL: fmt.Sprintf("https://example.com/test-%d.zip", id),
		Directory:   "/tmp/test",
		Status:      status,
		Progress:    progress,
		GroupID:     groupID,
		CreatedAt:   createdAt,
	}
}