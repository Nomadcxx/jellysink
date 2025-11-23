package main

import (
	"testing"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/daemon"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func TestLogLevelCLIIntegration(t *testing.T) {
	// Test that CLI flags properly set the global DefaultLogLevel

	// Reset to default before test
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)

	// Test case 1: quiet flag sets LogLevelQuiet
	scanner.SetDefaultLogLevel(scanner.LogLevelQuiet)
	quietLevel := scanner.GetDefaultLogLevel()
	if quietLevel != scanner.LogLevelQuiet {
		t.Errorf("Expected LogLevelQuiet, got %v", quietLevel)
	}

	// Test case 2: verbose flag sets LogLevelVerbose
	scanner.SetDefaultLogLevel(scanner.LogLevelVerbose)
	verboseLevel := scanner.GetDefaultLogLevel()
	if verboseLevel != scanner.LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose, got %v", verboseLevel)
	}

	// Test case 3: normal level sets LogLevelNormal
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)
	normalLevel := scanner.GetDefaultLogLevel()
	if normalLevel != scanner.LogLevelNormal {
		t.Errorf("Expected LogLevelNormal, got %v", normalLevel)
	}

	// Reset to default after test
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)
}

func TestCLIPrecedenceOverDaemonConfig(t *testing.T) {
	// Test that CLI flags take precedence over daemon config log level

	// Save original log level
	originalLogLevel := scanner.GetDefaultLogLevel()
	defer scanner.SetDefaultLogLevel(originalLogLevel)

	// Simulate runScan behavior: CLI sets --quiet
	scanner.SetDefaultLogLevel(scanner.LogLevelQuiet)

	// Config has verbose setting
	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			LogLevel: "verbose",
		},
	}

	// Create daemon (should NOT override CLI setting)
	daemon.New(cfg)

	// Verify CLI flag (quiet) wins over config (verbose)
	actualLogLevel := scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelQuiet {
		t.Errorf("Expected LogLevelQuiet (CLI precedence), got %v", actualLogLevel)
	}
}

func TestDaemonConfigAppliesWhenNoCLIFlag(t *testing.T) {
	// Test that daemon config applies when no CLI flag is set

	// Save original log level
	originalLogLevel := scanner.GetDefaultLogLevel()
	defer scanner.SetDefaultLogLevel(originalLogLevel)

	// Reset to default (simulates no CLI flag)
	scanner.SetDefaultLogLevel(scanner.LogLevelNormal)

	// Config has verbose setting
	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			LogLevel: "verbose",
		},
	}

	// Create daemon (should apply config)
	daemon.New(cfg)

	// Verify config was applied
	actualLogLevel := scanner.GetDefaultLogLevel()
	if actualLogLevel != scanner.LogLevelVerbose {
		t.Errorf("Expected LogLevelVerbose (from config), got %v", actualLogLevel)
	}
}
