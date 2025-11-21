package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
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
	list   list.Model
	config *config.Config
	width  int
	height int
	ctx    context.Context
	cancel context.CancelFunc
}

// NewMenuModel creates a new main menu
func NewMenuModel(cfg *config.Config) MenuModel {
	items := []list.Item{
		MenuItem{title: "Run Manual Scan", desc: "Scan your media libraries for duplicates and compliance issues"},
		MenuItem{title: "View Last Report", desc: "View the most recent scan report"},
		MenuItem{title: "Configure Frequency", desc: "Set automatic scan frequency (daily/weekly/biweekly)"},
		MenuItem{title: "Enable/Disable Daemon", desc: "Toggle automatic background scanning"},
		MenuItem{title: "Configure Libraries", desc: "Add or remove media library paths"},
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

	l := list.New(items, delegate, 0, 0)
	l.Title = "JELLYSINK MAIN MENU"
	l.Styles.Title = TitleStyle

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

		case "enter":
			selected := m.list.SelectedItem().(MenuItem)
			return m.handleSelection(selected.title)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Calculate list height after accounting for other content:
		// ASCII header (9 lines) + spacing (2) + config status (5) + spacing (1) + footer (3) + padding (2) = 22 lines
		listHeight := msg.Height - 22
		if listHeight < 6 {
			listHeight = 6 // Minimum list height
		}
		m.list.SetSize(msg.Width-4, listHeight)
		return m, nil

	case scanStatusMsg:
		// Scan completed
		if msg.err != nil {
			// Show error and return to menu
			return m, tea.Printf("Scan failed: %v", msg.err)
		}
		// Load report and switch to report view
		return m, tea.Printf("Scan complete! Report: %s", msg.reportPath)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleSelection processes menu selections
func (m MenuModel) handleSelection(title string) (tea.Model, tea.Cmd) {
	switch title {
	case "Run Manual Scan":
		return m, m.runScan

	case "View Last Report":
		return m, m.viewLastReport

	case "Configure Frequency":
		return NewFrequencyMenuModel(m.config), nil

	case "Enable/Disable Daemon":
		return NewDaemonMenuModel(m.config), nil

	case "Configure Libraries":
		return NewLibraryMenuModel(m.config), nil

	case "Exit":
		m.cancel()
		return m, tea.Quit
	}

	return m, nil
}

// runScan executes a manual scan
func (m MenuModel) runScan() tea.Msg {
	d := daemon.New(m.config)
	reportPath, err := d.RunScan(m.ctx)
	return scanStatusMsg{reportPath: reportPath, err: err}
}

// viewLastReport finds and displays the most recent report
func (m MenuModel) viewLastReport() tea.Msg {
	// TODO: Implement finding last report
	return nil
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

	// Show config status
	content.WriteString(InfoStyle.Render("Configuration Status:") + "\n")
	content.WriteString(fmt.Sprintf("  Movie libraries: %s\n", StatStyle.Render(fmt.Sprintf("%d", len(m.config.Libraries.Movies.Paths)))))
	content.WriteString(fmt.Sprintf("  TV libraries: %s\n", StatStyle.Render(fmt.Sprintf("%d", len(m.config.Libraries.TV.Paths)))))
	content.WriteString(fmt.Sprintf("  Scan frequency: %s\n", SuccessStyle.Render(m.config.Daemon.ScanFrequency)))

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
	content.WriteString(fmt.Sprintf("  Daemon status: %s\n", statusStyle.Render(daemonStatus)))
	content.WriteString("\n")

	// Add menu list
	content.WriteString(m.list.View())
	content.WriteString("\n\n")

	// Footer help text
	footer := MutedStyle.Render("↑/↓: Navigate  •  Enter: Select  •  Q/Ctrl+C: Quit")
	content.WriteString(footer)

	// Wrap in padding and border like installer
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4) // Account for padding

	return mainStyle.Render(content.String())
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

	l := list.New(items, delegate, 0, 0)
	l.Title = "SET SCAN FREQUENCY"
	l.Styles.Title = TitleStyle

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

	l := list.New(items, delegate, 0, 0)
	l.Title = "DAEMON MANAGEMENT"
	l.Styles.Title = TitleStyle

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
				return m, tea.Printf(statusMsg)
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

	l := list.New(items, delegate, 0, 0)
	l.Title = "LIBRARY CONFIGURATION"
	l.Styles.Title = TitleStyle

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
			return NewMenuModel(m.config), nil

		case "enter":
			selected := m.list.SelectedItem().(MenuItem)
			switch selected.title {
			case "Back":
				return NewMenuModel(m.config), nil
			case "Add Movie Library":
				return NewAddPathModel(m.config, "movie"), nil
			case "Add TV Library":
				return NewAddPathModel(m.config, "tv"), nil
			case "Remove Library":
				return NewRemovePathModel(m.config), nil
			case "List Libraries":
				return NewListLibrariesModel(m.config), nil
			default:
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// ASCII header (9) + title (1) + library preview (15) + footer (3) + padding (2) = 30 lines
		listHeight := msg.Height - 30
		if listHeight < 5 {
			listHeight = 5
		}
		m.list.SetSize(msg.Width-4, listHeight)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
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

	// Show current libraries (limited preview)
	content.WriteString(TitleStyle.Render("CURRENT LIBRARIES") + "\n\n")

	const maxPreview = 3 // Show max 3 paths per library type

	content.WriteString(InfoStyle.Render("Movies:") + "\n")
	if len(m.config.Libraries.Movies.Paths) == 0 {
		content.WriteString("  " + FormatStatusInfo("No paths configured") + "\n")
	} else {
		showCount := len(m.config.Libraries.Movies.Paths)
		if showCount > maxPreview {
			showCount = maxPreview
		}
		for i := 0; i < showCount; i++ {
			content.WriteString("  " + FormatStatusOK(m.config.Libraries.Movies.Paths[i]) + "\n")
		}
		if len(m.config.Libraries.Movies.Paths) > maxPreview {
			remaining := len(m.config.Libraries.Movies.Paths) - maxPreview
			content.WriteString("  " + MutedStyle.Render(fmt.Sprintf("...and %d more", remaining)) + "\n")
		}
	}
	content.WriteString("\n")

	content.WriteString(InfoStyle.Render("TV Shows:") + "\n")
	if len(m.config.Libraries.TV.Paths) == 0 {
		content.WriteString("  " + FormatStatusInfo("No paths configured") + "\n")
	} else {
		showCount := len(m.config.Libraries.TV.Paths)
		if showCount > maxPreview {
			showCount = maxPreview
		}
		for i := 0; i < showCount; i++ {
			content.WriteString("  " + FormatStatusOK(m.config.Libraries.TV.Paths[i]) + "\n")
		}
		if len(m.config.Libraries.TV.Paths) > maxPreview {
			remaining := len(m.config.Libraries.TV.Paths) - maxPreview
			content.WriteString("  " + MutedStyle.Render(fmt.Sprintf("...and %d more", remaining)) + "\n")
		}
	}

	if len(m.config.Libraries.Movies.Paths) > maxPreview || len(m.config.Libraries.TV.Paths) > maxPreview {
		content.WriteString("\n")
		content.WriteString(MutedStyle.Render("  Select 'List Libraries' to see all paths"))
	}
	content.WriteString("\n\n")

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
			return NewLibraryMenuModel(m.config), nil

		case "esc":
			// Cancel and return to library menu
			return NewLibraryMenuModel(m.config), nil

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

			// Show success and return to library menu
			return NewLibraryMenuModel(m.config), tea.Printf("Added %s library path: %s", m.libraryType, path)
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

	// Instructions
	content.WriteString(InfoStyle.Render("Enter the full path to your library folder:") + "\n\n")

	// Text input
	content.WriteString(m.textInput.View())
	content.WriteString("\n\n")

	// Error message
	if m.err != "" {
		content.WriteString(ErrorStyle.Render("✗ " + m.err) + "\n\n")
	}

	// Success message
	if m.success != "" {
		content.WriteString(SuccessStyle.Render("✓ " + m.success) + "\n\n")
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

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select Library Path to Remove"
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

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
			return NewLibraryMenuModel(m.config), nil
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
