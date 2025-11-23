package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/Nomadcxx/jellysink/internal/cleaner"
	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
	"github.com/Nomadcxx/jellysink/internal/ui"
)

var (
	cfgFile string
	dryRun  bool
	quiet   bool
	verbose bool

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
	Run:   runTUI, // Launch TUI by default when run without subcommands
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
	scanCmd.Flags().BoolVar(&quiet, "quiet", false, "minimal output (errors only)")
	scanCmd.Flags().BoolVar(&verbose, "verbose", false, "detailed output (debug info)")

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

// isRunningAsRoot checks if the program is running with root privileges
func isRunningAsRoot() bool {
	return os.Geteuid() == 0
}

// reexecWithSudo re-executes the current command with sudo
func reexecWithSudo() {
	fmt.Println(ui.FormatASCIIHeader())
	fmt.Println(ui.FormatStatusWarn("Root Access Required"))
	fmt.Println()
	fmt.Println("jellysink needs root access to:")
	fmt.Println("  • Control systemd services (enable/disable daemon)")
	fmt.Println("  • Delete media files during cleanup")
	fmt.Println()
	fmt.Println(ui.MutedStyle.Render("You will be prompted for your password..."))
	fmt.Println()

	// Get the current executable path
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Unable to determine executable path: %v\n", err)
		os.Exit(1)
	}

	// Build the sudo command with all original arguments
	args := append([]string{exe}, os.Args[1:]...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute with sudo
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to execute with sudo: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// runTUI launches the main menu TUI (default behavior)
func runTUI(cmd *cobra.Command, args []string) {
	// Check for root access and re-exec with sudo if needed
	if !isRunningAsRoot() {
		reexecWithSudo()
		return
	}

	// Load config
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Fprintf(os.Stderr, "Creating default config...\n")
		cfg = config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
			os.Exit(1)
		}
	}

	// Launch main menu TUI
	model := ui.NewMenuModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runScan(cmd *cobra.Command, args []string) {
	// Check for root access
	if !isRunningAsRoot() {
		reexecWithSudo()
		return
	}

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

	// Determine log level from flags
	logLevel := scanner.LogLevelNormal
	if quiet && verbose {
		fmt.Fprintf(os.Stderr, "Error: --quiet and --verbose are mutually exclusive\n")
		os.Exit(1)
	}
	if quiet {
		logLevel = scanner.LogLevelQuiet
	}
	if verbose {
		logLevel = scanner.LogLevelVerbose
	}

	// Set the global log level for progress reporters
	scanner.SetDefaultLogLevel(logLevel)

	fmt.Println("Starting scan...")

	// Create progress channel
	progressCh := make(chan scanner.ScanProgress, 100)

	// Start scan in goroutine
	type scanResult struct {
		path string
		err  error
	}
	resultCh := make(chan scanResult)

	go func() {
		d := daemon.New(cfg)
		path, err := d.RunScanWithProgress(ctx, progressCh)
		resultCh <- scanResult{path, err}
		close(progressCh)
	}()

	// Display progress with log level filtering
	lastOperation := ""
	for progress := range progressCh {
		// Apply log level filtering
		shouldShow := false
		switch logLevel {
		case scanner.LogLevelQuiet:
			shouldShow = progress.Severity == "error" || progress.Severity == "critical"
		case scanner.LogLevelNormal:
			shouldShow = progress.Severity != "debug"
		case scanner.LogLevelVerbose:
			shouldShow = true
		}

		if !shouldShow {
			continue
		}

		// Format output based on severity
		if progress.Severity == "error" || progress.Severity == "critical" {
			fmt.Fprintf(os.Stderr, "✗ %s\n", progress.Message)
		} else if progress.Operation != lastOperation {
			fmt.Printf("\n%s...\n", progress.Message)
			lastOperation = progress.Operation
		} else if logLevel == scanner.LogLevelVerbose || progress.Current%50 == 0 || progress.Stage == "complete" {
			fmt.Printf("  %.1f%% - %s\n", progress.Percentage, progress.Message)
		}
	}

	// Get result
	result := <-resultCh
	if result.err != nil {
		if result.err == context.Canceled {
			fmt.Fprintf(os.Stderr, "\nScan cancelled by user\n")
			os.Exit(130) // Exit code 130 for SIGINT
		}
		fmt.Fprintf(os.Stderr, "\nScan failed: %v\n", result.err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Scan complete! Report saved to:\n  %s\n\n", result.path)
	fmt.Printf("View report with: jellysink view %s\n", result.path)
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
		resolvedConflicts := m.GetResolvedConflicts()
		hasResolvedConflicts := false
		for _, c := range resolvedConflicts {
			if c.UserDecision != 0 {
				hasResolvedConflicts = true
				break
			}
		}

		if hasResolvedConflicts {
			performConflictRenames(report, resolvedConflicts)
		} else {
			editedTitles := m.GetEditedTitles()
			if len(editedTitles) > 0 {
				performManualRenames(report, editedTitles)
			} else {
				performClean(report)
			}
		}
	}
}

func runClean(cmd *cobra.Command, args []string) {
	// Check for root access (unless dry-run)
	if !dryRun && !isRunningAsRoot() {
		reexecWithSudo()
		return
	}

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

func performConflictRenames(report reporter.Report, conflicts []*scanner.TVTitleResolution) {
	fmt.Println("\nApplying resolved conflict renames...")

	activeConflicts := 0
	for _, c := range conflicts {
		if c.UserDecision != 0 {
			activeConflicts++
		}
	}

	fmt.Printf("Shows to rename: %d\n\n", activeConflicts)

	totalResults := []interface{}{}
	successCount := 0
	errorCount := 0

	for _, conflict := range conflicts {
		if conflict.UserDecision == 0 {
			continue
		}

		oldTitle := ""
		if conflict.FolderMatch != nil {
			oldTitle = conflict.FolderMatch.Title
		} else if conflict.FilenameMatch != nil {
			oldTitle = conflict.FilenameMatch.Title
		}

		newTitle := conflict.ResolvedTitle

		if oldTitle == "" || newTitle == "" {
			fmt.Printf("⚠ Skipping conflict with missing title data\n")
			continue
		}

		fmt.Printf("\nRenaming: %s -> %s\n", oldTitle, newTitle)

		for _, libPath := range report.LibraryPaths {
			results, err := scanner.ApplyManualTVRename(libPath, oldTitle, newTitle, false)
			if err != nil {
				fmt.Printf("  ✗ Error in %s: %v\n", libPath, err)
				errorCount++
				continue
			}

			for _, result := range results {
				totalResults = append(totalResults, result)
				if result.Success {
					successCount++
					typeStr := "file"
					if result.IsFolder {
						typeStr = "folder"
					}
					fmt.Printf("  ✓ Renamed %s: %s\n", typeStr, filepath.Base(result.NewPath))
				} else {
					errorCount++
					fmt.Printf("  ✗ Failed: %s - %s\n", result.OldPath, result.Error)
				}
			}
		}
	}

	fmt.Println("\nRename operation completed!")
	fmt.Printf("✓ Successful renames: %d\n", successCount)
	if errorCount > 0 {
		fmt.Printf("✗ Errors: %d\n", errorCount)
	}

	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".local/share/jellysink/rename.log")
	fmt.Printf("\nOperation log saved to: %s\n", logPath)
}

func performManualRenames(report reporter.Report, editedTitles map[int]string) {
	fmt.Println("\nApplying manual TV show renames...")
	fmt.Printf("Shows to rename: %d\n\n", len(editedTitles))

	// Confirm with user
	fmt.Print("Are you sure you want to proceed? (yes/no): ")
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Rename cancelled.")
		return
	}

	totalResults := []interface{}{}
	successCount := 0
	errorCount := 0

	// Apply each rename
	for idx, newTitle := range editedTitles {
		if idx >= len(report.AmbiguousTVShows) {
			fmt.Printf("⚠ Skipping invalid index %d\n", idx)
			continue
		}

		resolution := report.AmbiguousTVShows[idx]
		oldTitle := resolution.ResolvedTitle
		if oldTitle == "" && resolution.FolderMatch != nil {
			oldTitle = resolution.FolderMatch.Title
		}
		if oldTitle == "" && resolution.FilenameMatch != nil {
			oldTitle = resolution.FilenameMatch.Title
		}

		fmt.Printf("\nRenaming: %s -> %s\n", oldTitle, newTitle)

		// Apply rename to all library paths
		for _, libPath := range report.LibraryPaths {
			// Import scanner package
			results, err := scanner.ApplyManualTVRename(libPath, oldTitle, newTitle, false)
			if err != nil {
				fmt.Printf("  ✗ Error in %s: %v\n", libPath, err)
				errorCount++
				continue
			}

			for _, result := range results {
				totalResults = append(totalResults, result)
				if result.Success {
					successCount++
					typeStr := "file"
					if result.IsFolder {
						typeStr = "folder"
					}
					fmt.Printf("  ✓ Renamed %s: %s\n", typeStr, filepath.Base(result.NewPath))
				} else {
					errorCount++
					fmt.Printf("  ✗ Failed: %s - %s\n", result.OldPath, result.Error)
				}
			}
		}
	}

	// Show results
	fmt.Println("\nRename operation completed!")
	fmt.Printf("✓ Successful renames: %d\n", successCount)
	if errorCount > 0 {
		fmt.Printf("✗ Errors: %d\n", errorCount)
	}

	// Save operation log
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".local/share/jellysink/rename.log")
	fmt.Printf("\nOperation log saved to: %s\n", logPath)
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
