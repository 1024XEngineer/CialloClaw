package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Message represents a chat message.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Name      string     `json:"name,omitempty"`
}

// ToolCall represents a tool call from the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function to be called.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition represents a tool that can be called.
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

// ToolFunctionDefinition defines the function schema.
type ToolFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// Provider defines the interface for LLM providers.
type Provider interface {
	SendMessage(ctx context.Context, messages []Message, tools []ToolDefinition) ([]Message, error)
	GetName() string
}

// OpenAIProvider implements Provider for OpenAI-compatible APIs.
type OpenAIProvider struct {
	APIKey     string
	APIBase    string
	Model      string
	HTTPClient *http.Client
	Name       string
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
func NewOpenAIProvider(apiKey, apiBase, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	base := strings.TrimRight(apiBase, "/")
	// Determine provider name from base URL
	name := "OpenAI-Compatible"
	lower := strings.ToLower(base)
	switch {
	case strings.Contains(lower, "dashscope") || strings.Contains(lower, "bailian"):
		name = "阿里云百炼 (Qwen)"
	case strings.Contains(lower, "deepseek"):
		name = "DeepSeek"
	case strings.Contains(lower, "groq"):
		name = "Groq"
	case strings.Contains(lower, "ollama"):
		name = "Ollama"
	case strings.Contains(lower, "vllm"):
		name = "vLLM"
	case strings.Contains(lower, "moonshot") || strings.Contains(lower, "kimi"):
		name = "Moonshot (Kimi)"
	case strings.Contains(lower, "zhipu") || strings.Contains(lower, "bigmodel"):
		name = "智谱 AI (GLM)"
	}
	return &OpenAIProvider{
		APIKey:     apiKey,
		APIBase:    base,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		Name:       name,
	}
}

// GetName returns the provider name.
func (p *OpenAIProvider) GetName() string {
	return p.Name
}

// SendMessage sends messages to the LLM and handles tool calls.
func (p *OpenAIProvider) SendMessage(ctx context.Context, messages []Message, tools []ToolDefinition) ([]Message, error) {
	if p.APIBase == "" {
		return nil, fmt.Errorf("api base not configured")
	}

	// Build request body
	reqBody := map[string]any{
		"model":    p.Model,
		"messages": messages,
	}

	// Add tools if provided
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.APIBase + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		errMsg, _ := json.Marshal(body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(errMsg))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	assistantMsg := response.Choices[0].Message

	// Convert tool calls
	var toolCalls []ToolCall
	for _, tc := range assistantMsg.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return []Message{
		{
			Role:      assistantMsg.Role,
			Content:   assistantMsg.Content,
			ToolCalls: toolCalls,
		},
	}, nil
}
