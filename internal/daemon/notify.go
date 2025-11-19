package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NotifyUser sends a desktop notification about the scan completion
func NotifyUser(reportPath string, totalDuplicates int, spaceToFree int64) error {
	// Format message
	message := fmt.Sprintf("Found %d duplicate groups (%.2f GB to free)",
		totalDuplicates, float64(spaceToFree)/(1024*1024*1024))

	// Try notify-send first (most Linux desktops)
	if err := notifySend("jellysink - Scan Complete", message); err == nil {
		return nil
	}

	// Fallback to terminal bell
	fmt.Print("\a")
	fmt.Printf("\n[jellysink] Scan complete: %s\n", message)

	return nil
}

// notifySend sends notification using notify-send
func notifySend(title, message string) error {
	cmd := exec.Command("notify-send",
		"-u", "normal",
		"-i", "dialog-information",
		"-a", "jellysink",
		title,
		message)

	return cmd.Run()
}

// LaunchTUI opens the TUI to review the report
func LaunchTUI(reportPath string) error {
	// Get jellysink binary path
	binaryPath, err := exec.LookPath("jellysink")
	if err != nil {
		// Try local build
		wd, _ := os.Getwd()
		binaryPath = filepath.Join(wd, "jellysink")
		if _, err := os.Stat(binaryPath); err != nil {
			return fmt.Errorf("jellysink binary not found: %w", err)
		}
	}

	// Get current terminal
	terminal := os.Getenv("TERM")
	if terminal == "" {
		terminal = "xterm"
	}

	// Try to detect terminal emulator
	terminalCmd := detectTerminal()
	if terminalCmd == "" {
		return fmt.Errorf("no suitable terminal emulator found")
	}

	// Launch TUI in terminal
	cmd := exec.Command(terminalCmd, "-e", binaryPath, reportPath)
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch TUI: %w", err)
	}

	return nil
}

// detectTerminal tries to find a suitable terminal emulator
func detectTerminal() string {
	terminals := []string{
		"kitty",
		"alacritty",
		"wezterm",
		"gnome-terminal",
		"konsole",
		"xfce4-terminal",
		"xterm",
	}

	for _, term := range terminals {
		if _, err := exec.LookPath(term); err == nil {
			return term
		}
	}

	return ""
}
