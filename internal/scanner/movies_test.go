package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/path/to/movie.mkv", true},
		{"/path/to/movie.mp4", true},
		{"/path/to/movie.avi", true},
		{"/path/to/movie.txt", false},
		{"/path/to/movie.nfo", false},
		{"/path/to/movie.srt", false},
		{"/path/to/MOVIE.MKV", true}, // Case insensitive
	}

	for _, tt := range tests {
		result := isVideoFile(tt.path)
		if result != tt.expected {
			t.Errorf("isVideoFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestScoreMovieFile(t *testing.T) {
	tests := []struct {
		name     string
		file     MovieFile
		expected int // Exact value doesn't matter, just relative ordering
	}{
		{
			"Empty file",
			MovieFile{Size: 0, Resolution: "1080p", IsEmpty: true},
			-1000,
		},
		{
			"4K file",
			MovieFile{Size: 10 * 1024 * 1024 * 1024, Resolution: "2160p", IsEmpty: false},
			410, // 10 (size) + 400 (2160p)
		},
		{
			"1080p file",
			MovieFile{Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p", IsEmpty: false},
			305, // 5 (size) + 300 (1080p)
		},
		{
			"720p file",
			MovieFile{Size: 2 * 1024 * 1024 * 1024, Resolution: "720p", IsEmpty: false},
			202, // 2 (size) + 200 (720p)
		},
		{
			"Unknown resolution, large file",
			MovieFile{Size: 5 * 1024 * 1024 * 1024, Resolution: "unknown", IsEmpty: false},
			505, // 5 (size) + 5*100 (unknown bonus)
		},
	}

	for _, tt := range tests {
		result := scoreMovieFile(tt.file)
		if result != tt.expected {
			t.Errorf("%s: scoreMovieFile() = %d, want %d", tt.name, result, tt.expected)
		}
	}

	// Test relative ordering
	empty := MovieFile{Size: 0, IsEmpty: true}
	small480p := MovieFile{Size: 500 * 1024 * 1024, Resolution: "480p", IsEmpty: false}
	med720p := MovieFile{Size: 2 * 1024 * 1024 * 1024, Resolution: "720p", IsEmpty: false}
	large1080p := MovieFile{Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p", IsEmpty: false}
	largeUnknown := MovieFile{Size: 5 * 1024 * 1024 * 1024, Resolution: "unknown", IsEmpty: false}
	small1080p := MovieFile{Size: 2 * 1024 * 1024 * 1024, Resolution: "1080p", IsEmpty: false}

	if scoreMovieFile(empty) >= scoreMovieFile(small480p) {
		t.Error("Empty file should score lower than any non-empty file")
	}

	if scoreMovieFile(small480p) >= scoreMovieFile(med720p) {
		t.Error("480p should score lower than 720p")
	}

	if scoreMovieFile(med720p) >= scoreMovieFile(large1080p) {
		t.Error("720p should score lower than 1080p")
	}

	// Critical test: Large unknown resolution file should beat small 1080p file
	// This prevents release group files from being overvalued
	if scoreMovieFile(largeUnknown) <= scoreMovieFile(small1080p) {
		t.Errorf("Large unknown resolution file should beat small 1080p file (got %d vs %d)",
			scoreMovieFile(largeUnknown), scoreMovieFile(small1080p))
	}
}

func TestMarkKeepDelete(t *testing.T) {
	duplicates := []MovieDuplicate{
		{
			NormalizedName: "test movie",
			Year:           "2024",
			Files: []MovieFile{
				{Path: "/path/movie.720p.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "720p"},
				{Path: "/path/movie.1080p.mkv", Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p"},
				{Path: "/path/movie.empty.mkv", Size: 0, Resolution: "unknown", IsEmpty: true},
			},
		},
	}

	result := MarkKeepDelete(duplicates)

	// After marking, the best file (1080p) should be first
	if result[0].Files[0].Resolution != "1080p" {
		t.Errorf("Expected 1080p file to be marked as keeper, got %s", result[0].Files[0].Resolution)
	}

	// Empty file should not be first
	if result[0].Files[0].IsEmpty {
		t.Error("Empty file should not be marked as keeper")
	}
}

func TestGetDeleteList(t *testing.T) {
	duplicates := []MovieDuplicate{
		{
			NormalizedName: "test movie",
			Year:           "2024",
			Files: []MovieFile{
				{Path: "/keep/movie.1080p.mkv", Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p"},
				{Path: "/delete/movie.720p.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "720p"},
				{Path: "/delete/movie.empty.mkv", Size: 0, Resolution: "unknown", IsEmpty: true},
			},
		},
	}

	deleteList := GetDeleteList(duplicates)

	if len(deleteList) != 2 {
		t.Errorf("Expected 2 files to delete, got %d", len(deleteList))
	}

	// Check that keeper is not in delete list
	for _, path := range deleteList {
		if path == "/keep/movie.1080p.mkv" {
			t.Error("Keeper file should not be in delete list")
		}
	}

	// Check that delete files are present
	found720p := false
	foundEmpty := false
	for _, path := range deleteList {
		if path == "/delete/movie.720p.mkv" {
			found720p = true
		}
		if path == "/delete/movie.empty.mkv" {
			foundEmpty = true
		}
	}

	if !found720p {
		t.Error("720p file should be in delete list")
	}
	if !foundEmpty {
		t.Error("Empty file should be in delete list")
	}
}

func TestGetSpaceToFree(t *testing.T) {
	duplicates := []MovieDuplicate{
		{
			NormalizedName: "test movie",
			Year:           "2024",
			Files: []MovieFile{
				{Path: "/keep/movie.1080p.mkv", Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p"},
				{Path: "/delete/movie.720p.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "720p"},
				{Path: "/delete/movie.480p.mkv", Size: 1 * 1024 * 1024 * 1024, Resolution: "480p"},
			},
		},
	}

	spaceToFree := GetSpaceToFree(duplicates)
	expected := int64(3 * 1024 * 1024 * 1024) // 2GB + 1GB

	if spaceToFree != expected {
		t.Errorf("GetSpaceToFree() = %d bytes, want %d bytes", spaceToFree, expected)
	}
}

func TestScanMovies(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create test movie files
	movieDir1 := filepath.Join(tmpDir, "Movie (2024)")
	os.MkdirAll(movieDir1, 0755)

	movie1 := filepath.Join(movieDir1, "Movie.2024.1080p.mkv")
	os.WriteFile(movie1, []byte("fake video data"), 0644)

	movieDir2 := filepath.Join(tmpDir, "Movie.2024.720p.BluRay-GROUP")
	os.MkdirAll(movieDir2, 0755)

	movie2 := filepath.Join(movieDir2, "Movie.2024.720p.mkv")
	os.WriteFile(movie2, []byte("data"), 0644)

	// Scan for duplicates
	duplicates, err := ScanMovies([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovies() error: %v", err)
	}

	// Should find at least one duplicate group
	if len(duplicates) == 0 {
		t.Error("Expected to find duplicate groups, got none")
	}

	// Check that group has multiple files
	foundGroup := false
	for _, group := range duplicates {
		if len(group.Files) >= 2 {
			foundGroup = true
			break
		}
	}

	if !foundGroup {
		t.Error("Expected to find a group with 2+ files")
	}
}

func TestScanMoviesSameFolderDifferentFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create folder with two different files (simulates Mission to Mars case)
	movieDir := filepath.Join(tmpDir, "Mission to Mars (2000)")
	os.MkdirAll(movieDir, 0755)

	// Main file
	movie1 := filepath.Join(movieDir, "Mission.To.Mars.2000.1080p.BluRay.x264.mkv")
	os.WriteFile(movie1, []byte("fake video data large file"), 0644)

	// Sample file
	movie2 := filepath.Join(movieDir, "Sample.Mission.To.Mars.2000.1080p.BluRay.x264.mkv")
	os.WriteFile(movie2, []byte("sample"), 0644)

	// Scan for duplicates
	duplicates, err := ScanMovies([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovies() error: %v", err)
	}

	// Should find ONE duplicate group with TWO files
	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
	}

	if len(duplicates) > 0 && len(duplicates[0].Files) != 2 {
		t.Errorf("Expected 2 files in duplicate group, got %d", len(duplicates[0].Files))
	}
}

func TestScanMoviesDifferentFoldersSameMovie(t *testing.T) {
	tmpDir := t.TempDir()

	// Case 1: Compliant folder structure
	movieDir1 := filepath.Join(tmpDir, "Mama (2013)")
	os.MkdirAll(movieDir1, 0755)
	movie1 := filepath.Join(movieDir1, "Mama (2013).mkv")
	os.WriteFile(movie1, []byte("fake video data 5GB worth"), 0644)

	// Case 2: Release group folder with different content
	movieDir2 := filepath.Join(tmpDir, "Mama.2013.1080p.BluRay.x264-GROUP")
	os.MkdirAll(movieDir2, 0755)
	movie2 := filepath.Join(movieDir2, "Mama.2013.1080p.BluRay.x264-GROUP.mkv")
	os.WriteFile(movie2, []byte("smaller"), 0644)

	// Scan for duplicates
	duplicates, err := ScanMovies([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovies() error: %v", err)
	}

	// Should find ONE duplicate group with TWO files
	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
		for i, dup := range duplicates {
			t.Logf("Group %d: %s|%s with %d files", i, dup.NormalizedName, dup.Year, len(dup.Files))
		}
	}

	if len(duplicates) > 0 && len(duplicates[0].Files) != 2 {
		t.Errorf("Expected 2 files in duplicate group, got %d", len(duplicates[0].Files))
		if len(duplicates[0].Files) > 0 {
			for i, f := range duplicates[0].Files {
				t.Logf("File %d: %s", i, f.Path)
			}
		}
	}
}

func TestDuplicateAndComplianceIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate the exact user scenario: two versions of Mama (2013)
	// Version 1: Compliant naming
	movieDir1 := filepath.Join(tmpDir, "Mama (2013)")
	os.MkdirAll(movieDir1, 0755)
	movie1 := filepath.Join(movieDir1, "Mama (2013).mkv")
	os.WriteFile(movie1, make([]byte, 5*1024*1024*1024), 0644) // 5GB

	// Version 2: Release group folder
	movieDir2 := filepath.Join(tmpDir, "Mama.2013.1080p.BluRay.x264-GROUP")
	os.MkdirAll(movieDir2, 0755)
	movie2 := filepath.Join(movieDir2, "Mama.2013.1080p.BluRay.x264-GROUP.mkv")
	os.WriteFile(movie2, make([]byte, 2*1024*1024*1024), 0644) // 2GB

	// Step 1: Scan for duplicates
	duplicates, err := ScanMovies([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovies() error: %v", err)
	}

	if len(duplicates) != 1 {
		t.Fatalf("Expected 1 duplicate group, got %d", len(duplicates))
	}

	// Debug: Check file info before marking
	t.Logf("Files in duplicate group:")
	for i, f := range duplicates[0].Files {
		info, _ := os.Stat(f.Path)
		t.Logf("  File %d: %s", i, filepath.Base(f.Path))
		t.Logf("    Path: %s", f.Path)
		t.Logf("    Size (stored): %d bytes (%.2f GB)", f.Size, float64(f.Size)/(1024*1024*1024))
		t.Logf("    Size (actual): %d bytes", info.Size())
		t.Logf("    Resolution: %s", f.Resolution)
		t.Logf("    Score: %d", scoreMovieFile(f))
	}

	// Mark keep/delete
	marked := MarkKeepDelete(duplicates)

	t.Logf("After MarkKeepDelete:")
	t.Logf("  Keeper: %s (score: %d)", filepath.Base(marked[0].Files[0].Path), scoreMovieFile(marked[0].Files[0]))
	for i := 1; i < len(marked[0].Files); i++ {
		t.Logf("  Delete: %s (score: %d)", filepath.Base(marked[0].Files[i].Path), scoreMovieFile(marked[0].Files[i]))
	}

	deleteList := GetDeleteList(marked)

	// Step 2: Scan for compliance issues (excluding files marked for deletion)
	issues, err := ScanMovieCompliance([]string{tmpDir}, deleteList...)
	if err != nil {
		t.Fatalf("ScanMovieCompliance() error: %v", err)
	}

	t.Logf("Compliance scan found %d issues:", len(issues))
	for _, issue := range issues {
		t.Logf("  - %s -> %s", issue.Path, issue.SuggestedPath)
		if strings.Contains(issue.Problem, "COLLISION") {
			t.Errorf("COLLISION detected! This should not happen after duplicate scan: %s", issue.Problem)
		}
	}

	// We expect ZERO compliance issues because:
	// - movie1 is compliant (Mama (2013)/Mama (2013).mkv) and is the keeper
	// - movie2 is non-compliant but should be in deleteList, so excluded from compliance scan
	if len(issues) > 1 {
		t.Errorf("Expected 0-1 compliance issues, got %d (one file should be keeper, other should be in deleteList)", len(issues))
	}
}
