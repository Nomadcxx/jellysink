package scanner

import (
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Movie (2024)", "movie"},
		{"Movie Name (2024)", "movie name"},
		{"Lost In Translation (2003)", "lost in translation"},
		{"Lost in Translation (2003)", "lost in translation"},
		{"The Nun II (2023)", "the nun 2"},
		{"The Nun 2 (2023)", "the nun 2"},
		{"Movie.Name.2024.1080p", "movie name"},
		{"Movie & The Other (2024)", "movie the other"},
		{"Movie and The Other (2024)", "movie the other"},
	}

	for _, tt := range tests {
		result := NormalizeName(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Movie (2024)", "2024"},
		{"Movie [2024]", "2024"},
		{"Movie.2024.1080p", "2024"},
		{"Movie 2024", "2024"},
		{"Movie without year", ""},
	}

	for _, tt := range tests {
		result := ExtractYear(tt.input)
		if result != tt.expected {
			t.Errorf("ExtractYear(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractResolution(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Movie.2024.1080p.BluRay", "1080p"},
		{"Movie.2024.720p.WEB-DL", "720p"},
		{"Movie.2024.2160p.UHD", "2160p"},
		{"Movie.2024.4K.BluRay", "2160p"},
		{"Movie.2024.BluRay", "unknown"},
	}

	for _, tt := range tests {
		result := ExtractResolution(tt.input)
		if result != tt.expected {
			t.Errorf("ExtractResolution(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestStripReleaseGroup(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Movie.Name.2024.1080p.BluRay.x264-GROUP", "Movie Name 2024"},
		{"Movie.Name.2024.WEB-DL.AAC", "Movie Name 2024"},
		{"Movie.Name.2024", "Movie Name 2024"},
		{"Movie_Name_2024", "Movie Name 2024"},
	}

	for _, tt := range tests {
		result := StripReleaseGroup(tt.input)
		if result != tt.expected {
			t.Errorf("StripReleaseGroup(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCleanMovieName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Movie.Name.2024.1080p.BluRay.x264-GROUP", "Movie Name (2024)"},
		{"Movie.Name.(2024)", "Movie Name (2024)"},
		{"Movie_Name_(2024)", "Movie Name (2024)"},
		{"25th.Hour.2002.1080p.BluRay.H264.AC3.DD5.1.mp4", "25th Hour (2002)"},
		{"Shrek 2 (2004) 1080p 10bit Bluray x265 HEVC [Org BD 2.0 Hindi + DD 5.1 English] MSubs ~ TombDoc.mkv", "Shrek 2 (2004)"},
		{"The Matrix 1999 2160p UHD BluRay x265 HDR10", "The Matrix (1999)"},
		{"Inception.2010.1080p.BluRay.x264.DTS-HD.MA.5.1", "Inception (2010)"},
		{"21st Century", "21st Century"},
		{"The Man Who Fell to Earth 1976 HEVC D3FiL3R (iso)", "The Man Who Fell To Earth (1976)"},
		{"Blade.Runner.2049.2017.2160p.BluRay.REMUX.HEVC.DTS-HD.MA.TrueHD.7.1.Atmos-FGT", "Blade Runner 2049 (2017)"},
		{"The.Matrix.1999.REMASTERED.1080p.BluRay.x265.10bit.HDR.DTS-X.7.1-YTS", "The Matrix (1999)"},
	}

	for _, tt := range tests {
		result := CleanMovieName(tt.input)
		if result != tt.expected {
			t.Errorf("CleanMovieName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSimilarityRatio(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		minRatio float64
	}{
		{"lost in translation", "lost in translation", 1.0},
		{"lost in translation", "Lost In Translation", 0.80}, // Case difference (lower threshold)
		{"the nun ii", "the nun 2", 0.75},                    // Roman numeral (lower threshold)
		{"movie name", "completely different", 0.0},          // Very different
	}

	for _, tt := range tests {
		result := SimilarityRatio(tt.s1, tt.s2)
		if result < tt.minRatio {
			t.Errorf("SimilarityRatio(%q, %q) = %.2f, want >= %.2f", tt.s1, tt.s2, result, tt.minRatio)
		}
	}
}

func TestExtractEpisodeInfo(t *testing.T) {
	tests := []struct {
		input         string
		expectSeason  int
		expectEpisode int
		expectFound   bool
	}{
		{"Show.S01E01.1080p.mkv", 1, 1, true},
		{"Show.S02E15.720p.mkv", 2, 15, true},
		{"Show.1x05.mkv", 1, 5, true},
		{"Show.2x10.mkv", 2, 10, true},
		{"Show.s03e08.mkv", 3, 8, true},
		{"Show.Without.Episode.mkv", 0, 0, false},
	}

	for _, tt := range tests {
		season, episode, found := ExtractEpisodeInfo(tt.input)
		if found != tt.expectFound {
			t.Errorf("ExtractEpisodeInfo(%q) found = %v, want %v", tt.input, found, tt.expectFound)
		}
		if found && (season != tt.expectSeason || episode != tt.expectEpisode) {
			t.Errorf("ExtractEpisodeInfo(%q) = S%02dE%02d, want S%02dE%02d",
				tt.input, season, episode, tt.expectSeason, tt.expectEpisode)
		}
	}
}

func TestPinocchioWill1869(t *testing.T) {
	input := "Pinocchio.1940.1080p.BluRay.H264.AC3.DD5.1.Will1869"
	expected := "Pinocchio (1940)"
	result := CleanMovieName(input)

	if result != expected {
		t.Errorf("CleanMovieName(%q) = %q, want %q", input, result, expected)
	}
}

func TestCleanMovieNameSNAKE(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"The Pentagon Wars 1998 1080p WEBRip X264 Ac3 SNAKE", "The Pentagon Wars (1998)"},
		{"The Pentagon Wars 1998 1080p WEBRip X264 Ac3 SNAKE.mkv", "The Pentagon Wars (1998)"},
		{"The.Pentagon.Wars.1998.1080p.WEBRip.X264.Ac3-SNAKE", "The Pentagon Wars (1998)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CleanMovieName(tt.input)
			if got != tt.expected {
				t.Errorf("CleanMovieName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
