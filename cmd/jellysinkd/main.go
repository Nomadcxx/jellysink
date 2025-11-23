package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
	"github.com/Nomadcxx/jellysink/internal/reporter"
)

var (
	// Version information (set via -ldflags during build)
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"

	// CLI flags
	testMode = flag.Bool("test", false, "Test mode: run scan and launch kitty to verify workflow")
)

func main() {
	flag.Parse()

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

	// Create context with cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\njellysinkd: Cancelling scan...")
		cancel()
	}()

	// Create daemon instance
	d := daemon.New(cfg)

	// Run scan
	if *testMode {
		fmt.Println("jellysinkd: Running in TEST MODE...")
	} else {
		fmt.Println("jellysinkd: Starting scheduled scan...")
	}

	reportPath, err := d.RunScan(ctx)
	if err != nil {
		if err == context.Canceled {
			fmt.Fprintf(os.Stderr, "Scan cancelled by signal\n")
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "Scan failed: %v\n", err)
		os.Exit(1)
	}

	// Load report to get statistics
	report, err := loadReport(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scan complete! Found %d duplicate groups", report.TotalDuplicates)
	if len(report.ComplianceIssues) > 0 {
		fmt.Printf(" + %d compliance issues\n", len(report.ComplianceIssues))
	} else {
		fmt.Println()
	}
	fmt.Printf("Report saved to: %s\n", reportPath)

	// Clean up old reports (30+ days)
	if err := daemon.CleanupOldReports(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clean old reports: %v\n", err)
	}

	// Determine workflow: headless auto-clean or interactive review
	if d.IsHeadless() && !*testMode {
		fmt.Println("Headless mode detected - running auto-clean...")
		if err := d.AutoClean(report); err != nil {
			fmt.Fprintf(os.Stderr, "Auto-clean failed: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Interactive mode: launch kitty with report
		fmt.Println("Launching kitty for interactive review...")
		if err := daemon.NotifyUser(reportPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch kitty: %v\n", err)
			fmt.Fprintf(os.Stderr, "View report manually with: jellysink view %s\n", reportPath)
			os.Exit(1)
		}

		if *testMode {
			fmt.Println("\n✓ TEST MODE: Kitty launched successfully!")
			fmt.Println("  Check if kitty window opened with the scan report.")
		}
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
