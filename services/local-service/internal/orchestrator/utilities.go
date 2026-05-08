package orchestrator

import (
	"path/filepath"
	"strings"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

// currentTimeFromTask returns the latest task update time formatted for bubble
// payloads.
func currentTimeFromTask(engine *runengine.Engine, taskID string) string {
	task, ok := engine.GetTask(taskID)
	if !ok {
		return ""
	}
	return task.UpdatedAt.Format(dateTimeLayout)
}

// currentRuntimeWorkspaceRoot returns the workspace root that the currently
// running local-service instance is actually using. This avoids displaying or
// evaluating against a pending settings value before the required restart
// rebuilds bootstrap-scoped dependencies.
func currentRuntimeWorkspaceRoot(executorService *execution.Service) string {
	if executorService != nil {
		if workspaceRoot := strings.TrimSpace(executorService.WorkspaceRoot()); workspaceRoot != "" {
			return filepath.ToSlash(filepath.Clean(workspaceRoot))
		}
	}
	return filepath.ToSlash(filepath.Clean(serviceconfig.DefaultWorkspaceRoot()))
}

func compactAuditRecords(records ...map[string]any) []map[string]any {
	if len(records) == 0 {
		return nil
	}

	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		items = append(items, cloneMap(record))
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func sameDay(left, right time.Time) bool {
	left = left.In(right.Location())
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

func intValueFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func floatValueFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0.0
	}
}

// firstMapOrNil returns a copy of the first item in a list, or nil when empty.
func firstMapOrNil(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return nil
	}
	return cloneMap(items[0])
}

// cloneMap recursively copies a map[string]any payload.
func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneMap(typed)
		case []map[string]any:
			result[key] = cloneMapSlice(typed)
		case []string:
			result[key] = append([]string(nil), typed...)
		default:
			result[key] = value
		}
	}
	return result
}

func optionalFormalDeliveryResult(deliveryResult map[string]any) any {
	if len(deliveryResult) == 0 {
		return nil
	}
	return deliveryResult
}

// cloneMapSlice recursively copies a []map[string]any payload.
func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, cloneMap(value))
	}
	return result
}

func extensionAssetReferencesFromMaps(values []map[string]any) []storage.ExtensionAssetReference {
	if len(values) == 0 {
		return nil
	}
	items := make([]storage.ExtensionAssetReference, 0, len(values))
	for _, value := range values {
		items = append(items, storage.ExtensionAssetReference{
			AssetKind:    stringValue(value, "asset_kind", ""),
			AssetID:      stringValue(value, "asset_id", ""),
			Name:         stringValue(value, "name", ""),
			Version:      stringValue(value, "version", ""),
			Source:       stringValue(value, "source", ""),
			Summary:      stringValue(value, "summary", ""),
			Entry:        stringValue(value, "entry", ""),
			Capabilities: stringSliceValue(value["capabilities"]),
			Permissions:  stringSliceValue(value["permissions"]),
			RuntimeNames: stringSliceValue(value["runtime_names"]),
		})
	}
	return items
}

// mapValue safely reads a nested object field.
func mapValue(values map[string]any, key string) map[string]any {
	rawValue, ok := values[key]
	if !ok {
		return map[string]any{}
	}
	value, ok := rawValue.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return value
}

// stringValue safely reads a string field and falls back when empty.
func stringValue(values map[string]any, key, fallback string) string {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	value, ok := rawValue.(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func requestTraceID(values map[string]any) string {
	return stringValue(mapValue(values, "request_meta"), "trace_id", "")
}

// boolValue safely reads a boolean field.
func boolValue(values map[string]any, key string, fallback bool) bool {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	value, ok := rawValue.(bool)
	if !ok {
		return fallback
	}
	return value
}

// intValue safely reads a JSON-decoded numeric field.
func intValue(values map[string]any, key string, fallback int) int {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	switch value := rawValue.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}

// truncateText trims text to a fixed length for recommendation and memory
// query surfaces.
func truncateText(value string, maxLength int) string {
	return textutil.TruncateGraphemes(value, maxLength)
}

func stringSliceValue(rawValue any) []string {
	values, ok := rawValue.([]string)
	if ok {
		return append([]string(nil), values...)
	}

	anyValues, ok := rawValue.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(anyValues))
	for _, rawItem := range anyValues {
		item, ok := rawItem.(string)
		if ok && strings.TrimSpace(item) != "" {
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
