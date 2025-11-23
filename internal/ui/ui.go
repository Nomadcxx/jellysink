package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// Custom messages for progress updates
type progressMsg scanner.ScanProgress
type scanCompleteMsg reporter.Report
type scanErrorMsg error

// ViewMode represents the current TUI view
type ViewMode int

const (
	ViewSummary ViewMode = iota
	ViewDuplicates
	ViewCompliance
	ViewManualIntervention
	ViewScanning
)

// Model represents the TUI state
type Model struct {
	report                 reporter.Report
	mode                   ViewMode
	viewport               viewport.Model
	ready                  bool
	width                  int
	height                 int
	shouldClean            bool
	selectedAmbiguousIndex int
	editingTitle           bool
	titleInput             textinput.Model
	editedTitles           map[int]string

	// Scanning state
	scanning        bool
	scanLogs        []LogLine
	currentProgress string
	progressPercent float64
	cancelled       bool
}

// NewModel creates a new TUI model with a scan report
func NewModel(report reporter.Report) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter correct title..."
	ti.CharLimit = 200
	ti.Width = 60

	return Model{
		report:       report,
		mode:         ViewSummary,
		titleInput:   ti,
		editedTitles: make(map[int]string),
	}
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		// Update scanning progress
		m.currentProgress = msg.Message
		m.progressPercent = msg.Percentage

		// Add to log buffer (keep last 100 lines)
		logEntry := LogLine{
			Timestamp: fmt.Sprintf("%02d:%02d", msg.ElapsedSeconds/60, msg.ElapsedSeconds%60),
			Operation: msg.Operation,
			Message:   msg.Message,
			Severity:  msg.Severity,
		}
		m.scanLogs = append(m.scanLogs, logEntry)
		if len(m.scanLogs) > 1000 {
			m.scanLogs = m.scanLogs[len(m.scanLogs)-1000:]
		}

		// Update viewport content
		if m.mode == ViewScanning {
			m.viewport.SetContent(m.renderScanning())
			m.viewport.GotoBottom()
		}
		return m, nil

	case scanCompleteMsg:
		// Scan finished - switch to summary
		m.scanning = false
		m.report = reporter.Report(msg)
		m.mode = ViewSummary
		m.viewport.SetContent(m.renderSummary())
		return m, nil

	case scanErrorMsg:
		// Scan error - show error and exit
		m.scanning = false
		m.scanLogs = append(m.scanLogs, LogLine{Timestamp: fmt.Sprintf("%02d:%02d", 0, 0), Operation: "scan", Message: fmt.Sprintf("ERROR: %v", msg), Severity: "error"})
		m.viewport.SetContent(m.renderScanning())
		return m, nil

	case tea.KeyMsg:
		if m.editingTitle {
			switch msg.String() {
			case "esc":
				m.editingTitle = false
				m.titleInput.Blur()
				m.viewport.SetContent(m.renderManualIntervention())
				return m, nil

			case "enter":
				value := strings.TrimSpace(m.titleInput.Value())
				if value != "" {
					m.editedTitles[m.selectedAmbiguousIndex] = value
					m.editingTitle = false
					m.titleInput.Blur()
					m.titleInput.SetValue("")
					m.viewport.SetContent(m.renderManualIntervention())
				}
				return m, nil

			default:
				var cmd tea.Cmd
				m.titleInput, cmd = m.titleInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.mode == ViewScanning {
				m.cancelled = true
				m.scanLogs = append(m.scanLogs, LogLine{Timestamp: "", Operation: "scan", Message: "Cancelling scan...", Severity: "warn"})
			}
			return m, tea.Quit

		case "esc":
			if m.mode != ViewSummary {
				m.mode = ViewSummary
				m.viewport.SetContent(m.renderSummary())
				return m, nil
			}
			return m, tea.Quit

		case "f1":
			m.mode = ViewDuplicates
			m.viewport.SetContent(m.renderDuplicates())
			m.viewport.GotoTop()
			return m, nil

		case "f2":
			m.mode = ViewCompliance
			m.viewport.SetContent(m.renderCompliance())
			m.viewport.GotoTop()
			return m, nil

		case "f3":
			if len(m.report.AmbiguousTVShows) > 0 {
				m.mode = ViewManualIntervention
				m.viewport.SetContent(m.renderManualIntervention())
				m.viewport.GotoTop()
			}
			return m, nil

		case "up", "k":
			if m.mode == ViewManualIntervention && !m.editingTitle {
				if m.selectedAmbiguousIndex > 0 {
					m.selectedAmbiguousIndex--
					m.viewport.SetContent(m.renderManualIntervention())
				}
			}
			return m, nil

		case "down", "j":
			if m.mode == ViewManualIntervention && !m.editingTitle {
				if m.selectedAmbiguousIndex < len(m.report.AmbiguousTVShows)-1 {
					m.selectedAmbiguousIndex++
					m.viewport.SetContent(m.renderManualIntervention())
				}
			}
			return m, nil

		case "e":
			if m.mode == ViewManualIntervention && !m.editingTitle {
				m.editingTitle = true
				resolution := m.report.AmbiguousTVShows[m.selectedAmbiguousIndex]
				if editedTitle, exists := m.editedTitles[m.selectedAmbiguousIndex]; exists {
					m.titleInput.SetValue(editedTitle)
				} else if resolution.ResolvedTitle != "" {
					m.titleInput.SetValue(resolution.ResolvedTitle)
				} else if resolution.FolderMatch != nil {
					m.titleInput.SetValue(resolution.FolderMatch.Title)
				} else if resolution.FilenameMatch != nil {
					m.titleInput.SetValue(resolution.FilenameMatch.Title)
				}
				m.titleInput.Focus()
				m.viewport.SetContent(m.renderManualIntervention())
				return m, textinput.Blink
			}
			return m, nil

		case "enter":
			if m.mode == ViewManualIntervention && !m.editingTitle {
				if len(m.editedTitles) > 0 {
					m.shouldClean = true
					return m, tea.Quit
				}
				return m, nil
			}
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
		if len(m.report.AmbiguousTVShows) > 0 {
			footer = FormatFooter(
				FormatKeybinding("F1", "Duplicates"),
				FormatKeybinding("F2", "Compliance"),
				FormatKeybinding("F3", "Manual Fixes"),
				FormatKeybinding("Esc", "Exit"),
			)
		} else {
			footer = FormatFooter(
				FormatKeybinding("F1", "Duplicates"),
				FormatKeybinding("F2", "Compliance"),
				FormatKeybinding("Enter", "Clean"),
				FormatKeybinding("Esc", "Exit"),
			)
		}

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

	case ViewManualIntervention:
		header = FormatHeader("MANUAL INTERVENTION REQUIRED")
		footer = FormatFooter(
			FormatKeybinding("↑↓", "Navigate"),
			FormatKeybinding("E", "Edit Title"),
			FormatKeybinding("Enter", "Apply Renames"),
			FormatKeybinding("Esc", "Back"),
		)

	case ViewScanning:
		header = FormatHeader("SCANNING IN PROGRESS")
		footer = FormatFooter(
			FormatKeybinding("Ctrl+C", "Cancel Scan"),
			MutedStyle.Render("Please wait..."),
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

	// ASCII header
	sb.WriteString(FormatASCIIHeader() + "\n\n")

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

	// Manual Intervention section (if ambiguous TV shows exist)
	if len(m.report.AmbiguousTVShows) > 0 {
		sb.WriteString(TitleStyle.Render("⚠ MANUAL INTERVENTION REQUIRED") + "\n")
		sb.WriteString(ErrorStyle.Render(fmt.Sprintf("TV shows needing review: %d", len(m.report.AmbiguousTVShows))) + "\n")
		sb.WriteString(WarningStyle.Render("These shows have conflicting titles that could not be auto-resolved.") + "\n")
		sb.WriteString(InfoStyle.Render("Press F3 to review and fix these issues.") + "\n\n")
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

// renderManualIntervention renders the manual intervention view for ambiguous TV show titles
func (m Model) renderManualIntervention() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("TV SHOWS REQUIRING MANUAL REVIEW") + "\n\n")

	if len(m.report.AmbiguousTVShows) == 0 {
		sb.WriteString(SuccessStyle.Render("✓ No ambiguous TV show titles found") + "\n")
		return sb.String()
	}

	sb.WriteString(InfoStyle.Render(fmt.Sprintf("Found %d TV show(s) with conflicting titles that need your review:", len(m.report.AmbiguousTVShows))) + "\n\n")

	sb.WriteString(WarningStyle.Render("⚠ These shows have different titles in folder vs filename, and the API could not resolve them.") + "\n")
	sb.WriteString(MutedStyle.Render("   Please review each one and choose the correct title manually.") + "\n\n")

	for i, resolution := range m.report.AmbiguousTVShows {
		isSelected := i == m.selectedAmbiguousIndex
		prefix := "   "
		if isSelected {
			prefix = " → "
		}

		if isSelected {
			sb.WriteString(HighlightStyle.Render(fmt.Sprintf("%s%d. CONFLICT DETECTED", prefix, i+1)) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("%s%d. CONFLICT DETECTED\n", prefix, i+1))
		}

		if resolution.FolderMatch != nil {
			sb.WriteString(fmt.Sprintf("%s   %s %s",
				prefix,
				InfoStyle.Render("Folder title:  "),
				ContentStyle.Render(resolution.FolderMatch.Title)))
			if resolution.FolderMatch.Year != "" {
				sb.WriteString(MutedStyle.Render(fmt.Sprintf(" (%s)", resolution.FolderMatch.Year)))
			}
			sb.WriteString(fmt.Sprintf(" [confidence: %s]\n", StatStyle.Render(fmt.Sprintf("%.0f%%", resolution.FolderMatch.Confidence*100))))
		}

		if resolution.FilenameMatch != nil {
			sb.WriteString(fmt.Sprintf("%s   %s %s",
				prefix,
				InfoStyle.Render("Filename title:"),
				ContentStyle.Render(resolution.FilenameMatch.Title)))
			if resolution.FilenameMatch.Year != "" {
				sb.WriteString(MutedStyle.Render(fmt.Sprintf(" (%s)", resolution.FilenameMatch.Year)))
			}
			sb.WriteString(fmt.Sprintf(" [confidence: %s]\n", StatStyle.Render(fmt.Sprintf("%.0f%%", resolution.FilenameMatch.Confidence*100))))
		}

		sb.WriteString(fmt.Sprintf("%s   %s %s\n",
			prefix,
			MutedStyle.Render("Reason:       "),
			ErrorStyle.Render(resolution.Reason)))

		if resolution.APIVerified {
			sb.WriteString(fmt.Sprintf("%s   %s %s\n",
				prefix,
				MutedStyle.Render("API says:     "),
				WarningStyle.Render("API returned conflicting results")))
		} else {
			sb.WriteString(fmt.Sprintf("%s   %s %s\n",
				prefix,
				MutedStyle.Render("API says:     "),
				MutedStyle.Render("Could not verify (API key not configured or failed)")))
		}

		if isSelected && m.editingTitle {
			sb.WriteString(fmt.Sprintf("%s   %s %s\n",
				prefix,
				SuccessStyle.Render("Edit title:   "),
				m.titleInput.View()))
			sb.WriteString(fmt.Sprintf("%s   %s\n",
				prefix,
				MutedStyle.Render("              Press Enter to save, Esc to cancel")))
		} else {
			editedTitle, hasEdit := m.editedTitles[i]
			if hasEdit {
				sb.WriteString(fmt.Sprintf("%s   %s %s %s\n",
					prefix,
					SuccessStyle.Render("Edited to:    "),
					HighlightStyle.Render(editedTitle),
					SuccessStyle.Render("✓")))
			} else if resolution.ResolvedTitle != "" {
				sb.WriteString(fmt.Sprintf("%s   %s %s\n",
					prefix,
					InfoStyle.Render("Current:      "),
					ContentStyle.Render(resolution.ResolvedTitle)))
			} else {
				sb.WriteString(fmt.Sprintf("%s   %s %s\n",
					prefix,
					ErrorStyle.Render("Action needed:"),
					WarningStyle.Render("Press E to edit")))
			}
		}

		sb.WriteString("\n")
	}

	sb.WriteString(MutedStyle.Render("───────────────────────────────────────────────────────────────────────────────") + "\n\n")
	sb.WriteString(InfoStyle.Render("What to do:") + "\n")
	sb.WriteString("  1. Use ↑↓ to navigate between conflicts\n")
	sb.WriteString("  2. Press 'E' to edit the selected title\n")
	sb.WriteString("  3. Press 'Enter' to apply all renames\n")
	sb.WriteString("  4. Press 'Esc' to go back without changes\n\n")

	if len(m.editedTitles) > 0 {
		sb.WriteString(SuccessStyle.Render(fmt.Sprintf("✓ %d title(s) edited and ready to apply", len(m.editedTitles))) + "\n")
	}

	sb.WriteString(WarningStyle.Render("Note: Renames will be applied to both folders and filenames for consistency.") + "\n")

	return sb.String()
}

// renderScanning renders the scanning progress view
func (m Model) renderScanning() string {
	var sb strings.Builder

	// ASCII header
	sb.WriteString(FormatASCIIHeader() + "\n\n")

	// Progress bar
	progressBar := renderProgressBar(m.progressPercent, 50)
	sb.WriteString(progressBar + "\n")
	sb.WriteString(fmt.Sprintf("  %s %.1f%%\n\n", m.currentProgress, m.progressPercent))

	// Log viewport (last 20 lines)
	sb.WriteString(TitleStyle.Render("SCAN LOG") + "\n")
	sb.WriteString(strings.Repeat("─", 80) + "\n")

	startIdx := 0
	if len(m.scanLogs) > 20 {
		startIdx = len(m.scanLogs) - 20
	}

	for i := startIdx; i < len(m.scanLogs); i++ {
		entry := m.scanLogs[i]
		var lineStyle = MutedStyle
		if entry.Severity == "error" {
			lineStyle = ErrorStyle
		} else if entry.Severity == "warn" {
			lineStyle = WarningStyle
		}
		sb.WriteString(lineStyle.Render(fmt.Sprintf("%s %s [%s] %s", entry.Timestamp, entry.Operation, strings.ToUpper(entry.Severity), entry.Message)) + "\n")
	}

	if m.cancelled {
		sb.WriteString("\n" + ErrorStyle.Render("Scan cancelled by user") + "\n")
	}

	return sb.String()
}

// renderProgressBar creates a text-based progress bar
func renderProgressBar(percent float64, width int) string {
	filled := int((percent / 100.0) * float64(width))
	if filled > width {
		filled = width
	}

	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += " "
		}
	}
	bar += "]"

	return SuccessStyle.Render(bar)
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

// GetEditedTitles returns the map of edited titles by ambiguous show index
func (m Model) GetEditedTitles() map[int]string {
	return m.editedTitles
}
