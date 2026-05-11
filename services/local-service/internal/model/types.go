// Package model defines backend model request and response carriers.
//
// These Go structs mirror the corresponding shapes in
// /packages/protocol/types/core.ts until cross-language code generation owns the
// boundary.
package model

import "context"

// GenerateTextRequest is the minimal backend mirror of ModelGenerateTextRequest.
type GenerateTextRequest struct {
	TaskID string `json:"task_id"`
	RunID  string `json:"run_id"`
	Input  string `json:"input"`
}

// TokenUsage mirrors ModelTokenUsage for provider accounting.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// InvocationRecord mirrors ModelInvocationRecord for audit and event payloads.
type InvocationRecord struct {
	TaskID    string     `json:"task_id"`
	RunID     string     `json:"run_id"`
	RequestID string     `json:"request_id"`
	Provider  string     `json:"provider"`
	ModelID   string     `json:"model_id"`
	Usage     TokenUsage `json:"usage"`
	LatencyMS int64      `json:"latency_ms"`
}

// Map returns a protocol-friendly map while preserving serialized field names.
func (r InvocationRecord) Map() map[string]any {
	return map[string]any{
		"task_id":    r.TaskID,
		"run_id":     r.RunID,
		"request_id": r.RequestID,
		"provider":   r.Provider,
		"model_id":   r.ModelID,
		"usage": map[string]any{
			"input_tokens":  r.Usage.InputTokens,
			"output_tokens": r.Usage.OutputTokens,
			"total_tokens":  r.Usage.TotalTokens,
		},
		"latency_ms": r.LatencyMS,
	}
}

// GenerateTextResponse mirrors ModelGenerateTextResponse for backend callers.
type GenerateTextResponse struct {
	TaskID     string     `json:"task_id"`
	RunID      string     `json:"run_id"`
	RequestID  string     `json:"request_id"`
	Provider   string     `json:"provider"`
	ModelID    string     `json:"model_id"`
	OutputText string     `json:"output_text"`
	Usage      TokenUsage `json:"usage"`
	LatencyMS  int64      `json:"latency_ms"`
}

// InvocationRecord converts the response into the minimal model-call record.
func (r GenerateTextResponse) InvocationRecord() InvocationRecord {
	return InvocationRecord{
		TaskID:    r.TaskID,
		RunID:     r.RunID,
		RequestID: r.RequestID,
		Provider:  r.Provider,
		ModelID:   r.ModelID,
		Usage:     r.Usage,
		LatencyMS: r.LatencyMS,
	}
}

// Client is the minimal text-generation provider boundary.
type Client interface {
	GenerateText(ctx context.Context, request GenerateTextRequest) (GenerateTextResponse, error)
}
