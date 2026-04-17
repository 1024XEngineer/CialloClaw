// 该文件负责本地服务配置结构与默认值。
package config

// ModelConfig 描述当前模块配置。
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

// RPCConfig 描述当前模块配置。
type RPCConfig struct {
	Transport        string
	NamedPipeName    string
	DebugHTTPAddress string
}

// Config 描述当前模块配置。
type Config struct {
	RPC           RPCConfig
	WorkspaceRoot string
	DatabasePath  string
	Model         ModelConfig
}

// Load 加载当前能力。
func Load() Config {
	return Config{
		RPC: RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc`,
			DebugHTTPAddress: ":4317",
		},
		WorkspaceRoot: "workspace",
		DatabasePath:  "data/cialloclaw.db",
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
