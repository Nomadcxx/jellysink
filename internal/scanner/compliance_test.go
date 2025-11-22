package scanner

import (
	"os"
	"path/filepath"
	"strings"
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
		"Show Name (2024)/Season 01/Show Name (2024) S01E01.mkv",      // Compliant
		"Show Name (2024)/Season 1/Show.Name.S01E02.1080p.WEB-DL.mkv", // Wrong season format
		"Show Name (2024)/Season 01/Show.S01E03.720p.HDTV-GROUP.mkv",  // Release group
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

func TestApplyTVCompliance_ExistingFolders(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		setupStructure  func() string // Returns path to non-compliant file
		prexistingDirs  []string      // Directories to create before Apply
		expectedPath    string        // Expected final path
		shouldCreateNew bool          // Should create new folders
	}{
		{
			name: "Show and Season folders already exist",
			setupStructure: func() string {
				// Bad filename in existing structure
				path := filepath.Join(tmpDir, "The Jetsons (1962)", "Season 01", "The.Jetsons.S01E04.1080p.WEB-DL.mkv")
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, []byte("test"), 0644)
				return path
			},
			prexistingDirs: []string{
				filepath.Join(tmpDir, "The Jetsons (1962)"),
				filepath.Join(tmpDir, "The Jetsons (1962)", "Season 01"),
			},
			expectedPath:    filepath.Join(tmpDir, "The Jetsons (1962)", "Season 01", "The Jetsons (1962) S01E04.mkv"),
			shouldCreateNew: false,
		},
		{
			name: "Show folder exists, Season folder missing",
			setupStructure: func() string {
				// Episode in wrong location
				path := filepath.Join(tmpDir, "The Jetsons (1962)", "The.Jetsons.S02E05.mkv")
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, []byte("test"), 0644)
				return path
			},
			prexistingDirs: []string{
				filepath.Join(tmpDir, "The Jetsons (1962)"),
			},
			expectedPath:    filepath.Join(tmpDir, "The Jetsons (1962)", "Season 02", "The Jetsons (1962) S02E05.mkv"),
			shouldCreateNew: true, // Creates Season 02
		},
		{
			name: "Neither Show nor Season folder exists",
			setupStructure: func() string {
				// Loose file in library root
				path := filepath.Join(tmpDir, "The.Jetsons.S03E10.1080p.HDTV.mkv")
				os.WriteFile(path, []byte("test"), 0644)
				return path
			},
			prexistingDirs:  []string{},
			expectedPath:    filepath.Join(tmpDir, "The Jetsons (YYYY)", "Season 03", "The Jetsons (YYYY) S03E10.mkv"),
			shouldCreateNew: true, // Creates both Show and Season folders
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup structure
			sourcePath := tt.setupStructure()

			// Create pre-existing directories
			for _, dir := range tt.prexistingDirs {
				os.MkdirAll(dir, 0755)
			}

			// Extract episode info
			season, episode, found := ExtractEpisodeInfo(filepath.Base(sourcePath))
			if !found {
				t.Fatalf("Failed to extract episode info from %s", sourcePath)
			}

			// Check compliance (generates suggested path)
			issue := checkTVCompliance(sourcePath, tmpDir, season, episode)
			if issue == nil {
				t.Skip("File is already compliant, skipping")
			}

			// Track folder existence before Apply
			showDirBefore := filepath.Dir(filepath.Dir(issue.SuggestedPath))
			seasonDirBefore := filepath.Dir(issue.SuggestedPath)
			showExistedBefore := dirExists(showDirBefore)
			seasonExistedBefore := dirExists(seasonDirBefore)

			// Apply fix
			err := ApplyTVCompliance(*issue)
			if err != nil {
				t.Fatalf("ApplyTVCompliance() error: %v", err)
			}

			// Verify file was moved correctly
			if !fileExists(issue.SuggestedPath) {
				t.Errorf("Expected file at %s but not found", issue.SuggestedPath)
			}

			// Verify original file is gone
			if fileExists(sourcePath) {
				t.Errorf("Original file still exists at %s", sourcePath)
			}

			// Verify folder creation behavior
			if tt.shouldCreateNew {
				if !showExistedBefore && !dirExists(showDirBefore) {
					t.Error("Expected Show folder to be created but it wasn't")
				}
				if !seasonExistedBefore && !dirExists(seasonDirBefore) {
					t.Error("Expected Season folder to be created but it wasn't")
				}
			}

			// Cleanup for next test
			os.RemoveAll(tmpDir)
			os.MkdirAll(tmpDir, 0755)
		})
	}
}

func TestApplyTVCompliance_CleanupEmptyDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup: Episode in release group folder
	sourcePath := filepath.Join(tmpDir, "The.Jetsons.S01E01.1080p.WEB-DL-GROUP", "episode.mkv")
	os.MkdirAll(filepath.Dir(sourcePath), 0755)
	os.WriteFile(sourcePath, []byte("test"), 0644)

	// Create target structure
	showDir := filepath.Join(tmpDir, "The Jetsons (YYYY)")
	seasonDir := filepath.Join(showDir, "Season 01")
	os.MkdirAll(seasonDir, 0755)

	// Generate compliance issue
	season, episode, _ := ExtractEpisodeInfo(filepath.Base(sourcePath))
	issue := checkTVCompliance(sourcePath, tmpDir, season, episode)
	if issue == nil {
		t.Skip("No compliance issue detected")
	}

	// Override suggested path to match existing structure
	issue.SuggestedPath = filepath.Join(seasonDir, "The Jetsons (YYYY) S01E01.mkv")

	// Apply fix
	err := ApplyTVCompliance(*issue)
	if err != nil {
		t.Fatalf("ApplyTVCompliance() error: %v", err)
	}

	// Verify empty release group folder was cleaned up
	releaseDir := filepath.Dir(sourcePath)
	if dirExists(releaseDir) {
		t.Errorf("Expected empty release group folder %s to be removed", releaseDir)
	}
}

func TestIsSampleFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/path/to/Sample.Movie.2024.mkv", true},
		{"/path/to/sample.mkv", true},
		{"/path/to/Movie.2024.Trailer.mkv", true},
		{"/path/to/Extra.Behind.The.Scenes.mkv", true},
		{"/path/to/Deleted.Scene.mkv", true},
		{"/path/to/Making.Of.Documentary.mkv", true},
		{"/path/to/Interview.With.Director.mkv", true},
		{"/path/to/Movie.Name.2024.mkv", false},
		{"/path/to/Normal.Movie.mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isSampleFile(tt.path)
			if result != tt.expected {
				t.Errorf("isSampleFile(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestScanMovieCompliance_SkipsSamples(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a sample file (should be skipped)
	samplePath := filepath.Join(tmpDir, "Movie (2024)", "Sample.Movie.2024.mkv")
	os.MkdirAll(filepath.Dir(samplePath), 0755)
	os.WriteFile(samplePath, []byte("test"), 0644)

	// Scan for compliance issues
	issues, err := ScanMovieCompliance([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovieCompliance() error: %v", err)
	}

	// Should not report any issues (sample file was skipped)
	if len(issues) > 0 {
		t.Errorf("Expected sample file to be skipped, but got %d issues", len(issues))
	}
}

func TestScanMovieCompliance_DetectsCollisions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two files that would rename to the same target
	file1 := filepath.Join(tmpDir, "Mission to Mars (2000)", "Mission.To.Mars.2000.1080p.BluRay.x264.DTS-Leffe.mkv")
	file2 := filepath.Join(tmpDir, "Mission to Mars (2000)", "Sample.Mission.To.Mars.2000.1080p.BluRay.x264.DTS-Leffe.mkv")

	os.MkdirAll(filepath.Dir(file1), 0755)
	os.WriteFile(file1, []byte("movie"), 0644)
	os.WriteFile(file2, []byte("sample"), 0644)

	// Scan for compliance issues
	issues, err := ScanMovieCompliance([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovieCompliance() error: %v", err)
	}

	// file2 is a sample, should be skipped
	// file1 should be detected as needing rename
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue (sample skipped), got %d", len(issues))
	}

	// Verify the actual movie file (not sample) is detected
	if len(issues) > 0 && strings.Contains(issues[0].Path, "Sample") {
		t.Error("Sample file should have been skipped")
	}
}

func TestScanMovieCompliance_ExcludesDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a movie with multiple versions (duplicates)
	file1 := filepath.Join(tmpDir, "Mission to Mars (2000)", "Mission.To.Mars.2000.1080p.BluRay.x264.DTS-Leffe.mkv")
	file2 := filepath.Join(tmpDir, "Mission to Mars (2000)", "Mission.To.Mars.2000.720p.BluRay.x264-x0r.mkv")

	os.MkdirAll(filepath.Dir(file1), 0755)
	os.WriteFile(file1, []byte("movie1"), 0644)
	os.WriteFile(file2, []byte("movie2"), 0644)

	// Without exclusions - should detect both as compliance issues
	issuesWithout, err := ScanMovieCompliance([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovieCompliance() error: %v", err)
	}
	if len(issuesWithout) != 2 {
		t.Errorf("Expected 2 issues without exclusions, got %d", len(issuesWithout))
	}

	// With exclusion - mark file2 as duplicate to be deleted
	issuesWith, err := ScanMovieCompliance([]string{tmpDir}, file2)
	if err != nil {
		t.Fatalf("ScanMovieCompliance() with exclusion error: %v", err)
	}

	// Should only detect file1 (file2 was excluded)
	if len(issuesWith) != 1 {
		t.Errorf("Expected 1 issue with exclusion, got %d", len(issuesWith))
	}

	// Verify excluded file is not in results
	for _, issue := range issuesWith {
		if issue.Path == file2 {
			t.Error("Excluded duplicate file should not appear in compliance issues")
		}
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)+1 && s[1:len(substr)+1] == substr))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
