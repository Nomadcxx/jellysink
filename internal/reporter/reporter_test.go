package reporter

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 1024, "1.00 MB"},
		{5 * 1024 * 1024 * 1024, "5.00 GB"},
		{2 * 1024 * 1024 * 1024 * 1024, "2.00 TB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
		}
	}
}

func TestGetTopOffenders(t *testing.T) {
	report := Report{
		MovieDuplicates: []scanner.MovieDuplicate{
			{
				NormalizedName: "big movie",
				Year:           "2024",
				Files: []scanner.MovieFile{
					{Path: "/keep.mkv", Size: 5 * 1024 * 1024 * 1024},
					{Path: "/delete1.mkv", Size: 4 * 1024 * 1024 * 1024},
					{Path: "/delete2.mkv", Size: 3 * 1024 * 1024 * 1024},
				},
			},
			{
				NormalizedName: "small movie",
				Year:           "2024",
				Files: []scanner.MovieFile{
					{Path: "/keep.mkv", Size: 1 * 1024 * 1024 * 1024},
					{Path: "/delete.mkv", Size: 500 * 1024 * 1024},
				},
			},
		},
	}

	offenders := GetTopOffenders(report)

	// Should have 2 offenders
	if len(offenders) != 2 {
		t.Errorf("Expected 2 offenders, got %d", len(offenders))
	}

	// First should be big movie (more space)
	if offenders[0].Name != "big movie (2024)" {
		t.Errorf("Expected first offender to be 'big movie (2024)', got %q", offenders[0].Name)
	}

	// Check space calculation
	expectedSpace := int64(7 * 1024 * 1024 * 1024) // 4GB + 3GB
	if offenders[0].SpaceToFree != expectedSpace {
		t.Errorf("Expected %d bytes to free, got %d", expectedSpace, offenders[0].SpaceToFree)
	}
}

func TestBuildReportContent(t *testing.T) {
	report := Report{
		Timestamp:    time.Date(2025, 1, 20, 14, 30, 0, 0, time.UTC),
		LibraryType:  "movies",
		LibraryPaths: []string{"/mnt/STORAGE1/MOVIES", "/mnt/STORAGE5/MOVIES"},
		MovieDuplicates: []scanner.MovieDuplicate{
			{
				NormalizedName: "test movie",
				Year:           "2024",
				Files: []scanner.MovieFile{
					{Path: "/keep/test.mkv", Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p"},
					{Path: "/delete/test.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "720p"},
				},
			},
		},
		ComplianceIssues: []scanner.ComplianceIssue{
			{
				Path:            "/bad/Movie.2024.1080p/movie.mkv",
				Type:            "movie",
				Problem:         "Release group folder naming",
				SuggestedPath:   "/good/Movie (2024)/Movie (2024).mkv",
				SuggestedAction: "reorganize",
			},
		},
		TotalDuplicates:    1,
		TotalFilesToDelete: 1,
		SpaceToFree:        2 * 1024 * 1024 * 1024,
	}

	content := buildReportContent(report)

	// Check header
	if !strings.Contains(content, "JELLYSINK SCAN REPORT") {
		t.Error("Report missing header")
	}

	// Check timestamp
	if !strings.Contains(content, "2025-01-20 14:30:00") {
		t.Error("Report missing correct timestamp")
	}

	// Check summary
	if !strings.Contains(content, "Duplicate groups found: 1") {
		t.Error("Report missing duplicate count")
	}

	if !strings.Contains(content, "Space to free: 2.00 GB") {
		t.Error("Report missing space calculation")
	}

	// Check compliance section
	if !strings.Contains(content, "COMPLIANCE ISSUES") {
		t.Error("Report missing compliance section")
	}

	if !strings.Contains(content, "Release group folder naming") {
		t.Error("Report missing compliance issue")
	}

	// Check deletion list
	if !strings.Contains(content, "DELETION LIST") {
		t.Error("Report missing deletion list")
	}

	if !strings.Contains(content, "/delete/test.mkv") {
		t.Error("Report missing file in deletion list")
	}

	// Check that keeper is NOT in deletion list
	if strings.Contains(strings.Split(content, "DELETION LIST")[1], "/keep/test.mkv") {
		t.Error("Keeper file should not be in deletion list")
	}
}

func TestGenerate(t *testing.T) {
	// Create test report
	report := Report{
		Timestamp:          time.Now(),
		LibraryType:        "movies",
		LibraryPaths:       []string{"/test/path"},
		TotalDuplicates:    1,
		TotalFilesToDelete: 1,
		SpaceToFree:        1024 * 1024 * 1024,
	}

	// Generate report
	filename, err := Generate(report)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Report file not created: %s", filename)
	}

	// Read and verify content
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}

	if !strings.Contains(string(content), "JELLYSINK SCAN REPORT") {
		t.Error("Report file missing header")
	}

	// Cleanup
	os.Remove(filename)
}

func TestFormatMovieDuplicate(t *testing.T) {
	dup := scanner.MovieDuplicate{
		NormalizedName: "test movie",
		Year:           "2024",
		Files: []scanner.MovieFile{
			{Path: "/keep/movie.mkv", Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p"},
			{Path: "/delete/movie.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "720p"},
		},
	}

	result := formatMovieDuplicate(dup)

	// Check title
	if !strings.Contains(result, "test movie (2024)") {
		t.Error("Formatted output missing title")
	}

	// Check KEEP marker
	if !strings.Contains(result, "KEEP:") {
		t.Error("Formatted output missing KEEP marker")
	}

	// Check DELETE marker
	if !strings.Contains(result, "DELETE:") {
		t.Error("Formatted output missing DELETE marker")
	}

	// Check paths
	if !strings.Contains(result, "/keep/movie.mkv") {
		t.Error("Formatted output missing keeper path")
	}

	if !strings.Contains(result, "/delete/movie.mkv") {
		t.Error("Formatted output missing delete path")
	}
}

func TestFormatTVDuplicate(t *testing.T) {
	dup := scanner.TVDuplicate{
		ShowName: "test show",
		Season:   1,
		Episode:  5,
		Files: []scanner.TVFile{
			{Path: "/keep/s01e05.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "WEB-DL"},
			{Path: "/delete/s01e05.mkv", Size: 1 * 1024 * 1024 * 1024, Resolution: "720p", Source: "HDTV"},
		},
	}

	result := formatTVDuplicate(dup)

	// Check episode identifier
	if !strings.Contains(result, "test show S01E05") {
		t.Error("Formatted output missing episode identifier")
	}

	// Check source info
	if !strings.Contains(result, "WEB-DL") {
		t.Error("Formatted output missing source")
	}

	// Check paths
	if !strings.Contains(result, "/keep/s01e05.mkv") {
		t.Error("Formatted output missing keeper path")
	}
}
