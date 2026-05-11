package rpc

import (
	"bytes"
	"encoding/json"
	"math"
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
	taskListGroupSet = map[string]struct{}{
		"unfinished": {},
		"finished":   {},
	}
	taskListSortBySet = map[string]struct{}{
		"updated_at":  {},
		"started_at":  {},
		"finished_at": {},
	}
	taskListSortOrderSet = map[string]struct{}{
		"asc":  {},
		"desc": {},
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

type intentPayload struct {
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// AgentTaskConfirmParams mirrors packages/protocol AgentTaskConfirmParams and
// keeps correction fields typed at the RPC boundary so malformed payloads fail
// before confirm-flow business logic tries to reinterpret them.
type AgentTaskConfirmParams struct {
	RequestMeta     orchestrator.RequestMeta `json:"request_meta"`
	TaskID          string                   `json:"task_id,omitempty"`
	Confirmed       bool                     `json:"confirmed,omitempty"`
	CorrectedIntent *intentPayload           `json:"corrected_intent,omitempty"`
	CorrectionText  *string                  `json:"correction_text,omitempty"`
}

func decodeAgentInputSubmitParams(raw json.RawMessage) (map[string]any, *rpcError) {
	return decodeTypedProtocolParams(raw, validateAgentInputSubmitParams, orchestrator.SubmitInputRequestFromParams, func(request AgentInputSubmitParams) map[string]any {
		return request.ProtocolParamsMap()
	})
}

func decodeAgentTaskStartParams(raw json.RawMessage) (map[string]any, *rpcError) {
	return decodeTypedProtocolParams(raw, validateAgentTaskStartParams, func(params map[string]any) AgentTaskStartParams {
		request := orchestrator.StartTaskRequestFromParams(params)
		request.Intent = nil
		return request
	}, func(request AgentTaskStartParams) map[string]any {
		return request.ProtocolParamsMap()
	})
}

func decodeAgentTaskDetailGetParams(raw json.RawMessage) (map[string]any, *rpcError) {
	return decodeTypedProtocolParams(raw, validateAgentTaskDetailGetParams, orchestrator.TaskDetailGetRequestFromParams, func(request AgentTaskDetailGetParams) map[string]any {
		return request.ProtocolParamsMap()
	})
}

func decodeAgentTaskConfirmParams(raw json.RawMessage) (map[string]any, *rpcError) {
	return decodeParamsWithValidation(raw, validateAgentTaskConfirmParams)
}

func decodeAgentTaskListParams(raw json.RawMessage) (map[string]any, *rpcError) {
	return decodeParamsWithValidation(raw, validateAgentTaskListParams)
}

func decodeTypedProtocolParams[T any](raw json.RawMessage, validate func(map[string]any) *rpcError, fromParams func(map[string]any) T, normalize func(T) map[string]any) (map[string]any, *rpcError) {
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
	if fromParams != nil && normalize != nil {
		typedPayload := fromParams(payload)
		return normalize(typedPayload), nil
	}
	return map[string]any{}, nil
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
	if err := optionalStringField(params, "session_id"); err != nil {
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
	if err := requireNonEmptyString(input, "text"); err != nil {
		return err
	}
	if err := requireEnumValue(input, "input_mode", inputModeSet); err != nil {
		return err
	}
	if err := validateContextEnvelope(params); err != nil {
		return err
	}
	if err := validateVoiceMeta(params); err != nil {
		return err
	}
	options, err := optionalObject(params, "options")
	if err != nil {
		return err
	}
	if err := optionalEnumValue(options, "preferred_delivery", deliveryTypeSet); err != nil {
		return err
	}
	if err := optionalBoolField(options, "confirm_required"); err != nil {
		return err
	}
	return nil
}

func validateAgentTaskStartParams(params map[string]any) *rpcError {
	if err := requireRequestMeta(params); err != nil {
		return err
	}
	if err := optionalStringField(params, "session_id"); err != nil {
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
	if err := validateTaskStartInputPayload(input, mapObject(params, "context")); err != nil {
		return err
	}
	if err := validateContextEnvelope(params); err != nil {
		return err
	}
	delivery, err := optionalObject(params, "delivery")
	if err != nil {
		return err
	}
	if err := optionalEnumValue(delivery, "preferred", deliveryTypeSet); err != nil {
		return err
	}
	if err := optionalEnumValue(delivery, "fallback", deliveryTypeSet); err != nil {
		return err
	}
	options, err := optionalObject(params, "options")
	if err != nil {
		return err
	}
	if err := optionalBoolField(options, "confirm_required"); err != nil {
		return err
	}
	return nil
}

func validateAgentTaskConfirmParams(params map[string]any) *rpcError {
	if err := requireRequestMeta(params); err != nil {
		return err
	}
	if err := requireNonEmptyString(params, "task_id"); err != nil {
		return err
	}
	if err := optionalBoolField(params, "confirmed"); err != nil {
		return err
	}
	if err := optionalStringField(params, "correction_text"); err != nil {
		return err
	}
	correctedIntent, err := optionalObject(params, "corrected_intent")
	if err != nil {
		return err
	}
	if len(correctedIntent) == 0 {
		return nil
	}
	if err := requireNonEmptyString(correctedIntent, "name"); err != nil {
		return err
	}
	if _, err := optionalObject(correctedIntent, "arguments"); err != nil {
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

func validateAgentTaskListParams(params map[string]any) *rpcError {
	if err := requireRequestMeta(params); err != nil {
		return err
	}
	if err := requireEnumValue(params, "group", taskListGroupSet); err != nil {
		return err
	}
	if err := requireInteger(params, "limit"); err != nil {
		return err
	}
	if err := requireInteger(params, "offset"); err != nil {
		return err
	}
	if err := optionalEnumValue(params, "sort_by", taskListSortBySet); err != nil {
		return err
	}
	if err := optionalEnumValue(params, "sort_order", taskListSortOrderSet); err != nil {
		return err
	}
	return nil
}

// validateTaskStartInputPayload keeps the stable task-start DTO aligned with
// the context capture fallback rules. Structured starts must still carry the
// evidence that identifies the selected text, files, or error payload even when
// the caller provides that evidence via the broader context envelope.
func validateTaskStartInputPayload(input, context map[string]any) *rpcError {
	switch stringValue(input, "type", "") {
	case "text":
		if !hasNonEmptyString(input, "text") && !hasNonEmptyString(context, "text") {
			return invalidParamsError("text task input requires text")
		}
	case "text_selection":
		selection := mapObject(context, "selection")
		if !hasNonEmptyString(input, "text") &&
			!hasNonEmptyString(selection, "text") &&
			!hasNonEmptyString(context, "selection_text") {
			return invalidParamsError("text_selection task input requires selected text")
		}
	case "file":
		if !hasNonEmptyStringSlice(input, "files") &&
			!hasNonEmptyStringSlice(context, "files") &&
			!hasNonEmptyStringSlice(context, "file_paths") {
			return invalidParamsError("file task input requires at least one file")
		}
	case "error":
		errorContext := mapObject(context, "error")
		if !hasNonEmptyString(input, "error_message") &&
			!hasNonEmptyString(errorContext, "message") {
			return invalidParamsError("error task input requires an error message")
		}
	}
	return nil
}

func validateContextEnvelope(params map[string]any) *rpcError {
	input := mapObject(params, "input")
	pageContext, err := optionalObject(input, "page_context")
	if err != nil {
		return err
	}
	if err := validatePageContext(pageContext); err != nil {
		return err
	}
	context, err := optionalObject(params, "context")
	if err != nil {
		return err
	}
	if len(context) == 0 {
		return nil
	}
	page, err := optionalObject(context, "page")
	if err != nil {
		return err
	}
	if err := validatePageContext(page); err != nil {
		return err
	}
	screen, err := optionalObject(context, "screen")
	if err != nil {
		return err
	}
	if err := validateStringFields(screen, "summary", "screen_summary", "visible_text", "window_title", "hover_target"); err != nil {
		return err
	}
	behavior, err := optionalObject(context, "behavior")
	if err != nil {
		return err
	}
	if err := validateStringFields(behavior, "last_action"); err != nil {
		return err
	}
	if err := validateIntegerFields(behavior, "dwell_millis", "copy_count", "window_switch_count", "page_switch_count"); err != nil {
		return err
	}
	selection, err := optionalObject(context, "selection")
	if err != nil {
		return err
	}
	if err := validateStringFields(selection, "text"); err != nil {
		return err
	}
	errorContext, err := optionalObject(context, "error")
	if err != nil {
		return err
	}
	if err := validateStringFields(errorContext, "message"); err != nil {
		return err
	}
	clipboard, err := optionalObject(context, "clipboard")
	if err != nil {
		return err
	}
	if err := validateStringFields(clipboard, "text"); err != nil {
		return err
	}
	if err := validateStringFields(context, "text", "selection_text", "screen_summary", "clipboard_text", "hover_target", "last_action"); err != nil {
		return err
	}
	if err := validateIntegerFields(context, "dwell_millis", "copy_count", "window_switch_count", "page_switch_count"); err != nil {
		return err
	}
	if err := optionalStringSliceField(context, "files"); err != nil {
		return err
	}
	if err := optionalStringSliceField(context, "file_paths"); err != nil {
		return err
	}
	return nil
}

func validateVoiceMeta(params map[string]any) *rpcError {
	voiceMeta, err := optionalObject(params, "voice_meta")
	if err != nil {
		return err
	}
	if err := validateStringFields(voiceMeta, "voice_session_id", "segment_id"); err != nil {
		return err
	}
	if err := optionalBoolField(voiceMeta, "is_locked_session"); err != nil {
		return err
	}
	if err := optionalNumberField(voiceMeta, "asr_confidence"); err != nil {
		return err
	}
	return nil
}

func validatePageContext(values map[string]any) *rpcError {
	if err := validateStringFields(values, "title", "app_name", "url", "process_path", "window_title", "visible_text", "hover_target"); err != nil {
		return err
	}
	if err := optionalEnumValue(values, "browser_kind", browserKindSet); err != nil {
		return err
	}
	if err := validateIntegerFields(values, "process_id"); err != nil {
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

func optionalObject(values map[string]any, key string) (map[string]any, *rpcError) {
	raw, ok := values[key]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, invalidParamsError("field must be a json object: " + key)
	}
	return object, nil
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

func requireInteger(values map[string]any, key string) *rpcError {
	raw, ok := values[key]
	if !ok {
		return invalidParamsError("missing required integer field: " + key)
	}
	value, ok := raw.(float64)
	if !ok || math.Trunc(value) != value {
		return invalidParamsError("field must be an integer: " + key)
	}
	return nil
}

func optionalIntegerField(values map[string]any, key string) *rpcError {
	raw, ok := values[key]
	if !ok || raw == nil {
		return nil
	}
	value, ok := raw.(float64)
	if !ok || math.Trunc(value) != value {
		return invalidParamsError("field must be an integer: " + key)
	}
	return nil
}

func optionalNumberField(values map[string]any, key string) *rpcError {
	raw, ok := values[key]
	if !ok || raw == nil {
		return nil
	}
	switch raw.(type) {
	case float64:
		return nil
	default:
		return invalidParamsError("field must be a number: " + key)
	}
}

func optionalBoolField(values map[string]any, key string) *rpcError {
	raw, ok := values[key]
	if !ok || raw == nil {
		return nil
	}
	if _, ok := raw.(bool); !ok {
		return invalidParamsError("field must be a boolean: " + key)
	}
	return nil
}

func optionalStringField(values map[string]any, key string) *rpcError {
	raw, ok := values[key]
	if !ok || raw == nil {
		return nil
	}
	if _, ok := raw.(string); !ok {
		return invalidParamsError("field must be a string: " + key)
	}
	return nil
}

func optionalStringSliceField(values map[string]any, key string) *rpcError {
	raw, ok := values[key]
	if !ok || raw == nil {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return invalidParamsError("field must be an array of strings: " + key)
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return invalidParamsError("field must be an array of strings: " + key)
		}
	}
	return nil
}

func validateStringFields(values map[string]any, keys ...string) *rpcError {
	for _, key := range keys {
		if err := optionalStringField(values, key); err != nil {
			return err
		}
	}
	return nil
}

func validateIntegerFields(values map[string]any, keys ...string) *rpcError {
	for _, key := range keys {
		if err := optionalIntegerField(values, key); err != nil {
			return err
		}
	}
	return nil
}

func hasNonEmptyString(values map[string]any, key string) bool {
	raw, ok := values[key]
	if !ok {
		return false
	}
	value, ok := raw.(string)
	return ok && strings.TrimSpace(value) != ""
}

func hasNonEmptyStringSlice(values map[string]any, key string) bool {
	raw, ok := values[key]
	if !ok {
		return false
	}
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		value, ok := item.(string)
		if ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
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
