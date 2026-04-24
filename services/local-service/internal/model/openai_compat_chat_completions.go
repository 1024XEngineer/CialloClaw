package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// chatCompletionsFallbackEnabled keeps the OpenAI-compatible provider usable on
// endpoints that implement `/chat/completions` plus tool calls but not the
// newer `/responses` route yet.
func chatCompletionsFallbackEnabled(err error) bool {
	if err == nil {
		return false
	}
	return isChatCompletionsCompatibilityError(err)
}

// generateTextViaChatCompletions retries the minimal text generation contract
// against the Chat Completions route when the upstream does not implement the
// Responses API shape.
func (c *OpenAIResponsesClient) generateTextViaChatCompletions(ctx context.Context, request GenerateTextRequest) (GenerateTextResponse, error) {
	startedAt := time.Now()
	response, err := c.chatCompletionsRequest(ctx, chatCompletionsRequestBody{
		Model: c.modelID,
		Messages: []chatCompletionsMessage{{
			Role:    "user",
			Content: strings.TrimSpace(request.Input),
		}},
	})
	if err != nil {
		return GenerateTextResponse{}, err
	}

	return GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  strings.TrimSpace(response.ID),
		Provider:   OpenAIResponsesProvider,
		ModelID:    firstNonEmpty(strings.TrimSpace(response.Model), c.modelID),
		OutputText: extractChatCompletionsText(response),
		Usage: TokenUsage{
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
			TotalTokens:  response.Usage.TotalTokens,
		},
		LatencyMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

// generateToolCallsViaChatCompletions retries planner requests against the
// Chat Completions tool-calling contract so OpenAI-compatible gateways can
// still drive the local agent loop.
func (c *OpenAIResponsesClient) generateToolCallsViaChatCompletions(ctx context.Context, request ToolCallRequest) (ToolCallResult, error) {
	startedAt := time.Now()
	response, err := c.chatCompletionsRequest(ctx, chatCompletionsRequestBody{
		Model: c.modelID,
		Messages: []chatCompletionsMessage{{
			Role:    "user",
			Content: strings.TrimSpace(request.Input),
		}},
		Tools:      buildChatCompletionsTools(request.Tools),
		ToolChoice: "auto",
	})
	if err != nil {
		return ToolCallResult{}, err
	}

	return ToolCallResult{
		RequestID:  strings.TrimSpace(response.ID),
		Provider:   OpenAIResponsesProvider,
		ModelID:    firstNonEmpty(strings.TrimSpace(response.Model), c.modelID),
		OutputText: extractChatCompletionsText(response),
		ToolCalls:  extractChatCompletionsToolCalls(response),
		Usage: TokenUsage{
			InputTokens:  response.Usage.PromptTokens,
			OutputTokens: response.Usage.CompletionTokens,
			TotalTokens:  response.Usage.TotalTokens,
		},
		LatencyMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

func (c *OpenAIResponsesClient) chatCompletionsRequest(ctx context.Context, payload chatCompletionsRequestBody) (chatCompletionsResponseBody, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return chatCompletionsResponseBody{}, fmt.Errorf("marshal chat completions request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return chatCompletionsResponseBody{}, fmt.Errorf("build chat completions request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return chatCompletionsResponseBody{}, classifyOpenAIRequestError(err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return chatCompletionsResponseBody{}, fmt.Errorf("read chat completions response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return chatCompletionsResponseBody{}, &OpenAIHTTPStatusError{
			StatusCode: response.StatusCode,
			Message:    truncateErrorMessage(extractRawAPIErrorMessage(responseBody, response.StatusCode)),
		}
	}

	decoded := chatCompletionsResponseBody{}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return chatCompletionsResponseBody{}, fmt.Errorf("%w: %v", ErrOpenAIResponseInvalid, err)
	}
	return decoded, nil
}

func (c *OpenAIResponsesClient) chatCompletionsURL() string {
	baseURL, err := normalizeOpenAIBaseURL(c.endpoint)
	if err != nil {
		return strings.TrimSuffix(strings.TrimSpace(c.endpoint), "/") + "/chat/completions"
	}
	return strings.TrimSuffix(baseURL, "/") + "/chat/completions"
}

func isChatCompletionsCompatibilityError(err error) bool {
	if err == nil {
		return false
	}
	var statusErr *OpenAIHTTPStatusError
	if errorsAsHTTPStatus(err, &statusErr) {
		if statusErr.StatusCode == http.StatusNotFound || statusErr.StatusCode == http.StatusMethodNotAllowed || statusErr.StatusCode == http.StatusNotImplemented {
			return true
		}
		message := strings.ToLower(strings.TrimSpace(statusErr.Message))
		return strings.Contains(message, "responses") ||
			strings.Contains(message, "unsupported parameter") ||
			strings.Contains(message, "tool_choice") ||
			strings.Contains(message, "function_call")
	}
	return false
}

func errorsAsHTTPStatus(err error, target **OpenAIHTTPStatusError) bool {
	if err == nil {
		return false
	}
	return errors.As(err, target)
}

func extractRawAPIErrorMessage(body []byte, statusCode int) string {
	var raw struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &raw); err == nil {
		if strings.TrimSpace(raw.Error.Message) != "" {
			return raw.Error.Message
		}
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("openai-compatible chat completions returned http status %d", statusCode)
}

func extractChatCompletionsText(response chatCompletionsResponseBody) string {
	for _, choice := range response.Choices {
		if text := strings.TrimSpace(choice.Message.Content); text != "" {
			return text
		}
		for _, part := range choice.Message.ContentParts {
			if strings.TrimSpace(part.Type) == "text" && strings.TrimSpace(part.Text) != "" {
				return strings.TrimSpace(part.Text)
			}
		}
	}
	return ""
}

func buildChatCompletionsTools(definitions []ToolDefinition) []chatCompletionsToolDefinition {
	tools := make([]chatCompletionsToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		name := strings.TrimSpace(definition.Name)
		if name == "" {
			continue
		}
		tools = append(tools, chatCompletionsToolDefinition{
			Type: "function",
			Function: chatCompletionsFunctionDefinition{
				Name:        name,
				Description: strings.TrimSpace(definition.Description),
				Parameters:  normalizeToolSchema(definition.InputSchema),
				Strict:      true,
			},
		})
	}
	return tools
}

func extractChatCompletionsToolCalls(response chatCompletionsResponseBody) []ToolInvocation {
	toolCalls := make([]ToolInvocation, 0)
	for _, choice := range response.Choices {
		for _, toolCall := range choice.Message.ToolCalls {
			arguments := map[string]any{}
			if strings.TrimSpace(toolCall.Function.Arguments) != "" {
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
					arguments = map[string]any{"_raw_arguments": toolCall.Function.Arguments}
				}
			}
			toolCalls = append(toolCalls, ToolInvocation{
				Name:      strings.TrimSpace(toolCall.Function.Name),
				Arguments: arguments,
			})
		}
	}
	return toolCalls
}

type chatCompletionsRequestBody struct {
	Model      string                          `json:"model"`
	Messages   []chatCompletionsMessage        `json:"messages"`
	Tools      []chatCompletionsToolDefinition `json:"tools,omitempty"`
	ToolChoice string                          `json:"tool_choice,omitempty"`
}

type chatCompletionsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsToolDefinition struct {
	Type     string                            `json:"type"`
	Function chatCompletionsFunctionDefinition `json:"function"`
}

type chatCompletionsFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Strict      bool           `json:"strict,omitempty"`
}

type chatCompletionsResponseBody struct {
	ID      string                        `json:"id"`
	Model   string                        `json:"model"`
	Choices []chatCompletionsChoice       `json:"choices"`
	Usage   chatCompletionsUsageBreakdown `json:"usage"`
}

type chatCompletionsChoice struct {
	Message chatCompletionsAssistantMessage `json:"message"`
}

type chatCompletionsAssistantMessage struct {
	Content      string                          `json:"content"`
	ContentParts []chatCompletionsContentPart    `json:"-"`
	ToolCalls    []chatCompletionsToolInvocation `json:"tool_calls"`
}

func (m *chatCompletionsAssistantMessage) UnmarshalJSON(data []byte) error {
	type rawMessage struct {
		Content   json.RawMessage                 `json:"content"`
		ToolCalls []chatCompletionsToolInvocation `json:"tool_calls"`
	}
	decoded := rawMessage{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	m.ToolCalls = decoded.ToolCalls
	if len(decoded.Content) == 0 || string(decoded.Content) == "null" {
		return nil
	}
	if err := json.Unmarshal(decoded.Content, &m.Content); err == nil {
		return nil
	}
	return json.Unmarshal(decoded.Content, &m.ContentParts)
}

type chatCompletionsContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type chatCompletionsToolInvocation struct {
	ID       string                          `json:"id"`
	Type     string                          `json:"type"`
	Function chatCompletionsToolCallFunction `json:"function"`
}

type chatCompletionsToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionsUsageBreakdown struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
