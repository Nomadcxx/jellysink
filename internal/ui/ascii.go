package ui

import "github.com/charmbracelet/lipgloss"

// ASCII art for jellysink header
const jellysinkASCII = `  ████              ████   ████                            ████                ████
                    ████   ████                                                ████
  ████  ██████████  ████   ████  ████  ████  ██████████  ██████    ██████████  ████  ████
  ████  ████  ████  ████   ████  ████  ████  ██████      ██████    ██████████  ██████████
  ████  ██████████  ████   ████  ████  ████  ██████████    ████    ████  ████  ████████
  ████  ████        ████   ████  ██████████      ██████  ████████  ████  ████  ██████████
  ████  ██████████  ████   ████  ██████████  ██████████  ████████  ████  ████  ████  ████
██████                                 ████
██████                           ██████████`

// FormatASCIIHeader renders the jellysink ASCII header with RAMA theme
func FormatASCIIHeader() string {
	// Apply RAMA red color to ASCII art
	style := lipgloss.NewStyle().
		Foreground(RAMARed).
		Bold(true)

	return style.Render(jellysinkASCII)
}

// FormatASCIIHeaderWithSubtext renders header with subtitle
func FormatASCIIHeaderWithSubtext(subtext string) string {
	header := FormatASCIIHeader()

	// Subtitle in muted color
	subtitle := lipgloss.NewStyle().
		Foreground(RAMAMuted).
		Render(subtext)

	return header + "\n\n" + subtitle
}
