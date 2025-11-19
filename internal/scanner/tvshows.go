package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TVDuplicate represents a group of duplicate TV episodes
type TVDuplicate struct {
	ShowName string      // Normalized show name
	Season   int         // Season number
	Episode  int         // Episode number
	Files    []TVFile    // All versions found
}

// TVFile represents a single TV episode file
type TVFile struct {
	Path       string // Full path to file
	Size       int64  // File size in bytes
	Resolution string // 1080p, 720p, etc.
	Source     string // BluRay, WEB-DL, HDTV, etc.
	IsEmpty    bool   // True if 0 bytes or missing
}

// ScanTVShows scans TV library paths for duplicate episodes
// Returns groups of duplicate episodes
func ScanTVShows(paths []string) ([]TVDuplicate, error) {
	// Map: normalized_show|S##E## -> []TVFile
	episodeGroups := make(map[string]*TVDuplicate)

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

			// Extract episode info from filename
			season, episode, found := ExtractEpisodeInfo(filepath.Base(path))
			if !found {
				// Not a TV episode format, skip
				return nil
			}

			// Parse TV file metadata
			tvFile := parseTVFile(path, info)

			// Extract show name from parent directory structure
			// Expected: Show Name (Year)/Season ##/episode.mkv
			showDir := filepath.Dir(filepath.Dir(path)) // Go up two levels
			showName := filepath.Base(showDir)

			// Normalize show name
			normalized := NormalizeName(showName)

			// Create group key: normalized_show|S##E##
			key := fmt.Sprintf("%s|S%02dE%02d", normalized, season, episode)

			// Add to group
			if _, exists := episodeGroups[key]; !exists {
				episodeGroups[key] = &TVDuplicate{
					ShowName: normalized,
					Season:   season,
					Episode:  episode,
					Files:    []TVFile{},
				}
			}
			episodeGroups[key].Files = append(episodeGroups[key].Files, tvFile)

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error scanning %s: %w", libPath, err)
		}
	}

	// Filter to only duplicates (2+ files per episode)
	var duplicates []TVDuplicate
	for _, group := range episodeGroups {
		if len(group.Files) > 1 {
			duplicates = append(duplicates, *group)
		}
	}

	return duplicates, nil
}

// parseTVFile extracts metadata from TV episode file
func parseTVFile(path string, info os.FileInfo) TVFile {
	return TVFile{
		Path:       path,
		Size:       info.Size(),
		Resolution: ExtractResolution(path),
		Source:     extractSource(path),
		IsEmpty:    info.Size() == 0,
	}
}

// extractSource extracts source type from filename
func extractSource(name string) string {
	nameUpper := strings.ToUpper(name)

	// Check for source types (in quality order)
	sources := []string{
		"REMUX",
		"BLURAY", "BLU-RAY", "BDIRP",
		"WEB-DL", "WEBDL", "WEB",
		"HDTV",
		"DVDRIP",
	}

	for _, source := range sources {
		if strings.Contains(nameUpper, source) {
			return source
		}
	}

	return "unknown"
}

// MarkKeepDeleteTV marks which TV files to keep vs delete in each duplicate group
// Strategy: Keep highest quality (resolution + source scoring)
func MarkKeepDeleteTV(duplicates []TVDuplicate) []TVDuplicate {
	for i := range duplicates {
		group := &duplicates[i]

		// Find best file (highest quality score)
		bestIdx := 0
		bestScore := scoreTVFile(group.Files[0])

		for j := 1; j < len(group.Files); j++ {
			score := scoreTVFile(group.Files[j])
			if score > bestScore {
				bestScore = score
				bestIdx = j
			}
		}

		// Move best file to first position (keeper)
		group.Files[bestIdx], group.Files[0] = group.Files[0], group.Files[bestIdx]
	}

	return duplicates
}

// scoreTVFile assigns quality score for TV episodes
// Higher score = better to keep
func scoreTVFile(file TVFile) int {
	score := 0

	// Empty files are always worst
	if file.IsEmpty {
		return -1000
	}

	// Resolution scoring (same as movies)
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

	// Source scoring
	switch strings.ToUpper(file.Source) {
	case "REMUX":
		score += 50
	case "BLURAY", "BLU-RAY", "BDIRP":
		score += 40
	case "WEB-DL", "WEBDL":
		score += 30
	case "WEB":
		score += 25
	case "HDTV":
		score += 20
	case "DVDRIP":
		score += 10
	}

	// Size scoring (smaller boost than movies since TV episodes vary)
	// Add 1 point per GB, capped at 10GB
	sizeGB := file.Size / (1024 * 1024 * 1024)
	if sizeGB > 10 {
		sizeGB = 10
	}
	score += int(sizeGB)

	return score
}

// GetTVDeleteList returns paths of TV files marked for deletion
// First file in each group is kept, rest are deleted
func GetTVDeleteList(duplicates []TVDuplicate) []string {
	var deleteList []string

	for _, group := range duplicates {
		// Skip first file (it's the keeper)
		for i := 1; i < len(group.Files); i++ {
			deleteList = append(deleteList, group.Files[i].Path)
		}
	}

	return deleteList
}

// GetTVSpaceToFree calculates total bytes that can be freed from TV duplicates
func GetTVSpaceToFree(duplicates []TVDuplicate) int64 {
	var total int64

	for _, group := range duplicates {
		// Skip first file (it's the keeper)
		for i := 1; i < len(group.Files); i++ {
			total += group.Files[i].Size
		}
	}

	return total
}
