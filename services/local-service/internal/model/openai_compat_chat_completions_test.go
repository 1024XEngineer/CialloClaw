package model

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatCompletionsFallbackEnabledOnlyForCompatibilityErrors(t *testing.T) {
	if chatCompletionsFallbackEnabled(nil) {
		t.Fatal("expected nil error not to enable chat completions fallback")
	}
	if !chatCompletionsFallbackEnabled(&OpenAIHTTPStatusError{StatusCode: http.StatusNotFound}) {
		t.Fatal("expected 404 responses route failure to enable fallback")
	}
	if !chatCompletionsFallbackEnabled(&OpenAIHTTPStatusError{StatusCode: http.StatusBadRequest, Message: "tool_choice is not supported"}) {
		t.Fatal("expected compatibility text to enable fallback")
	}
	if chatCompletionsFallbackEnabled(errors.New("boom")) {
		t.Fatal("expected unrelated errors not to enable fallback")
	}
}

func TestChatCompletionsURLFallsBackToTrimmedEndpointWhenBaseURLNormalizationFails(t *testing.T) {
	client := &OpenAIResponsesClient{endpoint: "  http://%zz/  "}
	if url := client.chatCompletionsURL(); url != "http://%zz/chat/completions" {
		t.Fatalf("unexpected fallback chat completions url: %q", url)
	}
}

func TestExtractRawAPIErrorMessagePrefersJSONThenBodyThenDefault(t *testing.T) {
	if message := extractRawAPIErrorMessage([]byte(`{"error":{"message":"route unavailable"}}`), 404); message != "route unavailable" {
		t.Fatalf("expected structured API message, got %q", message)
	}
	if message := extractRawAPIErrorMessage([]byte("  upstream plain text  "), 500); message != "upstream plain text" {
		t.Fatalf("expected raw body fallback, got %q", message)
	}
	if message := extractRawAPIErrorMessage([]byte("   "), 502); !strings.Contains(message, "502") {
		t.Fatalf("expected default message to mention status code, got %q", message)
	}
}

func TestExtractChatCompletionsTextPrefersMessageContentThenTextParts(t *testing.T) {
	response := chatCompletionsResponseBody{
		Choices: []chatCompletionsChoice{{
			Message: chatCompletionsAssistantMessage{Content: "  answer  "},
		}, {
			Message: chatCompletionsAssistantMessage{ContentParts: []chatCompletionsContentPart{{Type: "text", Text: "part text"}}},
		}},
	}
	if text := extractChatCompletionsText(response); text != "answer" {
		t.Fatalf("expected direct message content to win, got %q", text)
	}

	partsOnly := chatCompletionsResponseBody{Choices: []chatCompletionsChoice{{
		Message: chatCompletionsAssistantMessage{ContentParts: []chatCompletionsContentPart{{Type: "ignored", Text: "skip"}, {Type: "text", Text: "  from parts  "}}},
	}}}
	if text := extractChatCompletionsText(partsOnly); text != "from parts" {
		t.Fatalf("expected text content part fallback, got %q", text)
	}
	if text := extractChatCompletionsText(chatCompletionsResponseBody{}); text != "" {
		t.Fatalf("expected empty response to yield no text, got %q", text)
	}
}

func TestBuildChatCompletionsToolsSkipsBlankNamesAndNormalizesDefinitions(t *testing.T) {
	tools := buildChatCompletionsTools([]ToolDefinition{{
		Name:        "  read_file  ",
		Description: "  Read a workspace file  ",
		InputSchema: map[string]any{"type": "object"},
	}, {
		Name: "  ",
	}})
	if len(tools) != 1 {
		t.Fatalf("expected blank tool names to be skipped, got %+v", tools)
	}
	if tools[0].Function.Name != "read_file" || tools[0].Function.Description != "Read a workspace file" || tools[0].Function.Strict != true {
		t.Fatalf("unexpected normalized tool definition: %+v", tools[0])
	}
}

func TestExtractChatCompletionsToolCallsPreservesDecodedAndRawArguments(t *testing.T) {
	toolCalls := extractChatCompletionsToolCalls(chatCompletionsResponseBody{Choices: []chatCompletionsChoice{{
		Message: chatCompletionsAssistantMessage{ToolCalls: []chatCompletionsToolInvocation{{
			Function: chatCompletionsToolCallFunction{Name: " read_file ", Arguments: `{"path":"notes/todo.md"}`},
		}, {
			Function: chatCompletionsToolCallFunction{Name: "broken", Arguments: `not-json`},
		}}},
	}}})
	if len(toolCalls) != 2 {
		t.Fatalf("expected two tool calls, got %+v", toolCalls)
	}
	if toolCalls[0].Name != "read_file" || toolCalls[0].Arguments["path"] != "notes/todo.md" {
		t.Fatalf("expected decoded JSON arguments, got %+v", toolCalls[0])
	}
	if toolCalls[1].Arguments["_raw_arguments"] != "not-json" {
		t.Fatalf("expected raw arguments fallback for invalid JSON, got %+v", toolCalls[1])
	}
}

func TestChatCompletionsAssistantMessageUnmarshalJSONSupportsStringPartsAndNull(t *testing.T) {
	var message chatCompletionsAssistantMessage
	if err := json.Unmarshal([]byte(`{"content":"hello"}`), &message); err != nil {
		t.Fatalf("expected string content to decode, got %v", err)
	}
	if message.Content != "hello" {
		t.Fatalf("unexpected string content decode: %+v", message)
	}

	message = chatCompletionsAssistantMessage{}
	if err := json.Unmarshal([]byte(`{"content":[{"type":"text","text":"hello parts"}]}`), &message); err != nil {
		t.Fatalf("expected content parts to decode, got %v", err)
	}
	if len(message.ContentParts) != 1 || message.ContentParts[0].Text != "hello parts" {
		t.Fatalf("unexpected content part decode: %+v", message)
	}

	message = chatCompletionsAssistantMessage{}
	if err := json.Unmarshal([]byte(`{"content":null}`), &message); err != nil {
		t.Fatalf("expected null content to decode, got %v", err)
	}
	if message.Content != "" || len(message.ContentParts) != 0 {
		t.Fatalf("expected null content to leave message empty, got %+v", message)
	}
}

func TestChatCompletionsRequestReturnsStatusAndDecodeErrors(t *testing.T) {
	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream unavailable"))
	}))
	defer statusServer.Close()

	client := &OpenAIResponsesClient{apiKey: "test-key", endpoint: statusServer.URL, httpClient: statusServer.Client()}
	if _, err := client.chatCompletionsRequest(context.Background(), chatCompletionsRequestBody{Model: "gpt-4.1-mini"}); err == nil {
		t.Fatal("expected non-2xx chat completions request to fail")
	} else {
		var statusErr *OpenAIHTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadGateway || !strings.Contains(statusErr.Message, "upstream unavailable") {
			t.Fatalf("expected typed status error, got %v", err)
		}
	}

	decodeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer decodeServer.Close()

	client = &OpenAIResponsesClient{apiKey: "test-key", endpoint: decodeServer.URL, httpClient: decodeServer.Client()}
	if _, err := client.chatCompletionsRequest(context.Background(), chatCompletionsRequestBody{Model: "gpt-4.1-mini"}); !errors.Is(err, ErrOpenAIResponseInvalid) {
		t.Fatalf("expected invalid response error, got %v", err)
	}
}

func TestErrorsAsHTTPStatusHandlesNilAndWrappedStatusErrors(t *testing.T) {
	var statusErr *OpenAIHTTPStatusError
	if errorsAsHTTPStatus(nil, &statusErr) {
		t.Fatal("expected nil error not to match http status errors")
	}
	wrapped := errors.Join(&OpenAIHTTPStatusError{StatusCode: http.StatusBadGateway, Message: "bad gateway"}, errors.New("ignored"))
	if !errorsAsHTTPStatus(wrapped, &statusErr) || statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected wrapped status error to be discovered, got %+v", statusErr)
	}
}
