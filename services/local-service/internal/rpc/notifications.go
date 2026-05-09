package rpc

import (
	"encoding/json"
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
	return method == methodAgentTaskStart || method == methodAgentInputSubmit
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
	switch value := rawValue.(type) {
	case interface{ Map() map[string]any }:
		collectTaskIDs(value.Map(), ids)
	case map[string]any:
		for key, item := range value {
			if strings.HasSuffix(key, "task_id") {
				if taskID, ok := item.(string); ok && taskID != "" {
					ids[taskID] = struct{}{}
				}
			}
			collectTaskIDs(item, ids)
		}
	case []map[string]any:
		for _, item := range value {
			collectTaskIDs(item, ids)
		}
	case []any:
		for _, item := range value {
			collectTaskIDs(item, ids)
		}
	}
}
