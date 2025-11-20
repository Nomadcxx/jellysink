package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
	"github.com/Nomadcxx/jellysink/internal/reporter"
)

func main() {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Fprintf(os.Stderr, "Create config at ~/.config/jellysink/config.toml\n")
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Create daemon instance
	d := daemon.New(cfg)

	// Run scan
	fmt.Println("jellysinkd: Starting scheduled scan...")
	reportPath, err := d.RunScan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan failed: %v\n", err)
		os.Exit(1)
	}

	// Load report to get statistics
	report, err := loadReport(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scan complete! Found %d duplicate groups\n", report.TotalDuplicates)
	fmt.Printf("Report saved to: %s\n", reportPath)

	// Send notification
	if err := daemon.NotifyUser(reportPath, report.TotalDuplicates, report.SpaceToFree); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to send notification: %v\n", err)
	}

	// Launch TUI for user review
	if err := daemon.LaunchTUI(reportPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to launch TUI: %v\n", err)
		fmt.Fprintf(os.Stderr, "View report manually with: jellysink view %s\n", reportPath)
	}
}

func loadConfig() (*config.Config, error) {
	return config.Load()
}

func loadReport(path string) (reporter.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reporter.Report{}, fmt.Errorf("failed to read report: %w", err)
	}

	var report reporter.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return reporter.Report{}, fmt.Errorf("failed to parse report: %w", err)
	}

	return report, nil
}
