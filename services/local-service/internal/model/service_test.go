// 该测试文件验证模型接入层行为。
package model

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

// mockClient 定义当前模块的数据结构。
type mockClient struct {
	response GenerateTextResponse
	err      error
	request  GenerateTextRequest
	called   bool
}

type stubSecretSource struct {
	apiKey string
	err    error
}

func (s stubSecretSource) ResolveModelAPIKey(provider string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if provider == "" {
		return "", nil
	}
	return s.apiKey, nil
}

// GenerateText 处理当前模块的相关逻辑。
func (m *mockClient) GenerateText(_ context.Context, request GenerateTextRequest) (GenerateTextResponse, error) {
	m.called = true
	m.request = request
	if m.err != nil {
		return GenerateTextResponse{}, m.err
	}

	return m.response, nil
}

// TestNewServiceStoresConfig 验证NewServiceStoresConfig。
func TestNewServiceStoresConfig(t *testing.T) {
	cfg := config.ModelConfig{
		Provider: "openai_responses",
		ModelID:  "gpt-5.4",
		Endpoint: "https://api.openai.com/v1/responses",
	}

	service := NewService(cfg, nil)

	if service.Provider() != cfg.Provider {
		t.Fatalf("provider mismatch: got %q want %q", service.Provider(), cfg.Provider)
	}

	if service.ModelID() != cfg.ModelID {
		t.Fatalf("model id mismatch: got %q want %q", service.ModelID(), cfg.ModelID)
	}

	if service.Endpoint() != cfg.Endpoint {
		t.Fatalf("endpoint mismatch: got %q want %q", service.Endpoint(), cfg.Endpoint)
	}

	if service.Descriptor() != "openai_responses:gpt-5.4" {
		t.Fatalf("descriptor mismatch: got %q", service.Descriptor())
	}
}

// TestGenerateTextReturnsErrorWhenClientMissing 验证GenerateTextReturnsErrorWhenClientMissing。
func TestGenerateTextReturnsErrorWhenClientMissing(t *testing.T) {
	service := NewService(config.ModelConfig{}, nil)

	_, err := service.GenerateText(context.Background(), GenerateTextRequest{Input: "hello"})
	if !errors.Is(err, ErrClientNotConfigured) {
		t.Fatalf("expected ErrClientNotConfigured, got %v", err)
	}
}

// TestGenerateTextDelegatesToClient 验证GenerateTextDelegatesToClient。
func TestGenerateTextDelegatesToClient(t *testing.T) {
	client := &mockClient{
		response: GenerateTextResponse{
			RequestID:  "req_123",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "done",
		},
	}
	service := NewService(config.ModelConfig{}, client)
	request := GenerateTextRequest{
		TaskID: "task_001",
		RunID:  "run_001",
		Input:  "summarize this",
	}

	response, err := service.GenerateText(context.Background(), request)
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}

	if !client.called {
		t.Fatal("expected client to be called")
	}

	if client.request != request {
		t.Fatalf("request mismatch: got %+v want %+v", client.request, request)
	}

	if response.OutputText != "done" {
		t.Fatalf("output mismatch: got %q", response.OutputText)
	}
}

// TestValidateModelConfigRequiresProvider 验证ValidateModelConfigRequiresProvider。
func TestValidateModelConfigRequiresProvider(t *testing.T) {
	err := ValidateModelConfig(config.ModelConfig{})
	if !errors.Is(err, ErrModelProviderRequired) {
		t.Fatalf("expected ErrModelProviderRequired, got %v", err)
	}
}

// TestValidateModelConfigRejectsUnsupportedProvider 验证ValidateModelConfigRejectsUnsupportedProvider。
func TestValidateModelConfigRejectsUnsupportedProvider(t *testing.T) {
	err := ValidateModelConfig(config.ModelConfig{Provider: "unknown"})
	if !errors.Is(err, ErrModelProviderUnsupported) {
		t.Fatalf("expected ErrModelProviderUnsupported, got %v", err)
	}
}

// TestValidateModelConfigTrimsWhitespace 验证ValidateModelConfigTrimsWhitespace。
func TestValidateModelConfigTrimsWhitespace(t *testing.T) {
	err := ValidateModelConfig(config.ModelConfig{
		Provider: "  openai_responses  ",
		ModelID:  "  gpt-5.4  ",
		Endpoint: "  https://api.openai.com/v1/responses  ",
	})
	if err != nil {
		t.Fatalf("expected trimmed config to pass validation, got %v", err)
	}
}

// TestNewServiceFromConfigBuildsOpenAIClient 验证NewServiceFromConfigBuildsOpenAIClient。
func TestNewServiceFromConfigBuildsOpenAIClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_service","output_text":"service ok","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	service, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider:            OpenAIResponsesProvider,
			ModelID:             "gpt-5.4",
			Endpoint:            server.URL,
			APIKey:              "test-key",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
	})
	if err != nil {
		t.Fatalf("NewServiceFromConfig returned error: %v", err)
	}

	response, err := service.GenerateText(context.Background(), GenerateTextRequest{Input: "hello"})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}

	if response.OutputText != "service ok" {
		t.Fatalf("output mismatch: got %q", response.OutputText)
	}
}

// TestNewServiceFromConfigUsesModelConfigAPIKey 验证NewServiceFromConfigUsesModelConfigAPIKey。
func TestNewServiceFromConfigUsesModelConfigAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer model-config-key" {
			t.Fatalf("authorization header mismatch: got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_service","output_text":"service ok","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	service, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider:            OpenAIResponsesProvider,
			ModelID:             "gpt-5.4",
			Endpoint:            server.URL,
			APIKey:              "model-config-key",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
	})
	if err != nil {
		t.Fatalf("NewServiceFromConfig returned error: %v", err)
	}

	if _, err := service.GenerateText(context.Background(), GenerateTextRequest{Input: "hello"}); err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}
}

// TestGenerateTextResponseInvocationRecord 验证GenerateTextResponseInvocationRecord。
func TestGenerateTextResponseInvocationRecord(t *testing.T) {
	response := GenerateTextResponse{
		TaskID:    "task_001",
		RunID:     "run_001",
		RequestID: "req_001",
		Provider:  OpenAIResponsesProvider,
		ModelID:   "gpt-5.4",
		Usage: TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
		LatencyMS: 123,
	}

	record := response.InvocationRecord()
	if record.TaskID != "task_001" || record.RunID != "run_001" || record.RequestID != "req_001" {
		t.Fatalf("record identity mismatch: got %+v", record)
	}

	if record.Usage.TotalTokens != 15 || record.LatencyMS != 123 {
		t.Fatalf("record metrics mismatch: got %+v", record)
	}
}

// TestNewServiceFromConfigUsesSecretSourceAPIKey 验证NewServiceFromConfigUsesSecretSourceAPIKey。
func TestNewServiceFromConfigUsesSecretSourceAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-source-key" {
			t.Fatalf("authorization header mismatch: got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_secret","output_text":"secret ok","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	service, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider:            OpenAIResponsesProvider,
			ModelID:             "gpt-5.4",
			Endpoint:            server.URL,
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
		SecretSource: stubSecretSource{apiKey: "secret-source-key"},
	})
	if err != nil {
		t.Fatalf("NewServiceFromConfig returned error: %v", err)
	}

	if _, err := service.GenerateText(context.Background(), GenerateTextRequest{Input: "hello"}); err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}
}

// TestNewServiceFromConfigReturnsSecretSourceError 验证NewServiceFromConfigReturnsSecretSourceError。
func TestNewServiceFromConfigReturnsSecretSourceError(t *testing.T) {
	_, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider:            OpenAIResponsesProvider,
			ModelID:             "gpt-5.4",
			Endpoint:            "https://api.openai.com/v1/responses",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
		SecretSource: stubSecretSource{err: errors.New("secret lookup failed")},
	})
	if !errors.Is(err, ErrSecretSourceFailed) {
		t.Fatalf("expected ErrSecretSourceFailed, got %v", err)
	}
}
