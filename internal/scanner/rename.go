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
// It finds all folders matching oldTitle and renames them and their contents to newTitle
func ApplyManualTVRename(basePath, oldTitle, newTitle string, dryRun bool) ([]RenameResult, error) {
	var results []RenameResult

	if newTitle == "" {
		return results, fmt.Errorf("new title cannot be empty")
	}

	if strings.ContainsAny(newTitle, `<>:"/\|?*`) {
		return results, fmt.Errorf("new title contains invalid characters")
	}

	normalizedOld := strings.ToLower(strings.TrimSpace(oldTitle))
	normalizedNew := strings.ToLower(strings.TrimSpace(newTitle))

	if normalizedOld == normalizedNew {
		return results, fmt.Errorf("old and new titles are the same")
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		dirName := filepath.Base(path)

		tvTitlePattern := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)$`)
		matches := tvTitlePattern.FindStringSubmatch(dirName)

		if len(matches) == 3 {
			folderTitle := matches[1]
			year := matches[2]
			normalizedFolderTitle := strings.ToLower(strings.TrimSpace(folderTitle))

			if normalizedFolderTitle == normalizedOld {
				newFolderName := fmt.Sprintf("%s (%s)", newTitle, year)
				newFolderPath := filepath.Join(filepath.Dir(path), newFolderName)

				episodeResults, err := renameEpisodesInFolder(path, oldTitle, newTitle, dryRun)
				if err != nil {
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

				if !dryRun {
					if err := os.Rename(path, newFolderPath); err != nil {
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
		}

		return nil
	})

	if err != nil {
		return results, err
	}

	return results, nil
}

// renameEpisodesInFolder renames all episode files inside a folder
func renameEpisodesInFolder(folderPath, oldTitle, newTitle string, dryRun bool) ([]RenameResult, error) {
	var results []RenameResult

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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
