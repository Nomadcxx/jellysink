package scanner

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Pre-compiled regexes for performance optimization
var (
	releasePatterns     []*regexp.Regexp
	collapseSpacesRegex *regexp.Regexp
	removePunctRegex    *regexp.Regexp
	yearParenRegex      *regexp.Regexp
	yearBracketRegex    *regexp.Regexp
	yearDotRegex        *regexp.Regexp
	yearSpaceRegex      *regexp.Regexp
	yearRemoveRegexes   []*regexp.Regexp
	episodeSERegex      *regexp.Regexp
	episodeXRegex       *regexp.Regexp
)

func init() {
	// Pre-compile all release group patterns
	patterns := []string{
		// Resolution markers
		`\b\d{3,4}[pi]\b`, // 1080p, 720p, 2160p, 480i, 576i
		`\b(4K|UHD)\b`,    // 4K, UHD

		// HDR formats (before generic HDR to catch specific variants)
		`\b(HDR10\+?|HDR10Plus|Dolby\s?Vision|DoVi|DV|HDR|HLG|PQ|SDR)\b`,

		// Audio formats with channels (most specific first)
		`\b(DTS-HD\s?MA|DTS-HD\s?HRA|DTS-HD|DTS-X|DTS-ES)\b`, // DTS variants
		`\b(DD\+?|DDP|E?AC3|AAC|AC3)\d\s\d\b`,                // Audio with channels
		`\b(DD\+?|DDP|E?AC3|AAC|AC3)\b`,                      // Audio without channels
		`\b(TrueHD|Atmos|FLAC|PCM|Opus|MP3|DTS)\b`,           // Other audio codecs
		`\d+Audio`,                             // Orphaned audio markers like "3Audio"
		`MA\d+\s\d+`,                           // Orphaned MA markers like "MA5 1"
		`\b\d\.\d\b`,                           // Matches 5.1, 7.1 etc.
		`\bHD\b`,                               // Remove solitary HD tokens
		`\bCBR\b`,                              // Remove CBR quality token
		`\b(DUAL|DUAL-CBR|DUAL-ENC|CBR|CRF)\b`, // Remove DUAL/quality tokens

		// Audio channels (after audio codecs)
		`\b\d\s\d\b`, // 7 1, 5 1, 2 0
		`\b\d\.\d\b`, // Matches 5.1, 7.1 etc.
		`\b(Stereo|Mono)\b`,
		// Commentary markers
		`\b(Plus Commentary|Commentary|Audio Commentary)\b`,
		`\bExtended Commentary\b`,
		// Locale tags (e.g., NORDiC)
		`\b(NORDiC|NF|ATVP|HULU)\b`,

		// Source types
		`\b(BluRay|Blu-ray|BDRip|BRRip|REMUX|WEB-DL|WEBDL|WEBRip|WEB)\b`,
		`\b(HDTV|PDTV|SDTV|DVDRip|DVD|DVDSCR)\b`,
		`\b(CAM|HDTS|TS|TC|SCR|R5)\b`,

		// Streaming platforms
		`\b(AMZN|NF|DSNP|HMAX|HULU|ATVP|PCOK|PMTP)\b`,

		// Locale/language and subtitle markers
		`\b(ITA|FRE|FRA|ENG|EN|ESP|ES|SPA|SUB|SUBS|SUBBED|DUB|DUBBED|DUAL|MULTI)\b`,

		// Video codecs (both spaced and non-spaced variants)
		`\bH\s?26[456]\b`, // H 264, H264, H 265, H265, etc.
		`\b(x264|x265|x266|HEVC|AVC|AV1|H264|H265|H266)\b`,
		`\b(XviD|DivX|MPEG2|VC-1|VP9)\b`,

		// Special editions
		`\b(IMAX\s?Enhanced|IMAX|Remastered|REMASTERED)\b`,
		`\b(Directors\s?Cut|DC|Theatrical|UNCUT|Criterion)\b`,

		// Multi-language and subtitles
		`\b(MULTI|DUAL|DL|DUBBED|SUBBED|MSubs|Subs)\b`,

		// Release tags
		`\b(PROPER|REPACK|iNTERNAL|INTERNAL|LiMiTED|LIMITED|UNRATED|EXTENDED)\b`,

		// Version tags
		`\bv\d+\b`, // v2, v3, v4

		// Parenthesized markers (iso, rip, etc.)
		`\((?:iso|rip|cd\d|disc\d|disk\d)\)`,

		// Release group suffix (e.g., "-GROUP" or "~ TombDoc") and hyphenated release groups
		`\s?[-~]\s?[A-Za-z0-9]+(\s[A-Za-z0-9]+)*$`,
		`-[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`,
		`\b(PSYCHD|MAG|CHAMELE0N|MIRCREW|MIRC|WILL1869|ASPiDe|CI?NEMIX|CiNEMiX|CINEMIX|MIRCREW)\b`,
		// Hyphenated tokens attached to codecs (x264-...) catch any trailing hyphen groups
		`[A-Za-z0-9]+-[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`,

		// Bracketed content (e.g., "[Org BD 2.0 Hindi + DD 5.1 English]")
		`\[.*?\]`,

		// Bit depth
		`\b(8bit|10bit|12bit)\b`,
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
		`\(\d{4}\)`, // (2025)
		`\[\d{4}\]`, // [2025]
		`\.\d{4}\.`, // .2025.
		`\s\d{4}\s`, // " 2025 "
		`^\d{4}\s`,  // "2025 " at start
		`\s\d{4}$`,  // " 2025" at end
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
// When multiple years are present, returns the LAST one (e.g., "Blade Runner 2049 2017" -> "2017")
func ExtractYear(name string) string {
	// Find all 4-digit numbers that could be years (1900-2099)
	// Use negative lookbehind/lookahead to avoid matching resolution markers
	allDigitsRegex := regexp.MustCompile(`\b(\d{4})\b`)
	matches := allDigitsRegex.FindAllStringSubmatch(name, -1)

	// Known resolution values to skip
	resolutions := map[string]bool{
		"2160": true, // 4K
		"1920": true, // 1080p width
		"1440": true, // 2K
		"1280": true, // 720p width
	}

	var validYears []string
	for _, match := range matches {
		if len(match) > 1 {
			year := match[1]

			// Skip known resolution values
			if resolutions[year] {
				continue
			}

			// Only consider valid year range (1900-2099)
			if year >= "1900" && year <= "2099" {
				validYears = append(validYears, year)
			}
		}
	}

	// Return the last valid year (most likely the release year, not title year)
	if len(validYears) > 0 {
		return validYears[len(validYears)-1]
	}

	return ""
}

// removeYear removes year from name (helper for NormalizeName)
// Uses pre-compiled regexes for performance
func removeYear(name string) string {
	// Apply all year removal patterns, replacing with space to prevent concatenation
	for _, re := range yearRemoveRegexes {
		name = re.ReplaceAllString(name, " ")
	}

	return name
}

// removeSpecificYear removes only the specified year from name (helper for CleanMovieName)
// This preserves years in movie titles (e.g., "Blade Runner 2049" keeps 2049)
func removeSpecificYear(name, year string) string {
	if year == "" {
		return name
	}

	// Try to remove year in various formats
	patterns := []string{
		`\(` + year + `\)`,       // (2025)
		`\[` + year + `\]`,       // [2025]
		`\.` + year + `\.`,       // .2025.
		`\s` + year + `(?:\s|$)`, // " 2025 " or " 2025" at end
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		name = re.ReplaceAllString(name, " ")
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
	// Replace dots, underscores and hyphens with spaces FIRST to separate tokens
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

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

// stripOrphanedReleaseGroups removes common release group names that remain after stripping markers
// These are typically alphanumeric strings with mixed case (e.g., "D3FiL3R", "RARBG", "YTS")
func stripOrphanedReleaseGroups(name string) string {
	// Common release group patterns (exact matches only, case-insensitive)
	knownGroups := map[string]bool{
		"rarbg": true, "yts": true, "yify": true, "etrg": true, "fgt": true, "mkvcage": true,
		"stuttershit": true, "sparks": true, "rovers": true, "phoenix": true, "cmrg": true,
		"evo": true, "ion10": true, "psa": true, "afg": true, "sampa": true, "tgx": true, "snake": true,
		"d3fil3r": true, "fistworld": true, "crys": true, "handjob": true, "rightsize": true,
		"ntb": true, "ntg": true, "getit": true, "pignus": true, "btn": true, "don": true, "ctrlhd": true,
		"mag": true, "psychd": true, "ml": true, "mirc": true, "mircrew": true, "chameleon": true, "cinemix": true,
		"will1869": true, "aspide": true, "nueng": true,
	}

	words := strings.Fields(name)

	// Only remove known groups from the tail. This avoids stripping title words in the middle.
	for len(words) > 0 {
		last := words[len(words)-1]
		lastLower := strings.ToLower(last)

		// If the last word is in the known groups, remove it
		if _, ok := knownGroups[lastLower]; ok {
			words = words[:len(words)-1]
			continue
		}

		// Heuristics: remove if the last token looks like a release group (digits+letters or ALL CAPS)
		hasDigit := false
		hasLetter := false
		hasLower := false
		hasUpper := false
		for _, ch := range last {
			if ch >= '0' && ch <= '9' {
				hasDigit = true
			} else if ch >= 'a' && ch <= 'z' {
				hasLetter = true
				hasLower = true
			} else if ch >= 'A' && ch <= 'Z' {
				hasLetter = true
				hasUpper = true
			}
		}

		// Language tags like ITA, Fre, ENG, etc.
		langTags := map[string]bool{"ita": true, "ita.": true, "fre": true, "fra": true, "eng": true, "en": true, "esp": true, "spa": true}
		if langTags[lastLower] {
			words = words[:len(words)-1]
			continue
		}

		// Remove if last token is short uppercase or contains letter+digit mix
		if (hasUpper && !hasLower && len(last) <= 4) || (hasLetter && hasDigit) {
			words = words[:len(words)-1]
			continue
		}

		// Stop removing any further tokens if the last word looks like a title word
		break
	}

	return strings.Join(words, " ")
}

// CleanMovieName converts release group folder to clean Jellyfin format
// Example: "Movie.Name.2024.1080p.BluRay.x264-GROUP" -> "Movie Name (2024)"
func CleanMovieName(name string) string {
	// Strip file extension FIRST (if present)
	ext := strings.ToLower(filepath.Ext(name))
	videoExts := []string{".mkv", ".mp4", ".avi", ".m4v", ".mov", ".wmv", ".flv", ".webm", ".mpg", ".mpeg"}
	for _, ve := range videoExts {
		if ext == ve {
			name = strings.TrimSuffix(name, ext)
			break
		}
	}

	// Extract year first (before any modifications)
	year := ExtractYear(name)

	// If year exists, only keep the part of the string before the year.
	// This removes resolution/codecs/release group tokens that come AFTER the year.
	if year != "" {
		idx := strings.LastIndex(name, year)
		if idx != -1 {
			// Move index left to strip preceding punctuation like '(' '[' '.' '-' and whitespace
			startIdx := idx
			for startIdx > 0 {
				ch := name[startIdx-1]
				if ch == '(' || ch == '[' || ch == '.' || ch == ' ' || ch == '_' || ch == '-' {
					startIdx--
				} else {
					break
				}
			}
			name = strings.TrimSpace(name[:startIdx])
		}
	}

	// Strip release group info (handles dots, resolution, codecs, etc.)
	name = StripReleaseGroup(name)

	// Remove only the specific release year (preserves years in titles like "2049")
	name = removeSpecificYear(name, year)

	// Collapse multiple spaces and trim
	name = collapseSpacesRegex.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Strip orphaned release groups that weren't caught by patterns
	name = stripOrphanedReleaseGroups(name)

	// Trim again after orphan removal
	name = strings.TrimSpace(name)

	// Title case with custom handling for ordinals
	name = titleCaseWithOrdinals(name)

	// Add year if found
	if year != "" {
		return name + " (" + year + ")"
	}

	return name
}

// titleCaseWithOrdinals applies title case while preserving ordinal numbers (1st, 2nd, 25th, etc.)
func titleCaseWithOrdinals(s string) string {
	// Case-insensitive ordinal detection
	ordinalRegex := regexp.MustCompile(`(?i)\b(\d+)(st|nd|rd|th)\b`)

	// Find all ordinals and their positions
	type ordinalMatch struct {
		original string
		number   string
		suffix   string
	}

	matches := ordinalRegex.FindAllStringSubmatch(s, -1)
	ordinals := make([]ordinalMatch, len(matches))

	// Store ordinals before title casing
	for i, match := range matches {
		if len(match) > 2 {
			ordinals[i] = ordinalMatch{
				original: match[0],
				number:   match[1],
				suffix:   match[2],
			}
		}
	}

	// Replace ordinals with unique placeholders (use special chars to avoid title-casing)
	for i, ord := range ordinals {
		placeholder := fmt.Sprintf("§§§%d§§§", i)
		s = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(ord.original)).ReplaceAllString(s, placeholder)
	}

	// Apply title case
	caser := cases.Title(language.English)
	s = caser.String(s)

	// Restore ordinals with lowercase suffix
	for i, ord := range ordinals {
		placeholder := fmt.Sprintf("§§§%d§§§", i)
		restored := ord.number + strings.ToLower(ord.suffix)
		s = strings.ReplaceAll(s, placeholder, restored)
	}

	return s
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
