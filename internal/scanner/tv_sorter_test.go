package scanner

import (
	"testing"
)

func TestExtractTVShowTitle(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTitle string
		expectedYear  string
	}{
		{
			name:          "Folder with year",
			input:         "Degrassi (2001)",
			expectedTitle: "Degrassi",
			expectedYear:  "2001",
		},
		{
			name:          "Filename with full title and episode",
			input:         "Degrassi The Next Generation_S07E12_Live To Tell.mkv",
			expectedTitle: "Degrassi The Next Generation",
			expectedYear:  "",
		},
		{
			name:          "Release group filename",
			input:         "Star.Trek.TNG.S01E01.720p.BluRay.x264-GROUP.mkv",
			expectedTitle: "Star Trek TNG",
			expectedYear:  "",
		},
		{
			name:          "Show with subtitle",
			input:         "Marvels Agents of SHIELD (2013)",
			expectedTitle: "Marvels Agents Of SHIELD",
			expectedYear:  "2013",
		},
		{
			name:          "Simple show name",
			input:         "Friends (1994)",
			expectedTitle: "Friends",
			expectedYear:  "1994",
		},
		{
			name:          "Show with dash separator",
			input:         "Breaking Bad - Pilot.mkv",
			expectedTitle: "Breaking Bad",
			expectedYear:  "",
		},
		{
			name:          "Show with The Series subtitle",
			input:         "US Marshals - The Series (2020)",
			expectedTitle: "US Marshals - The Series",
			expectedYear:  "2020",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, year := ExtractTVShowTitle(tt.input)
			if title != tt.expectedTitle {
				t.Errorf("ExtractTVShowTitle(%q) title = %q, want %q", tt.input, title, tt.expectedTitle)
			}
			if year != tt.expectedYear {
				t.Errorf("ExtractTVShowTitle(%q) year = %q, want %q", tt.input, year, tt.expectedYear)
			}
		})
	}
}

func TestResolveTVShowTitle(t *testing.T) {
	tests := []struct {
		name               string
		filePath           string
		expectedTitle      string
		shouldBeAmbiguous  bool
		expectLongerReason bool
	}{
		{
			name:               "Degrassi mismatch",
			filePath:           "/mnt/STORAGE7/TVSHOWS/Degrassi (2001)/Season 07/Degrassi The Next Generation_S07E12_Live To Tell.mkv",
			expectedTitle:      "Degrassi The Next Generation",
			shouldBeAmbiguous:  true,
			expectLongerReason: true,
		},
		{
			name:               "Matching titles",
			filePath:           "/mnt/STORAGE1/TVSHOWS/Friends (1994)/Season 01/Friends (1994) S01E01.mkv",
			expectedTitle:      "Friends",
			shouldBeAmbiguous:  false,
			expectLongerReason: false,
		},
		{
			name:               "Release group filename",
			filePath:           "/mnt/STORAGE1/TVSHOWS/Breaking Bad (2008)/Season 01/Breaking.Bad.S01E01.720p.BluRay.x264-GROUP.mkv",
			expectedTitle:      "Breaking Bad",
			shouldBeAmbiguous:  false,
			expectLongerReason: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution := ResolveTVShowTitle(tt.filePath, "/mnt/STORAGE7/TVSHOWS")

			if resolution.ResolvedTitle != tt.expectedTitle {
				t.Errorf("ResolveTVShowTitle(%q):\n  Expected: %q\n  Got:      %q\n  Reason:   %s",
					tt.filePath, tt.expectedTitle, resolution.ResolvedTitle, resolution.Reason)
			}

			if resolution.IsAmbiguous != tt.shouldBeAmbiguous {
				t.Errorf("ResolveTVShowTitle(%q) IsAmbiguous = %v, want %v (Reason: %s)",
					tt.filePath, resolution.IsAmbiguous, tt.shouldBeAmbiguous, resolution.Reason)
			}

			if tt.expectLongerReason && resolution.IsAmbiguous {
				if resolution.FilenameMatch == nil || resolution.FolderMatch == nil {
					t.Errorf("ResolveTVShowTitle(%q) missing match details", tt.filePath)
				}
				t.Logf("Folder: %q (%d chars, confidence: %.2f)",
					resolution.FolderMatch.Title, len(resolution.FolderMatch.Title), resolution.FolderMatch.Confidence)
				t.Logf("Filename: %q (%d chars, confidence: %.2f)",
					resolution.FilenameMatch.Title, len(resolution.FilenameMatch.Title), resolution.FilenameMatch.Confidence)
				t.Logf("Reason: %s", resolution.Reason)
			}
		})
	}
}

func TestLooksLikeShowSubtitle(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"The Series", true},
		{"The Next Generation", true},
		{"The Animated Series", true},
		{"The Pilot", true}, // Short "The X" pattern
		{"Live To Tell", false},
		{"Part 1", false},
		{"Episode Title", false},
		{"Deep Space Nine", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := looksLikeShowSubtitle(tt.input)
			if result != tt.expected {
				t.Errorf("looksLikeShowSubtitle(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractYearFromTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Show Name (2001)", "2001"},
		{"Show Name (2020)", "2020"},
		{"Show Name (1995)", "1995"},
		{"Show.Name.(2010)", "2010"},
		{"S.H.I.E.L.D. (2013)", "2013"},
		{"Show Name 2001", ""},
		{"Show Name [2001]", ""},
		{"Show (999)", ""},
		{"Show (3000)", ""},
		{"Show Name", ""},
	}

	for _, tt := range tests {
		result := extractYearFromTitle(tt.input)
		if result != tt.expected {
			t.Errorf("extractYearFromTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCalculateTitleConfidence(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		original  string
		minExpect float64
		maxExpect float64
	}{
		{
			name:      "Good quality extraction with year",
			title:     "Breaking Bad",
			original:  "Breaking Bad (2008)",
			minExpect: 0.9,
			maxExpect: 1.0,
		},
		{
			name:      "Short single-word title",
			title:     "X",
			original:  "X",
			minExpect: 0.0,
			maxExpect: 0.5,
		},
		{
			name:      "Title from release filename",
			title:     "Game Of Thrones",
			original:  "Game.of.Thrones.S01E01.1080p.BluRay.x264-GROUP.mkv",
			minExpect: 0.8,
			maxExpect: 1.0,
		},
		{
			name:      "Clean multi-word title",
			title:     "The Wire",
			original:  "The Wire S01E01.mkv",
			minExpect: 0.8,
			maxExpect: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTitleConfidence(tt.title, tt.original)
			if result < tt.minExpect || result > tt.maxExpect {
				t.Errorf("calculateTitleConfidence(%q, %q) = %f, want between %f and %f",
					tt.title, tt.original, result, tt.minExpect, tt.maxExpect)
			}
			if result < 0.0 || result > 1.0 {
				t.Errorf("calculateTitleConfidence(%q, %q) = %f, must be between 0.0 and 1.0",
					tt.title, tt.original, result)
			}
		})
	}
}

func TestResolveTVShowTitle_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		expectAmb   bool
		expectTitle string
	}{
		{
			name:        "Very short folder name vs longer filename",
			filePath:    "/storage/X (2022)/Season 01/X Files S01E01.mkv",
			expectAmb:   true,
			expectTitle: "X Files",
		},
		{
			name:        "Abbreviation in folder vs full in filename",
			filePath:    "/storage/SHIELD (2013)/Season 01/Agents of SHIELD S01E01.mkv",
			expectAmb:   true,
			expectTitle: "Agents Of SHIELD",
		},
		{
			name:        "Release group markers in filename",
			filePath:    "/storage/Show (2020)/Season 01/Show.S01E01.1080p.WEB-DL.x264-GROUP.mkv",
			expectAmb:   false,
			expectTitle: "Show",
		},
		{
			name:        "Episode title with underscore separator",
			filePath:    "/storage/Show/Season 01/Show_S01E01_The Beginning.mkv",
			expectAmb:   false,
			expectTitle: "Show",
		},
		{
			name:        "Show with subtitle in folder",
			filePath:    "/storage/Star Trek - The Next Generation (1987)/Season 01/Star Trek TNG S01E01.mkv",
			expectAmb:   true,
			expectTitle: "Star Trek - The Next Generation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution := ResolveTVShowTitle(tt.filePath, "/storage")
			if resolution == nil {
				t.Fatal("ResolveTVShowTitle returned nil")
			}
			if resolution.IsAmbiguous != tt.expectAmb {
				t.Errorf("Expected IsAmbiguous=%v, got %v (reason: %s)",
					tt.expectAmb, resolution.IsAmbiguous, resolution.Reason)
			}
			if resolution.ResolvedTitle != tt.expectTitle {
				t.Errorf("Expected resolved title %q, got %q",
					tt.expectTitle, resolution.ResolvedTitle)
			}
		})
	}
}

func TestGetAmbiguousTVShows(t *testing.T) {
	resolutions := []*TVTitleResolution{
		{
			ResolvedTitle: "Clear Show",
			IsAmbiguous:   false,
			APIVerified:   false,
			Confidence:    1.0,
		},
		{
			ResolvedTitle: "Ambiguous Show 1",
			IsAmbiguous:   true,
			APIVerified:   false,
			Confidence:    0.5,
			Reason:        "Conflicting titles",
		},
		{
			ResolvedTitle: "Verified Show",
			IsAmbiguous:   true,
			APIVerified:   true,
			Confidence:    0.95,
			Reason:        "TVDB verified",
		},
		{
			ResolvedTitle: "Ambiguous Show 2",
			IsAmbiguous:   true,
			APIVerified:   false,
			Confidence:    0.6,
			Reason:        "Filename longer",
		},
	}

	ambiguous := GetAmbiguousTVShows(resolutions)

	if len(ambiguous) != 2 {
		t.Errorf("Expected 2 ambiguous shows (excluding API verified), got %d", len(ambiguous))
	}

	for _, res := range ambiguous {
		if !res.IsAmbiguous {
			t.Error("Non-ambiguous show in ambiguous list")
		}
		if res.APIVerified {
			t.Error("API-verified show should not be in ambiguous list")
		}
	}
}

func TestResolveTVShowTitle_YearExtraction(t *testing.T) {
	tests := []struct {
		name           string
		filePath       string
		expectFolderYr string
		expectFileYr   string
	}{
		{
			name:           "Year in folder only",
			filePath:       "/storage/Breaking Bad (2008)/Season 01/Breaking.Bad.S01E01.mkv",
			expectFolderYr: "2008",
			expectFileYr:   "",
		},
		{
			name:           "Year in filename only",
			filePath:       "/storage/Breaking Bad/Season 01/Breaking.Bad.(2008).S01E01.mkv",
			expectFolderYr: "",
			expectFileYr:   "2008",
		},
		{
			name:           "No year anywhere",
			filePath:       "/storage/The Wire/Season 01/The.Wire.S01E01.mkv",
			expectFolderYr: "",
			expectFileYr:   "",
		},
		{
			name:           "Year in both (folder wins)",
			filePath:       "/storage/Show (2020)/Season 01/Show.(2020).S01E01.mkv",
			expectFolderYr: "2020",
			expectFileYr:   "2020",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution := ResolveTVShowTitle(tt.filePath, "/storage")
			if resolution.FolderMatch.Year != tt.expectFolderYr {
				t.Errorf("Expected folder year %q, got %q",
					tt.expectFolderYr, resolution.FolderMatch.Year)
			}
			if resolution.FilenameMatch.Year != tt.expectFileYr {
				t.Errorf("Expected filename year %q, got %q",
					tt.expectFileYr, resolution.FilenameMatch.Year)
			}
		})
	}
}
