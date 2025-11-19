package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/jellysink/internal/reporter"
)

// ViewMode represents the current TUI view
type ViewMode int

const (
	ViewSummary ViewMode = iota
	ViewDuplicates
	ViewCompliance
)

// Model represents the TUI state
type Model struct {
	report      reporter.Report
	mode        ViewMode
	viewport    viewport.Model
	ready       bool
	width       int
	height      int
	shouldClean bool // Set to true when user presses Enter to clean
}

// NewModel creates a new TUI model with a scan report
func NewModel(report reporter.Report) Model {
	return Model{
		report: report,
		mode:   ViewSummary,
	}
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "esc":
			// Return to summary from detail views
			if m.mode != ViewSummary {
				m.mode = ViewSummary
				m.viewport.SetContent(m.renderSummary())
				return m, nil
			}
			// Exit from summary
			return m, tea.Quit

		case "f1":
			// Switch to duplicates view
			m.mode = ViewDuplicates
			m.viewport.SetContent(m.renderDuplicates())
			m.viewport.GotoTop()
			return m, nil

		case "f2":
			// Switch to compliance view
			m.mode = ViewCompliance
			m.viewport.SetContent(m.renderCompliance())
			m.viewport.GotoTop()
			return m, nil

		case "enter":
			// Mark that user wants to clean and exit
			m.shouldClean = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Initialize viewport
			m.viewport = viewport.New(msg.Width, msg.Height-4) // Leave room for header/footer
			m.viewport.SetContent(m.renderSummary())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 4
		}

		return m, nil
	}

	// Handle viewport updates (scrolling)
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the TUI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var header string
	var footer string

	switch m.mode {
	case ViewSummary:
		header = FormatHeader("JELLYSINK SCAN SUMMARY")
		footer = FormatFooter(
			FormatKeybinding("F1", "Duplicates"),
			FormatKeybinding("F2", "Compliance"),
			FormatKeybinding("Enter", "Clean"),
			FormatKeybinding("Esc", "Exit"),
		)

	case ViewDuplicates:
		header = FormatHeader("DUPLICATE REPORT (DETAILED)")
		scrollInfo := fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))
		footer = FormatFooter(
			FormatKeybinding("↑↓", "Scroll"),
			FormatKeybinding("PgUp/PgDn", "Page"),
			FormatKeybinding("Esc", "Back"),
			MutedStyle.Render(scrollInfo),
		)

	case ViewCompliance:
		header = FormatHeader("COMPLIANCE REPORT (DETAILED)")
		scrollInfo := fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))
		footer = FormatFooter(
			FormatKeybinding("↑↓", "Scroll"),
			FormatKeybinding("PgUp/PgDn", "Page"),
			FormatKeybinding("Esc", "Back"),
			MutedStyle.Render(scrollInfo),
		)
	}

	// Build full view
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.viewport.View(),
		footer,
	)
}

// renderSummary renders the summary view
func (m Model) renderSummary() string {
	var sb strings.Builder

	// Timestamp and library info
	sb.WriteString(InfoStyle.Render("Generated: ") + ContentStyle.Render(m.report.Timestamp.Format("2006-01-02 15:04:05")) + "\n")
	sb.WriteString(InfoStyle.Render("Library: ") + ContentStyle.Render(m.report.LibraryType) + "\n")
	sb.WriteString(InfoStyle.Render("Paths: ") + ContentStyle.Render(strings.Join(m.report.LibraryPaths, ", ")) + "\n\n")

	// Duplicates section
	sb.WriteString(TitleStyle.Render("DUPLICATES") + "\n")
	sb.WriteString(InfoStyle.Render("Groups found: ") + StatStyle.Render(fmt.Sprintf("%d", m.report.TotalDuplicates)) + "\n")
	sb.WriteString(InfoStyle.Render("Files to delete: ") + StatStyle.Render(fmt.Sprintf("%d", m.report.TotalFilesToDelete)) + "\n")
	sb.WriteString(InfoStyle.Render("Space to free: ") + StatStyle.Render(formatBytes(m.report.SpaceToFree)) + "\n\n")

	if m.report.TotalDuplicates > 0 {
		sb.WriteString(MutedStyle.Render("Top 5 examples:") + "\n")
		// Get top offenders
		offenders := getTopOffenders(m.report)
		limit := 5
		if len(offenders) < limit {
			limit = len(offenders)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("  %s %s - %s versions, %s\n",
				WarningStyle.Render(fmt.Sprintf("%d.", i+1)),
				ContentStyle.Render(offenders[i].Name),
				StatStyle.Render(fmt.Sprintf("%d", offenders[i].Count)),
				StatStyle.Render(formatBytes(offenders[i].SpaceToFree))))
		}
		sb.WriteString("\n")
	}

	// Compliance section
	sb.WriteString(TitleStyle.Render("COMPLIANCE ISSUES") + "\n")
	sb.WriteString(InfoStyle.Render("Files to rename: ") + StatStyle.Render(fmt.Sprintf("%d", len(m.report.ComplianceIssues))) + "\n\n")

	if len(m.report.ComplianceIssues) > 0 {
		sb.WriteString(MutedStyle.Render("First 5 examples:") + "\n")
		limit := 5
		if len(m.report.ComplianceIssues) < limit {
			limit = len(m.report.ComplianceIssues)
		}
		for i := 0; i < limit; i++ {
			issue := m.report.ComplianceIssues[i]
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				WarningStyle.Render(fmt.Sprintf("%d.", i+1)),
				ContentStyle.Render(issue.Path)))
			sb.WriteString(fmt.Sprintf("     %s %s\n",
				MutedStyle.Render("Problem:"),
				ContentStyle.Render(issue.Problem)))
			sb.WriteString(fmt.Sprintf("     %s %s\n",
				MutedStyle.Render("Action:"),
				InfoStyle.Render(issue.SuggestedAction)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderDuplicates renders the duplicates detail view
func (m Model) renderDuplicates() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("MOVIE DUPLICATES") + "\n\n")

	if len(m.report.MovieDuplicates) == 0 && len(m.report.TVDuplicates) == 0 {
		sb.WriteString(MutedStyle.Render("No duplicates found.") + "\n")
		return sb.String()
	}

	// Render movie duplicates
	for _, dup := range m.report.MovieDuplicates {
		title := dup.NormalizedName
		if dup.Year != "" {
			title = title + " (" + dup.Year + ")"
		}
		sb.WriteString(HighlightStyle.Render(fmt.Sprintf("%s (%d versions)", title, len(dup.Files))) + "\n")

		for i, file := range dup.Files {
			if i == 0 {
				sb.WriteString(fmt.Sprintf("  %s [%s] [%s] %s\n",
					SuccessStyle.Render("KEEP:  "),
					StatStyle.Render(formatBytes(file.Size)),
					InfoStyle.Render(file.Resolution),
					ContentStyle.Render(file.Path)))
			} else {
				sb.WriteString(fmt.Sprintf("  %s [%s] [%s] %s\n",
					ErrorStyle.Render("DELETE:"),
					StatStyle.Render(formatBytes(file.Size)),
					InfoStyle.Render(file.Resolution),
					MutedStyle.Render(file.Path)))
			}
		}
		sb.WriteString("\n")
	}

	// Render TV duplicates
	if len(m.report.TVDuplicates) > 0 {
		sb.WriteString(TitleStyle.Render("TV EPISODE DUPLICATES") + "\n\n")

		for _, dup := range m.report.TVDuplicates {
			title := fmt.Sprintf("%s S%02dE%02d", dup.ShowName, dup.Season, dup.Episode)
			sb.WriteString(HighlightStyle.Render(fmt.Sprintf("%s (%d versions)", title, len(dup.Files))) + "\n")

			for i, file := range dup.Files {
				if i == 0 {
					sb.WriteString(fmt.Sprintf("  %s [%s] [%s] [%s] %s\n",
						SuccessStyle.Render("KEEP:  "),
						StatStyle.Render(formatBytes(file.Size)),
						InfoStyle.Render(file.Resolution),
						InfoStyle.Render(file.Source),
						ContentStyle.Render(file.Path)))
				} else {
					sb.WriteString(fmt.Sprintf("  %s [%s] [%s] [%s] %s\n",
						ErrorStyle.Render("DELETE:"),
						StatStyle.Render(formatBytes(file.Size)),
						InfoStyle.Render(file.Resolution),
						InfoStyle.Render(file.Source),
						MutedStyle.Render(file.Path)))
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderCompliance renders the compliance detail view
func (m Model) renderCompliance() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("NON-COMPLIANT FILES AND FOLDERS") + "\n\n")

	if len(m.report.ComplianceIssues) == 0 {
		sb.WriteString(SuccessStyle.Render("✓ All files follow Jellyfin naming conventions") + "\n")
		return sb.String()
	}

	sb.WriteString(InfoStyle.Render(fmt.Sprintf("Total issues: %d", len(m.report.ComplianceIssues))) + "\n\n")

	for i, issue := range m.report.ComplianceIssues {
		sb.WriteString(fmt.Sprintf("%s %s %s\n",
			WarningStyle.Render(fmt.Sprintf("%d.", i+1)),
			MutedStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(issue.Type))),
			ContentStyle.Render(issue.Problem)))

		sb.WriteString(fmt.Sprintf("   %s %s\n",
			MutedStyle.Render("Current: "),
			ErrorStyle.Render(issue.Path)))

		sb.WriteString(fmt.Sprintf("   %s %s\n",
			MutedStyle.Render("Fixed:   "),
			SuccessStyle.Render(issue.SuggestedPath)))

		sb.WriteString(fmt.Sprintf("   %s %s\n\n",
			MutedStyle.Render("Action:  "),
			InfoStyle.Render(issue.SuggestedAction)))
	}

	return sb.String()
}

// Helper functions

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func getTopOffenders(report reporter.Report) []reporter.Offender {
	return reporter.GetTopOffenders(report)
}

// ShouldClean returns whether the user requested a clean operation
func (m Model) ShouldClean() bool {
	return m.shouldClean
}

