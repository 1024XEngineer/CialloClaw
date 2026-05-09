// Package model defines provider extension contracts for streaming and tool calls.
package model

import "context"

// GenerateTextStreamRequest carries one streaming text-generation request.
// TaskID and RunID keep provider output attributable to one task/run pair.
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

// GenerateTextStreamEvent is one provider stream event. DeltaText is only set
// on delta events; Error is only set on terminal error events.
type GenerateTextStreamEvent struct {
	Type      StreamEventType
	DeltaText string
	Error     string
}

// StreamClient is the optional provider boundary for streaming text generation.
type StreamClient interface {
	GenerateTextStream(ctx context.Context, request GenerateTextStreamRequest) (<-chan GenerateTextStreamEvent, error)
}

// ToolDefinition describes one model-visible tool contract.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolCallRequest asks a provider to choose tools and/or produce text for a run.
type ToolCallRequest struct {
	TaskID string
	RunID  string
	Input  string
	Tools  []ToolDefinition
}

// ToolCallResult is the normalized provider output after tool-call planning.
type ToolCallResult struct {
	RequestID  string
	Provider   string
	ModelID    string
	OutputText string
	ToolCalls  []ToolInvocation
	Usage      TokenUsage
	LatencyMS  int64
}

// ToolInvocation captures one provider-selected tool name and argument payload.
type ToolInvocation struct {
	Name      string
	Arguments map[string]any
}

// ToolCallingClient is implemented by providers that return structured tool calls.
type ToolCallingClient interface {
	GenerateToolCalls(ctx context.Context, request ToolCallRequest) (ToolCallResult, error)
}
