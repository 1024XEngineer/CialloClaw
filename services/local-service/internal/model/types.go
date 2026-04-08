package model

import "context"

type GenerateTextRequest struct {
	TaskID string
	RunID  string
	Input  string
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type InvocationRecord struct {
	TaskID    string
	RunID     string
	RequestID string
	Provider  string
	ModelID   string
	Usage     TokenUsage
	LatencyMS int64
}

type GenerateTextResponse struct {
	TaskID     string
	RunID      string
	RequestID  string
	Provider   string
	ModelID    string
	OutputText string
	Usage      TokenUsage
	LatencyMS  int64
}

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

type Client interface {
	GenerateText(ctx context.Context, request GenerateTextRequest) (GenerateTextResponse, error)
}
