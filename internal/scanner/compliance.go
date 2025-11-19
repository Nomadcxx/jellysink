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
func ScanMovieCompliance(paths []string) ([]ComplianceIssue, error) {
	var issues []ComplianceIssue

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

			// Check if this is compliant
			issue := checkMovieCompliance(path, libPath)
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
		// Check if both follow pattern but just don't match
		if hasYear(parentDir) && hasYear(filenameNoExt) {
			// Both have years, should match
			suggestedPath := filepath.Join(filepath.Dir(filePath), parentDir+filepath.Ext(filePath))

			return &ComplianceIssue{
				Path:            filePath,
				Type:            "movie",
				Problem:         "Folder name doesn't match filename",
				SuggestedPath:   suggestedPath,
				SuggestedAction: "rename",
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
func ScanTVCompliance(paths []string) ([]ComplianceIssue, error) {
	var issues []ComplianceIssue

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
	showDir := filepath.Base(filepath.Dir(filepath.Dir(filePath)))

	// Check if in proper Season ## folder
	expectedSeasonDir := fmt.Sprintf("Season %02d", season)
	if seasonDir != expectedSeasonDir {
		// Non-compliant season folder
		cleanShowName := CleanMovieName(showDir) // Reuse for show names

		suggestedDir := filepath.Join(libRoot, cleanShowName, expectedSeasonDir)
		suggestedFilename := fmt.Sprintf("%s S%02dE%02d%s", cleanShowName, season, episode, filepath.Ext(filePath))
		suggestedPath := filepath.Join(suggestedDir, suggestedFilename)

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "tv",
			Problem:         fmt.Sprintf("Not in proper 'Season %02d' folder (found: %s)", season, seasonDir),
			SuggestedPath:   suggestedPath,
			SuggestedAction: "reorganize",
		}
	}

	// Check if filename follows release group naming
	if isReleaseGroupFolder(filename) {
		cleanShowName := CleanMovieName(showDir)

		suggestedFilename := fmt.Sprintf("%s S%02dE%02d%s", cleanShowName, season, episode, filepath.Ext(filePath))
		suggestedPath := filepath.Join(filepath.Dir(filePath), suggestedFilename)

		return &ComplianceIssue{
			Path:            filePath,
			Type:            "tv",
			Problem:         "Release group naming in filename",
			SuggestedPath:   suggestedPath,
			SuggestedAction: "rename",
		}
	}

	return nil
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
