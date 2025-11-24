package scanner

import "testing"

func TestIsGarbageTitle(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		expectGarbage bool
	}{
		// Release group combinations
		{
			name:          "Multiple release groups",
			title:         "Airtv Rome",
			expectGarbage: true,
		},
		{
			name:          "NTb PlayWeb combo",
			title:         "NTb PlayWeb",
			expectGarbage: true,
		},
		{
			name:          "Dimension NTb",
			title:         "Dimension NTb",
			expectGarbage: true,
		},

		// Single obvious release groups
		{
			name:          "Airtv alone",
			title:         "Airtv",
			expectGarbage: true,
		},
		{
			name:          "RARBG alone",
			title:         "RARBG",
			expectGarbage: true,
		},
		{
			name:          "x264 codec",
			title:         "x264",
			expectGarbage: true,
		},

		// Single words that could be legitimate titles
		{
			name:          "Rome HBO show",
			title:         "Rome",
			expectGarbage: false,
		},
		{
			name:          "Memento movie",
			title:         "Memento",
			expectGarbage: false,
		},
		{
			name:          "Lost show",
			title:         "Lost",
			expectGarbage: false,
		},

		// Clean TV show titles
		{
			name:          "Star Trek",
			title:         "Star Trek",
			expectGarbage: false,
		},
		{
			name:          "Last Week Tonight",
			title:         "Last Week Tonight",
			expectGarbage: false,
		},
		{
			name:          "Breaking Bad",
			title:         "Breaking Bad",
			expectGarbage: false,
		},
		{
			name:          "The Wire",
			title:         "The Wire",
			expectGarbage: false,
		},
		{
			name:          "IT Welcome To Derry",
			title:         "IT Welcome To Derry",
			expectGarbage: false,
		},

		// Codec/release group combinations
		{
			name:          "x264 RARBG",
			title:         "x264 RARBG",
			expectGarbage: true,
		},

		// Leetspeak
		{
			name:          "D3FiL3R leetspeak",
			title:         "D3FiL3R",
			expectGarbage: true,
		},
		{
			name:          "H264 in title",
			title:         "H264",
			expectGarbage: true,
		},

		// Edge cases
		{
			name:          "Empty string",
			title:         "",
			expectGarbage: true,
		},
		{
			name:          "Valid caps IT",
			title:         "IT",
			expectGarbage: false,
		},
		{
			name:          "Valid caps FBI",
			title:         "FBI",
			expectGarbage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGarbageTitle(tt.title)
			if result != tt.expectGarbage {
				t.Errorf("IsGarbageTitle(%q) = %v, want %v", tt.title, result, tt.expectGarbage)
			}
		})
	}
}
