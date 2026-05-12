package model

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOpenAIResponsesClientRequiresAPIKey(t *testing.T) {
	_, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		Endpoint: "https://api.openai.com/v1/responses",
		ModelID:  "gpt-5.4",
	})
	if !errors.Is(err, ErrOpenAIAPIKeyRequired) {
		t.Fatalf("expected ErrOpenAIAPIKeyRequired, got %v", err)
	}
}

func TestNewOpenAIResponsesClientRequiresEndpoint(t *testing.T) {
	_, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:  "test-key",
		ModelID: "gpt-5.4",
	})
	if !errors.Is(err, ErrOpenAIEndpointRequired) {
		t.Fatalf("expected ErrOpenAIEndpointRequired, got %v", err)
	}
}

func TestNewOpenAIResponsesClientRequiresModelID(t *testing.T) {
	_, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:   "test-key",
		Endpoint: "https://api.openai.com/v1/responses",
	})
	if !errors.Is(err, ErrOpenAIModelIDRequired) {
		t.Fatalf("expected ErrOpenAIModelIDRequired, got %v", err)
	}
}

func TestNewOpenAIResponsesClientUsesProvidedConfig(t *testing.T) {
	customHTTPClient := &http.Client{}
	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   "https://api.openai.com/v1/responses",
		ModelID:    "gpt-5.4",
		Timeout:    5 * time.Second,
		HTTPClient: customHTTPClient,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	if client.Provider() != OpenAIResponsesProvider {
		t.Fatalf("provider mismatch: got %q", client.Provider())
	}

	if client.Endpoint() != "https://api.openai.com/v1/responses" {
		t.Fatalf("endpoint mismatch: got %q", client.Endpoint())
	}

	if client.ModelID() != "gpt-5.4" {
		t.Fatalf("model id mismatch: got %q", client.ModelID())
	}

	if client.httpClient == customHTTPClient {
		t.Fatal("expected custom http client clone to be used")
	}

	if client.httpClient.Timeout != 5*time.Second {
		t.Fatalf("timeout mismatch: got %v", client.httpClient.Timeout)
	}
}

func TestNewOpenAIResponsesClientUsesDefaultHTTPClient(t *testing.T) {
	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:   "test-key",
		Endpoint: "https://api.openai.com/v1/responses",
		ModelID:  "gpt-5.4",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	if client.httpClient == http.DefaultClient {
		t.Fatal("expected dedicated default-timeout client, got shared default client")
	}

	if client.httpClient.Timeout != defaultOpenAIResponsesTimeout {
		t.Fatalf("default timeout mismatch: got %v", client.httpClient.Timeout)
	}

	if client.timeout != defaultOpenAIResponsesTimeout {
		t.Fatalf("client timeout mismatch: got %v", client.timeout)
	}
}

func TestNewOpenAIResponsesClientPreservesExistingHTTPClientTimeout(t *testing.T) {
	customHTTPClient := &http.Client{Timeout: 2 * time.Second}
	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   "https://api.openai.com/v1/responses",
		ModelID:    "gpt-5.4",
		Timeout:    5 * time.Second,
		HTTPClient: customHTTPClient,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	if client.httpClient.Timeout != 2*time.Second {
		t.Fatalf("expected existing timeout to be preserved, got %v", client.httpClient.Timeout)
	}
}

func TestGenerateTextSuccess(t *testing.T) {
	type capturedRequest struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}

	var receivedAuthHeader string
	var receivedContentType string
	var receivedRequest capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		receivedAuthHeader = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		if err := json.Unmarshal(body, &receivedRequest); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","model":"gpt-5.4","output_text":"hello world","usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}`))
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		ModelID:    "gpt-5.4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	response, err := client.GenerateText(context.Background(), GenerateTextRequest{
		TaskID: "task_001",
		RunID:  "run_001",
		Input:  "say hello",
	})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}

	if receivedAuthHeader != "Bearer test-key" {
		t.Fatalf("authorization header mismatch: got %q", receivedAuthHeader)
	}

	if receivedContentType != "application/json" {
		t.Fatalf("content type mismatch: got %q", receivedContentType)
	}

	if receivedRequest.Model != "gpt-5.4" {
		t.Fatalf("request model mismatch: got %q", receivedRequest.Model)
	}

	if receivedRequest.Input != "say hello" {
		t.Fatalf("request input mismatch: got %q", receivedRequest.Input)
	}

	if response.RequestID != "resp_123" {
		t.Fatalf("request id mismatch: got %q", response.RequestID)
	}

	if response.TaskID != "task_001" {
		t.Fatalf("task id mismatch: got %q", response.TaskID)
	}

	if response.RunID != "run_001" {
		t.Fatalf("run id mismatch: got %q", response.RunID)
	}

	if response.Provider != OpenAIResponsesProvider {
		t.Fatalf("provider mismatch: got %q", response.Provider)
	}

	if response.ModelID != "gpt-5.4" {
		t.Fatalf("model id mismatch: got %q", response.ModelID)
	}

	if response.OutputText != "hello world" {
		t.Fatalf("output text mismatch: got %q", response.OutputText)
	}

	if response.Usage.InputTokens != 11 || response.Usage.OutputTokens != 7 || response.Usage.TotalTokens != 18 {
		t.Fatalf("usage mismatch: got %+v", response.Usage)
	}

	if response.LatencyMS < 0 {
		t.Fatalf("latency must be non-negative: got %d", response.LatencyMS)
	}

	record := response.InvocationRecord()
	if record.TaskID != "task_001" || record.RunID != "run_001" || record.RequestID != "resp_123" {
		t.Fatalf("invocation record mismatch: got %+v", record)
	}
}

func TestGenerateTextFallsBackToOutputContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_456","output":[{"content":[{"type":"output_text","text":"fallback text"}]}]}`))
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		ModelID:    "gpt-5.4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	response, err := client.GenerateText(context.Background(), GenerateTextRequest{Input: "fallback"})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}

	if response.OutputText != "fallback text" {
		t.Fatalf("fallback output mismatch: got %q", response.OutputText)
	}
}

func TestGenerateToolCallsSuccess(t *testing.T) {
	type capturedRequest struct {
		Model      string        `json:"model"`
		Input      string        `json:"input"`
		Tools      []interface{} `json:"tools"`
		ToolChoice string        `json:"tool_choice"`
	}

	var received capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"resp_tool_123",
			"model":"gpt-5.4",
			"output_text":"",
			"output":[
				{
					"type":"function_call",
					"name":"read_file",
					"call_id":"call_001",
					"arguments":"{\"path\":\"notes/todo.md\"}"
				}
			],
			"usage":{"input_tokens":21,"output_tokens":9,"total_tokens":30}
		}`))
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		ModelID:    "gpt-5.4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	result, err := client.GenerateToolCalls(context.Background(), ToolCallRequest{
		TaskID: "task_001",
		RunID:  "run_001",
		Input:  "Please inspect the workspace note before answering.",
		Tools: []ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a workspace file",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
					"required": []string{"path"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}

	if received.Model != "gpt-5.4" {
		t.Fatalf("request model mismatch: got %q", received.Model)
	}
	if received.Input != "Please inspect the workspace note before answering." {
		t.Fatalf("request input mismatch: got %q", received.Input)
	}
	if received.ToolChoice != "auto" {
		t.Fatalf("tool choice mismatch: got %q", received.ToolChoice)
	}
	if len(received.Tools) != 1 {
		t.Fatalf("expected one tool definition, got %d", len(received.Tools))
	}
	toolMap, ok := received.Tools[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tool payload to be an object, got %+v", received.Tools[0])
	}
	parameters, ok := toolMap["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("expected parameters object, got %+v", toolMap["parameters"])
	}
	properties, ok := parameters["properties"].(map[string]any)
	if !ok || properties == nil {
		t.Fatalf("expected properties object in serialized schema, got %+v", parameters)
	}
	pathSchema, ok := properties["path"].(map[string]any)
	if !ok || pathSchema["type"] != "string" {
		t.Fatalf("expected path property schema, got %+v", properties["path"])
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %+v", result.ToolCalls)
	}
	if result.ToolCalls[0].Name != "read_file" {
		t.Fatalf("unexpected tool name: %+v", result.ToolCalls[0])
	}
	if result.ToolCalls[0].Arguments["path"] != "notes/todo.md" {
		t.Fatalf("unexpected tool arguments: %+v", result.ToolCalls[0].Arguments)
	}
	if result.RequestID != "resp_tool_123" {
		t.Fatalf("request id mismatch: got %q", result.RequestID)
	}
}

func TestGenerateToolCallsNormalizesNestedNullToolSchemaNodes(t *testing.T) {
	type capturedRequest struct {
		Tools []interface{} `json:"tools"`
	}

	var received capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"resp_tool_schema_123",
			"model":"gpt-5.4",
			"output_text":"",
			"output":[],
			"usage":{"input_tokens":10,"output_tokens":0,"total_tokens":10}
		}`))
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		ModelID:    "gpt-5.4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	_, err = client.GenerateToolCalls(context.Background(), ToolCallRequest{
		TaskID: "task_schema_001",
		RunID:  "run_schema_001",
		Input:  "Attach the current browser tab before summarizing it.",
		Tools: []ToolDefinition{
			{
				Name:        "browser_attach_current",
				Description: "Attach the current browser tab",
				InputSchema: map[string]any{
					"type":                 "object",
					"properties":           nil,
					"contains":             nil,
					"not":                  nil,
					"if":                   nil,
					"then":                 nil,
					"else":                 nil,
					"additionalProperties": false,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}

	if len(received.Tools) != 1 {
		t.Fatalf("expected one tool definition, got %d", len(received.Tools))
	}
	toolMap, ok := received.Tools[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tool payload to be an object, got %+v", received.Tools[0])
	}
	parameters, ok := toolMap["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("expected parameters object, got %+v", toolMap["parameters"])
	}
	properties, ok := parameters["properties"].(map[string]any)
	if !ok || properties == nil || len(properties) != 0 {
		t.Fatalf("expected browser attach schema properties to serialize as an empty object, got %+v", parameters["properties"])
	}
	if _, ok := parameters["contains"]; ok {
		t.Fatalf("expected contains:nil to be omitted from serialized schema, got %+v", parameters)
	}
	if _, ok := parameters["not"]; ok {
		t.Fatalf("expected not:nil to be omitted from serialized schema, got %+v", parameters)
	}
	if _, ok := parameters["if"]; ok {
		t.Fatalf("expected if:nil to be omitted from serialized schema, got %+v", parameters)
	}
	if _, ok := parameters["then"]; ok {
		t.Fatalf("expected then:nil to be omitted from serialized schema, got %+v", parameters)
	}
	if _, ok := parameters["else"]; ok {
		t.Fatalf("expected else:nil to be omitted from serialized schema, got %+v", parameters)
	}
}

func TestGenerateTextFallsBackToChatCompletionsWhenResponsesRouteIsMissing(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/responses":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"responses route is not available"}}`))
		case "/chat/completions":
			_, _ = w.Write([]byte(`{"id":"chatcmpl_123","model":"z-ai/glm-5","choices":[{"message":{"content":"fallback hello"}}],"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL + "/responses",
		ModelID:    "z-ai/glm-5",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	response, err := client.GenerateText(context.Background(), GenerateTextRequest{TaskID: "task_compat_text", RunID: "run_compat_text", Input: "hello"})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}
	if response.OutputText != "fallback hello" {
		t.Fatalf("expected fallback text output, got %+v", response)
	}
	if len(requests) != 2 || requests[0] != "/responses" || requests[1] != "/chat/completions" {
		t.Fatalf("expected responses then chat completions fallback, got %+v", requests)
	}
}

func TestGenerateToolCallsFallBackToChatCompletionsWhenResponsesRouteIsMissing(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/responses":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"responses route is not available"}}`))
		case "/chat/completions":
			_, _ = w.Write([]byte(`{"id":"chatcmpl_tool_123","model":"z-ai/glm-5","choices":[{"message":{"content":"","tool_calls":[{"id":"call_001","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"notes/todo.md\"}"}}]}}],"usage":{"prompt_tokens":20,"completion_tokens":8,"total_tokens":28}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL + "/responses",
		ModelID:    "z-ai/glm-5",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	result, err := client.GenerateToolCalls(context.Background(), ToolCallRequest{
		TaskID: "task_compat_tool",
		RunID:  "run_compat_tool",
		Input:  "Please inspect the workspace note before answering.",
		Tools: []ToolDefinition{{
			Name:        "read_file",
			Description: "Read a workspace file",
			InputSchema: map[string]any{"type": "object"},
		}},
	})
	if err != nil {
		t.Fatalf("GenerateToolCalls returned error: %v", err)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "read_file" || result.ToolCalls[0].Arguments["path"] != "notes/todo.md" {
		t.Fatalf("expected fallback tool call, got %+v", result.ToolCalls)
	}
	if len(requests) != 2 || requests[0] != "/responses" || requests[1] != "/chat/completions" {
		t.Fatalf("expected responses then chat completions fallback, got %+v", requests)
	}
}

func TestGenerateTextReturnsHTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid request"}}`))
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		ModelID:    "gpt-5.4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	_, err = client.GenerateText(context.Background(), GenerateTextRequest{Input: "bad"})
	if !errors.Is(err, ErrOpenAIHTTPStatus) {
		t.Fatalf("expected ErrOpenAIHTTPStatus, got %v", err)
	}

	var statusErr *OpenAIHTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected OpenAIHTTPStatusError, got %T", err)
	}

	if statusErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status code mismatch: got %d", statusErr.StatusCode)
	}
}

func TestGenerateTextReturnsHTTPStatusErrorForNonJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal upstream error"))
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		ModelID:    "gpt-5.4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	_, err = client.GenerateText(context.Background(), GenerateTextRequest{Input: "bad"})
	if !errors.Is(err, ErrOpenAIHTTPStatus) {
		t.Fatalf("expected ErrOpenAIHTTPStatus, got %v", err)
	}

	var statusErr *OpenAIHTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected OpenAIHTTPStatusError, got %T", err)
	}

	if statusErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status code mismatch: got %d", statusErr.StatusCode)
	}

	if strings.TrimSpace(statusErr.Message) == "" {
		t.Fatalf("expected non-empty status message, got %q", statusErr.Message)
	}
}

func TestGenerateTextReturnsInvalidResponseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:     "test-key",
		Endpoint:   server.URL,
		ModelID:    "gpt-5.4",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	_, err = client.GenerateText(context.Background(), GenerateTextRequest{Input: "bad-json"})
	if !errors.Is(err, ErrOpenAIResponseInvalid) {
		t.Fatalf("expected ErrOpenAIResponseInvalid, got %v", err)
	}
}

func TestGenerateTextReturnsTimeoutError(t *testing.T) {
	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:   "test-key",
		Endpoint: "https://example.test/responses",
		ModelID:  "gpt-5.4",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, timeoutError{}
			}),
		},
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	_, err = client.GenerateText(context.Background(), GenerateTextRequest{Input: "timeout"})
	if !errors.Is(err, ErrOpenAIRequestTimeout) {
		t.Fatalf("expected ErrOpenAIRequestTimeout, got %v", err)
	}
}

func TestGenerateTextReturnsRequestError(t *testing.T) {
	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:   "test-key",
		Endpoint: "https://example.test/responses",
		ModelID:  "gpt-5.4",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, net.UnknownNetworkError("boom")
			}),
		},
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	_, err = client.GenerateText(context.Background(), GenerateTextRequest{Input: "boom"})
	if !errors.Is(err, ErrOpenAIRequestFailed) {
		t.Fatalf("expected ErrOpenAIRequestFailed, got %v", err)
	}
}

func TestOpenAIHTTPStatusErrorUsesFallbackMessageWhenMessageMissing(t *testing.T) {
	err := (&OpenAIHTTPStatusError{StatusCode: http.StatusBadGateway}).Error()
	if err != "openai responses returned http status 502" {
		t.Fatalf("unexpected fallback status error text: %q", err)
	}
}

func TestGenerateTextRejectsEmptyInput(t *testing.T) {
	client, err := NewOpenAIResponsesClient(OpenAIResponsesClientConfig{
		APIKey:   "test-key",
		Endpoint: "https://example.test/responses",
		ModelID:  "gpt-5.4",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient returned error: %v", err)
	}

	_, err = client.GenerateText(context.Background(), GenerateTextRequest{Input: "   "})
	if !errors.Is(err, ErrGenerateTextInputRequired) {
		t.Fatalf("expected ErrGenerateTextInputRequired, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip adapts a function literal to http.RoundTripper for tests.
func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type timeoutError struct{}

// Error returns the timeout test error message.
func (timeoutError) Error() string {
	return "timeout"
}

// Timeout reports that this test error represents a timeout.
func (timeoutError) Timeout() bool {
	return true
}

// Temporary reports that this test error is not temporary.
func (timeoutError) Temporary() bool {
	return false
}

var _ net.Error = timeoutError{}

var _ = time.Second
