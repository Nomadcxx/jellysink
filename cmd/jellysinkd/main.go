package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

const version = "0.1.0-dev"

var (
	logFile *os.File
	logger  *log.Logger
)

func main() {
	// Setup logging
	if err := setupLogging(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger.Printf("jellysinkd v%s starting...", version)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Configuration validation failed: %v", err)
	}

	logger.Printf("Configuration loaded successfully")
	logger.Printf("Scan frequency: %s", cfg.Daemon.ScanFrequency)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Setup ticker for periodic scans
	ticker := createTicker(cfg.Daemon.ScanFrequency)
	defer ticker.Stop()

	logger.Printf("Daemon initialized. Waiting for scheduled scans...")

	// Run initial scan on startup (after 1 minute to avoid boot load)
	time.AfterFunc(1*time.Minute, func() {
		logger.Printf("Running initial scan after startup delay...")
		performScan(cfg)
	})

	// Main loop
	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				// Reload configuration
				logger.Printf("Received SIGHUP, reloading configuration...")
				newCfg, err := config.Load()
				if err != nil {
					logger.Printf("Failed to reload config: %v", err)
					continue
				}
				if err := newCfg.Validate(); err != nil {
					logger.Printf("New configuration invalid: %v", err)
					continue
				}
				cfg = newCfg

				// Recreate ticker with new frequency
				ticker.Stop()
				ticker = createTicker(cfg.Daemon.ScanFrequency)
				logger.Printf("Configuration reloaded. New scan frequency: %s", cfg.Daemon.ScanFrequency)

			case syscall.SIGINT, syscall.SIGTERM:
				logger.Printf("Received shutdown signal (%s), exiting gracefully...", sig)
				return
			}

		case <-ticker.C:
			logger.Printf("Starting scheduled scan...")
			performScan(cfg)
		}
	}
}

func setupLogging() error {
	// Create log directory
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir := filepath.Join(home, ".local/share/jellysink")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Open log file
	logPath := filepath.Join(logDir, "daemon.log")
	logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Create logger
	logger = log.New(logFile, "", log.LstdFlags)

	return nil
}

func createTicker(frequency string) *time.Ticker {
	var duration time.Duration

	switch frequency {
	case "daily":
		duration = 24 * time.Hour
	case "weekly":
		duration = 7 * 24 * time.Hour
	case "biweekly":
		duration = 14 * 24 * time.Hour
	default:
		logger.Printf("Unknown scan frequency '%s', defaulting to weekly", frequency)
		duration = 7 * 24 * time.Hour
	}

	return time.NewTicker(duration)
}

func performScan(cfg *config.Config) {
	startTime := time.Now()
	logger.Printf("=== Scan started at %s ===", startTime.Format("2006-01-02 15:04:05"))

	report := reporter.Report{
		Timestamp:    startTime,
		LibraryPaths: cfg.GetAllPaths(),
	}

	// Scan movies
	if len(cfg.Libraries.Movies.Paths) > 0 {
		logger.Printf("Scanning movie libraries...")
		movieDups, err := scanner.ScanMovies(cfg.Libraries.Movies.Paths)
		if err != nil {
			logger.Printf("Error scanning movies: %v", err)
		} else {
			report.MovieDuplicates = scanner.MarkKeepDelete(movieDups)
			logger.Printf("Found %d duplicate movie groups", len(report.MovieDuplicates))
		}

		logger.Printf("Checking movie naming compliance...")
		movieCompliance, err := scanner.ScanMovieCompliance(cfg.Libraries.Movies.Paths)
		if err != nil {
			logger.Printf("Error checking movie compliance: %v", err)
		} else {
			report.ComplianceIssues = append(report.ComplianceIssues, movieCompliance...)
			logger.Printf("Found %d movie compliance issues", len(movieCompliance))
		}
	}

	// Scan TV shows
	if len(cfg.Libraries.TV.Paths) > 0 {
		logger.Printf("Scanning TV show libraries...")
		tvDups, err := scanner.ScanTVShows(cfg.Libraries.TV.Paths)
		if err != nil {
			logger.Printf("Error scanning TV shows: %v", err)
		} else {
			report.TVDuplicates = scanner.MarkKeepDeleteTV(tvDups)
			logger.Printf("Found %d duplicate TV episode groups", len(report.TVDuplicates))
		}

		logger.Printf("Checking TV show naming compliance...")
		tvCompliance, err := scanner.ScanTVCompliance(cfg.Libraries.TV.Paths)
		if err != nil {
			logger.Printf("Error checking TV compliance: %v", err)
		} else {
			report.ComplianceIssues = append(report.ComplianceIssues, tvCompliance...)
			logger.Printf("Found %d TV show compliance issues", len(tvCompliance))
		}
	}

	// Calculate statistics
	report.TotalDuplicates = len(report.MovieDuplicates) + len(report.TVDuplicates)
	report.TotalFilesToDelete = countFilesToDelete(report)
	report.SpaceToFree = calculateSpaceToFree(report)

	// Set library type
	if len(cfg.Libraries.Movies.Paths) > 0 && len(cfg.Libraries.TV.Paths) > 0 {
		report.LibraryType = "mixed"
	} else if len(cfg.Libraries.Movies.Paths) > 0 {
		report.LibraryType = "movies"
	} else {
		report.LibraryType = "tv"
	}

	// Generate reports
	logger.Printf("Generating reports...")
	files, err := reporter.GenerateDetailed(report)
	if err != nil {
		logger.Printf("Error generating reports: %v", err)
	} else {
		logger.Printf("Reports saved:")
		logger.Printf("  Summary:     %s", files.Summary)
		logger.Printf("  Duplicates:  %s", files.Duplicates)
		logger.Printf("  Compliance:  %s", files.Compliance)
	}

	// Log summary
	elapsed := time.Since(startTime)
	logger.Printf("=== Scan completed in %v ===", elapsed.Round(time.Second))
	logger.Printf("Duplicate groups:  %d", report.TotalDuplicates)
	logger.Printf("Files to delete:   %d", report.TotalFilesToDelete)
	logger.Printf("Space to free:     %s", formatBytes(report.SpaceToFree))
	logger.Printf("Compliance issues: %d", len(report.ComplianceIssues))

	// If configured, trigger TUI report display
	if cfg.Daemon.ReportOnComplete {
		logger.Printf("ReportOnComplete is enabled (manual launch required)")
		// Note: Actual TUI launch would require user session context
		// This is a placeholder for future implementation
	}
}

func countFilesToDelete(report reporter.Report) int {
	count := 0
	for _, dup := range report.MovieDuplicates {
		count += len(dup.Files) - 1
	}
	for _, dup := range report.TVDuplicates {
		count += len(dup.Files) - 1
	}
	return count
}

func calculateSpaceToFree(report reporter.Report) int64 {
	var total int64
	for _, dup := range report.MovieDuplicates {
		for i := 1; i < len(dup.Files); i++ {
			total += dup.Files[i].Size
		}
	}
	for _, dup := range report.TVDuplicates {
		for i := 1; i < len(dup.Files); i++ {
			total += dup.Files[i].Size
		}
	}
	return total
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
