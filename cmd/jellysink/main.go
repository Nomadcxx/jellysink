package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nomadcxx/jellysink/internal/cleaner"
	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

const (
	version = "0.1.0-dev"
	banner  = `
    __     ____           _       __
   / /__  / / /_  _______(_)___  / /__
  / / _ \/ / / / / / ___/ / __ \/ //_/
 / /  __/ / / /_/ (__  ) / / / / ,<
/_/\___/_/_/\__, /____/_/_/ /_/_/|_|
           /____/
`
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "scan":
		runScan()
	case "clean":
		runClean()
	case "config":
		runConfig()
	case "report":
		runReport()
	case "version":
		fmt.Printf("jellysink version %s\n", version)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(banner)
	fmt.Printf("jellysink version %s\n", version)
	fmt.Println("\nAutomated media library maintenance for Jellyfin/Plex")
	fmt.Println("\nUSAGE:")
	fmt.Println("  jellysink <command> [options]")
	fmt.Println("\nCOMMANDS:")
	fmt.Println("  scan       Scan libraries for duplicates and compliance issues")
	fmt.Println("  clean      Clean duplicates and fix compliance (interactive)")
	fmt.Println("  config     Manage configuration")
	fmt.Println("  report     View latest scan report")
	fmt.Println("  version    Show version information")
	fmt.Println("  help       Show this help message")
	fmt.Println("\nOPTIONS:")
	fmt.Println("  --dry-run  Simulate actions without making changes")
	fmt.Println("\nEXAMPLES:")
	fmt.Println("  jellysink scan")
	fmt.Println("  jellysink clean --dry-run")
	fmt.Println("  jellysink config add-movie /media/movies")
	fmt.Println("\nCONFIGURATION:")
	fmt.Println("  Config file: ~/.config/jellysink/config.toml")
	fmt.Println("  Reports:     ~/.local/share/jellysink/scan_results/")
	fmt.Println()
}

func runScan() {
	fmt.Println("🔍 Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'jellysink config init' to create a default configuration.\n")
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'jellysink config' to manage your configuration.\n")
		os.Exit(1)
	}

	report := reporter.Report{
		Timestamp:    time.Now(),
		LibraryPaths: cfg.GetAllPaths(),
	}

	// Scan movies
	if len(cfg.Libraries.Movies.Paths) > 0 {
		fmt.Println("🎬 Scanning movie libraries...")
		movieDups, err := scanner.ScanMovies(cfg.Libraries.Movies.Paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning movies: %v\n", err)
			os.Exit(1)
		}
		report.MovieDuplicates = scanner.MarkKeepDelete(movieDups)
		fmt.Printf("   Found %d duplicate movie groups\n", len(report.MovieDuplicates))

		// Check movie compliance
		fmt.Println("📋 Checking movie naming compliance...")
		movieCompliance, err := scanner.ScanMovieCompliance(cfg.Libraries.Movies.Paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking movie compliance: %v\n", err)
			os.Exit(1)
		}
		report.ComplianceIssues = append(report.ComplianceIssues, movieCompliance...)
		fmt.Printf("   Found %d movie compliance issues\n", len(movieCompliance))
	}

	// Scan TV shows
	if len(cfg.Libraries.TV.Paths) > 0 {
		fmt.Println("📺 Scanning TV show libraries...")
		tvDups, err := scanner.ScanTVShows(cfg.Libraries.TV.Paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning TV shows: %v\n", err)
			os.Exit(1)
		}
		report.TVDuplicates = scanner.MarkKeepDeleteTV(tvDups)
		fmt.Printf("   Found %d duplicate TV episode groups\n", len(report.TVDuplicates))

		// Check TV compliance
		fmt.Println("📋 Checking TV show naming compliance...")
		tvCompliance, err := scanner.ScanTVCompliance(cfg.Libraries.TV.Paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking TV compliance: %v\n", err)
			os.Exit(1)
		}
		report.ComplianceIssues = append(report.ComplianceIssues, tvCompliance...)
		fmt.Printf("   Found %d TV show compliance issues\n", len(tvCompliance))
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
	fmt.Println("\n📄 Generating reports...")
	files, err := reporter.GenerateDetailed(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating reports: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("SCAN COMPLETE")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Duplicate groups:     %d\n", report.TotalDuplicates)
	fmt.Printf("Files to delete:      %d\n", report.TotalFilesToDelete)
	fmt.Printf("Space to free:        %s\n", formatBytes(report.SpaceToFree))
	fmt.Printf("Compliance issues:    %d\n", len(report.ComplianceIssues))
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\nReports saved:\n")
	fmt.Printf("  Summary:     %s\n", files.Summary)
	fmt.Printf("  Duplicates:  %s\n", files.Duplicates)
	fmt.Printf("  Compliance:  %s\n", files.Compliance)
	fmt.Println("\nRun 'jellysink clean' to start interactive cleanup")
	fmt.Println("Run 'jellysink report' to view the detailed report")
}

func runClean() {
	isDryRun := false
	for _, arg := range os.Args {
		if arg == "--dry-run" {
			isDryRun = true
			break
		}
	}

	if isDryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made")
	}

	fmt.Println("🧹 Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// Scan first
	fmt.Println("🔍 Scanning libraries...")
	movieDups, err := scanner.ScanMovies(cfg.Libraries.Movies.Paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning movies: %v\n", err)
		os.Exit(1)
	}
	movieDups = scanner.MarkKeepDelete(movieDups)

	tvDups, err := scanner.ScanTVShows(cfg.Libraries.TV.Paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning TV shows: %v\n", err)
		os.Exit(1)
	}
	tvDups = scanner.MarkKeepDeleteTV(tvDups)

	var complianceIssues []scanner.ComplianceIssue
	if len(cfg.Libraries.Movies.Paths) > 0 {
		movieCompliance, err := scanner.ScanMovieCompliance(cfg.Libraries.Movies.Paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking movie compliance: %v\n", err)
			os.Exit(1)
		}
		complianceIssues = append(complianceIssues, movieCompliance...)
	}

	if len(cfg.Libraries.TV.Paths) > 0 {
		tvCompliance, err := scanner.ScanTVCompliance(cfg.Libraries.TV.Paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking TV compliance: %v\n", err)
			os.Exit(1)
		}
		complianceIssues = append(complianceIssues, tvCompliance...)
	}

	// Show what we found
	totalDuplicates := len(movieDups) + len(tvDups)
	if totalDuplicates == 0 && len(complianceIssues) == 0 {
		fmt.Println("✅ No issues found. Library is clean!")
		return
	}

	fmt.Printf("\nFound:\n")
	fmt.Printf("  Duplicate groups:     %d\n", totalDuplicates)
	fmt.Printf("  Compliance issues:    %d\n", len(complianceIssues))

	// Ask for confirmation
	if !isDryRun {
		fmt.Print("\nProceed with cleanup? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cleanup cancelled.")
			return
		}
	}

	// Perform cleanup
	cleanerCfg := cleaner.DefaultConfig()
	cleanerCfg.DryRun = isDryRun

	fmt.Println("\n🧹 Cleaning...")
	result, err := cleaner.Clean(movieDups, tvDups, complianceIssues, cleanerCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during cleanup: %v\n", err)
		os.Exit(1)
	}

	// Show results
	fmt.Println("\n" + strings.Repeat("=", 70))
	if isDryRun {
		fmt.Println("DRY RUN COMPLETE (no changes made)")
	} else {
		fmt.Println("CLEANUP COMPLETE")
	}
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Duplicates deleted:   %d\n", result.DuplicatesDeleted)
	fmt.Printf("Compliance fixed:     %d\n", result.ComplianceFixed)
	fmt.Printf("Space freed:          %s\n", formatBytes(result.SpaceFreed))
	fmt.Printf("Errors:               %d\n", len(result.Errors))
	fmt.Println(strings.Repeat("=", 70))

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors encountered:")
		for i, err := range result.Errors {
			fmt.Printf("  %d. %v\n", i+1, err)
		}
	}

	if !isDryRun && result.DuplicatesDeleted > 0 {
		fmt.Println("\n💡 Tip: Refresh your Jellyfin/Plex library to update the media server")
	}
}

func runConfig() {
	if len(os.Args) < 3 {
		printConfigHelp()
		return
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "init":
		initConfig()
	case "show":
		showConfig()
	case "add-movie":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: Missing path argument\n")
			fmt.Fprintf(os.Stderr, "Usage: jellysink config add-movie <path>\n")
			os.Exit(1)
		}
		addMoviePath(os.Args[3])
	case "add-tv":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: Missing path argument\n")
			fmt.Fprintf(os.Stderr, "Usage: jellysink config add-tv <path>\n")
			os.Exit(1)
		}
		addTVPath(os.Args[3])
	case "remove-movie":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: Missing path argument\n")
			fmt.Fprintf(os.Stderr, "Usage: jellysink config remove-movie <path>\n")
			os.Exit(1)
		}
		removeMoviePath(os.Args[3])
	case "remove-tv":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: Missing path argument\n")
			fmt.Fprintf(os.Stderr, "Usage: jellysink config remove-tv <path>\n")
			os.Exit(1)
		}
		removeTVPath(os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "Unknown config command: %s\n\n", subcommand)
		printConfigHelp()
		os.Exit(1)
	}
}

func printConfigHelp() {
	fmt.Println("USAGE:")
	fmt.Println("  jellysink config <subcommand>")
	fmt.Println("\nSUBCOMMANDS:")
	fmt.Println("  init              Create default configuration")
	fmt.Println("  show              Show current configuration")
	fmt.Println("  add-movie <path>  Add movie library path")
	fmt.Println("  add-tv <path>     Add TV show library path")
	fmt.Println("  remove-movie <p>  Remove movie library path")
	fmt.Println("  remove-tv <path>  Remove TV show library path")
}

func initConfig() {
	cfg := config.DefaultConfig()
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	path, _ := config.ConfigPath()
	fmt.Printf("✅ Configuration initialized at: %s\n", path)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Add your media library paths:")
	fmt.Println("     jellysink config add-movie /path/to/movies")
	fmt.Println("     jellysink config add-tv /path/to/tvshows")
	fmt.Println("  2. Run a scan:")
	fmt.Println("     jellysink scan")
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	path, _ := config.ConfigPath()
	fmt.Printf("Configuration file: %s\n\n", path)

	fmt.Println("Movie Libraries:")
	if len(cfg.Libraries.Movies.Paths) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range cfg.Libraries.Movies.Paths {
			fmt.Printf("  - %s\n", p)
		}
	}

	fmt.Println("\nTV Show Libraries:")
	if len(cfg.Libraries.TV.Paths) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range cfg.Libraries.TV.Paths {
			fmt.Printf("  - %s\n", p)
		}
	}

	fmt.Println("\nDaemon Settings:")
	fmt.Printf("  Scan Frequency:      %s\n", cfg.Daemon.ScanFrequency)
	fmt.Printf("  Report on Complete:  %v\n", cfg.Daemon.ReportOnComplete)
}

func addMoviePath(path string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.AddMoviePath(absPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error adding movie path: %v\n", err)
		os.Exit(1)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Added movie library path: %s\n", absPath)
}

func addTVPath(path string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.AddTVPath(absPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error adding TV path: %v\n", err)
		os.Exit(1)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Added TV show library path: %s\n", absPath)
}

func removeMoviePath(path string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.RemoveMoviePath(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing movie path: %v\n", err)
		os.Exit(1)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Removed movie library path: %s\n", path)
}

func removeTVPath(path string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.RemoveTVPath(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing TV path: %v\n", err)
		os.Exit(1)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Removed TV show library path: %s\n", path)
}

func runReport() {
	// Find most recent report
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	reportDir := filepath.Join(home, ".local/share/jellysink/scan_results")
	entries, err := os.ReadDir(reportDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading report directory: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'jellysink scan' first to generate a report.\n")
		os.Exit(1)
	}

	// Find most recent summary file
	var latestSummary string
	var latestTime time.Time

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_summary.txt") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestSummary = entry.Name()
			}
		}
	}

	if latestSummary == "" {
		fmt.Fprintf(os.Stderr, "No reports found. Run 'jellysink scan' first.\n")
		os.Exit(1)
	}

	// Display the report
	summaryPath := filepath.Join(reportDir, latestSummary)
	content, err := os.ReadFile(summaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading report: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(content))

	// Offer to view detailed reports
	fmt.Println("\nVIEW DETAILED REPORTS:")
	baseTime := strings.TrimSuffix(latestSummary, "_summary.txt")
	duplicatesPath := filepath.Join(reportDir, baseTime+"_duplicates.txt")
	compliancePath := filepath.Join(reportDir, baseTime+"_compliance.txt")

	fmt.Printf("  Duplicates:  cat %s\n", duplicatesPath)
	fmt.Printf("  Compliance:  cat %s\n", compliancePath)
}

// Helper functions

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
