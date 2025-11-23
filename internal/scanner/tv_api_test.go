package scanner

import (
	"strings"
	"testing"
)

func TestTVDBClient_Login(t *testing.T) {
	apiKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"

	client := NewTVDBClient(apiKey)
	err := client.Login()

	if err != nil {
		t.Fatalf("Login() failed: %v", err)
	}

	if client.Token == "" {
		t.Error("Login() succeeded but token is empty")
	}

	t.Logf("Successfully logged in, token: %s...", client.Token[:20])
}

func TestTVDBClient_LoginWithInvalidKey(t *testing.T) {
	client := NewTVDBClient("invalid-key-123")
	err := client.Login()

	if err == nil {
		t.Error("Expected login to fail with invalid API key, but it succeeded")
	}

	t.Logf("Expected error: %v", err)
}

func TestTVDBClient_SearchSeries(t *testing.T) {
	apiKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"

	tests := []struct {
		name          string
		searchQuery   string
		expectResults bool
		minResults    int
	}{
		{
			name:          "Breaking Bad exact match",
			searchQuery:   "Breaking Bad",
			expectResults: true,
			minResults:    1,
		},
		{
			name:          "Degrassi partial match",
			searchQuery:   "Degrassi",
			expectResults: true,
			minResults:    1,
		},
		{
			name:          "Non-existent show",
			searchQuery:   "ThisShowDoesNotExistXYZ123",
			expectResults: false,
			minResults:    0,
		},
		{
			name:          "Star Trek variations",
			searchQuery:   "Star Trek",
			expectResults: true,
			minResults:    3,
		},
	}

	client := NewTVDBClient(apiKey)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := client.SearchSeries(tt.searchQuery)

			if err != nil {
				t.Fatalf("SearchSeries(%q) failed: %v", tt.searchQuery, err)
			}

			if tt.expectResults && len(results) == 0 {
				t.Errorf("SearchSeries(%q) expected results, got none", tt.searchQuery)
			}

			if !tt.expectResults && len(results) > 0 {
				t.Errorf("SearchSeries(%q) expected no results, got %d", tt.searchQuery, len(results))
			}

			if len(results) < tt.minResults {
				t.Errorf("SearchSeries(%q) expected at least %d results, got %d",
					tt.searchQuery, tt.minResults, len(results))
			}

			if len(results) > 0 {
				t.Logf("SearchSeries(%q) returned %d results:", tt.searchQuery, len(results))
				for i, result := range results {
					if i >= 3 {
						t.Logf("  ... and %d more", len(results)-3)
						break
					}
					t.Logf("  - %s (%s) [ID: %s]", result.Name, result.Year, result.ID)
				}
			}
		})
	}
}

func TestTVDBClient_SearchSeries_Fuzzy(t *testing.T) {
	apiKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"
	client := NewTVDBClient(apiKey)

	tests := []struct {
		folderTitle   string
		filenameTitle string
		expectSameID  bool
		description   string
	}{
		{
			folderTitle:   "Degrassi",
			filenameTitle: "Degrassi The Next Generation",
			expectSameID:  true,
			description:   "TVDB treats 'Degrassi The Next Generation' as alias for 'Degrassi (2001)'",
		},
		{
			folderTitle:   "Breaking Bad",
			filenameTitle: "Breaking Bad",
			expectSameID:  true,
			description:   "Exact match should return same series",
		},
		{
			folderTitle:   "Marvel Agents of Shield",
			filenameTitle: "Marvels Agents of SHIELD",
			expectSameID:  true,
			description:   "Minor spelling variations should match",
		},
		{
			folderTitle:   "Star Trek",
			filenameTitle: "Star Trek The Next Generation",
			expectSameID:  false,
			description:   "Different series within same franchise",
		},
	}

	for _, tt := range tests {
		t.Run(tt.folderTitle+" vs "+tt.filenameTitle, func(t *testing.T) {
			t.Logf("Test case: %s", tt.description)

			folderResults, err := client.SearchSeries(tt.folderTitle)
			if err != nil {
				t.Fatalf("SearchSeries(%q) failed: %v", tt.folderTitle, err)
			}

			filenameResults, err := client.SearchSeries(tt.filenameTitle)
			if err != nil {
				t.Fatalf("SearchSeries(%q) failed: %v", tt.filenameTitle, err)
			}

			if len(folderResults) == 0 {
				t.Logf("No results for folder title %q", tt.folderTitle)
				return
			}

			if len(filenameResults) == 0 {
				t.Logf("No results for filename title %q", tt.filenameTitle)
				return
			}

			sameID := folderResults[0].ID == filenameResults[0].ID

			t.Logf("Folder result: %s (%s) [ID: %s]",
				folderResults[0].Name, folderResults[0].Year, folderResults[0].ID)
			t.Logf("Filename result: %s (%s) [ID: %s]",
				filenameResults[0].Name, filenameResults[0].Year, filenameResults[0].ID)

			if sameID != tt.expectSameID {
				t.Errorf("Expected same ID = %v, got %v", tt.expectSameID, sameID)
			}
		})
	}
}

func TestVerifyTVShowTitle_FolderMatchOnly(t *testing.T) {
	apiKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"

	resolution := &TVTitleResolution{
		FolderMatch: &TVTitleMatch{
			Title:      "Breaking Bad",
			Source:     "folder",
			Confidence: 0.9,
			Year:       "2008",
		},
		FilenameMatch: &TVTitleMatch{
			Title:      "NonExistentShowXYZ",
			Source:     "filename",
			Confidence: 0.5,
			Year:       "",
		},
		IsAmbiguous: true,
		Confidence:  0.5,
	}

	err := VerifyTVShowTitle(resolution, apiKey, "")
	if err != nil {
		t.Fatalf("VerifyTVShowTitle() failed: %v", err)
	}

	if !resolution.APIVerified {
		t.Error("Expected APIVerified to be true")
	}

	if resolution.IsAmbiguous {
		t.Error("Expected IsAmbiguous to be false after API verification")
	}

	if resolution.ResolvedTitle == "" {
		t.Error("Expected ResolvedTitle to be set")
	}

	t.Logf("Resolved title: %q (confidence: %.2f, reason: %s)",
		resolution.ResolvedTitle, resolution.Confidence, resolution.Reason)
}

func TestVerifyTVShowTitle_BothMatch(t *testing.T) {
	apiKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"

	resolution := &TVTitleResolution{
		FolderMatch: &TVTitleMatch{
			Title:      "Breaking Bad",
			Source:     "folder",
			Confidence: 0.9,
			Year:       "2008",
		},
		FilenameMatch: &TVTitleMatch{
			Title:      "Breaking Bad",
			Source:     "filename",
			Confidence: 0.9,
			Year:       "",
		},
		IsAmbiguous: false,
		Confidence:  0.9,
	}

	err := VerifyTVShowTitle(resolution, apiKey, "")
	if err != nil {
		t.Fatalf("VerifyTVShowTitle() failed: %v", err)
	}

	if !resolution.APIVerified {
		t.Error("Expected APIVerified to be true")
	}

	if resolution.IsAmbiguous {
		t.Error("Expected IsAmbiguous to remain false after API verification")
	}

	if resolution.Confidence != 1.0 {
		t.Errorf("Expected confidence 1.0 for matching series, got %.2f", resolution.Confidence)
	}

	t.Logf("Resolved title: %q (confidence: %.2f, reason: %s)",
		resolution.ResolvedTitle, resolution.Confidence, resolution.Reason)
}

func TestVerifyTVShowTitle_DegrassiConflict(t *testing.T) {
	apiKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"

	resolution := &TVTitleResolution{
		FolderMatch: &TVTitleMatch{
			Title:      "Degrassi",
			Source:     "folder",
			Confidence: 0.8,
			Year:       "2001",
		},
		FilenameMatch: &TVTitleMatch{
			Title:      "Degrassi The Next Generation",
			Source:     "filename",
			Confidence: 1.0,
			Year:       "",
		},
		IsAmbiguous: true,
		Confidence:  0.6,
	}

	err := VerifyTVShowTitle(resolution, apiKey, "")
	if err != nil {
		t.Fatalf("VerifyTVShowTitle() failed: %v", err)
	}

	if !resolution.APIVerified {
		t.Error("Expected APIVerified to be true")
	}

	if resolution.IsAmbiguous {
		t.Error("Expected IsAmbiguous to be false after TVDB confirms both match same series")
	}

	if resolution.Confidence != 1.0 {
		t.Errorf("Expected confidence 1.0 when TVDB confirms both match, got %.2f", resolution.Confidence)
	}

	t.Logf("Resolved title: %q", resolution.ResolvedTitle)
	t.Logf("Confidence: %.2f", resolution.Confidence)
	t.Logf("Reason: %s", resolution.Reason)
	t.Logf("Note: TVDB treats 'Degrassi The Next Generation' as an alias for 'Degrassi (2001)'")
}

func TestOMDBClient_SearchSeries(t *testing.T) {
	omdbKey := "484cc32"
	client := NewOMDBClient(omdbKey)

	result, err := client.SearchSeries("Breaking Bad")
	if err != nil {
		t.Fatalf("SearchSeries() failed: %v", err)
	}

	if result.Title == "" {
		t.Error("Expected non-empty title")
	}

	if result.ImdbID == "" {
		t.Error("Expected non-empty IMDB ID")
	}

	if result.Type != "series" {
		t.Errorf("Expected type 'series', got %q", result.Type)
	}

	t.Logf("Found series: %s (%s) - IMDB: %s", result.Title, result.Year, result.ImdbID)
}

func TestOMDBClient_SearchSeries_NotFound(t *testing.T) {
	omdbKey := "484cc32"
	client := NewOMDBClient(omdbKey)

	_, err := client.SearchSeries("ThisShowDoesNotExist12345XYZ")
	if err == nil {
		t.Error("Expected error for non-existent show")
	}

	t.Logf("Expected error: %v", err)
}

func TestVerifyTVShowTitle_WithOMDBFallback(t *testing.T) {
	tvdbKey := ""
	omdbKey := "484cc32"

	resolution := &TVTitleResolution{
		FolderMatch: &TVTitleMatch{
			Title:      "Breaking Bad",
			Source:     "folder",
			Confidence: 0.9,
			Year:       "2008",
		},
		FilenameMatch: &TVTitleMatch{
			Title:      "Breaking Bad",
			Source:     "filename",
			Confidence: 0.9,
			Year:       "",
		},
		IsAmbiguous: false,
		Confidence:  0.9,
	}

	err := VerifyTVShowTitle(resolution, tvdbKey, omdbKey)
	if err != nil {
		t.Fatalf("VerifyTVShowTitle() failed: %v", err)
	}

	if !resolution.APIVerified {
		t.Error("Expected APIVerified to be true")
	}

	t.Logf("OMDB fallback worked: %q (confidence: %.2f, reason: %s)",
		resolution.ResolvedTitle, resolution.Confidence, resolution.Reason)
}

func TestVerifyTVShowTitle_TVDBWithOMDBFallback(t *testing.T) {
	tvdbKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"
	omdbKey := "484cc32"

	resolution := &TVTitleResolution{
		FolderMatch: &TVTitleMatch{
			Title:      "Breaking Bad",
			Source:     "folder",
			Confidence: 0.9,
			Year:       "2008",
		},
		FilenameMatch: &TVTitleMatch{
			Title:      "Breaking Bad",
			Source:     "filename",
			Confidence: 0.9,
			Year:       "",
		},
		IsAmbiguous: false,
		Confidence:  0.9,
	}

	err := VerifyTVShowTitle(resolution, tvdbKey, omdbKey)
	if err != nil {
		t.Fatalf("VerifyTVShowTitle() failed: %v", err)
	}

	if !resolution.APIVerified {
		t.Error("Expected APIVerified to be true")
	}

	if !strings.Contains(resolution.Reason, "TVDB") {
		t.Errorf("Expected TVDB to be used (not OMDB fallback), reason: %s", resolution.Reason)
	}

	t.Logf("TVDB used (not fallback): %q (confidence: %.2f, reason: %s)",
		resolution.ResolvedTitle, resolution.Confidence, resolution.Reason)
}
