package execution

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"

func assignLatestToolTrace(result *Result, toolCall tools.ToolCallRecord) {
	if result == nil || toolCall.ToolName == "" {
		return
	}
	result.ToolName = toolCall.ToolName
	result.ToolInput = cloneMap(toolCall.Input)
	result.ToolOutput = cloneMap(toolCall.Output)
}

func enrichToolTrace(result *Result, extras map[string]any) {
	if result == nil || len(extras) == 0 {
		return
	}
	if result.ToolOutput == nil {
		result.ToolOutput = map[string]any{}
	}
	for key, value := range extras {
		result.ToolOutput[key] = value
	}
}

func enrichLatestToolCall(result *Result, extras map[string]any) {
	if result == nil || len(result.ToolCalls) == 0 || len(extras) == 0 {
		return
	}

	lastIndex := len(result.ToolCalls) - 1
	if result.ToolCalls[lastIndex].Output == nil {
		result.ToolCalls[lastIndex].Output = map[string]any{}
	}
	for key, value := range extras {
		result.ToolCalls[lastIndex].Output[key] = value
	}
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			cloned[key] = cloneMap(typed)
		case []map[string]any:
			cloned[key] = cloneMapSlice(typed)
		case []string:
			cloned[key] = append([]string(nil), typed...)
		default:
			cloned[key] = value
		}
	}
	return cloned
}

func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneMap(value))
	}
	return cloned
}
