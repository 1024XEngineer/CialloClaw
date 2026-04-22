package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubModelCapability struct {
	response model.GenerateTextResponse
	err      error
}

func (s stubModelCapability) GenerateText(_ context.Context, _ model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	if s.err != nil {
		return model.GenerateTextResponse{}, s.err
	}
	return s.response, nil
}

func (s stubModelCapability) Provider() string {
	if s.response.Provider != "" {
		return s.response.Provider
	}
	return "openai_responses"
}

func (s stubModelCapability) ModelID() string {
	if s.response.ModelID != "" {
		return s.response.ModelID
	}
	return "gpt-5.4"
}

func TestGenerateTextToolUsesModelOutputWhenAvailable(t *testing.T) {
	tool := NewGenerateTextTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{
		TaskID: "task_001",
		RunID:  "run_001",
		Model: stubModelCapability{
			response: model.GenerateTextResponse{
				TaskID:     "task_001",
				RunID:      "run_001",
				RequestID:  "req_001",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "generated content",
				Usage: model.TokenUsage{
					InputTokens:  11,
					OutputTokens: 17,
					TotalTokens:  28,
				},
				LatencyMS: 42,
			},
		},
	}, map[string]any{
		"prompt":        "say something",
		"fallback_text": "fallback content",
		"intent_name":   "summarize",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.RawOutput["content"] != "generated content" {
		t.Fatalf("expected generated model content, got %+v", result.RawOutput)
	}
	if result.RawOutput["fallback"] != false {
		t.Fatalf("expected non-fallback result, got %+v", result.RawOutput)
	}
	tokenUsage := result.RawOutput["token_usage"].(map[string]any)
	if tokenUsage["total_tokens"] != 28 {
		t.Fatalf("expected token usage to be preserved, got %+v", tokenUsage)
	}
}

func TestGenerateTextToolFallsBackOnModelError(t *testing.T) {
	tool := NewGenerateTextTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{
		TaskID: "task_002",
		RunID:  "run_002",
		Model:  stubModelCapability{err: errors.New("request failed")},
	}, map[string]any{
		"prompt":        "say something",
		"fallback_text": "fallback content",
		"intent_name":   "rewrite",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.RawOutput["content"] != "fallback content" {
		t.Fatalf("expected fallback content, got %+v", result.RawOutput)
	}
	if result.RawOutput["fallback"] != true {
		t.Fatalf("expected fallback result, got %+v", result.RawOutput)
	}
	if result.SummaryOutput["fallback_reason"] == nil {
		t.Fatalf("expected fallback reason to be included, got %+v", result.SummaryOutput)
	}
}

func TestGenerateTextToolMarksEmptyModelOutputAsInvalidFallback(t *testing.T) {
	tool := NewGenerateTextTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{
		TaskID: "task_003",
		RunID:  "run_003",
		Model: stubModelCapability{response: model.GenerateTextResponse{
			TaskID:    "task_003",
			RunID:     "run_003",
			RequestID: "req_003",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
		}},
	}, map[string]any{
		"prompt":        "say something",
		"fallback_text": "fallback content",
		"intent_name":   "rewrite",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["fallback_reason"] != tools.ErrToolOutputInvalid.Error() {
		t.Fatalf("expected empty model output to map to tool output invalid, got %+v", result.RawOutput)
	}
}

func TestGenerateTextToolValidateRejectsEmptyPrompt(t *testing.T) {
	tool := NewGenerateTextTool()

	if err := tool.Validate(map[string]any{"prompt": "   "}); err == nil {
		t.Fatal("expected Validate to reject empty prompt")
	}
}
