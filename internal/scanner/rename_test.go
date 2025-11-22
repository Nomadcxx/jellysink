package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTVShowTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"valid title", "Breaking Bad", false},
		{"valid with year", "Breaking Bad (2008)", false},
		{"empty string", "", true},
		{"only spaces", "   ", true},
		{"invalid char colon", "Title: Subtitle", true},
		{"invalid char question", "What?", true},
		{"invalid char asterisk", "Title*", true},
		{"invalid char pipe", "Title|Name", true},
		{"too long", string(make([]byte, 201)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTVShowTitle(tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTVShowTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyManualTVRename_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	showFolder := filepath.Join(tmpDir, "Degrassi (2001)")
	if err := os.MkdirAll(showFolder, 0755); err != nil {
		t.Fatal(err)
	}

	season1 := filepath.Join(showFolder, "Season 01")
	if err := os.MkdirAll(season1, 0755); err != nil {
		t.Fatal(err)
	}

	ep1 := filepath.Join(season1, "Degrassi S01E01.mkv")
	ep2 := filepath.Join(season1, "Degrassi S01E02.mkv")
	if err := os.WriteFile(ep1, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep2, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := ApplyManualTVRename(tmpDir, "Degrassi", "Degrassi The Next Generation", true)
	if err != nil {
		t.Fatalf("ApplyManualTVRename() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected rename results, got none")
	}

	if _, err := os.Stat(showFolder); os.IsNotExist(err) {
		t.Error("Dry run should not delete original folder")
	}

	if _, err := os.Stat(ep1); os.IsNotExist(err) {
		t.Error("Dry run should not delete original episode")
	}
}

func TestApplyManualTVRename_ActualRename(t *testing.T) {
	tmpDir := t.TempDir()

	showFolder := filepath.Join(tmpDir, "Degrassi (2001)")
	if err := os.MkdirAll(showFolder, 0755); err != nil {
		t.Fatal(err)
	}

	season1 := filepath.Join(showFolder, "Season 01")
	if err := os.MkdirAll(season1, 0755); err != nil {
		t.Fatal(err)
	}

	ep1 := filepath.Join(season1, "Degrassi S01E01.mkv")
	if err := os.WriteFile(ep1, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := ApplyManualTVRename(tmpDir, "Degrassi", "Degrassi The Next Generation", false)
	if err != nil {
		t.Fatalf("ApplyManualTVRename() error = %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("Expected at least 2 rename results (folder + episode), got %d", len(results))
	}

	for _, result := range results {
		if !result.Success {
			t.Errorf("Rename failed: %s -> %s: %s", result.OldPath, result.NewPath, result.Error)
		}
	}

	newShowFolder := filepath.Join(tmpDir, "Degrassi The Next Generation (2001)")
	if _, err := os.Stat(newShowFolder); os.IsNotExist(err) {
		t.Error("New folder should exist after rename")
	}

	if _, err := os.Stat(showFolder); !os.IsNotExist(err) {
		t.Error("Old folder should not exist after rename")
	}
}

func TestApplyManualTVRename_InvalidInput(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		oldTitle string
		newTitle string
		wantErr  bool
	}{
		{"empty new title", "Old Title", "", true},
		{"invalid chars", "Old Title", "New: Title", true},
		{"same title", "Title", "Title", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ApplyManualTVRename(tmpDir, tt.oldTitle, tt.newTitle, true)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyManualTVRename() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRenameEpisodesInFolder(t *testing.T) {
	tmpDir := t.TempDir()

	ep1 := filepath.Join(tmpDir, "Show Title S01E01.mkv")
	ep2 := filepath.Join(tmpDir, "Show Title S01E02 1080p.mkv")
	nonEp := filepath.Join(tmpDir, "README.txt")

	if err := os.WriteFile(ep1, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep2, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nonEp, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := renameEpisodesInFolder(tmpDir, "Show Title", "New Show Title", true)
	if err != nil {
		t.Fatalf("renameEpisodesInFolder() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 episode renames, got %d", len(results))
	}

	for _, result := range results {
		if result.IsFolder {
			t.Error("Episode rename result should not be marked as folder")
		}
	}
}
