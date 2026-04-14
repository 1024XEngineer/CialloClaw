package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
)

const defaultExternalConfigPath = `E:\code\opencode\CialloClaw\config\config.yaml`

type externalModelConfig struct {
	ConfigPath string
	APIKey     string
	BaseURL    string
	Model      string
}

func loadExternalModelConfig(path string) (externalModelConfig, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return externalModelConfig{}, fmt.Errorf("external config path is required")
	}

	raw, err := os.ReadFile(trimmedPath)
	if err != nil {
		return externalModelConfig{}, fmt.Errorf("read external config: %w", err)
	}

	parsed := externalModelConfig{ConfigPath: trimmedPath}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := parseYAMLScalar(parts[1])
		switch key {
		case "api_key":
			parsed.APIKey = value
		case "base_url":
			parsed.BaseURL = value
		case "model":
			parsed.Model = value
		}
	}
	if err := scanner.Err(); err != nil {
		return externalModelConfig{}, fmt.Errorf("scan external config: %w", err)
	}

	missing := make([]string, 0, 3)
	if parsed.APIKey == "" {
		missing = append(missing, "api_key")
	}
	if parsed.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	if parsed.Model == "" {
		missing = append(missing, "model")
	}
	if len(missing) > 0 {
		return externalModelConfig{}, fmt.Errorf("external config missing fields: %s", strings.Join(missing, ", "))
	}

	return parsed, nil
}

func parseYAMLScalar(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, `"`) {
		if end := strings.LastIndex(trimmed, `"`); end > 0 {
			return strings.TrimSpace(trimmed[1:end])
		}
		return strings.TrimSpace(strings.Trim(trimmed, `"`))
	}
	if strings.HasPrefix(trimmed, `'`) {
		if end := strings.LastIndex(trimmed, `'`); end > 0 {
			return strings.TrimSpace(trimmed[1:end])
		}
		return strings.TrimSpace(strings.Trim(trimmed, `'`))
	}

	if comment := strings.Index(trimmed, " #"); comment >= 0 {
		trimmed = strings.TrimSpace(trimmed[:comment])
		return trimmed
	}
	if comment := strings.Index(trimmed, "#"); comment == 0 {
		return ""
	}

	return trimmed
}
