package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

const (
	// DefaultMaxSizeGB is the default maximum total size to delete in one operation (in GB)
	// Set to 3TB to accommodate users with large media libraries
	DefaultMaxSizeGB = 3000
)

// CleanResult represents the result of a cleaning operation
type CleanResult struct {
	DuplicatesDeleted int
	ComplianceFixed   int
	SpaceFreed        int64
	Errors            []error
	Operations        []Operation // For rollback capability
	DryRun            bool
}

// Operation represents a single filesystem operation
type Operation struct {
	Type        string // "delete", "rename", "move"
	Source      string // Original path
	Destination string // New path (for rename/move)
	Timestamp   time.Time
	Completed   bool
}

// Config holds cleaner configuration
type Config struct {
	DryRun         bool
	MaxSizeGB      int64 // Maximum total size to delete in one operation
	ProtectedPaths []string
	LogPath        string // Path to operation log for rollback
}

// DefaultConfig returns safe default configuration
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		DryRun:    false,
		MaxSizeGB: DefaultMaxSizeGB,
		ProtectedPaths: []string{
			// System directories
			"/usr", "/etc", "/bin", "/sbin", "/boot",
			"/sys", "/proc", "/dev", "/run",
			"/lib", "/lib32", "/lib64", "/libx32",
			"/var", "/opt", "/srv",
			// Root and system user homes
			"/root",
			// Windows system paths (for cross-platform safety)
			"C:\\Windows", "C:\\Program Files", "C:\\Program Files (x86)",
		},
		LogPath: filepath.Join(home, ".local/share/jellysink/operations.log"),
	}
}

// Clean performs both duplicate deletion AND compliance fixes
func Clean(duplicates []scanner.MovieDuplicate, tvDuplicates []scanner.TVDuplicate,
	compliance []scanner.ComplianceIssue, config Config) (CleanResult, error) {
	return CleanWithProgress(duplicates, tvDuplicates, compliance, config, nil)
}

// CleanWithProgress performs both duplicate deletion and compliance fixes,
// reporting progress to the provided channel. Progress messages include info and errors.
func CleanWithProgress(duplicates []scanner.MovieDuplicate, tvDuplicates []scanner.TVDuplicate,
	compliance []scanner.ComplianceIssue, config Config, progressCh chan<- scanner.ScanProgress) (CleanResult, error) {

	result := CleanResult{
		DryRun:     config.DryRun,
		Operations: []Operation{},
		Errors:     []error{},
	}

	// Create progress reporter
	var pr *scanner.ProgressReporter
	if progressCh != nil {
		pr = scanner.NewProgressReporterWithInterval(progressCh, "cleaning", 200*time.Millisecond)
	}

	// Calculate total operations (deletes + compliance fixes)
	totalOps := 0
	for _, dup := range duplicates {
		if len(dup.Files) > 1 {
			totalOps += len(dup.Files) - 1
		}
	}
	for _, dup := range tvDuplicates {
		if len(dup.Files) > 1 {
			totalOps += len(dup.Files) - 1
		}
	}
	totalOps += len(compliance)

	if pr != nil {
		pr.Start(totalOps, fmt.Sprintf("Preparing cleanup (%d operations)", totalOps))
	}

	// Safety check: verify total size doesn't exceed limit
	totalSize := calculateTotalSize(duplicates, tvDuplicates)
	if totalSize > config.MaxSizeGB*1024*1024*1024 {
		return result, fmt.Errorf("total size to delete (%d GB) exceeds limit (%d GB)",
			totalSize/(1024*1024*1024), config.MaxSizeGB)
	}

	processed := 0

	// Process duplicate deletions
	for _, dup := range duplicates {
		// Skip first file (keeper)
		for i := 1; i < len(dup.Files); i++ {
			file := dup.Files[i]

			// Safety check
			if isProtectedPath(file.Path, config.ProtectedPaths) {
				err := fmt.Errorf("refusing to delete protected path: %s", file.Path)
				result.Errors = append(result.Errors, err)
				if pr != nil {
					pr.LogError(err, err.Error())
				}
				continue
			}

			op := Operation{
				Type:      "delete",
				Source:    file.Path,
				Timestamp: time.Now(),
			}

			if !config.DryRun {
				if err := os.Remove(file.Path); err != nil {
					result.Errors = append(result.Errors,
						fmt.Errorf("failed to delete %s: %w", file.Path, err))
					op.Completed = false
					if pr != nil {
						pr.LogError(err, fmt.Sprintf("Failed to delete: %s", file.Path))
					}
				} else {
					op.Completed = true
					result.DuplicatesDeleted++
					result.SpaceFreed += file.Size
					if pr != nil {
						pr.Update(processed+1, fmt.Sprintf("Deleted: %s", file.Path))
					}

				}
			} else {
				op.Completed = true
				if pr != nil {
					pr.Update(processed+1, fmt.Sprintf("Dry-run: delete %s", file.Path))
				}
			}

			result.Operations = append(result.Operations, op)
			processed++
			if pr != nil {
				pr.Update(processed, fmt.Sprintf("Processed %d/%d", processed, totalOps))
			}
		}
	}

	// Process TV duplicates
	for _, dup := range tvDuplicates {
		for i := 1; i < len(dup.Files); i++ {
			file := dup.Files[i]

			if isProtectedPath(file.Path, config.ProtectedPaths) {
				err := fmt.Errorf("refusing to delete protected path: %s", file.Path)
				result.Errors = append(result.Errors, err)
				if pr != nil {
					pr.LogError(err, err.Error())
				}
				continue
			}

			op := Operation{
				Type:      "delete",
				Source:    file.Path,
				Timestamp: time.Now(),
			}

			if !config.DryRun {
				if err := os.Remove(file.Path); err != nil {
					result.Errors = append(result.Errors,
						fmt.Errorf("failed to delete %s: %w", file.Path, err))
					op.Completed = false
					if pr != nil {
						pr.LogError(err, fmt.Sprintf("Failed to delete: %s", file.Path))
					}
				} else {
					op.Completed = true
					result.DuplicatesDeleted++
					result.SpaceFreed += file.Size
					if pr != nil {
						pr.Update(processed+1, fmt.Sprintf("Deleted: %s", file.Path))
					}

				}
			} else {
				op.Completed = true
				if pr != nil {
					pr.Update(processed+1, fmt.Sprintf("Dry-run: delete %s", file.Path))
				}
			}

			result.Operations = append(result.Operations, op)
			processed++
			if pr != nil {
				pr.Update(processed, fmt.Sprintf("Processed %d/%d", processed, totalOps))
			}
		}
	}

	// Process compliance fixes using scanner's Apply functions
	for i, issue := range compliance {
		// Skip manual review items (collisions, sample files, etc.)
		if issue.SuggestedAction == "manual_review" {
			err := fmt.Errorf("skipped (needs manual review): %s - %s", issue.Path, issue.Problem)
			result.Errors = append(result.Errors, err)
			if pr != nil {
				pr.LogError(err, err.Error())
			}
			continue
		}

		if isProtectedPath(issue.Path, config.ProtectedPaths) {
			err := fmt.Errorf("refusing to modify protected path: %s", issue.Path)
			result.Errors = append(result.Errors, err)
			if pr != nil {
				pr.LogError(err, err.Error())
			}
			continue
		}

		var op Operation
		var err error

		// Use scanner's Apply functions which handle folder detection
		if !config.DryRun {
			// Progress indicator
			if pr != nil && len(compliance) > 5 && i%5 == 0 {
				pr.StageUpdate("fixing", fmt.Sprintf("Fixing compliance issues: %d/%d", i, len(compliance)))
			} else if len(compliance) > 5 && i%5 == 0 {
				fmt.Printf("Fixing compliance issues: %d/%d\n", i, len(compliance))
			}

			if issue.Type == "movie" {
				err = scanner.ApplyMovieComplianceWithReporter(issue, pr)
			} else if issue.Type == "tv" {
				err = scanner.ApplyTVComplianceWithReporter(issue, pr)
			} else {
				err = fmt.Errorf("unknown issue type: %s", issue.Type)
			}
		}

		// Build operation record
		op = Operation{
			Type:        issue.SuggestedAction,
			Source:      issue.Path,
			Destination: issue.SuggestedPath,
			Timestamp:   time.Now(),
		}

		if err != nil {
			result.Errors = append(result.Errors, err)
			op.Completed = false
			if pr != nil {
				pr.LogError(err, fmt.Sprintf("Failed to apply compliance: %s", issue.Path))
			}
		} else {
			op.Completed = true
			result.ComplianceFixed++
			if pr != nil {
				pr.Update(processed+1, fmt.Sprintf("Fixed compliance: %s", issue.Path))
			}
		}

		result.Operations = append(result.Operations, op)
		processed++
		if pr != nil {
			pr.Update(processed, fmt.Sprintf("Processed %d/%d", processed, totalOps))
		}
	}

	// Final progress message
	if !config.DryRun && len(compliance) > 0 {
		fmt.Printf("Fixed %d compliance issues\n", result.ComplianceFixed)
	}

	if pr != nil {
		pr.Complete(fmt.Sprintf("Finished cleanup: %d deleted, %d fixed", result.DuplicatesDeleted, result.ComplianceFixed))
	}

	// Write operation log (for potential rollback)
	if !config.DryRun && len(result.Operations) > 0 {
		if err := writeOperationLog(result.Operations, config.LogPath); err != nil {
			result.Errors = append(result.Errors,
				fmt.Errorf("failed to write operation log: %w", err))
			if pr != nil {
				pr.LogError(err, "failed to write operation log")
			}
		}
	}

	return result, nil
}

// validatePath sanitizes and validates a file path for safety
func validatePath(path string) error {
	// Clean the path (removes .., redundant slashes, etc.)
	cleaned := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("invalid path: contains path traversal (..) sequence")
	}

	// Ensure path is absolute for safety
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("invalid path: must be absolute path")
	}

	return nil
}

// getFileOwnership returns the UID and GID of a file
// This is critical when running as root to preserve original ownership
func getFileOwnership(path string) (int, int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to stat file: %w", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		// On non-Unix systems, return 0,0 (no-op)
		return 0, 0, nil
	}

	return int(stat.Uid), int(stat.Gid), nil
}

// preserveOwnership restores file ownership after operations
// When running as root/sudo, this prevents files from being owned by root
func preserveOwnership(path string, uid, gid int) error {
	// Skip if UID/GID are both 0 (root or unsupported platform)
	if uid == 0 && gid == 0 {
		return nil
	}

	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("failed to restore ownership: %w", err)
	}

	return nil
}

// performRename renames a file in place (same directory)
func performRename(oldPath, newPath string, dryRun bool) (Operation, error) {
	// Validate paths for security
	if err := validatePath(oldPath); err != nil {
		return Operation{}, fmt.Errorf("invalid source path: %w", err)
	}
	if err := validatePath(newPath); err != nil {
		return Operation{}, fmt.Errorf("invalid destination path: %w", err)
	}

	op := Operation{
		Type:        "rename",
		Source:      oldPath,
		Destination: newPath,
		Timestamp:   time.Now(),
	}

	if !dryRun {
		// Get original ownership before rename (critical for sudo operations)
		uid, gid, err := getFileOwnership(oldPath)
		if err != nil {
			return op, fmt.Errorf("failed to get ownership: %w", err)
		}

		// Perform rename
		if err := os.Rename(oldPath, newPath); err != nil {
			return op, fmt.Errorf("rename failed %s -> %s: %w", oldPath, newPath, err)
		}

		// Restore original ownership to prevent root takeover
		if err := preserveOwnership(newPath, uid, gid); err != nil {
			return op, fmt.Errorf("rename succeeded but ownership restore failed: %w", err)
		}

		op.Completed = true
	} else {
		op.Completed = true
	}

	return op, nil
}

// performReorganize moves file to new directory structure (creates dirs as needed)
func performReorganize(oldPath, newPath string, dryRun bool) (Operation, error) {
	// Validate paths for security
	if err := validatePath(oldPath); err != nil {
		return Operation{}, fmt.Errorf("invalid source path: %w", err)
	}
	if err := validatePath(newPath); err != nil {
		return Operation{}, fmt.Errorf("invalid destination path: %w", err)
	}

	op := Operation{
		Type:        "move",
		Source:      oldPath,
		Destination: newPath,
		Timestamp:   time.Now(),
	}

	if !dryRun {
		// Get original ownership before move (critical for sudo operations)
		uid, gid, err := getFileOwnership(oldPath)
		if err != nil {
			return op, fmt.Errorf("failed to get ownership: %w", err)
		}

		// Get parent directory ownership to preserve it for new dirs
		parentDir := filepath.Dir(oldPath)
		parentUID, parentGID, err := getFileOwnership(parentDir)
		if err != nil {
			// If we can't get parent ownership, use file ownership
			parentUID, parentGID = uid, gid
		}

		// Create destination directory if it doesn't exist
		destDir := filepath.Dir(newPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return op, fmt.Errorf("failed to create directory %s: %w", destDir, err)
		}

		// Preserve ownership on newly created directories
		if err := preserveOwnership(destDir, parentUID, parentGID); err != nil {
			// Non-fatal - log but continue
		}

		// Move the file
		if err := os.Rename(oldPath, newPath); err != nil {
			return op, fmt.Errorf("move failed %s -> %s: %w", oldPath, newPath, err)
		}

		// Restore original ownership to prevent root takeover
		if err := preserveOwnership(newPath, uid, gid); err != nil {
			return op, fmt.Errorf("move succeeded but ownership restore failed: %w", err)
		}

		// Clean up empty parent directories
		cleanupEmptyDirs(filepath.Dir(oldPath))

		op.Completed = true
	} else {
		op.Completed = true
	}

	return op, nil
}

// cleanupEmptyDirs removes empty directories after moving files
func cleanupEmptyDirs(dir string) {
	// Check if directory is empty
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}

	// Remove empty directory
	os.Remove(dir)

	// Recursively check parent
	parent := filepath.Dir(dir)
	if parent != dir && parent != "/" && parent != "." {
		cleanupEmptyDirs(parent)
	}
}

// calculateTotalSize calculates total bytes to be deleted
func calculateTotalSize(movies []scanner.MovieDuplicate, tv []scanner.TVDuplicate) int64 {
	var total int64

	for _, dup := range movies {
		for i := 1; i < len(dup.Files); i++ {
			total += dup.Files[i].Size
		}
	}

	for _, dup := range tv {
		for i := 1; i < len(dup.Files); i++ {
			total += dup.Files[i].Size
		}
	}

	return total
}

// isProtectedPath checks if path is in protected list
func isProtectedPath(path string, protected []string) bool {
	for _, p := range protected {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// writeOperationLog writes operations to log file for rollback capability
func writeOperationLog(ops []Operation, logPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}

	// Open log file (append mode) with user-only permissions for security
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write operations
	for _, op := range ops {
		if !op.Completed {
			continue
		}

		line := fmt.Sprintf("%s|%s|%s|%s\n",
			op.Timestamp.Format(time.RFC3339),
			op.Type,
			op.Source,
			op.Destination)

		if _, err := f.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

// CleanDuplicatesOnly performs only duplicate deletion (no compliance fixes)
// Used for targeted cleanup operations
func CleanDuplicatesOnly(duplicates []scanner.MovieDuplicate, tvDuplicates []scanner.TVDuplicate,
	config Config) (CleanResult, error) {

	return Clean(duplicates, tvDuplicates, []scanner.ComplianceIssue{}, config)
}

// FixComplianceOnly performs only compliance fixes (no duplicate deletion)
// Used for targeted compliance operations
func FixComplianceOnly(compliance []scanner.ComplianceIssue, config Config) (CleanResult, error) {
	return Clean([]scanner.MovieDuplicate{}, []scanner.TVDuplicate{}, compliance, config)
}
