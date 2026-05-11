package rpc

import (
	"bytes"
	"encoding/json"
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

type intentPayload struct {
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// AgentTaskConfirmParams mirrors packages/protocol AgentTaskConfirmParams and
// keeps correction fields typed at the RPC boundary so malformed payloads fail
// before confirm-flow business logic tries to reinterpret them.
type AgentTaskConfirmParams struct {
	RequestMeta     RequestMeta    `json:"request_meta"`
	TaskID          string         `json:"task_id,omitempty"`
	Confirmed       bool           `json:"confirmed,omitempty"`
	CorrectedIntent *intentPayload `json:"corrected_intent,omitempty"`
	CorrectionText  *string        `json:"correction_text,omitempty"`
}

func decodeAgentInputSubmitParams(raw json.RawMessage) (map[string]any, *rpcError) {
	var params AgentInputSubmitParams
	return decodeTypedProtocolParams(raw, &params)
}

func decodeAgentTaskStartParams(raw json.RawMessage) (map[string]any, *rpcError) {
	var params AgentTaskStartParams
	return decodeTypedProtocolParams(raw, &params)
}

func decodeAgentTaskConfirmParams(raw json.RawMessage) (map[string]any, *rpcError) {
	var params AgentTaskConfirmParams
	return decodeTypedProtocolParams(raw, &params)
}

func decodeTypedProtocolParams(raw json.RawMessage, target any) (map[string]any, *rpcError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		trimmed = []byte("{}")
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
