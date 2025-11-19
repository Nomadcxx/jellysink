package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSource(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/Show.S01E01.1080p.BluRay.x264.mkv", "BLURAY"},
		{"/path/Show.S01E01.1080p.WEB-DL.x264.mkv", "WEB-DL"},
		{"/path/Show.S01E01.720p.HDTV.x264.mkv", "HDTV"},
		{"/path/Show.S01E01.REMUX.mkv", "REMUX"},
		{"/path/Show.S01E01.mkv", "unknown"},
	}

	for _, tt := range tests {
		result := extractSource(tt.input)
		if result != tt.expected {
			t.Errorf("extractSource(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestScoreTVFile(t *testing.T) {
	tests := []struct {
		name     string
		file     TVFile
		expected int
	}{
		{
			"Empty file",
			TVFile{Size: 0, Resolution: "1080p", Source: "WEB-DL", IsEmpty: true},
			-1000,
		},
		{
			"1080p REMUX",
			TVFile{Size: 5 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "REMUX", IsEmpty: false},
			355, // 300 (1080p) + 50 (REMUX) + 5 (5GB)
		},
		{
			"1080p BluRay",
			TVFile{Size: 3 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "BLURAY", IsEmpty: false},
			343, // 300 (1080p) + 40 (BluRay) + 3 (3GB)
		},
		{
			"1080p WEB-DL",
			TVFile{Size: 2 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "WEB-DL", IsEmpty: false},
			332, // 300 (1080p) + 30 (WEB-DL) + 2 (2GB)
		},
		{
			"720p HDTV",
			TVFile{Size: 1 * 1024 * 1024 * 1024, Resolution: "720p", Source: "HDTV", IsEmpty: false},
			221, // 200 (720p) + 20 (HDTV) + 1 (1GB)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scoreTVFile(tt.file)
			if result != tt.expected {
				t.Errorf("scoreTVFile() = %d, want %d", result, tt.expected)
			}
		})
	}

	// Test relative ordering
	empty := TVFile{Size: 0, IsEmpty: true}
	hdtv720p := TVFile{Size: 1 * 1024 * 1024 * 1024, Resolution: "720p", Source: "HDTV", IsEmpty: false}
	webdl1080p := TVFile{Size: 2 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "WEB-DL", IsEmpty: false}
	bluray1080p := TVFile{Size: 3 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "BLURAY", IsEmpty: false}

	if scoreTVFile(empty) >= scoreTVFile(hdtv720p) {
		t.Error("Empty file should score lower than any non-empty file")
	}

	if scoreTVFile(hdtv720p) >= scoreTVFile(webdl1080p) {
		t.Error("720p HDTV should score lower than 1080p WEB-DL")
	}

	if scoreTVFile(webdl1080p) >= scoreTVFile(bluray1080p) {
		t.Error("1080p WEB-DL should score lower than 1080p BluRay")
	}
}

func TestMarkKeepDeleteTV(t *testing.T) {
	duplicates := []TVDuplicate{
		{
			ShowName: "test show",
			Season:   1,
			Episode:  1,
			Files: []TVFile{
				{Path: "/path/show.s01e01.720p.hdtv.mkv", Size: 1 * 1024 * 1024 * 1024, Resolution: "720p", Source: "HDTV"},
				{Path: "/path/show.s01e01.1080p.web-dl.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "WEB-DL"},
				{Path: "/path/show.s01e01.empty.mkv", Size: 0, Resolution: "unknown", Source: "unknown", IsEmpty: true},
			},
		},
	}

	result := MarkKeepDeleteTV(duplicates)

	// After marking, the best file (1080p WEB-DL) should be first
	if result[0].Files[0].Resolution != "1080p" {
		t.Errorf("Expected 1080p file to be marked as keeper, got %s", result[0].Files[0].Resolution)
	}

	if result[0].Files[0].Source != "WEB-DL" {
		t.Errorf("Expected WEB-DL file to be marked as keeper, got %s", result[0].Files[0].Source)
	}

	// Empty file should not be first
	if result[0].Files[0].IsEmpty {
		t.Error("Empty file should not be marked as keeper")
	}
}

func TestGetTVDeleteList(t *testing.T) {
	duplicates := []TVDuplicate{
		{
			ShowName: "test show",
			Season:   1,
			Episode:  1,
			Files: []TVFile{
				{Path: "/keep/show.s01e01.1080p.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "WEB-DL"},
				{Path: "/delete/show.s01e01.720p.mkv", Size: 1 * 1024 * 1024 * 1024, Resolution: "720p", Source: "HDTV"},
				{Path: "/delete/show.s01e01.empty.mkv", Size: 0, Resolution: "unknown", Source: "unknown", IsEmpty: true},
			},
		},
	}

	deleteList := GetTVDeleteList(duplicates)

	if len(deleteList) != 2 {
		t.Errorf("Expected 2 files to delete, got %d", len(deleteList))
	}

	// Check that keeper is not in delete list
	for _, path := range deleteList {
		if path == "/keep/show.s01e01.1080p.mkv" {
			t.Error("Keeper file should not be in delete list")
		}
	}

	// Check that delete files are present
	found720p := false
	foundEmpty := false
	for _, path := range deleteList {
		if path == "/delete/show.s01e01.720p.mkv" {
			found720p = true
		}
		if path == "/delete/show.s01e01.empty.mkv" {
			foundEmpty = true
		}
	}

	if !found720p {
		t.Error("720p file should be in delete list")
	}
	if !foundEmpty {
		t.Error("Empty file should be in delete list")
	}
}

func TestGetTVSpaceToFree(t *testing.T) {
	duplicates := []TVDuplicate{
		{
			ShowName: "test show",
			Season:   1,
			Episode:  1,
			Files: []TVFile{
				{Path: "/keep/show.s01e01.1080p.mkv", Size: 2 * 1024 * 1024 * 1024, Resolution: "1080p", Source: "WEB-DL"},
				{Path: "/delete/show.s01e01.720p.mkv", Size: 1 * 1024 * 1024 * 1024, Resolution: "720p", Source: "HDTV"},
				{Path: "/delete/show.s01e01.480p.mkv", Size: 500 * 1024 * 1024, Resolution: "480p", Source: "HDTV"},
			},
		},
	}

	spaceToFree := GetTVSpaceToFree(duplicates)
	expected := int64(1*1024*1024*1024 + 500*1024*1024) // 1GB + 500MB

	if spaceToFree != expected {
		t.Errorf("GetTVSpaceToFree() = %d bytes, want %d bytes", spaceToFree, expected)
	}
}

func TestScanTVShows(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create test TV show structure
	showDir := filepath.Join(tmpDir, "Show Name (2024)")
	season01 := filepath.Join(showDir, "Season 01")
	os.MkdirAll(season01, 0755)

	// Create duplicate episodes
	ep1v1 := filepath.Join(season01, "Show.Name.S01E01.1080p.WEB-DL.mkv")
	os.WriteFile(ep1v1, []byte("fake video data"), 0644)

	ep1v2 := filepath.Join(season01, "Show.Name.S01E01.720p.HDTV.mkv")
	os.WriteFile(ep1v2, []byte("data"), 0644)

	// Create non-duplicate episode
	ep2 := filepath.Join(season01, "Show.Name.S01E02.1080p.WEB-DL.mkv")
	os.WriteFile(ep2, []byte("episode 2 data"), 0644)

	// Scan for duplicates
	duplicates, err := ScanTVShows([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanTVShows() error: %v", err)
	}

	// Should find exactly one duplicate group (S01E01)
	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
	}

	if len(duplicates) > 0 {
		// Check that the duplicate group is for S01E01
		if duplicates[0].Season != 1 || duplicates[0].Episode != 1 {
			t.Errorf("Expected duplicate for S01E01, got S%02dE%02d", duplicates[0].Season, duplicates[0].Episode)
		}

		// Check that it has 2 versions
		if len(duplicates[0].Files) != 2 {
			t.Errorf("Expected 2 versions of S01E01, got %d", len(duplicates[0].Files))
		}
	}
}

func TestScanTVShows_NoEpisodePattern(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create file without S##E## pattern
	showDir := filepath.Join(tmpDir, "Show Name")
	os.MkdirAll(showDir, 0755)

	noPattern := filepath.Join(showDir, "random.video.mkv")
	os.WriteFile(noPattern, []byte("not an episode"), 0644)

	// Scan for duplicates
	duplicates, err := ScanTVShows([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanTVShows() error: %v", err)
	}

	// Should find no duplicates (file doesn't match episode pattern)
	if len(duplicates) != 0 {
		t.Errorf("Expected 0 duplicate groups for non-episode file, got %d", len(duplicates))
	}
}
