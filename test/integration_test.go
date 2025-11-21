package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nomadcxx/jellysink/internal/cleaner"
	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// TestFullScanWorkflow tests the complete scan workflow from start to finish
func TestFullScanWorkflow(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()

	// Create test movie structure with duplicates
	movieLib := filepath.Join(tmpDir, "movies")
	createTestMovieLibrary(t, movieLib)

	// Create config
	cfg := &config.Config{
		Libraries: config.LibraryConfig{
			Movies: config.MovieLibrary{
				Paths: []string{movieLib},
			},
		},
	}

	// Create daemon and run scan
	d := daemon.New(cfg)
	ctx := context.Background()

	reportPath, err := d.RunScan(ctx)
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	// Verify report was created
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Errorf("Report file not created: %s", reportPath)
	}

	// Load and verify report
	report := loadReport(t, reportPath)

	if report.TotalDuplicates < 1 {
		t.Errorf("Expected at least 1 duplicate group, got %d", report.TotalDuplicates)
	}

	if report.SpaceToFree <= 0 {
		t.Errorf("Expected positive space to free, got %d", report.SpaceToFree)
	}
}

// TestParallelScanPerformance tests that parallel scanning works correctly
func TestParallelScanPerformance(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple library paths
	movieLib1 := filepath.Join(tmpDir, "movies1")
	movieLib2 := filepath.Join(tmpDir, "movies2")
	createTestMovieLibrary(t, movieLib1)
	createTestMovieLibrary(t, movieLib2)

	ctx := context.Background()
	parallelConfig := scanner.DefaultParallelConfig()

	// Scan both libraries in parallel
	duplicates, err := scanner.ScanMoviesParallel(ctx, []string{movieLib1, movieLib2}, parallelConfig)
	if err != nil {
		t.Fatalf("Parallel scan failed: %v", err)
	}

	// Should find duplicates from both libraries
	if len(duplicates) < 2 {
		t.Errorf("Expected at least 2 duplicate groups from 2 libraries, got %d", len(duplicates))
	}
}

// TestContextCancellation tests that scans can be cancelled gracefully
func TestContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a large test library
	movieLib := filepath.Join(tmpDir, "movies")
	for i := 0; i < 1000; i++ {
		createTestMovie(t, movieLib, "Movie", 1, "2020")
	}

	// Create context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before scan starts

	parallelConfig := scanner.DefaultParallelConfig()

	// This should return context.Canceled
	_, err := scanner.ScanMoviesParallel(ctx, []string{movieLib}, parallelConfig)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

// TestCleanWorkflow tests the full cleaning workflow
func TestCleanWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test library with duplicates
	movieLib := filepath.Join(tmpDir, "movies")
	createTestMovieLibrary(t, movieLib)

	// Scan for duplicates
	duplicates, err := scanner.ScanMovies([]string{movieLib})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Mark which files to keep/delete
	duplicates = scanner.MarkKeepDelete(duplicates)

	// Configure cleaner for dry run
	cleanConfig := cleaner.DefaultConfig()
	cleanConfig.DryRun = true
	cleanConfig.ProtectedPaths = []string{} // No protection for test

	// Run clean operation (dry run)
	result, err := cleaner.CleanDuplicatesOnly(duplicates, []scanner.TVDuplicate{}, cleanConfig)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify results
	if result.DuplicatesDeleted < 0 {
		t.Errorf("Invalid duplicates deleted count: %d", result.DuplicatesDeleted)
	}

	if result.SpaceFreed < 0 {
		t.Errorf("Invalid space freed: %d", result.SpaceFreed)
	}
}

// TestReportGeneration tests report generation
func TestReportGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test library
	movieLib := filepath.Join(tmpDir, "movies")
	createTestMovieLibrary(t, movieLib)

	// Scan for duplicates
	duplicates, err := scanner.ScanMovies([]string{movieLib})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Create report
	report := reporter.Report{
		Timestamp:       time.Now(),
		LibraryType:     "movies",
		LibraryPaths:    []string{movieLib},
		MovieDuplicates: duplicates,
		TotalDuplicates: len(duplicates),
	}

	// Calculate statistics
	for _, dup := range duplicates {
		report.TotalFilesToDelete += len(dup.Files) - 1
		for i := 1; i < len(dup.Files); i++ {
			report.SpaceToFree += dup.Files[i].Size
		}
	}

	// Generate report
	reportPath, err := reporter.Generate(report)
	if err != nil {
		t.Fatalf("Report generation failed: %v", err)
	}

	// Verify report exists and has content
	content, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}

	if len(content) == 0 {
		t.Error("Report file is empty")
	}

	// Clean up
	os.Remove(reportPath)
}

// TestStreamingReporterIntegration tests streaming reporter with real scan data
func TestStreamingReporterIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test library
	movieLib := filepath.Join(tmpDir, "movies")
	createTestMovieLibrary(t, movieLib)

	// Scan for duplicates
	duplicates, err := scanner.ScanMovies([]string{movieLib})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Create streaming reporter
	sr, err := reporter.NewStreamingReporter("movies", []string{movieLib})
	if err != nil {
		t.Fatalf("Failed to create streaming reporter: %v", err)
	}
	defer sr.Close()

	// Write duplicates
	ctx := context.Background()
	for _, dup := range duplicates {
		if err := sr.WriteMovieDuplicate(ctx, dup); err != nil {
			t.Fatalf("Failed to write duplicate: %v", err)
		}
	}

	// Finalize
	if err := sr.Finalize(); err != nil {
		t.Fatalf("Failed to finalize: %v", err)
	}

	// Verify files were created
	summaryPath, detailPath := sr.GetPaths()

	if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
		t.Errorf("Summary file not created")
	}

	if _, err := os.Stat(detailPath); os.IsNotExist(err) {
		t.Errorf("Detail file not created")
	}

	// Clean up
	os.Remove(summaryPath)
	os.Remove(detailPath)
}

// TestTVShowScanWorkflow tests TV show scanning
func TestTVShowScanWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test TV library
	tvLib := filepath.Join(tmpDir, "tv")
	createTestTVLibrary(t, tvLib)

	// Create config
	cfg := &config.Config{
		Libraries: config.LibraryConfig{
			TV: config.TVLibrary{
				Paths: []string{tvLib},
			},
		},
	}

	// Create daemon and run scan
	d := daemon.New(cfg)
	ctx := context.Background()

	reportPath, err := d.RunScan(ctx)
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	// Load and verify report
	report := loadReport(t, reportPath)

	if len(report.TVDuplicates) < 1 {
		t.Errorf("Expected at least 1 TV duplicate group, got %d", len(report.TVDuplicates))
	}
}

// Helper functions

func createTestMovieLibrary(t *testing.T, basePath string) {
	// Create duplicates (2 versions each)
	createTestMovie(t, basePath, "The Matrix", 2, "1999")
	createTestMovie(t, basePath, "Inception", 2, "2010")
}

func createTestMovie(t *testing.T, basePath, title string, dupCount int, year string) {
	movieDir := filepath.Join(basePath, title+" ("+year+")")
	if err := os.MkdirAll(movieDir, 0755); err != nil {
		t.Fatalf("Failed to create movie directory: %v", err)
	}

	resolutions := []string{"2160p", "1080p", "720p"}
	for i := 0; i < dupCount && i < len(resolutions); i++ {
		filename := title + "." + year + "." + resolutions[i] + ".mkv"
		filePath := filepath.Join(movieDir, filename)

		// Create file with some content (size varies by resolution)
		size := 1000000 * (len(resolutions) - i)
		content := make([]byte, size)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create movie file: %v", err)
		}
	}
}

func createTestTVLibrary(t *testing.T, basePath string) {
	showDir := filepath.Join(basePath, "Breaking Bad (2008)", "Season 01")
	if err := os.MkdirAll(showDir, 0755); err != nil {
		t.Fatalf("Failed to create TV directory: %v", err)
	}

	// Create duplicate episodes
	files := []string{
		"Breaking.Bad.S01E01.1080p.BluRay.mkv",
		"Breaking.Bad.S01E01.720p.WEB-DL.mkv",
		"Breaking.Bad.S01E02.1080p.BluRay.mkv",
	}

	for _, file := range files {
		filePath := filepath.Join(showDir, file)
		content := make([]byte, 500000)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create TV file: %v", err)
		}
	}
}

func loadReport(t *testing.T, path string) reporter.Report {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}

	var report reporter.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Failed to parse report: %v", err)
	}

	return report
}
