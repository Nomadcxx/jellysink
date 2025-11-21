package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
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
		m.list.SetSize(msg.Width, msg.Height-2)
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
	// Show ASCII header
	header := FormatASCIIHeader() + "\n\n"

	// Show config status
	var status strings.Builder
	status.WriteString(InfoStyle.Render("Configuration Status:") + "\n")
	status.WriteString(fmt.Sprintf("  Movie libraries: %s\n", StatStyle.Render(fmt.Sprintf("%d", len(m.config.Libraries.Movies.Paths)))))
	status.WriteString(fmt.Sprintf("  TV libraries: %s\n", StatStyle.Render(fmt.Sprintf("%d", len(m.config.Libraries.TV.Paths)))))
	status.WriteString(fmt.Sprintf("  Scan frequency: %s\n", SuccessStyle.Render(m.config.Daemon.ScanFrequency)))
	status.WriteString("\n")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		status.String(),
		m.list.View(),
	)
}

// FrequencyMenuModel handles scan frequency configuration
type FrequencyMenuModel struct {
	list   list.Model
	config *config.Config
}

// NewFrequencyMenuModel creates frequency selection menu
func NewFrequencyMenuModel(cfg *config.Config) FrequencyMenuModel {
	items := []list.Item{
		MenuItem{title: "Daily", desc: "Scan every day at 2:00 AM"},
		MenuItem{title: "Weekly", desc: "Scan every Sunday at 2:00 AM"},
		MenuItem{title: "Biweekly", desc: "Scan every other Sunday at 2:00 AM"},
		MenuItem{title: "Back", desc: "Return to main menu"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
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
		m.list.SetSize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m FrequencyMenuModel) View() string {
	return m.list.View()
}

// DaemonMenuModel handles daemon enable/disable
type DaemonMenuModel struct {
	list   list.Model
	config *config.Config
}

// NewDaemonMenuModel creates daemon toggle menu
func NewDaemonMenuModel(cfg *config.Config) DaemonMenuModel {
	items := []list.Item{
		MenuItem{title: "Enable Daemon", desc: "Enable automatic background scanning"},
		MenuItem{title: "Disable Daemon", desc: "Disable automatic background scanning"},
		MenuItem{title: "Daemon Status", desc: "Check if daemon is running"},
		MenuItem{title: "Back", desc: "Return to main menu"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
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
			if selected.title == "Back" {
				return NewMenuModel(m.config), nil
			}
			// TODO: Implement systemctl enable/disable
			return NewMenuModel(m.config), tea.Printf("%s (systemctl integration pending)", selected.title)
		}

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m DaemonMenuModel) View() string {
	return m.list.View()
}

// LibraryMenuModel handles library path configuration
type LibraryMenuModel struct {
	list   list.Model
	config *config.Config
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

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
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
			if selected.title == "Back" {
				return NewMenuModel(m.config), nil
			}
			// TODO: Implement library path input
			return NewMenuModel(m.config), tea.Printf("%s (text input pending)", selected.title)
		}

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m LibraryMenuModel) View() string {
	var sb strings.Builder

	// Show current libraries
	sb.WriteString(TitleStyle.Render("CURRENT LIBRARIES") + "\n\n")

	sb.WriteString(InfoStyle.Render("Movies:") + "\n")
	if len(m.config.Libraries.Movies.Paths) == 0 {
		sb.WriteString(MutedStyle.Render("  (none configured)") + "\n")
	} else {
		for _, path := range m.config.Libraries.Movies.Paths {
			sb.WriteString(fmt.Sprintf("  %s %s\n", SuccessStyle.Render("•"), ContentStyle.Render(path)))
		}
	}
	sb.WriteString("\n")

	sb.WriteString(InfoStyle.Render("TV Shows:") + "\n")
	if len(m.config.Libraries.TV.Paths) == 0 {
		sb.WriteString(MutedStyle.Render("  (none configured)") + "\n")
	} else {
		for _, path := range m.config.Libraries.TV.Paths {
			sb.WriteString(fmt.Sprintf("  %s %s\n", SuccessStyle.Render("•"), ContentStyle.Render(path)))
		}
	}
	sb.WriteString("\n\n")

	sb.WriteString(m.list.View())

	return sb.String()
}
