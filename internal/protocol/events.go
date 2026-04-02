package protocol

import (
	"time"

	"cialloclaw/internal/runtime/id"
)

type EventPriority int

const (
	PriorityLow EventPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

type Event struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Version      string            `json:"version"`
	SessionID    string            `json:"session_id"`
	TaskID       *string           `json:"task_id,omitempty"`
	TaskStepID   *string           `json:"task_step_id,omitempty"`
	LoopID       *string           `json:"loop_id,omitempty"`
	ApprovalID   *string           `json:"approval_id,omitempty"`
	Source       string            `json:"source"`
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID *string           `json:"parent_span_id,omitempty"`
	Priority     EventPriority     `json:"priority"`
	Timestamp    time.Time         `json:"timestamp"`
	Headers      map[string]string `json:"headers,omitempty"`
	Payload      any               `json:"payload,omitempty"`
}

const (
	EventTypeServiceStarted        = "service.started"
	EventTypeServiceStopped        = "service.stopped"
	EventTypeSystemError           = "system.error"
	EventTypeUserInputReceived     = "user.input.received"
	EventTypeClipboardChanged      = "clipboard.changed"
	EventTypeContextUpdated        = "context.updated"
	EventTypeIntentRecognized      = "intent.recognized"
	EventTypeTaskCreated           = "task.created"
	EventTypeTaskPlanned           = "task.planned"
	EventTypeTaskAssigned          = "task.assigned"
	EventTypeTaskExecutionStarted  = "task.execution.started"
	EventTypeToolCallRequested     = "tool.call.requested"
	EventTypeToolCallCompleted     = "tool.call.completed"
	EventTypeApprovalRequested     = "approval.requested"
	EventTypeApprovalResponded     = "approval.responded"
	EventTypeTaskExecutionComplete = "task.execution.completed"
	EventTypeTaskExecutionFailed   = "task.execution.failed"
	EventTypeUIMessageDisplay      = "ui.message.display"
	EventTypeVideoLinkDetected     = "video.link.detected"
	EventTypeFileChanged           = "file.changed"
	EventTypeTodoScanCompleted     = "todo.scan.completed"
)

type UserInputPayload struct {
	Text            string         `json:"text"`
	Raw             string         `json:"raw"`
	InputSource     string         `json:"input_source"`
	RequestedAction string         `json:"requested_action,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type ClipboardChangedPayload struct {
	Text       string `json:"text"`
	Kind       string `json:"kind"`
	IsURL      bool   `json:"is_url"`
	IsVideoURL bool   `json:"is_video_url"`
}

type ContextUpdatedPayload struct {
	Summary       string   `json:"summary"`
	Clipboard     *string  `json:"clipboard,omitempty"`
	ClipboardKind string   `json:"clipboard_kind,omitempty"`
	RecentEventID []string `json:"recent_event_ids,omitempty"`
}

type IntentRecognizedPayload struct {
	Intent          string         `json:"intent"`
	Confidence      float64        `json:"confidence"`
	Entities        map[string]any `json:"entities,omitempty"`
	RawInput        string         `json:"raw_input"`
	RequestedAction string         `json:"requested_action,omitempty"`
}

type TaskCreatedPayload struct {
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	Goal         string  `json:"goal"`
	ParentTaskID *string `json:"parent_task_id,omitempty"`
}

type PlannedStep struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ToolHint    *string `json:"tool_hint,omitempty"`
	AgentHint   *string `json:"agent_hint,omitempty"`
}

type TaskPlannedPayload struct {
	PlanID string        `json:"plan_id"`
	Steps  []PlannedStep `json:"steps"`
}

type TaskAssignedPayload struct {
	AgentName string `json:"agent_name"`
}

type ToolCallRequestedPayload struct {
	ToolName        string         `json:"tool_name"`
	Args            map[string]any `json:"args"`
	RiskLevel       string         `json:"risk_level"`
	RequireApproval bool           `json:"require_approval"`
}

type ApprovalRequestedPayload struct {
	ActionType    string         `json:"action_type"`
	ActionSummary string         `json:"action_summary"`
	ProposedArgs  map[string]any `json:"proposed_args,omitempty"`
	RiskLevel     string         `json:"risk_level"`
	TimeoutAt     *time.Time     `json:"timeout_at,omitempty"`
}

type ApprovalRespondedPayload struct {
	Decision string  `json:"decision"`
	Reason   *string `json:"reason,omitempty"`
}

type ToolCallCompletedPayload struct {
	ToolName   string         `json:"tool_name"`
	Success    bool           `json:"success"`
	Result     map[string]any `json:"result,omitempty"`
	Error      *string        `json:"error,omitempty"`
	DurationMS int64          `json:"duration_ms"`
}

type VideoLinkDetectedPayload struct {
	URL string `json:"url"`
}

type FileChangedPayload struct {
	Path string `json:"path"`
}

type TodoScanCompletedPayload struct {
	PendingCount   int `json:"pending_count"`
	CompletedCount int `json:"completed_count"`
}

func NewEvent(eventType, source, sessionID string, priority EventPriority, payload any) Event {
	traceID := id.New("trace")
	return Event{
		ID:        id.New("evt"),
		Type:      eventType,
		Version:   "v1",
		SessionID: sessionID,
		Source:    source,
		TraceID:   traceID,
		SpanID:    id.New("span"),
		Priority:  priority,
		Timestamp: time.Now(),
		Payload:   payload,
		Headers:   map[string]string{},
	}
}

func DeriveEvent(parent Event, eventType, source string, priority EventPriority, payload any) Event {
	spanParent := parent.SpanID
	return Event{
		ID:           id.New("evt"),
		Type:         eventType,
		Version:      "v1",
		SessionID:    parent.SessionID,
		TaskID:       parent.TaskID,
		TaskStepID:   parent.TaskStepID,
		LoopID:       parent.LoopID,
		ApprovalID:   parent.ApprovalID,
		Source:       source,
		TraceID:      parent.TraceID,
		SpanID:       id.New("span"),
		ParentSpanID: &spanParent,
		Priority:     priority,
		Timestamp:    time.Now(),
		Payload:      payload,
		Headers:      map[string]string{},
	}
}
