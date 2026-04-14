package builtin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const generateTextPreviewLimit = 200

type GenerateTextInput struct {
	Prompt       string
	FallbackText string
	IntentName   string
}

type GenerateTextTool struct {
	meta tools.ToolMetadata
}

func NewGenerateTextTool() *GenerateTextTool {
	return &GenerateTextTool{
		meta: tools.ToolMetadata{
			Name:            "generate_text",
			DisplayName:     "生成文本",
			Description:     "通过统一模型层生成当前任务所需的文本结果",
			Source:          tools.ToolSourceBuiltin,
			RiskHint:        tools.RiskLevelGreen,
			TimeoutSec:      90,
			InputSchemaRef:  "tools/generate_text/input",
			OutputSchemaRef: "tools/generate_text/output",
			SupportsDryRun:  false,
		},
	}
}

func (t *GenerateTextTool) Metadata() tools.ToolMetadata {
	return t.meta
}

func (t *GenerateTextTool) Validate(input map[string]any) error {
	_, err := parseGenerateTextInput(input)
	return err
}

func (t *GenerateTextTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	parsed, err := parseGenerateTextInput(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}

	result := map[string]any{
		"content":         fallbackTextForGenerateText(parsed),
		"provider":        "local_fallback",
		"model_id":        "",
		"request_id":      "",
		"latency_ms":      int64(0),
		"fallback":        true,
		"fallback_reason": "model client not configured",
		"token_usage": map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
			"total_tokens":  0,
		},
	}

	if execCtx != nil && execCtx.Model != nil {
		response, modelErr := execCtx.Model.GenerateText(ctx, model.GenerateTextRequest{
			TaskID: execCtx.TaskID,
			RunID:  execCtx.RunID,
			Input:  parsed.Prompt,
		})
		if modelErr == nil && strings.TrimSpace(response.OutputText) != "" {
			result["content"] = strings.TrimSpace(response.OutputText)
			result["provider"] = firstNonEmptyString(response.Provider, execCtx.Model.Provider())
			result["model_id"] = firstNonEmptyString(response.ModelID, execCtx.Model.ModelID())
			result["request_id"] = response.RequestID
			result["latency_ms"] = response.LatencyMS
			result["fallback"] = false
			delete(result, "fallback_reason")
			result["token_usage"] = map[string]any{
				"input_tokens":  response.Usage.InputTokens,
				"output_tokens": response.Usage.OutputTokens,
				"total_tokens":  response.Usage.TotalTokens,
			}
		} else if modelErr != nil {
			result["fallback_reason"] = modelErr.Error()
		}
	}

	result["audit_candidate"] = map[string]any{
		"type":    "model",
		"action":  "generate_text",
		"summary": "generate text output",
		"target":  firstNonEmptyString(parsed.IntentName, "main_flow"),
		"result":  "success",
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     result,
		SummaryOutput: buildGenerateTextSummary(result),
	}, nil
}

func parseGenerateTextInput(input map[string]any) (GenerateTextInput, error) {
	promptValue, ok := input["prompt"].(string)
	if !ok || strings.TrimSpace(promptValue) == "" {
		return GenerateTextInput{}, errors.New("input field 'prompt' must be a non-empty string")
	}

	fallbackValue, _ := input["fallback_text"].(string)
	intentName, _ := input["intent_name"].(string)
	return GenerateTextInput{
		Prompt:       promptValue,
		FallbackText: fallbackValue,
		IntentName:   intentName,
	}, nil
}

func fallbackTextForGenerateText(input GenerateTextInput) string {
	if strings.TrimSpace(input.FallbackText) != "" {
		return input.FallbackText
	}
	return strings.TrimSpace(input.Prompt)
}

func buildGenerateTextSummary(raw map[string]any) map[string]any {
	content, _ := raw["content"].(string)
	return map[string]any{
		"provider":        raw["provider"],
		"model_id":        raw["model_id"],
		"request_id":      raw["request_id"],
		"latency_ms":      raw["latency_ms"],
		"fallback":        raw["fallback"],
		"fallback_reason": raw["fallback_reason"],
		"content_preview": previewReadFileText(content, generateTextPreviewLimit),
		"token_usage":     raw["token_usage"],
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
