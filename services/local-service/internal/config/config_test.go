package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultRuntimePathsPreferOverrides(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	workspaceRoot := filepath.Join(t.TempDir(), "workspace-root")
	databasePath := filepath.Join(t.TempDir(), "data", "override.db")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	t.Setenv("CIALLOCLAW_WORKSPACE_ROOT", workspaceRoot)
	t.Setenv("CIALLOCLAW_DATABASE_PATH", databasePath)

	if got := DefaultRuntimeRoot(); got != runtimeRoot {
		t.Fatalf("expected runtime override %q, got %q", runtimeRoot, got)
	}
	if got := DefaultWorkspaceRoot(); got != workspaceRoot {
		t.Fatalf("expected workspace override %q, got %q", workspaceRoot, got)
	}
	if got := DefaultDatabasePath(); got != databasePath {
		t.Fatalf("expected database override %q, got %q", databasePath, got)
	}

	loaded := Load()
	if loaded.WorkspaceRoot != workspaceRoot || loaded.DatabasePath != databasePath {
		t.Fatalf("expected Load to reuse overrides, got %+v", loaded)
	}
}

func TestDefaultRuntimePathsUseLocalAppDataRoot(t *testing.T) {
	localAppData := filepath.Join(t.TempDir(), "LocalAppData")
	expectedRuntimeRoot := filepath.Join(localAppData, defaultRuntimeDirectoryName)
	if got := defaultRuntimeRootFromValues("windows", "", localAppData, filepath.Join(t.TempDir(), "home"), filepath.Join(t.TempDir(), "xdg")); got != expectedRuntimeRoot {
		t.Fatalf("expected runtime root under LOCALAPPDATA, got %q", got)
	}
	if got := filepath.Join(expectedRuntimeRoot, defaultWorkspaceDirName); got != filepath.Join(localAppData, defaultRuntimeDirectoryName, defaultWorkspaceDirName) {
		t.Fatalf("expected workspace root under LOCALAPPDATA, got %q", got)
	}
	if got := filepath.Join(expectedRuntimeRoot, "data", defaultDatabaseFileName); got != filepath.Join(localAppData, defaultRuntimeDirectoryName, "data", defaultDatabaseFileName) {
		t.Fatalf("expected database path under LOCALAPPDATA, got %q", got)
	}
}

func TestDefaultRuntimeRootFromValuesCoversFallbackOrder(t *testing.T) {
	tests := []struct {
		name           string
		goos           string
		runtimeRoot    string
		localAppData   string
		homeDir        string
		xdgDataHome    string
		expectedSuffix string
	}{
		{
			name:           "runtime override wins",
			goos:           "windows",
			runtimeRoot:    filepath.Join("C:", "runtime", "override"),
			localAppData:   filepath.Join("C:", "Users", "tester", "AppData", "Local"),
			homeDir:        filepath.Join("C:", "Users", "tester"),
			xdgDataHome:    filepath.Join("C:", "Users", "tester", "xdg-data"),
			expectedSuffix: filepath.Join("C:", "runtime", "override"),
		},
		{
			name:           "windows local app data fallback",
			goos:           "windows",
			localAppData:   filepath.Join("C:", "Users", "tester", "AppData", "Local"),
			homeDir:        filepath.Join("C:", "Users", "tester"),
			xdgDataHome:    filepath.Join("C:", "Users", "tester", "xdg-data"),
			expectedSuffix: filepath.Join("C:", "Users", "tester", "AppData", "Local", defaultRuntimeDirectoryName),
		},
		{
			name:           "macos application support fallback",
			goos:           "darwin",
			homeDir:        filepath.Join("/Users", "tester"),
			expectedSuffix: filepath.Join("/Users", "tester", "Library", "Application Support", defaultRuntimeDirectoryName),
		},
		{
			name:           "xdg data home fallback",
			goos:           "linux",
			xdgDataHome:    filepath.Join("/tmp", "xdg-home"),
			expectedSuffix: filepath.Join("/tmp", "xdg-home", defaultRuntimeDirectoryName),
		},
		{
			name:           "home directory fallback when xdg missing",
			goos:           "linux",
			homeDir:        filepath.Join("/tmp", "home"),
			expectedSuffix: filepath.Join("/tmp", "home", ".local", "share", defaultRuntimeDirectoryName),
		},
		{
			name:           "final relative fallback",
			goos:           "linux",
			expectedSuffix: defaultRuntimeDirectoryName,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := defaultRuntimeRootFromValues(test.goos, test.runtimeRoot, test.localAppData, test.homeDir, test.xdgDataHome); got != test.expectedSuffix {
				t.Fatalf("expected runtime root %q, got %q", test.expectedSuffix, got)
			}
		})
	}
}
