package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NotifyUser launches kitty with the scan report (this IS the notification)
func NotifyUser(reportPath string) error {
	return LaunchTUI(reportPath)
}

// LaunchTUI opens kitty terminal with the TUI to review the report
func LaunchTUI(reportPath string) error {
	// Check if kitty is available
	kittyPath, err := exec.LookPath("kitty")
	if err != nil {
		return fmt.Errorf("kitty terminal not found (required for jellysink daemon): %w", err)
	}

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

	// Launch TUI in kitty
	// Use kitty's proper syntax: kitty [options] [program-to-run [program-args]]
	// jellysink uses "view <report-file>" command to display reports
	cmd := exec.Command(kittyPath, "--hold", binaryPath, "view", reportPath)

	// Explicitly pass display environment variables (critical for GUI from systemd)
	env := os.Environ()
	if display := os.Getenv("DISPLAY"); display != "" {
		env = append(env, fmt.Sprintf("DISPLAY=%s", display))
	}
	if waylandDisplay := os.Getenv("WAYLAND_DISPLAY"); waylandDisplay != "" {
		env = append(env, fmt.Sprintf("WAYLAND_DISPLAY=%s", waylandDisplay))
	}
	if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
		env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntime))
	}
	cmd.Env = env

	// Start the process (non-blocking, like sysc-walls does)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch kitty with jellysink: %w", err)
	}

	// Don't wait for the process - let it run independently
	return nil
}
