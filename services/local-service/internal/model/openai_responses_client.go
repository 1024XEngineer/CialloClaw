// 该文件负责 OpenAI Responses provider 的最小实现。
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

// OpenAIResponsesProvider 定义当前模块的基础变量。
const OpenAIResponsesProvider = "openai_responses"

// ErrOpenAIAPIKeyRequired 定义当前模块的基础变量。
var ErrOpenAIAPIKeyRequired = errors.New("openai responses api key is required")

// ErrOpenAIEndpointRequired 定义当前模块的基础变量。
var ErrOpenAIEndpointRequired = errors.New("openai responses endpoint is required")

// ErrOpenAIModelIDRequired 定义当前模块的基础变量。
var ErrOpenAIModelIDRequired = errors.New("openai responses model id is required")

// ErrOpenAIRequestFailed 定义当前模块的基础变量。
var ErrOpenAIRequestFailed = errors.New("openai responses request failed")

// ErrOpenAIRequestTimeout 定义当前模块的基础变量。
var ErrOpenAIRequestTimeout = errors.New("openai responses request timed out")

// ErrOpenAIResponseInvalid 定义当前模块的基础变量。
var ErrOpenAIResponseInvalid = errors.New("openai responses response invalid")

// ErrOpenAIHTTPStatus 定义当前模块的基础变量。
var ErrOpenAIHTTPStatus = errors.New("openai responses http status error")

// ErrGenerateTextInputRequired 定义当前模块的基础变量。
var ErrGenerateTextInputRequired = errors.New("generate text input is required")

// OpenAIResponsesClientConfig 描述当前模块配置。
type OpenAIResponsesClientConfig struct {
	APIKey     string
	Endpoint   string
	ModelID    string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// OpenAIResponsesClient 封装 OpenAI 官方 Responses SDK。
type OpenAIResponsesClient struct {
	apiKey     string
	endpoint   string
	modelID    string
	timeout    time.Duration
	httpClient *http.Client
	client     openai.Client
}

// OpenAIHTTPStatusError 归一化 SDK 返回的 HTTP 状态错误。
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

// NewOpenAIResponsesClient 创建并返回基于官方 SDK 的 client。
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

// GenerateText 通过官方 Responses SDK 执行最小文本生成。
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

// Provider 返回 provider 名称。
func (c *OpenAIResponsesClient) Provider() string {
	return OpenAIResponsesProvider
}

// ModelID 返回 model id。
func (c *OpenAIResponsesClient) ModelID() string {
	return c.modelID
}

// Endpoint 返回配置的原始 endpoint。
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
	return schema
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
