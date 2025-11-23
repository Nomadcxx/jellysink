package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ScanProgress represents real-time scan progress
type ScanProgress struct {
	Operation  string  // "scanning_movies", "scanning_tv", "compliance_movies", "compliance_tv", "generating_report"
	Stage      string  // "counting_files", "scanning", "analyzing", "complete"
	Current    int     // Current file/item number
	Total      int     // Total files/items
	Percentage float64 // 0-100
	Message    string  // Human-readable status

	// Statistics
	DuplicatesFound   int
	ComplianceIssues  int
	FilesProcessed    int
	ErrorsEncountered int

	// Optional list of error messages
	Errors []string

	// Timing
	StartTime      time.Time
	ElapsedSeconds int
}

// ProgressReporter helps send progress updates
type ProgressReporter struct {
	ch        chan<- ScanProgress
	operation string
	startTime time.Time
	total     int

	// Stats
	duplicatesFound   int
	complianceIssues  int
	filesProcessed    int
	errorsEncountered int
	errors            []string
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(ch chan<- ScanProgress, operation string) *ProgressReporter {
	return &ProgressReporter{
		ch:        ch,
		operation: operation,
		startTime: time.Now(),
	}
}

// Start sends initial progress with total count
func (pr *ProgressReporter) Start(total int, message string) {
	pr.total = total
	pr.send(0, message)
}

// Update sends progress update
func (pr *ProgressReporter) Update(current int, message string) {
	pr.send(current, message)
}

// Complete sends completion message
func (pr *ProgressReporter) Complete(message string) {
	progress := ScanProgress{
		Operation:      pr.operation,
		Stage:          "complete",
		Current:        pr.total,
		Total:          pr.total,
		Percentage:     100.0,
		Message:        message,
		StartTime:      pr.startTime,
		ElapsedSeconds: int(time.Since(pr.startTime).Seconds()),
	}
	pr.ch <- progress
}

// send helper for building and sending progress
func (pr *ProgressReporter) send(current int, message string) {
	percentage := 0.0
	if pr.total > 0 {
		percentage = (float64(current) / float64(pr.total)) * 100.0
	}

	progress := ScanProgress{
		Operation:      pr.operation,
		Stage:          "scanning",
		Current:        current,
		Total:          pr.total,
		Percentage:     percentage,
		Message:        message,
		StartTime:      pr.startTime,
		ElapsedSeconds: int(time.Since(pr.startTime).Seconds()),
	}
	pr.ch <- progress
}

// CountVideoFiles counts all video files in the given paths (for accurate progress)
func CountVideoFiles(paths []string) (int, error) {
	count := 0

	for _, libPath := range paths {
		if _, err := os.Stat(libPath); err != nil {
			return 0, fmt.Errorf("library path not accessible: %s: %w", libPath, err)
		}

		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && isVideoFile(path) {
				count++
			}

			return nil
		})

		if err != nil {
			return 0, fmt.Errorf("error counting files in %s: %w", libPath, err)
		}
	}

	return count, nil
}
