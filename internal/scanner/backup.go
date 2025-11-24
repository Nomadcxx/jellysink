package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileEntry struct {
	OriginalPath string    `json:"original_path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsDir        bool      `json:"is_dir"`
	RelPath      string    `json:"rel_path"`
}

type BackupMetadata struct {
	BackupID     string            `json:"backup_id"`
	CreatedAt    time.Time         `json:"created_at"`
	LibraryType  string            `json:"library_type"`
	LibraryPaths []string          `json:"library_paths"`
	TotalFiles   int               `json:"total_files"`
	TotalSize    int64             `json:"total_size"`
	Entries      []FileEntry       `json:"entries"`
	Operations   []BackupOperation `json:"operations"`
	Status       string            `json:"status"`
}

type BackupOperation struct {
	Type      string    `json:"type"`
	OldPath   string    `json:"old_path,omitempty"`
	NewPath   string    `json:"new_path,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

type BackupSnapshot struct {
	Metadata *BackupMetadata
	FilePath string
}

func GetBackupDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	backupDir := filepath.Join(homeDir, ".local", "share", "jellysink", "backups")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	return backupDir, nil
}

func CreateBackup(libraryType string, paths []string, progressCh chan<- ScanProgress) (*BackupSnapshot, error) {
	var pr *ProgressReporter
	if progressCh != nil {
		pr = NewProgressReporterWithInterval(progressCh, "backup_library", 500*time.Millisecond)
		pr.StageUpdate("validating", "Validating library paths for backup...")
	}

	if err := ValidateBeforeScan(paths, "backup", pr); err != nil {
		if pr != nil {
			pr.LogCritical(err, "Backup validation failed")
		}
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	backupID := fmt.Sprintf("%s_%s", libraryType, time.Now().Format("20060102_150405"))

	metadata := &BackupMetadata{
		BackupID:     backupID,
		CreatedAt:    time.Now(),
		LibraryType:  libraryType,
		LibraryPaths: paths,
		Entries:      []FileEntry{},
		Operations:   []BackupOperation{},
		Status:       "in_progress",
	}

	if pr != nil {
		pr.StageUpdate("scanning", "Scanning library for backup...")
	}

	totalFiles := 0
	var totalSize int64

	for _, libPath := range paths {
		if _, err := os.Stat(libPath); err != nil {
			if pr != nil {
				pr.Send("warn", fmt.Sprintf("Skipping inaccessible path: %s", libPath))
			}
			continue
		}

		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if pr != nil {
					pr.Send("warn", fmt.Sprintf("Error accessing %s: %v", path, err))
				}
				return nil
			}

			if !info.IsDir() && isVideoFile(path) {
				relPath, _ := filepath.Rel(libPath, path)

				entry := FileEntry{
					OriginalPath: path,
					Size:         info.Size(),
					ModTime:      info.ModTime(),
					IsDir:        false,
					RelPath:      relPath,
				}

				metadata.Entries = append(metadata.Entries, entry)
				totalFiles++
				totalSize += info.Size()

				if pr != nil && totalFiles%100 == 0 {
					pr.Update(totalFiles, fmt.Sprintf("Backed up %d files...", totalFiles))
				}
			}

			return nil
		})

		if err != nil {
			if pr != nil {
				pr.LogError(err, fmt.Sprintf("Error walking %s", libPath))
			}
		}
	}

	metadata.TotalFiles = totalFiles
	metadata.TotalSize = totalSize
	metadata.Status = "completed"

	backupDir, err := GetBackupDir()
	if err != nil {
		return nil, err
	}

	backupFile := filepath.Join(backupDir, backupID+".json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal backup metadata: %w", err)
	}

	if err := os.WriteFile(backupFile, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write backup file: %w", err)
	}

	snapshot := &BackupSnapshot{
		Metadata: metadata,
		FilePath: backupFile,
	}

	if pr != nil {
		pr.Complete(fmt.Sprintf("Backup complete: %d files, %.2f GB", totalFiles, float64(totalSize)/(1024*1024*1024)))
	}

	return snapshot, nil
}

func LoadBackup(backupID string) (*BackupSnapshot, error) {
	backupDir, err := GetBackupDir()
	if err != nil {
		return nil, err
	}

	backupFile := filepath.Join(backupDir, backupID+".json")

	data, err := os.ReadFile(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}

	var metadata BackupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse backup metadata: %w", err)
	}

	return &BackupSnapshot{
		Metadata: &metadata,
		FilePath: backupFile,
	}, nil
}

func ListBackups() ([]*BackupSnapshot, error) {
	backupDir, err := GetBackupDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []*BackupSnapshot

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		backupID := strings.TrimSuffix(entry.Name(), ".json")
		snapshot, err := LoadBackup(backupID)
		if err != nil {
			continue
		}

		backups = append(backups, snapshot)
	}

	return backups, nil
}

func (b *BackupSnapshot) VerifyIntegrity(progressCh chan<- ScanProgress) (bool, []string) {
	var pr *ProgressReporter
	if progressCh != nil {
		pr = NewProgressReporterWithInterval(progressCh, "verify_backup", 500*time.Millisecond)
		pr.StageUpdate("verifying", "Verifying backup integrity...")
		pr.Start(len(b.Metadata.Entries), fmt.Sprintf("Checking %d files...", len(b.Metadata.Entries)))
	}

	var missingFiles []string
	checked := 0

	for _, entry := range b.Metadata.Entries {
		checked++

		info, err := os.Stat(entry.OriginalPath)
		if err != nil {
			missingFiles = append(missingFiles, entry.OriginalPath)
		} else if info.Size() != entry.Size {
			missingFiles = append(missingFiles, fmt.Sprintf("%s (size changed)", entry.OriginalPath))
		}

		if pr != nil && checked%50 == 0 {
			pr.Update(checked, fmt.Sprintf("Verified %d/%d files...", checked, len(b.Metadata.Entries)))
		}
	}

	intact := len(missingFiles) == 0

	if pr != nil {
		if intact {
			pr.Complete("Backup integrity verified: all files intact")
		} else {
			pr.Complete(fmt.Sprintf("Backup integrity check complete: %d issues found", len(missingFiles)))
		}
	}

	return intact, missingFiles
}

func (b *BackupSnapshot) RecordOperation(opType, oldPath, newPath string, success bool, err error) {
	op := BackupOperation{
		Type:      opType,
		OldPath:   oldPath,
		NewPath:   newPath,
		Timestamp: time.Now(),
		Success:   success,
	}

	if err != nil {
		op.Error = err.Error()
	}

	b.Metadata.Operations = append(b.Metadata.Operations, op)
}

func (b *BackupSnapshot) Save() error {
	data, err := json.MarshalIndent(b.Metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup metadata: %w", err)
	}

	if err := os.WriteFile(b.FilePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

func RevertBackup(backupID string, progressCh chan<- ScanProgress) error {
	var pr *ProgressReporter
	if progressCh != nil {
		pr = NewProgressReporterWithInterval(progressCh, "revert_backup", 500*time.Millisecond)
		pr.StageUpdate("loading", "Loading backup...")
	}

	snapshot, err := LoadBackup(backupID)
	if err != nil {
		if pr != nil {
			pr.LogCritical(err, "Failed to load backup")
		}
		return fmt.Errorf("failed to load backup: %w", err)
	}

	if len(snapshot.Metadata.Operations) == 0 {
		if pr != nil {
			pr.Send("warn", "No operations recorded in backup (nothing to revert)")
		}
		return fmt.Errorf("no operations to revert")
	}

	if pr != nil {
		pr.StageUpdate("reverting", "Reverting operations...")
		pr.Start(len(snapshot.Metadata.Operations), fmt.Sprintf("Reverting %d operations...", len(snapshot.Metadata.Operations)))
	}

	reverted := 0
	failed := 0

	for i := len(snapshot.Metadata.Operations) - 1; i >= 0; i-- {
		op := snapshot.Metadata.Operations[i]

		if !op.Success {
			continue
		}

		var revertErr error

		switch op.Type {
		case "rename":
			revertErr = os.Rename(op.NewPath, op.OldPath)
		case "move":
			revertErr = os.Rename(op.NewPath, op.OldPath)
		case "delete":
			if pr != nil {
				pr.Send("warn", fmt.Sprintf("Cannot restore deleted file: %s", op.OldPath))
			}
			failed++
			continue
		}

		if revertErr != nil {
			failed++
			if pr != nil {
				pr.Send("error", fmt.Sprintf("Failed to revert %s: %v", op.OldPath, revertErr))
			}
		} else {
			reverted++
		}

		if pr != nil && (reverted+failed)%10 == 0 {
			pr.Update(reverted+failed, fmt.Sprintf("Reverted %d operations...", reverted))
		}
	}

	if pr != nil {
		if failed == 0 {
			pr.Complete(fmt.Sprintf("Successfully reverted %d operations", reverted))
		} else {
			pr.Complete(fmt.Sprintf("Reverted %d operations (%d failed)", reverted, failed))
		}
	}

	snapshot.Metadata.Status = "reverted"
	snapshot.Save()

	return nil
}

func DeleteBackup(backupID string) error {
	backupDir, err := GetBackupDir()
	if err != nil {
		return err
	}

	backupFile := filepath.Join(backupDir, backupID+".json")

	if err := os.Remove(backupFile); err != nil {
		return fmt.Errorf("failed to delete backup file: %w", err)
	}

	return nil
}
