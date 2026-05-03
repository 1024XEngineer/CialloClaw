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

// ModelConfig describes the runtime model configuration.
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

// RPCConfig describes the JSON-RPC transport configuration.
type RPCConfig struct {
	Transport        string
	NamedPipeName    string
	DebugHTTPAddress string
}

// Config contains the assembled local-service runtime configuration.
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

// DefaultWorkspaceRoot resolves the canonical workspace root used by the local
// service runtime.
func DefaultWorkspaceRoot() string {
	if value := cleanPathEnv("CIALLOCLAW_WORKSPACE_ROOT"); value != "" {
		return value
	}
	return filepath.Join(DefaultRuntimeRoot(), defaultWorkspaceDirName)
}

// DefaultDatabasePath resolves the canonical SQLite database path used by the
// local service runtime.
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

// Load returns the assembled local-service configuration.
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
