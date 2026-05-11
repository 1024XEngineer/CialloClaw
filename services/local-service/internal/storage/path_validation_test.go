package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareSQLiteDatabasePathRejectsWindowsAbsolutePathOnNonWindows(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("windows accepts drive-letter paths")
	}

	cleanupGeneratedWindowsDriveDir(t)

	_, err := prepareSQLiteDatabasePath("  D:/CialloClaw/data.db  ")
	if !errors.Is(err, errUnsupportedWindowsAbsolutePath) {
		t.Fatalf("expected unsupported Windows path error, got %v", err)
	}
	assertNoGeneratedWindowsDriveDir(t)
}

func TestNewServiceDoesNotCreateWindowsDriveDirectoriesOnNonWindows(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("windows accepts drive-letter paths")
	}

	cleanupGeneratedWindowsDriveDir(t)

	service := NewService(stubAdapter{databasePath: "  D:/CialloClaw/data.db  "})
	defer func() { _ = service.Close() }()

	assertNoGeneratedWindowsDriveDir(t)
}

func cleanupGeneratedWindowsDriveDir(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(filepath.Join("D:"))
}

func assertNoGeneratedWindowsDriveDir(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join("D:")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no generated windows-drive directory, got stat error %v", err)
	}
}
