package daemon

import (
	"testing"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func TestDaemonLogLevelConfiguration(t *testing.T) {
	// Test that daemon.New() sets the global log level based on config

	// Save current global log level to restore later
	originalLogLevel := scanner.GetDefaultLogLevel()
	defer scanner.SetDefaultLogLevel(originalLogLevel)

	// Test case 1: Config with "quiet" log level (from default Normal)
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)
	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			LogLevel: "quiet",
		},
	}

	d := New(cfg)
	if d == nil {
		t.Fatal("New() returned nil")
	}

	// Check that the global log level was set to quiet
	actualLogLevel := scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelQuiet {
		t.Errorf("Expected LogLevelQuiet, got %v", actualLogLevel)
	}

	// Test case 2: Config with "verbose" log level (from default Normal)
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)
	cfg.Daemon.LogLevel = "verbose"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose, got %v", actualLogLevel)
	}

	// Test case 3: Config with "normal" log level (from default Normal)
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)
	cfg.Daemon.LogLevel = "normal"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal, got %v", actualLogLevel)
	}

	// Test case 4: Invalid log level - config ignored, remains Normal
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)
	cfg.Daemon.LogLevel = "invalid_level"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal (config parse failed, unchanged), got %v", actualLogLevel)
	}

	// Test case 5: Nil config - remains Normal
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)
	New(nil)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal (nil config, unchanged), got %v", actualLogLevel)
	}

	// Test case 6: CLI sets Quiet, config has Verbose - CLI wins (precedence)
	scanner.SetDefaultLogLevel(scanner.LogLevelQuiet) // Simulate CLI setting
	cfg.Daemon.LogLevel = "verbose"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelQuiet {
		t.Errorf("Expected LogLevelQuiet (CLI precedence), got %v", actualLogLevel)
	}

	// Test case 7: CLI sets Verbose, config has Quiet - CLI wins
	scanner.SetDefaultLogLevel(scanner.LogLevelVerbose) // Simulate CLI setting
	cfg.Daemon.LogLevel = "quiet"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose (CLI precedence), got %v", actualLogLevel)
	}
}
