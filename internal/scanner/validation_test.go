package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLibraryPaths(t *testing.T) {
	tmpDir := t.TempDir()

	existingPath := filepath.Join(tmpDir, "movies")
	if err := os.MkdirAll(existingPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo := filepath.Join(existingPath, "test.mkv")
	if err := os.WriteFile(testVideo, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	nonExistentPath := filepath.Join(tmpDir, "nonexistent")

	tests := []struct {
		name            string
		paths           []string
		requireWritable bool
		wantAccessible  int
		wantErr         bool
	}{
		{
			name:            "valid path",
			paths:           []string{existingPath},
			requireWritable: false,
			wantAccessible:  1,
			wantErr:         false,
		},
		{
			name:            "mixed valid and invalid",
			paths:           []string{existingPath, nonExistentPath},
			requireWritable: false,
			wantAccessible:  1,
			wantErr:         false,
		},
		{
			name:            "all invalid paths",
			paths:           []string{nonExistentPath},
			requireWritable: false,
			wantAccessible:  0,
			wantErr:         true,
		},
		{
			name:            "empty paths",
			paths:           []string{},
			requireWritable: false,
			wantAccessible:  0,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := ValidateLibraryPaths(tt.paths, tt.requireWritable)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLibraryPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if report.AccessiblePaths != tt.wantAccessible {
				t.Errorf("AccessiblePaths = %d, want %d", report.AccessiblePaths, tt.wantAccessible)
			}

			if tt.wantErr && report.CanProceed {
				t.Error("CanProceed = true, want false when error expected")
			}
		})
	}
}

func TestValidatePathDepth(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		operation string
		wantErr   bool
	}{
		{
			name:      "valid deep path",
			path:      "/mnt/storage1/movies",
			operation: "scan",
			wantErr:   false,
		},
		{
			name:      "root path",
			path:      "/",
			operation: "scan",
			wantErr:   true,
		},
		{
			name:      "protected path /mnt",
			path:      "/mnt",
			operation: "scan",
			wantErr:   true,
		},
		{
			name:      "shallow path",
			path:      "/mnt/storage",
			operation: "rename",
			wantErr:   true,
		},
		{
			name:      "empty path",
			path:      "",
			operation: "scan",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathDepth(tt.path, tt.operation)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathDepth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateAndLoadBackup(t *testing.T) {
	tmpDir := t.TempDir()

	moviesPath := filepath.Join(tmpDir, "movies")
	if err := os.MkdirAll(moviesPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo1 := filepath.Join(moviesPath, "movie1.mkv")
	testVideo2 := filepath.Join(moviesPath, "movie2.mp4")

	if err := os.WriteFile(testVideo1, []byte("fake video 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testVideo2, []byte("fake video 2"), 0644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := CreateBackup("test_movies", []string{moviesPath}, nil)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	if snapshot == nil {
		t.Fatal("CreateBackup() returned nil snapshot")
	}

	if snapshot.Metadata.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", snapshot.Metadata.TotalFiles)
	}

	if snapshot.Metadata.LibraryType != "test_movies" {
		t.Errorf("LibraryType = %s, want test_movies", snapshot.Metadata.LibraryType)
	}

	loaded, err := LoadBackup(snapshot.Metadata.BackupID)
	if err != nil {
		t.Fatalf("LoadBackup() error = %v", err)
	}

	if loaded.Metadata.BackupID != snapshot.Metadata.BackupID {
		t.Errorf("Loaded BackupID = %s, want %s", loaded.Metadata.BackupID, snapshot.Metadata.BackupID)
	}

	if loaded.Metadata.TotalFiles != snapshot.Metadata.TotalFiles {
		t.Errorf("Loaded TotalFiles = %d, want %d", loaded.Metadata.TotalFiles, snapshot.Metadata.TotalFiles)
	}

	if err := DeleteBackup(snapshot.Metadata.BackupID); err != nil {
		t.Errorf("DeleteBackup() error = %v", err)
	}
}

func TestBackupVerifyIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	moviesPath := filepath.Join(tmpDir, "movies")
	if err := os.MkdirAll(moviesPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo := filepath.Join(moviesPath, "movie.mkv")
	if err := os.WriteFile(testVideo, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := CreateBackup("test_integrity", []string{moviesPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteBackup(snapshot.Metadata.BackupID)

	intact, missing := snapshot.VerifyIntegrity(nil)
	if !intact {
		t.Errorf("VerifyIntegrity() intact = false, want true")
	}
	if len(missing) != 0 {
		t.Errorf("VerifyIntegrity() missing = %d files, want 0", len(missing))
	}

	if err := os.Remove(testVideo); err != nil {
		t.Fatal(err)
	}

	intact, missing = snapshot.VerifyIntegrity(nil)
	if intact {
		t.Errorf("VerifyIntegrity() intact = true, want false after deletion")
	}
	if len(missing) != 1 {
		t.Errorf("VerifyIntegrity() missing = %d files, want 1", len(missing))
	}
}

func TestValidatePathWithSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	realPath := filepath.Join(tmpDir, "storage5", "movies")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo := filepath.Join(realPath, "test.mkv")
	if err := os.WriteFile(testVideo, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(tmpDir, "storage1", "movies")
	symlinkParent := filepath.Join(tmpDir, "storage1")
	if err := os.MkdirAll(symlinkParent, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(realPath, symlinkPath); err != nil {
		t.Skip("Symlink creation not supported on this system")
	}

	report, err := ValidateLibraryPaths([]string{symlinkPath}, false)
	if err != nil {
		t.Fatalf("ValidateLibraryPaths() failed on symlink: %v", err)
	}

	if report.AccessiblePaths != 1 {
		t.Errorf("AccessiblePaths = %d, want 1 (symlink should be resolved)", report.AccessiblePaths)
	}

	fileViaSymlink := filepath.Join(symlinkPath, "test.mkv")
	err = ValidatePathInLibrary(fileViaSymlink, []string{symlinkPath})
	if err != nil {
		t.Errorf("ValidatePathInLibrary() failed for file accessed via symlink: %v", err)
	}

	err = ValidatePathInLibrary(testVideo, []string{symlinkPath})
	if err != nil {
		t.Errorf("ValidatePathInLibrary() failed for real path when library is symlinked: %v", err)
	}
}

func TestValidatePathInLibraryWithSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	library1 := filepath.Join(tmpDir, "library1")
	library2 := filepath.Join(tmpDir, "library2")

	if err := os.MkdirAll(library1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(library2, 0755); err != nil {
		t.Fatal(err)
	}

	linkedDir := filepath.Join(library1, "linked")
	if err := os.Symlink(library2, linkedDir); err != nil {
		t.Skip("Symlink creation not supported")
	}

	fileInLibrary2 := filepath.Join(library2, "movie.mkv")
	if err := os.WriteFile(fileInLibrary2, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	fileViaSymlink := filepath.Join(linkedDir, "movie.mkv")

	err := ValidatePathInLibrary(fileViaSymlink, []string{library1})
	if err != nil {
		t.Errorf("ValidatePathInLibrary() should allow file accessed via symlink within library: %v", err)
	}

	err = ValidatePathInLibrary(fileInLibrary2, []string{library1})
	if err == nil {
		t.Errorf("ValidatePathInLibrary() should reject file from different library even if symlinked")
	}
}

func TestValidateLibraryPathsEmptyBackup(t *testing.T) {
	tmpDir := t.TempDir()

	emptyPath := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyPath, 0755); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateLibraryPaths([]string{emptyPath}, false)
	if err != nil {
		t.Fatalf("ValidateLibraryPaths() error = %v, want nil for accessible but empty path", err)
	}

	if len(report.Warnings) == 0 {
		t.Error("Expected warning for path with no video files")
	}

	if !report.CanProceed {
		t.Error("CanProceed = false, want true (accessible path should allow proceeding)")
	}
}

func TestCreateBackupWithNoVideos(t *testing.T) {
	tmpDir := t.TempDir()

	emptyPath := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyPath, 0755); err != nil {
		t.Fatal(err)
	}

	snapshot, err := CreateBackup("test_empty", []string{emptyPath}, nil)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v, want nil for empty library", err)
	}
	defer DeleteBackup(snapshot.Metadata.BackupID)

	if snapshot.Metadata.TotalFiles != 0 {
		t.Errorf("TotalFiles = %d, want 0 for empty library", snapshot.Metadata.TotalFiles)
	}

	if snapshot.Metadata.Status != "completed" {
		t.Errorf("Status = %s, want completed", snapshot.Metadata.Status)
	}
}

func TestBackupWithMixedAccessiblePaths(t *testing.T) {
	tmpDir := t.TempDir()

	validPath := filepath.Join(tmpDir, "valid")
	if err := os.MkdirAll(validPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo := filepath.Join(validPath, "movie.mkv")
	if err := os.WriteFile(testVideo, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	invalidPath := filepath.Join(tmpDir, "nonexistent")

	snapshot, err := CreateBackup("test_mixed", []string{validPath, invalidPath}, nil)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v, want nil (should skip inaccessible paths)", err)
	}
	defer DeleteBackup(snapshot.Metadata.BackupID)

	if snapshot.Metadata.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1 (should only backup accessible path)", snapshot.Metadata.TotalFiles)
	}
}

func TestBackupIntegritySizeChange(t *testing.T) {
	tmpDir := t.TempDir()

	moviesPath := filepath.Join(tmpDir, "movies")
	if err := os.MkdirAll(moviesPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo := filepath.Join(moviesPath, "movie.mkv")
	if err := os.WriteFile(testVideo, []byte("original content"), 0644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := CreateBackup("test_size_change", []string{moviesPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteBackup(snapshot.Metadata.BackupID)

	if err := os.WriteFile(testVideo, []byte("modified content with different size"), 0644); err != nil {
		t.Fatal(err)
	}

	intact, missing := snapshot.VerifyIntegrity(nil)
	if intact {
		t.Errorf("VerifyIntegrity() intact = true, want false after size change")
	}
	if len(missing) != 1 {
		t.Errorf("VerifyIntegrity() missing = %d, want 1 (size changed file)", len(missing))
	}
}

func TestRevertBackupNoOperations(t *testing.T) {
	tmpDir := t.TempDir()

	moviesPath := filepath.Join(tmpDir, "movies")
	if err := os.MkdirAll(moviesPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo := filepath.Join(moviesPath, "movie.mkv")
	if err := os.WriteFile(testVideo, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := CreateBackup("test_no_ops", []string{moviesPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteBackup(snapshot.Metadata.BackupID)

	err = RevertBackup(snapshot.Metadata.BackupID, nil)
	if err == nil {
		t.Error("RevertBackup() error = nil, want error for backup with no operations")
	}
}

func TestRevertBackupInvalidID(t *testing.T) {
	err := RevertBackup("nonexistent_backup_id", nil)
	if err == nil {
		t.Error("RevertBackup() error = nil, want error for invalid backup ID")
	}
}

func TestListBackupsMultiple(t *testing.T) {
	tmpDir := t.TempDir()

	moviesPath := filepath.Join(tmpDir, "movies")
	if err := os.MkdirAll(moviesPath, 0755); err != nil {
		t.Fatal(err)
	}

	testVideo := filepath.Join(moviesPath, "movie.mkv")
	if err := os.WriteFile(testVideo, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	snapshot1, err := CreateBackup("test_list_1", []string{moviesPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteBackup(snapshot1.Metadata.BackupID)

	snapshot2, err := CreateBackup("test_list_2", []string{moviesPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteBackup(snapshot2.Metadata.BackupID)

	backups, err := ListBackups()
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}

	if len(backups) < 2 {
		t.Errorf("ListBackups() returned %d backups, want at least 2", len(backups))
	}

	found1, found2 := false, false
	for _, backup := range backups {
		if backup.Metadata.BackupID == snapshot1.Metadata.BackupID {
			found1 = true
		}
		if backup.Metadata.BackupID == snapshot2.Metadata.BackupID {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("ListBackups() did not return both created backups")
	}
}

func TestValidatePathDepthWithSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	deepPath := filepath.Join(tmpDir, "storage", "media", "movies")
	if err := os.MkdirAll(deepPath, 0755); err != nil {
		t.Fatal(err)
	}

	shallowLink := filepath.Join(tmpDir, "movies")
	if err := os.Symlink(deepPath, shallowLink); err != nil {
		t.Skip("Symlink creation not supported")
	}

	err := ValidatePathDepth(shallowLink, "scan")
	if err != nil {
		t.Errorf("ValidatePathDepth() error = %v, want nil (symlink resolves to deep path)", err)
	}

	if err := os.MkdirAll("/tmp/test_jellysink_shallow", 0755); err != nil {
		t.Skip("Cannot create test path in /tmp")
	}
	defer os.RemoveAll("/tmp/test_jellysink_shallow")

	shallowLinkToShallow := filepath.Join(tmpDir, "link")
	if err := os.Symlink("/tmp/test_jellysink_shallow", shallowLinkToShallow); err != nil {
		t.Skip("Symlink creation not supported")
	}

	err = ValidatePathDepth(shallowLinkToShallow, "scan")
	if err == nil {
		realResolved, _ := filepath.EvalSymlinks(shallowLinkToShallow)
		parts := strings.Split(strings.TrimPrefix(realResolved, "/"), "/")
		t.Errorf("ValidatePathDepth() error = nil, want error when symlink resolves to shallow path (resolved: %s, parts: %d)", realResolved, len(parts))
	}
}
