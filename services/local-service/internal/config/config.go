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

type RPCConfig struct {
	Transport        string
	NamedPipeName    string
	DebugHTTPAddress string
}

type LoadOptions struct {
	DataDir          string
	NamedPipeName    string
	DebugHTTPAddress string
}

type Config struct {
	RPC           RPCConfig
	DataDir       string
	WorkspaceRoot string
	DatabasePath  string
	Model         ModelConfig
}

func DefaultRuntimeRoot() string {
	return defaultRuntimeRootFromValues(
		runtime.GOOS,
		cleanPathEnv("CIALLOCLAW_RUNTIME_ROOT"),
		cleanPathEnv("LOCALAPPDATA"),
		cleanPathEnv("HOME"),
		cleanPathEnv("XDG_DATA_HOME"),
	)
}

func DefaultWorkspaceRoot() string {
	if value := cleanPathEnv("CIALLOCLAW_WORKSPACE_ROOT"); value != "" {
		return value
	}
	return filepath.Join(DefaultRuntimeRoot(), defaultWorkspaceDirName)
}

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

func Load(options ...LoadOptions) Config {
	loadOptions := LoadOptions{}
	if len(options) > 0 {
		loadOptions = options[0]
	}

	dataDir := resolveOptionalPath(loadOptions.DataDir)
	if dataDir == "" {
		dataDir = DefaultRuntimeRoot()
	}

	namedPipeName := resolveOptionalPipeName(loadOptions.NamedPipeName)
	if namedPipeName == "" {
		namedPipeName = `\\.\pipe\cialloclaw-rpc`
	}

	debugHTTPAddress := resolveOptionalDebugHTTPAddress(loadOptions.DebugHTTPAddress)
	if debugHTTPAddress == "" {
		debugHTTPAddress = ":4317"
	}

	workspaceRoot := DefaultWorkspaceRoot()
	databasePath := DefaultDatabasePath()
	if strings.TrimSpace(loadOptions.DataDir) != "" {
		workspaceRoot = filepath.Join(dataDir, defaultWorkspaceDirName)
		databasePath = filepath.Join(dataDir, "data", defaultDatabaseFileName)
	}

	return Config{
		RPC: RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    namedPipeName,
			DebugHTTPAddress: debugHTTPAddress,
		},
		DataDir:       dataDir,
		WorkspaceRoot: workspaceRoot,
		DatabasePath:  databasePath,
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

func resolveOptionalPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	return filepath.Clean(trimmed)
}

func resolveOptionalPipeName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, `\\.\pipe\`) {
		return trimmed
	}

	if strings.HasPrefix(trimmed, `\.\pipe\`) {
		return `\` + trimmed
	}

	return trimmed
}

func resolveOptionalDebugHTTPAddress(raw string) string {
	return strings.TrimSpace(raw)
}
