package safety

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type SnapshotService struct {
	BackupDir string
}

func NewSnapshotService(backupDir string) (*SnapshotService, error) {
	if backupDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		backupDir = filepath.Join(home, ".local", "share", "hyprAgent", "backups")
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, err
	}
	return &SnapshotService{BackupDir: backupDir}, nil
}

// CreateSnapshot creates a backup of the specified files
func (s *SnapshotService) CreateSnapshot(files []string) (string, error) {
	id := time.Now().Format("20060102-150405")
	snapshotDir := filepath.Join(s.BackupDir, id)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return "", err
	}

	for _, src := range files {
		// Determine destination path inside snapshot
		// We flatly store them or mimic structure?
		// For simplicity, flat storage with name handling or preserving relative structure is hard.
		// Let's just store basenames for MVP, assuming unique filenames or single main config.
		// If we have multiple files, this needs better logic (e.g., storing metadata).
		base := filepath.Base(src)
		dst := filepath.Join(snapshotDir, base)

		if err := copyFile(src, dst); err != nil {
			return "", fmt.Errorf("failed to copy %s: %w", src, err)
		}
	}
	return id, nil
}

// Restore restores the files from the snapshot
func (s *SnapshotService) Restore(id string, targetFiles []string) error {
	snapshotDir := filepath.Join(s.BackupDir, id)
	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		return fmt.Errorf("snapshot %s not found", id)
	}

	// This simplistic restore assumes targetFiles match what's in snapshot by name
	// In a real system, we need a manifest.
	for _, target := range targetFiles {
		base := filepath.Base(target)
		src := filepath.Join(snapshotDir, base)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, target); err != nil {
				return fmt.Errorf("failed to restore %s: %w", target, err)
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}


