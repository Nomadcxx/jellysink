package scanner_test

import (
	"testing"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func TestProgressReporting(t *testing.T) {
	// Create progress channel
	progressCh := make(chan scanner.ScanProgress, 100)

	// Test progress reporter
	pr := scanner.NewProgressReporter(progressCh, "test_operation")

	pr.Start(100, "Starting test")
	progress1 := <-progressCh
	if progress1.Operation != "test_operation" {
		t.Errorf("Expected operation 'test_operation', got '%s'", progress1.Operation)
	}
	if progress1.Total != 100 {
		t.Errorf("Expected total 100, got %d", progress1.Total)
	}

	pr.Update(50, "Halfway there")
	progress2 := <-progressCh
	if progress2.Current != 50 {
		t.Errorf("Expected current 50, got %d", progress2.Current)
	}
	if progress2.Percentage < 49.0 || progress2.Percentage > 51.0 {
		t.Errorf("Expected percentage ~50, got %.2f", progress2.Percentage)
	}

	pr.Complete("Done!")
	progress3 := <-progressCh
	if progress3.Stage != "complete" {
		t.Errorf("Expected stage 'complete', got '%s'", progress3.Stage)
	}
	if progress3.Percentage != 100.0 {
		t.Errorf("Expected percentage 100, got %.2f", progress3.Percentage)
	}
}
