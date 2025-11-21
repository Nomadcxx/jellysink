package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/Nomadcxx/jellysink/internal/cleaner"
	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/ui"
)

var (
	cfgFile string
	dryRun  bool

	// Version information (set via -ldflags during build)
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

const exampleConfig = `[libraries.movies]
paths = ["/path/to/your/movies"]

[libraries.tv]
paths = ["/path/to/your/tvshows"]

[daemon]
scan_frequency = "weekly"  # daily, weekly, biweekly
`

var rootCmd = &cobra.Command{
	Use:   "jellysink",
	Short: "Media library maintenance tool for Jellyfin/Plex",
	Long:  getLongDescription(),
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan media libraries for duplicates and compliance issues",
	Run:   runScan,
}

var viewCmd = &cobra.Command{
	Use:   "view <report-file>",
	Short: "View a scan report in the TUI",
	Args:  cobra.ExactArgs(1),
	Run:   runView,
}

var cleanCmd = &cobra.Command{
	Use:   "clean <report-file>",
	Short: "Clean duplicates and fix compliance issues from a report",
	Args:  cobra.ExactArgs(1),
	Run:   runClean,
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show configuration file location and contents",
	Run:   runConfig,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("jellysink %s\n", version)
		fmt.Printf("  Commit:     %s\n", commit)
		fmt.Printf("  Built:      %s\n", buildTime)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/jellysink/config.toml)")
	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without actually deleting")

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runScan(cmd *cobra.Command, args []string) {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create context with cancellation support (Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nCancelling scan...")
		cancel()
	}()

	fmt.Println("Starting scan...")
	d := daemon.New(cfg)
	reportPath, err := d.RunScan(ctx)
	if err != nil {
		if err == context.Canceled {
			fmt.Fprintf(os.Stderr, "Scan cancelled by user\n")
			os.Exit(130) // Exit code 130 for SIGINT
		}
		fmt.Fprintf(os.Stderr, "Scan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nScan complete! Report saved to:\n  %s\n\n", reportPath)
	fmt.Printf("View report with: jellysink view %s\n", reportPath)
}

func runView(cmd *cobra.Command, args []string) {
	reportPath := args[0]

	// Load the report
	report, err := loadReport(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading report: %v\n", err)
		os.Exit(1)
	}

	// Create TUI model
	model := ui.NewModel(report)

	// Run the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Check if user pressed Enter (clean operation)
	m := finalModel.(ui.Model)
	if m.ShouldClean() {
		performClean(report)
	}
}

func runClean(cmd *cobra.Command, args []string) {
	reportPath := args[0]

	report, err := loadReport(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading report: %v\n", err)
		os.Exit(1)
	}

	performClean(report)
}

func runConfig(cmd *cobra.Command, args []string) {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config/jellysink/config.toml")

	fmt.Printf("Configuration file: %s\n\n", configPath)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("Config file does not exist. Create it with:")
		fmt.Println("\n  mkdir -p ~/.config/jellysink")
		fmt.Println("  cat > ~/.config/jellysink/config.toml <<EOF")
		fmt.Print(exampleConfig)
		fmt.Println("EOF")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Current configuration:")
	fmt.Printf("\nMovie libraries (%d):\n", len(cfg.Libraries.Movies.Paths))
	for _, path := range cfg.Libraries.Movies.Paths {
		fmt.Printf("  - %s\n", path)
	}

	fmt.Printf("\nTV libraries (%d):\n", len(cfg.Libraries.TV.Paths))
	for _, path := range cfg.Libraries.TV.Paths {
		fmt.Printf("  - %s\n", path)
	}

	fmt.Printf("\nDaemon settings:\n")
	fmt.Printf("  Scan frequency: %s\n", cfg.Daemon.ScanFrequency)
}

func loadConfig() (*config.Config, error) {
	return config.Load()
}

func getLongDescription() string {
	return ui.FormatASCIIHeader() + "\n\n" +
		"jellysink scans your media libraries for duplicates and naming compliance issues.\n" +
		"It generates reports and provides a TUI for reviewing and cleaning your library."
}

func loadReport(path string) (reporter.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reporter.Report{}, fmt.Errorf("failed to read report file: %w", err)
	}

	var report reporter.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return reporter.Report{}, fmt.Errorf("failed to parse report: %w", err)
	}

	return report, nil
}

func performClean(report reporter.Report) {
	fmt.Println("\nStarting cleanup operation...")
	fmt.Printf("Duplicates to delete: %d files\n", report.TotalFilesToDelete)
	fmt.Printf("Compliance issues to fix: %d\n", len(report.ComplianceIssues))
	fmt.Printf("Space to free: %s\n\n", formatBytes(report.SpaceToFree))

	// Confirm with user
	fmt.Print("Are you sure you want to proceed? (yes/no): ")
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Cleanup cancelled.")
		return
	}

	// Execute cleanup
	config := cleaner.DefaultConfig()
	config.DryRun = false

	result, err := cleaner.Clean(
		report.MovieDuplicates,
		report.TVDuplicates,
		report.ComplianceIssues,
		config,
	)

	if err != nil {
		fmt.Printf("Error during cleanup: %v\n", err)
		os.Exit(1)
	}

	// Show results
	fmt.Println("\nCleanup completed!")
	fmt.Printf("✓ Duplicates deleted: %d\n", result.DuplicatesDeleted)
	fmt.Printf("✓ Compliance issues fixed: %d\n", result.ComplianceFixed)
	fmt.Printf("✓ Space freed: %s\n", formatBytes(result.SpaceFreed))

	if len(result.Errors) > 0 {
		fmt.Printf("\n⚠ Errors encountered: %d\n", len(result.Errors))
		for i, err := range result.Errors {
			fmt.Printf("  %d. %v\n", i+1, err)
		}
	}

	// Save operation log location
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".local/share/jellysink/operations.log")
	fmt.Printf("\nOperation log saved to: %s\n", logPath)
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
