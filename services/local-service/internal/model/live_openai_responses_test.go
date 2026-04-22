package model

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

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
		Input:  "Return no natural language. Only call the capture_probe tool once with JSON {\"probe\":\"pong\"}.",
		Tools: []ToolDefinition{{
			Name:        "capture_probe",
			Description: "A smoke-test probe tool that confirms OpenAI Responses tool-calling is reachable.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"probe": map[string]any{"type": "string"},
				},
				"required": []string{"probe"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}
	if result.RequestID == "" {
		t.Fatal("expected non-empty request id")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected exactly one tool call, got %+v", result.ToolCalls)
	}
	if result.ToolCalls[0].Name != "capture_probe" {
		t.Fatalf("unexpected tool name: %+v", result.ToolCalls[0])
	}
	if result.ToolCalls[0].Arguments["probe"] != "pong" {
		t.Fatalf("unexpected tool arguments: %+v", result.ToolCalls[0].Arguments)
	}
}
