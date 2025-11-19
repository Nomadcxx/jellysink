package cleaner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func TestIsProtectedPath(t *testing.T) {
	protected := []string{"/usr", "/etc", "/home"}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/usr/bin/something", true},
		{"/etc/config", true},
		{"/home/user/file", true},
		{"/mnt/storage/file", false},
		{"/tmp/file", false},
	}

	for _, tt := range tests {
		result := isProtectedPath(tt.path, protected)
		if result != tt.expected {
			t.Errorf("isProtectedPath(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestCalculateTotalSize(t *testing.T) {
	movies := []scanner.MovieDuplicate{
		{
			Files: []scanner.MovieFile{
				{Size: 5 * 1024 * 1024 * 1024}, // Keep
				{Size: 2 * 1024 * 1024 * 1024}, // Delete
				{Size: 1 * 1024 * 1024 * 1024}, // Delete
			},
		},
	}

	tv := []scanner.TVDuplicate{
		{
			Files: []scanner.TVFile{
				{Size: 1 * 1024 * 1024 * 1024}, // Keep
				{Size: 500 * 1024 * 1024},      // Delete
			},
		},
	}

	total := calculateTotalSize(movies, tv)
	expected := int64(3*1024*1024*1024 + 500*1024*1024) // 3GB + 500MB

	if total != expected {
		t.Errorf("calculateTotalSize() = %d, want %d", total, expected)
	}
}

func TestCleanDryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	keepFile := filepath.Join(tmpDir, "keep.mkv")
	deleteFile := filepath.Join(tmpDir, "delete.mkv")

	os.WriteFile(keepFile, []byte("keeper"), 0644)
	os.WriteFile(deleteFile, []byte("delete me"), 0644)

	// Create duplicate group
	duplicates := []scanner.MovieDuplicate{
		{
			Files: []scanner.MovieFile{
				{Path: keepFile, Size: 100},
				{Path: deleteFile, Size: 50},
			},
		},
	}

	// Run in dry-run mode
	config := DefaultConfig()
	config.DryRun = true

	result, err := Clean(duplicates, []scanner.TVDuplicate{}, []scanner.ComplianceIssue{}, config)
	if err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	// Check that files still exist
	if _, err := os.Stat(deleteFile); os.IsNotExist(err) {
		t.Error("Dry-run deleted file (should not happen)")
	}

	// Check that operation was recorded
	if len(result.Operations) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(result.Operations))
	}

	if result.Operations[0].Type != "delete" {
		t.Errorf("Expected delete operation, got %s", result.Operations[0].Type)
	}
}

func TestCleanDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	keepFile := filepath.Join(tmpDir, "keep.mkv")
	deleteFile := filepath.Join(tmpDir, "delete.mkv")

	os.WriteFile(keepFile, []byte("keeper"), 0644)
	os.WriteFile(deleteFile, []byte("delete me"), 0644)

	// Create duplicate group
	duplicates := []scanner.MovieDuplicate{
		{
			Files: []scanner.MovieFile{
				{Path: keepFile, Size: 100},
				{Path: deleteFile, Size: 50},
			},
		},
	}

	// Run actual deletion
	config := DefaultConfig()
	config.DryRun = false

	result, err := Clean(duplicates, []scanner.TVDuplicate{}, []scanner.ComplianceIssue{}, config)
	if err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	// Check that delete file is gone
	if _, err := os.Stat(deleteFile); !os.IsNotExist(err) {
		t.Error("Delete file still exists")
	}

	// Check that keep file still exists
	if _, err := os.Stat(keepFile); os.IsNotExist(err) {
		t.Error("Keep file was deleted")
	}

	// Check result
	if result.DuplicatesDeleted != 1 {
		t.Errorf("Expected 1 duplicate deleted, got %d", result.DuplicatesDeleted)
	}

	if result.SpaceFreed != 50 {
		t.Errorf("Expected 50 bytes freed, got %d", result.SpaceFreed)
	}
}

func TestPerformRename(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	oldPath := filepath.Join(tmpDir, "old_name.mkv")
	newPath := filepath.Join(tmpDir, "new_name.mkv")

	os.WriteFile(oldPath, []byte("content"), 0644)

	// Perform rename
	op, err := performRename(oldPath, newPath, false)
	if err != nil {
		t.Fatalf("performRename() error: %v", err)
	}

	// Check old path doesn't exist
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old path still exists after rename")
	}

	// Check new path exists
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("New path doesn't exist after rename")
	}

	// Check operation details
	if op.Type != "rename" {
		t.Errorf("Expected operation type 'rename', got %s", op.Type)
	}

	if !op.Completed {
		t.Error("Operation not marked as completed")
	}
}

func TestPerformReorganize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file in subdirectory
	oldDir := filepath.Join(tmpDir, "old", "folder")
	os.MkdirAll(oldDir, 0755)

	oldPath := filepath.Join(oldDir, "file.mkv")
	os.WriteFile(oldPath, []byte("content"), 0644)

	// New path in different structure
	newPath := filepath.Join(tmpDir, "new", "structure", "file.mkv")

	// Perform reorganize
	op, err := performReorganize(oldPath, newPath, false)
	if err != nil {
		t.Fatalf("performReorganize() error: %v", err)
	}

	// Check old path doesn't exist
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old path still exists after reorganize")
	}

	// Check new path exists
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("New path doesn't exist after reorganize")
	}

	// Check new directory was created
	if _, err := os.Stat(filepath.Dir(newPath)); os.IsNotExist(err) {
		t.Error("New directory wasn't created")
	}

	// Check operation details
	if op.Type != "move" {
		t.Errorf("Expected operation type 'move', got %s", op.Type)
	}

	if !op.Completed {
		t.Error("Operation not marked as completed")
	}
}

func TestCleanCompliance(t *testing.T) {
	tmpDir := t.TempDir()

	// Create non-compliant file
	oldPath := filepath.Join(tmpDir, "Movie.2024.1080p.mkv")
	os.WriteFile(oldPath, []byte("content"), 0644)

	newPath := filepath.Join(tmpDir, "Movie (2024).mkv")

	// Create compliance issue
	issues := []scanner.ComplianceIssue{
		{
			Path:            oldPath,
			Type:            "movie",
			Problem:         "Wrong format",
			SuggestedPath:   newPath,
			SuggestedAction: "rename",
		},
	}

	// Run cleanup
	config := DefaultConfig()
	config.DryRun = false

	result, err := Clean([]scanner.MovieDuplicate{}, []scanner.TVDuplicate{}, issues, config)
	if err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	// Check file was renamed
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old file still exists after compliance fix")
	}

	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("New file doesn't exist after compliance fix")
	}

	// Check result
	if result.ComplianceFixed != 1 {
		t.Errorf("Expected 1 compliance issue fixed, got %d", result.ComplianceFixed)
	}
}

func TestCleanProtectedPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file in "protected" location
	protectedFile := filepath.Join(tmpDir, "protected", "file.mkv")
	os.MkdirAll(filepath.Dir(protectedFile), 0755)
	os.WriteFile(protectedFile, []byte("content"), 0644)

	// Try to delete it
	duplicates := []scanner.MovieDuplicate{
		{
			Files: []scanner.MovieFile{
				{Path: "/keep/file.mkv", Size: 100},
				{Path: protectedFile, Size: 50},
			},
		},
	}

	config := DefaultConfig()
	config.DryRun = false
	config.ProtectedPaths = []string{filepath.Join(tmpDir, "protected")}

	result, err := Clean(duplicates, []scanner.TVDuplicate{}, []scanner.ComplianceIssue{}, config)
	if err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	// Check that protected file still exists
	if _, err := os.Stat(protectedFile); os.IsNotExist(err) {
		t.Error("Protected file was deleted")
	}

	// Check that error was recorded
	if len(result.Errors) == 0 {
		t.Error("Expected error for protected path, got none")
	}
}

func TestCleanSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create duplicates that exceed size limit
	duplicates := []scanner.MovieDuplicate{
		{
			Files: []scanner.MovieFile{
				{Path: filepath.Join(tmpDir, "keep.mkv"), Size: 5 * 1024 * 1024 * 1024},
				{Path: filepath.Join(tmpDir, "delete.mkv"), Size: 200 * 1024 * 1024 * 1024}, // 200GB
			},
		},
	}

	config := DefaultConfig()
	config.MaxSizeGB = 100 // 100GB limit

	_, err := Clean(duplicates, []scanner.TVDuplicate{}, []scanner.ComplianceIssue{}, config)

	// Should return error due to size limit
	if err == nil {
		t.Error("Expected error for size limit exceeded, got none")
	}
}
