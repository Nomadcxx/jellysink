package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// Report represents a scan report with duplicates and compliance issues
type Report struct {
	Timestamp          time.Time
	LibraryType        string   // "movies" or "tv"
	LibraryPaths       []string
	MovieDuplicates    []scanner.MovieDuplicate
	TVDuplicates       []scanner.TVDuplicate
	ComplianceIssues   []scanner.ComplianceIssue
	TotalDuplicates    int
	TotalFilesToDelete int
	SpaceToFree        int64
}

// Generate creates a timestamped report file
func Generate(report Report) (string, error) {
	// Create report directory
	reportDir := getReportDir()
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create report directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := report.Timestamp.Format("20060102_150405")
	filename := filepath.Join(reportDir, timestamp+".txt")

	// Build report content
	content := buildReportContent(report)

	// Write to file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	return filename, nil
}

// getReportDir returns the report directory path
func getReportDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/jellysink/scan_results"
	}
	return filepath.Join(home, ".local/share/jellysink/scan_results")
}

// buildReportContent generates the report text
func buildReportContent(report Report) string {
	var sb strings.Builder

	// Header
	sb.WriteString("JELLYSINK SCAN REPORT\n")
	sb.WriteString(strings.Repeat("=", 80) + "\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n", report.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Library Type: %s\n", report.LibraryType))
	sb.WriteString(fmt.Sprintf("Library Paths: %s\n", strings.Join(report.LibraryPaths, ", ")))
	sb.WriteString("\n")

	// Summary
	sb.WriteString("SUMMARY\n")
	sb.WriteString(strings.Repeat("=", 80) + "\n")
	sb.WriteString(fmt.Sprintf("Duplicate groups found: %d\n", report.TotalDuplicates))
	sb.WriteString(fmt.Sprintf("Files to delete: %d\n", report.TotalFilesToDelete))
	sb.WriteString(fmt.Sprintf("Space to free: %s\n", formatBytes(report.SpaceToFree)))
	sb.WriteString(fmt.Sprintf("Compliance issues: %d\n", len(report.ComplianceIssues)))
	sb.WriteString("\n")

	// Top offenders (if duplicates exist)
	if report.TotalDuplicates > 0 {
		sb.WriteString("TOP OFFENDERS\n")
		sb.WriteString(strings.Repeat("=", 80) + "\n")
		topOffenders := getTopOffenders(report)
		for i, offender := range topOffenders {
			sb.WriteString(fmt.Sprintf("%d. %s - %d versions, %s to free\n",
				i+1, offender.Name, offender.Count, formatBytes(offender.SpaceToFree)))
		}
		sb.WriteString("\n")
	}

	// Detailed duplicates
	if len(report.MovieDuplicates) > 0 {
		sb.WriteString("MOVIE DUPLICATES (DETAILED)\n")
		sb.WriteString(strings.Repeat("=", 80) + "\n")
		for _, dup := range report.MovieDuplicates {
			sb.WriteString(formatMovieDuplicate(dup))
			sb.WriteString("\n")
		}
	}

	if len(report.TVDuplicates) > 0 {
		sb.WriteString("TV DUPLICATES (DETAILED)\n")
		sb.WriteString(strings.Repeat("=", 80) + "\n")
		for _, dup := range report.TVDuplicates {
			sb.WriteString(formatTVDuplicate(dup))
			sb.WriteString("\n")
		}
	}

	// Compliance issues
	if len(report.ComplianceIssues) > 0 {
		sb.WriteString("COMPLIANCE ISSUES\n")
		sb.WriteString(strings.Repeat("=", 80) + "\n")
		for i, issue := range report.ComplianceIssues {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, strings.ToUpper(issue.Type), issue.Problem))
			sb.WriteString(fmt.Sprintf("   Current:  %s\n", issue.Path))
			sb.WriteString(fmt.Sprintf("   Suggested: %s\n", issue.SuggestedPath))
			sb.WriteString(fmt.Sprintf("   Action: %s\n\n", issue.SuggestedAction))
		}
	}

	// Footer with deletion list (machine-readable section)
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", 80) + "\n")
	sb.WriteString("DELETION LIST (DO NOT EDIT)\n")
	sb.WriteString(strings.Repeat("=", 80) + "\n")

	// Add all files marked for deletion
	for _, dup := range report.MovieDuplicates {
		for i := 1; i < len(dup.Files); i++ {
			sb.WriteString(dup.Files[i].Path + "\n")
		}
	}

	for _, dup := range report.TVDuplicates {
		for i := 1; i < len(dup.Files); i++ {
			sb.WriteString(dup.Files[i].Path + "\n")
		}
	}

	return sb.String()
}

// Offender represents a duplicate group with stats
type Offender struct {
	Name        string
	Count       int
	SpaceToFree int64
}

// getTopOffenders returns top 15 duplicate groups by space saved
func getTopOffenders(report Report) []Offender {
	var offenders []Offender

	// Add movie duplicates
	for _, dup := range report.MovieDuplicates {
		space := int64(0)
		for i := 1; i < len(dup.Files); i++ {
			space += dup.Files[i].Size
		}

		name := dup.NormalizedName
		if dup.Year != "" {
			name = name + " (" + dup.Year + ")"
		}

		offenders = append(offenders, Offender{
			Name:        name,
			Count:       len(dup.Files),
			SpaceToFree: space,
		})
	}

	// Add TV duplicates
	for _, dup := range report.TVDuplicates {
		space := int64(0)
		for i := 1; i < len(dup.Files); i++ {
			space += dup.Files[i].Size
		}

		name := fmt.Sprintf("%s S%02dE%02d", dup.ShowName, dup.Season, dup.Episode)

		offenders = append(offenders, Offender{
			Name:        name,
			Count:       len(dup.Files),
			SpaceToFree: space,
		})
	}

	// Sort by space descending
	sort.Slice(offenders, func(i, j int) bool {
		return offenders[i].SpaceToFree > offenders[j].SpaceToFree
	})

	// Return top 15
	if len(offenders) > 15 {
		return offenders[:15]
	}
	return offenders
}

// formatMovieDuplicate formats a movie duplicate group for display
func formatMovieDuplicate(dup scanner.MovieDuplicate) string {
	var sb strings.Builder

	title := dup.NormalizedName
	if dup.Year != "" {
		title = title + " (" + dup.Year + ")"
	}

	sb.WriteString(fmt.Sprintf("%s (%d versions):\n", title, len(dup.Files)))

	for i, file := range dup.Files {
		marker := "  DELETE:"
		if i == 0 {
			marker = "  KEEP:  "
		}

		sb.WriteString(fmt.Sprintf("%s [%s] [%s] %s\n",
			marker,
			formatBytes(file.Size),
			file.Resolution,
			filepath.Base(file.Path)))
		sb.WriteString(fmt.Sprintf("          %s\n", file.Path))
	}

	return sb.String()
}

// formatTVDuplicate formats a TV duplicate group for display
func formatTVDuplicate(dup scanner.TVDuplicate) string {
	var sb strings.Builder

	title := fmt.Sprintf("%s S%02dE%02d", dup.ShowName, dup.Season, dup.Episode)
	sb.WriteString(fmt.Sprintf("%s (%d versions):\n", title, len(dup.Files)))

	for i, file := range dup.Files {
		marker := "  DELETE:"
		if i == 0 {
			marker = "  KEEP:  "
		}

		sb.WriteString(fmt.Sprintf("%s [%s] [%s] [%s] %s\n",
			marker,
			formatBytes(file.Size),
			file.Resolution,
			file.Source,
			filepath.Base(file.Path)))
		sb.WriteString(fmt.Sprintf("          %s\n", file.Path))
	}

	return sb.String()
}

// formatBytes formats byte count to human-readable size
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
