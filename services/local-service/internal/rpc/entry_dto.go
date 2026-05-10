package rpc

import (
	"bytes"
	"encoding/json"
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

// RequestMeta mirrors packages/protocol RequestMeta and carries the trace
// anchor that error envelopes return to clients.
type RequestMeta struct {
	TraceID    string `json:"trace_id,omitempty"`
	ClientTime string `json:"client_time,omitempty"`
}

type pageContext struct {
	Title       string `json:"title,omitempty"`
	AppName     string `json:"app_name,omitempty"`
	URL         string `json:"url,omitempty"`
	BrowserKind string `json:"browser_kind,omitempty"`
	ProcessPath string `json:"process_path,omitempty"`
	ProcessID   int    `json:"process_id,omitempty"`
	WindowTitle string `json:"window_title,omitempty"`
	VisibleText string `json:"visible_text,omitempty"`
	HoverTarget string `json:"hover_target,omitempty"`
}

type screenContext struct {
	Summary       string `json:"summary,omitempty"`
	ScreenSummary string `json:"screen_summary,omitempty"`
	VisibleText   string `json:"visible_text,omitempty"`
	WindowTitle   string `json:"window_title,omitempty"`
	HoverTarget   string `json:"hover_target,omitempty"`
}

type behaviorContext struct {
	LastAction        string `json:"last_action,omitempty"`
	DwellMillis       int    `json:"dwell_millis,omitempty"`
	CopyCount         int    `json:"copy_count,omitempty"`
	WindowSwitchCount int    `json:"window_switch_count,omitempty"`
	PageSwitchCount   int    `json:"page_switch_count,omitempty"`
}

type selectionContext struct {
	Text string `json:"text,omitempty"`
}

type errorContext struct {
	Message string `json:"message,omitempty"`
}

type clipboardContext struct {
	Text string `json:"text,omitempty"`
}

// inputContext mirrors the stable InputContext envelope shared by
// agent.input.submit and agent.task.start. It stays typed at the RPC boundary,
// then converts to the orchestrator's normalized context capture payload.
type inputContext struct {
	Page              *pageContext      `json:"page,omitempty"`
	Screen            *screenContext    `json:"screen,omitempty"`
	Behavior          *behaviorContext  `json:"behavior,omitempty"`
	Selection         *selectionContext `json:"selection,omitempty"`
	Error             *errorContext     `json:"error,omitempty"`
	Clipboard         *clipboardContext `json:"clipboard,omitempty"`
	Text              string            `json:"text,omitempty"`
	SelectionText     string            `json:"selection_text,omitempty"`
	Files             []string          `json:"files,omitempty"`
	FilePaths         []string          `json:"file_paths,omitempty"`
	ScreenSummary     string            `json:"screen_summary,omitempty"`
	ClipboardText     string            `json:"clipboard_text,omitempty"`
	HoverTarget       string            `json:"hover_target,omitempty"`
	LastAction        string            `json:"last_action,omitempty"`
	DwellMillis       int               `json:"dwell_millis,omitempty"`
	CopyCount         int               `json:"copy_count,omitempty"`
	WindowSwitchCount int               `json:"window_switch_count,omitempty"`
	PageSwitchCount   int               `json:"page_switch_count,omitempty"`
}

type agentInputSubmitInput struct {
	Type      string `json:"type,omitempty"`
	Text      string `json:"text,omitempty"`
	InputMode string `json:"input_mode,omitempty"`
}

type voiceMeta struct {
	VoiceSessionID  string  `json:"voice_session_id,omitempty"`
	IsLockedSession bool    `json:"is_locked_session,omitempty"`
	ASRConfidence   float64 `json:"asr_confidence,omitempty"`
	SegmentID       string  `json:"segment_id,omitempty"`
}

type agentInputSubmitOptions struct {
	ConfirmRequired   bool   `json:"confirm_required,omitempty"`
	PreferredDelivery string `json:"preferred_delivery,omitempty"`
}

// AgentInputSubmitParams mirrors packages/protocol AgentInputSubmitParams for
// the Go RPC boundary.
type AgentInputSubmitParams struct {
	RequestMeta RequestMeta              `json:"request_meta"`
	SessionID   string                   `json:"session_id,omitempty"`
	Source      string                   `json:"source,omitempty"`
	Trigger     string                   `json:"trigger,omitempty"`
	Input       agentInputSubmitInput    `json:"input"`
	Context     *inputContext            `json:"context,omitempty"`
	VoiceMeta   *voiceMeta               `json:"voice_meta,omitempty"`
	Options     *agentInputSubmitOptions `json:"options,omitempty"`
}

type agentTaskStartInput struct {
	Type         string       `json:"type,omitempty"`
	Text         string       `json:"text,omitempty"`
	Files        []string     `json:"files,omitempty"`
	PageContext  *pageContext `json:"page_context,omitempty"`
	ErrorMessage string       `json:"error_message,omitempty"`
}

type deliveryPreference struct {
	Preferred string `json:"preferred,omitempty"`
	Fallback  string `json:"fallback,omitempty"`
}

type agentTaskStartOptions struct {
	ConfirmRequired bool `json:"confirm_required,omitempty"`
}

// AgentTaskStartParams mirrors packages/protocol AgentTaskStartParams. The
// struct intentionally has no intent field, so unsupported client intent input
// is dropped before the orchestrator suggests the authoritative task intent.
type AgentTaskStartParams struct {
	RequestMeta RequestMeta            `json:"request_meta"`
	SessionID   string                 `json:"session_id,omitempty"`
	Source      string                 `json:"source,omitempty"`
	Trigger     string                 `json:"trigger,omitempty"`
	Input       agentTaskStartInput    `json:"input"`
	Context     *inputContext          `json:"context,omitempty"`
	Delivery    *deliveryPreference    `json:"delivery,omitempty"`
	Options     *agentTaskStartOptions `json:"options,omitempty"`
}

// AgentTaskDetailGetParams mirrors packages/protocol AgentTaskDetailGetParams
// so stable detail requests can reject incomplete task identifiers at the RPC
// boundary instead of reaching orchestrator lookups as empty strings.
type AgentTaskDetailGetParams struct {
	RequestMeta RequestMeta `json:"request_meta"`
	TaskID      string      `json:"task_id"`
}

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

func validateAgentInputSubmitParams(params map[string]any) *rpcError {
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
	if err := requireNonEmptyString(params, "task_id"); err != nil {
		return err
	}
	return nil
}

func validateContextEnvelope(params map[string]any) *rpcError {
	context := mapObject(params, "context")
	if len(context) == 0 {
		return nil
	}
	page := mapObject(context, "page")
	if err := optionalEnumValue(page, "browser_kind", browserKindSet); err != nil {
		return err
	}
	pageContext := mapObject(mapObject(params, "input"), "page_context")
	if err := optionalEnumValue(pageContext, "browser_kind", browserKindSet); err != nil {
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
	if !ok || value == "" {
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
