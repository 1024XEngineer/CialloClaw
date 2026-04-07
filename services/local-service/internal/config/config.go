package config

type ModelConfig struct {
	Provider string
	ModelID  string
	Endpoint string
}

type Config struct {
	RPCAddress    string
	WorkspaceRoot string
	DatabasePath  string
	Model         ModelConfig
}

func Load() Config {
	return Config{
		RPCAddress:    ":4317",
		WorkspaceRoot: "workspace",
		DatabasePath:  "data/cialloclaw.db",
		Model: ModelConfig{
			Provider: "openai_responses",
			ModelID:  "gpt-5.4",
			Endpoint: "https://api.openai.com/v1/responses",
		},
	}
}
