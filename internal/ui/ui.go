package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/jellysink/internal/cleaner"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// Custom messages for progress updates
type progressMsg scanner.ScanProgress
type cleanProgressMsg scanner.ScanProgress
type renameProgressMsg scanner.ScanProgress
type scanCompleteMsg reporter.Report
type scanErrorMsg error
type renameCompleteMsg struct {
	result string
}

// ViewMode represents the current TUI view
type ViewMode int

const (
	ViewSummary ViewMode = iota
	ViewDuplicates
	ViewCompliance
	ViewManualIntervention
	ViewConflictReview
	ViewBatchSummary
	ViewBatchRenaming
	ViewScanning
	ViewCleanOptions
	ViewCleanConfirm
	ViewCleaning
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

	// New conflict resolution state
	currentConflictIndex int
	conflicts            []*scanner.TVTitleResolution
	batchReviewCursor    int

	// Scanning state
	scanning        bool
	scanLogs        []LogLine
	currentProgress string
	progressPercent float64
	cancelled       bool

	// Cleaning state
	cleaning          bool
	cleanProgressCh   chan scanner.ScanProgress
	cleanResult       string
	dryRun            bool
	cleanOptionCursor int // 0 = Dry Run, 1 = Full Clean

	// Batch rename state
	renaming         bool
	renameProgressCh chan scanner.ScanProgress
	renameResult     string
	renameErrors     []error
}

// NewModel creates a new TUI model with a scan report
func NewModel(report reporter.Report) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter correct title..."
	ti.CharLimit = 200
	ti.Width = 60

	conflicts := make([]*scanner.TVTitleResolution, len(report.AmbiguousTVShows))
	copy(conflicts, report.AmbiguousTVShows)

	return Model{
		report:       report,
		mode:         ViewSummary,
		titleInput:   ti,
		editedTitles: make(map[int]string),
		conflicts:    conflicts,
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

	case cleanProgressMsg:
		// Update cleaning progress (similar to scanning progress)
		m.currentProgress = msg.Message
		m.progressPercent = msg.Percentage

		// Add to log buffer
		logEntry := LogLine{
			Timestamp: fmt.Sprintf("%02d:%02d", msg.ElapsedSeconds/60, msg.ElapsedSeconds%60),
			Operation: msg.Operation,
			Message:   msg.Message,
			Severity:  msg.Severity,
		}
		m.scanLogs = append(m.scanLogs, logEntry)
		if len(m.scanLogs) > 100 {
			m.scanLogs = m.scanLogs[1:]
		}

		// Update viewport content
		m.viewport.SetContent(m.renderCleaning())

		// Continue listening for progress
		return m, waitForCleanProgress(m.cleanProgressCh)

	case cleanCompleteMsg:
		// Cleaning finished
		m.cleaning = false
		if msg.result != "" {
			m.cleanResult = msg.result
		}
		// If cleanResult is still empty, set a default message
		if m.cleanResult == "" {
			m.cleanResult = SuccessStyle.Render("✓ Cleanup completed")
		}
		m.viewport.SetContent(m.renderCleaning())
		return m, nil

	case renameProgressMsg:
		// Batch rename progress update
		progress := scanner.ScanProgress(msg)
		m.currentProgress = progress.Message
		m.progressPercent = progress.Percentage

		// Add to log buffer
		logEntry := LogLine{
			Timestamp: fmt.Sprintf("%02d:%02d", progress.ElapsedSeconds/60, progress.ElapsedSeconds%60),
			Operation: progress.Operation,
			Message:   progress.Message,
			Severity:  progress.Severity,
		}
		m.scanLogs = append(m.scanLogs, logEntry)
		if len(m.scanLogs) > 100 {
			m.scanLogs = m.scanLogs[1:]
		}

		// Update viewport content
		m.viewport.SetContent(m.renderBatchRenaming())

		// Continue listening for progress
		return m, waitForRenameProgress(m.renameProgressCh)

	case renameCompleteMsg:
		// Batch rename finished
		m.renaming = false
		if msg.result != "" {
			m.renameResult = msg.result
		}
		// If renameResult is still empty, set a default message
		if m.renameResult == "" {
			m.renameResult = SuccessStyle.Render("✓ Batch rename completed")
		}
		m.viewport.SetContent(m.renderBatchRenaming())
		return m, nil

	case tea.KeyMsg:
		if m.editingTitle {
			switch msg.String() {
			case "esc":
				m.editingTitle = false
				m.titleInput.Blur()
				if m.mode == ViewConflictReview {
					m.viewport.SetContent(m.renderConflictReview())
				} else {
					m.viewport.SetContent(m.renderManualIntervention())
				}
				return m, nil

			case "enter":
				value := strings.TrimSpace(m.titleInput.Value())
				if value != "" {
					if m.mode == ViewConflictReview {
						conflict := m.conflicts[m.currentConflictIndex]
						conflict.UserDecision = scanner.DecisionCustomTitle
						conflict.CustomTitle = value
						// Don't update ResolvedTitle - we need the original for rename
						m.viewport.SetContent(m.renderConflictReview())
					} else {
						m.editedTitles[m.selectedAmbiguousIndex] = value
						m.viewport.SetContent(m.renderManualIntervention())
					}
					m.editingTitle = false
					m.titleInput.Blur()
					m.titleInput.SetValue("")
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
			// Handle ESC in batch summary
			if m.mode == ViewBatchSummary {
				m.mode = ViewConflictReview
				m.viewport.SetContent(m.renderConflictReview())
				m.viewport.GotoTop()
				return m, nil
			}
			// Handle ESC in conflict review
			if m.mode == ViewConflictReview {
				m.mode = ViewSummary
				m.viewport.SetContent(m.renderSummary())
				return m, nil
			}
			// Handle ESC in cleaning options
			if m.mode == ViewCleanOptions {
				m.mode = ViewSummary
				m.viewport.SetContent(m.renderSummary())
				return m, nil
			}
			// Handle ESC in cleaning confirmation
			if m.mode == ViewCleanConfirm {
				m.mode = ViewCleanOptions
				m.viewport.SetContent(m.renderCleanOptions())
				m.viewport.GotoTop()
				return m, nil
			}
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
			if len(m.conflicts) > 0 {
				m.mode = ViewConflictReview
				m.currentConflictIndex = 0
				m.viewport.SetContent(m.renderConflictReview())
				m.viewport.GotoTop()
			}
			return m, nil

		case "up", "k":
			if m.mode == ViewManualIntervention && !m.editingTitle {
				if m.selectedAmbiguousIndex > 0 {
					m.selectedAmbiguousIndex--
					m.viewport.SetContent(m.renderManualIntervention())
				}
				return m, nil
			}
			if m.mode == ViewCleanOptions {
				if m.cleanOptionCursor > 0 {
					m.cleanOptionCursor--
					m.viewport.SetContent(m.renderCleanOptions())
				}
				return m, nil
			}
			if m.mode == ViewDuplicates || m.mode == ViewCompliance {
				m.viewport.LineUp(1)
				return m, nil
			}

		case "down", "j":
			if m.mode == ViewManualIntervention && !m.editingTitle {
				if m.selectedAmbiguousIndex < len(m.report.AmbiguousTVShows)-1 {
					m.selectedAmbiguousIndex++
					m.viewport.SetContent(m.renderManualIntervention())
				}
				return m, nil
			}
			if m.mode == ViewCleanOptions {
				if m.cleanOptionCursor < 1 {
					m.cleanOptionCursor++
					m.viewport.SetContent(m.renderCleanOptions())
				}
				return m, nil
			}
			if m.mode == ViewDuplicates || m.mode == ViewCompliance {
				m.viewport.LineDown(1)
				return m, nil
			}

		case "pgup":
			if m.mode == ViewDuplicates || m.mode == ViewCompliance || m.mode == ViewManualIntervention {
				m.viewport.ViewUp()
				return m, nil
			}

		case "pgdown":
			if m.mode == ViewDuplicates || m.mode == ViewCompliance || m.mode == ViewManualIntervention {
				m.viewport.ViewDown()
				return m, nil
			}

		case "e":
			if m.mode == ViewConflictReview && !m.editingTitle {
				m.editingTitle = true
				conflict := m.conflicts[m.currentConflictIndex]
				if conflict.CustomTitle != "" {
					m.titleInput.SetValue(conflict.CustomTitle)
				} else if conflict.FolderMatch != nil && conflict.FolderMatch.Confidence >= 0.5 {
					m.titleInput.SetValue(conflict.FolderMatch.Title)
				} else if conflict.FilenameMatch != nil {
					m.titleInput.SetValue(conflict.FilenameMatch.Title)
				}
				m.titleInput.Focus()
				m.viewport.SetContent(m.renderConflictReview())
				return m, textinput.Blink
			}
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
			if m.mode == ViewConflictReview && !m.editingTitle {
				allDecided := true
				for _, c := range m.conflicts {
					if c.UserDecision == scanner.DecisionNone {
						allDecided = false
						break
					}
				}
				if allDecided {
					m.mode = ViewBatchSummary
					m.batchReviewCursor = 0
					m.viewport.SetContent(m.renderBatchSummary())
					m.viewport.GotoTop()
				}
				return m, nil
			}
			if m.mode == ViewManualIntervention && !m.editingTitle {
				if len(m.editedTitles) > 0 {
					m.shouldClean = true
					return m, tea.Quit
				}
				return m, nil
			}
			// Enter in summary mode triggers clean options
			if m.mode == ViewSummary {
				m.mode = ViewCleanOptions
				m.cleanOptionCursor = 0
				m.viewport.SetContent(m.renderCleanOptions())
				m.viewport.GotoTop()
				return m, nil
			}
			// Enter in clean options mode selects the highlighted option
			if m.mode == ViewCleanOptions {
				if m.cleanOptionCursor == 0 {
					// Dry run selected
					m.dryRun = true
					m.mode = ViewCleaning
					m.cleaning = true
					m.scanLogs = []LogLine{}
					m.viewport.SetContent(m.renderCleaning())
					return m, m.runCleaning()
				} else {
					// Full clean selected - show confirmation
					m.dryRun = false
					m.mode = ViewCleanConfirm
					m.viewport.SetContent(m.renderCleanConfirm())
					m.viewport.GotoTop()
					return m, nil
				}
			}
			// Enter in clean confirm mode starts cleaning
			if m.mode == ViewCleanConfirm {
				m.mode = ViewCleaning
				m.cleaning = true
				m.scanLogs = []LogLine{} // Clear previous logs
				m.viewport.SetContent(m.renderCleaning())
				return m, m.runCleaning()
			}
			// Enter in batch summary applies renames
			if m.mode == ViewBatchSummary {
				m.mode = ViewBatchRenaming
				m.renaming = true
				m.scanLogs = []LogLine{}
				m.viewport.SetContent(m.renderBatchRenaming())
				return m, m.runBatchRename()
			}
			// Enter in batch renaming complete returns to summary
			if m.mode == ViewBatchRenaming && !m.renaming {
				m.mode = ViewSummary
				m.viewport.SetContent(m.renderSummary())
				return m, nil
			}
			return m, nil

		case "1":
			if m.mode == ViewConflictReview && !m.editingTitle {
				conflict := m.conflicts[m.currentConflictIndex]
				if conflict.FolderMatch != nil {
					conflict.UserDecision = scanner.DecisionFolderTitle
					// Don't update ResolvedTitle - we need the original for rename
					m.viewport.SetContent(m.renderConflictReview())
				}
				return m, nil
			}
			// Dry run selected from clean options
			if m.mode == ViewCleanOptions {
				m.dryRun = true
				m.mode = ViewCleaning
				m.cleaning = true
				m.scanLogs = []LogLine{}
				m.viewport.SetContent(m.renderCleaning())
				return m, m.runCleaning()
			}
			return m, nil

		case "2":
			if m.mode == ViewConflictReview && !m.editingTitle {
				conflict := m.conflicts[m.currentConflictIndex]
				if conflict.FilenameMatch != nil {
					conflict.UserDecision = scanner.DecisionFilenameTitle
					// Don't update ResolvedTitle - we need the original for rename
					m.viewport.SetContent(m.renderConflictReview())
				}
				return m, nil
			}
			// Full clean selected from clean options
			if m.mode == ViewCleanOptions {
				m.dryRun = false
				m.mode = ViewCleanConfirm
				m.viewport.SetContent(m.renderCleanConfirm())
				m.viewport.GotoTop()
				return m, nil
			}
			return m, nil

		case "right":
			if m.mode == ViewConflictReview && !m.editingTitle {
				if m.currentConflictIndex < len(m.conflicts)-1 {
					m.currentConflictIndex++
					m.viewport.SetContent(m.renderConflictReview())
					m.viewport.GotoTop()
				}
				return m, nil
			}
			return m, nil

		case "left":
			if m.mode == ViewConflictReview && !m.editingTitle {
				if m.currentConflictIndex > 0 {
					m.currentConflictIndex--
					m.viewport.SetContent(m.renderConflictReview())
					m.viewport.GotoTop()
				}
				return m, nil
			}
			return m, nil

		case "n":
			// Cancel cleaning confirmation
			if m.mode == ViewCleanConfirm {
				m.mode = ViewCleanOptions
				m.viewport.SetContent(m.renderCleanOptions())
				m.viewport.GotoTop()
				return m, nil
			}
			return m, nil

		case "s":
			if m.mode == ViewConflictReview && !m.editingTitle {
				conflict := m.conflicts[m.currentConflictIndex]
				conflict.UserDecision = scanner.DecisionSkipped

				if conflict.FolderMatch != nil && conflict.FilenameMatch != nil {
					if conflict.FolderMatch.Confidence >= conflict.FilenameMatch.Confidence {
						conflict.ResolvedTitle = conflict.FolderMatch.Title
					} else {
						conflict.ResolvedTitle = conflict.FilenameMatch.Title
					}
				} else if conflict.FolderMatch != nil {
					conflict.ResolvedTitle = conflict.FolderMatch.Title
				} else if conflict.FilenameMatch != nil {
					conflict.ResolvedTitle = conflict.FilenameMatch.Title
				}

				if m.currentConflictIndex < len(m.conflicts)-1 {
					m.currentConflictIndex++
				}
				m.viewport.SetContent(m.renderConflictReview())
				m.viewport.GotoTop()
				return m, nil
			}
			return m, nil

		default:
			// After cleaning completes, any key returns to summary
			if m.mode == ViewCleaning && !m.cleaning {
				m.mode = ViewSummary
				m.viewport.SetContent(m.renderSummary())
				return m, nil
			}
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
		header = "" // No header, title is in content below ASCII art
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

	case ViewConflictReview:
		header = FormatHeader("CONFLICT RESOLUTION")
		if m.editingTitle {
			footer = FormatFooter(
				FormatKeybinding("Type", "Edit"),
				FormatKeybinding("Enter", "Save"),
				FormatKeybinding("Esc", "Cancel"),
			)
		} else {
			progressInfo := fmt.Sprintf("%d/%d", m.currentConflictIndex+1, len(m.conflicts))
			footer = FormatFooter(
				FormatKeybinding("1/2/E", "Select"),
				FormatKeybinding("N/P", "Navigate"),
				FormatKeybinding("S", "Skip"),
				FormatKeybinding("Enter", "Review"),
				FormatKeybinding("Esc", "Back"),
				MutedStyle.Render(progressInfo),
			)
		}

	case ViewBatchSummary:
		header = FormatHeader("BATCH REVIEW")
		footer = FormatFooter(
			FormatKeybinding("Enter", "Apply Changes"),
			FormatKeybinding("Esc", "Back"),
		)

	case ViewBatchRenaming:
		if m.renaming {
			header = FormatHeader("BATCH RENAMING")
			footer = FormatFooter(MutedStyle.Render("Please wait..."))
		} else {
			header = FormatHeader("BATCH RENAME COMPLETE")
			footer = FormatFooter(FormatKeybinding("Enter", "Back to Summary"))
		}

	case ViewManualIntervention:
		header = FormatHeader("MANUAL INTERVENTION REQUIRED")
		if m.editingTitle {
			footer = FormatFooter(
				FormatKeybinding("Type", "Edit"),
				FormatKeybinding("Enter", "Save"),
				FormatKeybinding("Esc", "Cancel"),
			)
		} else {
			scrollInfo := fmt.Sprintf("%d/%d", m.selectedAmbiguousIndex+1, len(m.report.AmbiguousTVShows))
			footer = FormatFooter(
				FormatKeybinding("↑↓/PgUp/PgDn", "Navigate"),
				FormatKeybinding("E", "Edit Title"),
				FormatKeybinding("Enter", "Apply Renames"),
				FormatKeybinding("Esc", "Back"),
				MutedStyle.Render(scrollInfo),
			)
		}

	case ViewScanning:
		header = FormatHeader("SCANNING IN PROGRESS")
		footer = FormatFooter(
			FormatKeybinding("Ctrl+C", "Cancel Scan"),
			MutedStyle.Render("Please wait..."),
		)

	case ViewCleanConfirm:
		header = FormatHeader("CLEANUP CONFIRMATION")
		footer = FormatFooter(
			FormatKeybinding("Enter", "Confirm"),
			FormatKeybinding("N/Esc", "Cancel"),
		)

	case ViewCleanOptions:
		header = FormatHeader("CLEANUP OPTIONS")
		footer = FormatFooter(
			FormatKeybinding("↑↓", "Navigate"),
			FormatKeybinding("Enter", "Select"),
			FormatKeybinding("Esc", "Back"),
		)

	case ViewCleaning:
		header = FormatHeader("CLEANING")
		if m.cleaning {
			footer = FormatFooter(
				MutedStyle.Render("Please wait..."),
			)
		} else {
			footer = FormatFooter(
				FormatKeybinding("Any key", "Return to Menu"),
			)
		}
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

	// Title with different background to separate from ASCII art
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(RAMAForeground).
		Background(RAMABackground).
		Padding(0, 1)
	sb.WriteString(titleStyle.Render("JELLYSINK SCAN SUMMARY") + "\n\n")

	// Timestamp and library info
	sb.WriteString(InfoStyle.Render("Generated: ") + ContentStyle.Render(m.report.Timestamp.Format("2006-01-02 15:04:05")) + "\n")
	sb.WriteString(InfoStyle.Render("Library: ") + ContentStyle.Render(m.report.LibraryType) + "\n")

	// Format paths with max 4 per line to prevent overflow
	sb.WriteString(InfoStyle.Render("Paths: "))
	if len(m.report.LibraryPaths) > 0 {
		for i, path := range m.report.LibraryPaths {
			if i > 0 {
				if i%4 == 0 {
					// New line after every 4 paths
					sb.WriteString("\n       ")
				} else {
					sb.WriteString(", ")
				}
			}
			sb.WriteString(ContentStyle.Render(path))
		}
	}
	sb.WriteString("\n\n")

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

// GetResolvedConflicts returns conflicts with user decisions
func (m Model) GetResolvedConflicts() []*scanner.TVTitleResolution {
	return m.conflicts
}

// renderCleanOptions renders the cleanup options selection view
func (m Model) renderCleanOptions() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("CLEANUP OPTIONS") + "\n\n")

	sb.WriteString(InfoStyle.Render("Choose how to proceed with cleanup:") + "\n\n")

	// Show what will be cleaned
	if m.report.TotalFilesToDelete > 0 {
		sb.WriteString(MutedStyle.Render("Duplicate Deletions:") + "\n")
		sb.WriteString(fmt.Sprintf("  • %s files marked for deletion\n", StatStyle.Render(fmt.Sprintf("%d", m.report.TotalFilesToDelete))))
		sb.WriteString(fmt.Sprintf("  • %s of space to be freed\n", StatStyle.Render(formatBytes(m.report.SpaceToFree))))
		sb.WriteString("\n")
	}

	if len(m.report.ComplianceIssues) > 0 {
		sb.WriteString(MutedStyle.Render("Compliance Fixes:") + "\n")
		sb.WriteString(fmt.Sprintf("  • %s files/folders to be renamed or reorganized\n", StatStyle.Render(fmt.Sprintf("%d", len(m.report.ComplianceIssues)))))
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("─", 80) + "\n\n")

	// Options with selection cursor
	cursor := " "
	if m.cleanOptionCursor == 0 {
		cursor = "→"
	}
	selectedStyle := ContentStyle
	if m.cleanOptionCursor == 0 {
		selectedStyle = HighlightStyle
	}
	sb.WriteString(cursor + " " + selectedStyle.Render("1. DRY RUN") + " - Preview operations without making changes\n")
	sb.WriteString("     • Shows exactly what would be done\n")
	sb.WriteString("     • No files are modified or deleted\n")
	sb.WriteString("     • Safe to run multiple times\n\n")

	cursor = " "
	if m.cleanOptionCursor == 1 {
		cursor = "→"
	}
	selectedStyle = ContentStyle
	if m.cleanOptionCursor == 1 {
		selectedStyle = WarningStyle
	}
	sb.WriteString(cursor + " " + selectedStyle.Render("2. FULL CLEAN") + " - Execute all operations\n")
	sb.WriteString("     • Deletes duplicate files\n")
	sb.WriteString("     • Renames/reorganizes for compliance\n")
	sb.WriteString("     • ⚠ CANNOT BE UNDONE\n\n")

	sb.WriteString(strings.Repeat("─", 80) + "\n\n")

	sb.WriteString(SuccessStyle.Render("Press Enter to select") + " | " +
		MutedStyle.Render("Use ↑↓ to navigate") + " | " +
		MutedStyle.Render("Press Esc to cancel") + "\n")

	return sb.String()
}

func (m Model) renderCleanConfirm() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("CONFIRM CLEANUP OPERATION") + "\n\n")

	sb.WriteString(WarningStyle.Render("⚠ WARNING: You are about to perform the following operations:") + "\n\n")

	// Show what will be cleaned
	if m.report.TotalFilesToDelete > 0 {
		sb.WriteString(InfoStyle.Render("Duplicate Deletions:") + "\n")
		sb.WriteString(fmt.Sprintf("  • %s files will be deleted\n", StatStyle.Render(fmt.Sprintf("%d", m.report.TotalFilesToDelete))))
		sb.WriteString(fmt.Sprintf("  • %s of space will be freed\n", SuccessStyle.Render(formatBytes(m.report.SpaceToFree))))
		sb.WriteString("\n")
	}

	if len(m.report.ComplianceIssues) > 0 {
		sb.WriteString(InfoStyle.Render("Compliance Fixes:") + "\n")
		sb.WriteString(fmt.Sprintf("  • %s files/folders will be renamed or reorganized\n", StatStyle.Render(fmt.Sprintf("%d", len(m.report.ComplianceIssues)))))
		sb.WriteString("\n")
	}

	sb.WriteString(ErrorStyle.Render("⚠ THIS OPERATION CANNOT BE UNDONE") + "\n\n")

	sb.WriteString(MutedStyle.Render("Are you sure you want to proceed?") + "\n\n")

	sb.WriteString(SuccessStyle.Render("Press Enter to confirm") + " | " + ErrorStyle.Render("Press N or Esc to cancel") + "\n")

	return sb.String()
}

// renderCleaning renders the cleaning progress view
func (m Model) renderCleaning() string {
	var sb strings.Builder

	// ASCII header
	sb.WriteString(FormatASCIIHeader() + "\n\n")

	if m.cleaning {
		// Show title based on mode
		if m.dryRun {
			sb.WriteString(TitleStyle.Render("DRY RUN - PREVIEW MODE") + "\n")
			sb.WriteString(InfoStyle.Render("No files will be modified") + "\n\n")
		} else {
			sb.WriteString(TitleStyle.Render("CLEANING IN PROGRESS") + "\n\n")
		}

		// Show progress bar
		progressBar := renderProgressBar(m.progressPercent, 50)
		sb.WriteString(progressBar + "\n")
		sb.WriteString(fmt.Sprintf("  %s %.1f%%\n\n", m.currentProgress, m.progressPercent))

		// Show cleaning log (last 20 lines)
		if m.dryRun {
			sb.WriteString(TitleStyle.Render("PREVIEW LOG (No Changes Made)") + "\n")
		} else {
			sb.WriteString(TitleStyle.Render("CLEANING LOG") + "\n")
		}
		sb.WriteString(strings.Repeat("─", 80) + "\n")

		startIdx := 0
		if len(m.scanLogs) > 20 {
			startIdx = len(m.scanLogs) - 20
		}

		for _, log := range m.scanLogs[startIdx:] {
			var prefix string
			switch log.Severity {
			case "error":
				prefix = ErrorStyle.Render("✗")
			case "warn":
				prefix = WarningStyle.Render("⚠")
			case "success":
				prefix = SuccessStyle.Render("✓")
			default:
				prefix = InfoStyle.Render("•")
			}

			timeStr := ""
			if log.Timestamp != "" {
				timeStr = MutedStyle.Render(fmt.Sprintf("[%s] ", log.Timestamp))
			}

			sb.WriteString(fmt.Sprintf("%s %s%s\n", prefix, timeStr, log.Message))
		}
	} else {
		// Cleaning complete
		if m.dryRun {
			sb.WriteString(TitleStyle.Render("DRY RUN COMPLETE") + "\n\n")
			sb.WriteString(InfoStyle.Render("✓ Preview completed - no files were modified") + "\n\n")
		} else {
			sb.WriteString(TitleStyle.Render("CLEANUP COMPLETE") + "\n\n")
		}
		sb.WriteString(m.cleanResult + "\n\n")
		sb.WriteString(MutedStyle.Render("Press any key to exit") + "\n")
	}

	return sb.String()
}

// runCleaning executes the cleaning operation
func (m *Model) runCleaning() tea.Cmd {
	// Configure cleaner with safe defaults
	cfg := cleaner.DefaultConfig()
	cfg.DryRun = m.dryRun // Use the dryRun flag from model

	// Create progress channel and store in model
	m.cleanProgressCh = make(chan scanner.ScanProgress, 100)

	// Start cleaning in goroutine
	go func() {
		result, err := cleaner.CleanWithProgress(
			m.report.MovieDuplicates,
			m.report.TVDuplicates,
			m.report.ComplianceIssues,
			cfg,
			m.cleanProgressCh,
		)
		close(m.cleanProgressCh)

		// Send final result through a special completion progress message
		if err != nil {
			// Error case will be handled by the final message
			return
		}

		// Build result summary and send as final progress message
		var sb strings.Builder
		if result.DryRun {
			sb.WriteString(SuccessStyle.Render("✓ Dry run preview completed!") + "\n\n")
			sb.WriteString(InfoStyle.Render("Operations that would be performed:") + "\n")

			// Calculate totals from operations
			totalDuplicates := 0
			totalCompliance := 0
			for _, op := range result.Operations {
				if op.Type == "delete" {
					totalDuplicates++
				} else {
					totalCompliance++
				}
			}

			sb.WriteString(fmt.Sprintf("  • Duplicates would be deleted: %s\n", StatStyle.Render(fmt.Sprintf("%d", totalDuplicates))))
			sb.WriteString(fmt.Sprintf("  • Compliance issues would be fixed: %s\n", StatStyle.Render(fmt.Sprintf("%d", totalCompliance))))

			// Calculate potential space from duplicate operations
			potentialSpace := int64(0)
			for _, dup := range m.report.MovieDuplicates {
				for i := 1; i < len(dup.Files); i++ {
					potentialSpace += dup.Files[i].Size
				}
			}
			for _, dup := range m.report.TVDuplicates {
				for i := 1; i < len(dup.Files); i++ {
					potentialSpace += dup.Files[i].Size
				}
			}
			sb.WriteString(fmt.Sprintf("  • Space would be freed: %s\n", SuccessStyle.Render(formatBytes(potentialSpace))))
		} else {
			sb.WriteString(SuccessStyle.Render("✓ Cleanup completed successfully!") + "\n\n")
			sb.WriteString(InfoStyle.Render("Results:") + "\n")
			sb.WriteString(fmt.Sprintf("  • Duplicates deleted: %s\n", StatStyle.Render(fmt.Sprintf("%d", result.DuplicatesDeleted))))
			sb.WriteString(fmt.Sprintf("  • Compliance fixed: %s\n", StatStyle.Render(fmt.Sprintf("%d", result.ComplianceFixed))))
			sb.WriteString(fmt.Sprintf("  • Space freed: %s\n", SuccessStyle.Render(formatBytes(result.SpaceFreed))))
		}

		if len(result.Errors) > 0 {
			sb.WriteString(fmt.Sprintf("\n%s\n", WarningStyle.Render(fmt.Sprintf("⚠ %d error(s) occurred:", len(result.Errors)))))
			for i, err := range result.Errors {
				if i >= 5 {
					sb.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(result.Errors)-5)))
					break
				}
				sb.WriteString(ErrorStyle.Render(fmt.Sprintf("  • %v\n", err)))
			}
		}

		m.cleanResult = sb.String()
	}()

	// Wait for first progress message
	return waitForCleanProgress(m.cleanProgressCh)
}

func waitForCleanProgress(progressCh chan scanner.ScanProgress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-progressCh
		if !ok {
			// Channel closed, cleaning is complete
			return cleanCompleteMsg{
				result: "", // Result already set in model
				err:    nil,
			}
		}
		return cleanProgressMsg(progress)
	}
}

// cleanCompleteMsg is sent when cleaning finishes
type cleanCompleteMsg struct {
	result string
	err    error
}

// renderBatchRenaming renders the batch rename progress view
func (m Model) renderBatchRenaming() string {
	var sb strings.Builder

	// ASCII header
	sb.WriteString(FormatASCIIHeader() + "\n\n")

	if m.renaming {
		sb.WriteString(TitleStyle.Render("BATCH RENAMING IN PROGRESS") + "\n\n")

		// Show progress bar
		progressBar := renderProgressBar(m.progressPercent, 50)
		sb.WriteString(progressBar + "\n")
		sb.WriteString(fmt.Sprintf("  %s %.1f%%\n\n", m.currentProgress, m.progressPercent))

		// Show rename log (last 20 lines)
		sb.WriteString(TitleStyle.Render("RENAME LOG") + "\n")
		sb.WriteString(strings.Repeat("─", 80) + "\n")

		startIdx := 0
		if len(m.scanLogs) > 20 {
			startIdx = len(m.scanLogs) - 20
		}

		for _, log := range m.scanLogs[startIdx:] {
			var prefix string
			switch log.Severity {
			case "error":
				prefix = ErrorStyle.Render("✗")
			case "warn":
				prefix = WarningStyle.Render("⚠")
			case "success":
				prefix = SuccessStyle.Render("✓")
			default:
				prefix = InfoStyle.Render("•")
			}

			timeStr := ""
			if log.Timestamp != "" {
				timeStr = MutedStyle.Render(fmt.Sprintf("[%s] ", log.Timestamp))
			}

			sb.WriteString(fmt.Sprintf("%s %s%s\n", prefix, timeStr, log.Message))
		}
	} else {
		// Renaming complete
		sb.WriteString(TitleStyle.Render("BATCH RENAME COMPLETE") + "\n\n")
		sb.WriteString(m.renameResult + "\n\n")
		sb.WriteString(MutedStyle.Render("Press Enter to return to summary") + "\n")
	}

	return sb.String()
}

// runBatchRename executes the batch rename operation
func (m *Model) runBatchRename() tea.Cmd {
	// Create progress channel and store in model
	m.renameProgressCh = make(chan scanner.ScanProgress, 100)

	// Start renaming in goroutine
	go func() {
		var allResults []scanner.RenameResult
		var allErrors []error
		totalConflicts := 0
		successCount := 0
		errorCount := 0

		pr := scanner.NewProgressReporter(m.renameProgressCh, "batch_rename")
		pr.Start(len(m.conflicts), "Starting batch rename")

		// Process each conflict resolution
		for i, conflict := range m.conflicts {
			if conflict.UserDecision == scanner.DecisionSkipped {
				continue
			}

			totalConflicts++
			percent := float64(i+1) / float64(len(m.conflicts)) * 100
			pr.Update(int(percent), fmt.Sprintf("Processing %d/%d: %s", i+1, len(m.conflicts), conflict.ResolvedTitle))

			// Determine the new title based on user decision
			var newTitle string
			switch conflict.UserDecision {
			case scanner.DecisionFolderTitle:
				if conflict.FolderMatch != nil {
					newTitle = conflict.FolderMatch.Title
				}
			case scanner.DecisionFilenameTitle:
				if conflict.FilenameMatch != nil {
					newTitle = conflict.FilenameMatch.Title
				}
			case scanner.DecisionCustomTitle:
				newTitle = conflict.CustomTitle
			default:
				newTitle = conflict.ResolvedTitle
			}

			if newTitle == "" {
				pr.SendSeverityImmediate("error", fmt.Sprintf("No valid title for: %s", conflict.FolderPath))
				errorCount++
				continue
			}

			// Apply rename for this show
			// We need to pass the parent directory (library path) not the show folder itself
			basePath := filepath.Dir(conflict.FolderPath)
			// Compute oldTitle based on folder match or filename match (fallback to ResolvedTitle)
			oldTitle := ""
			if conflict.FolderMatch != nil {
				oldTitle = conflict.FolderMatch.Title
			} else if conflict.FilenameMatch != nil {
				oldTitle = conflict.FilenameMatch.Title
			} else {
				// As a last resort, extract title from folder path
				oldTitle, _ = scanner.ExtractTVShowTitle(filepath.Base(conflict.FolderPath))
			}

			results, err := scanner.ApplyManualTVRenameWithProgress(
				basePath,
				oldTitle,
				newTitle,
				false, // Not dry run
				pr,
			)

			if err != nil {
				pr.SendSeverityImmediate("error", fmt.Sprintf("Failed to rename %s: %v", oldTitle, err))
				allErrors = append(allErrors, err)
				errorCount++
			} else if len(results) == 0 {
				pr.SendSeverityImmediate("warn", fmt.Sprintf("No files or folder found to rename for: %s", oldTitle))
				allErrors = append(allErrors, fmt.Errorf("no files renamed for %s", oldTitle))
				errorCount++
			} else {
				allResults = append(allResults, results...)
				successCount++
				pr.SendSeverityImmediate("success", fmt.Sprintf("Renamed: %s → %s (%d files)", oldTitle, newTitle, len(results)))
			}
		}

		pr.Complete("Batch rename complete")
		close(m.renameProgressCh)

		// Build result summary
		var sb strings.Builder
		if errorCount == 0 {
			sb.WriteString(SuccessStyle.Render("✓ Batch rename completed successfully!") + "\n\n")
		} else {
			sb.WriteString(WarningStyle.Render("⚠ Batch rename completed with errors") + "\n\n")
		}

		sb.WriteString(InfoStyle.Render("Results:") + "\n")
		sb.WriteString(fmt.Sprintf("  • Shows processed: %s\n", StatStyle.Render(fmt.Sprintf("%d", totalConflicts))))
		sb.WriteString(fmt.Sprintf("  • Successful: %s\n", SuccessStyle.Render(fmt.Sprintf("%d", successCount))))
		if errorCount > 0 {
			sb.WriteString(fmt.Sprintf("  • Failed: %s\n", ErrorStyle.Render(fmt.Sprintf("%d", errorCount))))
		}
		sb.WriteString(fmt.Sprintf("  • Total file operations: %s\n", StatStyle.Render(fmt.Sprintf("%d", len(allResults)))))

		if len(allErrors) > 0 {
			sb.WriteString(fmt.Sprintf("\n%s\n", ErrorStyle.Render(fmt.Sprintf("✗ %d error(s) occurred:", len(allErrors)))))
			for i, err := range allErrors {
				if i >= 5 {
					sb.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(allErrors)-5)))
					break
				}
				sb.WriteString(ErrorStyle.Render(fmt.Sprintf("  • %v\n", err)))
			}
		}

		m.renameResult = sb.String()
		m.renameErrors = allErrors
	}()

	// Wait for first progress message
	return waitForRenameProgress(m.renameProgressCh)
}

func waitForRenameProgress(progressCh chan scanner.ScanProgress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-progressCh
		if !ok {
			// Channel closed, renaming is complete
			return renameCompleteMsg{
				result: "", // Result already set in model
			}
		}
		return renameProgressMsg(progress)
	}
}

// renderConflictReview renders single conflict review screen (Stage 1)
func (m Model) renderConflictReview() string {
	var sb strings.Builder

	if len(m.conflicts) == 0 {
		sb.WriteString(SuccessStyle.Render("✓ No conflicts to resolve") + "\n")
		return sb.String()
	}

	conflict := m.conflicts[m.currentConflictIndex]

	sb.WriteString(TitleStyle.Render("TV SHOW TITLE CONFLICT") + "\n\n")

	sb.WriteString(InfoStyle.Render(fmt.Sprintf("Reviewing conflict %d of %d", m.currentConflictIndex+1, len(m.conflicts))) + "\n\n")

	sb.WriteString(HighlightStyle.Render("⚠ CONFLICTING TITLES DETECTED") + "\n\n")

	if conflict.FolderMatch != nil {
		sb.WriteString(InfoStyle.Render("Option 1: Folder Title") + "\n")
		sb.WriteString(fmt.Sprintf("  %s", ContentStyle.Render(conflict.FolderMatch.Title)))
		if conflict.FolderMatch.Year != "" {
			sb.WriteString(MutedStyle.Render(fmt.Sprintf(" (%s)", conflict.FolderMatch.Year)))
		}
		sb.WriteString(fmt.Sprintf(" [confidence: %s]\n", StatStyle.Render(fmt.Sprintf("%.0f%%", conflict.FolderMatch.Confidence*100))))
		if conflict.UserDecision == scanner.DecisionFolderTitle {
			sb.WriteString(SuccessStyle.Render("  ✓ SELECTED") + "\n")
		} else {
			sb.WriteString(MutedStyle.Render("  Press '1' to select") + "\n")
		}
		sb.WriteString("\n")
	}

	if conflict.FilenameMatch != nil {
		sb.WriteString(InfoStyle.Render("Option 2: Filename Title") + "\n")
		sb.WriteString(fmt.Sprintf("  %s", ContentStyle.Render(conflict.FilenameMatch.Title)))
		if conflict.FilenameMatch.Year != "" {
			sb.WriteString(MutedStyle.Render(fmt.Sprintf(" (%s)", conflict.FilenameMatch.Year)))
		}
		sb.WriteString(fmt.Sprintf(" [confidence: %s]\n", StatStyle.Render(fmt.Sprintf("%.0f%%", conflict.FilenameMatch.Confidence*100))))
		if conflict.UserDecision == scanner.DecisionFilenameTitle {
			sb.WriteString(SuccessStyle.Render("  ✓ SELECTED") + "\n")
		} else {
			sb.WriteString(MutedStyle.Render("  Press '2' to select") + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(InfoStyle.Render("Option 3: Custom Title") + "\n")
	if conflict.UserDecision == scanner.DecisionCustomTitle && conflict.CustomTitle != "" {
		sb.WriteString(fmt.Sprintf("  %s\n", SuccessStyle.Render(conflict.CustomTitle)))
		sb.WriteString(SuccessStyle.Render("  ✓ SELECTED") + "\n")
	} else if m.editingTitle {
		sb.WriteString("  " + m.titleInput.View() + "\n")
		sb.WriteString(MutedStyle.Render("  Press Enter to save, Esc to cancel") + "\n")
	} else {
		sb.WriteString(MutedStyle.Render("  Press 'E' to enter custom title") + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString(strings.Repeat("─", 80) + "\n\n")

	sb.WriteString(MutedStyle.Render("Conflict Reason: ") + ErrorStyle.Render(conflict.Reason) + "\n")

	if conflict.APIVerified {
		sb.WriteString(WarningStyle.Render("⚠ API returned conflicting results") + "\n")
	} else {
		sb.WriteString(MutedStyle.Render("ℹ API verification unavailable") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", 80) + "\n\n")

	hasDecision := conflict.UserDecision != scanner.DecisionNone

	// Check if all conflicts have decisions
	allDecided := true
	for _, c := range m.conflicts {
		if c.UserDecision == scanner.DecisionNone {
			allDecided = false
			break
		}
	}

	if hasDecision {
		sb.WriteString(SuccessStyle.Render("✓ Decision recorded") + "\n")
		sb.WriteString(MutedStyle.Render("  ← → to navigate conflicts") + "\n")
		if allDecided {
			sb.WriteString(SuccessStyle.Render("  ✓ All conflicts resolved - Press Enter to proceed to batch review") + "\n")
		}
	} else {
		sb.WriteString(WarningStyle.Render("⚠ No decision made yet") + "\n")
		sb.WriteString(MutedStyle.Render("  Select an option (1/2/E) or press 'S' to skip") + "\n")
		sb.WriteString(MutedStyle.Render("  ← → to navigate conflicts") + "\n")
	}

	return sb.String()
}

// renderBatchSummary renders decision summary table (Stage 2)
func (m Model) renderBatchSummary() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render("BATCH REVIEW SUMMARY") + "\n\n")

	sb.WriteString(InfoStyle.Render(fmt.Sprintf("Reviewing %d decision(s) before applying changes", len(m.conflicts))) + "\n\n")

	sb.WriteString(strings.Repeat("─", 100) + "\n")
	sb.WriteString(fmt.Sprintf("%-4s %-30s %-30s %-15s\n",
		HighlightStyle.Render("#"),
		HighlightStyle.Render("Show Name"),
		HighlightStyle.Render("New Title"),
		HighlightStyle.Render("Source")))
	sb.WriteString(strings.Repeat("─", 100) + "\n")

	for i, conflict := range m.conflicts {
		cursor := "  "
		if i == m.batchReviewCursor {
			cursor = "→ "
		}

		var oldTitle string
		if conflict.FolderMatch != nil {
			oldTitle = conflict.FolderMatch.Title
		} else if conflict.FilenameMatch != nil {
			oldTitle = conflict.FilenameMatch.Title
		}

		newTitle := conflict.ResolvedTitle
		var source string
		switch conflict.UserDecision {
		case scanner.DecisionFolderTitle:
			source = "Folder"
		case scanner.DecisionFilenameTitle:
			source = "Filename"
		case scanner.DecisionCustomTitle:
			source = "Custom"
		case scanner.DecisionSkipped:
			source = "Auto (skipped)"
		default:
			source = "Unknown"
		}

		lineStyle := ContentStyle
		if i == m.batchReviewCursor {
			lineStyle = HighlightStyle
		}

		sb.WriteString(lineStyle.Render(fmt.Sprintf("%s%-2d %-30s %-30s %-15s\n",
			cursor,
			i+1,
			truncate(oldTitle, 30),
			truncate(newTitle, 30),
			source)))
	}

	sb.WriteString(strings.Repeat("─", 100) + "\n\n")

	sb.WriteString(InfoStyle.Render("Next Steps:") + "\n")
	sb.WriteString("  • Review the decisions above\n")
	sb.WriteString("  • Press Enter to apply all renames\n")
	sb.WriteString("  • Press Esc to go back and make changes\n\n")

	sb.WriteString(WarningStyle.Render("⚠ Renames will be applied to both folders and filenames for consistency") + "\n")

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
