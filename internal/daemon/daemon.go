package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// Daemon represents the background service
type Daemon struct {
	config *config.Config
}

// New creates a new daemon instance
func New(cfg *config.Config) *Daemon {
	return &Daemon{
		config: cfg,
	}
}

// RunScan executes a full scan and generates a report
// Supports context cancellation for graceful shutdown
func (d *Daemon) RunScan(ctx context.Context) (string, error) {
	report := reporter.Report{
		Timestamp:    time.Now(),
		LibraryPaths: []string{},
	}

	// Use parallel scanning for better performance
	parallelConfig := scanner.DefaultParallelConfig()

	// Scan movies if configured
	if len(d.config.Libraries.Movies.Paths) > 0 {
		report.LibraryType = "movies"
		report.LibraryPaths = d.config.Libraries.Movies.Paths

		movieDuplicates, err := scanner.ScanMoviesParallel(ctx, d.config.Libraries.Movies.Paths, parallelConfig)
		if err != nil {
			return "", fmt.Errorf("failed to scan movies: %w", err)
		}
		report.MovieDuplicates = movieDuplicates

		// Scan for compliance issues
		complianceIssues, err := scanner.ScanMovieCompliance(d.config.Libraries.Movies.Paths)
		if err != nil {
			return "", fmt.Errorf("failed to scan movie compliance: %w", err)
		}
		report.ComplianceIssues = complianceIssues
	}

	// Scan TV shows if configured
	if len(d.config.Libraries.TV.Paths) > 0 {
		if report.LibraryType == "" {
			report.LibraryType = "tv"
			report.LibraryPaths = d.config.Libraries.TV.Paths
		} else {
			report.LibraryType = "mixed"
			report.LibraryPaths = append(report.LibraryPaths, d.config.Libraries.TV.Paths...)
		}

		tvDuplicates, err := scanner.ScanTVShowsParallel(ctx, d.config.Libraries.TV.Paths, parallelConfig)
		if err != nil {
			return "", fmt.Errorf("failed to scan TV shows: %w", err)
		}
		report.TVDuplicates = tvDuplicates

		// Scan for TV compliance issues
		tvComplianceIssues, err := scanner.ScanTVCompliance(d.config.Libraries.TV.Paths)
		if err != nil {
			return "", fmt.Errorf("failed to scan TV compliance: %w", err)
		}
		report.ComplianceIssues = append(report.ComplianceIssues, tvComplianceIssues...)
	}

	// Calculate statistics
	report.TotalDuplicates = len(report.MovieDuplicates) + len(report.TVDuplicates)

	for _, dup := range report.MovieDuplicates {
		report.TotalFilesToDelete += len(dup.Files) - 1 // Don't count keeper
		for i := 1; i < len(dup.Files); i++ {
			report.SpaceToFree += dup.Files[i].Size
		}
	}

	for _, dup := range report.TVDuplicates {
		report.TotalFilesToDelete += len(dup.Files) - 1
		for i := 1; i < len(dup.Files); i++ {
			report.SpaceToFree += dup.Files[i].Size
		}
	}

	// Save report
	reportPath, err := d.saveReport(report)
	if err != nil {
		return "", fmt.Errorf("failed to save report: %w", err)
	}

	return reportPath, nil
}

// saveReport saves the report as JSON
func (d *Daemon) saveReport(report reporter.Report) (string, error) {
	// Get report directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	reportDir := filepath.Join(home, ".local/share/jellysink/scan_results")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create report directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := report.Timestamp.Format("20060102_150405")
	reportPath := filepath.Join(reportDir, timestamp+".json")

	// Marshal to JSON
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}

	// Write to file
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	// Also generate text reports for reference
	_, err = reporter.GenerateDetailed(report)
	if err != nil {
		// Non-fatal - log but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to generate text reports: %v\n", err)
	}

	return reportPath, nil
}

// GetReportDir returns the directory where reports are stored
func GetReportDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/jellysink/scan_results"
	}
	return filepath.Join(home, ".local/share/jellysink/scan_results")
}
