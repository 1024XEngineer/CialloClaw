package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesRelativeDefaultsWithoutOverrides(t *testing.T) {
	cfg := Load(LoadOptions{})

	if cfg.DataDir != "" {
		t.Fatalf("expected empty data dir, got %q", cfg.DataDir)
	}
	if cfg.WorkspaceRoot != "workspace" {
		t.Fatalf("expected default workspace root, got %q", cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != filepath.Join("data", "cialloclaw.db") {
		t.Fatalf("expected default database path, got %q", cfg.DatabasePath)
	}
}

func TestLoadUsesProvidedDataDirectory(t *testing.T) {
	dataDir := filepath.Join(`C:\Users`, "tester", "AppData", "Roaming", "com.cialloclaw.desktop")
	cfg := Load(LoadOptions{DataDir: dataDir})

	if cfg.DataDir != dataDir {
		t.Fatalf("expected cleaned data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if cfg.WorkspaceRoot != filepath.Join(dataDir, "workspace") {
		t.Fatalf("expected workspace root under data dir, got %q", cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != filepath.Join(dataDir, "data", "cialloclaw.db") {
		t.Fatalf("expected database path under data dir, got %q", cfg.DatabasePath)
	}
}

func TestLoadTrimsDataDirectoryOverride(t *testing.T) {
	dataDir := filepath.Join("D:", "runtime", "cialloclaw")
	cfg := Load(LoadOptions{DataDir: "  " + dataDir + "  "})

	if cfg.DataDir != dataDir {
		t.Fatalf("expected trimmed data dir %q, got %q", dataDir, cfg.DataDir)
	}
}

func TestLoadUsesProvidedNamedPipeOverride(t *testing.T) {
	pipeName := `\\.\pipe\cialloclaw-rpc-test-user`
	cfg := Load(LoadOptions{NamedPipeName: "  " + pipeName + "  "})

	if cfg.RPC.NamedPipeName != pipeName {
		t.Fatalf("expected named pipe %q, got %q", pipeName, cfg.RPC.NamedPipeName)
	}
}

func TestLoadRepairsNamedPipePathMissingLeadingSlash(t *testing.T) {
	brokenPipeName := `\.\pipe\cialloclaw-rpc-test-user`
	pipeName := `\\.\pipe\cialloclaw-rpc-test-user`
	cfg := Load(LoadOptions{NamedPipeName: brokenPipeName})

	if cfg.RPC.NamedPipeName != pipeName {
		t.Fatalf("expected repaired named pipe %q, got %q", pipeName, cfg.RPC.NamedPipeName)
	}
}

func TestLoadUsesProvidedDebugHTTPOverride(t *testing.T) {
	debugHTTPAddress := "127.0.0.1:0"
	cfg := Load(LoadOptions{DebugHTTPAddress: debugHTTPAddress})

	if cfg.RPC.DebugHTTPAddress != debugHTTPAddress {
		t.Fatalf("expected debug http address %q, got %q", debugHTTPAddress, cfg.RPC.DebugHTTPAddress)
	}
}
