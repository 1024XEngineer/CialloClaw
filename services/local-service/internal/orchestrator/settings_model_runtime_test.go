package orchestrator

import (
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

func TestSettingsUpdateMarksModelChangesNextTaskEffectiveAndReloadsRuntimeModel(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings model reload")

	result, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"base_url": "https://example.invalid/v1/responses",
			"model":    "gpt-4.1-mini",
		},
	})
	if err != nil {
		t.Fatalf("settings update failed: %v", err)
	}
	if result["apply_mode"] != "next_task_effective" || result["need_restart"] != false {
		t.Fatalf("expected model settings update to be next_task_effective, got %+v", result)
	}
	if service.currentModel() == nil {
		t.Fatal("expected orchestrator model to stay wired")
	}
	runtimeModel := service.currentModel().RuntimeConfig()
	if runtimeModel.Provider != model.OpenAIResponsesProvider || runtimeModel.Endpoint != "https://example.invalid/v1/responses" || runtimeModel.ModelID != "gpt-4.1-mini" {
		t.Fatalf("expected orchestrator runtime model to rebuild from settings, got %+v", runtimeModel)
	}
	if service.executor == nil || service.executor.CurrentModel() == nil {
		t.Fatal("expected execution model to stay wired")
	}
	executionModel := service.executor.CurrentModel().RuntimeConfig()
	if executionModel != runtimeModel {
		t.Fatalf("expected execution runtime model to match orchestrator model, got execution=%+v orchestrator=%+v", executionModel, runtimeModel)
	}
	runtimeSettings := service.runEngine.Settings()
	models := runtimeSettings["models"].(map[string]any)
	if models["provider"] != "openai" {
		t.Fatalf("expected persisted settings provider to keep control-panel alias, got %+v", models)
	}
}

func TestSettingsUpdateRoutesArbitraryProviderAliasesThroughOpenAIResponsesRuntime(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings arbitrary provider alias")

	result, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider": "anthropic",
			"base_url": "https://example.invalid/v1/messages",
			"model":    "claude-3-7-sonnet",
		},
	})
	if err != nil {
		t.Fatalf("settings update failed: %v", err)
	}
	if result["apply_mode"] != "next_task_effective" || result["need_restart"] != false {
		t.Fatalf("expected arbitrary provider alias update to stay next_task_effective, got %+v", result)
	}
	runtimeModel := service.currentModel()
	if runtimeModel == nil {
		t.Fatal("expected runtime model to stay installed")
	}
	configSnapshot := runtimeModel.RuntimeConfig()
	if configSnapshot.Provider != model.OpenAIResponsesProvider || configSnapshot.Endpoint != "https://example.invalid/v1/messages" || configSnapshot.ModelID != "claude-3-7-sonnet" {
		t.Fatalf("expected arbitrary provider alias to normalize into openai runtime config, got %+v", configSnapshot)
	}
	runtimeSettings := service.runEngine.Settings()
	models := runtimeSettings["models"].(map[string]any)
	if models["provider"] != "anthropic" {
		t.Fatalf("expected persisted settings provider to keep original alias, got %+v", models)
	}
}

func TestSettingsUpdateMapsZAIProviderAliasToOpenAIResponsesRuntime(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings z-ai alias runtime")

	result, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider": "z-ai",
			"base_url": "https://api.qnaigc.com/v1",
			"model":    "z-ai/glm-5",
		},
	})
	if err != nil {
		t.Fatalf("settings update failed: %v", err)
	}
	if result["apply_mode"] != "next_task_effective" || result["need_restart"] != false {
		t.Fatalf("expected z-ai alias update to be next_task_effective, got %+v", result)
	}
	runtimeModel := service.currentModel()
	if runtimeModel == nil {
		t.Fatal("expected runtime model to stay wired")
	}
	configSnapshot := runtimeModel.RuntimeConfig()
	if configSnapshot.Provider != model.OpenAIResponsesProvider || configSnapshot.Endpoint != "https://api.qnaigc.com/v1" || configSnapshot.ModelID != "z-ai/glm-5" {
		t.Fatalf("expected z-ai alias to normalize into openai runtime config, got %+v", configSnapshot)
	}
	runtimeSettings := service.runEngine.Settings()
	models := runtimeSettings["models"].(map[string]any)
	if models["provider"] != "z-ai" {
		t.Fatalf("expected persisted settings provider to keep z-ai alias, got %+v", models)
	}
}
