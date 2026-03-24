package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the minimal agent configuration.
type Config struct {
	Provider  ProviderConfig  `yaml:"provider"`
	Agent     AgentConfig     `yaml:"agent"`
	Tools     ToolsConfig     `yaml:"tools"`
	Memory    MemoryConfig    `yaml:"memory"`
	Workspace WorkspaceConfig `yaml:"workspace"`
}

// ProviderConfig holds LLM provider settings.
type ProviderConfig struct {
	APIKey  string `yaml:"api_key"`
	APIBase string `yaml:"api_base"`
	Model   string `yaml:"model"`
}

// AgentConfig holds agent settings.
type AgentConfig struct {
	Name         string `yaml:"name"`
	SystemPrompt string `yaml:"system_prompt"`
}

// ToolsConfig holds tools settings.
type ToolsConfig struct {
	Shell struct {
		Enabled     bool   `yaml:"enabled"`
		Description string `yaml:"description"`
	} `yaml:"shell"`
}

// MemoryConfig holds memory settings.
type MemoryConfig struct {
	Type      string `yaml:"type"`
	Workspace string `yaml:"workspace"`
}

// WorkspaceConfig holds workspace settings.
type WorkspaceConfig struct {
	Path string `yaml:"path"`
}

// Load reads and parses the configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply environment variable overrides
	if apiKey := os.Getenv("API_KEY"); apiKey != "" {
		cfg.Provider.APIKey = apiKey
	}
	if apiBase := os.Getenv("API_BASE"); apiBase != "" {
		cfg.Provider.APIBase = apiBase
	}
	if model := os.Getenv("MODEL"); model != "" {
		cfg.Provider.Model = model
	}

	// Set defaults
	if cfg.Provider.Model == "" {
		cfg.Provider.Model = "gpt-3.5-turbo"
	}
	if cfg.Provider.APIBase == "" {
		cfg.Provider.APIBase = "https://api.openai.com/v1"
	}

	return &cfg, nil
}

// GetProviderName returns a friendly name based on the API base URL.
func (c *Config) GetProviderName() string {
	base := strings.ToLower(c.Provider.APIBase)
	switch {
	case strings.Contains(base, "dashscope") || strings.Contains(base, "bailian"):
		return "阿里云百炼 (Qwen)"
	case strings.Contains(base, "deepseek"):
		return "DeepSeek"
	case strings.Contains(base, "groq"):
		return "Groq"
	case strings.Contains(base, "openai"):
		return "OpenAI"
	case strings.Contains(base, "anthropic"):
		return "Anthropic"
	case strings.Contains(base, "ollama"):
		return "Ollama"
	case strings.Contains(base, "vllm"):
		return "vLLM"
	case strings.Contains(base, "moonshot") || strings.Contains(base, "kimi"):
		return "Moonshot (Kimi)"
	case strings.Contains(base, "zhipu") || strings.Contains(base, "bigmodel"):
		return "智谱 AI (GLM)"
	default:
		return "OpenAI-Compatible"
	}
}
