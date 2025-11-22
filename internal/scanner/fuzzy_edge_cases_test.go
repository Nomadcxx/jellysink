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
