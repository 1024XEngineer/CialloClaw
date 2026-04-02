package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	AppName               string        `json:"app_name"`
	HTTPAddr              string        `json:"http_addr"`
	DataDir               string        `json:"data_dir"`
	WorkspaceDir          string        `json:"workspace_dir"`
	TodoRoots             []string      `json:"todo_roots"`
	ClipboardPollInterval time.Duration `json:"clipboard_poll_interval"`
	TodoPollInterval      time.Duration `json:"todo_poll_interval"`
	LLM                   LLMConfig     `json:"llm"`
}

type LLMConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type fileConfig struct {
	AppName               string    `json:"app_name"`
	HTTPAddr              string    `json:"http_addr"`
	DataDir               string    `json:"data_dir"`
	WorkspaceDir          string    `json:"workspace_dir"`
	TodoRoots             []string  `json:"todo_roots"`
	ClipboardPollInterval string    `json:"clipboard_poll_interval"`
	TodoPollInterval      string    `json:"todo_poll_interval"`
	LLM                   LLMConfig `json:"llm"`
}

func Load(root string) (Config, error) {
	cfg := Config{
		AppName:               "CialloClaw",
		HTTPAddr:              "127.0.0.1:8090",
		DataDir:               filepath.Join(root, ".data"),
		WorkspaceDir:          filepath.Join(root, "workspace"),
		TodoRoots:             []string{filepath.Join(root, "workspace", "todos")},
		ClipboardPollInterval: 2 * time.Second,
		TodoPollInterval:      15 * time.Second,
	}

	rawPath := filepath.Join(root, "config.json")
	data, err := os.ReadFile(rawPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, prepare(cfg)
		}
		return Config{}, err
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return Config{}, err
	}

	if fc.AppName != "" {
		cfg.AppName = fc.AppName
	}
	if fc.HTTPAddr != "" {
		cfg.HTTPAddr = fc.HTTPAddr
	}
	if fc.DataDir != "" {
		cfg.DataDir = absOrJoin(root, fc.DataDir)
	}
	if fc.WorkspaceDir != "" {
		cfg.WorkspaceDir = absOrJoin(root, fc.WorkspaceDir)
	}
	if len(fc.TodoRoots) > 0 {
		cfg.TodoRoots = cfg.TodoRoots[:0]
		for _, entry := range fc.TodoRoots {
			cfg.TodoRoots = append(cfg.TodoRoots, absOrJoin(root, entry))
		}
	}
	if fc.ClipboardPollInterval != "" {
		d, err := time.ParseDuration(fc.ClipboardPollInterval)
		if err != nil {
			return Config{}, err
		}
		cfg.ClipboardPollInterval = d
	}
	if fc.TodoPollInterval != "" {
		d, err := time.ParseDuration(fc.TodoPollInterval)
		if err != nil {
			return Config{}, err
		}
		cfg.TodoPollInterval = d
	}
	if fc.LLM.BaseURL != "" {
		cfg.LLM.BaseURL = fc.LLM.BaseURL
	}
	if fc.LLM.APIKey != "" {
		cfg.LLM.APIKey = fc.LLM.APIKey
	}
	if fc.LLM.Model != "" {
		cfg.LLM.Model = fc.LLM.Model
	}

	return cfg, prepare(cfg)
}

func Save(root string, cfg Config) error {
	if err := prepare(cfg); err != nil {
		return err
	}

	rawPath := filepath.Join(root, "config.json")
	payload := fileConfig{
		AppName:               cfg.AppName,
		HTTPAddr:              cfg.HTTPAddr,
		DataDir:               cfg.DataDir,
		WorkspaceDir:          cfg.WorkspaceDir,
		TodoRoots:             append([]string{}, cfg.TodoRoots...),
		ClipboardPollInterval: cfg.ClipboardPollInterval.String(),
		TodoPollInterval:      cfg.TodoPollInterval.String(),
		LLM:                   cfg.LLM,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rawPath, data, 0o644)
}

func prepare(cfg Config) error {
	dirs := []string{
		cfg.DataDir,
		filepath.Join(cfg.DataDir, "runtime"),
		filepath.Join(cfg.DataDir, "logs"),
		filepath.Join(cfg.DataDir, "memory"),
		filepath.Join(cfg.DataDir, "blobs"),
		cfg.WorkspaceDir,
	}
	dirs = append(dirs, cfg.TodoRoots...)
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func absOrJoin(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func (c LLMConfig) Configured() bool {
	return c.BaseURL != "" && c.APIKey != "" && c.Model != ""
}

func (c LLMConfig) MaskedAPIKey() string {
	if c.APIKey == "" {
		return ""
	}
	if len(c.APIKey) <= 8 {
		return "已设置"
	}
	return c.APIKey[:4] + "..." + c.APIKey[len(c.APIKey)-4:]
}
