package tools

import "testing"

func TestSummarizeMapPreservesAuditAndTokenUsage(t *testing.T) {
	summary := summarizeMap(map[string]any{
		"provider": "openai_responses",
		"token_usage": map[string]any{
			"input_tokens":  12,
			"output_tokens": 24,
			"total_tokens":  36,
		},
		"audit_candidate": map[string]any{
			"type":    "model",
			"action":  "generate_text",
			"summary": "generate text output",
			"target":  "summarize",
			"result":  "success",
		},
		"content": "hello world",
	})

	tokenUsage, ok := summary["token_usage"].(map[string]any)
	if !ok || tokenUsage["total_tokens"] != 36 {
		t.Fatalf("expected token_usage to be preserved, got %+v", summary["token_usage"])
	}

	auditCandidate, ok := summary["audit_candidate"].(map[string]any)
	if !ok || auditCandidate["action"] != "generate_text" {
		t.Fatalf("expected audit_candidate to be preserved, got %+v", summary["audit_candidate"])
	}

	if summary["provider"] != "openai_responses" {
		t.Fatalf("expected provider to stay intact, got %+v", summary)
	}
}
