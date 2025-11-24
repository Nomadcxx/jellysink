package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LooseFile represents a video file not in proper Jellyfin structure
type LooseFile struct {
	Path          string
	Type          string // "tv" or "movie"
	DetectedTitle string
	DetectedYear  string
	Season        int
	Episode       int
	Size          int64
	SuggestedPath string
	Action        string // "organize" or "skip"
	SkipReason    string
}

// LooseFileResult represents the result of organizing loose files
type LooseFileResult struct {
	Original    string
	Destination string
	Type        string // "tv" or "movie"
	Success     bool
	Error       string
}

// ScanLooseFiles finds video files not in proper Jellyfin structure
func ScanLooseFiles(paths []string) ([]LooseFile, error) {
	return ScanLooseFilesWithProgress(paths, nil)
}

// ScanLooseFilesWithProgress finds loose files with progress reporting
func ScanLooseFilesWithProgress(paths []string, progressCh chan<- ScanProgress) ([]LooseFile, error) {
	var pr *ProgressReporter
	if progressCh != nil {
		pr = NewProgressReporterWithInterval(progressCh, "loose_files", 200*time.Millisecond)
		pr.StageUpdate("scanning", "Scanning for loose files...")

		total, err := CountVideoFilesWithProgress(paths, pr)
		if err != nil {
			pr.LogError(err, "Failed to count files")
			return nil, fmt.Errorf("failed to count files: %w", err)
		}
		pr.Start(total, fmt.Sprintf("Scanning %d files for loose organization...", total))
	}

	var looseFiles []LooseFile
	filesProcessed := 0

	for _, libPath := range paths {
		if _, err := os.Stat(libPath); err != nil {
			if pr != nil {
				pr.LogError(err, fmt.Sprintf("Library path not accessible: %s", libPath))
			}
			return nil, fmt.Errorf("library path not accessible: %s: %w", libPath, err)
		}

		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if pr != nil {
					pr.LogError(err, fmt.Sprintf("Error accessing: %s", path))
				}
				return err
			}

			if info.IsDir() || !isVideoFile(path) {
				return nil
			}

			filesProcessed++
			if pr != nil && filesProcessed%10 == 0 {
				pr.Update(filesProcessed, fmt.Sprintf("Checking: %s", filepath.Base(path)))
			}

			// Skip sample files
			if isSampleFile(path) {
				return nil
			}

			// Check if file is loose (not in proper structure)
			if isLooseFile(path, libPath) {
				loose := classifyLooseFile(path, libPath)
				if loose != nil {
					looseFiles = append(looseFiles, *loose)
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error scanning %s: %w", libPath, err)
		}
	}

	if pr != nil {
		pr.Complete(fmt.Sprintf("Found %d loose files", len(looseFiles)))
	}

	return looseFiles, nil
}

// isLooseFile checks if a file is not in proper Jellyfin structure
func isLooseFile(path, libPath string) bool {
	// Get relative path from library root
	relPath, err := filepath.Rel(libPath, path)
	if err != nil {
		return false
	}

	// Count directory depth
	parts := strings.Split(relPath, string(filepath.Separator))

	// Proper TV structure: ShowName/Season##/episode.mkv (3 parts)
	// Proper movie structure: MovieName/movie.mkv (2 parts)

	// If file is directly in library root (1 part = just filename)
	if len(parts) == 1 {
		return true // Loose file
	}

	// If file is in a folder directly under library (2 parts)
	if len(parts) == 2 {
		// Check if it looks like a TV episode
		_, _, hasEpisode := ExtractEpisodeInfo(filepath.Base(path))
		if hasEpisode {
			// TV episode should be in ShowName/Season##/ structure
			// Being in ShowName/ only is loose
			return true
		}
		// Movies in MovieName/ are fine
		return false
	}

	// Check for TV shows - should be in Season## folder
	if len(parts) == 3 {
		seasonFolder := parts[1] // Should be "Season 01", "Season 02", etc.
		if strings.HasPrefix(strings.ToLower(seasonFolder), "season") {
			return false // Proper structure
		}
		// Not in Season folder = loose
		return true
	}

	// Deeper than 3 levels or unusual structure
	if len(parts) > 3 {
		return true // Consider loose for review
	}

	return false
}

// classifyLooseFile determines if a loose file is TV or movie and suggests organization
func classifyLooseFile(path, libPath string) *LooseFile {
	filename := filepath.Base(path)
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	loose := &LooseFile{
		Path: path,
		Size: info.Size(),
	}

	// Try to detect TV episode
	season, episode, hasEpisode := ExtractEpisodeInfo(filename)
	if hasEpisode {
		// This is a TV episode
		loose.Type = "tv"
		loose.Season = season
		loose.Episode = episode

		// Extract show name
		showName, year := ExtractTVShowTitle(filename)
		if showName == "" {
			// Try parent folder name
			parentFolder := filepath.Base(filepath.Dir(path))
			showName, year = ExtractTVShowTitle(parentFolder)
		}

		if showName == "" {
			loose.Action = "skip"
			loose.SkipReason = "Cannot determine show name"
			return loose
		}

		loose.DetectedTitle = showName
		loose.DetectedYear = year

		// Build suggested path
		folderName := showName
		if year != "" {
			folderName = fmt.Sprintf("%s (%s)", showName, year)
		}

		seasonFolder := fmt.Sprintf("Season %02d", season)
		episodeFilename := fmt.Sprintf("%s S%02dE%02d%s", folderName, season, episode, filepath.Ext(filename))

		loose.SuggestedPath = filepath.Join(libPath, folderName, seasonFolder, episodeFilename)
		loose.Action = "organize"

		return loose
	}

	// Not a TV episode - treat as movie
	loose.Type = "movie"

	// Extract movie name and year
	title, year := ExtractMovieTitleFromFilename(filename)
	if title == "" {
		// Try parent folder name
		parentFolder := filepath.Base(filepath.Dir(path))
		title, year = ExtractMovieTitleFromFilename(parentFolder)
	}

	if title == "" {
		loose.Action = "skip"
		loose.SkipReason = "Cannot determine movie title"
		return loose
	}

	loose.DetectedTitle = title
	loose.DetectedYear = year

	// Build suggested path
	folderName := title
	if year != "" {
		folderName = fmt.Sprintf("%s (%s)", title, year)
	}

	movieFilename := folderName + filepath.Ext(filename)
	loose.SuggestedPath = filepath.Join(libPath, folderName, movieFilename)
	loose.Action = "organize"

	return loose
}

// ExtractMovieTitleFromFilename extracts movie title and year from filename
func ExtractMovieTitleFromFilename(filename string) (title, year string) {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Extract year if present
	year = ExtractYear(nameWithoutExt)

	// Remove year and clean up title
	titlePart := nameWithoutExt
	if year != "" {
		// Remove year and surrounding delimiters
		titlePart = removeYear(nameWithoutExt)
	}

	// Clean title: remove release group markers, dots, etc.
	title = CleanMovieName(titlePart)

	// If cleaning produced empty string, try without cleaning
	if title == "" {
		title = strings.TrimSpace(titlePart)
	}

	return title, year
}

// OrganizeLooseFiles moves loose files to proper Jellyfin structure
func OrganizeLooseFiles(files []LooseFile, dryRun bool) ([]LooseFileResult, error) {
	return OrganizeLooseFilesWithProgress(files, dryRun, nil)
}

// OrganizeLooseFilesWithProgress organizes loose files with progress reporting
func OrganizeLooseFilesWithProgress(files []LooseFile, dryRun bool, pr *ProgressReporter) ([]LooseFileResult, error) {
	var results []LooseFileResult

	if pr != nil {
		pr.Start(len(files), fmt.Sprintf("Organizing %d loose files", len(files)))
	}

	for i, file := range files {
		if pr != nil {
			pr.Update(i+1, fmt.Sprintf("Organizing: %s", filepath.Base(file.Path)))
		}

		// Skip files marked as skip
		if file.Action == "skip" {
			results = append(results, LooseFileResult{
				Original:    file.Path,
				Destination: "",
				Type:        file.Type,
				Success:     false,
				Error:       file.SkipReason,
			})
			continue
		}

		// Create destination directory
		destDir := filepath.Dir(file.SuggestedPath)
		if !dryRun {
			if err := os.MkdirAll(destDir, 0755); err != nil {
				if pr != nil {
					pr.LogError(err, fmt.Sprintf("Failed to create directory: %s", destDir))
				}
				results = append(results, LooseFileResult{
					Original:    file.Path,
					Destination: file.SuggestedPath,
					Type:        file.Type,
					Success:     false,
					Error:       fmt.Sprintf("Failed to create directory: %v", err),
				})
				continue
			}
		}

		// Check if destination already exists
		if _, err := os.Stat(file.SuggestedPath); err == nil && !dryRun {
			results = append(results, LooseFileResult{
				Original:    file.Path,
				Destination: file.SuggestedPath,
				Type:        file.Type,
				Success:     false,
				Error:       "Destination already exists",
			})
			continue
		}

		// Move file
		if !dryRun {
			if err := os.Rename(file.Path, file.SuggestedPath); err != nil {
				if pr != nil {
					pr.LogError(err, fmt.Sprintf("Failed to move file: %s", filepath.Base(file.Path)))
				}
				results = append(results, LooseFileResult{
					Original:    file.Path,
					Destination: file.SuggestedPath,
					Type:        file.Type,
					Success:     false,
					Error:       fmt.Sprintf("Failed to move file: %v", err),
				})
				continue
			}

			// Clean up empty source directory
			sourceDir := filepath.Dir(file.Path)
			if entries, err := os.ReadDir(sourceDir); err == nil && len(entries) == 0 {
				_ = os.Remove(sourceDir)
			}
		}

		results = append(results, LooseFileResult{
			Original:    file.Path,
			Destination: file.SuggestedPath,
			Type:        file.Type,
			Success:     true,
		})
	}

	if pr != nil {
		successCount := 0
		for _, r := range results {
			if r.Success {
				successCount++
			}
		}
		pr.Complete(fmt.Sprintf("Organized %d/%d files successfully", successCount, len(files)))
	}

	return results, nil
}
