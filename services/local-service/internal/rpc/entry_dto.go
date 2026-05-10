package rpc

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

var (
	requestSourceSet = map[string]struct{}{
		"floating_ball": {},
		"dashboard":     {},
		"tray_panel":    {},
	}
	inputSubmitTriggerSet = map[string]struct{}{
		"voice_commit":     {},
		"hover_text_input": {},
	}
	requestTriggerSet = map[string]struct{}{
		"voice_commit":         {},
		"hover_text_input":     {},
		"text_selected_click":  {},
		"file_drop":            {},
		"error_detected":       {},
		"recommendation_click": {},
	}
	inputModeSet = map[string]struct{}{
		"voice": {},
		"text":  {},
	}
	inputTypeSet = map[string]struct{}{
		"text":           {},
		"text_selection": {},
		"file":           {},
		"error":          {},
	}
	deliveryTypeSet = map[string]struct{}{
		"bubble":             {},
		"workspace_document": {},
		"result_page":        {},
		"open_file":          {},
		"reveal_in_folder":   {},
		"task_detail":        {},
	}
	browserKindSet = map[string]struct{}{
		"chrome":        {},
		"edge":          {},
		"other_browser": {},
		"non_browser":   {},
	}
)

// AgentInputSubmitParams reuses the orchestrator request DTO so the stable
// contract only needs to be maintained in one Go package.
type AgentInputSubmitParams = orchestrator.SubmitInputRequest

// AgentTaskStartParams reuses the orchestrator request DTO. Its json:"-"
// Intent field keeps unsupported client intent input out of the stable RPC
// contract while preserving the orchestrator's internal testing hook.
type AgentTaskStartParams = orchestrator.StartTaskRequest

// AgentTaskDetailGetParams reuses the orchestrator request DTO so stable task
// detail lookups validate the same typed contract the orchestrator consumes.
type AgentTaskDetailGetParams = orchestrator.TaskDetailGetRequest

func decodeAgentInputSubmitParams(raw json.RawMessage) (map[string]any, *rpcError) {
	var params AgentInputSubmitParams
	return decodeTypedProtocolParams(raw, &params, validateAgentInputSubmitParams)
}

func decodeAgentTaskStartParams(raw json.RawMessage) (map[string]any, *rpcError) {
	var params AgentTaskStartParams
	return decodeTypedProtocolParams(raw, &params, validateAgentTaskStartParams)
}

func decodeAgentTaskDetailGetParams(raw json.RawMessage) (map[string]any, *rpcError) {
	var params AgentTaskDetailGetParams
	return decodeTypedProtocolParams(raw, &params, validateAgentTaskDetailGetParams)
}

func decodeTypedProtocolParams(raw json.RawMessage, target any, validate func(map[string]any) *rpcError) (map[string]any, *rpcError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		trimmed = []byte("{}")
	}
	var payload map[string]any
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return nil, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "params do not match the registered method dto",
			TraceID: "trace_rpc_params",
		}
	}
	if validate != nil {
		if err := validate(payload); err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal(trimmed, target); err != nil {
		return nil, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "params do not match the registered method dto",
			TraceID: "trace_rpc_params",
		}
	}
	return protocolParamsMap(target)
}

func decodeParamsRequiringRequestMeta(raw json.RawMessage) (map[string]any, *rpcError) {
	return decodeParamsWithValidation(raw, requireRequestMeta)
}

func decodeParamsWithValidation(raw json.RawMessage, validate func(map[string]any) *rpcError) (map[string]any, *rpcError) {
	params, err := decodeParams(raw)
	if err != nil {
		return nil, err
	}
	if validate != nil {
		if err := validate(params); err != nil {
			return nil, err
		}
	}
	return params, nil
}

func validateAgentInputSubmitParams(params map[string]any) *rpcError {
	if err := requireRequestMeta(params); err != nil {
		return err
	}
	input, err := requireObject(params, "input")
	if err != nil {
		return err
	}
	if _, err := requireObject(params, "context"); err != nil {
		return err
	}
	if err := requireEnumValue(params, "source", requestSourceSet); err != nil {
		return err
	}
	if err := requireEnumValue(params, "trigger", inputSubmitTriggerSet); err != nil {
		return err
	}
	if err := requireExactString(input, "type", "text"); err != nil {
		return err
	}
	if err := requireEnumValue(input, "input_mode", inputModeSet); err != nil {
		return err
	}
	if err := validateContextEnvelope(params); err != nil {
		return err
	}
	options := mapObject(params, "options")
	if err := optionalEnumValue(options, "preferred_delivery", deliveryTypeSet); err != nil {
		return err
	}
	return nil
}

func validateAgentTaskStartParams(params map[string]any) *rpcError {
	if err := requireRequestMeta(params); err != nil {
		return err
	}
	input, err := requireObject(params, "input")
	if err != nil {
		return err
	}
	if err := requireEnumValue(params, "source", requestSourceSet); err != nil {
		return err
	}
	if err := requireEnumValue(params, "trigger", requestTriggerSet); err != nil {
		return err
	}
	if err := requireEnumValue(input, "type", inputTypeSet); err != nil {
		return err
	}
	if err := validateContextEnvelope(params); err != nil {
		return err
	}
	delivery := mapObject(params, "delivery")
	if err := optionalEnumValue(delivery, "preferred", deliveryTypeSet); err != nil {
		return err
	}
	if err := optionalEnumValue(delivery, "fallback", deliveryTypeSet); err != nil {
		return err
	}
	return nil
}

func validateAgentTaskDetailGetParams(params map[string]any) *rpcError {
	if err := requireRequestMeta(params); err != nil {
		return err
	}
	if err := requireNonEmptyString(params, "task_id"); err != nil {
		return err
	}
	return nil
}

func validateContextEnvelope(params map[string]any) *rpcError {
	pageContext := mapObject(mapObject(params, "input"), "page_context")
	if err := optionalEnumValue(pageContext, "browser_kind", browserKindSet); err != nil {
		return err
	}
	context := mapObject(params, "context")
	if len(context) == 0 {
		return nil
	}
	page := mapObject(context, "page")
	if err := optionalEnumValue(page, "browser_kind", browserKindSet); err != nil {
		return err
	}
	return nil
}

func requireRequestMeta(params map[string]any) *rpcError {
	requestMeta, err := requireObject(params, "request_meta")
	if err != nil {
		return err
	}
	if err := requireNonEmptyString(requestMeta, "trace_id"); err != nil {
		return err
	}
	if err := requireNonEmptyString(requestMeta, "client_time"); err != nil {
		return err
	}
	return nil
}

func requireObject(values map[string]any, key string) (map[string]any, *rpcError) {
	raw, ok := values[key]
	if !ok {
		return nil, invalidParamsError("missing required object field: " + key)
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, invalidParamsError("field must be a json object: " + key)
	}
	return object, nil
}

func mapObject(values map[string]any, key string) map[string]any {
	raw, ok := values[key]
	if !ok {
		return map[string]any{}
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return object
}

func requireNonEmptyString(values map[string]any, key string) *rpcError {
	raw, ok := values[key]
	if !ok {
		return invalidParamsError("missing required string field: " + key)
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return invalidParamsError("field must be a non-empty string: " + key)
	}
	return nil
}

func requireExactString(values map[string]any, key, expected string) *rpcError {
	if err := requireNonEmptyString(values, key); err != nil {
		return err
	}
	if values[key].(string) != expected {
		return invalidParamsError("field must equal " + expected + ": " + key)
	}
	return nil
}

func requireEnumValue(values map[string]any, key string, allowed map[string]struct{}) *rpcError {
	if err := requireNonEmptyString(values, key); err != nil {
		return err
	}
	return optionalEnumValue(values, key, allowed)
}

func optionalEnumValue(values map[string]any, key string, allowed map[string]struct{}) *rpcError {
	raw, ok := values[key]
	if !ok {
		return nil
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return invalidParamsError("field must be a non-empty string: " + key)
	}
	if _, ok := allowed[value]; !ok {
		return invalidParamsError("field is outside the stable enum domain: " + key)
	}
	return nil
}

func invalidParamsError(detail string) *rpcError {
	return &rpcError{
		Code:    errInvalidParams,
		Message: "INVALID_PARAMS",
		Detail:  detail,
		TraceID: "trace_rpc_params",
	}
}

func protocolParamsMap(value any) (map[string]any, *rpcError) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "params could not be normalized",
			TraceID: "trace_rpc_params",
		}
	}
	var params map[string]any
	if err := json.Unmarshal(payload, &params); err != nil {
		return nil, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "params normalized to an invalid object",
			TraceID: "trace_rpc_params",
		}
	}
	return params, nil
}
