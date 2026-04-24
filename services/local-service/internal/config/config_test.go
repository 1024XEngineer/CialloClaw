package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesExplicitDataDirForPackagedRuntime(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "desktop-data")
	cfg := Load(LoadOptions{DataDir: dataDir})

	if cfg.DataDir != filepath.Clean(dataDir) {
		t.Fatalf("expected data dir %q, got %q", filepath.Clean(dataDir), cfg.DataDir)
	}
	if cfg.WorkspaceRoot != filepath.Join(filepath.Clean(dataDir), "workspace") {
		t.Fatalf("expected workspace root to derive from data dir, got %q", cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != filepath.Join(filepath.Clean(dataDir), "data", "cialloclaw.db") {
		t.Fatalf("expected database path to derive from data dir, got %q", cfg.DatabasePath)
	}
}

func TestLoadKeepsRepoRelativeDefaultsWithoutExplicitDataDir(t *testing.T) {
	cfg := Load(LoadOptions{})

	if cfg.DataDir != "" {
		t.Fatalf("expected empty default data dir, got %q", cfg.DataDir)
	}
	if cfg.WorkspaceRoot != "workspace" {
		t.Fatalf("expected default workspace root to stay repo-relative, got %q", cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != filepath.Join("data", "cialloclaw.db") {
		t.Fatalf("expected default database path to stay repo-relative, got %q", cfg.DatabasePath)
	}
}
