package orchestrator

// RequestMeta carries the protocol trace anchor attached to stable RPC calls.
type RequestMeta struct {
	TraceID    string `json:"trace_id,omitempty"`
	ClientTime string `json:"client_time,omitempty"`
}

// PageContext describes the foreground page or host application that anchored
// a task entry request.
type PageContext struct {
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

// ScreenContext carries screen-derived signals used to infer controlled visual
// tasks without adding a separate public task object.
type ScreenContext struct {
	Summary       string `json:"summary,omitempty"`
	ScreenSummary string `json:"screen_summary,omitempty"`
	VisibleText   string `json:"visible_text,omitempty"`
	WindowTitle   string `json:"window_title,omitempty"`
	HoverTarget   string `json:"hover_target,omitempty"`
}

// BehaviorContext carries lightweight interaction signals that help route a
// task entry without becoming a business state machine.
type BehaviorContext struct {
	LastAction        string `json:"last_action,omitempty"`
	DwellMillis       int    `json:"dwell_millis,omitempty"`
	CopyCount         int    `json:"copy_count,omitempty"`
	WindowSwitchCount int    `json:"window_switch_count,omitempty"`
	PageSwitchCount   int    `json:"page_switch_count,omitempty"`
}

// SelectionContext carries selected text for task entry inference.
type SelectionContext struct {
	Text string `json:"text,omitempty"`
}

// ErrorContext carries foreground error text for task entry inference.
type ErrorContext struct {
	Message string `json:"message,omitempty"`
}

// ClipboardContext carries clipboard text for task entry inference.
type ClipboardContext struct {
	Text string `json:"text,omitempty"`
}

// InputContext is the stable context envelope shared by task entry methods.
// The orchestrator converts it back to its normalized capture payload before
// intent inference so context semantics stay centralized in the context service.
type InputContext struct {
	Page              *PageContext      `json:"page,omitempty"`
	Screen            *ScreenContext    `json:"screen,omitempty"`
	Behavior          *BehaviorContext  `json:"behavior,omitempty"`
	Selection         *SelectionContext `json:"selection,omitempty"`
	Error             *ErrorContext     `json:"error,omitempty"`
	Clipboard         *ClipboardContext `json:"clipboard,omitempty"`
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

// VoiceMeta carries voice-entry metadata for agent.input.submit.
type VoiceMeta struct {
	VoiceSessionID  string  `json:"voice_session_id,omitempty"`
	IsLockedSession bool    `json:"is_locked_session,omitempty"`
	ASRConfidence   float64 `json:"asr_confidence,omitempty"`
	SegmentID       string  `json:"segment_id,omitempty"`
}

// InputSubmitInput is the formal input object for agent.input.submit.
type InputSubmitInput struct {
	Type      string `json:"type,omitempty"`
	Text      string `json:"text,omitempty"`
	InputMode string `json:"input_mode,omitempty"`
}

// InputSubmitOptions carries execution preferences for agent.input.submit.
type InputSubmitOptions struct {
	ConfirmRequired   bool   `json:"confirm_required,omitempty"`
	PreferredDelivery string `json:"preferred_delivery,omitempty"`
}

// SubmitInputRequest is the typed orchestrator boundary for agent.input.submit.
type SubmitInputRequest struct {
	RequestMeta RequestMeta         `json:"request_meta"`
	SessionID   string              `json:"session_id,omitempty"`
	Source      string              `json:"source,omitempty"`
	Trigger     string              `json:"trigger,omitempty"`
	Input       InputSubmitInput    `json:"input"`
	Context     *InputContext       `json:"context,omitempty"`
	VoiceMeta   *VoiceMeta          `json:"voice_meta,omitempty"`
	Options     *InputSubmitOptions `json:"options,omitempty"`

	raw map[string]any
}

// TaskStartInput is the formal input object for agent.task.start.
type TaskStartInput struct {
	Type         string       `json:"type,omitempty"`
	Text         string       `json:"text,omitempty"`
	Files        []string     `json:"files,omitempty"`
	PageContext  *PageContext `json:"page_context,omitempty"`
	ErrorMessage string       `json:"error_message,omitempty"`
}

// DeliveryPreference carries the formal delivery preference for task starts.
type DeliveryPreference struct {
	Preferred string `json:"preferred,omitempty"`
	Fallback  string `json:"fallback,omitempty"`
}

// TaskStartOptions carries task-start execution preferences.
type TaskStartOptions struct {
	ConfirmRequired bool `json:"confirm_required,omitempty"`
}

// StartTaskRequest is the typed orchestrator boundary for agent.task.start.
// Intent is an orchestrator-only testing and resume hook; the RPC decoder never
// admits an external intent field for agent.task.start.
type StartTaskRequest struct {
	RequestMeta RequestMeta         `json:"request_meta"`
	SessionID   string              `json:"session_id,omitempty"`
	Source      string              `json:"source,omitempty"`
	Trigger     string              `json:"trigger,omitempty"`
	Input       TaskStartInput      `json:"input"`
	Context     *InputContext       `json:"context,omitempty"`
	Delivery    *DeliveryPreference `json:"delivery,omitempty"`
	Options     *TaskStartOptions   `json:"options,omitempty"`
	Intent      map[string]any      `json:"-"`

	raw map[string]any
}

// TaskDetailGetRequest is the typed orchestrator boundary for
// agent.task.detail.get.
type TaskDetailGetRequest struct {
	RequestMeta RequestMeta `json:"request_meta"`
	TaskID      string      `json:"task_id"`

	raw map[string]any
}
