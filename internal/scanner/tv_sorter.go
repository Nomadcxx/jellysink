package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TVTitleMatch represents a potential title match with confidence score
type TVTitleMatch struct {
	Title      string  // Extracted title
	Source     string  // "folder" or "filename"
	Confidence float64 // 0.0 to 1.0 (higher = more confident)
	Year       string  // Extracted year if present
}

// TVTitleResolution contains the resolved show name and metadata
type TVTitleResolution struct {
	ResolvedTitle string        // Final canonical title
	FolderMatch   *TVTitleMatch // Title from folder name
	FilenameMatch *TVTitleMatch // Title from filename
	IsAmbiguous   bool          // True if needs manual review
	Confidence    float64       // Overall confidence (0.0 to 1.0)
	APIVerified   bool          // True if verified via TVDB/OMDB
	Reason        string        // Explanation for resolution choice
}

// ExtractTVShowTitle extracts show title from folder or filename
// Examples:
//   - "Degrassi (2001)" → "Degrassi", "2001"
//   - "Degrassi The Next Generation_S07E12_Live To Tell.mkv" → "Degrassi The Next Generation", ""
//   - "Star.Trek.TNG.S01E01.720p.mkv" → "Star Trek TNG", ""
func ExtractTVShowTitle(name string) (title string, year string) {
	// Remove file extension if present
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Extract year first (if in parentheses) - BEFORE any cleaning
	year = extractYearFromTitle(name)
	if year != "" {
		// Remove year from name
		name = regexp.MustCompile(`\s*\(`+year+`\)`).ReplaceAllString(name, "")
	}

	// Clean up episode titles (after underscores, dashes at end) BEFORE removing episode patterns
	// This prevents the episode title from being included
	name = removeEpisodeTitles(name)

	// Remove season/episode patterns (S01E01, 1x01, etc.)
	name = removeEpisodePatterns(name)

	// Remove quality markers and release group info (but NOT abbreviations)
	// Use StripReleaseGroup which handles dots properly
	name = StripReleaseGroup(name)

	// Title case the result
	name = titleCaseWithOrdinals(name)

	// Collapse spaces and trim
	name = strings.TrimSpace(collapseSpacesRegex.ReplaceAllString(name, " "))

	return name, year
}

// extractYearFromTitle extracts year from title (year must be in parentheses)
func extractYearFromTitle(name string) string {
	// Match year in parentheses: (2001), (2020), etc.
	// May have dots/spaces before parenthesis (e.g., "S.H.I.E.L.D. (2013)")
	re := regexp.MustCompile(`[.\s]*\((\d{4})\)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 1 {
		year := matches[1]
		// Validate year range (1900-2099)
		if year >= "1900" && year <= "2099" {
			return year
		}
	}
	return ""
}

// removeEpisodePatterns removes episode numbering from filename
func removeEpisodePatterns(name string) string {
	// Remove S##E## patterns
	name = episodeSERegex.ReplaceAllString(name, " ")
	// Remove #x## patterns (1x01, 12x34)
	name = episodeXRegex.ReplaceAllString(name, " ")
	// Remove episode titles after common separators
	// (e.g., "_Live To Tell", "- The Pilot", ".Part.1")
	return name
}

// removeEpisodeTitles removes episode titles that come after the show name
// These typically follow patterns like "_Title", "- Title", or multiple dots
func removeEpisodeTitles(name string) string {
	// Find first S##E## pattern - everything after is episode info
	loc := episodeSERegex.FindStringIndex(name)
	if loc != nil {
		// Found S##E## pattern - strip everything after it
		// Keep everything before the pattern
		name = name[:loc[0]]
		return strings.TrimSpace(name)
	}

	// No S##E## pattern found - check for other separators
	// If ends with "- Something", remove it ONLY if it looks like an episode title
	if idx := strings.LastIndex(name, " - "); idx != -1 {
		after := name[idx+3:]
		// Keep if it looks like a show subtitle, remove if it looks like episode title
		if !looksLikeShowSubtitle(after) {
			name = name[:idx]
		}
	}

	return strings.TrimSpace(name)
}

// looksLikeShowSubtitle checks if a string looks like a show subtitle vs episode title
// Show subtitles: "The Series", "The Next Generation", "The Animated Series"
// Episode titles: "The Pilot", "Live To Tell", "Part 1"
func looksLikeShowSubtitle(s string) bool {
	s = strings.ToLower(s)

	// Common show subtitle patterns
	showSubtitleWords := []string{
		"the series",
		"the animated series",
		"the next generation",
		"the original series",
		"deep space nine",
		"voyager",
		"enterprise",
	}

	for _, pattern := range showSubtitleWords {
		if strings.Contains(s, pattern) {
			return true
		}
	}

	// Short phrases (1-3 words) with articles often indicate show subtitles
	words := strings.Fields(s)
	if len(words) <= 3 {
		// Check for "The X" patterns
		if len(words) >= 2 && words[0] == "the" {
			return true
		}
	}

	return false
}

// ResolveTVShowTitle compares folder and filename titles to determine canonical name
func ResolveTVShowTitle(filePath, libRoot string) *TVTitleResolution {
	filename := filepath.Base(filePath)
	showDir := filepath.Base(filepath.Dir(filepath.Dir(filePath)))

	// Extract titles from both sources
	folderTitle, folderYear := ExtractTVShowTitle(showDir)
	filenameTitle, filenameYear := ExtractTVShowTitle(filename)

	// Calculate confidence scores
	folderMatch := &TVTitleMatch{
		Title:      folderTitle,
		Source:     "folder",
		Confidence: calculateTitleConfidence(folderTitle, showDir),
		Year:       folderYear,
	}

	filenameMatch := &TVTitleMatch{
		Title:      filenameTitle,
		Source:     "filename",
		Confidence: calculateTitleConfidence(filenameTitle, filename),
		Year:       filenameYear,
	}

	// Determine resolution strategy
	resolution := &TVTitleResolution{
		FolderMatch:   folderMatch,
		FilenameMatch: filenameMatch,
		APIVerified:   false,
	}

	// Case 1: Titles are identical (or very similar)
	if strings.EqualFold(folderTitle, filenameTitle) {
		resolution.ResolvedTitle = folderTitle
		resolution.Confidence = folderMatch.Confidence
		resolution.IsAmbiguous = false
		resolution.Reason = "Folder and filename titles match"
		return resolution
	}

	// Case 2: Filename title is significantly longer - check if it's just episode title appended
	if len(filenameTitle) > len(folderTitle)+5 {
		// Special case: Very short folder names (< 5 chars) are likely truncated/abbreviated
		// Always mark as ambiguous and prefer the longer filename
		if len(folderTitle) < 5 {
			resolution.ResolvedTitle = filenameTitle
			resolution.Confidence = filenameMatch.Confidence * 0.8
			resolution.IsAmbiguous = true
			resolution.Reason = fmt.Sprintf("Filename has longer title (%d chars vs %d chars)", len(filenameTitle), len(folderTitle))
			return resolution
		}

		// Check if filename starts with folder title followed by a word boundary
		// This handles: "A Place To Call Home" (folder) vs "A Place To Call Home The Prodigal Daughter" (filename)
		// But NOT: "Degrassi" vs "Degrassi The Next Generation" (show subtitle, should be ambiguous)
		folderLower := strings.ToLower(folderTitle)
		filenameLower := strings.ToLower(filenameTitle)

		if strings.HasPrefix(filenameLower, folderLower) {
			// Check if there's a space after the folder title (word boundary)
			if len(filenameLower) > len(folderLower) && filenameLower[len(folderLower)] == ' ' {
				// Extract the "extra" text after the folder title
				extraText := filenameTitle[len(folderTitle)+1:] // +1 to skip the space

				// Check if extra text looks like a show subtitle (e.g., "The Next Generation")
				// If yes, mark as ambiguous (user should confirm)
				// If no, it's likely an episode title that wasn't stripped
				if looksLikeShowSubtitle(extraText) {
					// Likely a show subtitle - mark ambiguous, prefer filename (longer/more complete)
					resolution.ResolvedTitle = filenameTitle
					resolution.Confidence = filenameMatch.Confidence * 0.8
					resolution.IsAmbiguous = true
					resolution.Reason = fmt.Sprintf("Filename has longer title (%d chars vs %d chars)", len(filenameTitle), len(folderTitle))
					return resolution
				}

				// Extra text is episode title, use folder title
				resolution.ResolvedTitle = folderTitle
				resolution.Confidence = folderMatch.Confidence
				resolution.IsAmbiguous = false
				resolution.Reason = "Filename includes episode title after show name"
				return resolution
			}
		}

		// Filename has different/longer title (not just appended episode name)
		resolution.ResolvedTitle = filenameTitle
		resolution.Confidence = filenameMatch.Confidence * 0.8 // Reduce confidence slightly
		resolution.IsAmbiguous = true
		resolution.Reason = fmt.Sprintf("Filename has longer title (%d chars vs %d chars)", len(filenameTitle), len(folderTitle))
		return resolution
	}

	// Case 3: Folder title is longer (use folder, but mark ambiguous)
	if len(folderTitle) > len(filenameTitle)+5 {
		resolution.ResolvedTitle = folderTitle
		resolution.Confidence = folderMatch.Confidence * 0.8
		resolution.IsAmbiguous = true
		resolution.Reason = fmt.Sprintf("Folder has longer title (%d chars vs %d chars)", len(folderTitle), len(filenameTitle))
		return resolution
	}

	// Case 4: Titles differ but similar length - ambiguous, needs API verification
	resolution.ResolvedTitle = folderTitle // Default to folder
	resolution.Confidence = 0.5            // Low confidence
	resolution.IsAmbiguous = true
	resolution.Reason = fmt.Sprintf("Conflicting titles: '%s' (folder) vs '%s' (filename)", folderTitle, filenameTitle)

	return resolution
}

// calculateTitleConfidence estimates how confident we are in the extracted title
func calculateTitleConfidence(title, original string) float64 {
	confidence := 1.0

	// Penalty for very short titles (likely truncated)
	if len(title) < 3 {
		confidence -= 0.5
	}

	// Penalty for single-word titles (often incomplete)
	if !strings.Contains(title, " ") {
		confidence -= 0.3
	}

	// Bonus for year presence in original
	if strings.Contains(original, "(") && strings.Contains(original, ")") {
		confidence += 0.1
	}

	// Penalty if original has lots of release markers (likely not cleaned well)
	releaseMarkers := []string{"1080p", "720p", "x264", "x265", "BluRay", "WEB-DL"}
	for _, marker := range releaseMarkers {
		if strings.Contains(strings.ToUpper(original), marker) {
			confidence -= 0.1
			break
		}
	}

	// Clamp to 0.0-1.0 range
	if confidence < 0.0 {
		confidence = 0.0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// GetAmbiguousTVShows returns a list of TV shows that need manual review
func GetAmbiguousTVShows(resolutions []*TVTitleResolution) []*TVTitleResolution {
	var ambiguous []*TVTitleResolution
	for _, res := range resolutions {
		if res.IsAmbiguous && !res.APIVerified {
			ambiguous = append(ambiguous, res)
		}
	}
	return ambiguous
}

// TVDBSearchResult represents a search result from TVDB API
type TVDBSearchResult struct {
	Status string       `json:"status"`
	Data   []TVDBSeries `json:"data"`
}

// TVDBSeries represents a TV series from TVDB
type TVDBSeries struct {
	ObjectID     string   `json:"objectID"`
	ID           string   `json:"id"`
	TVDBID       string   `json:"tvdb_id"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	FirstAirTime string   `json:"first_air_time"`
	Overview     string   `json:"overview"`
	Year         string   `json:"year"`
	PrimaryType  string   `json:"primary_type"`
	Type         string   `json:"type"`
}

// TVDBClient handles TVDB API requests
type TVDBClient struct {
	APIKey     string
	Token      string
	HTTPClient *http.Client
}

// TVDBLoginResponse represents the login response from TVDB
type TVDBLoginResponse struct {
	Status string `json:"status"`
	Data   struct {
		Token string `json:"token"`
	} `json:"data"`
}

// NewTVDBClient creates a new TVDB API client
func NewTVDBClient(apiKey string) *TVDBClient {
	return &TVDBClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Login authenticates with TVDB and retrieves a JWT token
func (c *TVDBClient) Login() error {
	if c.APIKey == "" {
		return fmt.Errorf("TVDB API key not configured")
	}

	loginURL := "https://api4.thetvdb.com/v4/login"

	payload := map[string]string{
		"apikey": c.APIKey,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", loginURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login returned status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp TVDBLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResp.Data.Token == "" {
		return fmt.Errorf("no token returned from login")
	}

	c.Token = loginResp.Data.Token
	return nil
}

// SearchSeries searches TVDB for a series by name
func (c *TVDBClient) SearchSeries(name string) ([]TVDBSeries, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("TVDB API key not configured")
	}

	// Login to get token if we don't have one
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		}
	}

	encodedName := url.QueryEscape(name)
	apiURL := fmt.Sprintf("https://api4.thetvdb.com/v4/search?query=%s&type=series", encodedName)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result TVDBSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}

// OMDBClient handles OMDB API requests
type OMDBClient struct {
	APIKey     string
	HTTPClient *http.Client
}

// OMDBSeries represents a TV series from OMDB
type OMDBSeries struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	Type   string `json:"Type"`
	Error  string `json:"Error,omitempty"`
}

// NewOMDBClient creates a new OMDB API client
func NewOMDBClient(apiKey string) *OMDBClient {
	return &OMDBClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SearchSeries searches OMDB for a series by name
func (c *OMDBClient) SearchSeries(name string) (*OMDBSeries, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("OMDB API key not configured")
	}

	encodedName := url.QueryEscape(name)
	apiURL := fmt.Sprintf("https://www.omdbapi.com/?t=%s&type=series&apikey=%s", encodedName, c.APIKey)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result OMDBSeries
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("OMDB error: %s", result.Error)
	}

	return &result, nil
}

// VerifyTVShowTitle uses TVDB (with OMDB fallback) to verify and resolve a TV show title
func VerifyTVShowTitle(resolution *TVTitleResolution, tvdbKey, omdbKey string) error {
	if tvdbKey == "" && omdbKey == "" {
		return fmt.Errorf("no API keys configured (TVDB or OMDB required)")
	}

	// Try TVDB first
	if tvdbKey != "" {
		if err := verifyWithTVDB(resolution, tvdbKey); err == nil {
			return nil
		}
	}

	// Fallback to OMDB if TVDB fails
	if omdbKey != "" {
		if err := verifyWithOMDB(resolution, omdbKey); err == nil {
			return nil
		}
	}

	return fmt.Errorf("both TVDB and OMDB verification failed")
}

// verifyWithTVDB uses TVDB API to verify title
func verifyWithTVDB(resolution *TVTitleResolution, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("TVDB API key not configured")
	}

	client := NewTVDBClient(apiKey)

	folderResults, err := client.SearchSeries(resolution.FolderMatch.Title)
	if err != nil {
		return fmt.Errorf("failed to search folder title: %w", err)
	}

	filenameResults, err := client.SearchSeries(resolution.FilenameMatch.Title)
	if err != nil {
		return fmt.Errorf("failed to search filename title: %w", err)
	}

	if len(folderResults) == 0 && len(filenameResults) == 0 {
		return fmt.Errorf("no results found for either title")
	}

	if len(folderResults) > 0 && len(filenameResults) == 0 {
		resolution.ResolvedTitle = folderResults[0].Name
		resolution.APIVerified = true
		resolution.IsAmbiguous = false
		resolution.Confidence = 0.95
		resolution.Reason = fmt.Sprintf("TVDB verified: '%s' (folder match, no filename match)", folderResults[0].Name)
		return nil
	}

	if len(filenameResults) > 0 && len(folderResults) == 0 {
		resolution.ResolvedTitle = filenameResults[0].Name
		resolution.APIVerified = true
		resolution.IsAmbiguous = false
		resolution.Confidence = 0.95
		resolution.Reason = fmt.Sprintf("TVDB verified: '%s' (filename match, no folder match)", filenameResults[0].Name)
		return nil
	}

	if len(folderResults) > 0 && len(filenameResults) > 0 {
		if folderResults[0].ID == filenameResults[0].ID {
			resolution.ResolvedTitle = folderResults[0].Name
			resolution.APIVerified = true
			resolution.IsAmbiguous = false
			resolution.Confidence = 1.0
			resolution.Reason = fmt.Sprintf("TVDB verified: '%s' (both match same series)", folderResults[0].Name)
			return nil
		}

		resolution.ResolvedTitle = folderResults[0].Name
		resolution.APIVerified = true
		resolution.IsAmbiguous = true
		resolution.Confidence = 0.6
		resolution.Reason = fmt.Sprintf("TVDB conflict: '%s' (folder) vs '%s' (filename) - different series", folderResults[0].Name, filenameResults[0].Name)
		return nil
	}

	return fmt.Errorf("unexpected API response")
}

// verifyWithOMDB uses OMDB API to verify title
func verifyWithOMDB(resolution *TVTitleResolution, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("OMDB API key not configured")
	}

	client := NewOMDBClient(apiKey)

	folderResult, folderErr := client.SearchSeries(resolution.FolderMatch.Title)
	filenameResult, filenameErr := client.SearchSeries(resolution.FilenameMatch.Title)

	if folderErr != nil && filenameErr != nil {
		return fmt.Errorf("no results found for either title")
	}

	if folderResult != nil && filenameResult == nil {
		resolution.ResolvedTitle = folderResult.Title
		resolution.APIVerified = true
		resolution.IsAmbiguous = false
		resolution.Confidence = 0.90
		resolution.Reason = fmt.Sprintf("OMDB verified: '%s' (folder match, no filename match)", folderResult.Title)
		return nil
	}

	if filenameResult != nil && folderResult == nil {
		resolution.ResolvedTitle = filenameResult.Title
		resolution.APIVerified = true
		resolution.IsAmbiguous = false
		resolution.Confidence = 0.90
		resolution.Reason = fmt.Sprintf("OMDB verified: '%s' (filename match, no folder match)", filenameResult.Title)
		return nil
	}

	if folderResult != nil && filenameResult != nil {
		if folderResult.ImdbID == filenameResult.ImdbID {
			resolution.ResolvedTitle = folderResult.Title
			resolution.APIVerified = true
			resolution.IsAmbiguous = false
			resolution.Confidence = 0.95
			resolution.Reason = fmt.Sprintf("OMDB verified: '%s' (both match same series)", folderResult.Title)
			return nil
		}

		resolution.ResolvedTitle = folderResult.Title
		resolution.APIVerified = true
		resolution.IsAmbiguous = true
		resolution.Confidence = 0.6
		resolution.Reason = fmt.Sprintf("OMDB conflict: '%s' (folder) vs '%s' (filename) - different series", folderResult.Title, filenameResult.Title)
		return nil
	}

	return fmt.Errorf("unexpected API response")
}
