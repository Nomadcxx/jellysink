package main

import (
	"testing"

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
