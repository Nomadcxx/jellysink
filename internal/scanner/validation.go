package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PathValidationResult struct {
	Path       string
	Accessible bool
	IsDir      bool
	Readable   bool
	Writable   bool
	Error      error
	VideoCount int
	SizeBytes  int64
}

type ValidationReport struct {
	TotalPaths        int
	AccessiblePaths   int
	InaccessiblePaths []PathValidationResult
	Warnings          []string
	CanProceed        bool
	ErrorMessage      string
}

func ValidateLibraryPaths(paths []string, requireWritable bool) (*ValidationReport, error) {
	report := &ValidationReport{
		TotalPaths:        len(paths),
		InaccessiblePaths: []PathValidationResult{},
		Warnings:          []string{},
		CanProceed:        false,
	}

	if len(paths) == 0 {
		report.ErrorMessage = "no library paths provided"
		return report, fmt.Errorf("no library paths provided")
	}

	for _, path := range paths {
		result := validateSinglePath(path, requireWritable)

		if result.Accessible {
			report.AccessiblePaths++

			if result.VideoCount == 0 {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("Path contains no video files: %s", path))
			}
		} else {
			report.InaccessiblePaths = append(report.InaccessiblePaths, result)
		}
	}

	if report.AccessiblePaths == 0 {
		report.ErrorMessage = fmt.Sprintf("no accessible paths found (checked %d paths)", len(paths))
		return report, fmt.Errorf("no accessible paths found (checked %d paths)", len(paths))
	}

	report.CanProceed = true
	return report, nil
}

func validateSinglePath(path string, requireWritable bool) PathValidationResult {
	result := PathValidationResult{
		Path:       path,
		Accessible: false,
		IsDir:      false,
		Readable:   false,
		Writable:   false,
	}

	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve symlinks: %w", err)
		return result
	}

	if realPath != path {
		result.Path = realPath
	}

	info, err := os.Stat(realPath)
	if err != nil {
		result.Error = err
		return result
	}

	if !info.IsDir() {
		result.Error = fmt.Errorf("path is not a directory")
		return result
	}
	result.IsDir = true

	readable := checkReadable(realPath)
	result.Readable = readable
	if !readable {
		result.Error = fmt.Errorf("path is not readable")
		return result
	}

	if requireWritable {
		writable := checkWritable(realPath)
		result.Writable = writable
		if !writable {
			result.Error = fmt.Errorf("path is not writable (required for this operation)")
			return result
		}
	}

	videoCount, totalSize := quickCountVideos(realPath)
	result.VideoCount = videoCount
	result.SizeBytes = totalSize
	result.Accessible = true

	return result
}

func checkReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	_, err = file.Readdirnames(1)
	return err == nil || err.Error() == "EOF"
}

func checkWritable(path string) bool {
	testFile := filepath.Join(path, ".jellysink_write_test")
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	os.Remove(testFile)
	return true
}

func quickCountVideos(path string) (int, int64) {
	count := 0
	var totalSize int64
	maxCount := 10

	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil || count >= maxCount {
			return filepath.SkipDir
		}

		if !info.IsDir() && isVideoFile(p) {
			count++
			totalSize += info.Size()
		}

		return nil
	})

	return count, totalSize
}

func ValidateBeforeScan(paths []string, operation string, pr *ProgressReporter) error {
	if pr != nil {
		pr.Send("info", fmt.Sprintf("Validating %d library paths for %s...", len(paths), operation))
	}

	requireWritable := strings.Contains(operation, "rename") ||
		strings.Contains(operation, "organize") ||
		strings.Contains(operation, "delete")

	report, err := ValidateLibraryPaths(paths, requireWritable)

	if pr != nil {
		if report.AccessiblePaths > 0 {
			pr.Send("info", fmt.Sprintf("✓ %d/%d paths accessible", report.AccessiblePaths, report.TotalPaths))
		}

		for _, inaccessible := range report.InaccessiblePaths {
			pr.Send("warn", fmt.Sprintf("✗ Skipping %s: %v", inaccessible.Path, inaccessible.Error))
		}

		for _, warning := range report.Warnings {
			pr.Send("warn", warning)
		}

		if !report.CanProceed {
			pr.LogCritical(err, "Pre-scan validation failed")
			return err
		}
	}

	return nil
}

func ValidatePathDepth(path, operation string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	cleanPath := filepath.Clean(path)

	realPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		realPath = cleanPath
	}

	protectedPaths := []string{"/", "/mnt", "/home", "/usr", "/etc", "/var", "/tmp", "/opt"}
	for _, protected := range protectedPaths {
		if realPath == protected || cleanPath == protected {
			return fmt.Errorf("refusing to %s on protected path: %s", operation, realPath)
		}
	}

	parts := strings.Split(strings.TrimPrefix(realPath, "/"), "/")
	if len(parts) < 3 {
		return fmt.Errorf("path too shallow for safe %s (minimum 3 levels deep): %s", operation, realPath)
	}

	return nil
}

func ValidatePathInLibrary(targetPath string, libraryPaths []string) error {
	cleanTarget := filepath.Clean(targetPath)

	realTarget, err := filepath.EvalSymlinks(cleanTarget)
	if err != nil {
		realTarget = cleanTarget
	}

	for _, libPath := range libraryPaths {
		cleanLib := filepath.Clean(libPath)

		realLib, err := filepath.EvalSymlinks(cleanLib)
		if err != nil {
			realLib = cleanLib
		}

		if strings.HasPrefix(realTarget, realLib) {
			return nil
		}

		if strings.HasPrefix(cleanTarget, cleanLib) {
			return nil
		}
	}

	return fmt.Errorf("path %s is not within any configured library path", targetPath)
}
