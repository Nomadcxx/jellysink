package scanner

import (
	"testing"
)

func TestReleaseGroupRemoval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Pentagon Wars with SNAKE",
			input:    "The Pentagon Wars 1998 1080p WEBRip X264 Ac3 SNAKE",
			expected: "The Pentagon Wars (1998)",
		},
		{
			name:     "Princess with 3Audio MA5 1",
			input:    "The Princess And The Frog 2009 1080p BluRay x264 3Audio DTS-HD MA5 1",
			expected: "The Princess And The Frog (2009)",
		},
		{
			name:     "Moon with RightSiZE",
			input:    "Moon 2009 1080p BluRay x264 RightSiZE",
			expected: "Moon (2009)",
		},
		{
			name:     "Multiple orphaned markers",
			input:    "Movie Name 2024 1080p 5Audio DTS MA7 1 RARBG",
			expected: "Movie Name (2024)",
		},
		// Invasion of the Body Snatchers - commentary tag should be removed
		{
			name:     "Invasion Plus Commentary",
			input:    "Invasion.of.the.Body.Snatchers.1956.DVDRip.Plus.Commentary.x264-MaG-Chamele0n.mkv",
			expected: "Invasion Of The Body Snatchers (1956)",
		},
		// Men at Work - hyphenated group and multi-hyphen group
		{
			name:     "Men At Work psychd-ml",
			input:    "men.at.work.1990.720p.bluray.x264-psychd-ml.mkv",
			expected: "Men At Work (1990)",
		},
		// Idea of You - NORDiC should be stripped
		{
			name:     "Idea of You Nordic",
			input:    "The.Idea.of.You.2024.NORDiC.1080p.WEB-DL.H.265.DDP5.1-CiNEMiX.mkv",
			expected: "The Idea Of You (2024)",
		},
		// Vite Vendute - foreign filename, prefer parent
		{
			name:     "Vite Vendute foreign",
			input:    "Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-.MIRCrew.mkv",
			expected: "Vite Vendute (2024)",
		},
		// Thelma O Unicornio - translate then keep parent
		{
			name:     "Thelma O Unicornio",
			input:    "Thelma.O.Unicornio.2024.1080p.NF.WEB-DL.DDP5.1.Atmos.x264.DUAL-CBR.mkv",
			expected: "Thelma The Unicorn (2024)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanMovieName(tt.input)
			if result != tt.expected {
				t.Errorf("\nInput:    %s\nExpected: %s\nGot:      %s", tt.input, tt.expected, result)
			}
		})
	}
}
