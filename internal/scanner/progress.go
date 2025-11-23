package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	Severity   string  // "info", "warn", "error", "critical"

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

	// UI Alert flag - if true, TUI should show modal alert
	ShowAlert bool
	AlertType string // "error", "critical", "warning"
}

// LogLevel controls which messages get sent
type LogLevel int

const (
	LogLevelQuiet   LogLevel = 0 // Only errors and critical messages
	LogLevelNormal  LogLevel = 1 // Info, warnings, errors
	LogLevelVerbose LogLevel = 2 // All messages including debug
)

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

	// Rate limiting for UI updates
	minInterval time.Duration
	lastSent    time.Time

	// Log level filtering
	logLevel LogLevel
}

// NewProgressReporter creates a new progress reporter with normal log level
// DefaultLogLevel controls global default filtering for new reporters
var DefaultLogLevel LogLevel = LogLevelNormal

// SetDefaultLogLevel sets global default log level for new reporters
func SetDefaultLogLevel(level LogLevel) {
	DefaultLogLevel = level
}

// GetDefaultLogLevel returns the global default log level
func GetDefaultLogLevel() LogLevel {
	return DefaultLogLevel
}

// ParseLogLevel converts a string to the LogLevel enum
func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(s) {
	case "quiet":
		return LogLevelQuiet, nil
	case "normal":
		return LogLevelNormal, nil
	case "verbose":
		return LogLevelVerbose, nil
	default:
		return LogLevelNormal, fmt.Errorf("invalid log level: %s", s)
	}
}

func NewProgressReporter(ch chan<- ScanProgress, operation string) *ProgressReporter {
	return &ProgressReporter{
		ch:          ch,
		operation:   operation,
		startTime:   time.Now(),
		minInterval: 0,
		lastSent:    time.Time{},
		logLevel:    DefaultLogLevel,
	}
}

// NewProgressReporterWithInterval creates a new reporter with specified minimum interval between UI updates
func NewProgressReporterWithInterval(ch chan<- ScanProgress, operation string, minInterval time.Duration) *ProgressReporter {
	pr := NewProgressReporter(ch, operation)
	pr.minInterval = minInterval
	return pr
}

// SetLogLevel sets the log level filter
func (pr *ProgressReporter) SetLogLevel(level LogLevel) {
	pr.logLevel = level
}

// Start sends initial progress with total count and counting stage
func (pr *ProgressReporter) Start(total int, message string) {
	pr.total = total
	pr.StageUpdate("counting_files", message)
}

// Update sends progress update (scanning stage)
func (pr *ProgressReporter) Update(current int, message string) {
	pr.filesProcessed = current
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

// StageUpdate sends a message for a specific stage (counting_files, scanning, analyzing, complete)
func (pr *ProgressReporter) StageUpdate(stage, message string) {
	// Build stage-style progress update (no percentage change except for complete)
	progress := ScanProgress{
		Operation:         pr.operation,
		Stage:             stage,
		Current:           pr.filesProcessed,
		Total:             pr.total,
		Percentage:        pr.calculatePercentage(),
		Message:           message,
		Severity:          "info",
		StartTime:         pr.startTime,
		ElapsedSeconds:    int(time.Since(pr.startTime).Seconds()),
		FilesProcessed:    pr.filesProcessed,
		DuplicatesFound:   pr.duplicatesFound,
		ComplianceIssues:  pr.complianceIssues,
		ErrorsEncountered: pr.errorsEncountered,
		Errors:            pr.errors,
	}

	// Throttle like other updates
	if pr.minInterval > 0 {
		if time.Since(pr.lastSent) < pr.minInterval {
			return
		}
		pr.lastSent = time.Now()
	}

	pr.ch <- progress
}

// send helper for building and sending progress (info severity)
func (pr *ProgressReporter) send(current int, message string) {
	pr.sendSeverity(current, message, "info")
}

// sendSeverity builds and sends progress with the specified severity
// calculatePercentage returns current percentage to send
func (pr *ProgressReporter) calculatePercentage() float64 {
	if pr.total == 0 {
		return 0
	}
	return (float64(pr.filesProcessed) / float64(pr.total)) * 100.0
}

func (pr *ProgressReporter) sendSeverity(current int, message, severity string) {
	// Apply log level filtering
	if !pr.shouldSend(severity) {
		return
	}

	percentage := 0.0
	if pr.total > 0 {
		percentage = (float64(current) / float64(pr.total)) * 100.0
	}

	progress := ScanProgress{
		Operation:         pr.operation,
		Stage:             "scanning",
		Current:           current,
		Total:             pr.total,
		Percentage:        percentage,
		Message:           message,
		Severity:          severity,
		StartTime:         pr.startTime,
		ElapsedSeconds:    int(time.Since(pr.startTime).Seconds()),
		FilesProcessed:    pr.filesProcessed,
		DuplicatesFound:   pr.duplicatesFound,
		ComplianceIssues:  pr.complianceIssues,
		ErrorsEncountered: pr.errorsEncountered,
		Errors:            pr.errors,
	}

	// Throttle if minInterval is set
	if pr.minInterval > 0 {
		if time.Since(pr.lastSent) < pr.minInterval {
			// skip this update
			return
		}
		pr.lastSent = time.Now()
	}

	pr.ch <- progress
}

// shouldSend checks if message should be sent based on log level and severity
func (pr *ProgressReporter) shouldSend(severity string) bool {
	switch pr.logLevel {
	case LogLevelQuiet:
		return severity == "error" || severity == "critical"
	case LogLevelNormal:
		return severity != "debug"
	case LogLevelVerbose:
		return true
	default:
		return true
	}
}

// SendSeverityImmediate sends a message bypassing throttling and log level filtering
// Useful for critical errors that need immediate UI attention
func (pr *ProgressReporter) SendSeverityImmediate(severity, message string) {
	percentage := 0.0
	if pr.total > 0 {
		percentage = (float64(pr.filesProcessed) / float64(pr.total)) * 100.0
	}

	// Set alert flags for critical/error severities
	showAlert := severity == "critical" || severity == "error"
	alertType := ""
	if showAlert {
		alertType = severity
	}

	progress := ScanProgress{
		Operation:         pr.operation,
		Stage:             "scanning",
		Current:           pr.filesProcessed,
		Total:             pr.total,
		Percentage:        percentage,
		Message:           message,
		Severity:          severity,
		StartTime:         pr.startTime,
		ElapsedSeconds:    int(time.Since(pr.startTime).Seconds()),
		FilesProcessed:    pr.filesProcessed,
		DuplicatesFound:   pr.duplicatesFound,
		ComplianceIssues:  pr.complianceIssues,
		ErrorsEncountered: pr.errorsEncountered,
		Errors:            pr.errors,
		ShowAlert:         showAlert,
		AlertType:         alertType,
	}

	pr.ch <- progress
}

// LogError records an error and sends immediate error message (bypasses throttling)
func (pr *ProgressReporter) LogError(err error, message string) {
	fullMsg := message
	if err != nil {
		fullMsg = fmt.Sprintf("%s: %v", message, err)
	}
	pr.errorsEncountered++
	pr.errors = append(pr.errors, fullMsg)
	pr.SendSeverityImmediate("error", fullMsg)
}

// LogCritical records a critical error and sends immediate critical message
func (pr *ProgressReporter) LogCritical(err error, message string) {
	fullMsg := message
	if err != nil {
		fullMsg = fmt.Sprintf("%s: %v", message, err)
	}
	pr.errorsEncountered++
	pr.errors = append(pr.errors, fullMsg)
	pr.SendSeverityImmediate("critical", fullMsg)
}

// SetMinInterval sets the minimum interval between UI updates
func (pr *ProgressReporter) SetMinInterval(d time.Duration) {
	pr.minInterval = d
}

// Send sends a progress message with a specific severity (respects log level filter and throttling)
func (pr *ProgressReporter) Send(severity, message string) {
	pr.sendSeverity(pr.filesProcessed, message, severity)
}

// CountVideoFiles counts all video files in the given paths (for accurate progress)
func CountVideoFiles(paths []string) (int, error) {
	return CountVideoFilesWithProgress(paths, nil)
}

// CountVideoFilesWithProgress counts video files and reports progress
func CountVideoFilesWithProgress(paths []string, pr *ProgressReporter) (int, error) {
	count := 0
	directoriesScanned := 0

	for _, libPath := range paths {
		if _, err := os.Stat(libPath); err != nil {
			return 0, fmt.Errorf("library path not accessible: %s: %w", libPath, err)
		}

		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Report progress every 100 directories
			if info.IsDir() {
				directoriesScanned++
				if pr != nil && directoriesScanned%100 == 0 {
					pr.Send("info", fmt.Sprintf("Counting files... (%d found so far)", count))
				}
			}

			if !info.IsDir() && isVideoFile(path) {
				count++
				// Also report every 500 files found
				if pr != nil && count%500 == 0 {
					pr.Send("info", fmt.Sprintf("Counting files... (%d found so far)", count))
				}
			}

			return nil
		})

		if err != nil {
			return 0, fmt.Errorf("error counting files in %s: %w", libPath, err)
		}
	}

	return count, nil
}
