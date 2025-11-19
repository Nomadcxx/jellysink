package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MovieDuplicate represents a group of duplicate movies
type MovieDuplicate struct {
	NormalizedName string      // Normalized movie name for grouping
	Year           string      // Movie year
	Files          []MovieFile // All versions found
}

// MovieFile represents a single movie file
type MovieFile struct {
	Path       string // Full path to file
	Size       int64  // File size in bytes
	Resolution string // 1080p, 720p, etc.
	IsEmpty    bool   // True if 0 bytes or missing
}

// ScanMovies scans movie library paths for duplicates
// Returns groups of duplicate movies
func ScanMovies(paths []string) ([]MovieDuplicate, error) {
	// Map: normalized_name|year -> []MovieFile
	movieGroups := make(map[string]*MovieDuplicate)

	for _, libPath := range paths {
		// Verify path exists
		if _, err := os.Stat(libPath); err != nil {
			return nil, fmt.Errorf("library path not accessible: %s: %w", libPath, err)
		}

		// Walk directory tree
		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Only process video files
			if !isVideoFile(path) {
				return nil
			}

			// Extract movie info from filename/path
			movieFile := parseMovieFile(path, info)

			// Extract movie title from parent directory name (Jellyfin format)
			// or from filename if not following convention
			parentDir := filepath.Base(filepath.Dir(path))
			movieTitle := parentDir
			if parentDir == "." || parentDir == "/" {
				// Fallback to filename
				movieTitle = filepath.Base(path)
			}

			// Create group key: normalized_name|year
			normalized := NormalizeName(movieTitle)
			year := ExtractYear(movieTitle)
			key := normalized + "|" + year

			// Add to group
			if _, exists := movieGroups[key]; !exists {
				movieGroups[key] = &MovieDuplicate{
					NormalizedName: normalized,
					Year:           year,
					Files:          []MovieFile{},
				}
			}
			movieGroups[key].Files = append(movieGroups[key].Files, movieFile)

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error scanning %s: %w", libPath, err)
		}
	}

	// Filter to only duplicates (2+ files per group)
	var duplicates []MovieDuplicate
	for _, group := range movieGroups {
		if len(group.Files) > 1 {
			duplicates = append(duplicates, *group)
		}
	}

	return duplicates, nil
}

// parseMovieFile extracts metadata from movie file
func parseMovieFile(path string, info os.FileInfo) MovieFile {
	return MovieFile{
		Path:       path,
		Size:       info.Size(),
		Resolution: ExtractResolution(path),
		IsEmpty:    info.Size() == 0,
	}
}

// isVideoFile checks if file extension is a video format
func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := []string{
		".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv",
		".webm", ".m4v", ".mpg", ".mpeg", ".m2ts", ".ts",
	}

	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}

	return false
}

// MarkKeepDelete marks which files to keep vs delete in each duplicate group
// Strategy: Keep largest non-empty file with highest resolution
func MarkKeepDelete(duplicates []MovieDuplicate) []MovieDuplicate {
	for i := range duplicates {
		group := &duplicates[i]

		// Find best file (largest non-empty with highest resolution)
		bestIdx := 0
		bestScore := scoreMovieFile(group.Files[0])

		for j := 1; j < len(group.Files); j++ {
			score := scoreMovieFile(group.Files[j])
			if score > bestScore {
				bestScore = score
				bestIdx = j
			}
		}

		// Mark all except best as delete (in practice, we'll use index comparison)
		// The caller will know that bestIdx is the one to keep
		// We'll add a KeepIndex field to MovieDuplicate
		group.Files[bestIdx], group.Files[0] = group.Files[0], group.Files[bestIdx]
	}

	return duplicates
}

// scoreMovieFile assigns quality score for comparison
// Higher score = better to keep
func scoreMovieFile(file MovieFile) int {
	score := 0

	// Empty files are always worst
	if file.IsEmpty {
		return -1000
	}

	// Resolution scoring
	switch file.Resolution {
	case "2160p":
		score += 400
	case "1080p":
		score += 300
	case "720p":
		score += 200
	case "480p":
		score += 100
	}

	// Size scoring (larger is better, up to reasonable limit)
	// Add 1 point per GB, capped at 50GB
	sizeGB := file.Size / (1024 * 1024 * 1024)
	if sizeGB > 50 {
		sizeGB = 50
	}
	score += int(sizeGB)

	return score
}

// GetDeleteList returns paths of files marked for deletion
// First file in each group is kept, rest are deleted
func GetDeleteList(duplicates []MovieDuplicate) []string {
	var deleteList []string

	for _, group := range duplicates {
		// Skip first file (it's the keeper)
		for i := 1; i < len(group.Files); i++ {
			deleteList = append(deleteList, group.Files[i].Path)
		}
	}

	return deleteList
}

// GetSpaceToFree calculates total bytes that can be freed
func GetSpaceToFree(duplicates []MovieDuplicate) int64 {
	var total int64

	for _, group := range duplicates {
		// Skip first file (it's the keeper)
		for i := 1; i < len(group.Files); i++ {
			total += group.Files[i].Size
		}
	}

	return total
}
