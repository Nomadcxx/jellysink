package ui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nomadcxx/jellysink/internal/config"
	"github.com/Nomadcxx/jellysink/internal/scanner"
	"github.com/Nomadcxx/jellysink/internal/ui"
)

func TestAlertModalShowsAndDismisses(t *testing.T) {
	cfg := config.DefaultConfig()
	m := ui.NewScanningModel(cfg)
	m.SetSize(120, 40)

	progress := scanner.ScanProgress{
		Operation: "scanning_movies",
		Stage:     "scanning",
		Message:   "Disk read failed",
		Severity:  "error",
		ShowAlert: true,
		AlertType: "error",
	}

	// Pass progress message into Update
	ret, _ := m.Update(progress)
	newModel := ret.(ui.ScanningModel)

	// Wait briefly for UI to process
	<-time.After(10 * time.Millisecond)

	// Verify the alert message appears in the model
	if newModel.AlertMessage() != "Disk read failed" {
		t.Fatalf("expected alert message 'Disk read failed', got '%s'", newModel.AlertMessage())
	}

	// Dismiss with Enter
	ret2, _ := newModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel2 := ret2.(ui.ScanningModel)
	if newModel2.AlertMessage() != "" {
		t.Fatalf("expected alert dismissed, got '%s'", newModel2.AlertMessage())
	}
}
