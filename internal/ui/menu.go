package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
	"github.com/Nomadcxx/jellysink/internal/reporter"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

// MenuItem represents a menu option
type MenuItem struct {
	title string
	desc  string
}

func (i MenuItem) Title() string       { return i.title }
func (i MenuItem) Description() string { return i.desc }
func (i MenuItem) FilterValue() string { return i.title }

// MenuModel represents the main menu TUI
type MenuModel struct {
	list       list.Model
	config     *config.Config
	width      int
	height     int
	ctx        context.Context
	cancel     context.CancelFunc
	showStatus bool // Show config status popup
}

// NewMenuModel creates a new main menu
func NewMenuModel(cfg *config.Config) MenuModel {
	items := []list.Item{
		MenuItem{title: "Run Manual Scan", desc: "Scan your media libraries for duplicates and compliance issues"},
		MenuItem{title: "View Last Report", desc: "View the most recent scan report"},
		MenuItem{title: "Configure Frequency", desc: "Set automatic scan frequency (daily/weekly/biweekly)"},
		MenuItem{title: "Enable/Disable Daemon", desc: "Toggle automatic background scanning"},
		MenuItem{title: "Configure Libraries", desc: "Add or remove media library paths"},
		MenuItem{title: "Configure API Keys", desc: "Set TVDB/OMDB API keys for metadata resolution"},
		MenuItem{title: "Exit", desc: "Quit jellysink"},
	}

	// Create delegate with RAMA theme styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMARed).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMAFireRed)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(RAMAForeground)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(RAMAMuted)

	l := list.New(items, delegate, 80, 20)
	l.Title = "JELLYSINK MAIN MENU"
	l.Styles.Title = TitleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.SetShowTitle(true)

	ctx, cancel := context.WithCancel(context.Background())

	return MenuModel{
		list:   l,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// scanStatusMsg is sent when scan completes
type scanStatusMsg struct {
	reportPath string
	err        error
}

// progressTickMsg is sent periodically to update progress animation
type progressTickMsg struct{}

// Init initializes the menu
func (m MenuModel) Init() tea.Cmd {
	return nil
}

// Update handles menu messages
func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit

		case "i", "s":
			// Toggle status popup with 'i' (info) or 's' (status)
			m.showStatus = !m.showStatus
			return m, nil

		case "esc":
			// Close status popup if open
			if m.showStatus {
				m.showStatus = false
				return m, nil
			}

		case "enter":
			selected := m.list.SelectedItem().(MenuItem)
			return m.handleSelection(selected.title)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Calculate list height after accounting for other content:
		// ASCII header (9 lines) + spacing (2) + footer (3) + padding (2) = 16 lines
		listHeight := msg.Height - 16
		if listHeight < 8 {
			listHeight = 8
		}
		m.list.SetSize(msg.Width-4, listHeight)
		return m, nil

	case scanStatusMsg:
		// Scan completed
		if msg.err != nil {
			// Show error and return to menu
			return m, tea.Printf("Scan failed: %v", msg.err)
		}
		// Load report JSON and switch to report view
		report, err := loadReportJSON(msg.reportPath)
		if err != nil {
			return m, tea.Printf("Failed to load report: %v", err)
		}

		// Create report view model and transfer dimensions
		reportModel := NewModel(report)
		return reportModel, func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleSelection processes menu selections
func (m MenuModel) handleSelection(title string) (tea.Model, tea.Cmd) {
	switch title {
	case "Run Manual Scan":
		scanningModel := NewScanningModel(m.config)
		scanningModel.width = m.width
		scanningModel.height = m.height
		return scanningModel, scanningModel.Init()

	case "View Last Report":
		return m, m.viewLastReport

	case "Configure Frequency":
		freqModel := NewFrequencyMenuModel(m.config)
		freqModel.width = m.width
		freqModel.height = m.height
		return freqModel, nil

	case "Enable/Disable Daemon":
		daemonModel := NewDaemonMenuModel(m.config)
		daemonModel.width = m.width
		daemonModel.height = m.height
		return daemonModel, nil

	case "Configure Libraries":
		libModel := NewLibraryMenuModel(m.config)
		libModel.width = m.width
		libModel.height = m.height
		// Set initial list size
		listHeight := m.height - 16
		if listHeight < 8 {
			listHeight = 8
		}
		libModel.list.SetSize(m.width-4, listHeight)
		return libModel, nil

	case "Configure API Keys":
		apiModel := NewAPIConfigModel(m.config)
		apiModel.width = m.width
		apiModel.height = m.height
		return apiModel, apiModel.Init()

	case "Exit":
		m.cancel()
		return m, tea.Quit
	}

	return m, nil
}

// viewLastReport finds and displays the most recent report
func (m MenuModel) viewLastReport() tea.Msg {
	// Find most recent JSON report
	var scanResultsPath string

	// If running with sudo, use the real user's home directory
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		scanResultsPath = "/home/" + sudoUser + "/.local/share/jellysink/scan_results"
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return scanStatusMsg{err: fmt.Errorf("failed to get home directory: %w", err)}
		}
		scanResultsPath = homeDir + "/.local/share/jellysink/scan_results"
	}

	// List all JSON files
	files, err := os.ReadDir(scanResultsPath)
	if err != nil {
		return scanStatusMsg{err: fmt.Errorf("failed to read scan results directory: %w", err)}
	}

	// Filter for JSON files and find most recent
	var mostRecent string
	var mostRecentTime time.Time

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if mostRecent == "" || info.ModTime().After(mostRecentTime) {
			mostRecent = file.Name()
			mostRecentTime = info.ModTime()
		}
	}

	if mostRecent == "" {
		return scanStatusMsg{err: fmt.Errorf("no scan reports found in %s", scanResultsPath)}
	}

	reportPath := scanResultsPath + "/" + mostRecent

	// Load and return as scanStatusMsg to trigger report view
	return scanStatusMsg{reportPath: reportPath, err: nil}
}

// loadReportJSON loads a report from a JSON file
func loadReportJSON(path string) (reporter.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reporter.Report{}, fmt.Errorf("failed to read report: %w", err)
	}

	var report reporter.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return reporter.Report{}, fmt.Errorf("failed to parse report: %w", err)
	}

	return report, nil
}

// View renders the menu
func (m MenuModel) View() string {
	// Minimum dimensions for ASCII art: 100 width x 25 height
	const minWidth = 100
	const minHeight = 25

	// Check if terminal is too small (only after dimensions are set)
	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	// Show ASCII header
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	// Add menu list
	content.WriteString(m.list.View())
	content.WriteString("\n\n")

	// Footer help text
	footer := MutedStyle.Render("↑/↓: Navigate  •  Enter: Select  •  I/S: Status  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	// Wrap in padding
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	mainView := mainStyle.Render(content.String())

	// If status popup is showing, overlay it on top
	if m.showStatus {
		return m.renderWithStatusPopup(mainView)
	}

	return mainView
}

// renderWithStatusPopup overlays the status popup on the main view
func (m MenuModel) renderWithStatusPopup(baseView string) string {
	// Build status popup content
	var popup strings.Builder

	popup.WriteString(TitleStyle.Render("CONFIGURATION STATUS") + "\n\n")

	popup.WriteString(InfoStyle.Render("Libraries:") + "\n")
	popup.WriteString(fmt.Sprintf("  Movie paths: %s\n", StatStyle.Render(fmt.Sprintf("%d", len(m.config.Libraries.Movies.Paths)))))
	popup.WriteString(fmt.Sprintf("  TV paths: %s\n", StatStyle.Render(fmt.Sprintf("%d", len(m.config.Libraries.TV.Paths)))))
	popup.WriteString("\n")

	popup.WriteString(InfoStyle.Render("Daemon:") + "\n")
	popup.WriteString(fmt.Sprintf("  Scan frequency: %s\n", SuccessStyle.Render(m.config.Daemon.ScanFrequency)))

	// Show daemon status
	daemonStatus := getDaemonStatusString()
	var statusStyle lipgloss.Style
	if daemonStatus == "Running" {
		statusStyle = SuccessStyle
	} else if daemonStatus == "Stopped" {
		statusStyle = MutedStyle
	} else {
		statusStyle = WarningStyle
	}
	popup.WriteString(fmt.Sprintf("  Status: %s\n", statusStyle.Render(daemonStatus)))

	// Create bordered popup (sysc-greet style)
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(RAMARed).
		Background(RAMABackground).
		Padding(2, 4)

	popupBox := popupStyle.Render(popup.String())

	// Add help text for closing popup
	closeHelp := MutedStyle.Render("Press I/S or Esc to close")
	popupWithHelp := popupBox + "\n" + lipgloss.NewStyle().Align(lipgloss.Center).Width(lipgloss.Width(popupBox)).Render(closeHelp)

	// Center popup on screen using Place
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popupWithHelp)
}

// FrequencyMenuModel handles scan frequency configuration
type FrequencyMenuModel struct {
	list   list.Model
	config *config.Config
	width  int
	height int
}

// NewFrequencyMenuModel creates frequency selection menu
func NewFrequencyMenuModel(cfg *config.Config) FrequencyMenuModel {
	items := []list.Item{
		MenuItem{title: "Daily", desc: "Scan every day at 2:00 AM"},
		MenuItem{title: "Weekly", desc: "Scan every Sunday at 2:00 AM"},
		MenuItem{title: "Biweekly", desc: "Scan every other Sunday at 2:00 AM"},
		MenuItem{title: "Back", desc: "Return to main menu"},
	}

	// Create delegate with RAMA theme styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMARed).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMAFireRed)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(RAMAForeground)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(RAMAMuted)

	l := list.New(items, delegate, 80, 20)
	l.Title = "SET SCAN FREQUENCY"
	l.Styles.Title = TitleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.SetShowTitle(true)

	return FrequencyMenuModel{
		list:   l,
		config: cfg,
	}
}

func (m FrequencyMenuModel) Init() tea.Cmd {
	return nil
}

func (m FrequencyMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return NewMenuModel(m.config), nil

		case "enter":
			selected := m.list.SelectedItem().(MenuItem)
			freq := strings.ToLower(selected.title)
			if freq == "back" {
				return NewMenuModel(m.config), nil
			}
			m.config.Daemon.ScanFrequency = freq
			config.Save(m.config)
			return NewMenuModel(m.config), tea.Printf("Scan frequency set to %s", freq)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// ASCII header (9) + spacing (2) + footer (3) + padding (2) = 16 lines
		listHeight := msg.Height - 16
		if listHeight < 8 {
			listHeight = 8
		}
		m.list.SetSize(msg.Width-4, listHeight)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m FrequencyMenuModel) View() string {
	// Minimum dimensions check
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")
	content.WriteString(m.list.View())
	content.WriteString("\n\n")

	// Footer help text
	footer := MutedStyle.Render("↑/↓: Navigate  •  Enter: Select  •  Esc: Back  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// DaemonMenuModel handles daemon enable/disable
type DaemonMenuModel struct {
	list   list.Model
	config *config.Config
	width  int
	height int
}

// NewDaemonMenuModel creates daemon toggle menu
func NewDaemonMenuModel(cfg *config.Config) DaemonMenuModel {
	items := []list.Item{
		MenuItem{title: "Enable Daemon", desc: "Enable automatic background scanning"},
		MenuItem{title: "Disable Daemon", desc: "Disable automatic background scanning"},
		MenuItem{title: "Daemon Status", desc: "Check if daemon is running"},
		MenuItem{title: "Back", desc: "Return to main menu"},
	}

	// Create delegate with RAMA theme styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMARed).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMAFireRed)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(RAMAForeground)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(RAMAMuted)

	l := list.New(items, delegate, 80, 20)
	l.Title = "DAEMON MANAGEMENT"
	l.Styles.Title = TitleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.SetShowTitle(true)

	return DaemonMenuModel{
		list:   l,
		config: cfg,
	}
}

func (m DaemonMenuModel) Init() tea.Cmd {
	return nil
}

func (m DaemonMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return NewMenuModel(m.config), nil

		case "enter":
			selected := m.list.SelectedItem().(MenuItem)
			switch selected.title {
			case "Back":
				return NewMenuModel(m.config), nil
			case "Enable Daemon":
				// Enable and start the timer
				cmd := exec.Command("systemctl", "enable", "--now", "jellysink.timer")
				if err := cmd.Run(); err != nil {
					return NewMenuModel(m.config), tea.Printf("Failed to enable daemon: %v", err)
				}
				return NewMenuModel(m.config), tea.Printf("Daemon enabled successfully")
			case "Disable Daemon":
				// Disable and stop the timer
				cmd := exec.Command("systemctl", "disable", "--now", "jellysink.timer")
				if err := cmd.Run(); err != nil {
					return NewMenuModel(m.config), tea.Printf("Failed to disable daemon: %v", err)
				}
				return NewMenuModel(m.config), tea.Printf("Daemon disabled successfully")
			case "Daemon Status":
				// Show detailed status
				timerActive, serviceActive := checkDaemonStatus()
				statusMsg := fmt.Sprintf("Timer: %s, Service: %s",
					boolToStatus(timerActive),
					boolToStatus(serviceActive))
				return m, tea.Printf("%s", statusMsg)
			default:
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// ASCII header (9) + status section (5) + footer (3) + padding (2) = 19 lines
		listHeight := msg.Height - 19
		if listHeight < 6 {
			listHeight = 6
		}
		m.list.SetSize(msg.Width-4, listHeight)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m DaemonMenuModel) View() string {
	// Minimum dimensions check
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	// Show current daemon status with markers
	timerActive, serviceActive := checkDaemonStatus()
	content.WriteString(InfoStyle.Render("Current Status:") + "\n")

	// Timer status with marker
	if timerActive {
		content.WriteString("  " + FormatStatusOK("Timer Active") + "\n")
	} else {
		content.WriteString("  " + FormatStatusInfo("Timer Inactive") + "\n")
	}

	// Service status with marker
	if serviceActive {
		content.WriteString("  " + FormatStatusOK("Service Active") + "\n")
	} else {
		content.WriteString("  " + FormatStatusInfo("Service Inactive") + "\n")
	}
	content.WriteString("\n")

	content.WriteString(m.list.View())
	content.WriteString("\n\n")

	// Footer help text
	footer := MutedStyle.Render("↑/↓: Navigate  •  Enter: Select  •  Esc: Back  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// LibraryMenuModel handles library path configuration
type LibraryMenuModel struct {
	list   list.Model
	config *config.Config
	width  int
	height int
}

// NewLibraryMenuModel creates library configuration menu
func NewLibraryMenuModel(cfg *config.Config) LibraryMenuModel {
	items := []list.Item{
		MenuItem{title: "Add Movie Library", desc: "Add a new movie library path"},
		MenuItem{title: "Add TV Library", desc: "Add a new TV show library path"},
		MenuItem{title: "Remove Library", desc: "Remove an existing library path"},
		MenuItem{title: "List Libraries", desc: "Show all configured library paths"},
		MenuItem{title: "Back", desc: "Return to main menu"},
	}

	// Create delegate with RAMA theme styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMARed).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMAFireRed)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(RAMAForeground)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(RAMAMuted)

	l := list.New(items, delegate, 80, 20)
	l.Title = "LIBRARY CONFIGURATION"
	l.Styles.Title = TitleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.SetShowTitle(true)

	return LibraryMenuModel{
		list:   l,
		config: cfg,
	}
}

func (m LibraryMenuModel) Init() tea.Cmd {
	return nil
}

func (m LibraryMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			mainMenu := NewMenuModel(m.config)
			mainMenu.width = m.width
			mainMenu.height = m.height
			listHeight := m.height - 16
			if listHeight < 8 {
				listHeight = 8
			}
			mainMenu.list.SetSize(m.width-4, listHeight)
			return mainMenu, nil

		case "enter":
			selected := m.list.SelectedItem().(MenuItem)
			switch selected.title {
			case "Back":
				mainMenu := NewMenuModel(m.config)
				mainMenu.width = m.width
				mainMenu.height = m.height
				listHeight := m.height - 16
				if listHeight < 8 {
					listHeight = 8
				}
				mainMenu.list.SetSize(m.width-4, listHeight)
				return mainMenu, nil
			case "Add Movie Library":
				addModel := NewAddPathModel(m.config, "movie")
				addModel.width = m.width
				addModel.height = m.height
				return addModel, nil
			case "Add TV Library":
				addModel := NewAddPathModel(m.config, "tv")
				addModel.width = m.width
				addModel.height = m.height
				return addModel, nil
			case "Remove Library":
				removeModel := NewRemovePathModel(m.config)
				removeModel.width = m.width
				removeModel.height = m.height
				// Set initial list size for remove model
				listHeight := m.height - 16
				if listHeight < 8 {
					listHeight = 8
				}
				removeModel.list.SetSize(m.width-4, listHeight)
				return removeModel, nil
			case "List Libraries":
				listModel := NewListLibrariesModel(m.config)
				listModel.width = m.width
				listModel.height = m.height
				// Initialize viewport immediately with dimensions
				headerHeight := 15
				footerHeight := 4
				listModel.viewport = viewport.New(m.width-4, m.height-headerHeight-footerHeight)
				listModel.viewport.Style = lipgloss.NewStyle().Padding(0, 1)
				listModel.viewport.SetContent(listModel.buildLibraryList())
				listModel.ready = true
				return listModel, nil
			default:
				// Let list handle other keys
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(msg)
				return m, cmd
			}
		default:
			// Let list handle all other keys
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// ASCII header (9) + spacing (2) + footer (3) + padding (2) = 16 lines
		listHeight := msg.Height - 16
		if listHeight < 8 {
			listHeight = 8
		}
		m.list.SetSize(msg.Width-4, listHeight)
		return m, nil
	}

	return m, nil
}

func (m LibraryMenuModel) View() string {
	// Minimum dimensions check
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	// Show ASCII header
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	// Show menu list directly (library preview removed - use "List Libraries" option instead)
	content.WriteString(m.list.View())
	content.WriteString("\n\n")

	// Footer help text
	footer := MutedStyle.Render("↑/↓: Navigate  •  Enter: Select  •  Esc: Back  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	// Wrap in padding like other menus
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// AddPathModel handles adding a library path with text input
type AddPathModel struct {
	textInput   textinput.Model
	config      *config.Config
	libraryType string // "movie" or "tv"
	width       int
	height      int
	err         string
	success     string
}

// NewAddPathModel creates a new path input model
func NewAddPathModel(cfg *config.Config, libraryType string) AddPathModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/your/" + libraryType + "s"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80

	// Style the text input with RAMA theme
	ti.PromptStyle = lipgloss.NewStyle().Foreground(RAMARed)
	ti.TextStyle = lipgloss.NewStyle().Foreground(RAMAForeground)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(RAMAMuted)

	return AddPathModel{
		textInput:   ti,
		config:      cfg,
		libraryType: libraryType,
	}
}

func (m AddPathModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m AddPathModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			libModel := NewLibraryMenuModel(m.config)
			libModel.width = m.width
			libModel.height = m.height
			listHeight := m.height - 16
			if listHeight < 8 {
				listHeight = 8
			}
			libModel.list.SetSize(m.width-4, listHeight)
			return libModel, nil

		case "esc":
			// Cancel and return to library menu
			libModel := NewLibraryMenuModel(m.config)
			libModel.width = m.width
			libModel.height = m.height
			listHeight := m.height - 16
			if listHeight < 8 {
				listHeight = 8
			}
			libModel.list.SetSize(m.width-4, listHeight)
			return libModel, nil

		case "enter":
			// Validate and add path
			path := strings.TrimSpace(m.textInput.Value())
			if path == "" {
				m.err = "Path cannot be empty"
				m.success = ""
				return m, nil
			}

			// Validate path exists
			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					m.err = "Path does not exist"
				} else {
					m.err = fmt.Sprintf("Cannot access path: %v", err)
				}
				m.success = ""
				return m, nil
			}

			// Check if it's a directory
			if !info.IsDir() {
				m.err = "Path must be a directory"
				m.success = ""
				return m, nil
			}

			// Check if path already exists in the library
			var existingPaths []string
			if m.libraryType == "movie" {
				existingPaths = m.config.Libraries.Movies.Paths
			} else {
				existingPaths = m.config.Libraries.TV.Paths
			}

			for _, existing := range existingPaths {
				if existing == path {
					m.err = "Path already exists in library"
					m.success = ""
					return m, nil
				}
			}

			// Add path to config
			if m.libraryType == "movie" {
				m.config.Libraries.Movies.Paths = append(m.config.Libraries.Movies.Paths, path)
			} else {
				m.config.Libraries.TV.Paths = append(m.config.Libraries.TV.Paths, path)
			}

			// Save config
			if err := config.Save(m.config); err != nil {
				m.err = fmt.Sprintf("Failed to save config: %v", err)
				m.success = ""
				return m, nil
			}

			// Show success and clear input for next path
			m.success = fmt.Sprintf("Added: %s", path)
			m.err = ""
			m.textInput.SetValue("")
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m AddPathModel) View() string {
	// Minimum dimensions check
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	// Show ASCII header
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	// Title
	var title string
	if m.libraryType == "movie" {
		title = "ADD MOVIE LIBRARY PATH"
	} else {
		title = "ADD TV LIBRARY PATH"
	}
	content.WriteString(TitleStyle.Render(title) + "\n\n")

	// Show currently configured paths
	var currentPaths []string
	if m.libraryType == "movie" {
		currentPaths = m.config.Libraries.Movies.Paths
		content.WriteString(InfoStyle.Render("Currently configured Movie libraries:") + "\n")
	} else {
		currentPaths = m.config.Libraries.TV.Paths
		content.WriteString(InfoStyle.Render("Currently configured TV libraries:") + "\n")
	}

	if len(currentPaths) == 0 {
		content.WriteString("  " + MutedStyle.Render("None") + "\n")
	} else {
		for i, p := range currentPaths {
			content.WriteString(fmt.Sprintf("  %d. %s\n", i+1, MutedStyle.Render(p)))
		}
	}
	content.WriteString("\n")

	// Instructions
	content.WriteString(InfoStyle.Render("Enter the full path to your library folder:") + "\n\n")

	// Text input
	content.WriteString(m.textInput.View())
	content.WriteString("\n\n")

	// Error message
	if m.err != "" {
		content.WriteString(ErrorStyle.Render("✗ "+m.err) + "\n\n")
	}

	// Success message
	if m.success != "" {
		content.WriteString(SuccessStyle.Render("✓ "+m.success) + "\n\n")
	}

	// Help text
	content.WriteString(MutedStyle.Render("Enter: Add path  •  Esc: Cancel  •  Ctrl+C/Q: Exit"))

	// Wrap in padding
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// RemovePathModel handles removing a library path
type RemovePathModel struct {
	list   list.Model
	config *config.Config
	width  int
	height int
}

// RemovablePathItem represents a path that can be removed
type RemovablePathItem struct {
	path        string
	libraryType string // "movie" or "tv"
}

func (i RemovablePathItem) FilterValue() string { return i.path }
func (i RemovablePathItem) Title() string       { return i.path }
func (i RemovablePathItem) Description() string {
	if i.libraryType == "movie" {
		return "Movie Library"
	}
	return "TV Library"
}

// NewRemovePathModel creates a removal selection menu
func NewRemovePathModel(cfg *config.Config) RemovePathModel {
	items := []list.Item{}

	// Add all movie paths
	for _, path := range cfg.Libraries.Movies.Paths {
		items = append(items, RemovablePathItem{
			path:        path,
			libraryType: "movie",
		})
	}

	// Add all TV paths
	for _, path := range cfg.Libraries.TV.Paths {
		items = append(items, RemovablePathItem{
			path:        path,
			libraryType: "tv",
		})
	}

	// Add Back option
	items = append(items, MenuItem{
		title: "Back",
		desc:  "Return to library menu",
	})

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMARed).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMAFireRed)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(RAMAForeground)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(RAMAMuted)

	l := list.New(items, delegate, 80, 20)
	l.Title = "Select Library Path to Remove"
	l.Styles.Title = TitleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.SetShowTitle(true)

	return RemovePathModel{
		list:   l,
		config: cfg,
	}
}

func (m RemovePathModel) Init() tea.Cmd {
	return nil
}

func (m RemovePathModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return NewLibraryMenuModel(m.config), nil

		case "enter":
			selected := m.list.SelectedItem()

			// Handle Back option
			if menuItem, ok := selected.(MenuItem); ok && menuItem.title == "Back" {
				return NewLibraryMenuModel(m.config), nil
			}

			// Handle path removal
			if pathItem, ok := selected.(RemovablePathItem); ok {
				// Remove from config
				if pathItem.libraryType == "movie" {
					newPaths := []string{}
					for _, p := range m.config.Libraries.Movies.Paths {
						if p != pathItem.path {
							newPaths = append(newPaths, p)
						}
					}
					m.config.Libraries.Movies.Paths = newPaths
				} else {
					newPaths := []string{}
					for _, p := range m.config.Libraries.TV.Paths {
						if p != pathItem.path {
							newPaths = append(newPaths, p)
						}
					}
					m.config.Libraries.TV.Paths = newPaths
				}

				// Save config
				if err := config.Save(m.config); err != nil {
					return NewLibraryMenuModel(m.config), tea.Printf("Failed to save: %v", err)
				}

				// Return to library menu with success message
				return NewLibraryMenuModel(m.config), tea.Printf("Removed %s library path: %s", pathItem.libraryType, pathItem.path)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// ASCII header (9) + spacing (2) + footer (3) + padding (2) = 16 lines
		listHeight := msg.Height - 16
		if listHeight < 8 {
			listHeight = 8
		}
		m.list.SetSize(msg.Width-4, listHeight)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m RemovePathModel) View() string {
	// Minimum dimensions check
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	// Show ASCII header
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	// Show warning
	content.WriteString(WarningStyle.Render("⚠ WARNING: Removing a path will not delete any files") + "\n\n")

	content.WriteString(m.list.View())
	content.WriteString("\n\n")

	// Footer help text
	footer := MutedStyle.Render("↑/↓: Navigate  •  Enter: Remove  •  Esc: Back  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	// Wrap in padding
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// checkDaemonStatus checks if jellysink timer/service is active
func checkDaemonStatus() (timerActive bool, serviceActive bool) {
	// Check if timer is active
	cmd := exec.Command("systemctl", "is-active", "jellysink.timer")
	output, err := cmd.CombinedOutput()
	timerActive = err == nil && strings.TrimSpace(string(output)) == "active"

	// Check if service is active
	cmd = exec.Command("systemctl", "is-active", "jellysink.service")
	output, err = cmd.CombinedOutput()
	serviceActive = err == nil && strings.TrimSpace(string(output)) == "active"

	return timerActive, serviceActive
}

// getDaemonStatusString returns a formatted status string for display
func getDaemonStatusString() string {
	timerActive, serviceActive := checkDaemonStatus()

	if timerActive && serviceActive {
		return "Running"
	} else if timerActive {
		return "Timer Active"
	} else if serviceActive {
		return "Service Active"
	} else {
		return "Stopped"
	}
}

// boolToStatus converts boolean status to readable string
func boolToStatus(active bool) string {
	if active {
		return "Active"
	}
	return "Inactive"
}

// ListLibrariesModel shows all library paths in a scrollable viewport
type ListLibrariesModel struct {
	viewport viewport.Model
	config   *config.Config
	width    int
	height   int
	ready    bool
}

// NewListLibrariesModel creates a new list libraries view with scrolling
func NewListLibrariesModel(cfg *config.Config) ListLibrariesModel {
	return ListLibrariesModel{
		config: cfg,
		ready:  false,
	}
}

func (m ListLibrariesModel) Init() tea.Cmd {
	return nil
}

func (m ListLibrariesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			// Transfer dimensions when returning to menu
			libModel := NewLibraryMenuModel(m.config)
			libModel.width = m.width
			libModel.height = m.height
			listHeight := m.height - 16
			if listHeight < 8 {
				listHeight = 8
			}
			libModel.list.SetSize(m.width-4, listHeight)
			return libModel, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Initialize viewport with content
			headerHeight := 15 // ASCII header + title + padding
			footerHeight := 4  // Help text + padding
			m.viewport = viewport.New(msg.Width-4, msg.Height-headerHeight-footerHeight)
			m.viewport.Style = lipgloss.NewStyle().
				Padding(0, 1)

			// Set content
			m.viewport.SetContent(m.buildLibraryList())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - 19
		}
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m ListLibrariesModel) buildLibraryList() string {
	var b strings.Builder

	b.WriteString(InfoStyle.Render("Movie Libraries:") + "\n\n")
	if len(m.config.Libraries.Movies.Paths) == 0 {
		b.WriteString("  " + FormatStatusInfo("No paths configured") + "\n")
	} else {
		for i, path := range m.config.Libraries.Movies.Paths {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, FormatStatusOK(path)))
		}
	}
	b.WriteString("\n")

	b.WriteString(InfoStyle.Render("TV Show Libraries:") + "\n\n")
	if len(m.config.Libraries.TV.Paths) == 0 {
		b.WriteString("  " + FormatStatusInfo("No paths configured") + "\n")
	} else {
		for i, path := range m.config.Libraries.TV.Paths {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, FormatStatusOK(path)))
		}
	}

	totalPaths := len(m.config.Libraries.Movies.Paths) + len(m.config.Libraries.TV.Paths)
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Total: %d configured path(s)", totalPaths)))

	return b.String()
}

func (m ListLibrariesModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Minimum dimensions check
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	// Show ASCII header
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	// Title
	content.WriteString(TitleStyle.Render("ALL LIBRARY PATHS") + "\n\n")

	// Scrollable viewport with all paths
	content.WriteString(m.viewport.View())
	content.WriteString("\n\n")

	// Scroll progress indicator
	scrollPercent := fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	scrollInfo := MutedStyle.Render(fmt.Sprintf("Scroll: %s", scrollPercent))
	content.WriteString(scrollInfo + "\n")

	// Footer help text
	footer := MutedStyle.Render("↑/↓/PgUp/PgDn: Scroll  •  Esc: Back  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	// Wrap in padding
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// ScanningModel shows a loading screen while scan is running
type ScanStats struct {
	FilesProcessed    int
	DuplicatesFound   int
	ComplianceIssues  int
	ErrorsEncountered int
}

type opState struct {
	Operation  string
	Stage      string
	Percentage float64
	Message    string
}

type ScanningModel struct {
	config     *config.Config
	width      int
	height     int
	ctx        context.Context
	cancel     context.CancelFunc
	progressCh chan scanner.ScanProgress

	// Full progress state - last received
	currentProgress scanner.ScanProgress

	// Per-operation stage/status
	opStates map[string]opState
	opOrder  []string

	// Viewport and logs
	viewport  viewport.Model
	logBuffer []string // Up to 1000 lines

	// Live statistics
	stats ScanStats

	// ETA and timing
	startTime time.Time
	eta       time.Duration

	// Keyboard state
	scrollOffset int
}

// NewScanningModel creates a new scanning screen
func NewScanningModel(cfg *config.Config) ScanningModel {
	ctx, cancel := context.WithCancel(context.Background())
	return ScanningModel{
		config:          cfg,
		ctx:             ctx,
		cancel:          cancel,
		progressCh:      make(chan scanner.ScanProgress, 100),
		currentProgress: scanner.ScanProgress{},
		opStates:        make(map[string]opState),
		opOrder:         []string{},
		viewport:        viewport.Model{},
		logBuffer:       []string{},
		stats:           ScanStats{},
		startTime:       time.Now(),
		eta:             0,
		scrollOffset:    0,
	}
}

// Init starts the scan
func (m ScanningModel) Init() tea.Cmd {
	return tea.Batch(m.runScan, m.waitForProgress)
}

// waitForProgress listens for progress updates
func (m ScanningModel) waitForProgress() tea.Msg {
	progress := <-m.progressCh
	return progress
}

func (m ScanningModel) renderLogs() string {
	return strings.Join(m.logBuffer, "\n")
}

// runScan executes the scan in background
func (m ScanningModel) runScan() tea.Msg {
	d := daemon.New(m.config)
	reportPath, err := d.RunScanWithProgress(m.ctx, m.progressCh)
	return scanStatusMsg{reportPath: reportPath, err: err}
}

// Update handles messages
func (m ScanningModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancel()
			close(m.progressCh)
			return m, tea.Quit
		case "up", "k":
			m.viewport, _ = m.viewport.Update(msg)
			return m, nil
		case "down", "j":
			m.viewport, _ = m.viewport.Update(msg)
			return m, nil
		case "pgup":
			m.viewport, _ = m.viewport.Update(msg)
			return m, nil
		case "pgdown":
			m.viewport, _ = m.viewport.Update(msg)
			return m, nil

		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Initialize or resize viewport for logs
		headerHeight := 10 // ASCII header + progress area
		footerHeight := 4  // Help/footer
		vpWidth := msg.Width - 4
		vpHeight := msg.Height - headerHeight - footerHeight
		if vpHeight < 4 {
			vpHeight = 4
		}
		if m.viewport.Width == 0 || m.viewport.Height == 0 {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.viewport.Style = lipgloss.NewStyle().Padding(0, 1)
			m.viewport.SetContent(m.renderLogs())
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}
		m.viewport.SetContent(m.renderLogs())
		return m, nil

	case scanner.ScanProgress:
		// Real progress update received
		m.currentProgress = msg

		// Update live stats
		m.stats.FilesProcessed = msg.FilesProcessed
		m.stats.DuplicatesFound = msg.DuplicatesFound
		m.stats.ComplianceIssues = msg.ComplianceIssues

		// Start time: prefer progress StartTime if provided
		if m.startTime.IsZero() && !msg.StartTime.IsZero() {
			m.startTime = msg.StartTime
		}

		// Calculate ETA
		if msg.Total > 0 && msg.Current > 0 {
			elapsed := time.Since(m.startTime)
			rate := float64(msg.Current) / elapsed.Seconds()
			remaining := msg.Total - msg.Current
			if rate > 0 {
				m.eta = time.Duration(float64(remaining)/rate) * time.Second
			}
		}

		// Add to log buffer (max 1000 lines)
		logEntry := fmt.Sprintf("[%02d:%02d] [%s] %s",
			msg.ElapsedSeconds/60,
			msg.ElapsedSeconds%60,
			msg.Operation,
			msg.Message)

		m.logBuffer = append(m.logBuffer, logEntry)
		if len(m.logBuffer) > 1000 {
			m.logBuffer = m.logBuffer[len(m.logBuffer)-1000:]
		}

		// Update viewport content and auto-scroll if available
		if m.viewport.Width > 0 && m.viewport.Height > 0 {
			m.viewport.SetContent(m.renderLogs())
			// Auto-scroll to bottom when new message arrives
			m.viewport.GotoBottom()
		}

		// Continue waiting for more progress
		return m, m.waitForProgress

	case scanStatusMsg:
		// Scan completed - switch to report view
		if msg.err != nil {
			close(m.progressCh)
			return m, tea.Printf("Scan failed: %v", msg.err)
		}

		// Load report and switch to report view
		close(m.progressCh)
		report, err := loadReportJSON(msg.reportPath)
		if err != nil {
			return m, tea.Printf("Failed to load report: %v", err)
		}

		// Create report model with dimensions
		reportModel := NewModel(report)
		return reportModel, func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		}
	}

	return m, nil
}

// View renders the scanning screen
func (m ScanningModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	var content strings.Builder

	// Show ASCII header
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	// Progress header
	progressHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(RAMARed).
		Align(lipgloss.Center).
		Width(m.width - 8)
	content.WriteString(progressHeaderStyle.Render("SCANNING LIBRARIES"))
	content.WriteString("\n\n")

	// Current phase
	if m.currentProgress.Message != "" {
		phaseStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorInfo).
			Align(lipgloss.Center).
			Width(m.width - 8)
		content.WriteString(phaseStyle.Render(m.currentProgress.Message))
		content.WriteString("\n\n")
	}

	// Progress bar (50 characters wide)
	barWidth := int(m.currentProgress.Percentage * 50 / 100)
	if barWidth > 50 {
		barWidth = 50
	}
	if barWidth < 0 {
		barWidth = 0
	}
	progressBar := strings.Repeat("█", barWidth) + strings.Repeat("░", 50-barWidth)
	progressBarStyle := lipgloss.NewStyle().
		Foreground(RAMARed).
		Align(lipgloss.Center).
		Width(m.width - 8)
	content.WriteString(progressBarStyle.Render(fmt.Sprintf("[%s] %.1f%%", progressBar, m.currentProgress.Percentage)))
	content.WriteString("\n\n")

	// Live statistics
	statsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(RAMARed).
		Padding(0, 2).
		Width(m.width - 10)
	statsContent := fmt.Sprintf(
		"Files Processed: %d  |  Duplicates: %d  |  Compliance Issues: %d  |  Errors: %d",
		m.stats.FilesProcessed,
		m.stats.DuplicatesFound,
		m.stats.ComplianceIssues,
		m.stats.ErrorsEncountered,
	)
	content.WriteString(statsStyle.Render(statsContent))
	content.WriteString("\n\n")

	// ETA display
	if m.eta > 0 {
		etaStyle := lipgloss.NewStyle().
			Foreground(ColorInfo).
			Align(lipgloss.Center).
			Width(m.width - 8)
		etaText := fmt.Sprintf("Estimated Time Remaining: %s", m.eta.Round(time.Second))
		content.WriteString(etaStyle.Render(etaText))
		content.WriteString("\n\n")
	}

	// Scrolling log viewport
	logTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorInfo)

	content.WriteString(logTitleStyle.Render("Activity Log:"))
	content.WriteString("\n")
	content.WriteString(m.viewport.View())
	content.WriteString("\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Align(lipgloss.Center).
		Width(m.width - 8)

	helpText := "↑/↓: Scroll logs  •  PgUp/PgDn: Page scroll  •  Ctrl+C: Cancel"
	content.WriteString(helpStyle.Render(helpText))

	// Wrap in main container
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width)

	return mainStyle.Render(content.String())
}

// APIConfigModel handles API key configuration
type APIConfigModel struct {
	list   list.Model
	config *config.Config
	width  int
	height int
}

// NewAPIConfigModel creates API key configuration menu
func NewAPIConfigModel(cfg *config.Config) APIConfigModel {
	items := []list.Item{
		MenuItem{title: "Configure TVDB API", desc: "Set TVDB API key for TV show metadata verification"},
		MenuItem{title: "Configure OMDB API", desc: "Set OMDB API key for movie metadata verification"},
		MenuItem{title: "View API Status", desc: "Check configured API keys and their status"},
		MenuItem{title: "Back", desc: "Return to main menu"},
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMARed).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(RAMABackground).
		Background(RAMAFireRed)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(RAMAForeground)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(RAMAMuted)

	l := list.New(items, delegate, 80, 20)
	l.Title = "API CONFIGURATION"
	l.Styles.Title = TitleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.SetShowTitle(true)

	return APIConfigModel{
		list:   l,
		config: cfg,
	}
}

func (m APIConfigModel) Init() tea.Cmd {
	return nil
}

func (m APIConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return NewMenuModel(m.config), nil

		case "enter":
			selected := m.list.SelectedItem().(MenuItem)
			switch selected.title {
			case "Back":
				return NewMenuModel(m.config), nil
			case "Configure TVDB API":
				tvdbModel := NewSetAPIKeyModel(m.config, "tvdb")
				tvdbModel.width = m.width
				tvdbModel.height = m.height
				return tvdbModel, tvdbModel.Init()
			case "Configure OMDB API":
				omdbModel := NewSetAPIKeyModel(m.config, "omdb")
				omdbModel.width = m.width
				omdbModel.height = m.height
				return omdbModel, omdbModel.Init()
			case "View API Status":
				statusModel := NewAPIStatusModel(m.config)
				statusModel.width = m.width
				statusModel.height = m.height
				return statusModel, nil
			default:
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listHeight := msg.Height - 16
		if listHeight < 8 {
			listHeight = 8
		}
		m.list.SetSize(msg.Width-4, listHeight)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m APIConfigModel) View() string {
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder
	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")
	content.WriteString(m.list.View())
	content.WriteString("\n\n")

	footer := MutedStyle.Render("↑/↓: Navigate  •  Enter: Select  •  Esc: Back  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// SetAPIKeyModel handles setting an API key
type SetAPIKeyModel struct {
	textInput textinput.Model
	config    *config.Config
	apiType   string // "tvdb" or "omdb"
	width     int
	height    int
	err       string
	success   string
}

// NewSetAPIKeyModel creates a new API key input model
func NewSetAPIKeyModel(cfg *config.Config, apiType string) SetAPIKeyModel {
	ti := textinput.New()
	ti.Placeholder = "Paste your API key here"
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	ti.PromptStyle = lipgloss.NewStyle().Foreground(RAMARed)
	ti.TextStyle = lipgloss.NewStyle().Foreground(RAMAForeground)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(RAMAMuted)

	return SetAPIKeyModel{
		textInput: ti,
		config:    cfg,
		apiType:   apiType,
	}
}

func (m SetAPIKeyModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SetAPIKeyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return NewAPIConfigModel(m.config), nil

		case "esc":
			return NewAPIConfigModel(m.config), nil

		case "ctrl+u":
			m.textInput.SetValue("")
			return m, nil

		case "ctrl+v":
			if m.textInput.EchoMode == textinput.EchoPassword {
				m.textInput.EchoMode = textinput.EchoNormal
			} else {
				m.textInput.EchoMode = textinput.EchoPassword
			}
			return m, nil

		case "enter":
			apiKey := strings.TrimSpace(m.textInput.Value())
			if apiKey == "" {
				m.err = "API key cannot be empty"
				m.success = ""
				return m, nil
			}

			if m.apiType == "tvdb" {
				m.config.API.TVDB.APIKey = apiKey
				m.config.API.TVDB.Enabled = true
			} else if m.apiType == "omdb" {
				m.config.API.OMDB.APIKey = apiKey
				m.config.API.OMDB.Enabled = true
			}

			if err := config.Save(m.config); err != nil {
				m.err = fmt.Sprintf("Failed to save config: %v", err)
				m.success = ""
				return m, nil
			}

			m.success = fmt.Sprintf("%s API key saved successfully", strings.ToUpper(m.apiType))
			m.err = ""
			m.textInput.SetValue("")
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m SetAPIKeyModel) View() string {
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	apiName := strings.ToUpper(m.apiType)
	title := fmt.Sprintf("CONFIGURE %s API KEY", apiName)
	content.WriteString(TitleStyle.Render(title) + "\n\n")

	content.WriteString(InfoStyle.Render(fmt.Sprintf("%s API Key Configuration", apiName)) + "\n\n")

	var guidance strings.Builder
	if m.apiType == "tvdb" {
		guidance.WriteString("TVDB (TheTVDB) provides TV show metadata for title verification.\n")
		guidance.WriteString("Get your free API key at: https://thetvdb.com/api-information\n\n")
		guidance.WriteString("Steps:\n")
		guidance.WriteString("  1. Create a free account at TheTVDB\n")
		guidance.WriteString("  2. Navigate to 'API Keys' in your dashboard\n")
		guidance.WriteString("  3. Generate a new API key (v4)\n")
		guidance.WriteString("  4. Copy and paste it below\n")
	} else {
		guidance.WriteString("OMDB (Open Movie Database) provides movie metadata for title verification.\n")
		guidance.WriteString("Get your free API key at: https://www.omdbapi.com/apikey.aspx\n\n")
		guidance.WriteString("Steps:\n")
		guidance.WriteString("  1. Submit your email for a FREE key\n")
		guidance.WriteString("  2. Check your email and activate the key\n")
		guidance.WriteString("  3. Copy and paste it below\n")
	}
	content.WriteString(MutedStyle.Render(guidance.String()))
	content.WriteString("\n")

	var currentKey string
	if m.apiType == "tvdb" {
		if m.config.API.TVDB.APIKey != "" {
			currentKey = "***" + m.config.API.TVDB.APIKey[len(m.config.API.TVDB.APIKey)-4:]
		}
	} else {
		if m.config.API.OMDB.APIKey != "" {
			currentKey = "***" + m.config.API.OMDB.APIKey[len(m.config.API.OMDB.APIKey)-4:]
		}
	}

	if currentKey != "" {
		content.WriteString(InfoStyle.Render("Current key: ") + MutedStyle.Render(currentKey) + "\n")
	}

	content.WriteString("Enter API Key: ")
	content.WriteString(m.textInput.View())
	content.WriteString("\n\n")

	if m.err != "" {
		content.WriteString(ErrorStyle.Render("✗ "+m.err) + "\n\n")
	}

	if m.success != "" {
		content.WriteString(SuccessStyle.Render("✓ "+m.success) + "\n\n")
	}

	content.WriteString(MutedStyle.Render("Ctrl+V: Toggle visibility  •  Ctrl+U: Clear  •  Enter: Save  •  Esc: Cancel"))

	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

// APIStatusModel shows API configuration status
type APIStatusModel struct {
	config *config.Config
	width  int
	height int
}

// NewAPIStatusModel creates API status view
func NewAPIStatusModel(cfg *config.Config) APIStatusModel {
	return APIStatusModel{
		config: cfg,
	}
}

func (m APIStatusModel) Init() tea.Cmd {
	return nil
}

func (m APIStatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return NewAPIConfigModel(m.config), nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m APIStatusModel) View() string {
	const minWidth = 100
	const minHeight = 25

	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize your terminal.",
			minWidth, minHeight, m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	content.WriteString(TitleStyle.Render("API CONFIGURATION STATUS") + "\n\n")

	content.WriteString(InfoStyle.Render("TVDB (TheTVDB) API:") + "\n")
	if m.config.API.TVDB.Enabled && m.config.API.TVDB.APIKey != "" {
		maskedKey := "***" + m.config.API.TVDB.APIKey[len(m.config.API.TVDB.APIKey)-4:]
		content.WriteString("  " + FormatStatusOK("Configured") + " - Key: " + MutedStyle.Render(maskedKey) + "\n")
		content.WriteString("  Used for: TV show title verification and metadata resolution\n")
	} else {
		content.WriteString("  " + FormatStatusInfo("Not configured") + "\n")
		content.WriteString("  Get your key at: https://thetvdb.com/api-information\n")
	}
	content.WriteString("\n")

	content.WriteString(InfoStyle.Render("OMDB (Open Movie Database) API:") + "\n")
	if m.config.API.OMDB.Enabled && m.config.API.OMDB.APIKey != "" {
		maskedKey := "***" + m.config.API.OMDB.APIKey[len(m.config.API.OMDB.APIKey)-4:]
		content.WriteString("  " + FormatStatusOK("Configured") + " - Key: " + MutedStyle.Render(maskedKey) + "\n")
		content.WriteString("  Used for: Movie metadata verification (optional)\n")
	} else {
		content.WriteString("  " + FormatStatusInfo("Not configured") + "\n")
		content.WriteString("  Get your key at: https://www.omdbapi.com/apikey.aspx\n")
	}
	content.WriteString("\n")

	var notes strings.Builder
	notes.WriteString("Note: API keys are optional but recommended for TV show title resolution.\n")
	notes.WriteString("Without API keys, jellysink will use local heuristics for title matching.\n\n")
	content.WriteString(MutedStyle.Render(notes.String()))

	content.WriteString(MutedStyle.Render("Press Esc to return"))

	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}
