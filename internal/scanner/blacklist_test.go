package scanner

import "testing"

func TestIsKnownReleaseGroup(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		{"Public torrent group", "rarbg", true},
		{"TV scene group", "airtv", true},
		{"Movie scene group", "veto", true},
		{"HD remux group", "framestor", true},
		{"Codec marker", "x264", true},
		{"Quality marker", "1080p", true},
		{"Regional group", "viethd", true},
		{"Anime group", "horriblesubs", true},
		{"Valid title", "rome", false},
		{"Valid title uppercase", "lost", false},
		{"Non-existent group", "notareleasegroup", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsKnownReleaseGroup(tt.word)
			if result != tt.expected {
				t.Errorf("IsKnownReleaseGroup(%q) = %v, want %v", tt.word, result, tt.expected)
			}
		})
	}
}

func TestIsPreservedAcronym(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		{"Star Trek TNG", "tng", true},
		{"NCIS", "ncis", true},
		{"FBI", "fbi", true},
		{"Game of Thrones", "got", true},
		{"Not an acronym", "airtv", false},
		{"Random word", "example", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPreservedAcronym(tt.word)
			if result != tt.expected {
				t.Errorf("IsPreservedAcronym(%q) = %v, want %v", tt.word, result, tt.expected)
			}
		})
	}
}

func TestIsCodecMarker(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		{"x264 codec", "x264", true},
		{"x265 codec", "x265", true},
		{"HEVC codec", "hevc", true},
		{"AAC audio", "aac", true},
		{"1080p resolution", "1080p", true},
		{"HDR marker", "hdr", true},
		{"Valid title", "movie", false},
		{"Valid title uppercase", "action", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCodecMarker(tt.word)
			if result != tt.expected {
				t.Errorf("IsCodecMarker(%q) = %v, want %v", tt.word, result, tt.expected)
			}
		})
	}
}

func TestIsAllCapsLegitTitle(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		{"IT movie", "it", true},
		{"FBI show", "fbi", true},
		{"NCIS show", "ncis", true},
		{"Rome show", "rome", true},
		{"Lost show", "lost", true},
		{"Not a title", "xyz", false},
		{"Release group", "airtv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAllCapsLegitTitle(tt.word)
			if result != tt.expected {
				t.Errorf("IsAllCapsLegitTitle(%q) = %v, want %v", tt.word, result, tt.expected)
			}
		})
	}
}

func TestBlacklistCoverage(t *testing.T) {
	if len(KnownReleaseGroups) < 100 {
		t.Errorf("Expected at least 100 release groups in blacklist, got %d", len(KnownReleaseGroups))
	}

	if len(CodecMarkers) < 20 {
		t.Errorf("Expected at least 20 codec markers, got %d", len(CodecMarkers))
	}

	if len(PreservedAcronyms) < 10 {
		t.Errorf("Expected at least 10 preserved acronyms, got %d", len(PreservedAcronyms))
	}

	if len(AllCapsLegitTitles) < 10 {
		t.Errorf("Expected at least 10 all-caps legit titles, got %d", len(AllCapsLegitTitles))
	}
}

func TestBlacklistNoConflicts(t *testing.T) {
	for acronym := range PreservedAcronyms {
		if IsCodecMarker(acronym) {
			t.Errorf("Preserved acronym %q conflicts with codec marker", acronym)
		}
	}

	for title := range AllCapsLegitTitles {
		if IsCodecMarker(title) {
			t.Errorf("All-caps legit title %q conflicts with codec marker", title)
		}
	}
}
