package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nomadcxx/jellysink/internal/cleaner"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: jellysink <report-file.json>")
		fmt.Println("\nExample: jellysink ~/.local/share/jellysink/scan_results/20250119_143025.json")
		os.Exit(1)
	}

	reportPath := os.Args[1]

	// Load the report
	report, err := loadReport(reportPath)
	if err != nil {
		fmt.Printf("Error loading report: %v\n", err)
		os.Exit(1)
	}

	// Create TUI model
	model := ui.NewModel(report)

	// Run the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Check if user pressed Enter (clean operation)
	m := finalModel.(ui.Model)
	if m.ShouldClean() {
		performClean(report)
	}
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
