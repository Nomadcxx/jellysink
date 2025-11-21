package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParallelConfig(t *testing.T) {
	config := DefaultParallelConfig()
	if config.Workers <= 0 {
		t.Errorf("DefaultParallelConfig() returned invalid workers: %d", config.Workers)
	}
}

func TestScanMoviesParallel(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()

	// Create test movie files
	movies := map[string][]string{
		"The Matrix (1999)": {
			"The.Matrix.1999.1080p.BluRay.mkv",
			"The.Matrix.1999.720p.WEB-DL.mkv",
		},
		"Inception (2010)": {
			"Inception.2010.2160p.UHD.mkv",
		},
	}

	for movieDir, files := range movies {
		moviePath := filepath.Join(tmpDir, movieDir)
		if err := os.MkdirAll(moviePath, 0755); err != nil {
			t.Fatalf("Failed to create movie directory: %v", err)
		}

		for _, file := range files {
			filePath := filepath.Join(moviePath, file)
			if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
		}
	}

	// Test parallel scanning
	config := ParallelConfig{Workers: 2}
	duplicates, err := ScanMoviesParallel(context.Background(), []string{tmpDir}, config)
	if err != nil {
		t.Fatalf("ScanMoviesParallel failed: %v", err)
	}

	// Should find 1 duplicate group (The Matrix with 2 versions)
	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
	}

	// Verify the duplicate group is The Matrix
	if len(duplicates) > 0 {
		if len(duplicates[0].Files) != 2 {
			t.Errorf("Expected 2 files in duplicate group, got %d", len(duplicates[0].Files))
		}
	}
}

func TestScanTVShowsParallel(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()

	// Create test TV show files
	shows := map[string]map[string][]string{
		"Breaking Bad (2008)": {
			"Season 01": {
				"Breaking.Bad.S01E01.1080p.BluRay.mkv",
				"Breaking.Bad.S01E01.720p.WEB-DL.mkv",
				"Breaking.Bad.S01E02.1080p.BluRay.mkv",
			},
		},
	}

	for showDir, seasons := range shows {
		for seasonDir, files := range seasons {
			seasonPath := filepath.Join(tmpDir, showDir, seasonDir)
			if err := os.MkdirAll(seasonPath, 0755); err != nil {
				t.Fatalf("Failed to create season directory: %v", err)
			}

			for _, file := range files {
				filePath := filepath.Join(seasonPath, file)
				if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}
		}
	}

	// Test parallel scanning
	config := ParallelConfig{Workers: 2}
	duplicates, err := ScanTVShowsParallel(context.Background(), []string{tmpDir}, config)
	if err != nil {
		t.Fatalf("ScanTVShowsParallel failed: %v", err)
	}

	// Should find 1 duplicate group (S01E01 with 2 versions)
	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
	}

	// Verify the duplicate group
	if len(duplicates) > 0 {
		if len(duplicates[0].Files) != 2 {
			t.Errorf("Expected 2 files in duplicate group, got %d", len(duplicates[0].Files))
		}
		if duplicates[0].Season != 1 || duplicates[0].Episode != 1 {
			t.Errorf("Expected S01E01, got S%02dE%02d", duplicates[0].Season, duplicates[0].Episode)
		}
	}
}

func TestParallelVsSequentialMovies(t *testing.T) {
	// Create temporary test directory with multiple movies
	tmpDir := t.TempDir()

	movies := map[string][]string{
		"Movie A (2020)": {"Movie.A.2020.1080p.mkv", "Movie.A.2020.720p.mkv"},
		"Movie B (2021)": {"Movie.B.2021.1080p.mkv"},
		"Movie C (2022)": {"Movie.C.2022.2160p.mkv", "Movie.C.2022.1080p.mkv", "Movie.C.2022.720p.mkv"},
	}

	for movieDir, files := range movies {
		moviePath := filepath.Join(tmpDir, movieDir)
		if err := os.MkdirAll(moviePath, 0755); err != nil {
			t.Fatalf("Failed to create movie directory: %v", err)
		}

		for _, file := range files {
			filePath := filepath.Join(moviePath, file)
			if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
		}
	}

	// Test both sequential and parallel
	sequential, err := ScanMovies([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovies failed: %v", err)
	}

	config := ParallelConfig{Workers: 4}
	parallel, err := ScanMoviesParallel(context.Background(), []string{tmpDir}, config)
	if err != nil {
		t.Fatalf("ScanMoviesParallel failed: %v", err)
	}

	// Both should find same number of duplicates
	if len(sequential) != len(parallel) {
		t.Errorf("Sequential found %d duplicates, parallel found %d", len(sequential), len(parallel))
	}

	// Should find 2 duplicate groups (Movie A with 2, Movie C with 3)
	if len(parallel) != 2 {
		t.Errorf("Expected 2 duplicate groups, got %d", len(parallel))
	}
}

func TestParallelVsSequentialTV(t *testing.T) {
	// Create temporary test directory with TV shows
	tmpDir := t.TempDir()

	shows := map[string]map[string][]string{
		"Show A (2020)": {
			"Season 01": {
				"Show.A.S01E01.1080p.mkv",
				"Show.A.S01E01.720p.mkv",
				"Show.A.S01E02.1080p.mkv",
			},
		},
	}

	for showDir, seasons := range shows {
		for seasonDir, files := range seasons {
			seasonPath := filepath.Join(tmpDir, showDir, seasonDir)
			if err := os.MkdirAll(seasonPath, 0755); err != nil {
				t.Fatalf("Failed to create season directory: %v", err)
			}

			for _, file := range files {
				filePath := filepath.Join(seasonPath, file)
				if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}
		}
	}

	// Test both sequential and parallel
	sequential, err := ScanTVShows([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanTVShows failed: %v", err)
	}

	config := ParallelConfig{Workers: 4}
	parallel, err := ScanTVShowsParallel(context.Background(), []string{tmpDir}, config)
	if err != nil {
		t.Fatalf("ScanTVShowsParallel failed: %v", err)
	}

	// Both should find same number of duplicates
	if len(sequential) != len(parallel) {
		t.Errorf("Sequential found %d duplicates, parallel found %d", len(sequential), len(parallel))
	}

	// Should find 1 duplicate group (S01E01 with 2 versions)
	if len(parallel) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(parallel))
	}
}

func TestParallelWithMultiplePaths(t *testing.T) {
	// Create multiple temporary directories
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	// Create movies in first directory
	moviePath1 := filepath.Join(tmpDir1, "Movie A (2020)")
	os.MkdirAll(moviePath1, 0755)
	os.WriteFile(filepath.Join(moviePath1, "Movie.A.2020.1080p.mkv"), []byte("test"), 0644)

	// Create movies in second directory
	moviePath2 := filepath.Join(tmpDir2, "Movie B (2021)")
	os.MkdirAll(moviePath2, 0755)
	os.WriteFile(filepath.Join(moviePath2, "Movie.B.2021.1080p.mkv"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(moviePath2, "Movie.B.2021.720p.mkv"), []byte("test"), 0644)

	// Scan both paths in parallel
	config := ParallelConfig{Workers: 2}
	duplicates, err := ScanMoviesParallel(context.Background(), []string{tmpDir1, tmpDir2}, config)
	if err != nil {
		t.Fatalf("ScanMoviesParallel failed: %v", err)
	}

	// Should find 1 duplicate group (Movie B with 2 versions)
	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
	}
}

func TestParallelWithInvalidPath(t *testing.T) {
	config := ParallelConfig{Workers: 2}

	// Test with non-existent path
	_, err := ScanMoviesParallel(context.Background(), []string{"/nonexistent/path"}, config)
	if err == nil {
		t.Error("Expected error for non-existent path, got nil")
	}

	_, err = ScanTVShowsParallel(context.Background(), []string{"/nonexistent/path"}, config)
	if err == nil {
		t.Error("Expected error for non-existent path, got nil")
	}
}

func TestParallelConfigWithZeroWorkers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test movie
	moviePath := filepath.Join(tmpDir, "Test Movie (2020)")
	os.MkdirAll(moviePath, 0755)
	os.WriteFile(filepath.Join(moviePath, "Test.Movie.2020.1080p.mkv"), []byte("test"), 0644)

	// Test with zero workers (should default to runtime.NumCPU())
	config := ParallelConfig{Workers: 0}
	_, err := ScanMoviesParallel(context.Background(), []string{tmpDir}, config)
	if err != nil {
		t.Fatalf("ScanMoviesParallel with zero workers failed: %v", err)
	}
}
