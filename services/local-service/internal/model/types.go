// 该文件定义 model 模块内部当前使用的最小请求/响应结构。
//
// 注意：这些 Go 结构当前是 `/packages/protocol/types/core.ts` 中对应模型结构的
// 后端镜像，用于在未引入跨语言代码生成前保持字段命名和语义对齐。
package model

import "context"

// GenerateTextRequest 描述最小文本生成请求。
//
// 字段与 protocol 中的 `ModelGenerateTextRequest` 对齐。
type GenerateTextRequest struct {
	TaskID string `json:"task_id"`
	RunID  string `json:"run_id"`
	Input  string `json:"input"`
}

// TokenUsage 描述最小 token 使用结构。
//
// 字段与 protocol 中的 `ModelTokenUsage` 对齐。
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// InvocationRecord 描述最小模型调用记录。
//
// 字段与 protocol 中的 `ModelInvocationRecord` 对齐。
type InvocationRecord struct {
	TaskID    string     `json:"task_id"`
	RunID     string     `json:"run_id"`
	RequestID string     `json:"request_id"`
	Provider  string     `json:"provider"`
	ModelID   string     `json:"model_id"`
	Usage     TokenUsage `json:"usage"`
	LatencyMS int64      `json:"latency_ms"`
}

// Map 将最小调用记录转换为便于上层消费的结构化 map。
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

// GenerateTextResponse 描述最小文本生成返回。
//
// 字段与 protocol 中的 `ModelGenerateTextResponse` 对齐。
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

// InvocationRecord 将 GenerateTextResponse 转换为最小调用记录。
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

// Client 定义 model provider 的最小接口约束。
type Client interface {
	GenerateText(ctx context.Context, request GenerateTextRequest) (GenerateTextResponse, error)
}
