package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Nomadcxx/jellysink/internal/cleaner"
	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// Daemon represents the background service
type Daemon struct {
	config       *config.Config
	headlessMode bool
}

// New creates a new daemon instance
func New(cfg *config.Config) *Daemon {
	// Only apply config log level if DefaultLogLevel hasn't been changed by CLI
	// CLI flags take precedence over config file
	currentLogLevel := scanner.GetDefaultLogLevel()

	// If log level is still the default (Normal), apply config setting
	if currentLogLevel == scanner.LogLevelNormal && cfg != nil && cfg.Daemon.LogLevel != "" {
		if lvl, err := scanner.ParseLogLevel(cfg.Daemon.LogLevel); err == nil {
			scanner.SetDefaultLogLevel(lvl)
		}
	}

	return &Daemon{
		config:       cfg,
		headlessMode: detectHeadlessMode(),
	}
}

// detectHeadlessMode checks if running in a headless environment (no display available)
func detectHeadlessMode() bool {
	display := os.Getenv("DISPLAY")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	return display == "" && waylandDisplay == ""
}

// IsHeadless returns whether the daemon is running in headless mode
func (d *Daemon) IsHeadless() bool {
	return d.headlessMode
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

	// Get report directory - use real user's home when running as root
	home, err := getRealUserHome()
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

// getRealUserHome returns the real user's home directory
// When running as root via sudo, returns SUDO_USER's home
// Otherwise returns current user's home
func getRealUserHome() (string, error) {
	// Check if running as root via sudo
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		// Return the real user's home directory
		return filepath.Join("/home", sudoUser), nil
	}

	// Not running via sudo, use current user's home
	return os.UserHomeDir()
}

// GetReportDir returns the directory where reports are stored
func GetReportDir() string {
	home, err := getRealUserHome()
	if err != nil {
		return "/tmp/jellysink/scan_results"
	}
	return filepath.Join(home, ".local/share/jellysink/scan_results")
}

// CleanupOldReports removes reports older than 30 days
func CleanupOldReports() error {
	reportDir := GetReportDir()
	entries, err := os.ReadDir(reportDir)
	if err != nil {
		return fmt.Errorf("failed to read report directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -30)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(reportDir, entry.Name())
			if err := os.Remove(fullPath); err == nil {
				deleted++
			}
		}
	}

	if deleted > 0 {
		fmt.Printf("Cleaned up %d old reports (>30 days)\n", deleted)
	}

	return nil
}

// AutoClean performs automatic cleanup of duplicates and compliance issues
// Used in headless mode or when user enables auto-clean in config
func (d *Daemon) AutoClean(report reporter.Report) error {
	fmt.Println("Running auto-clean (headless mode)...")

	cleanerCfg := cleaner.DefaultConfig()
	cleanerCfg.DryRun = false

	result, err := cleaner.Clean(
		report.MovieDuplicates,
		report.TVDuplicates,
		report.ComplianceIssues,
		cleanerCfg,
	)

	if err != nil {
		return fmt.Errorf("auto-clean failed: %w", err)
	}

	fmt.Printf("Auto-clean complete:\n")
	fmt.Printf("  Duplicates deleted: %d\n", result.DuplicatesDeleted)
	fmt.Printf("  Compliance fixed: %d\n", result.ComplianceFixed)
	fmt.Printf("  Space freed: %.2f GB\n", float64(result.SpaceFreed)/(1024*1024*1024))

	if len(result.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "    - %v\n", err)
		}
	}

	return nil
}

// GenerateSystemdTimer creates systemd timer configuration based on scan frequency
func GenerateSystemdTimer(frequency string) (string, error) {
	var onCalendar string

	switch frequency {
	case "daily":
		onCalendar = "*-*-* 02:00:00"
	case "weekly":
		onCalendar = "Sun *-*-* 02:00:00"
	case "biweekly":
		onCalendar = "Sun/2 *-*-* 02:00:00"
	default:
		return "", fmt.Errorf("invalid scan frequency: %s (must be daily, weekly, or biweekly)", frequency)
	}

	timer := fmt.Sprintf(`[Unit]
Description=Jellysink media library scan timer
Requires=jellysink.service

[Timer]
OnCalendar=%s
Persistent=true

[Install]
WantedBy=timers.target
`, onCalendar)

	return timer, nil
}

// InstallSystemdTimer writes the systemd timer file
func InstallSystemdTimer(frequency string) error {
	timerContent, err := GenerateSystemdTimer(frequency)
	if err != nil {
		return err
	}

	timerPath := "/etc/systemd/system/jellysink.timer"

	if err := os.WriteFile(timerPath, []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("failed to write timer file: %w", err)
	}

	fmt.Printf("Systemd timer installed at %s\n", timerPath)
	fmt.Println("Run 'sudo systemctl daemon-reload && sudo systemctl enable --now jellysink.timer' to activate")

	return nil
}
