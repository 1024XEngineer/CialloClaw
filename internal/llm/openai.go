package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cialloclaw/internal/config"
)

type Client struct {
	getConfig func() config.LLMConfig
	http      *http.Client
}

func NewClient(getConfig func() config.LLMConfig) *Client {
	return &Client{
		getConfig: getConfig,
		http: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c.getConfig().Configured()
}

func (c *Client) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	cfg := c.getConfig()
	if !cfg.Configured() {
		return "", fmt.Errorf("大模型未配置")
	}

	reqBody := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.2,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointForBaseURL(cfg.BaseURL), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("模型接口返回错误: %s", strings.TrimSpace(string(body)))
	}

	var payload chatCompletionResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("模型接口未返回可用结果")
	}
	content := payload.Choices[0].Message.Content
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value), nil
	case []any:
		var builder strings.Builder
		for _, item := range value {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				if builder.Len() > 0 {
					builder.WriteString("\n")
				}
				builder.WriteString(text)
			}
		}
		if builder.Len() > 0 {
			return strings.TrimSpace(builder.String()), nil
		}
	}
	return "", fmt.Errorf("模型接口返回了未知内容格式")
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func endpointForBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/") + "/chat/completions"
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = "/v1/chat/completions"
	case strings.HasSuffix(path, "/v1"):
		parsed.Path = path + "/chat/completions"
	default:
		parsed.Path = path + "/chat/completions"
	}
	return parsed.String()
}
