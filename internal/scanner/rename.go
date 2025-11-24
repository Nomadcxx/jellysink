package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RenameResult tracks a single rename operation
type RenameResult struct {
	OldPath  string
	NewPath  string
	IsFolder bool
	Success  bool
	Error    string
}

// RenamePreview provides details about what would be renamed
type RenamePreview struct {
	MatchCount           int
	MatchingFolders      []string
	TargetPaths          []string
	EpisodeCounts        []int
	TotalEpisodes        int
	CollisionWarnings    []string
	EmptyFolderWarnings  []string
	DuplicateTargetPaths map[string][]string
	CanProceed           bool
	ErrorMessage         string
}

// PreviewTVRename scans for matching folders and provides a preview of what would be renamed
func PreviewTVRename(basePath, oldTitle, newTitle string) (*RenamePreview, error) {
	preview := &RenamePreview{
		MatchingFolders:      []string{},
		TargetPaths:          []string{},
		EpisodeCounts:        []int{},
		CollisionWarnings:    []string{},
		EmptyFolderWarnings:  []string{},
		DuplicateTargetPaths: make(map[string][]string),
		CanProceed:           false,
	}

	// Same validation as ApplyManualTVRename
	if newTitle == "" {
		preview.ErrorMessage = "new title cannot be empty"
		return preview, fmt.Errorf("new title cannot be empty")
	}

	if strings.ContainsAny(newTitle, `<>:"/\|?*`) {
		preview.ErrorMessage = "new title contains invalid characters"
		return preview, fmt.Errorf("new title contains invalid characters")
	}

	normalizedOld := strings.ToLower(strings.TrimSpace(oldTitle))
	normalizedNew := strings.ToLower(strings.TrimSpace(newTitle))

	if normalizedOld == normalizedNew {
		preview.ErrorMessage = "old and new titles are the same"
		return preview, fmt.Errorf("old and new titles are the same")
	}

	// Validate basePath (abbreviated version)
	if basePath == "" {
		preview.ErrorMessage = "basePath cannot be empty"
		return preview, fmt.Errorf("basePath cannot be empty")
	}

	basePath = filepath.Clean(basePath)
	realBasePath, err := filepath.EvalSymlinks(basePath)
	if err != nil {
		realBasePath = basePath
	}

	if err := ValidatePathDepth(realBasePath, "rename"); err != nil {
		preview.ErrorMessage = fmt.Sprintf("path validation failed: %v", err)
		return preview, err
	}

	info, err := os.Stat(realBasePath)
	if err != nil {
		preview.ErrorMessage = fmt.Sprintf("basePath does not exist: %s", realBasePath)
		return preview, fmt.Errorf("basePath does not exist: %s: %w", realBasePath, err)
	}
	if !info.IsDir() {
		preview.ErrorMessage = fmt.Sprintf("basePath is not a directory: %s", realBasePath)
		return preview, fmt.Errorf("basePath is not a directory: %s", realBasePath)
	}

	basePath = realBasePath

	// Scan for matching folders
	entries, err := os.ReadDir(basePath)
	if err != nil {
		preview.ErrorMessage = fmt.Sprintf("failed to read directory: %v", err)
		return preview, fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}

	tvTitlePattern := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)$`)
	targetPathCount := make(map[string][]string)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		path := filepath.Join(basePath, dirName)

		matches := tvTitlePattern.FindStringSubmatch(dirName)
		if len(matches) != 3 {
			continue
		}

		folderTitle := matches[1]
		year := matches[2]
		normalizedFolderTitle := strings.ToLower(strings.TrimSpace(folderTitle))

		// Only match folders that match oldTitle exactly
		if normalizedFolderTitle != normalizedOld {
			continue
		}

		// Found a match!
		preview.MatchCount++
		preview.MatchingFolders = append(preview.MatchingFolders, path)

		// Count episodes in this folder
		episodeCount := countEpisodesInFolder(path)
		preview.EpisodeCounts = append(preview.EpisodeCounts, episodeCount)
		preview.TotalEpisodes += episodeCount

		if episodeCount == 0 {
			preview.EmptyFolderWarnings = append(preview.EmptyFolderWarnings,
				fmt.Sprintf("Folder contains no episode files: %s", path))
		}

		// Compute target path
		newFolderName := fmt.Sprintf("%s (%s)", newTitle, year)
		newFolderPath := filepath.Join(basePath, newFolderName)
		preview.TargetPaths = append(preview.TargetPaths, newFolderPath)

		// Check if target already exists
		if _, err := os.Stat(newFolderPath); err == nil {
			if newFolderPath != path {
				preview.CollisionWarnings = append(preview.CollisionWarnings,
					fmt.Sprintf("Target path already exists: %s", newFolderPath))
			}
		}

		// Track duplicate target paths
		targetPathCount[newFolderPath] = append(targetPathCount[newFolderPath], path)
	}

	if preview.MatchCount == 0 {
		preview.ErrorMessage = fmt.Sprintf("no folders matching '%s' found in %s", oldTitle, basePath)
		return preview, fmt.Errorf(preview.ErrorMessage)
	}

	// Check for duplicate target paths (multiple sources renaming to same destination)
	for targetPath, sourcePaths := range targetPathCount {
		if len(sourcePaths) > 1 {
			preview.DuplicateTargetPaths[targetPath] = sourcePaths
			preview.CollisionWarnings = append(preview.CollisionWarnings,
				fmt.Sprintf("Multiple folders would rename to same path: %s", targetPath))
		}
	}

	// Determine if operation can safely proceed
	preview.CanProceed = true
	if len(preview.CollisionWarnings) > 0 {
		preview.CanProceed = false
		preview.ErrorMessage = "Collision detected - cannot proceed"
	}

	return preview, nil
}

// countEpisodesInFolder counts episode files (*.mkv, *.mp4, etc) in a folder recursively
func countEpisodesInFolder(folderPath string) int {
	count := 0
	episodePattern := regexp.MustCompile(`(?i)S\d{2}E\d{2}`)

	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && isVideoFile(path) && episodePattern.MatchString(filepath.Base(path)) {
			count++
		}
		return nil
	})

	return count
}

// ApplyManualTVRename renames folders and episode files for a TV show
func ApplyManualTVRename(basePath, oldTitle, newTitle string, dryRun bool) ([]RenameResult, error) {
	return ApplyManualTVRenameWithProgress(basePath, oldTitle, newTitle, dryRun, nil)
}

// ApplyManualTVRenameWithProgress renames folders and episode files for a TV show with progress reporting
func ApplyManualTVRenameWithProgress(basePath, oldTitle, newTitle string, dryRun bool, pr *ProgressReporter) ([]RenameResult, error) {
	var results []RenameResult

	// Create backup snapshot to track rename operations
	var snapshot *BackupSnapshot
	if !dryRun {
		backupID := fmt.Sprintf("rename_%s", time.Now().Format("20060102_150405"))
		snapshot = &BackupSnapshot{
			Metadata: &BackupMetadata{
				BackupID:     backupID,
				CreatedAt:    time.Now(),
				LibraryType:  "tv",
				LibraryPaths: []string{basePath},
				Entries:      []FileEntry{},
				Operations:   []BackupOperation{},
				Status:       "in_progress",
			},
		}

		// Set backup file path
		backupDir, err := GetBackupDir()
		if err == nil {
			snapshot.FilePath = filepath.Join(backupDir, backupID+".json")
		}
	}

	// Ensure snapshot is saved at the end
	defer func() {
		if snapshot != nil {
			snapshot.Metadata.Status = "completed"
			if err := snapshot.Save(); err != nil && pr != nil {
				pr.Send("warn", fmt.Sprintf("Failed to save rename log: %v", err))
			}
		}
	}()

	if pr != nil {
		pr.Update(0, fmt.Sprintf("Starting rename: %s -> %s", oldTitle, newTitle))
	}

	if newTitle == "" {
		if pr != nil {
			pr.LogError(fmt.Errorf("new title cannot be empty"), "Invalid new title")
		}
		return results, fmt.Errorf("new title cannot be empty")
	}

	if strings.ContainsAny(newTitle, `<>:"/\|?*`) {
		if pr != nil {
			pr.LogError(fmt.Errorf("new title contains invalid characters"), "Invalid characters in new title")
		}
		return results, fmt.Errorf("new title contains invalid characters")
	}

	normalizedOld := strings.ToLower(strings.TrimSpace(oldTitle))
	normalizedNew := strings.ToLower(strings.TrimSpace(newTitle))

	if normalizedOld == normalizedNew {
		if pr != nil {
			pr.LogError(fmt.Errorf("old and new titles are the same"), "Titles are identical")
		}
		return results, fmt.Errorf("old and new titles are the same")
	}

	// CRITICAL SAFETY CHECK: Comprehensive validation pipeline
	if basePath == "" {
		if pr != nil {
			pr.LogError(fmt.Errorf("basePath is empty"), "Invalid base path")
		}
		return results, fmt.Errorf("basePath cannot be empty")
	}

	// Step 1: Clean and resolve symlinks to prevent symlink tricks
	basePath = filepath.Clean(basePath)
	realBasePath, err := filepath.EvalSymlinks(basePath)
	if err != nil {
		// If symlink resolution fails, use cleaned path but log warning
		if pr != nil {
			pr.Send("warn", fmt.Sprintf("Could not resolve symlinks for %s: %v", basePath, err))
		}
		realBasePath = basePath
	}

	// Step 2: Use existing ValidatePathDepth function (checks protected paths + depth)
	if err := ValidatePathDepth(realBasePath, "rename"); err != nil {
		if pr != nil {
			pr.LogError(err, "SAFETY CHECK FAILED: Path depth validation")
		}
		return results, err
	}

	// Step 3: Verify basePath exists and is a directory
	info, err := os.Stat(realBasePath)
	if err != nil {
		if pr != nil {
			pr.LogError(err, fmt.Sprintf("basePath does not exist: %s", realBasePath))
		}
		return results, fmt.Errorf("basePath does not exist: %s: %w", realBasePath, err)
	}
	if !info.IsDir() {
		err := fmt.Errorf("basePath is not a directory: %s", realBasePath)
		if pr != nil {
			pr.LogError(err, "Invalid base path")
		}
		return results, err
	}

	// Step 4: Check writability (required for rename operations)
	report, err := ValidateLibraryPaths([]string{realBasePath}, true)
	if err != nil || !report.CanProceed {
		errMsg := fmt.Sprintf("library validation failed: %v", err)
		if err == nil && !report.CanProceed {
			errMsg = fmt.Sprintf("library validation failed: %s", report.ErrorMessage)
		}
		if pr != nil {
			pr.LogError(fmt.Errorf(errMsg), "SAFETY CHECK FAILED: Library validation")
		}
		return results, fmt.Errorf(errMsg)
	}

	// Use realBasePath for all subsequent operations
	basePath = realBasePath

	if pr != nil {
		pr.Update(10, "Scanning directories")
	}

	// CRITICAL FIX: Only scan immediate children of basePath (depth 1), not recursive
	// This prevents renaming unrelated shows in the same library
	entries, err := os.ReadDir(basePath)
	if err != nil {
		if pr != nil {
			pr.LogError(err, fmt.Sprintf("Failed to read directory: %s", basePath))
		}
		return results, fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}

	tvTitlePattern := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)$`)
	matchCount := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		path := filepath.Join(basePath, dirName)

		matches := tvTitlePattern.FindStringSubmatch(dirName)
		if len(matches) != 3 {
			continue
		}

		folderTitle := matches[1]
		year := matches[2]
		normalizedFolderTitle := strings.ToLower(strings.TrimSpace(folderTitle))

		// Only rename folders that match oldTitle exactly
		if normalizedFolderTitle != normalizedOld {
			continue
		}

		matchCount++
		newFolderName := fmt.Sprintf("%s (%s)", newTitle, year)
		newFolderPath := filepath.Join(basePath, newFolderName)

		// Check if target path already exists (and is not the same as source)
		if _, err := os.Stat(newFolderPath); err == nil && newFolderPath != path {
			err := fmt.Errorf("target path already exists: %s", newFolderPath)
			if pr != nil {
				pr.LogError(err, "Destination collision detected")
			}
			results = append(results, RenameResult{
				OldPath:  path,
				NewPath:  newFolderPath,
				IsFolder: true,
				Success:  false,
				Error:    "target path already exists",
			})
			continue
		}

		if pr != nil {
			pr.Update(50, fmt.Sprintf("Renaming episodes in: %s", dirName))
		}

		// Rename episodes inside this folder
		episodeResults, err := renameEpisodesInFolderWithProgress(path, oldTitle, newTitle, dryRun, snapshot, pr)
		if err != nil {
			if pr != nil {
				pr.LogError(err, fmt.Sprintf("Failed to rename episodes in: %s", dirName))
			}
			results = append(results, RenameResult{
				OldPath:  path,
				NewPath:  newFolderPath,
				IsFolder: true,
				Success:  false,
				Error:    fmt.Sprintf("failed to rename episodes: %v", err),
			})
			continue
		}
		results = append(results, episodeResults...)

		if pr != nil {
			pr.Update(90, fmt.Sprintf("Renaming folder: %s", dirName))
		}

		// Rename the folder itself
		if !dryRun {
			if err := os.Rename(path, newFolderPath); err != nil {
				if pr != nil {
					pr.LogError(err, fmt.Sprintf("Failed to rename folder: %s", dirName))
				}
				results = append(results, RenameResult{
					OldPath:  path,
					NewPath:  newFolderPath,
					IsFolder: true,
					Success:  false,
					Error:    err.Error(),
				})
				// Record failed operation
				if snapshot != nil {
					snapshot.RecordOperation("rename", path, newFolderPath, false, err)
				}
				continue
			}
			// Record successful folder rename
			if snapshot != nil {
				snapshot.RecordOperation("rename", path, newFolderPath, true, nil)
			}
		}

		results = append(results, RenameResult{
			OldPath:  path,
			NewPath:  newFolderPath,
			IsFolder: true,
			Success:  true,
		})
	}

	if matchCount == 0 {
		err := fmt.Errorf("no folders matching '%s' found in %s", oldTitle, basePath)
		if pr != nil {
			pr.LogError(err, "No matching folders")
		}
		return results, err
	}

	if pr != nil {
		pr.Complete(fmt.Sprintf("Rename complete: %d operations", len(results)))
	}

	return results, nil
}

// renameEpisodesInFolder renames all episode files inside a folder
func renameEpisodesInFolder(folderPath, oldTitle, newTitle string, dryRun bool) ([]RenameResult, error) {
	return renameEpisodesInFolderWithProgress(folderPath, oldTitle, newTitle, dryRun, nil, nil)
}

// renameEpisodesInFolderWithProgress renames all episode files inside a folder with progress reporting
func renameEpisodesInFolderWithProgress(folderPath, oldTitle, newTitle string, dryRun bool, snapshot *BackupSnapshot, pr *ProgressReporter) ([]RenameResult, error) {
	var results []RenameResult

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if pr != nil {
				pr.LogError(err, fmt.Sprintf("Failed to access: %s", path))
			}
			return err
		}

		if info.IsDir() {
			return nil
		}

		fileName := filepath.Base(path)
		ext := filepath.Ext(fileName)
		nameWithoutExt := strings.TrimSuffix(fileName, ext)

		episodePattern := regexp.MustCompile(`(?i)(S\d{2}E\d{2})`)
		if !episodePattern.MatchString(nameWithoutExt) {
			return nil
		}

		normalizedFileName := strings.ToLower(nameWithoutExt)
		normalizedOld := strings.ToLower(oldTitle)

		if strings.Contains(normalizedFileName, normalizedOld) {
			newFileName := strings.Replace(nameWithoutExt, oldTitle, newTitle, 1)

			if strings.ToLower(nameWithoutExt) != strings.ToLower(newFileName) {
				newFileName = nameWithoutExt
				parts := episodePattern.Split(nameWithoutExt, -1)
				episodeCode := episodePattern.FindString(nameWithoutExt)

				if len(parts) > 0 && episodeCode != "" {
					suffix := ""
					if len(parts) > 1 {
						suffix = parts[1]
					}
					newFileName = newTitle + " " + episodeCode + suffix
				}
			}

			newFileName = newFileName + ext
			newPath := filepath.Join(filepath.Dir(path), newFileName)

			if !dryRun {
				if err := os.Rename(path, newPath); err != nil {
					if pr != nil {
						pr.LogError(err, fmt.Sprintf("Failed to rename: %s", fileName))
					}
					results = append(results, RenameResult{
						OldPath:  path,
						NewPath:  newPath,
						IsFolder: false,
						Success:  false,
						Error:    err.Error(),
					})
					// Record failed episode rename
					if snapshot != nil {
						snapshot.RecordOperation("rename", path, newPath, false, err)
					}
					return nil
				}
				// Record successful episode rename
				if snapshot != nil {
					snapshot.RecordOperation("rename", path, newPath, true, nil)
				}
			}

			results = append(results, RenameResult{
				OldPath:  path,
				NewPath:  newPath,
				IsFolder: false,
				Success:  true,
			})
		}

		return nil
	})

	return results, err
}

// ValidateTVShowTitle checks if a title is valid for use
func ValidateTVShowTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("title cannot be empty")
	}

	if strings.ContainsAny(title, `<>:"/\|?*`) {
		return fmt.Errorf("title contains invalid characters: < > : \" / \\ | ? *")
	}

	if len(title) > 200 {
		return fmt.Errorf("title is too long (max 200 characters)")
	}

	return nil
}
