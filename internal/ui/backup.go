package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

type BackupMenuModel struct {
	config     *config.Config
	list       list.Model
	width      int
	height     int
	mode       string
	backups    []*scanner.BackupSnapshot
	creating   bool
	verifying  bool
	reverting  bool
	error      string
	progressCh chan scanner.ScanProgress
	progress   []scanner.ScanProgress
}

func NewBackupMenuModel(cfg *config.Config) BackupMenuModel {
	items := []list.Item{
		MenuItem{title: "Create New Backup", desc: "Backup current library structure before making changes"},
		MenuItem{title: "View Existing Backups", desc: "List all saved backups and their details"},
		MenuItem{title: "Verify Backup Integrity", desc: "Check if backed up files still exist"},
		MenuItem{title: "Revert to Backup", desc: "Restore library to a previous backup state"},
		MenuItem{title: "Delete Old Backups", desc: "Remove outdated backup files"},
		MenuItem{title: "Back to Main Menu", desc: "Return to main menu"},
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
	l.Title = "BACKUP MANAGEMENT"
	l.Styles.Title = TitleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	return BackupMenuModel{
		config: cfg,
		list:   l,
		mode:   "menu",
	}
}

func (m BackupMenuModel) Init() tea.Cmd {
	return m.loadBackups
}

func (m BackupMenuModel) loadBackups() tea.Msg {
	backups, err := scanner.ListBackups()
	if err != nil {
		return backupListMsg{err: err}
	}
	return backupListMsg{backups: backups}
}

type backupListMsg struct {
	backups []*scanner.BackupSnapshot
	err     error
}

type backupCreatedMsg struct {
	snapshot *scanner.BackupSnapshot
	err      error
}

type backupRevertedMsg struct {
	err error
}

type backupVerifiedMsg struct {
	intact  bool
	missing []string
}

func (m BackupMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.creating || m.verifying || m.reverting {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.progressCh != nil {
				close(m.progressCh)
			}
			return m, tea.Quit

		case "esc":
			if m.mode == "list" || m.mode == "error" {
				m.mode = "menu"
				m.error = ""
				return m, nil
			}
			return NewMenuModel(m.config), nil

		case "enter":
			if m.mode == "menu" {
				selected := m.list.SelectedItem().(MenuItem)
				return m.handleSelection(selected.title)
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
		return m, nil

	case backupListMsg:
		if msg.err != nil {
			m.error = fmt.Sprintf("Failed to load backups: %v", msg.err)
			m.mode = "error"
		} else {
			m.backups = msg.backups
			m.mode = "list"
		}
		return m, nil

	case backupCreatedMsg:
		m.creating = false
		if msg.err != nil {
			m.error = fmt.Sprintf("Backup failed: %v", msg.err)
			m.mode = "error"
		} else {
			m.error = ""
			return m, m.loadBackups
		}
		return m, nil

	case backupRevertedMsg:
		m.reverting = false
		if msg.err != nil {
			m.error = fmt.Sprintf("Revert failed: %v", msg.err)
			m.mode = "error"
		} else {
			m.error = ""
			m.mode = "menu"
		}
		return m, nil

	case backupVerifiedMsg:
		m.verifying = false
		if msg.intact {
			m.error = "✓ Backup integrity verified - all files intact"
		} else {
			m.error = fmt.Sprintf("⚠ %d files missing or modified", len(msg.missing))
		}
		m.mode = "error"
		return m, nil

	case scanner.ScanProgress:
		m.progress = append(m.progress, msg)
		return m, nil
	}

	if m.mode == "menu" {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m BackupMenuModel) handleSelection(title string) (tea.Model, tea.Cmd) {
	switch title {
	case "Create New Backup":
		return m, m.createBackup

	case "View Existing Backups":
		if len(m.backups) == 0 {
			m.error = "No backups found"
			m.mode = "error"
			return m, nil
		}
		m.mode = "list"
		return m, nil

	case "Verify Backup Integrity":
		if len(m.backups) == 0 {
			m.error = "No backups to verify"
			m.mode = "error"
			return m, nil
		}
		return m, m.verifyLatestBackup

	case "Revert to Backup":
		if len(m.backups) == 0 {
			m.error = "No backups to revert to"
			m.mode = "error"
			return m, nil
		}
		return m, m.revertToLatestBackup

	case "Delete Old Backups":
		m.error = "Feature coming soon"
		m.mode = "error"
		return m, nil

	case "Back to Main Menu":
		return NewMenuModel(m.config), nil
	}

	return m, nil
}

func (m BackupMenuModel) createBackup() tea.Msg {
	m.creating = true

	var paths []string
	for _, path := range m.config.Libraries.Movies.Paths {
		paths = append(paths, path)
	}
	for _, path := range m.config.Libraries.TV.Paths {
		paths = append(paths, path)
	}

	snapshot, err := scanner.CreateBackup("all_libraries", paths, nil)
	return backupCreatedMsg{snapshot: snapshot, err: err}
}

func (m BackupMenuModel) verifyLatestBackup() tea.Msg {
	if len(m.backups) == 0 {
		return backupVerifiedMsg{intact: false, missing: []string{"No backups available"}}
	}

	m.verifying = true
	latest := m.backups[0]

	intact, missing := latest.VerifyIntegrity(nil)
	return backupVerifiedMsg{intact: intact, missing: missing}
}

func (m BackupMenuModel) revertToLatestBackup() tea.Msg {
	if len(m.backups) == 0 {
		return backupRevertedMsg{err: fmt.Errorf("no backups available")}
	}

	m.reverting = true
	latest := m.backups[0]
	err := scanner.RevertBackup(latest.Metadata.BackupID, nil)
	return backupRevertedMsg{err: err}
}

func (m BackupMenuModel) View() string {
	if m.width > 0 && m.height > 0 && (m.width < 100 || m.height < 25) {
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center, lipgloss.Center).
			Width(m.width).
			Height(m.height)

		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: 100x25\nCurrent: %dx%d\n\nPlease resize your terminal.",
			m.width, m.height,
		)
		return warningStyle.Render(warning)
	}

	var content strings.Builder

	content.WriteString(FormatASCIIHeader())
	content.WriteString("\n\n")

	switch m.mode {
	case "menu":
		content.WriteString(m.list.View())
		content.WriteString("\n\n")
		content.WriteString(MutedStyle.Render("Press Enter to select • Esc to go back • q to quit"))

	case "list":
		content.WriteString(TitleStyle.Render("EXISTING BACKUPS") + "\n\n")

		if len(m.backups) == 0 {
			content.WriteString(MutedStyle.Render("No backups found.") + "\n")
		} else {
			for i, backup := range m.backups {
				status := ""
				if backup.Metadata.Status == "completed" {
					status = FormatStatusOK("✓ Complete")
				} else if backup.Metadata.Status == "reverted" {
					status = FormatStatusInfo("↺ Reverted")
				} else {
					status = FormatStatusWarn("⚠ " + backup.Metadata.Status)
				}

				sizeGB := float64(backup.Metadata.TotalSize) / (1024 * 1024 * 1024)
				age := time.Since(backup.Metadata.CreatedAt)
				ageStr := formatDuration(age)

				content.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, status, InfoStyle.Render(backup.Metadata.BackupID)))
				content.WriteString(fmt.Sprintf("   Created: %s ago • Files: %d • Size: %.2f GB\n",
					ageStr, backup.Metadata.TotalFiles, sizeGB))
				content.WriteString(fmt.Sprintf("   Type: %s • Libraries: %d paths\n\n",
					backup.Metadata.LibraryType, len(backup.Metadata.LibraryPaths)))
			}
		}

		content.WriteString("\n")
		content.WriteString(MutedStyle.Render("Press Esc to go back"))

	case "error":
		content.WriteString(TitleStyle.Render("BACKUP STATUS") + "\n\n")
		if strings.HasPrefix(m.error, "✓") {
			content.WriteString(FormatStatusOK(m.error) + "\n")
		} else {
			content.WriteString(FormatStatusWarn(m.error) + "\n")
		}
		content.WriteString("\n")
		content.WriteString(MutedStyle.Render("Press Esc to go back"))
	}

	if m.creating {
		content.WriteString("\n\n")
		content.WriteString(FormatStatusInfo("Creating backup..."))
		if len(m.progress) > 0 {
			latest := m.progress[len(m.progress)-1]
			content.WriteString(fmt.Sprintf("\n%s", latest.Message))
		}
	}

	if m.verifying {
		content.WriteString("\n\n")
		content.WriteString(FormatStatusInfo("Verifying backup integrity..."))
	}

	if m.reverting {
		content.WriteString("\n\n")
		content.WriteString(FormatStatusWarn("Reverting to backup..."))
		if len(m.progress) > 0 {
			latest := m.progress[len(m.progress)-1]
			content.WriteString(fmt.Sprintf("\n%s", latest.Message))
		}
	}

	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width - 4)

	return mainStyle.Render(content.String())
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		mins := int(d.Minutes())
		return fmt.Sprintf("%d min", mins)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		return fmt.Sprintf("%d hr", hours)
	} else {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days", days)
	}
}
