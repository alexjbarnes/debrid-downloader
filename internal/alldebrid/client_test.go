package alldebrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	client := New("test-api-key")
	require.NotNil(t, client)
	require.Equal(t, "test-api-key", client.apiKey)
}

func TestClient_UnrestrictLink(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		serverResponse string
		statusCode     int
		wantErr        bool
		wantURL        string
		wantFilename   string
	}{
		{
			name: "successful unrestrict",
			url:  "https://example.com/file.zip",
			serverResponse: `{
				"status": "success",
				"data": {
					"link": "https://alldebrid.com/dl/file.zip",
					"filename": "file.zip",
					"filesize": 1024000
				}
			}`,
			statusCode:   200,
			wantErr:      false,
			wantURL:      "https://alldebrid.com/dl/file.zip",
			wantFilename: "file.zip",
		},
		{
			name: "API error response",
			url:  "https://invalid.com/file.zip",
			serverResponse: `{
				"status": "error",
				"error": {
					"message": "Invalid link"
				}
			}`,
			statusCode: 200,
			wantErr:    true,
		},
		{
			name:           "HTTP error",
			url:            "https://example.com/file.zip",
			serverResponse: "Internal Server Error",
			statusCode:     500,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
					t.Errorf("Failed to write test response: %v", err)
				}
			}))
			defer server.Close()

			// Create client with test server URL
			client := New("test-api-key")
			client.baseURL = server.URL

			ctx := context.Background()
			result, err := client.UnrestrictLink(ctx, tt.url)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.wantURL, result.UnrestrictedURL)
			require.Equal(t, tt.wantFilename, result.Filename)
		})
	}
}

func TestClient_CheckAPIKey(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		statusCode     int
		wantErr        bool
		expectedError  string
	}{
		{
			name: "valid API key",
			serverResponse: `{
				"status": "success",
				"data": {
					"user": "testuser"
				}
			}`,
			statusCode: 200,
			wantErr:    false,
		},
		{
			name: "invalid API key with int code",
			serverResponse: `{
				"status": "error",
				"error": {
					"message": "Invalid API key",
					"code": 401
				}
			}`,
			statusCode:    200,
			wantErr:       true,
			expectedError: "Invalid API key (code: 401)",
		},
		{
			name: "invalid API key with string code",
			serverResponse: `{
				"status": "error",
				"error": {
					"message": "Invalid API key",
					"code": "INVALID_KEY"
				}
			}`,
			statusCode:    200,
			wantErr:       true,
			expectedError: "Invalid API key (code: INVALID_KEY)",
		},
		{
			name: "error without code",
			serverResponse: `{
				"status": "error",
				"error": {
					"message": "Some API error"
				}
			}`,
			statusCode:    200,
			wantErr:       true,
			expectedError: "Some API error",
		},
		{
			name:           "HTTP error",
			serverResponse: "Internal Server Error",
			statusCode:     500,
			wantErr:        true,
		},
		{
			name: "malformed JSON",
			serverResponse: `{
				"status": "error",
				"error": {
					"message": "Bad request"
					// missing comma
				}
			}`,
			statusCode: 200,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
					t.Errorf("Failed to write test response: %v", err)
				}
			}))
			defer server.Close()

			// Create client with test server URL
			client := New("test-api-key")
			client.baseURL = server.URL

			ctx := context.Background()
			err := client.CheckAPIKey(ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.expectedError != "" {
					require.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiError APIError
		want     string
	}{
		{
			name: "error with int code",
			apiError: APIError{
				Message: "Invalid request",
				Code:    401,
			},
			want: "Invalid request (code: 401)",
		},
		{
			name: "error with string code",
			apiError: APIError{
				Message: "Bad API key",
				Code:    "INVALID_KEY",
			},
			want: "Bad API key (code: INVALID_KEY)",
		},
		{
			name: "error without code",
			apiError: APIError{
				Message: "Something went wrong",
				Code:    nil,
			},
			want: "Something went wrong",
		},
		{
			name: "error with empty code",
			apiError: APIError{
				Message: "Another error",
			},
			want: "Another error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.apiError.Error()
			require.Equal(t, tt.want, got)
		})
	}
}
