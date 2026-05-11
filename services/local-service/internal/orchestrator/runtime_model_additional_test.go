package orchestrator

import (
	"context"
	"errors"
	"testing"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestRuntimeModelHelpersHandleNilService(t *testing.T) {
	var service *Service
	service.ReplaceModel(nil)
	if service.currentModel() != nil {
		t.Fatal("expected nil service currentModel to stay nil")
	}
	if service.currentModelConfig() != (serviceconfig.ModelConfig{}) {
		t.Fatalf("expected nil service currentModelConfig to be empty, got %+v", service.currentModelConfig())
	}
	if service.currentModelDescriptor() != "" {
		t.Fatalf("expected nil service descriptor to be empty, got %q", service.currentModelDescriptor())
	}
	if err := service.reloadRuntimeModelFromSettings(); err != nil {
		t.Fatalf("expected nil service reload to be ignored, got %v", err)
	}
}

func TestReloadRuntimeModelFromSettingsFallsBackToPlaceholderWhenSecretUnavailable(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "runtime model reload fallback")
	if _, _, _, _, err := service.runEngine.UpdateSettings(map[string]any{
		"models": map[string]any{
			"provider": "anthropic",
			"base_url": "https://example.invalid/v1/messages",
			"model":    "claude-3-7-sonnet",
		},
	}); err != nil {
		t.Fatalf("seed settings failed: %v", err)
	}

	if err := service.reloadRuntimeModelFromSettings(); err != nil {
		t.Fatalf("reloadRuntimeModelFromSettings returned error: %v", err)
	}
	if service.currentModel() == nil {
		t.Fatal("expected placeholder runtime model to stay installed")
	}
	configSnapshot := service.currentModelConfig()
	if configSnapshot.Provider != model.OpenAIResponsesProvider || configSnapshot.Endpoint != "https://example.invalid/v1/messages" || configSnapshot.ModelID != "claude-3-7-sonnet" {
		t.Fatalf("unexpected placeholder runtime config: %+v", configSnapshot)
	}
	if service.currentModelDescriptor() != "openai_responses:claude-3-7-sonnet" {
		t.Fatalf("unexpected runtime descriptor: %q", service.currentModelDescriptor())
	}
	if _, err := service.currentModel().GenerateText(context.Background(), model.GenerateTextRequest{Input: "hello"}); !errors.Is(err, model.ErrClientNotConfigured) {
		t.Fatalf("expected placeholder model to remain clientless, got %v", err)
	}
	if service.executor == nil || service.executor.CurrentModel() == nil {
		t.Fatal("expected execution runtime model to stay wired")
	}
	if service.executor.CurrentModel().RuntimeConfig() != configSnapshot {
		t.Fatalf("expected execution model to match orchestrator placeholder, got %+v want %+v", service.executor.CurrentModel().RuntimeConfig(), configSnapshot)
	}
}

func TestShouldFallbackRuntimeModelReloadRecognizesSupportedFailureKinds(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "unsupported provider", err: model.ErrModelProviderUnsupported, want: true},
		{name: "secret source failed", err: model.ErrSecretSourceFailed, want: true},
		{name: "secret not found", err: storage.ErrSecretNotFound, want: true},
		{name: "secret store access failed", err: storage.ErrSecretStoreAccessFailed, want: true},
		{name: "stronghold unavailable", err: storage.ErrStrongholdUnavailable, want: true},
		{name: "stronghold access failed", err: storage.ErrStrongholdAccessFailed, want: true},
		{name: "unrelated", err: errors.New("boom"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldFallbackRuntimeModelReload(test.err); got != test.want {
				t.Fatalf("unexpected fallback result: got %v want %v", got, test.want)
			}
		})
	}
}
