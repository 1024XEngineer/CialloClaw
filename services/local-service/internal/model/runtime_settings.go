package model

import (
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"strings"
)

// CanonicalProviderName normalizes persisted settings aliases into the runtime
// provider identifiers registered by the model layer.
func CanonicalProviderName(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return ""
	}
	return OpenAIResponsesProvider
}

// IsOpenAICompatibleProviderAlias reports whether one control-panel provider
// label should resolve to the built-in OpenAI-compatible runtime route.
func IsOpenAICompatibleProviderAlias(provider string) bool {
	return strings.TrimSpace(provider) != ""
}

// RuntimeConfigFromSettings overlays the persisted `models` settings scope onto
// the boot-time model defaults. Blank settings fields keep the existing runtime
// defaults so an empty control-panel field does not erase a valid bootstrap
// endpoint or model identifier.
func RuntimeConfigFromSettings(base serviceconfig.ModelConfig, settings map[string]any) serviceconfig.ModelConfig {
	resolved := base
	models := runtimeSettingsMap(settings, "models")
	credentials := runtimeSettingsMap(models, "credentials")

	if provider := CanonicalProviderName(runtimeFirstNonEmptyString(runtimeSettingsString(models, "provider"), runtimeSettingsString(credentials, "provider"))); provider != "" {
		resolved.Provider = provider
	}
	if endpoint := runtimeFirstNonEmptyString(runtimeSettingsString(models, "base_url"), runtimeSettingsString(credentials, "base_url")); endpoint != "" {
		resolved.Endpoint = endpoint
	}
	if modelID := runtimeFirstNonEmptyString(runtimeSettingsString(models, "model"), runtimeSettingsString(credentials, "model")); modelID != "" {
		resolved.ModelID = modelID
	}
	if value, ok := runtimeSettingsBool(models, "budget_auto_downgrade"); ok {
		resolved.BudgetAutoDowngrade = value
	} else if value, ok := runtimeSettingsBool(credentials, "budget_auto_downgrade"); ok {
		resolved.BudgetAutoDowngrade = value
	}

	budgetPolicy := runtimeSettingsMap(credentials, "budget_policy")
	if len(budgetPolicy) == 0 {
		budgetPolicy = runtimeSettingsMap(models, "budget_policy")
	}
	if value, ok := runtimeSettingsInt(budgetPolicy, "planner_retry_budget"); ok {
		resolved.PlannerRetryBudget = value
	}
	if value, ok := runtimeSettingsInt(budgetPolicy, "tool_retry_budget"); ok {
		resolved.ToolRetryBudget = value
	}
	if value, ok := runtimeSettingsInt(budgetPolicy, "max_tool_iterations"); ok {
		resolved.MaxToolIterations = value
	}
	if value, ok := runtimeSettingsInt(budgetPolicy, "context_compress_chars"); ok {
		resolved.ContextCompressChars = value
	}
	if value, ok := runtimeSettingsInt(budgetPolicy, "context_keep_recent"); ok {
		resolved.ContextKeepRecent = value
	}

	return resolved
}

func runtimeSettingsMap(source map[string]any, key string) map[string]any {
	if source == nil {
		return nil
	}
	value, ok := source[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func runtimeSettingsString(source map[string]any, key string) string {
	if source == nil {
		return ""
	}
	value, ok := source[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func runtimeSettingsBool(source map[string]any, key string) (bool, bool) {
	if source == nil {
		return false, false
	}
	value, ok := source[key].(bool)
	return value, ok
}

func runtimeSettingsInt(source map[string]any, key string) (int, bool) {
	if source == nil {
		return 0, false
	}
	switch value := source[key].(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

func runtimeFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
