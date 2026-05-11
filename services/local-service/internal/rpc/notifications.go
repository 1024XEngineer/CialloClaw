package rpc

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// taskIDsFromResponse extracts task identifiers from successful RPC payloads
// so transports can replay matching buffered notifications.
func taskIDsFromResponse(response any) []string {
	success, ok := response.(successEnvelope)
	if !ok {
		return nil
	}

	ids := map[string]struct{}{}
	collectTaskIDs(success.Result.Data, ids)

	result := make([]string, 0, len(ids))
	for taskID := range ids {
		result = append(result, taskID)
	}

	return result
}

func requestRoutingHints(request requestEnvelope) (map[string]bool, string, string) {
	params, rpcErr := decodeParams(request.Params)
	if rpcErr != nil {
		return nil, "", ""
	}

	ids := map[string]struct{}{}
	collectTaskIDs(params, ids)
	var result map[string]bool
	if len(ids) > 0 {
		result = make(map[string]bool, len(ids))
		for taskID := range ids {
			result[taskID] = true
		}
	}
	return result, stringValue(params, "session_id", ""), stringValue(mapValue(params, "request_meta"), "trace_id", "")
}

func shouldTrackStartedTask(method string) bool {
	return method == methodAgentTaskStart || method == methodAgentInputSubmit || method == methodAgentNotepadConvertToTask
}

// shouldClaimResponseTaskOwnership scopes late response-based task ownership to
// methods that legitimately create or discover the task at runtime.
func shouldClaimResponseTaskOwnership(method string) bool {
	return shouldTrackStartedTask(method)
}

func ownedTaskIDsForReplay(method string, trackedTaskIDs map[string]bool, response any) []string {
	owned := map[string]bool{}
	for taskID, tracked := range trackedTaskIDs {
		trimmed := strings.TrimSpace(taskID)
		if tracked && trimmed != "" {
			owned[trimmed] = true
		}
	}
	if shouldClaimResponseTaskOwnership(method) {
		for _, taskID := range taskIDsFromResponse(response) {
			trimmed := strings.TrimSpace(taskID)
			if trimmed != "" {
				owned[trimmed] = true
			}
		}
	}
	if len(owned) == 0 {
		return nil
	}
	result := make([]string, 0, len(owned))
	for taskID := range owned {
		result = append(result, taskID)
	}
	sort.Strings(result)
	return result
}

func isLiveRuntimeMethod(method string) bool {
	return strings.HasPrefix(method, "loop.") || method == "task.steered"
}

func runtimeNotificationTaskID(taskID string, params map[string]any) string {
	if strings.TrimSpace(taskID) != "" {
		return taskID
	}
	if params == nil {
		return ""
	}
	rawTaskID, _ := params["task_id"].(string)
	return strings.TrimSpace(rawTaskID)
}

func notificationKey(method, taskID string, params map[string]any) string {
	encoded, err := json.Marshal(normalizeNotificationKey(method, taskID, params))
	if err != nil {
		return method
	}
	return method + ":" + string(encoded)
}

func normalizeNotificationKey(method, taskID string, params map[string]any) map[string]any {
	if !isLiveRuntimeMethod(method) {
		return map[string]any{
			"task_id": strings.TrimSpace(taskID),
			"params":  params,
		}
	}

	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		normalizedTaskID = runtimeNotificationTaskID("", params)
	}

	payload := map[string]any{}
	if event := mapValue(params, "event"); len(event) > 0 {
		payload = mapValue(event, "payload")
	} else {
		for key, value := range params {
			if key == "task_id" {
				continue
			}
			payload[key] = value
		}
	}

	return map[string]any{
		"task_id": normalizedTaskID,
		"type":    method,
		"payload": payload,
	}
}

// collectTaskIDs walks arbitrary decoded payloads and gathers every field with
// a task_id suffix.
func collectTaskIDs(rawValue any, ids map[string]struct{}) {
	collectTaskIDsValue(reflect.ValueOf(rawValue), "", ids)
}

func collectTaskIDsValue(value reflect.Value, fieldName string, ids map[string]struct{}) {
	if !value.IsValid() {
		return
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.String:
		if strings.HasSuffix(fieldName, "task_id") {
			if taskID := strings.TrimSpace(value.String()); taskID != "" {
				ids[taskID] = struct{}{}
			}
		}
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return
		}
		iter := value.MapRange()
		for iter.Next() {
			collectTaskIDsValue(iter.Value(), iter.Key().String(), ids)
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < value.Len(); index++ {
			collectTaskIDsValue(value.Index(index), fieldName, ids)
		}
	case reflect.Struct:
		valueType := value.Type()
		for index := 0; index < value.NumField(); index++ {
			field := valueType.Field(index)
			if !field.IsExported() {
				continue
			}
			jsonName := notificationJSONFieldName(field)
			if jsonName == "" {
				continue
			}
			collectTaskIDsValue(value.Field(index), jsonName, ids)
		}
	}
}

func notificationJSONFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	if tag == "" {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return field.Name
	}
	return name
}
