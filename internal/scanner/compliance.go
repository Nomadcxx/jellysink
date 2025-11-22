package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ComplianceIssue represents a naming compliance problem
type ComplianceIssue struct {
	Path            string // Current path
	Type            string // "movie" or "tv"
	Problem         string // Description of the issue
	SuggestedPath   string // Suggested compliant path
	SuggestedAction string // "rename" or "reorganize"
}

// ScanMovieCompliance scans for non-Jellyfin-compliant movie folders
// Expected format: Movie Name (Year)/Movie Name (Year).ext
// excludePaths: list of file paths to skip (e.g., files marked for deletion in duplicate scan)
func ScanMovieCompliance(paths []string, excludePaths ...string) ([]ComplianceIssue, error) {
	var issues []ComplianceIssue
	targetPaths := make(map[string]string) // suggestedPath -> originalPath

	// Build exclusion set for fast lookup
	excludeSet := make(map[string]bool)
	if len(excludePaths) > 0 {
		for _, path := range excludePaths[0:] {
			excludeSet[path] = true
		}
	}

	for _, libPath := range paths {
		if _, err := os.Stat(libPath); err != nil {
			return nil, fmt.Errorf("library path not accessible: %s: %w", libPath, err)
		}

		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the library root itself
			if path == libPath {
				return nil
			}

			// Only check video files
			if info.IsDir() || !isVideoFile(path) {
				return nil
			}

			// Skip files marked for deletion in duplicate scan
			if excludeSet[path] {
				return nil
			}

			// Skip sample files - they should be deleted, not renamed
			if isSampleFile(path) {
				return nil
			}

			// Check if this is compliant
			issue := checkMovieCompliance(path, libPath)
			if issue != nil {
				// Check for collision: another file already wants this target path
				if existingSource, exists := targetPaths[issue.SuggestedPath]; exists {
					// Collision detected! Skip this one and add warning to existing issue
					issue.Problem = fmt.Sprintf("COLLISION: Multiple files want same target (also: %s)", filepath.Base(existingSource))
					issue.SuggestedAction = "manual_review"
				} else {
					// No collision, track this target
					targetPaths[issue.SuggestedPath] = path
				}

				issues = append(issues, *issue)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error scanning %s: %w", libPath, err)
		}
	}

	return issues, nil
}

// checkMovieCompliance checks if a movie file follows Jellyfin conventions
func checkMovieCompliance(filePath, libRoot string) *ComplianceIssue {
	filename := filepath.Base(filePath)
	parentDir := filepath.Base(filepath.Dir(filePath))

	// Check if parent directory looks like a release group folder
	if isReleaseGroupFolder(parentDir) {
		// Non-compliant: Movie.Name.2024.1080p.BluRay-GROUP/movie.mkv
		cleanName := CleanMovieName(parentDir)

		suggestedDir := filepath.Join(libRoot, cleanName)
		suggestedPath := filepath.Join(suggestedDir, cleanName+filepath.Ext(filePath))

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "movie",
			Problem:         "Release group folder naming (contains resolution/codec/source markers)",
			SuggestedPath:   suggestedPath,
			SuggestedAction: "reorganize",
		}
	}

	// Check if file is directly in library root (should be in subfolder)
	if filepath.Dir(filePath) == libRoot {
		// Non-compliant: MOVIES/Movie.Name.2024.mkv (no parent folder)
		cleanName := CleanMovieName(filename)

		suggestedDir := filepath.Join(libRoot, cleanName)
		suggestedPath := filepath.Join(suggestedDir, cleanName+filepath.Ext(filePath))

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "movie",
			Problem:         "Movie file directly in library root (should be in subfolder)",
			SuggestedPath:   suggestedPath,
			SuggestedAction: "reorganize",
		}
	}

	// Check if parent directory name matches filename (minus extension)
	filenameNoExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	if parentDir != filenameNoExt {
		// If parent dir is clean (has year in parentheses) prefer it as the source of truth
		if hasYearInParentheses(parentDir) {
			// Use parent dir as source of truth
			cleanName := CleanMovieName(parentDir)
			suggestedDir := filepath.Join(filepath.Dir(filepath.Dir(filePath)), cleanName)
			suggestedPath := filepath.Join(suggestedDir, cleanName+filepath.Ext(filePath))

			// Only suggest a change if filename does not already match that parent dir's cleaned name
			if CleanMovieName(filenameNoExt) != cleanName {
				return &ComplianceIssue{
					Path:            filePath,
					Type:            "movie",
					Problem:         "Folder name doesn't match filename",
					SuggestedPath:   suggestedPath,
					SuggestedAction: "reorganize",
				}
			}
		}
		// Check if both follow pattern but just don't match
		if hasYear(parentDir) && hasYear(filenameNoExt) {
			// Both have years but don't match - clean the filename to get the correct name
			// This handles cases where folder has release group remnants like "Moon RightSiZE (2009)"
			cleanName := CleanMovieName(filenameNoExt)

			// Use the cleaned filename as the source of truth
			suggestedDir := filepath.Join(filepath.Dir(filepath.Dir(filePath)), cleanName)
			suggestedPath := filepath.Join(suggestedDir, cleanName+filepath.Ext(filePath))

			return &ComplianceIssue{
				Path:            filePath,
				Type:            "movie",
				Problem:         "Folder name doesn't match filename",
				SuggestedPath:   suggestedPath,
				SuggestedAction: "reorganize",
			}
		}
	}

	// Check if year is present in parentheses
	if !hasYearInParentheses(parentDir) && hasYear(parentDir) {
		// Has year but not in correct format
		year := ExtractYear(parentDir)
		nameWithoutYear := removeYear(parentDir)
		cleanName := strings.TrimSpace(nameWithoutYear) + " (" + year + ")"

		suggestedDir := filepath.Join(filepath.Dir(filepath.Dir(filePath)), cleanName)
		suggestedPath := filepath.Join(suggestedDir, cleanName+filepath.Ext(filePath))

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "movie",
			Problem:         "Year not in parentheses format",
			SuggestedPath:   suggestedPath,
			SuggestedAction: "reorganize",
		}
	}

	return nil
}

// ScanTVCompliance scans for non-Jellyfin-compliant TV show folders
// Expected format: Show Name (Year)/Season ##/Show Name (Year) S##E##.ext
// excludePaths: list of file paths to skip (e.g., files marked for deletion in duplicate scan)
func ScanTVCompliance(paths []string, excludePaths ...string) ([]ComplianceIssue, error) {
	var issues []ComplianceIssue

	// Build exclusion set for fast lookup
	excludeSet := make(map[string]bool)
	if len(excludePaths) > 0 {
		for _, path := range excludePaths[0:] {
			excludeSet[path] = true
		}
	}

	for _, libPath := range paths {
		if _, err := os.Stat(libPath); err != nil {
			return nil, fmt.Errorf("library path not accessible: %s: %w", libPath, err)
		}

		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the library root itself
			if path == libPath {
				return nil
			}

			// Only check video files
			if info.IsDir() || !isVideoFile(path) {
				return nil
			}

			// Skip files marked for deletion in duplicate scan
			if excludeSet[path] {
				return nil
			}

			// Skip sample files - they should be deleted, not renamed
			if isSampleFile(path) {
				return nil
			}

			// Must have S##E## pattern to be a TV episode
			season, episode, found := ExtractEpisodeInfo(filepath.Base(path))
			if !found {
				// Not a TV episode format, skip
				return nil
			}

			// Check if this is compliant
			issue := checkTVCompliance(path, libPath, season, episode)
			if issue != nil {
				issues = append(issues, *issue)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error scanning %s: %w", libPath, err)
		}
	}

	return issues, nil
}

// checkTVCompliance checks if a TV episode file follows Jellyfin conventions
func checkTVCompliance(filePath, libRoot string, season, episode int) *ComplianceIssue {
	filename := filepath.Base(filePath)
	seasonDir := filepath.Base(filepath.Dir(filePath))

	resolution := ResolveTVShowTitle(filePath, libRoot)

	var cleanShowName string
	if resolution.IsAmbiguous {
		cleanShowName = resolution.ResolvedTitle
	} else {
		cleanShowName = resolution.ResolvedTitle
	}

	expectedSeasonDir := fmt.Sprintf("Season %02d", season)
	if seasonDir != expectedSeasonDir {
		suggestedDir := filepath.Join(libRoot, cleanShowName, expectedSeasonDir)
		suggestedFilename := fmt.Sprintf("%s S%02dE%02d%s", cleanShowName, season, episode, filepath.Ext(filePath))
		suggestedPath := filepath.Join(suggestedDir, suggestedFilename)

		problem := fmt.Sprintf("Not in proper 'Season %02d' folder (found: %s)", season, seasonDir)
		if resolution.IsAmbiguous {
			problem += fmt.Sprintf(" [AMBIGUOUS: %s]", resolution.Reason)
		}

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "tv",
			Problem:         problem,
			SuggestedPath:   suggestedPath,
			SuggestedAction: "reorganize",
		}
	}

	if isReleaseGroupFolder(filename) {
		suggestedFilename := fmt.Sprintf("%s S%02dE%02d%s", cleanShowName, season, episode, filepath.Ext(filePath))
		suggestedPath := filepath.Join(filepath.Dir(filePath), suggestedFilename)

		problem := "Release group naming in filename"
		if resolution.IsAmbiguous {
			problem += fmt.Sprintf(" [AMBIGUOUS: %s]", resolution.Reason)
		}

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "tv",
			Problem:         problem,
			SuggestedPath:   suggestedPath,
			SuggestedAction: "rename",
		}
	}

	if resolution.IsAmbiguous && (resolution.FolderMatch.Title != resolution.FilenameMatch.Title) {
		suggestedFilename := fmt.Sprintf("%s S%02dE%02d%s", cleanShowName, season, episode, filepath.Ext(filePath))
		suggestedPath := filepath.Join(filepath.Dir(filePath), suggestedFilename)

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "tv",
			Problem:         fmt.Sprintf("Title mismatch: %s", resolution.Reason),
			SuggestedPath:   suggestedPath,
			SuggestedAction: "manual_review",
		}
	}

	return nil
}

// isSampleFile checks if a file is a sample/trailer/extra (should be deleted, not fixed)
func isSampleFile(path string) bool {
	filename := strings.ToLower(filepath.Base(path))
	// Remove dots and other separators for matching
	normalized := strings.ReplaceAll(filename, ".", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")

	sampleMarkers := []string{
		"sample",
		"trailer",
		"extra",
		"deleted scene",
		"behind the scenes",
		"making of",
		"interview",
		"featurette",
	}

	for _, marker := range sampleMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	return false
}

// isReleaseGroupFolder checks if a folder name contains release group markers
func isReleaseGroupFolder(name string) bool {
	nameUpper := strings.ToUpper(name)

	// Check for common release markers
	markers := []string{
		"1080P", "720P", "2160P", "4K",
		"BLURAY", "BLU-RAY", "BDRIP", "REMUX",
		"WEB-DL", "WEBDL", "WEB-RIP", "WEBRIP",
		"HDTV", "X264", "X265", "HEVC",
	}

	for _, marker := range markers {
		if strings.Contains(nameUpper, marker) {
			return true
		}
	}

	// Check for hyphenated release group at end (e.g., "-GROUP")
	if strings.Contains(name, "-") && !strings.Contains(name, " - ") {
		return true
	}

	// Check for dots as separators (release naming style)
	dotCount := strings.Count(name, ".")
	if dotCount >= 3 {
		return true
	}

	return false
}

// hasYear checks if string contains a 4-digit year
func hasYear(s string) bool {
	return ExtractYear(s) != ""
}

// hasYearInParentheses checks if string has year in (YYYY) format
func hasYearInParentheses(s string) bool {
	re := regexp.MustCompile(`\(\d{4}\)`)
	return re.MatchString(s)
}

// ApplyMovieCompliance applies the suggested fix for a compliance issue
// Creates folder if needed, renames/moves file to compliant location
func ApplyMovieCompliance(issue ComplianceIssue) error {
	if issue.Type != "movie" {
		return fmt.Errorf("not a movie compliance issue")
	}

	// Create parent directory if it doesn't exist
	targetDir := filepath.Dir(issue.SuggestedPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Check if target already exists
	if _, err := os.Stat(issue.SuggestedPath); err == nil {
		return fmt.Errorf("target file already exists: %s", issue.SuggestedPath)
	}

	// Move/rename file
	if err := os.Rename(issue.Path, issue.SuggestedPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	// If original directory is now empty (and not library root), remove it
	originalDir := filepath.Dir(issue.Path)
	if entries, err := os.ReadDir(originalDir); err == nil && len(entries) == 0 {
		// Safe to remove empty directory
		_ = os.Remove(originalDir)
	}

	return nil
}

// ApplyTVCompliance applies the suggested fix for a TV show compliance issue
// Checks if Show Name (Year) and Season ## folders exist before creating
func ApplyTVCompliance(issue ComplianceIssue) error {
	if issue.Type != "tv" {
		return fmt.Errorf("not a TV compliance issue")
	}

	// Parse target path components
	targetSeasonDir := filepath.Dir(issue.SuggestedPath)
	targetShowDir := filepath.Dir(targetSeasonDir)

	// Check if Show Name (Year) folder already exists
	showDirExists := false
	if info, err := os.Stat(targetShowDir); err == nil && info.IsDir() {
		showDirExists = true
	}

	// Check if Season ## folder already exists
	seasonDirExists := false
	if info, err := os.Stat(targetSeasonDir); err == nil && info.IsDir() {
		seasonDirExists = true
	}

	// Create missing directories
	if !showDirExists {
		if err := os.Mkdir(targetShowDir, 0755); err != nil {
			return fmt.Errorf("failed to create show directory %s: %w", targetShowDir, err)
		}
	}

	if !seasonDirExists {
		if err := os.Mkdir(targetSeasonDir, 0755); err != nil {
			return fmt.Errorf("failed to create season directory %s: %w", targetSeasonDir, err)
		}
	}

	// Check if target file already exists
	if _, err := os.Stat(issue.SuggestedPath); err == nil {
		return fmt.Errorf("target file already exists: %s", issue.SuggestedPath)
	}

	// Move/rename file to compliant location
	if err := os.Rename(issue.Path, issue.SuggestedPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	// If original directory is now empty, remove it
	originalDir := filepath.Dir(issue.Path)
	if entries, err := os.ReadDir(originalDir); err == nil && len(entries) == 0 {
		_ = os.Remove(originalDir)

		// If original parent directory is also now empty, remove it too
		originalParentDir := filepath.Dir(originalDir)
		if entries, err := os.ReadDir(originalParentDir); err == nil && len(entries) == 0 {
			_ = os.Remove(originalParentDir)
		}
	}

	return nil
}
