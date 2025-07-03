package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"debrid-downloader/internal/alldebrid"
	"debrid-downloader/internal/alldebrid/mocks"
	"debrid-downloader/internal/database"
	"debrid-downloader/pkg/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewHandlers(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")

	handlers := NewHandlers(db, client, "/tmp/test")
	require.NotNil(t, handlers)
	require.Equal(t, db, handlers.db)
	require.Equal(t, client, handlers.allDebridClient)
}

func TestHandlers_Home(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handlers.Home(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestHandlers_History(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/history", nil)
	w := httptest.NewRecorder()

	handlers.History(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestHandlers_CurrentDownloads(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/downloads/current", nil)
	w := httptest.NewRecorder()

	handlers.CurrentDownloads(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestHandlers_SubmitDownload(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	tests := []struct {
		name     string
		formData string
		wantCode int
		wantBody string
	}{
		{
			name:     "valid submission with invalid API key",
			formData: "url=https://example.com/file.zip&directory=/downloads",
			wantCode: 200,
			wantBody: "apikey is invalid",
		},
		{
			name:     "missing URL",
			formData: "directory=/downloads",
			wantCode: 200,
			wantBody: "URL is required",
		},
		{
			name:     "missing directory",
			formData: "url=https://example.com/file.zip",
			wantCode: 200,
			wantBody: "Directory is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}
			for _, pair := range strings.Split(tt.formData, "&") {
				if pair != "" {
					parts := strings.Split(pair, "=")
					if len(parts) == 2 {
						form.Set(parts[0], parts[1])
					}
				}
			}

			req := httptest.NewRequest("POST", "/download", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			handlers.SubmitDownload(w, req)

			require.Equal(t, tt.wantCode, w.Code)
			require.Contains(t, w.Body.String(), tt.wantBody)
		})
	}
}

func TestHandlers_SubmitDownloadWithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	mockClient := mocks.NewMockAllDebridClient(ctrl)
	handlers := NewHandlers(db, mockClient, "/tmp/test")

	// Mock successful API response
	mockClient.EXPECT().
		UnrestrictLink(gomock.Any(), "https://example.com/file.zip").
		Return(&alldebrid.UnrestrictResult{
			UnrestrictedURL: "https://dl.alldebrid.com/file.zip",
			Filename:        "file.zip",
			FileSize:        1024000,
		}, nil)

	form := url.Values{}
	form.Set("url", "https://example.com/file.zip")
	form.Set("directory", "/downloads")

	req := httptest.NewRequest("POST", "/download", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handlers.SubmitDownload(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "successfully")

	// Verify download was created in database
	downloads, err := db.ListDownloads(10, 0)
	require.NoError(t, err)
	require.Len(t, downloads, 1)
	require.Equal(t, "https://example.com/file.zip", downloads[0].OriginalURL)
	require.Equal(t, "file.zip", downloads[0].Filename)
}

func TestHandlers_SubmitDownloadAPIError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	mockClient := mocks.NewMockAllDebridClient(ctrl)
	handlers := NewHandlers(db, mockClient, "/tmp/test")

	// Mock API error
	mockClient.EXPECT().
		UnrestrictLink(gomock.Any(), "https://example.com/file.zip").
		Return(nil, &alldebrid.APIError{Message: "Invalid link", Code: 42})

	form := url.Values{}
	form.Set("url", "https://example.com/file.zip")
	form.Set("directory", "/downloads")

	req := httptest.NewRequest("POST", "/download", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handlers.SubmitDownload(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Invalid link")

	// Verify no download was created in database
	downloads, err := db.ListDownloads(10, 0)
	require.NoError(t, err)
	require.Len(t, downloads, 0)
}

func TestHandlers_HistoryWithData(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create test data
	download := &models.Download{
		OriginalURL: "https://example.com/file.zip",
		Filename:    "file.zip",
		Directory:   "/downloads",
		Status:      models.StatusCompleted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = db.CreateDownload(download)
	require.NoError(t, err)

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/history", nil)
	w := httptest.NewRecorder()

	handlers.History(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "file.zip")
}

func TestHandlers_CurrentDownloadsWithData(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create test data
	download := &models.Download{
		OriginalURL: "https://example.com/file.zip",
		Filename:    "file.zip",
		Directory:   "/downloads",
		Status:      models.StatusDownloading,
		Progress:    50.0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err = db.CreateDownload(download)
	require.NoError(t, err)

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/downloads/current", nil)
	w := httptest.NewRecorder()

	handlers.CurrentDownloads(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestHandlers_HomeWithDirectorySuggestions(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/?filename=movie.mkv", nil)
	w := httptest.NewRecorder()

	handlers.Home(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestHandlers_SubmitDownloadParseError(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	// Create malformed request
	req := httptest.NewRequest("POST", "/download", strings.NewReader("%invalid%form%data"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handlers.SubmitDownload(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Failed to parse form data")
}

func TestHandlers_SubmitDownloadDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Use a closed database to trigger database error
	db, err := database.New(":memory:")
	require.NoError(t, err)
	db.Close() // Close immediately to cause errors

	mockClient := mocks.NewMockAllDebridClient(ctrl)
	handlers := NewHandlers(db, mockClient, "/tmp/test")

	// Mock successful API response
	mockClient.EXPECT().
		UnrestrictLink(gomock.Any(), "https://example.com/file.zip").
		Return(&alldebrid.UnrestrictResult{
			UnrestrictedURL: "https://dl.alldebrid.com/file.zip",
			Filename:        "file.zip",
			FileSize:        1024000,
		}, nil)

	form := url.Values{}
	form.Set("url", "https://example.com/file.zip")
	form.Set("directory", "/downloads")

	req := httptest.NewRequest("POST", "/download", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handlers.SubmitDownload(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Failed to create download record")
}

func TestHandlers_HistoryDatabaseError(t *testing.T) {
	// Use a closed database to trigger database error
	db, err := database.New(":memory:")
	require.NoError(t, err)
	db.Close() // Close immediately to cause errors

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/history", nil)
	w := httptest.NewRecorder()

	handlers.History(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "Internal server error")
}

func TestHandlers_CurrentDownloadsDatabaseError(t *testing.T) {
	// Use a closed database to trigger database error
	db, err := database.New(":memory:")
	require.NoError(t, err)
	db.Close() // Close immediately to cause errors

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	req := httptest.NewRequest("GET", "/downloads/current", nil)
	w := httptest.NewRecorder()

	handlers.CurrentDownloads(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "Internal server error")
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		pattern   string
		wantScore int
	}{
		{"exact match", "movie.mp4", "movie.mp4", 100},
		{"substring match", "action.movie.mp4", "movie", 100},
		{"extension match", "file.mp4", "test.mp4", 80},
		{"word match", "action_movie_2023.mp4", "movie_action", 60},
		{"no match", "document.pdf", "video.mp4", 0},
		{"empty pattern", "test.mp4", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := fuzzyMatch(tt.filename, tt.pattern)
			require.Equal(t, tt.wantScore, score)
		})
	}
}

func TestContentBasedScore(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/downloads")

	tests := []struct {
		name      string
		filename  string
		directory string
		wantScore int
	}{
		{"movie in movies dir", "action.mp4", "/downloads/movies", 90},
		{"movie in tv dir", "show.mp4", "/downloads/tv", 85},
		{"music in music dir", "song.mp3", "/downloads/music", 90},
		{"software in software dir", "setup.exe", "/downloads/software", 90},
		{"movie keyword in movies", "batman.movie.mkv", "/downloads/movies", 90},
		{"default downloads", "file.txt", "/downloads", 50},
		{"no match", "document.pdf", "/random", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := handlers.contentBasedScore(tt.filename, tt.directory)
			require.Equal(t, tt.wantScore, score)
		})
	}
}

func TestExtractPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"video extension", "movie.mp4", ".mp4"},
		{"audio extension", "song.mp3", ".mp3"},
		{"movie keyword", "action_movie_2023", "movie"},
		{"tv show pattern", "show_s01e01", "tv_show"},
		{"software keyword", "program_setup", "setup"},
		{"no pattern", "randomfile", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPattern(tt.filename)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestGetDirectorySuggestions(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	// Test with empty filename
	suggestedDir, recentDirs := handlers.getDirectorySuggestions("")
	require.Equal(t, "/tmp/test", suggestedDir)
	require.Len(t, recentDirs, 5)

	// Test with movie file
	suggestedDir, recentDirs = handlers.getDirectorySuggestions("action.movie.2023.mp4")
	require.NotEmpty(t, suggestedDir)
	require.Len(t, recentDirs, 5)

	// Test with music file
	suggestedDir, recentDirs = handlers.getDirectorySuggestions("album.song.mp3")
	require.NotEmpty(t, suggestedDir)
	require.Len(t, recentDirs, 5)
}

func TestCreateOrUpdateDirectoryMapping(t *testing.T) {
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	client := alldebrid.New("test-key")
	handlers := NewHandlers(db, client, "/tmp/test")

	// Test creating new mapping
	err = handlers.createOrUpdateDirectoryMapping("movie.mp4", "/downloads/movies")
	require.NoError(t, err)

	// Verify mapping was created
	mappings, err := db.GetDirectoryMappings()
	require.NoError(t, err)
	require.Len(t, mappings, 1)
	require.Equal(t, ".mp4", mappings[0].FilenamePattern)
	require.Equal(t, "/downloads/movies", mappings[0].Directory)

	// Test updating existing mapping
	err = handlers.createOrUpdateDirectoryMapping("another.mp4", "/downloads/movies")
	require.NoError(t, err)

	// Verify use count was updated
	mappings, err = db.GetDirectoryMappings()
	require.NoError(t, err)
	require.Len(t, mappings, 1)
	require.Equal(t, 2, mappings[0].UseCount)
}
