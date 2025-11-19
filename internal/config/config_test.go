package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Daemon.ScanFrequency != "weekly" {
		t.Errorf("expected scan frequency 'weekly', got '%s'", cfg.Daemon.ScanFrequency)
	}

	if !cfg.Daemon.ReportOnComplete {
		t.Error("expected ReportOnComplete to be true")
	}

	if len(cfg.Libraries.Movies.Paths) != 0 {
		t.Errorf("expected empty movie paths, got %d", len(cfg.Libraries.Movies.Paths))
	}

	if len(cfg.Libraries.TV.Paths) != 0 {
		t.Errorf("expected empty TV paths, got %d", len(cfg.Libraries.TV.Paths))
	}
}

func TestAddMoviePath(t *testing.T) {
	cfg := DefaultConfig()

	// Create temp directory for testing
	tmpDir := t.TempDir()

	// Add valid path
	if err := cfg.AddMoviePath(tmpDir); err != nil {
		t.Fatalf("failed to add movie path: %v", err)
	}

	if len(cfg.Libraries.Movies.Paths) != 1 {
		t.Errorf("expected 1 movie path, got %d", len(cfg.Libraries.Movies.Paths))
	}

	if cfg.Libraries.Movies.Paths[0] != tmpDir {
		t.Errorf("expected path %s, got %s", tmpDir, cfg.Libraries.Movies.Paths[0])
	}

	// Try to add duplicate
	if err := cfg.AddMoviePath(tmpDir); err == nil {
		t.Error("expected error when adding duplicate path")
	}

	// Try to add non-existent path
	if err := cfg.AddMoviePath("/nonexistent/path"); err == nil {
		t.Error("expected error when adding non-existent path")
	}
}

func TestAddTVPath(t *testing.T) {
	cfg := DefaultConfig()

	// Create temp directory for testing
	tmpDir := t.TempDir()

	// Add valid path
	if err := cfg.AddTVPath(tmpDir); err != nil {
		t.Fatalf("failed to add TV path: %v", err)
	}

	if len(cfg.Libraries.TV.Paths) != 1 {
		t.Errorf("expected 1 TV path, got %d", len(cfg.Libraries.TV.Paths))
	}

	if cfg.Libraries.TV.Paths[0] != tmpDir {
		t.Errorf("expected path %s, got %s", tmpDir, cfg.Libraries.TV.Paths[0])
	}
}

func TestRemovePaths(t *testing.T) {
	cfg := DefaultConfig()
	tmpDir := t.TempDir()

	// Add and remove movie path
	cfg.AddMoviePath(tmpDir)
	if err := cfg.RemoveMoviePath(tmpDir); err != nil {
		t.Fatalf("failed to remove movie path: %v", err)
	}
	if len(cfg.Libraries.Movies.Paths) != 0 {
		t.Error("expected empty movie paths after removal")
	}

	// Try to remove non-existent path
	if err := cfg.RemoveMoviePath("/nonexistent"); err == nil {
		t.Error("expected error when removing non-existent path")
	}

	// Add and remove TV path
	cfg.AddTVPath(tmpDir)
	if err := cfg.RemoveTVPath(tmpDir); err != nil {
		t.Fatalf("failed to remove TV path: %v", err)
	}
	if len(cfg.Libraries.TV.Paths) != 0 {
		t.Error("expected empty TV paths after removal")
	}
}

func TestValidate(t *testing.T) {
	cfg := DefaultConfig()

	// Empty config should fail validation (no paths)
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation to fail with no paths configured")
	}

	// Add valid path
	tmpDir := t.TempDir()
	cfg.AddMoviePath(tmpDir)

	// Should pass validation now
	if err := cfg.Validate(); err != nil {
		t.Errorf("validation failed with valid config: %v", err)
	}

	// Invalid scan frequency
	cfg.Daemon.ScanFrequency = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation to fail with invalid scan frequency")
	}

	// Reset to valid
	cfg.Daemon.ScanFrequency = "weekly"
	if err := cfg.Validate(); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Skip this test for now - would require mocking ConfigPath
	// We'll test Save/Load functionality in integration tests
	t.Skip("Skipping Save/Load test - requires mocking")
}

func TestGetAllPaths(t *testing.T) {
	cfg := DefaultConfig()
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	cfg.AddMoviePath(tmpDir1)
	cfg.AddTVPath(tmpDir2)

	allPaths := cfg.GetAllPaths()

	if len(allPaths) != 2 {
		t.Errorf("expected 2 total paths, got %d", len(allPaths))
	}

	// Check both paths are present
	foundMovie := false
	foundTV := false
	for _, path := range allPaths {
		if path == tmpDir1 {
			foundMovie = true
		}
		if path == tmpDir2 {
			foundTV = true
		}
	}

	if !foundMovie || !foundTV {
		t.Error("not all paths found in GetAllPaths()")
	}
}
