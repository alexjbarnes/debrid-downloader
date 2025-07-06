# pkg/fuzzy - Fuzzy Matching for Directory Suggestions

[![Go Reference](https://pkg.go.dev/badge/debrid-downloader/pkg/fuzzy.svg)](https://pkg.go.dev/debrid-downloader/pkg/fuzzy)
[![Test Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen)](./matcher_test.go)

**Last Updated:** July 6, 2025  
**Version:** 1.0.0  
**Token Count:** ~2,500

## Overview

The `pkg/fuzzy` package provides intelligent fuzzy matching functionality for directory suggestions in the debrid-downloader application. It implements a machine learning-like system that learns from user behavior to suggest appropriate download directories based on filename patterns.

### Key Features

- **Intelligent Pattern Matching**: Uses word-based fuzzy matching with exact word prioritization
- **Usage-Based Learning**: Learns from historical user choices to improve suggestions over time
- **Weighted Scoring**: Combines pattern matching scores with usage frequency for better suggestions
- **Case-Insensitive Matching**: Handles filenames regardless of case
- **Word Boundary Recognition**: Splits filenames on common delimiters (`.`, `_`, `-`, spaces)

## Quick Start

```go
import "debrid-downloader/pkg/fuzzy"

// Create a new matcher
matcher := fuzzy.NewMatcher()

// Get directory suggestions based on filename and historical mappings
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
}

suggestion := matcher.SuggestDirectory("Great.Movie.2023.mkv", mappings)
// Returns: "/downloads/movies"
```

## Architecture

### Core Components

#### Matcher
The main `Matcher` struct provides fuzzy matching functionality:

```go
type Matcher struct{}

func NewMatcher() *Matcher
func (m *Matcher) SuggestDirectory(filename string, mappings []*models.DirectoryMapping) string
```

#### Scoring Algorithm
The scoring system uses a two-phase approach:

1. **Pattern Matching Score**: Calculates fuzzy match score between pattern and filename
2. **Usage Weighting**: Multiplies base score by usage frequency factor

```go
// Base score calculation
score := m.calculateScore(mapping.FilenamePattern, filename)

// Usage weighting (10% boost per use)
weightedScore := score * (1.0 + float64(mapping.UseCount) * 0.1)
```

### Matching Algorithm Details

#### Word-Based Matching
The algorithm splits filenames and patterns into words using common delimiters:

```go
// Delimiter function
func(r rune) bool {
    return r == '.' || r == '_' || r == '-' || r == ' '
}
```

#### Score Calculation
- **Exact Word Matches**: Counts exact matches between pattern words and filename words
- **Proportional Scoring**: `score = exactMatches / totalFilenameWords`
- **Fallback**: Uses simple containment ratio if no word matches found

#### Example Scoring
```
Pattern: "movie"
Filename: "Great.Movie.2023.mkv"
Words: ["great", "movie", "2023", "mkv"]
Exact matches: 1 ("movie")
Score: 1/4 = 0.25
```

## Directory Suggestion System

### Learning Mechanism
The system learns from user behavior through `DirectoryMapping` records:

```go
type DirectoryMapping struct {
    FilenamePattern string    // Learned pattern from filename
    Directory       string    // User's chosen directory
    UseCount        int       // Number of times this mapping was used
    LastUsed        time.Time // When this mapping was last used
}
```

### Suggestion Process
1. **Pattern Matching**: Calculate base scores for all mappings
2. **Usage Weighting**: Apply usage frequency multiplier
3. **Ranking**: Sort by weighted score (descending)
4. **Selection**: Return directory from highest-scoring mapping

### Performance Characteristics
- **Time Complexity**: O(n × m × k) where:
  - n = number of mappings
  - m = average words per pattern
  - k = average words per filename
- **Space Complexity**: O(n) for scoring intermediate results
- **Optimization**: Early termination for zero-score patterns

## Configuration and Tuning

### Scoring Parameters

#### Usage Weight Factor
```go
// Current: 10% boost per use
weightedScore := score * (1.0 + float64(mapping.UseCount) * 0.1)

// Configurable alternatives:
// Conservative: 0.05 (5% boost)
// Aggressive: 0.2 (20% boost)
```

#### Minimum Score Threshold
```go
// Current: Any score > 0
if score > 0 {
    // Process mapping
}

// Configurable threshold:
const MinScoreThreshold = 0.1
if score > MinScoreThreshold {
    // Process mapping
}
```

### Customization Examples

#### Custom Delimiter Set
```go
// Add custom delimiters
func customDelimiterFunc(r rune) bool {
    return r == '.' || r == '_' || r == '-' || r == ' ' || r == '[' || r == ']'
}
```

#### Boosting Strategies
```go
// Recency-based boosting
recentBoost := 1.0
if time.Since(mapping.LastUsed) < 24*time.Hour {
    recentBoost = 1.2
}
weightedScore := score * (1.0 + float64(mapping.UseCount)*0.1) * recentBoost
```

## Testing

### Test Coverage
The package maintains 100% test coverage with comprehensive test cases:

#### Unit Tests
- **Pattern Matching**: Exact matches, partial matches, case sensitivity
- **Scoring Algorithm**: Various filename patterns and edge cases
- **Directory Suggestion**: Integration tests with multiple mappings

#### Test Data
```go
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
}
```

#### Test Cases
- **Exact Match**: `"Great.Movie.2023.mkv"` → `"/downloads/movies"`
- **Partial Match**: `"Some.Game.ISO"` → `"/downloads/games"`
- **Case Insensitive**: `"MOVIE.File.mkv"` → `"/downloads/movies"`
- **No Match**: `"random.file.txt"` → `""`

### Running Tests
```bash
# Run all tests
go test ./pkg/fuzzy

# Run with coverage
go test -cover ./pkg/fuzzy

# Run with race detection
go test -race ./pkg/fuzzy

# Detailed coverage
go test -coverprofile=coverage.out ./pkg/fuzzy
go tool cover -html=coverage.out
```

## Usage Examples

### Basic Directory Suggestion
```go
matcher := fuzzy.NewMatcher()
mappings := getHistoricalMappings() // From database

suggestion := matcher.SuggestDirectory("Linux.Distro.ISO", mappings)
if suggestion != "" {
    fmt.Printf("Suggested directory: %s\n", suggestion)
}
```

### Integration with Database
```go
func suggestDirectoryForDownload(filename string, db *sql.DB) string {
    // Fetch historical mappings from database
    mappings, err := database.GetDirectoryMappings(db)
    if err != nil {
        return ""
    }

    matcher := fuzzy.NewMatcher()
    return matcher.SuggestDirectory(filename, mappings)
}
```

### Learning from User Choices
```go
func recordUserChoice(filename, directory string, db *sql.DB) error {
    // Extract pattern from filename
    pattern := extractPattern(filename)
    
    // Update or create mapping
    mapping := &models.DirectoryMapping{
        FilenamePattern: pattern,
        Directory:       directory,
        UseCount:        1,
        LastUsed:        time.Now(),
    }
    
    return database.UpsertDirectoryMapping(db, mapping)
}
```

### Custom Scoring Strategy
```go
type CustomMatcher struct {
    *fuzzy.Matcher
    boostFactor float64
}

func (cm *CustomMatcher) SuggestDirectory(filename string, mappings []*models.DirectoryMapping) string {
    // Apply custom boost to recent mappings
    for _, mapping := range mappings {
        if time.Since(mapping.LastUsed) < 24*time.Hour {
            mapping.UseCount = int(float64(mapping.UseCount) * cm.boostFactor)
        }
    }
    
    return cm.Matcher.SuggestDirectory(filename, mappings)
}
```

## Integration Patterns

### Web Handler Integration
```go
func downloadHandler(w http.ResponseWriter, r *http.Request) {
    filename := r.FormValue("filename")
    
    // Get suggestion
    matcher := fuzzy.NewMatcher()
    mappings := getDirectoryMappings()
    suggestion := matcher.SuggestDirectory(filename, mappings)
    
    // Use suggestion as default in UI
    data := struct {
        Filename          string
        SuggestedDirectory string
    }{
        Filename:          filename,
        SuggestedDirectory: suggestion,
    }
    
    renderTemplate(w, "download.html", data)
}
```

### Background Learning Service
```go
type LearningService struct {
    db      *sql.DB
    matcher *fuzzy.Matcher
}

func (s *LearningService) ProcessCompletedDownload(download *models.Download) {
    // Extract pattern from filename
    pattern := extractPatternFromFilename(download.Filename)
    
    // Update mapping
    mapping := &models.DirectoryMapping{
        FilenamePattern: pattern,
        Directory:       download.Directory,
        UseCount:        1,
        LastUsed:        time.Now(),
    }
    
    s.db.UpsertDirectoryMapping(mapping)
}
```

## Performance Considerations

### Memory Usage
- **Minimal Overhead**: Stateless matcher with no internal caching
- **Temporary Allocations**: Scoring slice allocated per suggestion request
- **String Operations**: Case conversion and splitting create temporary strings

### CPU Usage
- **Linear Complexity**: Scales with number of mappings
- **String Operations**: Dominant cost in pattern matching
- **Optimization Opportunities**: Pre-computed pattern words, score caching

### Scaling Recommendations
- **Mapping Limits**: Consider limiting mappings to most recent/frequent entries
- **Pattern Optimization**: Pre-process patterns for faster matching
- **Caching**: Cache suggestions for identical filenames

## Error Handling

### Defensive Programming
```go
// Empty mappings check
if len(mappings) == 0 {
    return ""
}

// Score validation
if score > 0 {
    // Only process positive scores
}
```

### Edge Cases
- **Empty Filename**: Returns empty string
- **No Mappings**: Returns empty string  
- **No Matches**: Returns empty string
- **Nil Mappings**: Handled gracefully

## Future Enhancements

### Potential Improvements
1. **Advanced Algorithms**: Levenshtein distance, n-gram matching
2. **Machine Learning**: Train on larger datasets for better patterns
3. **Contextual Hints**: Consider file size, URL patterns, timestamps
4. **Performance**: Parallel processing for large mapping sets
5. **Configuration**: Pluggable scoring strategies

### Compatibility
- **Go Version**: Requires Go 1.21+ (generics used in dependencies)
- **Dependencies**: Only depends on `debrid-downloader/pkg/models`
- **Thread Safety**: Stateless design is safe for concurrent use

## Contributing

### Development Setup
1. Install Go 1.21+
2. Run `go mod tidy`
3. Run tests: `go test ./pkg/fuzzy`

### Code Style
- Follow standard Go formatting (`gofmt`)
- Add comprehensive test coverage for new features
- Document public APIs with Go doc comments

### Testing Requirements
- Maintain 100% test coverage
- Add benchmarks for performance-critical changes
- Test edge cases and error conditions

## License

This package is part of the debrid-downloader project and follows the same license terms.