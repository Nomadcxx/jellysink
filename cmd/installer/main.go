package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Theme colors - RAMA
var (
	BgBase       = lipgloss.Color("#2b2d42")  // RAMA Space cadet
	Primary      = lipgloss.Color("#ef233c")  // RAMA Red Pantone
	Secondary    = lipgloss.Color("#d90429")  // RAMA Fire engine red
	Accent       = lipgloss.Color("#edf2f4")  // RAMA Anti-flash white
	FgPrimary    = lipgloss.Color("#edf2f4")  // RAMA Anti-flash white
	FgSecondary  = lipgloss.Color("#8d99ae")  // RAMA Cool gray
	FgMuted      = lipgloss.Color("#8d99ae")  // RAMA Cool gray
	ErrorColor   = lipgloss.Color("#d90429")  // RAMA Fire engine red
	WarningColor = lipgloss.Color("#ef233c")  // RAMA Red Pantone
)

// Styles
var (
	checkMark   = lipgloss.NewStyle().Foreground(Accent).SetString("[OK]")
	failMark    = lipgloss.NewStyle().Foreground(ErrorColor).SetString("[FAIL]")
	skipMark    = lipgloss.NewStyle().Foreground(WarningColor).SetString("[SKIP]")
	headerStyle = lipgloss.NewStyle().Foreground(Primary).Bold(true)
)

type installStep int

const (
	stepWelcome installStep = iota
	stepConfigPrompt
	stepInstalling
	stepComplete
)

type taskStatus int

const (
	statusPending taskStatus = iota
	statusRunning
	statusComplete
	statusFailed
	statusSkipped
)

type installTask struct {
	name        string
	description string
	execute     func(*model) error
	optional    bool
	status      taskStatus
}

type model struct {
	step               installStep
	tasks              []installTask
	currentTaskIndex   int
	width              int
	height             int
	spinner            spinner.Model
	errors             []string
	uninstallMode      bool
	selectedOption     int  // 0 = Install, 1 = Uninstall
	configExists       bool // Whether config file already exists
	overrideConfig     bool // Whether to override existing config
	configPromptOption int  // 0 = Override, 1 = Keep existing
	binariesExist      bool // Whether binaries are already installed
}

type taskCompleteMsg struct {
	index   int
	success bool
	error   string
}

func newModel() model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(Secondary)
	s.Spinner = spinner.Dot

	// Check if binaries are already installed
	binariesExist := checkExistingBinaries()

	return model{
		step:             stepWelcome,
		currentTaskIndex: -1,
		spinner:          s,
		errors:           []string{},
		selectedOption:   0,
		binariesExist:    binariesExist,
	}
}

// checkExistingBinaries checks if jellysink binaries are already installed
func checkExistingBinaries() bool {
	paths := []string{"/usr/local/bin/jellysink", "/usr/local/bin/jellysinkd"}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return false // If any binary is missing, not fully installed
		}
	}
	return true
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Allow exit from any step except during installation
			if m.step != stepInstalling {
				return m, tea.Quit
			}
		case "up", "k":
			if m.step == stepWelcome && m.selectedOption > 0 {
				m.selectedOption--
			}
			if m.step == stepConfigPrompt && m.configPromptOption > 0 {
				m.configPromptOption--
			}
		case "down", "j":
			if m.step == stepWelcome && m.selectedOption < 1 {
				m.selectedOption++
			}
			if m.step == stepConfigPrompt && m.configPromptOption < 1 {
				m.configPromptOption++
			}
		case "enter":
			if m.step == stepWelcome {
				m.uninstallMode = m.selectedOption == 1

				// Check if config exists (only for install mode)
				if !m.uninstallMode {
					homeDir, err := os.UserHomeDir()
					if err == nil {
						configPath := filepath.Join(homeDir, ".config", "jellysink", "config.toml")
						if _, err := os.Stat(configPath); err == nil {
							m.configExists = true
							m.step = stepConfigPrompt
							m.configPromptOption = 1 // Default to "Keep existing"
							return m, nil
						}
					}
				}

				// No config exists or uninstall mode - proceed directly
				m.initTasks()
				m.step = stepInstalling
				m.currentTaskIndex = 0
				m.tasks[0].status = statusRunning
				return m, tea.Batch(
					m.spinner.Tick,
					executeTask(0, &m),
				)
			} else if m.step == stepConfigPrompt {
				// User has chosen whether to override config
				m.overrideConfig = m.configPromptOption == 0
				m.initTasks()
				m.step = stepInstalling
				m.currentTaskIndex = 0
				m.tasks[0].status = statusRunning
				return m, tea.Batch(
					m.spinner.Tick,
					executeTask(0, &m),
				)
			} else if m.step == stepComplete {
				return m, tea.Quit
			}
		}

	case taskCompleteMsg:
		// Update task status
		if msg.success {
			m.tasks[msg.index].status = statusComplete
		} else {
			if m.tasks[msg.index].optional {
				m.tasks[msg.index].status = statusSkipped
				m.errors = append(m.errors, fmt.Sprintf("%s (skipped): %s", m.tasks[msg.index].name, msg.error))
			} else {
				m.tasks[msg.index].status = statusFailed
				m.errors = append(m.errors, fmt.Sprintf("%s: %s", m.tasks[msg.index].name, msg.error))
				m.step = stepComplete
				return m, nil
			}
		}

		// Move to next task
		m.currentTaskIndex++
		if m.currentTaskIndex >= len(m.tasks) {
			m.step = stepComplete
			return m, nil
		}

		// Start next task
		m.tasks[m.currentTaskIndex].status = statusRunning
		return m, executeTask(m.currentTaskIndex, &m)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *model) initTasks() {
	if m.uninstallMode {
		m.tasks = []installTask{
			{name: "Check privileges", description: "Checking root access", execute: checkPrivileges, status: statusPending},
			{name: "Stop services", description: "Stopping jellysink services", execute: stopServices, status: statusPending, optional: true},
			{name: "Remove binaries", description: "Removing /usr/local/bin/jellysink*", execute: removeBinaries, status: statusPending},
			{name: "Remove systemd files", description: "Removing systemd service and timer", execute: removeSystemdFiles, status: statusPending},
		}
	} else {
		m.tasks = []installTask{
			{name: "Check privileges", description: "Checking root access", execute: checkPrivileges, status: statusPending},
			{name: "Build binaries", description: "Building jellysink and jellysinkd", execute: buildBinaries, status: statusPending},
			{name: "Install binaries", description: "Installing to /usr/local/bin", execute: installBinaries, status: statusPending},
			{name: "Create config", description: "Creating configuration directory", execute: createConfig, status: statusPending},
			{name: "Install systemd files", description: "Installing service and timer", execute: installSystemdFiles, status: statusPending},
		}
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content strings.Builder

	// ASCII Header from /home/nomadx/bit/JELLYSINK.txt
	jellysinkASCII := `  ████              ████   ████                            ████                ████
                    ████   ████                                                ████
  ████  ██████████  ████   ████  ████  ████  ██████████  ██████    ██████████  ████  ████
  ████  ████  ████  ████   ████  ████  ████  ██████      ██████    ██████████  ██████████
  ████  ██████████  ████   ████  ████  ████  ██████████    ████    ████  ████  ████████
  ████  ████        ████   ████  ██████████      ██████  ████████  ████  ████  ██████████
  ████  ██████████  ████   ████  ██████████  ██████████  ████████  ████  ████  ████  ████
██████                                 ████
██████                           ██████████                                              `

	content.WriteString(headerStyle.Render(jellysinkASCII))
	content.WriteString("\n\n")

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true).
		Align(lipgloss.Center)
	title := "jellysink installer"
	if m.uninstallMode {
		title = "jellysink uninstaller"
	}
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// Main content based on step
	var mainContent string
	switch m.step {
	case stepWelcome:
		mainContent = m.renderWelcome()
	case stepConfigPrompt:
		mainContent = m.renderConfigPrompt()
	case stepInstalling:
		mainContent = m.renderInstalling()
	case stepComplete:
		mainContent = m.renderComplete()
	}

	// Wrap in border
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Width(m.width - 4)
	content.WriteString(mainStyle.Render(mainContent))
	content.WriteString("\n")

	// Help text
	helpText := m.getHelpText()
	if helpText != "" {
		helpStyle := lipgloss.NewStyle().
			Foreground(FgMuted).
			Italic(true).
			Align(lipgloss.Center)
		content.WriteString("\n" + helpStyle.Render(helpText))
	}

	// Wrap everything in background with centering
	bgStyle := lipgloss.NewStyle().
		Background(BgBase).
		Foreground(FgPrimary).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Top)

	return bgStyle.Render(content.String())
}

func (m model) renderWelcome() string {
	var b strings.Builder

	// Show installation status if binaries exist
	if m.binariesExist {
		statusStyle := lipgloss.NewStyle().Foreground(Accent).Bold(true)
		b.WriteString(statusStyle.Render("✓ jellysink is already installed"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  Binaries found in /usr/local/bin"))
		b.WriteString("\n\n")
	}

	b.WriteString("Select an option:\n\n")

	// Install option
	installPrefix := "  "
	if m.selectedOption == 0 {
		installPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	b.WriteString(installPrefix + "Install jellysink\n")
	b.WriteString("    Builds binaries and installs system-wide\n\n")

	// Uninstall option
	uninstallPrefix := "  "
	if m.selectedOption == 1 {
		uninstallPrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	b.WriteString(uninstallPrefix + "Uninstall jellysink\n")
	b.WriteString("    Removes jellysink from your system\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("Requires root privileges"))

	return b.String()
}

func (m model) renderConfigPrompt() string {
	var b strings.Builder

	warningStyle := lipgloss.NewStyle().Foreground(WarningColor).Bold(true)
	b.WriteString(warningStyle.Render("⚠ Existing Configuration Detected"))
	b.WriteString("\n\n")
	b.WriteString("An existing jellysink configuration file was found at:\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("~/.config/jellysink/config.toml"))
	b.WriteString("\n\n")
	b.WriteString("What would you like to do?\n\n")

	// Override option
	overridePrefix := "  "
	if m.configPromptOption == 0 {
		overridePrefix = lipgloss.NewStyle().Foreground(Primary).Render("▸ ")
	}
	b.WriteString(overridePrefix + "Override with new default configuration\n")
	b.WriteString("    Your current config will be backed up to config.toml.backup\n\n")

	// Keep existing option
	keepPrefix := "  "
	if m.configPromptOption == 1 {
		keepPrefix = lipgloss.NewStyle().Foreground(Accent).Render("▸ ")
	}
	b.WriteString(keepPrefix + "Keep existing configuration\n")
	b.WriteString("    Your current settings will be preserved\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("Note: Binaries will be updated either way"))

	return b.String()
}

func (m model) renderInstalling() string {
	var b strings.Builder

	// Render all tasks with their current status
	for i, task := range m.tasks {
		var line string
		switch task.status {
		case statusPending:
			line = lipgloss.NewStyle().Foreground(FgMuted).Render("  " + task.name)
		case statusRunning:
			line = m.spinner.View() + " " + lipgloss.NewStyle().Foreground(Secondary).Render(task.description)
		case statusComplete:
			line = checkMark.String() + " " + task.name
		case statusFailed:
			line = failMark.String() + " " + task.name
		case statusSkipped:
			line = skipMark.String() + " " + task.name
		}

		b.WriteString(line)
		if i < len(m.tasks)-1 {
			b.WriteString("\n")
		}
	}

	// Show errors at bottom if any
	if len(m.errors) > 0 {
		b.WriteString("\n\n")
		for _, err := range m.errors {
			b.WriteString(lipgloss.NewStyle().Foreground(WarningColor).Render(err))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m model) renderComplete() string {
	var b strings.Builder

	hasCriticalFailure := false
	for _, task := range m.tasks {
		if task.status == statusFailed && !task.optional {
			hasCriticalFailure = true
			break
		}
	}

	if hasCriticalFailure {
		failMsg := "Installation failed"
		if m.uninstallMode {
			failMsg = "Uninstallation failed"
		}
		b.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Bold(true).Render(failMsg))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(FgSecondary).Render("Check errors above"))
	} else {
		if m.uninstallMode {
			b.WriteString(lipgloss.NewStyle().Foreground(Accent).Bold(true).Render("✓ Uninstallation complete!"))
			b.WriteString("\n\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgSecondary).Render("jellysink has been removed from your system"))
			b.WriteString("\n\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("Configuration preserved at ~/.config/jellysink/"))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(Accent).Bold(true).Render("✓ Installation complete!"))
			b.WriteString("\n\n")

			// Next steps
			b.WriteString(lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Get Started:"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(Accent).Render("  jellysink"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgSecondary).Render("  ↳ Interactive TUI to configure libraries, set scan frequency,"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgSecondary).Render("    enable daemon, and run scans/cleans"))
			b.WriteString("\n\n")

			b.WriteString(lipgloss.NewStyle().Foreground(Primary).Bold(true).Render("Command Line Options (optional):"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  jellysink scan              - Run manual scan"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  jellysink view <report>     - View scan report"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  jellysink clean <report>    - Clean from report"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  jellysink version           - Show version info"))
		}
	}

	b.WriteString("\n\nPress Enter to exit")

	return b.String()
}

func (m model) getHelpText() string {
	switch m.step {
	case stepWelcome:
		return "↑/↓: Navigate  •  Enter: Continue  •  Q/Ctrl+C: Quit"
	case stepConfigPrompt:
		return "↑/↓: Navigate  •  Enter: Continue  •  Q/Ctrl+C: Quit"
	case stepComplete:
		return "Enter: Exit  •  Q/Ctrl+C: Quit"
	default:
		return "Installation in progress..."
	}
}

func executeTask(index int, m *model) tea.Cmd {
	return func() tea.Msg {
		// Simulate work delay for visibility
		time.Sleep(200 * time.Millisecond)

		err := m.tasks[index].execute(m)

		if err != nil {
			return taskCompleteMsg{
				index:   index,
				success: false,
				error:   err.Error(),
			}
		}

		return taskCompleteMsg{
			index:   index,
			success: true,
		}
	}
}

// Task execution functions

func checkPrivileges(m *model) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("installer must be run with sudo or as root")
	}
	return nil
}

func buildBinaries(m *model) error {
	// Build main binary
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", "jellysink", "./cmd/jellysink/")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build jellysink: %s", string(output))
	}

	// Build daemon
	cmd = exec.Command("go", "build", "-buildvcs=false", "-o", "jellysinkd", "./cmd/jellysinkd/")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build jellysinkd: %s", string(output))
	}

	return nil
}

func installBinaries(m *model) error {
	binaries := []string{"jellysink", "jellysinkd"}
	for _, binary := range binaries {
		cmd := exec.Command("install", "-Dm755", binary, fmt.Sprintf("/usr/local/bin/%s", binary))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install %s: %v", binary, err)
		}
	}
	return nil
}

func createConfig(m *model) error {
	// Get actual user's home directory
	homeDir := os.Getenv("HOME")
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser != "" {
		homeDir = "/home/" + sudoUser
	}

	configDir := filepath.Join(homeDir, ".config", "jellysink")
	configPath := filepath.Join(configDir, "config.toml")

	// Create directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Check if config exists
	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	}

	// If exists and keeping, skip
	if configExists && !m.overrideConfig {
		return nil
	}

	// If exists and overriding, backup
	if configExists && m.overrideConfig {
		backupPath := configPath + ".backup"
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read existing config: %v", err)
		}
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
	}

	// Default config
	defaultConfig := `[libraries.movies]
paths = ["/path/to/your/movies"]

[libraries.tv]
paths = ["/path/to/your/tvshows"]

[daemon]
scan_frequency = "weekly"  # daily, weekly, biweekly
`

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	// Set ownership if running with sudo
	if sudoUser != "" {
		cmd := exec.Command("id", "-u", sudoUser)
		output, err := cmd.Output()
		if err == nil {
			if uid, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
				cmd = exec.Command("id", "-g", sudoUser)
				gidOutput, err := cmd.Output()
				if err == nil {
					if gid, err := strconv.Atoi(strings.TrimSpace(string(gidOutput))); err == nil {
						os.Chown(configDir, uid, gid)
						os.Chown(configPath, uid, gid)
					}
				}
			}
		}
	}

	return nil
}

func installSystemdFiles(m *model) error {
	files := []string{"jellysink.service", "jellysink.timer"}
	for _, file := range files {
		srcPath := filepath.Join("systemd", file)
		dstPath := filepath.Join("/etc/systemd/system", file)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", file, err)
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("failed to install %s: %v", file, err)
		}
	}

	// Reload systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %v", err)
	}

	return nil
}

func stopServices(m *model) error {
	// Stop timer and service if running
	exec.Command("systemctl", "stop", "jellysink.timer").Run()
	exec.Command("systemctl", "stop", "jellysink.service").Run()
	time.Sleep(500 * time.Millisecond)
	return nil
}

func removeBinaries(m *model) error {
	binaries := []string{"jellysink", "jellysinkd"}
	for _, binary := range binaries {
		path := filepath.Join("/usr/local/bin", binary)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %v", binary, err)
		}
	}
	return nil
}

func removeSystemdFiles(m *model) error {
	// Stop and disable first
	exec.Command("systemctl", "stop", "jellysink.timer").Run()
	exec.Command("systemctl", "disable", "jellysink.timer").Run()

	files := []string{"jellysink.service", "jellysink.timer"}
	for _, file := range files {
		path := filepath.Join("/etc/systemd/system", file)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %v", file, err)
		}
	}

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	return nil
}

func main() {
	// Check for Go
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Println("Error: Go is not installed or not in PATH")
		fmt.Println("Please install Go from https://golang.org/dl/")
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
