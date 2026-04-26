package orchestrator

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

const settingsModelValidateTimeout = 8 * time.Second

// SettingsModelValidate probes the effective model route that future tasks will
// use and reports whether text generation plus tool calling are ready.
func (s *Service) SettingsModelValidate(params map[string]any) (map[string]any, error) {
	probe := s.buildSettingsModelValidationProbe(params)
	return s.runSettingsModelValidationProbe(probe), nil
}

type settingsModelValidationProbe struct {
	provider            string
	canonicalProvider   string
	baseURL             string
	modelID             string
	apiKey              string
	useSecretSource     bool
	textGenerationReady bool
	toolCallingReady    bool
}

func (s *Service) buildSettingsModelValidationProbe(params map[string]any) settingsModelValidationProbe {
	baseConfig := s.currentModelConfig()
	currentModels := modelSettingsSection(s.runEngine.Settings())
	normalizedParams := normalizeSettingsUpdateParams(params)
	modelsPatch := cloneMap(mapValue(normalizedParams, "models"))

	provider := firstNonEmptyString(stringValue(currentModels, "provider", ""), s.defaultSettingsProvider())
	if value, ok := stringSettingOverride(modelsPatch, "provider"); ok {
		provider = value
	}

	baseURL := strings.TrimSpace(baseConfig.Endpoint)
	if value, ok := stringSettingOverride(modelsPatch, "base_url"); ok {
		baseURL = value
	}

	modelID := strings.TrimSpace(baseConfig.ModelID)
	if value, ok := stringSettingOverride(modelsPatch, "model"); ok {
		modelID = value
	}

	apiKey, apiKeyProvided := stringSettingOverride(modelsPatch, "api_key")
	deleteAPIKey := boolValue(modelsPatch, "delete_api_key", false)
	useSecretSource := !apiKeyProvided && !deleteAPIKey

	return settingsModelValidationProbe{
		provider:          provider,
		canonicalProvider: model.CanonicalProviderName(provider),
		baseURL:           baseURL,
		modelID:           modelID,
		apiKey:            apiKey,
		useSecretSource:   useSecretSource,
	}
}

func stringSettingOverride(values map[string]any, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	value, ok := values[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", true
	}
	return strings.TrimSpace(text), true
}

func (s *Service) runSettingsModelValidationProbe(probe settingsModelValidationProbe) map[string]any {
	serviceConfig := model.ServiceConfig{ModelConfig: s.currentModelConfig()}
	serviceConfig.ModelConfig.Provider = probe.canonicalProvider
	serviceConfig.ModelConfig.Endpoint = probe.baseURL
	serviceConfig.ModelConfig.ModelID = probe.modelID
	serviceConfig.APIKey = probe.apiKey
	if probe.useSecretSource && s.storage != nil {
		serviceConfig.SecretSource = model.NewStaticSecretSource(s.storage)
	}

	modelService, err := model.NewServiceFromConfig(serviceConfig)
	if err != nil {
		return settingsModelValidationFailure(probe, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), settingsModelValidateTimeout)
	defer cancel()

	if _, err := modelService.GenerateText(ctx, model.GenerateTextRequest{
		TaskID: "settings_model_validate",
		RunID:  "settings_model_validate_text",
		Input:  "Reply with OK.",
	}); err != nil {
		return settingsModelValidationFailure(probe, err)
	}
	probe.textGenerationReady = true

	toolResult, err := modelService.GenerateToolCalls(ctx, model.ToolCallRequest{
		TaskID: "settings_model_validate",
		RunID:  "settings_model_validate_tool",
		Input:  "Call the validation_echo tool with the value \"ok\". Do not answer with plain text.",
		Tools: []model.ToolDefinition{{
			Name:        "validation_echo",
			Description: "Echoes a short validation string.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value": map[string]any{"type": "string"},
				},
				"required": []string{"value"},
			},
		}},
	})
	if err != nil {
		return settingsModelValidationFailure(probe, err)
	}
	if len(toolResult.ToolCalls) == 0 {
		return settingsModelValidationResult(probe, false, "tool_calling_unavailable", "模型配置校验失败：文本生成可用，但工具调用未返回预期结果，请检查上游工具调用兼容性。")
	}
	probe.toolCallingReady = true

	return settingsModelValidationResult(probe, true, "valid", "当前模型配置校验通过，可执行文本生成与工具调用。")
}

func settingsModelValidationFailure(probe settingsModelValidationProbe, err error) map[string]any {
	status, message := classifySettingsModelValidationFailure(err)
	return settingsModelValidationResult(probe, false, status, message)
}

func settingsModelValidationResult(probe settingsModelValidationProbe, ok bool, status, message string) map[string]any {
	return map[string]any{
		"ok":                    ok,
		"status":                status,
		"message":               message,
		"provider":              probe.provider,
		"canonical_provider":    probe.canonicalProvider,
		"base_url":              probe.baseURL,
		"model":                 probe.modelID,
		"text_generation_ready": probe.textGenerationReady,
		"tool_calling_ready":    probe.toolCallingReady,
	}
}

func classifySettingsModelValidationFailure(err error) (string, string) {
	var statusErr *model.OpenAIHTTPStatusError
	switch {
	case errors.Is(err, model.ErrModelProviderRequired):
		return "missing_provider", "模型配置校验失败：Provider 不能为空。"
	case errors.Is(err, model.ErrOpenAIEndpointRequired):
		return "missing_base_url", "模型配置校验失败：Base URL 不能为空。"
	case errors.Is(err, model.ErrOpenAIModelIDRequired):
		return "missing_model", "模型配置校验失败：Model 不能为空。"
	case errors.Is(err, model.ErrSecretSourceFailed) && errors.Is(err, model.ErrSecretNotFound):
		return "missing_api_key", "模型配置校验失败：当前模型未配置可用的 API Key，请重新输入并保存。"
	case errors.Is(err, model.ErrSecretSourceFailed):
		return "secret_store_unavailable", "模型配置校验失败：暂时无法读取已保存的 API Key，请稍后重试。"
	case errors.Is(err, model.ErrClientNotConfigured):
		return "missing_api_key", "模型配置校验失败：当前模型未配置可用的 API Key，请重新输入并保存。"
	case errors.Is(err, model.ErrToolCallingNotSupported):
		return "tool_calling_unavailable", "模型配置校验失败：当前模型接口不支持工具调用。"
	case errors.Is(err, model.ErrOpenAIResponseInvalid):
		return "invalid_response", "模型配置校验失败：模型返回内容无法解析，请检查上游接口兼容性。"
	case errors.Is(err, model.ErrOpenAIRequestTimeout):
		return "request_timeout", "模型配置校验失败：模型请求超时，请稍后重试。"
	case errors.Is(err, model.ErrOpenAIRequestFailed):
		return "request_failed", "模型配置校验失败：模型请求发送失败，请检查网络连接或上游地址。"
	case errors.As(err, &statusErr):
		return classifySettingsModelHTTPStatusFailure(statusErr)
	default:
		return "unknown_error", "模型配置校验失败：暂时无法验证当前模型配置，请稍后重试。"
	}
}

func classifySettingsModelHTTPStatusFailure(statusErr *model.OpenAIHTTPStatusError) (string, string) {
	if statusErr == nil {
		return "unknown_error", "模型配置校验失败：暂时无法验证当前模型配置，请稍后重试。"
	}
	safeMessage := sanitizeModelProviderMessage(statusErr.Message)
	lowerSafeMessage := strings.ToLower(safeMessage)
	switch statusErr.StatusCode {
	case 400:
		if strings.Contains(lowerSafeMessage, "tool") || strings.Contains(lowerSafeMessage, "function_call") || strings.Contains(lowerSafeMessage, "tool_choice") {
			return "tool_calling_unavailable", "模型配置校验失败：当前模型接口不支持工具调用。"
		}
		if safeMessage != "" {
			return "request_rejected", "模型配置校验失败：上游拒绝当前配置（" + safeMessage + "）。"
		}
		return "request_rejected", "模型配置校验失败：上游拒绝当前配置，请检查 Provider、Base URL、Model 或输入兼容性。"
	case 401, 403:
		if safeMessage != "" {
			return "auth_failed", "模型配置校验失败：鉴权失败（" + safeMessage + "），请检查 API Key 或访问权限。"
		}
		return "auth_failed", "模型配置校验失败：鉴权失败，请检查 API Key 或访问权限。"
	case 404:
		if safeMessage != "" {
			return "endpoint_not_found", "模型配置校验失败：模型接口或模型标识不存在（" + safeMessage + "）。"
		}
		return "endpoint_not_found", "模型配置校验失败：模型接口或模型标识不存在，请检查 Base URL 与 Model。"
	case 429:
		if safeMessage != "" {
			return "request_rejected", "模型配置校验失败：模型请求过于频繁（" + safeMessage + "），请稍后重试。"
		}
		return "request_rejected", "模型配置校验失败：模型请求过于频繁，请稍后重试。"
	case 408, 504:
		return "request_timeout", "模型配置校验失败：模型请求超时，请稍后重试。"
	default:
		if safeMessage != "" {
			return "request_rejected", "模型配置校验失败：模型调用失败（" + safeMessage + "）。"
		}
		return "request_rejected", "模型配置校验失败：模型调用失败，请稍后重试。"
	}
}
