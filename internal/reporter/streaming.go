package reporter

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// StreamingReporter writes report sections incrementally to reduce memory usage
type StreamingReporter struct {
	timestamp     time.Time
	libraryType   string
	libraryPaths  []string
	summaryWriter *bufio.Writer
	detailWriter  *bufio.Writer
	summaryFile   *os.File
	detailFile    *os.File
	totalDups     int
	totalFiles    int
	totalSpace    int64
}

// NewStreamingReporter creates a new streaming reporter
func NewStreamingReporter(libraryType string, libraryPaths []string) (*StreamingReporter, error) {
	reportDir := getReportDir()
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create report directory: %w", err)
	}

	timestamp := time.Now()
	timestampStr := timestamp.Format("20060102_150405")

	// Create summary file
	summaryPath := filepath.Join(reportDir, timestampStr+"_summary.txt")
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create summary file: %w", err)
	}

	// Create detail file
	detailPath := filepath.Join(reportDir, timestampStr+"_duplicates.txt")
	detailFile, err := os.Create(detailPath)
	if err != nil {
		summaryFile.Close()
		return nil, fmt.Errorf("failed to create detail file: %w", err)
	}

	sr := &StreamingReporter{
		timestamp:     timestamp,
		libraryType:   libraryType,
		libraryPaths:  libraryPaths,
		summaryFile:   summaryFile,
		detailFile:    detailFile,
		summaryWriter: bufio.NewWriter(summaryFile),
		detailWriter:  bufio.NewWriter(detailFile),
	}

	// Write headers
	if err := sr.writeHeaders(); err != nil {
		sr.Close()
		return nil, err
	}

	return sr, nil
}

// writeHeaders writes initial headers to report files
func (sr *StreamingReporter) writeHeaders() error {
	// Summary header
	header := fmt.Sprintf("Jellysink Scan Report\n")
	header += fmt.Sprintf("Generated: %s\n", sr.timestamp.Format(time.RFC1123))
	header += fmt.Sprintf("Library Type: %s\n", sr.libraryType)
	header += fmt.Sprintf("Library Paths:\n")
	for _, path := range sr.libraryPaths {
		header += fmt.Sprintf("  - %s\n", path)
	}
	header += fmt.Sprintf("\n")

	if _, err := sr.summaryWriter.WriteString(header); err != nil {
		return fmt.Errorf("failed to write summary header: %w", err)
	}

	// Detail header
	detailHeader := fmt.Sprintf("=== Duplicate Files Report ===\n")
	detailHeader += fmt.Sprintf("Generated: %s\n\n", sr.timestamp.Format(time.RFC1123))

	if _, err := sr.detailWriter.WriteString(detailHeader); err != nil {
		return fmt.Errorf("failed to write detail header: %w", err)
	}

	return nil
}

// WriteMovieDuplicate writes a single movie duplicate group (streaming)
func (sr *StreamingReporter) WriteMovieDuplicate(ctx context.Context, dup scanner.MovieDuplicate) error {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if len(dup.Files) < 2 {
		return nil
	}

	sr.totalDups++
	filesToDelete := len(dup.Files) - 1
	sr.totalFiles += filesToDelete

	// Calculate space to free
	for i := 1; i < len(dup.Files); i++ {
		sr.totalSpace += dup.Files[i].Size
	}

	// Write to detail file
	content := fmt.Sprintf("Movie: %s (%s)\n", dup.NormalizedName, dup.Year)
	content += fmt.Sprintf("  Duplicate versions found: %d\n", len(dup.Files))
	content += fmt.Sprintf("  Files to delete: %d\n", filesToDelete)
	content += fmt.Sprintf("  Space to free: %s\n", formatBytes(sr.calculateGroupSpace(dup)))
	content += fmt.Sprintf("\n  Files:\n")

	for i, file := range dup.Files {
		status := "[DELETE]"
		if i == 0 {
			status = "[KEEP]"
		}
		content += fmt.Sprintf("    %s %s\n", status, file.Path)
		content += fmt.Sprintf("           Size: %s, Resolution: %s\n",
			formatBytes(file.Size), file.Resolution)
	}
	content += fmt.Sprintf("\n")

	if _, err := sr.detailWriter.WriteString(content); err != nil {
		return fmt.Errorf("failed to write duplicate: %w", err)
	}

	return nil
}

// WriteTVDuplicate writes a single TV duplicate group (streaming)
func (sr *StreamingReporter) WriteTVDuplicate(ctx context.Context, dup scanner.TVDuplicate) error {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if len(dup.Files) < 2 {
		return nil
	}

	sr.totalDups++
	filesToDelete := len(dup.Files) - 1
	sr.totalFiles += filesToDelete

	// Calculate space to free
	for i := 1; i < len(dup.Files); i++ {
		sr.totalSpace += dup.Files[i].Size
	}

	// Write to detail file
	content := fmt.Sprintf("TV Show: %s S%02dE%02d\n", dup.ShowName, dup.Season, dup.Episode)
	content += fmt.Sprintf("  Duplicate versions found: %d\n", len(dup.Files))
	content += fmt.Sprintf("  Files to delete: %d\n", filesToDelete)
	content += fmt.Sprintf("  Space to free: %s\n", formatBytes(sr.calculateTVGroupSpace(dup)))
	content += fmt.Sprintf("\n  Files:\n")

	for i, file := range dup.Files {
		status := "[DELETE]"
		if i == 0 {
			status = "[KEEP]"
		}
		content += fmt.Sprintf("    %s %s\n", status, file.Path)
		content += fmt.Sprintf("           Size: %s, Resolution: %s, Source: %s\n",
			formatBytes(file.Size), file.Resolution, file.Source)
	}
	content += fmt.Sprintf("\n")

	if _, err := sr.detailWriter.WriteString(content); err != nil {
		return fmt.Errorf("failed to write TV duplicate: %w", err)
	}

	return nil
}

// Finalize writes summary statistics and closes files
func (sr *StreamingReporter) Finalize() error {
	// Write summary statistics
	summary := fmt.Sprintf("=== Scan Results ===\n")
	summary += fmt.Sprintf("Total duplicate groups: %d\n", sr.totalDups)
	summary += fmt.Sprintf("Total files to delete: %d\n", sr.totalFiles)
	summary += fmt.Sprintf("Total space to free: %s\n", formatBytes(sr.totalSpace))
	summary += fmt.Sprintf("\n")

	if _, err := sr.summaryWriter.WriteString(summary); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	// Flush writers
	if err := sr.summaryWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush summary: %w", err)
	}
	if err := sr.detailWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush details: %w", err)
	}

	return nil
}

// Close closes all file handles
func (sr *StreamingReporter) Close() error {
	var errs []error

	if sr.summaryFile != nil {
		if err := sr.summaryFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close summary: %w", err))
		}
	}

	if sr.detailFile != nil {
		if err := sr.detailFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close detail: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing files: %v", errs)
	}

	return nil
}

// GetPaths returns the paths to the generated report files
func (sr *StreamingReporter) GetPaths() (string, string) {
	return sr.summaryFile.Name(), sr.detailFile.Name()
}

// calculateGroupSpace calculates total space to free for movie duplicate group
func (sr *StreamingReporter) calculateGroupSpace(dup scanner.MovieDuplicate) int64 {
	var total int64
	for i := 1; i < len(dup.Files); i++ {
		total += dup.Files[i].Size
	}
	return total
}

// calculateTVGroupSpace calculates total space to free for TV duplicate group
func (sr *StreamingReporter) calculateTVGroupSpace(dup scanner.TVDuplicate) int64 {
	var total int64
	for i := 1; i < len(dup.Files); i++ {
		total += dup.Files[i].Size
	}
	return total
}
