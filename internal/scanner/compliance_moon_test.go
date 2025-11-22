package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoonRightSiZECompliance(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create the exact structure from the bug report
	movieDir := filepath.Join(tmpDir, "Moon RightSiZE (2009)")
	if err := os.MkdirAll(movieDir, 0755); err != nil {
		t.Fatal(err)
	}

	movieFile := filepath.Join(movieDir, "Moon.2009.720p.BluRay.DD5.1.x264-RightSiZE.mkv")
	if err := os.WriteFile(movieFile, []byte("fake movie"), 0644); err != nil {
		t.Fatal(err)
	}

	// Scan for compliance issues
	issues, err := ScanMovieCompliance([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovieCompliance failed: %v", err)
	}

	// Should find 1 issue
	if len(issues) != 1 {
		t.Fatalf("Expected 1 compliance issue, got %d", len(issues))
	}

	issue := issues[0]

	// Check the issue details
	if issue.Problem != "Folder name doesn't match filename" {
		t.Errorf("Expected problem 'Folder name doesn't match filename', got %q", issue.Problem)
	}

	// The suggested path should be cleaned (no RightSiZE)
	expectedSuggestedDir := filepath.Join(tmpDir, "Moon (2009)")
	expectedSuggestedPath := filepath.Join(expectedSuggestedDir, "Moon (2009).mkv")

	if issue.SuggestedPath != expectedSuggestedPath {
		t.Errorf("Expected suggested path:\n  %s\nGot:\n  %s", expectedSuggestedPath, issue.SuggestedPath)
	}

	t.Logf("✓ Correctly suggests cleaning release group from folder/filename")
	t.Logf("  Current:  %s", issue.Path)
	t.Logf("  Suggested: %s", issue.SuggestedPath)
}
