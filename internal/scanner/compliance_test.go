package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsReleaseGroupFolder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Clean name", "Movie Name (2024)", false},
		{"With resolution", "Movie.Name.2024.1080p", true},
		{"With BluRay", "Movie Name 2024 BluRay", true},
		{"With codec", "Movie.Name.x264", true},
		{"With release group", "Movie-GROUP", true},
		{"With many dots", "Movie.Name.2024.Something.Else", true},
		{"Clean with spaces", "Movie Name", false},
		{"Year only", "Movie (2024)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReleaseGroupFolder(tt.input)
			if result != tt.expected {
				t.Errorf("isReleaseGroupFolder(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasYear(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Movie Name (2024)", true},
		{"Movie Name 2024", true},
		{"Movie.Name.2024.1080p", true},
		{"Movie Name", false},
		{"Movie Name 24", false},
	}

	for _, tt := range tests {
		result := hasYear(tt.input)
		if result != tt.expected {
			t.Errorf("hasYear(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestHasYearInParentheses(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Movie Name (2024)", true},
		{"Movie Name [2024]", false},
		{"Movie Name 2024", false},
		{"Movie.Name.2024.1080p", false},
		{"Movie (2024) 1080p", true},
	}

	for _, tt := range tests {
		result := hasYearInParentheses(tt.input)
		if result != tt.expected {
			t.Errorf("hasYearInParentheses(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestCheckMovieCompliance(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		setupPath       string
		expectIssue     bool
		expectedProblem string
	}{
		{
			name:            "Release group folder",
			setupPath:       "Movie.Name.2024.1080p.BluRay-GROUP/movie.mkv",
			expectIssue:     true,
			expectedProblem: "Release group folder naming",
		},
		{
			name:            "File in library root",
			setupPath:       "Movie.Name.2024.mkv",
			expectIssue:     true,
			expectedProblem: "Movie file directly in library root",
		},
		{
			name:            "Mismatched folder/filename",
			setupPath:       "Movie Name (2024)/Different Name (2024).mkv",
			expectIssue:     true,
			expectedProblem: "Folder name doesn't match filename",
		},
		{
			name:            "Year not in parentheses",
			setupPath:       "Movie Name 2024/Movie Name 2024.mkv",
			expectIssue:     true,
			expectedProblem: "Year not in parentheses format",
		},
		{
			name:        "Compliant structure",
			setupPath:   "Movie Name (2024)/Movie Name (2024).mkv",
			expectIssue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file structure
			fullPath := filepath.Join(tmpDir, tt.setupPath)
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			os.WriteFile(fullPath, []byte("test"), 0644)

			// Check compliance
			issue := checkMovieCompliance(fullPath, tmpDir)

			if tt.expectIssue {
				if issue == nil {
					t.Errorf("Expected compliance issue but got none")
				} else if !contains(issue.Problem, tt.expectedProblem) {
					t.Errorf("Expected problem containing %q, got %q", tt.expectedProblem, issue.Problem)
				}
			} else {
				if issue != nil {
					t.Errorf("Expected no issue but got: %s", issue.Problem)
				}
			}

			// Cleanup
			os.RemoveAll(filepath.Dir(fullPath))
		})
	}
}

func TestCheckTVCompliance(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		setupPath       string
		season          int
		episode         int
		expectIssue     bool
		expectedProblem string
	}{
		{
			name:            "Wrong season folder format",
			setupPath:       "Show Name (2024)/Season 1/episode.mkv",
			season:          1,
			episode:         1,
			expectIssue:     true,
			expectedProblem: "Not in proper 'Season 01' folder",
		},
		{
			name:            "Release group filename",
			setupPath:       "Show Name (2024)/Season 01/Show.Name.S01E01.1080p.WEB-DL-GROUP.mkv",
			season:          1,
			episode:         1,
			expectIssue:     true,
			expectedProblem: "Release group naming in filename",
		},
		{
			name:        "Compliant structure",
			setupPath:   "Show Name (2024)/Season 01/Show Name (2024) S01E01.mkv",
			season:      1,
			episode:     1,
			expectIssue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file structure
			fullPath := filepath.Join(tmpDir, tt.setupPath)
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			os.WriteFile(fullPath, []byte("test"), 0644)

			// Check compliance
			issue := checkTVCompliance(fullPath, tmpDir, tt.season, tt.episode)

			if tt.expectIssue {
				if issue == nil {
					t.Errorf("Expected compliance issue but got none")
				} else if !contains(issue.Problem, tt.expectedProblem) {
					t.Errorf("Expected problem containing %q, got %q", tt.expectedProblem, issue.Problem)
				}
			} else {
				if issue != nil {
					t.Errorf("Expected no issue but got: %s", issue.Problem)
				}
			}

			// Cleanup
			os.RemoveAll(filepath.Dir(fullPath))
		})
	}
}

func TestScanMovieCompliance(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mix of compliant and non-compliant structures
	testFiles := []struct {
		path      string
		compliant bool
	}{
		{"Movie Name (2024)/Movie Name (2024).mkv", true},
		{"Bad.Movie.2024.1080p.BluRay/movie.mkv", false},
		{"Another Movie 2024/Another Movie 2024.mkv", false}, // Year not in parens
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(tmpDir, tf.path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte("test"), 0644)
	}

	// Scan for compliance issues
	issues, err := ScanMovieCompliance([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovieCompliance() error: %v", err)
	}

	// Should find 2 issues (Bad.Movie and Another Movie)
	if len(issues) != 2 {
		t.Errorf("Expected 2 compliance issues, got %d", len(issues))
	}

	// Verify all issues have suggestions
	for _, issue := range issues {
		if issue.SuggestedPath == "" {
			t.Error("Issue missing suggested path")
		}
		if issue.SuggestedAction == "" {
			t.Error("Issue missing suggested action")
		}
	}
}

func TestScanTVCompliance(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mix of compliant and non-compliant structures
	testFiles := []string{
		"Show Name (2024)/Season 01/Show Name (2024) S01E01.mkv",             // Compliant
		"Show Name (2024)/Season 1/Show.Name.S01E02.1080p.WEB-DL.mkv",       // Wrong season format
		"Show Name (2024)/Season 01/Show.S01E03.720p.HDTV-GROUP.mkv",        // Release group
	}

	for _, path := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte("test"), 0644)
	}

	// Scan for compliance issues
	issues, err := ScanTVCompliance([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanTVCompliance() error: %v", err)
	}

	// Should find 2 issues (wrong season folder and release group filename)
	if len(issues) != 2 {
		t.Errorf("Expected 2 compliance issues, got %d", len(issues))
	}

	// Verify all issues have suggestions
	for _, issue := range issues {
		if issue.SuggestedPath == "" {
			t.Error("Issue missing suggested path")
		}
		if issue.SuggestedAction == "" {
			t.Error("Issue missing suggested action")
		}
		if issue.Type != "tv" {
			t.Errorf("Expected type 'tv', got %q", issue.Type)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		len(s) > len(substr)+1 && s[1:len(substr)+1] == substr))
}
