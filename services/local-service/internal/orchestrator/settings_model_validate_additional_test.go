package orchestrator

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

func TestBuildSettingsModelValidationProbeUsesMergedSettingsAndDeleteFlag(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "validation probe")
	if _, _, _, _, err := service.runEngine.UpdateSettings(map[string]any{
		"models": map[string]any{
			"provider": "anthropic",
		},
	}); err != nil {
		t.Fatalf("seed settings failed: %v", err)
	}

	probe := service.buildSettingsModelValidationProbe(map[string]any{
		"models": map[string]any{
			"base_url":       " https://example.invalid/v1/messages ",
			"model":          " claude-3-7-sonnet ",
			"delete_api_key": true,
		},
	})

	if probe.provider != "anthropic" {
		t.Fatalf("expected persisted provider alias to be preserved, got %+v", probe)
	}
	if probe.canonicalProvider != model.OpenAIResponsesProvider {
		t.Fatalf("expected canonical provider normalization, got %+v", probe)
	}
	if probe.baseURL != "https://example.invalid/v1/messages" || probe.modelID != "claude-3-7-sonnet" {
		t.Fatalf("expected string overrides to be trimmed, got %+v", probe)
	}
	if probe.useSecretSource {
		t.Fatalf("expected delete_api_key probe not to reuse stored secret, got %+v", probe)
	}
}

func TestStringSettingOverrideCoversNilMissingNonStringAndTrimmedValues(t *testing.T) {
	if value, ok := stringSettingOverride(nil, "provider"); ok || value != "" {
		t.Fatalf("expected nil map to return no override, got value=%q ok=%v", value, ok)
	}
	if value, ok := stringSettingOverride(map[string]any{"other": "value"}, "provider"); ok || value != "" {
		t.Fatalf("expected missing key to return no override, got value=%q ok=%v", value, ok)
	}
	if value, ok := stringSettingOverride(map[string]any{"provider": 7}, "provider"); !ok || value != "" {
		t.Fatalf("expected non-string override to be treated as explicit blank, got value=%q ok=%v", value, ok)
	}
	if value, ok := stringSettingOverride(map[string]any{"provider": "  openai  "}, "provider"); !ok || value != "openai" {
		t.Fatalf("expected string override to trim whitespace, got value=%q ok=%v", value, ok)
	}
}

func TestSettingsModelValidateReportsSuccessfulTextAndToolProbe(t *testing.T) {
	responsesCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		responsesCalls++
		if responsesCalls == 1 {
			_, _ = w.Write([]byte(`{"id":"resp_text_ok","model":"gpt-4.1-mini","output_text":"OK","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"resp_tool_ok","model":"gpt-4.1-mini","output":[{"type":"function_call","name":"validation_echo","call_id":"call_001","arguments":"{\"value\":\"ok\"}"}],"usage":{"input_tokens":2,"output_tokens":2,"total_tokens":4}}`))
	}))
	defer server.Close()

	service, _ := newTestServiceWithExecution(t, "validation success")
	result, err := service.SettingsModelValidate(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"base_url": server.URL,
			"model":    "gpt-4.1-mini",
			"api_key":  "explicit-secret",
		},
	})
	if err != nil {
		t.Fatalf("SettingsModelValidate returned error: %v", err)
	}
	if result["ok"] != true || result["status"] != "valid" {
		t.Fatalf("expected successful validation result, got %+v", result)
	}
	if result["text_generation_ready"] != true || result["tool_calling_ready"] != true {
		t.Fatalf("expected readiness markers after both probes, got %+v", result)
	}
	if responsesCalls != 2 {
		t.Fatalf("expected one text probe plus one tool probe, got %d calls", responsesCalls)
	}
	if result["canonical_provider"] != model.OpenAIResponsesProvider {
		t.Fatalf("expected canonical provider normalization, got %+v", result)
	}
}

func TestClassifySettingsModelValidationFailureCoversTypedBranches(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status string
	}{
		{name: "missing provider", err: model.ErrModelProviderRequired, status: "missing_provider"},
		{name: "missing endpoint", err: model.ErrOpenAIEndpointRequired, status: "missing_base_url"},
		{name: "missing model", err: model.ErrOpenAIModelIDRequired, status: "missing_model"},
		{name: "missing api key", err: errors.Join(model.ErrSecretSourceFailed, model.ErrSecretNotFound), status: "missing_api_key"},
		{name: "secret store unavailable", err: errors.Join(model.ErrSecretSourceFailed, errors.New("store down")), status: "secret_store_unavailable"},
		{name: "client not configured", err: model.ErrClientNotConfigured, status: "missing_api_key"},
		{name: "tool calling unsupported", err: model.ErrToolCallingNotSupported, status: "tool_calling_unavailable"},
		{name: "invalid response", err: model.ErrOpenAIResponseInvalid, status: "invalid_response"},
		{name: "timeout", err: model.ErrOpenAIRequestTimeout, status: "request_timeout"},
		{name: "request failed", err: model.ErrOpenAIRequestFailed, status: "request_failed"},
		{name: "unknown", err: errors.New("boom"), status: "unknown_error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, message := classifySettingsModelValidationFailure(test.err)
			if status != test.status {
				t.Fatalf("unexpected status: got %q want %q", status, test.status)
			}
			if message == "" {
				t.Fatalf("expected non-empty message for %s", test.name)
			}
		})
	}
}

func TestClassifySettingsModelHTTPStatusFailureCoversStatusMatrix(t *testing.T) {
	tests := []struct {
		name    string
		err     *model.OpenAIHTTPStatusError
		status  string
		message string
	}{
		{name: "nil", err: nil, status: "unknown_error", message: "暂时无法验证当前模型配置"},
		{name: "tool choice rejected", err: &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "tool_choice is not supported"}, status: "tool_calling_unavailable", message: "不支持工具调用"},
		{name: "bad request with detail", err: &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "model_not_found"}, status: "request_rejected", message: "model_not_found"},
		{name: "bad request generic", err: &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "Authorization: Bearer sk-secret-value invalid"}, status: "request_rejected", message: "请检查 Provider、Base URL、Model 或输入兼容性"},
		{name: "auth with secret redacted", err: &model.OpenAIHTTPStatusError{StatusCode: 401, Message: "Authorization: Bearer sk-secret-value invalid"}, status: "auth_failed", message: "鉴权失败，请检查 API Key"},
		{name: "missing endpoint detail", err: &model.OpenAIHTTPStatusError{StatusCode: 404, Message: "missing route"}, status: "endpoint_not_found", message: "missing route"},
		{name: "missing endpoint generic", err: &model.OpenAIHTTPStatusError{StatusCode: 404, Message: ""}, status: "endpoint_not_found", message: "请检查 Base URL 与 Model"},
		{name: "too many requests", err: &model.OpenAIHTTPStatusError{StatusCode: 429, Message: "rate limited"}, status: "request_rejected", message: "rate limited"},
		{name: "too many requests generic", err: &model.OpenAIHTTPStatusError{StatusCode: 429, Message: ""}, status: "request_rejected", message: "模型请求过于频繁，请稍后重试"},
		{name: "gateway timeout", err: &model.OpenAIHTTPStatusError{StatusCode: 504, Message: "gateway timeout"}, status: "request_timeout", message: "请求超时"},
		{name: "server failure", err: &model.OpenAIHTTPStatusError{StatusCode: 500, Message: "upstream failed"}, status: "request_rejected", message: "upstream failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, message := classifySettingsModelHTTPStatusFailure(test.err)
			if status != test.status {
				t.Fatalf("unexpected status: got %q want %q", status, test.status)
			}
			if !strings.Contains(message, test.message) {
				t.Fatalf("expected message %q to contain %q", message, test.message)
			}
		})
	}
}
