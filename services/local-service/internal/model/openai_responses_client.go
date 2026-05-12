// Package model contains the OpenAI Responses provider implementation.
package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/responses"
)

// OpenAIResponsesProvider is the provider identifier exposed by model services.
const OpenAIResponsesProvider = "openai_responses"

// ErrOpenAIAPIKeyRequired reports a missing API key before a client is created.
var ErrOpenAIAPIKeyRequired = errors.New("openai responses api key is required")

// ErrOpenAIEndpointRequired reports a missing or empty provider endpoint.
var ErrOpenAIEndpointRequired = errors.New("openai responses endpoint is required")

// ErrOpenAIModelIDRequired reports a missing model identifier.
var ErrOpenAIModelIDRequired = errors.New("openai responses model id is required")

// ErrOpenAIRequestFailed wraps transport or SDK errors that are not classified further.
var ErrOpenAIRequestFailed = errors.New("openai responses request failed")

// ErrOpenAIRequestTimeout wraps provider calls that exceeded their deadline.
var ErrOpenAIRequestTimeout = errors.New("openai responses request timed out")

// ErrOpenAIResponseInvalid wraps malformed provider response payloads.
var ErrOpenAIResponseInvalid = errors.New("openai responses response invalid")

// ErrOpenAIHTTPStatus wraps non-success HTTP statuses returned by the provider.
var ErrOpenAIHTTPStatus = errors.New("openai responses http status error")

// ErrGenerateTextInputRequired reports an empty generation prompt.
var ErrGenerateTextInputRequired = errors.New("generate text input is required")

// OpenAIResponsesClientConfig is the complete dependency/configuration input for
// one OpenAI Responses client.
type OpenAIResponsesClientConfig struct {
	APIKey     string
	Endpoint   string
	ModelID    string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// OpenAIResponsesClient wraps the official OpenAI SDK behind the local Client
// and ToolCallingClient contracts.
type OpenAIResponsesClient struct {
	apiKey     string
	endpoint   string
	modelID    string
	timeout    time.Duration
	httpClient *http.Client
	client     openai.Client
}

// OpenAIHTTPStatusError normalizes provider HTTP status failures.
type OpenAIHTTPStatusError struct {
	StatusCode int
	Message    string
}

func (e *OpenAIHTTPStatusError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("openai responses returned http status %d", e.StatusCode)
	}
	return fmt.Sprintf("openai responses returned http status %d: %s", e.StatusCode, e.Message)
}

func (e *OpenAIHTTPStatusError) Unwrap() error {
	return ErrOpenAIHTTPStatus
}

const defaultOpenAIResponsesTimeout = 30 * time.Second

// NewOpenAIResponsesClient validates provider config and returns an SDK-backed client.
func NewOpenAIResponsesClient(cfg OpenAIResponsesClientConfig) (*OpenAIResponsesClient, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrOpenAIAPIKeyRequired
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, ErrOpenAIEndpointRequired
	}
	if strings.TrimSpace(cfg.ModelID) == "" {
		return nil, ErrOpenAIModelIDRequired
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultOpenAIResponsesTimeout
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	} else {
		cloned := *httpClient
		if cloned.Timeout <= 0 {
			cloned.Timeout = timeout
		}
		httpClient = &cloned
	}

	baseURL, err := normalizeOpenAIBaseURL(cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(
		option.WithAPIKey(strings.TrimSpace(cfg.APIKey)),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
		option.WithRequestTimeout(timeout),
	)

	return &OpenAIResponsesClient{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		endpoint:   strings.TrimSpace(cfg.Endpoint),
		modelID:    strings.TrimSpace(cfg.ModelID),
		timeout:    timeout,
		httpClient: httpClient,
		client:     client,
	}, nil
}

// GenerateText performs one minimal text generation through the Responses API.
func (c *OpenAIResponsesClient) GenerateText(ctx context.Context, request GenerateTextRequest) (GenerateTextResponse, error) {
	startedAt := time.Now()
	if strings.TrimSpace(request.Input) == "" {
		return GenerateTextResponse{}, ErrGenerateTextInputRequired
	}

	response, err := c.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: responses.ResponsesModel(c.modelID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(strings.TrimSpace(request.Input)),
		},
	})
	if err != nil {
		classifiedErr := classifyOpenAIRequestError(err)
		if chatCompletionsFallbackEnabled(classifiedErr) {
			return c.generateTextViaChatCompletions(ctx, request)
		}
		return GenerateTextResponse{}, classifiedErr
	}

	return GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  response.ID,
		Provider:   OpenAIResponsesProvider,
		ModelID:    firstNonEmpty(string(response.Model), c.modelID),
		OutputText: extractSDKResponseText(*response),
		Usage: TokenUsage{
			InputTokens:  int(response.Usage.InputTokens),
			OutputTokens: int(response.Usage.OutputTokens),
			TotalTokens:  int(response.Usage.TotalTokens),
		},
		LatencyMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

// GenerateToolCalls asks the Responses API to decide whether to call custom tools.
func (c *OpenAIResponsesClient) GenerateToolCalls(ctx context.Context, request ToolCallRequest) (ToolCallResult, error) {
	startedAt := time.Now()
	if strings.TrimSpace(request.Input) == "" {
		return ToolCallResult{}, ErrGenerateTextInputRequired
	}

	params := responses.ResponseNewParams{
		Model: responses.ResponsesModel(c.modelID),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: param.NewOpt(strings.TrimSpace(request.Input)),
		},
		ToolChoice: responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsAuto),
		},
		Tools: buildOpenAIFunctionTools(request.Tools),
	}

	response, err := c.client.Responses.New(ctx, params)
	if err != nil {
		classifiedErr := classifyOpenAIRequestError(err)
		if chatCompletionsFallbackEnabled(classifiedErr) {
			return c.generateToolCallsViaChatCompletions(ctx, request)
		}
		return ToolCallResult{}, classifiedErr
	}

	return ToolCallResult{
		RequestID:  response.ID,
		Provider:   OpenAIResponsesProvider,
		ModelID:    firstNonEmpty(string(response.Model), c.modelID),
		OutputText: extractSDKResponseText(*response),
		ToolCalls:  extractFunctionToolCalls(*response),
		Usage: TokenUsage{
			InputTokens:  int(response.Usage.InputTokens),
			OutputTokens: int(response.Usage.OutputTokens),
			TotalTokens:  int(response.Usage.TotalTokens),
		},
		LatencyMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

// Provider returns the stable provider identifier used by model descriptors.
func (c *OpenAIResponsesClient) Provider() string {
	return OpenAIResponsesProvider
}

// ModelID returns the configured model identifier.
func (c *OpenAIResponsesClient) ModelID() string {
	return c.modelID
}

// Endpoint returns the configured raw endpoint before SDK path normalization.
func (c *OpenAIResponsesClient) Endpoint() string {
	return c.endpoint
}

func normalizeOpenAIBaseURL(endpoint string) (string, error) {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return "", ErrOpenAIEndpointRequired
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse openai responses endpoint: %w", err)
	}

	path := strings.TrimSuffix(parsed.Path, "/")
	if strings.HasSuffix(path, "/responses") {
		parsed.Path = strings.TrimSuffix(path, "/responses")
	}

	return strings.TrimSuffix(parsed.String(), "/"), nil
}

func classifyOpenAIRequestError(err error) error {
	if err == nil {
		return nil
	}
	if isOpenAITimeoutError(err) {
		return fmt.Errorf("%w: %v", ErrOpenAIRequestTimeout, err)
	}

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return &OpenAIHTTPStatusError{
			StatusCode: apiErr.StatusCode,
			Message:    truncateErrorMessage(extractAPIErrorMessage(apiErr, err)),
		}
	}

	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) || looksLikeJSONDecodeError(err) {
		return fmt.Errorf("%w: %v", ErrOpenAIResponseInvalid, err)
	}

	return fmt.Errorf("%w: %v", ErrOpenAIRequestFailed, err)
}

func isOpenAITimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func extractSDKResponseText(response responses.Response) string {
	if text := strings.TrimSpace(response.OutputText()); text != "" {
		return text
	}

	var raw struct {
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal([]byte(response.RawJSON()), &raw); err == nil {
		return strings.TrimSpace(raw.OutputText)
	}

	return ""
}

func buildOpenAIFunctionTools(definitions []ToolDefinition) []responses.ToolUnionParam {
	tools := make([]responses.ToolUnionParam, 0, len(definitions))
	for _, definition := range definitions {
		name := strings.TrimSpace(definition.Name)
		if name == "" {
			continue
		}
		params := responses.FunctionToolParam{
			Name:       name,
			Parameters: normalizeToolSchema(definition.InputSchema),
			Strict:     param.NewOpt(true),
		}
		if description := strings.TrimSpace(definition.Description); description != "" {
			params.Description = param.NewOpt(description)
		}
		tools = append(tools, responses.ToolUnionParam{OfFunction: &params})
	}
	return tools
}

func normalizeToolSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": true,
		}
	}
	return normalizeToolSchemaMap(schema)
}

func normalizeToolSchemaMap(schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{}
	}

	normalized := make(map[string]any, len(schema))
	for key, value := range schema {
		normalizedValue, keep := normalizeToolSchemaValue(key, value)
		if !keep {
			continue
		}
		normalized[key] = normalizedValue
	}
	return normalized
}

func normalizeToolSchemaValue(key string, value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeToolSchemaMap(typed), true
	case []map[string]any:
		normalized := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			normalized = append(normalized, normalizeToolSchemaMap(item))
		}
		return normalized, true
	case []any:
		normalized := make([]any, 0, len(typed))
		for _, item := range typed {
			normalizedValue, keep := normalizeToolSchemaValue("", item)
			if !keep {
				continue
			}
			normalized = append(normalized, normalizedValue)
		}
		return normalized, true
	case nil:
		if schemaNullRequiresObject(key) {
			return map[string]any{}, true
		}
		// Drop non-neutral nil schema keywords so provider-bound JSON stays valid
		// without changing validation semantics by inventing an empty schema node.
		return nil, false
	default:
		return value, true
	}
}

func schemaNullRequiresObject(key string) bool {
	switch key {
	case "$defs", "definitions", "properties", "patternProperties", "dependentSchemas",
		"items", "propertyNames",
		"additionalProperties", "unevaluatedProperties":
		return true
	default:
		return false
	}
}

func extractFunctionToolCalls(response responses.Response) []ToolInvocation {
	toolCalls := make([]ToolInvocation, 0)
	for _, item := range response.Output {
		if item.Type != "function_call" {
			continue
		}
		call := item.AsFunctionCall()
		arguments := map[string]any{}
		if strings.TrimSpace(call.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Arguments), &arguments); err != nil {
				arguments = map[string]any{
					"_raw_arguments": call.Arguments,
				}
			}
		}
		toolCalls = append(toolCalls, ToolInvocation{
			Name:      strings.TrimSpace(call.Name),
			Arguments: arguments,
		})
	}
	return toolCalls
}

func truncateErrorMessage(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 256 {
		return trimmed
	}
	return trimmed[:256]
}

func looksLikeJSONDecodeError(err error) bool {
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "invalid character") ||
		strings.Contains(message, "unexpected end of json input") ||
		strings.Contains(message, "content-type")
}

func extractAPIErrorMessage(apiErr *openai.Error, err error) string {
	if apiErr == nil {
		return ""
	}
	if msg := strings.TrimSpace(apiErr.Message); msg != "" {
		return msg
	}
	if raw := strings.TrimSpace(apiErr.RawJSON()); raw != "" {
		return raw
	}

	message := strings.TrimSpace(err.Error())
	marker := fmt.Sprintf("%d %s ", apiErr.StatusCode, http.StatusText(apiErr.StatusCode))
	if idx := strings.LastIndex(message, marker); idx >= 0 {
		tail := strings.TrimSpace(message[idx+len(marker):])
		if tail != "" {
			return tail
		}
	}

	return message
}
