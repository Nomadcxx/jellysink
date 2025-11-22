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
			expected: "Thelma O Unicornio (2024)",
		},
		// Edge cases reported by user
		{
			name:     "8MM",
			input:    "8MM.1999.720p.WEB-DL.DD5.1.H264-FGT-Obfuscated.cp(tt0134273).mkv",
			expected: "8MM (1999)",
		},
		{
			name:     "U.S. Marshals",
			input:    "U.S.Marshals.1998.DVDRip.x264-DJ.mkv",
			expected: "U.S. Marshals (1998)",
		},
		{
			name:     "D.E.B.S.",
			input:    "D.E.B.S..2004.1080p.x264.DTS-Relevant.mkv",
			expected: "D.E.B.S. (2004)",
		},
		{
			name:     "R.I.P.D. 2",
			input:    "R.I.P.D.2.Rise.of.the.Damned.2022.BluRay.720p.DTS.x264-MTeam.mkv",
			expected: "R.I.P.D. 2 Rise Of The Damned (2022)",
		},
		{
			name:     "Le Comte de Monte-Cristo",
			input:    "Le.Comte.de.Monte-Cristo.2024.1080p.WEB.H264-GP-M-NLsubs.mkv",
			expected: "Le Comte De Monte-Cristo (2024)",
		},
		// Edge cases from user compliance report
		{
			name:     "Trolls with SPARKS release group",
			input:    "Trolls.2016.720p.BluRay.x264-SPARKS.mkv",
			expected: "Trolls (2016)",
		},
		{
			name:     "The Invitation with Unrated edition marker",
			input:    "The Invitation-Unrated (2022)",
			expected: "The Invitation (2022)",
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

	// Additional compliance tests for parent folder precedence
	// Thelma - parent folder is 'Thelma the Unicorn (2024)', the filename is Spanish; expect compliance to prefer parent folder
	{
		name := "Thelma parent precedence"
		lib := "/tmp/test_jellysink_lib/MOVIES"
		filePath := lib + "/Thelma the Unicorn (2024)/Thelma.O.Unicornio.2024.1080p.NF.WEB-DL.DDP5.1.Atmos.x264.DUAL-CBR.mkv"
		issue := checkMovieCompliance(filePath, lib)
		if issue == nil {
			t.Fatalf("%s: expected compliance issue for %s, got nil", name, filePath)
		}
		expected := lib + "/Thelma The Unicorn (2024)/Thelma The Unicorn (2024).mkv"
		if issue.SuggestedPath != expected {
			t.Fatalf("%s: expected suggested path %s, got %s", name, expected, issue.SuggestedPath)
		}
	}
}
