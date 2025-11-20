package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// CleanResult represents the result of a cleaning operation
type CleanResult struct {
	DuplicatesDeleted    int
	ComplianceFixed      int
	SpaceFreed           int64
	Errors               []error
	Operations           []Operation // For rollback capability
	DryRun               bool
}

// Operation represents a single filesystem operation
type Operation struct {
	Type        string    // "delete", "rename", "move"
	Source      string    // Original path
	Destination string    // New path (for rename/move)
	Timestamp   time.Time
	Completed   bool
}

// Config holds cleaner configuration
type Config struct {
	DryRun         bool
	MaxSizeGB      int64  // Maximum total size to delete in one operation
	ProtectedPaths []string
	LogPath        string // Path to operation log for rollback
}

// DefaultConfig returns safe default configuration
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		DryRun:    false,
		MaxSizeGB: 100, // 100GB limit
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

	result := CleanResult{
		DryRun:     config.DryRun,
		Operations: []Operation{},
		Errors:     []error{},
	}

	// Safety check: verify total size doesn't exceed limit
	totalSize := calculateTotalSize(duplicates, tvDuplicates)
	if totalSize > config.MaxSizeGB*1024*1024*1024 {
		return result, fmt.Errorf("total size to delete (%d GB) exceeds limit (%d GB)",
			totalSize/(1024*1024*1024), config.MaxSizeGB)
	}

	// Process duplicate deletions
	for _, dup := range duplicates {
		// Skip first file (keeper)
		for i := 1; i < len(dup.Files); i++ {
			file := dup.Files[i]

			// Safety check
			if isProtectedPath(file.Path, config.ProtectedPaths) {
				result.Errors = append(result.Errors,
					fmt.Errorf("refusing to delete protected path: %s", file.Path))
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
				} else {
					op.Completed = true
					result.DuplicatesDeleted++
					result.SpaceFreed += file.Size
				}
			} else {
				// Dry-run: just log the operation
				op.Completed = true
			}

			result.Operations = append(result.Operations, op)
		}
	}

	// Process TV duplicates
	for _, dup := range tvDuplicates {
		for i := 1; i < len(dup.Files); i++ {
			file := dup.Files[i]

			if isProtectedPath(file.Path, config.ProtectedPaths) {
				result.Errors = append(result.Errors,
					fmt.Errorf("refusing to delete protected path: %s", file.Path))
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
				} else {
					op.Completed = true
					result.DuplicatesDeleted++
					result.SpaceFreed += file.Size
				}
			} else {
				op.Completed = true
			}

			result.Operations = append(result.Operations, op)
		}
	}

	// Process compliance fixes (rename/reorganize)
	for _, issue := range compliance {
		if isProtectedPath(issue.Path, config.ProtectedPaths) {
			result.Errors = append(result.Errors,
				fmt.Errorf("refusing to modify protected path: %s", issue.Path))
			continue
		}

		var op Operation
		var err error

		switch issue.SuggestedAction {
		case "rename":
			op, err = performRename(issue.Path, issue.SuggestedPath, config.DryRun)
		case "reorganize":
			op, err = performReorganize(issue.Path, issue.SuggestedPath, config.DryRun)
		default:
			err = fmt.Errorf("unknown action: %s", issue.SuggestedAction)
		}

		if err != nil {
			result.Errors = append(result.Errors, err)
			op.Completed = false
		} else {
			if op.Completed {
				result.ComplianceFixed++
			}
		}

		result.Operations = append(result.Operations, op)
	}

	// Write operation log (for potential rollback)
	if !config.DryRun && len(result.Operations) > 0 {
		if err := writeOperationLog(result.Operations, config.LogPath); err != nil {
			result.Errors = append(result.Errors,
				fmt.Errorf("failed to write operation log: %w", err))
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
		if err := os.Rename(oldPath, newPath); err != nil {
			return op, fmt.Errorf("rename failed %s -> %s: %w", oldPath, newPath, err)
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
		// Create destination directory if it doesn't exist
		destDir := filepath.Dir(newPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return op, fmt.Errorf("failed to create directory %s: %w", destDir, err)
		}

		// Move the file
		if err := os.Rename(oldPath, newPath); err != nil {
			return op, fmt.Errorf("move failed %s -> %s: %w", oldPath, newPath, err)
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
