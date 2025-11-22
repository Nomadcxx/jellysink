package ui

import "github.com/charmbracelet/lipgloss"

// RAMA theme colors (from sysc family)
var (
	// Primary colors
	RAMARed        = lipgloss.Color("#ef233c") // Pantone red
	RAMAFireRed    = lipgloss.Color("#d90429") // Fire engine red
	RAMABackground = lipgloss.Color("#2b2d42") // Space cadet
	RAMAForeground = lipgloss.Color("#edf2f4") // Anti-flash white
	RAMAMuted      = lipgloss.Color("#8d99ae") // Cool gray

	// Semantic colors
	ColorSuccess = lipgloss.Color("#2ecc71")
	ColorWarning = lipgloss.Color("#f39c12")
	ColorError   = RAMARed
	ColorInfo    = lipgloss.Color("#3498db")
)

// Styles for TUI components
var (
	// Border styles
	BorderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(RAMARed).
			Padding(1, 2)

	// Header style
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(RAMAForeground).
			Background(RAMARed).
			Padding(0, 1).
			Width(80)

	// Footer style (keybindings)
	FooterStyle = lipgloss.NewStyle().
			Foreground(RAMAMuted).
			Background(RAMABackground).
			Padding(0, 1).
			Width(80)

	// Title style (for sections)
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(RAMARed).
			MarginTop(1).
			MarginBottom(1)

	// Content style
	ContentStyle = lipgloss.NewStyle().
			Foreground(RAMAForeground)

	// Muted text style
	MutedStyle = lipgloss.NewStyle().
			Foreground(RAMAMuted)

	// Highlight style (for selections)
	HighlightStyle = lipgloss.NewStyle().
			Foreground(RAMABackground).
			Background(RAMARed).
			Bold(true)

	// Success style (KEEP markers)
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	// Error style (DELETE markers)
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	// Warning style (compliance issues)
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// Info style
	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// Stat style (for numbers)
	StatStyle = lipgloss.NewStyle().
			Foreground(RAMARed).
			Bold(true)
)

// FormatKeybinding formats a keybinding for display in footer
func FormatKeybinding(key, description string) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(RAMARed).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(RAMAMuted)

	return keyStyle.Render(key) + " " + descStyle.Render(description)
}

// FormatHeader formats a header with consistent styling
func FormatHeader(title string) string {
	return HeaderStyle.Render(title)
}

// FormatFooter formats footer with keybindings
func FormatFooter(keybindings ...string) string {
	footer := ""
	for i, kb := range keybindings {
		if i > 0 {
			footer += "  "
		}
		footer += kb
	}
	return FooterStyle.Render(footer)
}

// Status marker styles (moonbit-inspired)
var (
	OKMarker   = lipgloss.NewStyle().Foreground(ColorSuccess).SetString("[OK]")
	InfoMarker = lipgloss.NewStyle().Foreground(ColorInfo).SetString("[INFO]")
	WarnMarker = lipgloss.NewStyle().Foreground(ColorWarning).SetString("[WARN]")
	FailMarker = lipgloss.NewStyle().Foreground(ColorError).SetString("[FAIL]")
)

// FormatStatusOK returns an [OK] marker with message
func FormatStatusOK(message string) string {
	return OKMarker.String() + " " + message
}

// FormatStatusInfo returns an [INFO] marker with message
func FormatStatusInfo(message string) string {
	return InfoMarker.String() + " " + message
}

// FormatStatusWarn returns a [WARN] marker with message
func FormatStatusWarn(message string) string {
	return WarnMarker.String() + " " + message
}

// FormatStatusFail returns a [FAIL] marker with message
func FormatStatusFail(message string) string {
	return FailMarker.String() + " " + message
}
