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
	// Set default log level first
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)

	// Respect configured log level for the daemon
	if cfg != nil {
		if lvl, err := scanner.ParseLogLevel(cfg.Daemon.LogLevel); err == nil {
			scanner.SetDefaultLogLevel(lvl)
		}
	}

	return &Daemon{
		config: cfg,
	}
}

// RunScan executes a full scan and generates a report
// Supports context cancellation for graceful shutdown
func (d *Daemon) RunScan(ctx context.Context) (string, error) {
	return d.RunScanWithProgress(ctx, nil)
}

// RunScanWithProgress executes a full scan with progress reporting
func (d *Daemon) RunScanWithProgress(ctx context.Context, progressCh chan<- scanner.ScanProgress) (string, error) {
	// Use orchestrator for coordinated scanning with progress
	scanResult, err := scanner.RunFullScan(
		ctx,
		d.config.Libraries.Movies.Paths,
		d.config.Libraries.TV.Paths,
		progressCh,
	)
	if err != nil {
		return "", fmt.Errorf("scan failed: %w", err)
	}

	// Build report from scan result
	report := reporter.Report{
		Timestamp:          time.Now(),
		LibraryPaths:       []string{},
		MovieDuplicates:    scanResult.MovieDuplicates,
		TVDuplicates:       scanResult.TVDuplicates,
		ComplianceIssues:   scanResult.ComplianceIssues,
		AmbiguousTVShows:   scanResult.AmbiguousTVShows,
		TotalDuplicates:    scanResult.TotalDuplicates,
		TotalFilesToDelete: scanResult.TotalFilesToDelete,
		SpaceToFree:        scanResult.SpaceToFree,
	}

	// Set library type and paths
	if len(d.config.Libraries.Movies.Paths) > 0 {
		report.LibraryType = "movies"
		report.LibraryPaths = d.config.Libraries.Movies.Paths
	}
	if len(d.config.Libraries.TV.Paths) > 0 {
		if report.LibraryType == "" {
			report.LibraryType = "tv"
			report.LibraryPaths = d.config.Libraries.TV.Paths
		} else {
			report.LibraryType = "mixed"
			report.LibraryPaths = append(report.LibraryPaths, d.config.Libraries.TV.Paths...)
		}
	}

	// Save report with progress
	reportPath, err := d.saveReportWithProgress(report, progressCh)
	if err != nil {
		return "", fmt.Errorf("failed to save report: %w", err)
	}

	return reportPath, nil
}

// saveReport saves the report as JSON
func (d *Daemon) saveReport(report reporter.Report) (string, error) {
	return d.saveReportWithProgress(report, nil)
}

// saveReportWithProgress saves the report as JSON with progress reporting
func (d *Daemon) saveReportWithProgress(report reporter.Report, progressCh chan<- scanner.ScanProgress) (string, error) {
	var pr *scanner.ProgressReporter
	if progressCh != nil {
		pr = scanner.NewProgressReporter(progressCh, "report_generation")
		pr.Update(0, "Saving report")
	}

	// Get report directory
	home, err := os.UserHomeDir()
	if err != nil {
		if pr != nil {
			pr.LogError(err, "Failed to get home directory")
		}
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	reportDir := filepath.Join(home, ".local/share/jellysink/scan_results")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		if pr != nil {
			pr.LogError(err, "Failed to create report directory")
		}
		return "", fmt.Errorf("failed to create report directory: %w", err)
	}

	if pr != nil {
		pr.Update(25, "Formatting JSON report")
	}

	// Generate filename with timestamp
	timestamp := report.Timestamp.Format("20060102_150405")
	reportPath := filepath.Join(reportDir, timestamp+".json")

	// Marshal to JSON
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		if pr != nil {
			pr.LogError(err, "Failed to marshal report to JSON")
		}
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}

	if pr != nil {
		pr.Update(50, "Writing JSON report to disk")
	}

	// Write to file
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		if pr != nil {
			pr.LogError(err, "Failed to write JSON report")
		}
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	if pr != nil {
		pr.Update(75, "Generating text reports")
	}

	// Generate text reports with progress
	_, err = reporter.GenerateDetailedWithProgress(report, pr)
	if err != nil {
		// Non-fatal - log but continue
		if pr != nil {
			pr.LogError(err, "Failed to generate text reports")
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to generate text reports: %v\n", err)
	}

	if pr != nil {
		pr.Complete("Report saved successfully")
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
