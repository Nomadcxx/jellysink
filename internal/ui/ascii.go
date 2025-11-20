package ui

import "github.com/charmbracelet/lipgloss"

// ASCII art for jellysink header as single string to preserve exact formatting
const jellysinkASCII = `  ████              ████   ████                            ████                ████
                    ████   ████                                                ████
  ████  ██████████  ████   ████  ████  ████  ██████████  ██████    ██████████  ████  ████
  ████  ████  ████  ████   ████  ████  ████  ██████      ██████    ██████████  ██████████
  ████  ██████████  ████   ████  ████  ████  ██████████    ████    ████  ████  ████████
  ████  ████        ████   ████  ██████████      ██████  ████████  ████  ████  ██████████
  ████  ██████████  ████   ████  ██████████  ██████████  ████████  ████  ████  ████  ████
██████                                 ████
██████                           ██████████                                              `

// FormatASCIIHeader renders the jellysink ASCII header with RAMA theme
// Render as single block to preserve spacing and structure
func FormatASCIIHeader() string {
	headerStyle := lipgloss.NewStyle().
		Foreground(RAMARed).
		Bold(true)

	return headerStyle.Render(jellysinkASCII)
}

// FormatASCIIHeaderCentered renders header centered
func FormatASCIIHeaderCentered(width int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(RAMARed).
		Bold(true).
		Align(lipgloss.Center).
		Width(width)

	return headerStyle.Render(jellysinkASCII)
}

// FormatASCIIHeaderWithSubtext renders header with subtitle
func FormatASCIIHeaderWithSubtext(subtext string) string {
	header := FormatASCIIHeader()

	subtitle := lipgloss.NewStyle().
		Foreground(RAMAMuted).
		Render(subtext)

	return header + "\n\n" + subtitle
}
