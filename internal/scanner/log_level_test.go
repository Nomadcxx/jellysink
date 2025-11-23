package scanner_test

import (
	"testing"
	"time"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func TestLogLevelFiltering(t *testing.T) {
	progressCh := make(chan scanner.ScanProgress, 10)
	pr := scanner.NewProgressReporter(progressCh, "test_operation")

	// Quiet mode: only errors/critical should be sent
	pr.SetLogLevel(scanner.LogLevelQuiet)
	pr.Send("debug", "debug")
	pr.Send("info", "info")
	pr.Send("warn", "warn")

	// debug/info/warn should be filtered out
	select {
	case <-progressCh:
		t.Fatalf("expected no message for filtered severity in quiet mode")
	case <-time.After(25 * time.Millisecond):
	}

	// Now send an error - it should be received
	pr.Send("error", "error")
	select {
	case p := <-progressCh:
		if p.Severity != "error" {
			t.Fatalf("expected severity 'error', got '%s'", p.Severity)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected error message in quiet mode")
	}

	// Verbose mode: everything should pass
	pr.SetLogLevel(scanner.LogLevelVerbose)
	pr.Send("debug", "debug verbose")

	select {
	case p := <-progressCh:
		if p.Severity != "debug" {
			t.Fatalf("expected severity 'debug' in verbose mode, got '%s'", p.Severity)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected debug message in verbose mode")
	}

	// SendSeverityImmediate should bypass filtering in quiet mode
	pr.SetLogLevel(scanner.LogLevelQuiet)
	pr.Send("debug", "filtered debug")
	pr.SendSeverityImmediate("debug", "immediate debug")

	select {
	case p := <-progressCh:
		if p.Message != "immediate debug" {
			t.Fatalf("expected immediate debug message, got '%s'", p.Message)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected immediate debug message to be sent")
	}
}
