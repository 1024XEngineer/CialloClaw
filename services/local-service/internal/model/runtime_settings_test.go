package model

import (
	"reflect"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

func TestCanonicalProviderNameNormalizesKnownAliases(t *testing.T) {
	tests := map[string]string{
		"openai":                OpenAIResponsesProvider,
		" openai ":              OpenAIResponsesProvider,
		OpenAIResponsesProvider: OpenAIResponsesProvider,
		"anthropic":             "anthropic",
		"":                      "",
	}
	for input, want := range tests {
		if got := CanonicalProviderName(input); got != want {
			t.Fatalf("canonical provider mismatch for %q: got %q want %q", input, got, want)
		}
	}
}

func TestRuntimeConfigFromSettingsOverlaysModelsScope(t *testing.T) {
	base := config.ModelConfig{
		Provider:             OpenAIResponsesProvider,
		ModelID:              "gpt-5.4",
		Endpoint:             "https://api.openai.com/v1/responses",
		BudgetAutoDowngrade:  true,
		MaxToolIterations:    4,
		PlannerRetryBudget:   1,
		ToolRetryBudget:      1,
		ContextCompressChars: 2400,
		ContextKeepRecent:    4,
	}
	resolved := RuntimeConfigFromSettings(base, map[string]any{
		"models": map[string]any{
			"provider":              "openai",
			"budget_auto_downgrade": false,
			"credentials": map[string]any{
				"base_url": "https://example.invalid/v1/responses",
				"model":    "gpt-4.1-mini",
				"budget_policy": map[string]any{
					"planner_retry_budget":   3,
					"tool_retry_budget":      2,
					"max_tool_iterations":    7,
					"context_compress_chars": 3200,
					"context_keep_recent":    6,
				},
			},
		},
	})

	if resolved.Provider != OpenAIResponsesProvider {
		t.Fatalf("expected provider alias to normalize, got %+v", resolved)
	}
	if resolved.Endpoint != "https://example.invalid/v1/responses" || resolved.ModelID != "gpt-4.1-mini" {
		t.Fatalf("expected runtime config to use persisted route, got %+v", resolved)
	}
	if resolved.BudgetAutoDowngrade {
		t.Fatalf("expected budget auto downgrade override to apply, got %+v", resolved)
	}
	if resolved.PlannerRetryBudget != 3 || resolved.ToolRetryBudget != 2 || resolved.MaxToolIterations != 7 || resolved.ContextCompressChars != 3200 || resolved.ContextKeepRecent != 6 {
		t.Fatalf("expected budget policy values to overlay runtime config, got %+v", resolved)
	}
}

func TestRuntimeConfigFromSettingsKeepsBootstrapDefaultsForBlankValues(t *testing.T) {
	base := config.ModelConfig{
		Provider:             OpenAIResponsesProvider,
		ModelID:              "gpt-5.4",
		Endpoint:             "https://api.openai.com/v1/responses",
		BudgetAutoDowngrade:  true,
		MaxToolIterations:    4,
		PlannerRetryBudget:   1,
		ToolRetryBudget:      1,
		ContextCompressChars: 2400,
		ContextKeepRecent:    4,
	}
	resolved := RuntimeConfigFromSettings(base, map[string]any{
		"models": map[string]any{
			"provider": "   ",
			"credentials": map[string]any{
				"base_url": "",
				"model":    " ",
			},
		},
	})

	if !reflect.DeepEqual(resolved, base) {
		t.Fatalf("expected blank persisted settings to keep bootstrap defaults, got %+v want %+v", resolved, base)
	}
}

func TestServiceRuntimeConfigReflectsCurrentLoopBudgets(t *testing.T) {
	service := NewService(config.ModelConfig{
		Provider:             OpenAIResponsesProvider,
		ModelID:              "gpt-5.4",
		Endpoint:             "https://api.openai.com/v1/responses",
		MaxToolIterations:    6,
		PlannerRetryBudget:   2,
		ToolRetryBudget:      3,
		ContextCompressChars: 3600,
		ContextKeepRecent:    5,
	})

	configSnapshot := service.RuntimeConfig()
	if configSnapshot.Provider != OpenAIResponsesProvider || configSnapshot.ModelID != "gpt-5.4" || configSnapshot.Endpoint != "https://api.openai.com/v1/responses" {
		t.Fatalf("expected runtime config identity fields to round-trip, got %+v", configSnapshot)
	}
	if configSnapshot.MaxToolIterations != 6 || configSnapshot.PlannerRetryBudget != 2 || configSnapshot.ToolRetryBudget != 3 || configSnapshot.ContextCompressChars != 3600 || configSnapshot.ContextKeepRecent != 5 {
		t.Fatalf("expected runtime config loop budgets to round-trip, got %+v", configSnapshot)
	}
}
