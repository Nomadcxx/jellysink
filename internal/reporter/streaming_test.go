package reporter

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func TestStreamingReporterBasic(t *testing.T) {
	sr, err := NewStreamingReporter("movies", []string{"/test/path"})
	if err != nil {
		t.Fatalf("Failed to create streaming reporter: %v", err)
	}
	defer sr.Close()

	// Write a movie duplicate
	dup := scanner.MovieDuplicate{
		NormalizedName: "test movie",
		Year:           "2020",
		Files: []scanner.MovieFile{
			{Path: "/test/movie1.mkv", Size: 1000000, Resolution: "1080p"},
			{Path: "/test/movie2.mkv", Size: 500000, Resolution: "720p"},
		},
	}

	ctx := context.Background()
	if err := sr.WriteMovieDuplicate(ctx, dup); err != nil {
		t.Fatalf("Failed to write movie duplicate: %v", err)
	}

	if err := sr.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}

	// Verify statistics
	if sr.totalDups != 1 {
		t.Errorf("Expected 1 duplicate, got %d", sr.totalDups)
	}
	if sr.totalFiles != 1 {
		t.Errorf("Expected 1 file to delete, got %d", sr.totalFiles)
	}
	if sr.totalSpace != 500000 {
		t.Errorf("Expected 500000 bytes to free, got %d", sr.totalSpace)
	}
}

func TestStreamingReporterTV(t *testing.T) {
	sr, err := NewStreamingReporter("tv", []string{"/test/tv"})
	if err != nil {
		t.Fatalf("Failed to create streaming reporter: %v", err)
	}
	defer sr.Close()

	// Write a TV duplicate
	dup := scanner.TVDuplicate{
		ShowName: "test show",
		Season:   1,
		Episode:  1,
		Files: []scanner.TVFile{
			{Path: "/test/s01e01-1080p.mkv", Size: 2000000, Resolution: "1080p", Source: "BluRay"},
			{Path: "/test/s01e01-720p.mkv", Size: 1000000, Resolution: "720p", Source: "WEB-DL"},
		},
	}

	ctx := context.Background()
	if err := sr.WriteTVDuplicate(ctx, dup); err != nil {
		t.Fatalf("Failed to write TV duplicate: %v", err)
	}

	if err := sr.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}

	// Verify statistics
	if sr.totalDups != 1 {
		t.Errorf("Expected 1 duplicate, got %d", sr.totalDups)
	}
	if sr.totalSpace != 1000000 {
		t.Errorf("Expected 1000000 bytes to free, got %d", sr.totalSpace)
	}
}

func TestStreamingReporterMultipleDuplicates(t *testing.T) {
	sr, err := NewStreamingReporter("movies", []string{"/test/path"})
	if err != nil {
		t.Fatalf("Failed to create streaming reporter: %v", err)
	}
	defer sr.Close()

	ctx := context.Background()

	// Write multiple duplicates
	for i := 0; i < 10; i++ {
		dup := scanner.MovieDuplicate{
			NormalizedName: "test movie",
			Year:           "2020",
			Files: []scanner.MovieFile{
				{Path: "/test/movie1.mkv", Size: 1000000, Resolution: "1080p"},
				{Path: "/test/movie2.mkv", Size: 500000, Resolution: "720p"},
			},
		}
		if err := sr.WriteMovieDuplicate(ctx, dup); err != nil {
			t.Fatalf("Failed to write movie duplicate: %v", err)
		}
	}

	if err := sr.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}

	// Verify statistics
	if sr.totalDups != 10 {
		t.Errorf("Expected 10 duplicates, got %d", sr.totalDups)
	}
	if sr.totalFiles != 10 {
		t.Errorf("Expected 10 files to delete, got %d", sr.totalFiles)
	}
	if sr.totalSpace != 5000000 {
		t.Errorf("Expected 5000000 bytes to free, got %d", sr.totalSpace)
	}
}

func TestStreamingReporterCancellation(t *testing.T) {
	sr, err := NewStreamingReporter("movies", []string{"/test/path"})
	if err != nil {
		t.Fatalf("Failed to create streaming reporter: %v", err)
	}
	defer sr.Close()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Write should fail with context.Canceled
	dup := scanner.MovieDuplicate{
		NormalizedName: "test movie",
		Year:           "2020",
		Files: []scanner.MovieFile{
			{Path: "/test/movie1.mkv", Size: 1000000, Resolution: "1080p"},
			{Path: "/test/movie2.mkv", Size: 500000, Resolution: "720p"},
		},
	}

	err = sr.WriteMovieDuplicate(ctx, dup)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestStreamingReporterFileContent(t *testing.T) {
	sr, err := NewStreamingReporter("movies", []string{"/test/path"})
	if err != nil {
		t.Fatalf("Failed to create streaming reporter: %v", err)
	}
	defer sr.Close()

	dup := scanner.MovieDuplicate{
		NormalizedName: "test movie",
		Year:           "2020",
		Files: []scanner.MovieFile{
			{Path: "/test/movie1.mkv", Size: 1000000, Resolution: "1080p"},
			{Path: "/test/movie2.mkv", Size: 500000, Resolution: "720p"},
		},
	}

	ctx := context.Background()
	if err := sr.WriteMovieDuplicate(ctx, dup); err != nil {
		t.Fatalf("Failed to write movie duplicate: %v", err)
	}

	if err := sr.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}

	// Close files to flush content
	if err := sr.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Read and verify detail file content
	_, detailPath := sr.GetPaths()
	content, err := os.ReadFile(detailPath)
	if err != nil {
		t.Fatalf("Failed to read detail file: %v", err)
	}

	contentStr := string(content)

	// Check for expected content
	expectedStrings := []string{
		"test movie",
		"2020",
		"[KEEP]",
		"[DELETE]",
		"1080p",
		"720p",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("Expected content to contain %q, but it didn't", expected)
		}
	}

	// Clean up test files
	os.Remove(detailPath)
	summaryPath, _ := sr.GetPaths()
	os.Remove(summaryPath)
}

func TestStreamingReporterSkipsSingleFiles(t *testing.T) {
	sr, err := NewStreamingReporter("movies", []string{"/test/path"})
	if err != nil {
		t.Fatalf("Failed to create streaming reporter: %v", err)
	}
	defer sr.Close()

	// Write duplicate with only one file (should be skipped)
	dup := scanner.MovieDuplicate{
		NormalizedName: "single file",
		Year:           "2020",
		Files: []scanner.MovieFile{
			{Path: "/test/movie1.mkv", Size: 1000000, Resolution: "1080p"},
		},
	}

	ctx := context.Background()
	if err := sr.WriteMovieDuplicate(ctx, dup); err != nil {
		t.Fatalf("Failed to write movie duplicate: %v", err)
	}

	// Should not count single file as duplicate
	if sr.totalDups != 0 {
		t.Errorf("Expected 0 duplicates, got %d", sr.totalDups)
	}
}
