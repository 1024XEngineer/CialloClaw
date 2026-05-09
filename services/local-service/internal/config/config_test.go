package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaultsWithoutOverrides(t *testing.T) {
	cfg := Load()

	if cfg.DataDir != DefaultRuntimeRoot() {
		t.Fatalf("expected default data dir %q, got %q", DefaultRuntimeRoot(), cfg.DataDir)
	}
	if cfg.WorkspaceRoot != DefaultWorkspaceRoot() {
		t.Fatalf("expected default workspace root %q, got %q", DefaultWorkspaceRoot(), cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != DefaultDatabasePath() {
		t.Fatalf("expected default database path %q, got %q", DefaultDatabasePath(), cfg.DatabasePath)
	}
	if cfg.RPC.NamedPipeName != `\\.\pipe\cialloclaw-rpc` {
		t.Fatalf("expected default named pipe, got %q", cfg.RPC.NamedPipeName)
	}
	if cfg.RPC.DebugHTTPAddress != ":4317" {
		t.Fatalf("expected default debug http address, got %q", cfg.RPC.DebugHTTPAddress)
	}
}

func TestLoadUsesProvidedDataDirectory(t *testing.T) {
	dataDir := filepath.Join(`C:\Users`, "tester", "AppData", "Roaming", "com.cialloclaw.desktop")
	cfg := Load(LoadOptions{DataDir: dataDir})

	if cfg.DataDir != dataDir {
		t.Fatalf("expected cleaned data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if cfg.WorkspaceRoot != filepath.Join(dataDir, defaultWorkspaceDirName) {
		t.Fatalf("expected workspace root under data dir, got %q", cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != filepath.Join(dataDir, "data", defaultDatabaseFileName) {
		t.Fatalf("expected database path under data dir, got %q", cfg.DatabasePath)
	}
}

func TestLoadDataDirectoryDoesNotOverrideExplicitWorkspaceAndDatabaseEnv(t *testing.T) {
	dataDir := filepath.Join(`C:\Users`, "tester", "AppData", "Roaming", "com.cialloclaw.desktop")
	workspaceRoot := filepath.Join(t.TempDir(), "workspace-root")
	databasePath := filepath.Join(t.TempDir(), "data", "override.db")
	t.Setenv("CIALLOCLAW_WORKSPACE_ROOT", workspaceRoot)
	t.Setenv("CIALLOCLAW_DATABASE_PATH", databasePath)

	cfg := Load(LoadOptions{DataDir: dataDir})

	if cfg.DataDir != dataDir {
		t.Fatalf("expected cleaned data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if cfg.WorkspaceRoot != workspaceRoot {
		t.Fatalf("expected workspace env override %q, got %q", workspaceRoot, cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != databasePath {
		t.Fatalf("expected database env override %q, got %q", databasePath, cfg.DatabasePath)
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
	cfg := Load(LoadOptions{DebugHTTPAddress: debugHTTPAddress, DebugHTTPAddressSet: true})

	if cfg.RPC.DebugHTTPAddress != debugHTTPAddress {
		t.Fatalf("expected debug http address %q, got %q", debugHTTPAddress, cfg.RPC.DebugHTTPAddress)
	}
}

func TestLoadAllowsExplicitlyDisablingDebugHTTP(t *testing.T) {
	cfg := Load(LoadOptions{DebugHTTPAddress: "", DebugHTTPAddressSet: true})

	if cfg.RPC.DebugHTTPAddress != "" {
		t.Fatalf("expected explicit empty debug http override to disable the listener, got %q", cfg.RPC.DebugHTTPAddress)
	}
}

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
