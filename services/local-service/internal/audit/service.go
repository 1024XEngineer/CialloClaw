// Package audit provides the minimal audit-layer implementation.
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

var (
	ErrTaskIDRequired   = errors.New("audit: task_id is required")
	ErrTypeRequired     = errors.New("audit: type is required")
	ErrActionRequired   = errors.New("audit: action is required")
	ErrResultRequired   = errors.New("audit: result is required")
	ErrCandidateInvalid = errors.New("audit: candidate is invalid")
)

type noopWriter struct{}

func (noopWriter) WriteAuditRecord(_ context.Context, _ Record) error {
	return nil
}

// Service exposes the minimal audit-building and persistence helpers.
type Service struct {
	writer Writer
	now    func() time.Time
}

// NewService builds an audit service with an optional injected writer.
func NewService(writers ...Writer) *Service {
	writer := Writer(noopWriter{})
	if len(writers) > 0 && writers[0] != nil {
		writer = writers[0]
	}

	return &Service{
		writer: writer,
		now:    time.Now,
	}
}

// Status reports whether the audit service is ready to accept writes.
func (s *Service) Status() string {
	return "ready"
}

// BuildRecord normalizes one RecordInput into a persisted audit record.
func (s *Service) BuildRecord(input RecordInput) (Record, error) {
	if err := validateRecordInput(input); err != nil {
		return Record{}, err
	}

	return Record{
		AuditID:   s.nextAuditID(),
		TaskID:    strings.TrimSpace(input.TaskID),
		RunID:     strings.TrimSpace(input.RunID),
		Type:      strings.TrimSpace(input.Type),
		Action:    strings.TrimSpace(input.Action),
		Summary:   strings.TrimSpace(input.Summary),
		Target:    strings.TrimSpace(input.Target),
		Result:    strings.TrimSpace(input.Result),
		CreatedAt: s.now().UTC().Format(time.RFC3339Nano),
	}, nil
}

// BuildRecordInputFromCandidate adapts an upstream candidate payload into the
// minimal audit input shape.
func BuildRecordInputFromCandidate(taskID string, candidate map[string]any) (RecordInput, error) {
	if strings.TrimSpace(taskID) == "" {
		return RecordInput{}, ErrTaskIDRequired
	}
	if candidate == nil {
		return RecordInput{}, ErrCandidateInvalid
	}

	typeValue, ok := candidate["type"].(string)
	if !ok || strings.TrimSpace(typeValue) == "" {
		return RecordInput{}, ErrTypeRequired
	}
	actionValue, ok := candidate["action"].(string)
	if !ok || strings.TrimSpace(actionValue) == "" {
		return RecordInput{}, ErrActionRequired
	}
	resultValue, ok := candidate["result"].(string)
	if !ok || strings.TrimSpace(resultValue) == "" {
		return RecordInput{}, ErrResultRequired
	}

	summaryValue, _ := candidate["summary"].(string)
	targetValue, _ := candidate["target"].(string)
	runIDValue, _ := candidate["run_id"].(string)

	return RecordInput{
		TaskID:  strings.TrimSpace(taskID),
		RunID:   strings.TrimSpace(runIDValue),
		Type:    strings.TrimSpace(typeValue),
		Action:  strings.TrimSpace(actionValue),
		Summary: strings.TrimSpace(summaryValue),
		Target:  strings.TrimSpace(targetValue),
		Result:  strings.TrimSpace(resultValue),
	}, nil
}

// Write normalizes and persists one audit record.
func (s *Service) Write(ctx context.Context, input RecordInput) (Record, error) {
	record, err := s.BuildRecord(input)
	if err != nil {
		return Record{}, err
	}
	if err := s.writer.WriteAuditRecord(ctx, record); err != nil {
		return Record{}, fmt.Errorf("audit: write record: %w", err)
	}
	return record, nil
}

// BuildToolAudit builds a normalized audit record from a tool-call
// audit_candidate payload.
func (s *Service) BuildToolAudit(taskID, runID string, toolCall tools.ToolCallRecord) (map[string]any, map[string]any, bool) {
	candidate, _ := toolCall.Output["audit_candidate"].(map[string]any)
	tokenUsage := buildTokenUsage(toolCall.Output)
	if len(candidate) == 0 {
		candidate = fallbackToolAuditCandidate(toolCall, tokenUsage)
	}
	if len(candidate) == 0 && len(tokenUsage) == 0 {
		return nil, nil, false
	}

	metadata := map[string]any{
		"tool_call_id": toolCall.ToolCallID,
		"tool_name":    toolCall.ToolName,
		"duration_ms":  toolCall.DurationMS,
	}
	if provider := stringValue(toolCall.Output, "provider", ""); provider != "" {
		metadata["provider"] = provider
	}
	if modelID := stringValue(toolCall.Output, "model_id", ""); modelID != "" {
		metadata["model_id"] = modelID
	}
	if requestID := stringValue(toolCall.Output, "request_id", ""); requestID != "" {
		metadata["request_id"] = requestID
	}
	if latencyMS := int64Value(toolCall.Output, "latency_ms", 0); latencyMS > 0 {
		metadata["latency_ms"] = latencyMS
	}
	if fallback, ok := toolCall.Output["fallback"].(bool); ok {
		metadata["fallback"] = fallback
	}

	record := s.buildRuntimeRecord(
		taskID,
		runID,
		stringValue(candidate, "type", "tool"),
		firstNonEmptyString(stringValue(candidate, "action", ""), toolCall.ToolName),
		stringValue(candidate, "summary", "tool execution completed"),
		stringValue(candidate, "target", "main_flow"),
		stringValue(candidate, "result", "success"),
		metadata,
	)

	return record, tokenUsage, true
}

// BuildDeliveryAudit records the audit summary for the formal delivery stage.
func (s *Service) BuildDeliveryAudit(taskID, runID string, deliveryResult map[string]any) map[string]any {
	if len(deliveryResult) == 0 {
		return nil
	}

	target := stringValue(deliveryPayload(deliveryResult), "path", "")
	if target == "" {
		target = stringValue(deliveryResult, "type", "delivery_result")
	}

	return s.buildRuntimeRecord(
		taskID,
		runID,
		"delivery",
		"publish_result",
		fmt.Sprintf("delivery result generated: %s", firstNonEmptyString(stringValue(deliveryResult, "title", ""), "result")),
		target,
		"success",
		map[string]any{
			"delivery_type": stringValue(deliveryResult, "type", ""),
			"title":         stringValue(deliveryResult, "title", ""),
		},
	)
}

// BuildAuthorizationAudit records the audit summary for allow/deny decisions.
func (s *Service) BuildAuthorizationAudit(taskID, runID, decision string, impactScope map[string]any) map[string]any {
	decision = firstNonEmptyString(strings.TrimSpace(decision), "allow_once")
	result := "approved"
	summary := "authorization approved"
	if decision == "deny_once" {
		result = "denied"
		summary = "authorization denied"
	}

	target := "authorization_scope"
	switch files := impactScope["files"].(type) {
	case []string:
		if len(files) > 0 {
			target = files[0]
		}
	case []any:
		if len(files) > 0 {
			if firstFile, ok := files[0].(string); ok && strings.TrimSpace(firstFile) != "" {
				target = firstFile
			}
		}
	}

	return s.buildRuntimeRecord(
		taskID,
		runID,
		"authorization",
		decision,
		summary,
		target,
		result,
		map[string]any{
			"impact_scope": cloneMap(impactScope),
		},
	)
}

func validateRecordInput(input RecordInput) error {
	if strings.TrimSpace(input.TaskID) == "" {
		return ErrTaskIDRequired
	}
	if strings.TrimSpace(input.Type) == "" {
		return ErrTypeRequired
	}
	if strings.TrimSpace(input.Action) == "" {
		return ErrActionRequired
	}
	if strings.TrimSpace(input.Result) == "" {
		return ErrResultRequired
	}
	return nil
}

func (s *Service) buildRuntimeRecord(taskID, runID, auditType, action, summary, target, result string, metadata map[string]any) map[string]any {
	record := map[string]any{
		"audit_id":   s.nextAuditID(),
		"task_id":    strings.TrimSpace(taskID),
		"run_id":     strings.TrimSpace(runID),
		"type":       firstNonEmptyString(auditType, "system"),
		"action":     firstNonEmptyString(action, "unknown"),
		"summary":    firstNonEmptyString(summary, "runtime audit record"),
		"target":     firstNonEmptyString(target, "main_flow"),
		"result":     firstNonEmptyString(result, "success"),
		"created_at": s.now().UTC().Format(time.RFC3339Nano),
	}
	if len(metadata) > 0 {
		record["metadata"] = cloneMap(metadata)
	}
	return record
}

func (s *Service) nextAuditID() string {
	seq := atomic.AddUint64(&auditCounter, 1)
	return fmt.Sprintf("audit_%d_%d", s.now().UTC().UnixNano(), seq)
}

var auditCounter uint64

func buildTokenUsage(raw map[string]any) map[string]any {
	tokenUsage, ok := raw["token_usage"].(map[string]any)
	if !ok || len(tokenUsage) == 0 {
		return nil
	}

	result := map[string]any{
		"input_tokens":  intValue(tokenUsage, "input_tokens", 0),
		"output_tokens": intValue(tokenUsage, "output_tokens", 0),
		"total_tokens":  intValue(tokenUsage, "total_tokens", 0),
		"request_id":    stringValue(raw, "request_id", ""),
		"latency_ms":    int64Value(raw, "latency_ms", 0),
		"provider":      stringValue(raw, "provider", ""),
		"model_id":      stringValue(raw, "model_id", ""),
		"estimated_cost": floatValue(tokenUsage, "estimated_cost",
			0.0),
	}
	if fallback, ok := raw["fallback"].(bool); ok {
		result["fallback"] = fallback
	}
	return result
}

func fallbackToolAuditCandidate(toolCall tools.ToolCallRecord, tokenUsage map[string]any) map[string]any {
	if toolCall.ToolName == "" && len(tokenUsage) == 0 {
		return nil
	}

	candidate := map[string]any{
		"type":    "tool",
		"action":  firstNonEmptyString(toolCall.ToolName, "tool"),
		"summary": "tool execution completed",
		"target":  "main_flow",
		"result":  "success",
	}
	if toolCall.Status != tools.ToolCallStatusSucceeded {
		candidate["summary"] = "tool execution failed"
		candidate["result"] = string(toolCall.Status)
	}

	switch toolCall.ToolName {
	case "generate_text":
		candidate["type"] = "model"
		candidate["summary"] = "generate text output"
	case "write_file":
		candidate["type"] = "file"
		candidate["summary"] = "write file output"
		if pathValue := stringValue(toolCall.Input, "path", ""); pathValue != "" {
			candidate["target"] = pathValue
		}
	case "exec_command":
		candidate["type"] = "command"
		candidate["summary"] = "command execution completed"
		if workingDir := stringValue(toolCall.Input, "working_dir", ""); workingDir != "" {
			candidate["target"] = workingDir
		}
		if toolCall.Status != tools.ToolCallStatusSucceeded {
			candidate["summary"] = "command execution failed"
		}
	}

	if len(tokenUsage) > 0 {
		candidate["type"] = "model"
		if requestID := stringValue(tokenUsage, "request_id", ""); requestID != "" {
			candidate["target"] = requestID
		}
	}

	return candidate
}

func deliveryPayload(deliveryResult map[string]any) map[string]any {
	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return payload
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneMap(typed)
		case []map[string]any:
			result[key] = cloneMapSlice(typed)
		case []string:
			result[key] = append([]string(nil), typed...)
		case []any:
			result[key] = append([]any(nil), typed...)
		default:
			result[key] = value
		}
	}
	return result
}

func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, cloneMap(value))
	}
	return result
}

func stringValue(values map[string]any, key, fallback string) string {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	value, ok := rawValue.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func intValue(values map[string]any, key string, fallback int) int {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	switch value := rawValue.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}

func int64Value(values map[string]any, key string, fallback int64) int64 {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	switch value := rawValue.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	default:
		return fallback
	}
}

func floatValue(values map[string]any, key string, fallback float64) float64 {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	switch value := rawValue.(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return fallback
	}
}

func firstNonEmptyString(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return strings.TrimSpace(fallback)
}
