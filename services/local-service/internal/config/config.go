package config

import (
	"path/filepath"
	"strings"
)

// ModelConfig describes the model-side runtime settings used by the local service.
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

// RPCConfig describes the local transport endpoints exposed by the service.
type RPCConfig struct {
	Transport        string
	NamedPipeName    string
	DebugHTTPAddress string
}

// LoadOptions captures runtime path overrides coming from the desktop host.
type LoadOptions struct {
	DataDir string
}

// Config describes the resolved local-service runtime configuration.
type Config struct {
	RPC           RPCConfig
	DataDir       string
	WorkspaceRoot string
	DatabasePath  string
	Model         ModelConfig
}

// Load resolves the configuration snapshot for the current process.
func Load(options LoadOptions) Config {
	dataDir := resolveOptionalPath(options.DataDir)
	workspaceRoot := "workspace"
	databasePath := filepath.Join("data", "cialloclaw.db")
	if dataDir != "" {
		workspaceRoot = filepath.Join(dataDir, "workspace")
		databasePath = filepath.Join(dataDir, "data", "cialloclaw.db")
	}

	return Config{
		RPC: RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc`,
			DebugHTTPAddress: ":4317",
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
