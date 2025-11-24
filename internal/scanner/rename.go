package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RenameResult tracks a single rename operation
type RenameResult struct {
	OldPath  string
	NewPath  string
	IsFolder bool
	Success  bool
	Error    string
}

// ApplyManualTVRename renames folders and episode files for a TV show
func ApplyManualTVRename(basePath, oldTitle, newTitle string, dryRun bool) ([]RenameResult, error) {
	return ApplyManualTVRenameWithProgress(basePath, oldTitle, newTitle, dryRun, nil)
}

// ApplyManualTVRenameWithProgress renames folders and episode files for a TV show with progress reporting
func ApplyManualTVRenameWithProgress(basePath, oldTitle, newTitle string, dryRun bool, pr *ProgressReporter) ([]RenameResult, error) {
	var results []RenameResult

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

	normalizedOld := strings.ToLower(NormalizeName(oldTitle))
	normalizedNew := strings.ToLower(NormalizeName(newTitle))

	if normalizedOld == normalizedNew {
		if pr != nil {
			pr.LogError(fmt.Errorf("old and new titles are the same"), "Titles are identical")
		}
		return results, fmt.Errorf("old and new titles are the same")
	}

	if pr != nil {
		pr.Update(10, "Scanning directories")
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if pr != nil {
				pr.LogError(err, fmt.Sprintf("Failed to access path: %s", path))
			}
			return err
		}

		if !info.IsDir() {
			return nil
		}

		dirName := filepath.Base(path)

		// Extract and normalize folder title using existing extractor to handle release groups
		folderTitle, year := ExtractTVShowTitle(dirName)
		normalizedFolderTitle := strings.ToLower(NormalizeName(folderTitle))

		// Check if folder matches the old title OR if there are files inside that match oldTitle (filename match)
		if normalizedFolderTitle == normalizedOld {
			// full match on folder title
			var newFolderName string
			if year != "" {
				newFolderName = fmt.Sprintf("%s (%s)", newTitle, year)
			} else {
				newFolderName = newTitle
			}
			newFolderPath := filepath.Join(filepath.Dir(path), newFolderName)

			if pr != nil {
				pr.Update(50, fmt.Sprintf("Renaming episodes in: %s", dirName))
			}

			episodeResults, err := renameEpisodesInFolderWithProgress(path, oldTitle, newTitle, dryRun, pr)
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
				return nil
			}
			results = append(results, episodeResults...)

			if pr != nil {
				pr.Update(90, fmt.Sprintf("Renaming folder: %s", dirName))
			}

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
					return nil
				}
			}

			results = append(results, RenameResult{
				OldPath:  path,
				NewPath:  newFolderPath,
				IsFolder: true,
				Success:  true,
			})

			return filepath.SkipDir
		}

		// If folder title doesn't match the old title, check if files within the folder match oldTitle
		prUpdate := func(msg string) {
			if pr != nil {
				pr.Update(20, msg)
			}
		}

		prUpdate(fmt.Sprintf("Scanning files in: %s", dirName))
		// Dry-run detect any episodes that contain oldTitle
		detectResults, _ := renameEpisodesInFolderWithProgress(path, oldTitle, newTitle, true, pr)
		if len(detectResults) > 0 {
			// We have episode matches, perform actual rename
			if pr != nil {
				pr.Update(50, fmt.Sprintf("Renaming episodes in: %s", dirName))
			}
			episodeResults, err := renameEpisodesInFolderWithProgress(path, oldTitle, newTitle, dryRun, pr)
			if err != nil {
				if pr != nil {
					pr.LogError(err, fmt.Sprintf("Failed to rename episodes in: %s", dirName))
				}
				results = append(results, RenameResult{
					OldPath:  path,
					NewPath:  filepath.Join(filepath.Dir(path), newTitle),
					IsFolder: true,
					Success:  false,
					Error:    fmt.Sprintf("failed to rename episodes: %v", err),
				})
				return nil
			}
			results = append(results, episodeResults...)

			// Compute folder rename target
			var newFolderName string
			if year != "" {
				newFolderName = fmt.Sprintf("%s (%s)", newTitle, year)
			} else {
				newFolderName = newTitle
			}
			newFolderPath := filepath.Join(filepath.Dir(path), newFolderName)

			if pr != nil {
				pr.Update(90, fmt.Sprintf("Renaming folder: %s", dirName))
			}

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
					return nil
				}
			}

			results = append(results, RenameResult{
				OldPath:  path,
				NewPath:  newFolderPath,
				IsFolder: true,
				Success:  true,
			})

			return filepath.SkipDir
		}

		return nil

		return nil
	})

	if err != nil {
		if pr != nil {
			pr.LogError(err, "Rename operation failed")
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
	return renameEpisodesInFolderWithProgress(folderPath, oldTitle, newTitle, dryRun, nil)
}

// renameEpisodesInFolderWithProgress renames all episode files inside a folder with progress reporting
func renameEpisodesInFolderWithProgress(folderPath, oldTitle, newTitle string, dryRun bool, pr *ProgressReporter) ([]RenameResult, error) {
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
			// Build a new filename using the new title and episode code for consistency
			parts := episodePattern.Split(nameWithoutExt, -1)
			episodeCode := episodePattern.FindString(nameWithoutExt)
			if len(parts) > 0 && episodeCode != "" {
				suffix := ""
				if len(parts) > 1 {
					suffix = parts[1]
				}
				newFileName := newTitle + " " + episodeCode + suffix + ext
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
						return nil
					}
				}

				results = append(results, RenameResult{
					OldPath:  path,
					NewPath:  newPath,
					IsFolder: false,
					Success:  true,
				})
				return nil
			}
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
