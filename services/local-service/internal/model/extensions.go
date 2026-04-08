package model

import "context"

type GenerateTextStreamRequest struct {
	TaskID string
	RunID  string
	Input  string
}

type StreamEventType string

const (
	StreamEventDelta StreamEventType = "delta"
	StreamEventDone  StreamEventType = "done"
	StreamEventError StreamEventType = "error"
)

type GenerateTextStreamEvent struct {
	Type      StreamEventType
	DeltaText string
	Error     string
}

type StreamClient interface {
	GenerateTextStream(ctx context.Context, request GenerateTextStreamRequest) (<-chan GenerateTextStreamEvent, error)
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type ToolCallRequest struct {
	TaskID string
	RunID  string
	Input  string
	Tools  []ToolDefinition
}

type ToolCallResult struct {
	RequestID  string
	Provider   string
	ModelID    string
	OutputText string
	ToolCalls  []ToolInvocation
	Usage      TokenUsage
	LatencyMS  int64
}

type ToolInvocation struct {
	Name      string
	Arguments map[string]any
}

type ToolCallingClient interface {
	GenerateToolCalls(ctx context.Context, request ToolCallRequest) (ToolCallResult, error)
}
