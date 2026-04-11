package builtin

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func requireStringField(input map[string]any, key string) (string, error) {
	value, ok := input[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("input field %q must be a non-empty string", key)
	}
	return strings.TrimSpace(value), nil
}

func optionalStringField(input map[string]any, key string) (string, error) {
	value, ok := input[key]
	if !ok {
		return "", nil
	}
	stringValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("input field %q must be a string when provided", key)
	}
	return strings.TrimSpace(stringValue), nil
}

func optionalStringSliceField(input map[string]any, key string) ([]string, error) {
	value, ok := input[key]
	if !ok {
		return nil, nil
	}

	result := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			result = append(result, strings.TrimSpace(item))
		}
	case []any:
		for _, item := range typed {
			stringItem, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("input field %q must contain only strings", key)
			}
			result = append(result, strings.TrimSpace(stringItem))
		}
	default:
		return nil, fmt.Errorf("input field %q must be a string array when provided", key)
	}

	return result, nil
}

func optionalPositiveLimitField(input map[string]any, key string, defaultValue int) (int, error) {
	value, ok := input[key]
	if !ok {
		return defaultValue, nil
	}

	switch typed := value.(type) {
	case int:
		if typed > 0 && typed < defaultValue {
			return typed, nil
		}
	case float64:
		intValue := int(typed)
		if intValue > 0 && intValue < defaultValue {
			return intValue, nil
		}
	default:
		return 0, fmt.Errorf("input field %q must be a number when provided", key)
	}

	return defaultValue, nil
}

func ensurePlatform(execCtx *tools.ToolExecuteContext) error {
	if execCtx == nil || execCtx.Platform == nil {
		return fmt.Errorf("%w: platform adapter is required", tools.ErrCapabilityDenied)
	}
	return nil
}

func ensureExecution(execCtx *tools.ToolExecuteContext) error {
	if execCtx == nil || execCtx.Execution == nil || execCtx.Platform == nil {
		return fmt.Errorf("%w: execution adapter is required", tools.ErrCapabilityDenied)
	}
	return nil
}

func previewString(input string, limit int) string {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit]
}
