package fuzzy

import (
	"testing"

	"debrid-downloader/pkg/models"

	"github.com/stretchr/testify/require"
)

func TestMatcher_SuggestDirectory(t *testing.T) {
	matcher := NewMatcher()

	mappings := []*models.DirectoryMapping{
		{
			FilenamePattern: "movie",
			Directory:       "/downloads/movies",
			UseCount:        10,
		},
		{
			FilenamePattern: "music",
			Directory:       "/downloads/music",
			UseCount:        5,
		},
		{
			FilenamePattern: "game",
			Directory:       "/downloads/games",
			UseCount:        3,
		},
	}

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "exact match movie",
			filename: "Great.Movie.2023.mkv",
			want:     "/downloads/movies",
		},
		{
			name:     "exact match music",
			filename: "Best.Music.Album.mp3",
			want:     "/downloads/music",
		},
		{
			name:     "partial match",
			filename: "Some.Game.ISO",
			want:     "/downloads/games",
		},
		{
			name:     "no match",
			filename: "random.file.txt",
			want:     "",
		},
		{
			name:     "case insensitive",
			filename: "MOVIE.File.mkv",
			want:     "/downloads/movies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.SuggestDirectory(tt.filename, mappings)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMatcher_CalculateScore(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		pattern  string
		filename string
		want     float64
	}{
		{
			name:     "exact match",
			pattern:  "movie",
			filename: "movie.mkv",
			want:     0.5, // 1 match out of 2 words
		},
		{
			name:     "partial match",
			pattern:  "movie",
			filename: "great.movie.2023.mkv",
			want:     0.25, // 1 match out of 4 words
		},
		{
			name:     "no match",
			pattern:  "movie",
			filename: "music.album.mp3",
			want:     0.0,
		},
		{
			name:     "case insensitive",
			pattern:  "movie",
			filename: "MOVIE.FILE.MKV",
			want:     0.33, // 1 match out of 3 words
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.calculateScore(tt.pattern, tt.filename)
			require.InDelta(t, tt.want, got, 0.1)
		})
	}
}
