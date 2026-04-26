// 该测试文件验证模型接入层行为。
package model

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

type mockToolCallingClient struct {
	response ToolCallResult
	err      error
	request  ToolCallRequest
	called   bool
}

type stubSecretSource struct {
	apiKey string
	err    error
}

type stubSecretStore struct {
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

func (s stubSecretStore) ResolveModelAPIKey(provider string) (string, error) {
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

func (m *mockToolCallingClient) GenerateText(_ context.Context, _ GenerateTextRequest) (GenerateTextResponse, error) {
	return GenerateTextResponse{}, nil
}

func (m *mockToolCallingClient) GenerateToolCalls(_ context.Context, request ToolCallRequest) (ToolCallResult, error) {
	m.called = true
	m.request = request
	if m.err != nil {
		return ToolCallResult{}, m.err
	}
	return m.response, nil
}

// TestNewServiceStoresConfig 验证NewServiceStoresConfig。
func TestNewServiceStoresConfig(t *testing.T) {
	cfg := config.ModelConfig{
		Provider:             "openai_responses",
		ModelID:              "gpt-5.4",
		Endpoint:             "https://api.openai.com/v1/responses",
		MaxToolIterations:    6,
		ContextCompressChars: 3200,
		ContextKeepRecent:    5,
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
	if service.MaxToolIterations() != 6 {
		t.Fatalf("max tool iterations mismatch: got %d", service.MaxToolIterations())
	}
	if service.ContextCompressChars() != 3200 {
		t.Fatalf("context compress chars mismatch: got %d", service.ContextCompressChars())
	}
	if service.ContextKeepRecent() != 5 {
		t.Fatalf("context keep recent mismatch: got %d", service.ContextKeepRecent())
	}
}

func TestNewServiceAppliesLoopConfigDefaults(t *testing.T) {
	service := NewService(config.ModelConfig{}, nil)

	if service.MaxToolIterations() != defaultMaxToolIterations {
		t.Fatalf("expected default max tool iterations, got %d", service.MaxToolIterations())
	}
	if service.ContextCompressChars() != defaultContextCompressChars {
		t.Fatalf("expected default compress chars, got %d", service.ContextCompressChars())
	}
	if service.ContextKeepRecent() != defaultContextKeepRecent {
		t.Fatalf("expected default keep recent, got %d", service.ContextKeepRecent())
	}
	if service.PlannerRetryBudget() != 1 {
		t.Fatalf("expected default planner retry budget, got %d", service.PlannerRetryBudget())
	}
	if service.ToolRetryBudget() != 1 {
		t.Fatalf("expected default tool retry budget, got %d", service.ToolRetryBudget())
	}
}

func TestNewServicePreservesConfiguredRetryBudgets(t *testing.T) {
	service := NewService(config.ModelConfig{PlannerRetryBudget: 3, ToolRetryBudget: 2}, nil)
	if service.PlannerRetryBudget() != 3 {
		t.Fatalf("expected configured planner retry budget, got %d", service.PlannerRetryBudget())
	}
	if service.ToolRetryBudget() != 2 {
		t.Fatalf("expected configured tool retry budget, got %d", service.ToolRetryBudget())
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

	if !reflect.DeepEqual(client.request, request) {
		t.Fatalf("request mismatch: got %+v want %+v", client.request, request)
	}

	if response.OutputText != "done" {
		t.Fatalf("output mismatch: got %q", response.OutputText)
	}
}

func TestGenerateToolCallsReturnsErrorWhenClientMissing(t *testing.T) {
	service := NewService(config.ModelConfig{}, nil)
	_, err := service.GenerateToolCalls(context.Background(), ToolCallRequest{Input: "hello"})
	if !errors.Is(err, ErrClientNotConfigured) {
		t.Fatalf("expected ErrClientNotConfigured, got %v", err)
	}
}

func TestGenerateToolCallsReturnsErrorWhenToolCallingUnsupported(t *testing.T) {
	service := NewService(config.ModelConfig{}, &mockClient{})
	_, err := service.GenerateToolCalls(context.Background(), ToolCallRequest{Input: "hello"})
	if !errors.Is(err, ErrToolCallingNotSupported) {
		t.Fatalf("expected ErrToolCallingNotSupported, got %v", err)
	}
	if service.SupportsToolCalling() {
		t.Fatal("expected plain text client not to report tool-calling support")
	}
}

func TestGenerateToolCallsDelegatesToToolCallingClient(t *testing.T) {
	client := &mockToolCallingClient{response: ToolCallResult{RequestID: "req_tool_123", ToolCalls: []ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/todo.md"}}}}}
	service := NewService(config.ModelConfig{}, client)
	request := ToolCallRequest{TaskID: "task_001", RunID: "run_001", Input: "inspect", Tools: []ToolDefinition{{Name: "read_file"}}}

	result, err := service.GenerateToolCalls(context.Background(), request)
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}
	if !client.called {
		t.Fatal("expected tool-calling client to be called")
	}
	if !reflect.DeepEqual(client.request, request) {
		t.Fatalf("request mismatch: got %+v want %+v", client.request, request)
	}
	if !service.SupportsToolCalling() {
		t.Fatal("expected tool-calling client support to be reported")
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "read_file" {
		t.Fatalf("unexpected tool call result: %+v", result)
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

func TestRegisteredProviderDescriptorsExposeStableBoundary(t *testing.T) {
	descriptors := RegisteredProviderDescriptors()
	if len(descriptors) != 1 {
		t.Fatalf("expected one supported provider descriptor, got %+v", descriptors)
	}
	if descriptors[0].Name != OpenAIResponsesProvider || !descriptors[0].SupportsToolCalling {
		t.Fatalf("unexpected provider descriptor: %+v", descriptors[0])
	}
	descriptor, ok := defaultProviderRegistry.descriptor(OpenAIResponsesProvider)
	if !ok || descriptor.Name != OpenAIResponsesProvider {
		t.Fatalf("expected registry descriptor lookup to succeed, got descriptor=%+v ok=%v", descriptor, ok)
	}
}

func TestProviderRegistrySkipsBlankNamesAndHandlesMissingProviders(t *testing.T) {
	registry := newProviderRegistry([]providerAdapter{{
		descriptor: ProviderDescriptor{Name: "  ", SupportsToolCalling: false},
		validate:   func(cfg config.ModelConfig) error { return nil },
		build:      func(cfg ServiceConfig, apiKey string) (Client, error) { return nil, nil },
	}, {
		descriptor: ProviderDescriptor{Name: OpenAIResponsesProvider, SupportsToolCalling: true},
		validate:   func(cfg config.ModelConfig) error { return nil },
		build:      func(cfg ServiceConfig, apiKey string) (Client, error) { return nil, nil },
	}})
	descriptors := registry.descriptors()
	if len(descriptors) != 1 || descriptors[0].Name != OpenAIResponsesProvider {
		t.Fatalf("expected blank descriptor names to be skipped, got %+v", descriptors)
	}
	if _, ok := registry.descriptor("missing_provider"); ok {
		t.Fatal("expected missing provider descriptor lookup to fail")
	}
	if _, ok := registry.adapter("missing_provider"); ok {
		t.Fatal("expected missing provider adapter lookup to fail")
	}
}

func TestBuildProviderClientHandlesUnsupportedAndBuilderErrors(t *testing.T) {
	if _, err := buildProviderClient(ServiceConfig{ModelConfig: config.ModelConfig{Provider: "unsupported"}}, "api-key"); !errors.Is(err, ErrModelProviderUnsupported) {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
	if _, err := buildProviderClient(ServiceConfig{ModelConfig: config.ModelConfig{Provider: OpenAIResponsesProvider, Endpoint: "", ModelID: "gpt-5.4"}}, "api-key"); !errors.Is(err, ErrOpenAIEndpointRequired) {
		t.Fatalf("expected openai endpoint required error, got %v", err)
	}
	if _, err := buildProviderClient(ServiceConfig{ModelConfig: config.ModelConfig{Provider: OpenAIResponsesProvider, Endpoint: "https://api.openai.com/v1/responses", ModelID: ""}}, "api-key"); !errors.Is(err, ErrOpenAIModelIDRequired) {
		t.Fatalf("expected openai model id required error, got %v", err)
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
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
		APIKey: "test-key",
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

// TestNewServiceFromConfigUsesServiceConfigAPIKey 验证NewServiceFromConfigUsesServiceConfigAPIKey。
func TestNewServiceFromConfigUsesServiceConfigAPIKey(t *testing.T) {
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
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
		APIKey: "model-config-key",
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
	mapped := record.Map()
	usage, _ := mapped["usage"].(map[string]any)
	if mapped["provider"] != OpenAIResponsesProvider || mapped["model_id"] != "gpt-5.4" || usage["total_tokens"] != 15 {
		t.Fatalf("expected invocation record map to preserve fields, got %+v", mapped)
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

func TestNewServiceFromConfigReturnsSecretNotFound(t *testing.T) {
	_, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider:            OpenAIResponsesProvider,
			ModelID:             "gpt-5.4",
			Endpoint:            "https://api.openai.com/v1/responses",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
		SecretSource: stubSecretSource{err: ErrSecretNotFound},
	})
	if !errors.Is(err, ErrSecretSourceFailed) || !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected missing secret error chain, got %v", err)
	}
}

func TestStaticSecretSourceUsesSecretStore(t *testing.T) {
	source := NewStaticSecretSource(stubSecretStore{apiKey: "secret-store-key"})
	apiKey, err := source.ResolveModelAPIKey(OpenAIResponsesProvider)
	if err != nil {
		t.Fatalf("ResolveModelAPIKey returned error: %v", err)
	}
	if apiKey != "secret-store-key" {
		t.Fatalf("unexpected secret key: %q", apiKey)
	}
}

func TestStaticSecretSourceFailsWithoutStore(t *testing.T) {
	source := NewStaticSecretSource(nil)
	if _, err := source.ResolveModelAPIKey(OpenAIResponsesProvider); !errors.Is(err, ErrSecretSourceFailed) {
		t.Fatalf("expected ErrSecretSourceFailed, got %v", err)
	}
}

func TestNewServiceFromConfigReturnsMissingSecretWhenNoAPIKeyProvided(t *testing.T) {
	_, err := NewServiceFromConfig(ServiceConfig{
		ModelConfig: config.ModelConfig{
			Provider:            OpenAIResponsesProvider,
			ModelID:             "gpt-5.4",
			Endpoint:            "https://api.openai.com/v1/responses",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
	})
	if !errors.Is(err, ErrSecretSourceFailed) || !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected missing secret error chain, got %v", err)
	}
}
