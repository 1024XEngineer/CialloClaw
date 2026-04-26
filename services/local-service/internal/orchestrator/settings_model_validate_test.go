package orchestrator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

func TestSettingsModelValidateReportsMissingAPIKey(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "validation")

	result, err := service.SettingsModelValidate(map[string]any{
		"models": map[string]any{
			"provider":       "openai",
			"base_url":       "https://api.example.com/v1",
			"model":          "gpt-4.1-mini",
			"delete_api_key": true,
		},
	})
	if err != nil {
		t.Fatalf("SettingsModelValidate returned error: %v", err)
	}
	if result["ok"] != false || result["status"] != "missing_api_key" {
		t.Fatalf("expected missing_api_key validation result, got %+v", result)
	}
	if result["message"] != "模型配置校验失败：当前模型未配置可用的 API Key，请重新输入并保存。" {
		t.Fatalf("unexpected validation message: %+v", result)
	}
	if result["canonical_provider"] != model.OpenAIResponsesProvider {
		t.Fatalf("expected canonical provider normalization, got %+v", result)
	}
}

func TestSettingsModelValidateReportsMissingBaseURLFromExplicitBlankInput(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "validation")

	result, err := service.SettingsModelValidate(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"base_url": "",
			"model":    "gpt-4.1-mini",
			"api_key":  "explicit-secret",
		},
	})
	if err != nil {
		t.Fatalf("SettingsModelValidate returned error: %v", err)
	}
	if result["ok"] != false || result["status"] != "missing_base_url" {
		t.Fatalf("expected missing_base_url validation result, got %+v", result)
	}
}

func TestSettingsModelValidateSurfacesRejectedModelErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"model_not_found"}}`))
	}))
	defer server.Close()

	service, _ := newTestServiceWithExecution(t, "validation")
	result, err := service.SettingsModelValidate(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"base_url": server.URL,
			"model":    "missing-model",
			"api_key":  "explicit-secret",
		},
	})
	if err != nil {
		t.Fatalf("SettingsModelValidate returned error: %v", err)
	}
	if result["ok"] != false || result["status"] != "request_rejected" {
		t.Fatalf("expected request_rejected validation result, got %+v", result)
	}
	if message, _ := result["message"].(string); message == "" || message == "模型配置校验失败：上游拒绝当前配置，请检查 Provider、Base URL、Model 或输入兼容性。" {
		t.Fatalf("expected upstream detail to surface in validation message, got %+v", result)
	}
}

func TestSettingsModelValidateFlagsToolCallingCompatibilityAfterTextSuccess(t *testing.T) {
	responsesCalls := 0
	chatCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/responses":
			responsesCalls++
			if responsesCalls == 1 {
				_, _ = w.Write([]byte(`{"id":"resp_text_ok","model":"gpt-4.1-mini","output_text":"OK","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"responses tool route is not available"}}`))
		case "/chat/completions":
			chatCalls++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"tool_choice is not supported"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	service, _ := newTestServiceWithExecution(t, "validation")
	result, err := service.SettingsModelValidate(map[string]any{
		"models": map[string]any{
			"provider": "z-ai",
			"base_url": server.URL,
			"model":    "z-ai/glm-5",
			"api_key":  "explicit-secret",
		},
	})
	if err != nil {
		t.Fatalf("SettingsModelValidate returned error: %v", err)
	}
	if result["ok"] != false || result["status"] != "tool_calling_unavailable" {
		t.Fatalf("expected tool_calling_unavailable validation result, got %+v", result)
	}
	if result["text_generation_ready"] != true || result["tool_calling_ready"] != false {
		t.Fatalf("expected text-only readiness markers, got %+v", result)
	}
	if responsesCalls < 2 || chatCalls != 1 {
		t.Fatalf("expected responses text probe plus chat fallback tool probe, responses=%d chat=%d", responsesCalls, chatCalls)
	}
	if result["canonical_provider"] != model.OpenAIResponsesProvider {
		t.Fatalf("expected z-ai alias to validate through canonical provider, got %+v", result)
	}
}

func TestSettingsModelValidateUsesStoredSecretWhenAPIKeyIsOmitted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_text_ok","model":"gpt-4.1-mini","output_text":"OK","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	service, _ := newTestServiceWithExecution(t, "validation")
	if err := service.persistModelSecret("openai", "stored-secret"); err != nil {
		t.Fatalf("persistModelSecret failed: %v", err)
	}
	result, err := service.SettingsModelValidate(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"base_url": server.URL,
			"model":    "gpt-4.1-mini",
		},
	})
	if err != nil {
		t.Fatalf("SettingsModelValidate returned error: %v", err)
	}
	if result["status"] == "missing_api_key" {
		t.Fatalf("expected stored secret to satisfy validation secret lookup, got %+v", result)
	}
	if result["text_generation_ready"] != true {
		t.Fatalf("expected stored secret probe to reach text generation, got %+v", result)
	}
}
