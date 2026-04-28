//go:build live_openai_responses
// +build live_openai_responses

package model

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

// These tests intentionally stay behind an explicit build tag so the default CI
// suite never exercises network-bound live-provider checks. Developers can run
// them manually with `go test -tags=live_openai_responses ./internal/model` and
// the matching runtime environment variables.

func TestLiveOpenAIResponsesGenerateText(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_LIVE_OPENAI_RESPONSES_TEST")) != "1" {
		t.Skip("RUN_LIVE_OPENAI_RESPONSES_TEST is not enabled")
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	endpoint := strings.TrimSpace(os.Getenv("OPENAI_RESPONSES_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/responses"
	}

	modelID := strings.TrimSpace(os.Getenv("OPENAI_RESPONSES_MODEL"))
	if modelID == "" {
		modelID = "gpt-5.4"
	}

	service, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider: OpenAIResponsesProvider,
			ModelID:  modelID,
			Endpoint: endpoint,
		},
		APIKey: apiKey,
	})
	if err != nil {
		t.Fatalf("NewServiceFromConfig returned error: %v", err)
	}

	response, err := service.GenerateText(context.Background(), GenerateTextRequest{
		TaskID: "task_live_001",
		RunID:  "run_live_001",
		Input:  "Reply with exactly: pong",
	})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}
	if strings.TrimSpace(response.OutputText) == "" {
		t.Fatal("expected non-empty output text")
	}
	if response.RequestID == "" {
		t.Fatal("expected non-empty request id")
	}
}

func TestLiveOpenAIResponsesGenerateToolCalls(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_LIVE_OPENAI_RESPONSES_TEST")) != "1" {
		t.Skip("RUN_LIVE_OPENAI_RESPONSES_TEST is not enabled")
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	endpoint := strings.TrimSpace(os.Getenv("OPENAI_RESPONSES_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/responses"
	}

	modelID := strings.TrimSpace(os.Getenv("OPENAI_RESPONSES_MODEL"))
	if modelID == "" {
		modelID = "gpt-5.4"
	}

	service, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider: OpenAIResponsesProvider,
			ModelID:  modelID,
			Endpoint: endpoint,
		},
		APIKey: apiKey,
	})
	if err != nil {
		t.Fatalf("NewServiceFromConfig returned error: %v", err)
	}

	result, err := service.GenerateToolCalls(context.Background(), ToolCallRequest{
		TaskID: "task_live_tool_001",
		RunID:  "run_live_tool_001",
		Input:  "Use the read_file tool to inspect notes/todo.md before answering.",
		Tools: []ToolDefinition{{
			Name:        "read_file",
			Description: "Read a workspace file",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
				"required": []string{"path"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}
	if result.RequestID == "" {
		t.Fatal("expected non-empty request id")
	}
	if strings.TrimSpace(result.ModelID) == "" {
		t.Fatal("expected non-empty model id")
	}
}
