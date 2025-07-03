// Package fuzzy provides fuzzy matching functionality for directory suggestions
package fuzzy

import (
	"sort"
	"strings"

	"debrid-downloader/pkg/models"
)

// Matcher provides fuzzy matching functionality
type Matcher struct{}

// NewMatcher creates a new fuzzy matcher
func NewMatcher() *Matcher {
	return &Matcher{}
}

// SuggestDirectory suggests a directory based on filename and historical mappings
func (m *Matcher) SuggestDirectory(filename string, mappings []*models.DirectoryMapping) string {
	if len(mappings) == 0 {
		return ""
	}

	type scoredMapping struct {
		mapping *models.DirectoryMapping
		score   float64
	}

	var scored []scoredMapping
	filename = strings.ToLower(filename)

	// Calculate scores for each mapping
	for _, mapping := range mappings {
		score := m.calculateScore(mapping.FilenamePattern, filename)
		if score > 0 {
			// Weight score by usage count
			weightedScore := score * (1.0 + float64(mapping.UseCount)*0.1)
			scored = append(scored, scoredMapping{
				mapping: mapping,
				score:   weightedScore,
			})
		}
	}

	if len(scored) == 0 {
		return ""
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return scored[0].mapping.Directory
}

// calculateScore calculates the fuzzy match score between pattern and filename
func (m *Matcher) calculateScore(pattern, filename string) float64 {
	pattern = strings.ToLower(pattern)
	filename = strings.ToLower(filename)

	if !strings.Contains(filename, pattern) {
		return 0.0
	}

	// Calculate score based on how much of the filename is the pattern
	// Higher score for exact matches or when pattern represents more of the filename
	filenameWords := strings.FieldsFunc(filename, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})

	patternWords := strings.FieldsFunc(pattern, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})

	// Count exact word matches
	exactMatches := 0
	for _, pWord := range patternWords {
		for _, fWord := range filenameWords {
			if pWord == fWord {
				exactMatches++
				break
			}
		}
	}

	// Score based on exact word matches vs total words
	if len(patternWords) > 0 {
		return float64(exactMatches) / float64(len(filenameWords))
	}

	// Fallback to simple containment score
	return float64(len(pattern)) / float64(len(filename))
}
