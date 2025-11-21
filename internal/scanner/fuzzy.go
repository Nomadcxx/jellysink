package scanner

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Pre-compiled regexes for performance optimization
var (
	releasePatterns       []*regexp.Regexp
	collapseSpacesRegex   *regexp.Regexp
	removePunctRegex      *regexp.Regexp
	yearParenRegex        *regexp.Regexp
	yearBracketRegex      *regexp.Regexp
	yearDotRegex          *regexp.Regexp
	yearSpaceRegex        *regexp.Regexp
	yearRemoveRegexes     []*regexp.Regexp
	episodeSERegex        *regexp.Regexp
	episodeXRegex         *regexp.Regexp
)

func init() {
	// Pre-compile all release group patterns
	patterns := []string{
		// Resolution markers
		`\b\d{3,4}[pi]\b`,  // 1080p, 720p, 2160p, 480i, 576i
		`\b(4K|UHD)\b`,     // 4K, UHD

		// HDR formats (before generic HDR to catch specific variants)
		`\b(HDR10\+?|HDR10Plus|Dolby\s?Vision|DoVi|DV|HDR|HLG|PQ|SDR)\b`,

		// Audio formats with channels (most specific first)
		`\b(DTS-HD\s?MA|DTS-HD\s?HRA|DTS-HD|DTS-X|DTS-ES)\b`,  // DTS variants
		`\b(DD\+?|DDP|E?AC3|AAC|AC3)\d\s\d\b`,                  // Audio with channels
		`\b(DD\+?|DDP|E?AC3|AAC|AC3)\b`,                        // Audio without channels
		`\b(TrueHD|Atmos|FLAC|PCM|Opus|MP3|DTS)\b`,            // Other audio codecs

		// Audio channels (after audio codecs)
		`\b\d\s\d\b`,        // 7 1, 5 1, 2 0
		`\b(Stereo|Mono)\b`,

		// Source types
		`\b(BluRay|Blu-ray|BDRip|BRRip|REMUX|WEB-DL|WEBDL|WEBRip|WEB)\b`,
		`\b(HDTV|PDTV|SDTV|DVDRip|DVD|DVDSCR)\b`,
		`\b(CAM|HDTS|TS|TC|SCR|R5)\b`,

		// Streaming platforms
		`\b(AMZN|NF|DSNP|HMAX|HULU|ATVP|PCOK|PMTP)\b`,

		// Video codecs
		`\bH\s26[456]\b`,  // H 264, H 265, H 266
		`\b(x264|x265|x266|HEVC|AVC|AV1)\b`,
		`\b(XviD|DivX|MPEG2|VC-1|VP9)\b`,

		// Special editions
		`\b(IMAX\s?Enhanced|IMAX|Remastered|REMASTERED)\b`,
		`\b(Directors\s?Cut|DC|Theatrical|UNCUT|Criterion)\b`,

		// Multi-language
		`\b(MULTI|DUAL|DL|DUBBED|SUBBED)\b`,

		// Release tags
		`\b(PROPER|REPACK|iNTERNAL|INTERNAL|LiMiTED|LIMITED|UNRATED|EXTENDED)\b`,

		// Version tags
		`\bv\d+\b`,  // v2, v3, v4

		// Release group suffix
		`\s?-\s?[A-Za-z0-9]+(\s[A-Za-z0-9]+)*$`,

		// Bracketed content
		`\[.*?\]`,
	}

	releasePatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		releasePatterns = append(releasePatterns, regexp.MustCompile(`(?i)`+pattern))
	}

	// Pre-compile commonly used regexes
	collapseSpacesRegex = regexp.MustCompile(`\s+`)
	removePunctRegex = regexp.MustCompile(`[^\w\s]`)
	yearParenRegex = regexp.MustCompile(`\((\d{4})\)`)
	yearBracketRegex = regexp.MustCompile(`\[(\d{4})\]`)
	yearDotRegex = regexp.MustCompile(`\.(\d{4})\.`)
	yearSpaceRegex = regexp.MustCompile(`\s(\d{4})(?:\s|$)`)

	// Year removal regexes (without capture groups)
	yearRemovePatterns := []string{
		`\(\d{4}\)`,  // (2025)
		`\[\d{4}\]`,  // [2025]
		`\.\d{4}\.`,  // .2025.
		`\s\d{4}\s`,  // " 2025 "
		`^\d{4}\s`,   // "2025 " at start
		`\s\d{4}$`,   // " 2025" at end
	}
	yearRemoveRegexes = make([]*regexp.Regexp, 0, len(yearRemovePatterns))
	for _, pattern := range yearRemovePatterns {
		yearRemoveRegexes = append(yearRemoveRegexes, regexp.MustCompile(pattern))
	}

	episodeSERegex = regexp.MustCompile(`[Ss](\d{1,2})[Ee](\d{1,2})`)
	episodeXRegex = regexp.MustCompile(`(\d{1,2})x(\d{1,2})`)
}

// NormalizeName normalizes a media name for fuzzy matching
// Handles case, punctuation, roman numerals, word substitutions
func NormalizeName(name string) string {
	// Strip release group info first (includes resolution)
	name = StripReleaseGroup(name)

	// Remove year if present
	name = removeYear(name)

	// Lowercase
	name = strings.ToLower(name)

	// Roman numeral to number conversion
	romanMap := map[string]string{
		" ii ":   " 2 ",
		" iii ":  " 3 ",
		" iv ":   " 4 ",
		" vi ":   " 6 ",
		" vii ":  " 7 ",
		" viii ": " 8 ",
		" ix ":   " 9 ",
	}

	for roman, num := range romanMap {
		name = strings.ReplaceAll(name, roman, num)
	}

	// Word substitutions for common variations
	substitutions := map[string]string{
		" and ":    " & ",
		" versus ": " vs ",
		" vs. ":    " vs ",
		" part ":   " pt ",
		" pt. ":    " pt ",
	}

	for old, new := range substitutions {
		name = strings.ReplaceAll(name, old, new)
	}

	// Remove punctuation (keep only alphanumeric and spaces)
	name = removePunctRegex.ReplaceAllString(name, " ")

	// Collapse multiple spaces
	name = collapseSpacesRegex.ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

// ExtractYear extracts year from various formats
// Uses pre-compiled regexes for performance
func ExtractYear(name string) string {
	// Try different year patterns (in order of specificity)
	yearRegexes := []*regexp.Regexp{
		yearParenRegex,   // (2025)
		yearBracketRegex, // [2025]
		yearDotRegex,     // .2025.
		yearSpaceRegex,   // 2025 (with space)
	}

	for _, re := range yearRegexes {
		matches := re.FindStringSubmatch(name)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// removeYear removes year from name (helper for NormalizeName)
// Uses pre-compiled regexes for performance
func removeYear(name string) string {
	// Apply all year removal patterns
	for _, re := range yearRemoveRegexes {
		name = re.ReplaceAllString(name, "")
	}

	return name
}

// ExtractResolution extracts resolution from name (1080p, 720p, etc.)
func ExtractResolution(name string) string {
	nameUpper := strings.ToUpper(name)

	// Check in order of preference (highest first)
	if strings.Contains(nameUpper, "2160P") || strings.Contains(nameUpper, "4K") || strings.Contains(nameUpper, "UHD") {
		return "2160p"
	}
	if strings.Contains(nameUpper, "1080P") {
		return "1080p"
	}
	if strings.Contains(nameUpper, "720P") {
		return "720p"
	}
	if strings.Contains(nameUpper, "480P") {
		return "480p"
	}

	return "unknown"
}

// StripReleaseGroup removes release group markers from name
// Uses pre-compiled regexes for performance
func StripReleaseGroup(name string) string {
	// Replace dots and underscores with spaces FIRST
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Apply all pre-compiled release group patterns
	for _, re := range releasePatterns {
		name = re.ReplaceAllString(name, " ")
	}

	// Collapse spaces using pre-compiled regex
	name = collapseSpacesRegex.ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

// SimilarityRatio calculates similarity between two strings (0.0 to 1.0)
// Uses a simple character-based approach similar to SequenceMatcher
func SimilarityRatio(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Use longest common subsequence approach
	longer, shorter := s1, s2
	if len(s1) < len(s2) {
		longer, shorter = s2, s1
	}

	longerLen := len(longer)
	if longerLen == 0 {
		return 1.0
	}

	// Calculate edit distance (Levenshtein)
	distance := levenshteinDistance(longer, shorter)

	// Convert to similarity ratio
	return (float64(longerLen) - float64(distance)) / float64(longerLen)
}

// levenshteinDistance calculates edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	len1 := len(s1)
	len2 := len(s2)

	// Create matrix
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	// Initialize first row and column
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len1][len2]
}

// min returns minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// CleanMovieName converts release group folder to clean Jellyfin format
// Example: "Movie.Name.2024.1080p.BluRay.x264-GROUP" -> "Movie Name (2024)"
func CleanMovieName(name string) string {
	// Extract year first (before any modifications)
	year := ExtractYear(name)

	// Strip release group info FIRST (handles dots, resolution, codecs, etc.)
	name = StripReleaseGroup(name)

	// Now remove year from the cleaned name
	name = removeYear(name)

	// Collapse multiple spaces and trim
	name = collapseSpacesRegex.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Title case (using cases.Title instead of deprecated strings.Title)
	caser := cases.Title(language.English)
	name = caser.String(name)

	// Add year if found
	if year != "" {
		return name + " (" + year + ")"
	}

	return name
}

// ExtractEpisodeInfo extracts S##E## from filename
// Returns season, episode, and whether pattern was found
// Uses pre-compiled regexes for performance
func ExtractEpisodeInfo(filename string) (season int, episode int, found bool) {
	// Try S##E## pattern
	matches := episodeSERegex.FindStringSubmatch(filename)

	if len(matches) > 2 {
		// Parse season and episode
		var s, e int
		if _, err := fmt.Sscanf(matches[1], "%d", &s); err == nil {
			if _, err := fmt.Sscanf(matches[2], "%d", &e); err == nil {
				return s, e, true
			}
		}
	}

	// Try #x## pattern (e.g., 1x01)
	matches = episodeXRegex.FindStringSubmatch(filename)

	if len(matches) > 2 {
		var s, e int
		if _, err := fmt.Sscanf(matches[1], "%d", &s); err == nil {
			if _, err := fmt.Sscanf(matches[2], "%d", &e); err == nil {
				return s, e, true
			}
		}
	}

	return 0, 0, false
}
