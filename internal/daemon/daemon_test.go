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

	// Test case 1: Config with "quiet" log level
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

	// Test case 2: Config with "verbose" log level
	cfg.Daemon.LogLevel = "verbose"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose, got %v", actualLogLevel)
	}

	// Test case 3: Config with "normal" log level
	cfg.Daemon.LogLevel = "normal"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal, got %v", actualLogLevel)
	}

	// Test case 4: Invalid log level defaults to normal
	cfg.Daemon.LogLevel = "invalid_level"
	New(cfg)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal (default for invalid), got %v", actualLogLevel)
	}

	// Test case 5: Nil config defaults to normal
	scanner.SetDefaultLogLevel(scanner.LogLevelVerbose)
	New(nil)

	actualLogLevel = scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal (default for nil config), got %v", actualLogLevel)
	}
}
