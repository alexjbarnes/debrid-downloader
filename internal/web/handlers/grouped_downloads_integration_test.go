package handlers

import (
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

// TestGroupedDownloadsWorkflow tests the complete grouped downloads workflow
func TestGroupedDownloadsWorkflow(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	handlers := NewHandlers(db, client, "/tmp/test", worker)

	// Setup test data - mixed single and grouped downloads
	testDownloads := []*models.Download{
		// Single download
		{
			Filename:    "single-file.zip",
			OriginalURL: "https://example.com/single.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusCompleted,
			Progress:    100.0,
			CreatedAt:   time.Now().Add(-3 * time.Hour),
		},
		// Group 1 - mixed statuses
		{
			Filename:    "group1-file1.zip",
			OriginalURL: "https://example.com/group1-file1.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusCompleted,
			Progress:    100.0,
			GroupID:     "group-1",
			CreatedAt:   time.Now().Add(-2 * time.Hour),
		},
		{
			Filename:    "group1-file2.zip",
			OriginalURL: "https://example.com/group1-file2.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusDownloading,
			Progress:    65.5,
			GroupID:     "group-1",
			CreatedAt:   time.Now().Add(-90 * time.Minute),
		},
		{
			Filename:    "group1-file3.zip",
			OriginalURL: "https://example.com/group1-file3.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusPending,
			Progress:    0.0,
			GroupID:     "group-1",
			CreatedAt:   time.Now().Add(-75 * time.Minute),
		},
		// Group 2 - all completed
		{
			Filename:    "group2-file1.zip",
			OriginalURL: "https://example.com/group2-file1.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusCompleted,
			Progress:    100.0,
			GroupID:     "group-2",
			CreatedAt:   time.Now().Add(-1 * time.Hour),
		},
		{
			Filename:    "group2-file2.zip",
			OriginalURL: "https://example.com/group2-file2.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusCompleted,
			Progress:    100.0,
			GroupID:     "group-2",
			CreatedAt:   time.Now().Add(-45 * time.Minute),
		},
		// Another single download
		{
			Filename:    "another-single.zip",
			OriginalURL: "https://example.com/another-single.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusDownloading,
			Progress:    25.0,
			CreatedAt:   time.Now().Add(-30 * time.Minute),
		},
	}

	// Create downloads in database
	for _, download := range testDownloads {
		err = db.CreateDownload(download)
		require.NoError(t, err)
	}

	t.Run("SearchDownloads returns grouped display", func(t *testing.T) {
		form := url.Values{}
		form.Add("status", "completed")
		form.Add("status", "downloading")
		form.Add("status", "pending")
		form.Add("sort", "desc")

		req := httptest.NewRequest("POST", "/downloads/search", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handlers.SearchDownloads(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		responseBody := w.Body.String()

		// Should contain grouped downloads
		assert.Contains(t, responseBody, "group-1")
		assert.Contains(t, responseBody, "group-2")
		
		// Should contain single downloads
		assert.Contains(t, responseBody, "single-file.zip")
		assert.Contains(t, responseBody, "another-single.zip")

		// Group 1 should be expanded (has active downloads)
		assert.Contains(t, responseBody, "max-h-none opacity-100")
		
		// Should show status counts for groups
		assert.Contains(t, responseBody, "Completed")
		assert.Contains(t, responseBody, "Downloading")
		assert.Contains(t, responseBody, "Pending")
	})

	t.Run("Toggle group expand/collapse workflow", func(t *testing.T) {
		groupID := "group-1"
		
		// Check initial state (should be expanded due to active downloads)
		assert.True(t, handlers.isGroupExpanded(groupID, true))

		// Collapse the group
		req := httptest.NewRequest("POST", "/groups/"+groupID+"/toggle", nil)
		req.SetPathValue("id", groupID)
		w := httptest.NewRecorder()

		handlers.ToggleGroup(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, handlers.isGroupExpanded(groupID, true))

		// Response should contain collapsed group HTML
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "max-h-0 opacity-0")

		// Expand the group again
		w = httptest.NewRecorder()
		handlers.ToggleGroup(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, handlers.isGroupExpanded(groupID, true))

		// Response should contain expanded group HTML
		responseBody = w.Body.String()
		assert.Contains(t, responseBody, "max-h-none opacity-100")
	})

	t.Run("Toggle individual download in group", func(t *testing.T) {
		// Get one of the group downloads
		downloads, err := db.GetDownloadsByGroupID("group-1")
		require.NoError(t, err)
		require.NotEmpty(t, downloads)

		downloadID := downloads[0].ID
		
		// Check initial state (should be expanded if downloading, collapsed if completed)
		initialState := handlers.isDownloadExpanded(downloadID, downloads[0].Status)

		// Toggle the download
		req := httptest.NewRequest("POST", "/downloads/"+strconv.FormatInt(downloadID, 10)+"/toggle", nil)
		req.SetPathValue("id", strconv.FormatInt(downloadID, 10))
		w := httptest.NewRecorder()

		handlers.ToggleDownload(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		
		// State should be toggled
		newState := handlers.isDownloadExpanded(downloadID, downloads[0].Status)
		assert.NotEqual(t, initialState, newState)

		// Response should contain updated HTML
		responseBody := w.Body.String()
		assert.NotEmpty(t, responseBody)
	})

	t.Run("Progress updates for grouped downloads", func(t *testing.T) {
		// Update progress for downloading items
		downloads, err := db.SearchDownloads("", []string{"downloading"}, "desc", 50, 0)
		require.NoError(t, err)
		
		for _, download := range downloads {
			if download.Status == models.StatusDownloading {
				// Simulate progress update
				download.Progress += 10.0
				if download.Progress > 100.0 {
					download.Progress = 100.0
				}
				download.DownloadSpeed = 5.2 * 1024 * 1024 // 5.2 MB/s
				err = db.UpdateDownload(download)
				require.NoError(t, err)
			}
		}

		// Call progress update endpoint
		form := url.Values{}
		form.Add("status", "downloading")
		form.Add("status", "pending")
		form.Add("sort", "desc")

		req := httptest.NewRequest("POST", "/downloads/progress", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handlers.UpdateDownloadProgress(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		responseBody := w.Body.String()
		
		// Should contain progress updates for downloading items
		if len(downloads) > 0 {
			assert.Contains(t, responseBody, "hx-swap-oob")
			// Should contain updated progress values
			assert.Contains(t, responseBody, "75.5%") // Updated progress
		}
	})

	t.Run("Mixed search with filters", func(t *testing.T) {
		tests := []struct {
			name     string
			statuses []string
			expected []string
			notExpected []string
		}{
			{
				name:     "only completed",
				statuses: []string{"completed"},
				expected: []string{"single-file.zip", "group-2"},
				notExpected: []string{"another-single.zip"},
			},
			{
				name:     "only downloading",
				statuses: []string{"downloading"},
				expected: []string{"group-1", "another-single.zip"},
				notExpected: []string{"group-2"},
			},
			{
				name:     "completed and downloading",
				statuses: []string{"completed", "downloading"},
				expected: []string{"single-file.zip", "group-1", "group-2", "another-single.zip"},
				notExpected: []string{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				form := url.Values{}
				for _, status := range tt.statuses {
					form.Add("status", status)
				}
				form.Add("sort", "desc")

				req := httptest.NewRequest("POST", "/downloads/search", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				w := httptest.NewRecorder()

				handlers.SearchDownloads(w, req)
				assert.Equal(t, http.StatusOK, w.Code)

				responseBody := w.Body.String()

				for _, expected := range tt.expected {
					assert.Contains(t, responseBody, expected, "Should contain %s", expected)
				}

				for _, notExpected := range tt.notExpected {
					assert.NotContains(t, responseBody, notExpected, "Should not contain %s", notExpected)
				}
			})
		}
	})

	t.Run("Group statistics calculation", func(t *testing.T) {
		displayItems := handlers.createDisplayItems(testDownloads)
		
		var group1Item *models.DownloadDisplayItem
		var group2Item *models.DownloadDisplayItem
		
		for _, item := range displayItems {
			if item.IsGroup {
				switch item.GroupID {
				case "group-1":
					group1Item = item
				case "group-2":
					group2Item = item
				}
			}
		}

		require.NotNil(t, group1Item, "Group 1 should exist")
		require.NotNil(t, group2Item, "Group 2 should exist")

		// Test Group 1 (mixed statuses)
		assert.Equal(t, 3, len(group1Item.Downloads))
		assert.Equal(t, models.StatusDownloading, group1Item.HighestPriorityStatus)
		assert.True(t, group1Item.HasActiveDownloads)
		assert.Equal(t, 1, group1Item.StatusCounts[models.StatusCompleted])
		assert.Equal(t, 1, group1Item.StatusCounts[models.StatusDownloading])
		assert.Equal(t, 1, group1Item.StatusCounts[models.StatusPending])

		// Test Group 2 (all completed)
		assert.Equal(t, 2, len(group2Item.Downloads))
		assert.Equal(t, models.StatusCompleted, group2Item.HighestPriorityStatus)
		assert.False(t, group2Item.HasActiveDownloads)
		assert.Equal(t, 2, group2Item.StatusCounts[models.StatusCompleted])
		assert.Equal(t, 0, group2Item.StatusCounts[models.StatusDownloading])
	})

	t.Run("Sorting preservation with groups", func(t *testing.T) {
		// Test descending sort (newest first)
		displayItems := handlers.createDisplayItems(testDownloads)
		
		// Verify order is maintained (by order of first occurrence in the sorted downloads slice)
		expectedOrder := []string{
			"single-file.zip",    // First single download in slice
			"group-1",            // First occurrence of group-1 
			"group-2",            // First occurrence of group-2
			"another-single.zip", // Last single download in slice
		}

		actualOrder := make([]string, 0, len(displayItems))
		for _, item := range displayItems {
			if item.IsGroup {
				actualOrder = append(actualOrder, item.GroupID)
			} else {
				actualOrder = append(actualOrder, item.Download.Filename)
			}
		}

		assert.Equal(t, expectedOrder, actualOrder, "Items should be sorted by creation time")
	})
}

// TestGroupedDownloadsEdgeCases tests edge cases and error conditions
func TestGroupedDownloadsEdgeCases(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	worker := downloader.NewWorker(db, "/tmp/test")
	handlers := NewHandlers(db, client, "/tmp/test", worker)

	t.Run("single item in group", func(t *testing.T) {
		download := &models.Download{
			Filename:    "lonely-file.zip",
			OriginalURL: "https://example.com/lonely.zip",
			Directory:   "/tmp/test",
			Status:      models.StatusCompleted,
			Progress:    100.0,
			GroupID:     "lonely-group",
			CreatedAt:   time.Now(),
		}

		err = db.CreateDownload(download)
		require.NoError(t, err)

		displayItems := handlers.createDisplayItems([]*models.Download{download})
		
		assert.Equal(t, 1, len(displayItems))
		assert.True(t, displayItems[0].IsGroup)
		assert.Equal(t, "lonely-group", displayItems[0].GroupID)
		assert.Equal(t, 1, len(displayItems[0].Downloads))
	})

	t.Run("empty downloads list", func(t *testing.T) {
		displayItems := handlers.createDisplayItems([]*models.Download{})
		assert.Equal(t, 0, len(displayItems))
	})

	t.Run("progress update with database error", func(t *testing.T) {
		// Close the database to simulate error
		db.Close()

		form := url.Values{}
		form.Add("status", "downloading")

		req := httptest.NewRequest("POST", "/downloads/progress", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handlers.UpdateDownloadProgress(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("concurrent state modifications", func(t *testing.T) {
		// Test thread safety of state management
		downloadID := int64(123)
		groupID := "test-group"

		// Simulate concurrent access
		done := make(chan bool, 2)

		go func() {
			for i := 0; i < 100; i++ {
				handlers.setDownloadExpanded(downloadID, i%2 == 0)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				handlers.setGroupExpanded(groupID, i%2 == 0)
			}
			done <- true
		}()

		// Wait for both goroutines to complete
		<-done
		<-done

		// State should be consistent (no race conditions)
		_ = handlers.isDownloadExpanded(downloadID, models.StatusCompleted)
		_ = handlers.isGroupExpanded(groupID, false)
	})
}