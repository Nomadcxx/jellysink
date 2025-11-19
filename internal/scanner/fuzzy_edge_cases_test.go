package scanner

import (
	"testing"
)

// TestStripReleaseGroup_EdgeCases tests comprehensive scene release naming conventions
func TestStripReleaseGroup_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// REMUX releases
		{"REMUX basic", "Movie.Name.2024.2160p.BluRay.REMUX.HDR.DTS-HD.MA.7.1.x265-GROUP", "Movie Name 2024"},
		{"REMUX short", "Movie.2024.1080p.REMUX.x264-GROUP", "Movie 2024"},

		// HDR formats
		{"HDR10", "Movie.2024.2160p.WEB-DL.HDR10.DDP5.1.H.265-GROUP", "Movie 2024"},
		{"HDR10+", "Movie.2024.2160p.WEB.HDR10Plus.DDP5.1.H.265-GROUP", "Movie 2024"},
		{"Dolby Vision", "Movie.2024.2160p.WEB-DL.DV.HDR.DDP5.1.H.265-GROUP", "Movie 2024"},
		{"DoVi", "Movie.2024.2160p.BluRay.DoVi.HDR.TrueHD.Atmos.7.1-GROUP", "Movie 2024"},
		{"HLG", "Movie.2024.1080p.HDTV.HLG.AAC.2.0.H.264-GROUP", "Movie 2024"},

		// Advanced audio formats
		{"DTS-HD MA", "Movie.2024.1080p.BluRay.DTS-HD.MA.5.1.x264-GROUP", "Movie 2024"},
		{"DTS-X", "Movie.2024.2160p.BluRay.DTS-X.7.1.x265-GROUP", "Movie 2024"},
		{"FLAC", "Movie.2024.1080p.BluRay.FLAC.2.0.x264-GROUP", "Movie 2024"},
		{"PCM", "Movie.2024.1080p.BluRay.PCM.Stereo.x264-GROUP", "Movie 2024"},
		{"DD+ (Dolby Digital Plus)", "Movie.2024.1080p.WEB-DL.DD+5.1.H.264-GROUP", "Movie 2024"},
		{"DDP (DD Plus shorthand)", "Movie.2024.1080p.WEB-DL.DDP.5.1.H.264-GROUP", "Movie 2024"},

		// Audio channels
		{"2.0 audio", "Movie.2024.1080p.BluRay.AAC.2.0.x264-GROUP", "Movie 2024"},
		{"5.1 audio", "Movie.2024.1080p.BluRay.AC3.5.1.x264-GROUP", "Movie 2024"},
		{"7.1 audio", "Movie.2024.1080p.BluRay.DTS.7.1.x264-GROUP", "Movie 2024"},

		// Special sources
		{"BDRip", "Movie.2024.720p.BDRip.x264-GROUP", "Movie 2024"},
		{"CAM", "Movie.2024.CAM.XviD-GROUP", "Movie 2024"},
		{"TS (Telesync)", "Movie.2024.TS.XviD-GROUP", "Movie 2024"},
		{"HDTS", "Movie.2024.HDTS.x264-GROUP", "Movie 2024"},
		{"SCR (Screener)", "Movie.2024.DVDSCR.XviD-GROUP", "Movie 2024"},

		// Platform tags
		{"Disney+", "Movie.2024.1080p.DSNP.WEB-DL.DDP5.1.H.264-GROUP", "Movie 2024"},
		{"HBO Max", "Movie.2024.1080p.HMAX.WEB-DL.DD5.1.H.264-GROUP", "Movie 2024"},
		{"Hulu", "Movie.2024.1080p.HULU.WEB-DL.AAC2.0.H.264-GROUP", "Movie 2024"},
		{"Apple TV+", "Movie.2024.1080p.ATVP.WEB-DL.DDP5.1.H.264-GROUP", "Movie 2024"},
		{"Paramount+", "Movie.2024.1080p.PMTP.WEB-DL.DDP5.1.H.264-GROUP", "Movie 2024"},

		// Special editions
		{"IMAX", "Movie.2024.1080p.BluRay.IMAX.DTS.5.1.x264-GROUP", "Movie 2024"},
		{"IMAX Enhanced", "Movie.2024.2160p.WEB-DL.IMAX.Enhanced.DDP.5.1.H.265-GROUP", "Movie 2024"},
		{"Remastered", "Movie.2024.1080p.BluRay.REMASTERED.DTS.5.1.x264-GROUP", "Movie 2024"},
		{"Director's Cut", "Movie.2024.1080p.BluRay.DC.DTS.5.1.x264-GROUP", "Movie 2024"},
		{"Theatrical", "Movie.2024.1080p.BluRay.Theatrical.DTS.5.1.x264-GROUP", "Movie 2024"},
		{"UNCUT", "Movie.2024.1080p.BluRay.UNCUT.DTS.5.1.x264-GROUP", "Movie 2024"},

		// Multi-language
		{"MULTI audio", "Movie.2024.1080p.BluRay.MULTI.DTS.5.1.x264-GROUP", "Movie 2024"},
		{"Dual Language", "Movie.2024.1080p.BluRay.DL.AC3.5.1.x264-GROUP", "Movie 2024"},
		{"DUBBED", "Movie.2024.1080p.BluRay.DUBBED.AAC.2.0.x264-GROUP", "Movie 2024"},

		// Old/Legacy codecs
		{"XviD", "Movie.2024.DVDRip.XviD-GROUP", "Movie 2024"},
		{"DivX", "Movie.2024.DVDRip.DivX-GROUP", "Movie 2024"},
		{"MPEG2", "Movie.2024.DVD.MPEG2-GROUP", "Movie 2024"},

		// Version tags
		{"v2 repack", "Movie.2024.1080p.BluRay.x264.v2-GROUP", "Movie 2024"},
		{"v3", "Movie.2024.1080p.WEB-DL.x264.v3-GROUP", "Movie 2024"},

		// Complex real-world examples
		{"Complex REMUX", "The.Movie.Name.2024.2160p.UHD.BluRay.REMUX.HDR10Plus.DV.TrueHD.Atmos.7.1.x265-FraMeSToR", "The Movie Name 2024"},
		{"Complex WEB-DL", "Movie.Name.2024.1080p.AMZN.WEB-DL.DDP5.1.H.264.HDR.DoVi-GROUP", "Movie Name 2024"},
		{"Complex BDRip", "Movie.Name.2024.720p.BDRip.x264.AAC-YTS.MX", "Movie Name 2024"},
		{"IMAX REMUX combo", "Movie.2024.2160p.BluRay.REMUX.IMAX.Enhanced.DV.HDR10.DTS-X.7.1-GROUP", "Movie 2024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripReleaseGroup(tt.input)
			if result != tt.expected {
				t.Errorf("StripReleaseGroup(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractResolution_EdgeCases tests resolution detection edge cases
func TestExtractResolution_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Movie.2024.2160p.UHD.BluRay", "2160p"},
		{"Movie.2024.4K.WEB-DL", "2160p"},
		{"Movie.2024.UHD.BluRay", "2160p"},
		{"Movie.2024.576p.DVDRip", "unknown"}, // SD resolution not in our list
		{"Movie.2024.480i.HDTV", "unknown"},   // Interlaced format not tracked
	}

	for _, tt := range tests {
		result := ExtractResolution(tt.input)
		if result != tt.expected {
			t.Errorf("ExtractResolution(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
