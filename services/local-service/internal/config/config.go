// 该文件负责本地服务配置结构与默认值。
package config

type ModelConfig struct {
	Provider string
	ModelID  string
	Endpoint string
}

type RPCConfig struct {
	Transport        string
	NamedPipeName    string
	DebugHTTPAddress string
}

type Config struct {
	RPC           RPCConfig
	WorkspaceRoot string
	DatabasePath  string
	Model         ModelConfig
}

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
			Provider: "openai_responses",
			ModelID:  "gpt-5.4",
			Endpoint: "https://api.openai.com/v1/responses",
		},
	}
}
