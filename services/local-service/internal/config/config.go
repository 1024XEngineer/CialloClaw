// Package config defines local-service configuration defaults and runtime path
// resolution.
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultRuntimeDirectoryName = "CialloClaw"
	defaultWorkspaceDirName     = "workspace"
	defaultDatabaseFileName     = "cialloclaw.db"
)

// ModelConfig contains the provider defaults and execution budgets used before
// the settings store applies user overrides.
type ModelConfig struct {
	Provider             string
	ModelID              string
	Endpoint             string
	SingleTaskLimit      float64
	DailyLimit           float64
	BudgetAutoDowngrade  bool
	MaxToolIterations    int
	PlannerRetryBudget   int
	ToolRetryBudget      int
	ContextCompressChars int
	ContextKeepRecent    int
}

// RPCConfig contains the local JSON-RPC transport defaults. The named-pipe
// value is used by the desktop bridge, while the debug HTTP address remains an
// opt-in bootstrap surface.
type RPCConfig struct {
	Transport        string
	NamedPipeName    string
	DebugHTTPAddress string
}

// Config is the immutable bootstrap snapshot consumed by the local service.
// Runtime paths are resolved before construction so downstream packages do not
// need to read process environment variables.
type Config struct {
	RPC           RPCConfig
	WorkspaceRoot string
	DatabasePath  string
	Model         ModelConfig
}

// DefaultRuntimeRoot resolves the canonical local runtime root. The resolver
// prefers explicit environment overrides, then platform user-scoped app-data
// locations, and falls back to a relative directory only when no profile root
// is available.
func DefaultRuntimeRoot() string {
	return defaultRuntimeRootFromValues(
		runtime.GOOS,
		cleanPathEnv("CIALLOCLAW_RUNTIME_ROOT"),
		cleanPathEnv("LOCALAPPDATA"),
		cleanPathEnv("HOME"),
		cleanPathEnv("XDG_DATA_HOME"),
	)
}

// DefaultWorkspaceRoot resolves the workspace root used for controlled file
// tools and artifacts. CIALLOCLAW_WORKSPACE_ROOT takes precedence over the
// profile-scoped runtime directory.
func DefaultWorkspaceRoot() string {
	if value := cleanPathEnv("CIALLOCLAW_WORKSPACE_ROOT"); value != "" {
		return value
	}
	return filepath.Join(DefaultRuntimeRoot(), defaultWorkspaceDirName)
}

// DefaultDatabasePath resolves the SQLite database path used by storage
// bootstrap. CIALLOCLAW_DATABASE_PATH overrides the default data directory but
// does not create or validate the file.
func DefaultDatabasePath() string {
	if value := cleanPathEnv("CIALLOCLAW_DATABASE_PATH"); value != "" {
		return value
	}
	return filepath.Join(DefaultRuntimeRoot(), "data", defaultDatabaseFileName)
}

func cleanPathEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return ""
	}
	return filepath.Clean(value)
}

func defaultRuntimeRootFromValues(goos, runtimeOverride, localAppData, homeDir, xdgDataHome string) string {
	if strings.TrimSpace(runtimeOverride) != "" {
		return filepath.Clean(runtimeOverride)
	}
	if goos == "windows" && strings.TrimSpace(localAppData) != "" {
		return filepath.Join(filepath.Clean(localAppData), defaultRuntimeDirectoryName)
	}
	if goos == "darwin" && strings.TrimSpace(homeDir) != "" {
		return filepath.Join(filepath.Clean(homeDir), "Library", "Application Support", defaultRuntimeDirectoryName)
	}
	if strings.TrimSpace(xdgDataHome) != "" {
		return filepath.Join(filepath.Clean(xdgDataHome), defaultRuntimeDirectoryName)
	}
	if strings.TrimSpace(homeDir) != "" {
		return filepath.Join(filepath.Clean(homeDir), ".local", "share", defaultRuntimeDirectoryName)
	}
	return filepath.Join(defaultRuntimeDirectoryName)
}

// Load returns the built-in local-service configuration used during bootstrap.
// It reads only path-related environment overrides through the default
// resolvers and leaves validation, directory creation, and user settings
// overlay to later startup phases.
func Load() Config {
	return Config{
		RPC: RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc`,
			DebugHTTPAddress: ":4317",
		},
		WorkspaceRoot: DefaultWorkspaceRoot(),
		DatabasePath:  DefaultDatabasePath(),
		Model: ModelConfig{
			Provider:             "openai_responses",
			ModelID:              "gpt-5.4",
			Endpoint:             "https://api.openai.com/v1/responses",
			SingleTaskLimit:      10.0,
			DailyLimit:           50.0,
			BudgetAutoDowngrade:  true,
			MaxToolIterations:    4,
			PlannerRetryBudget:   1,
			ToolRetryBudget:      1,
			ContextCompressChars: 2400,
			ContextKeepRecent:    4,
		},
	}
}
