package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/titlegen"
)

// fallbackTaskTitle keeps the task lifecycle synchronous by using deterministic
// local data on the hot path. Model-backed refinement, when enabled, happens
// after the formal task mutation succeeds.
func (s *Service) fallbackTaskTitle(snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, fallback string) string {
	intentName := strings.TrimSpace(stringValue(taskIntent, "name", ""))
	subjectFallback := strings.TrimSpace(originalTextFromTaskTitle(fallback))
	if subjectFallback == "" {
		subjectFallback = intent.ComposeTaskTitle(snapshot, intentName, "")
	}
	return intent.ComposeTaskTitle(snapshot, intentName, subjectFallback)
}

// scheduleTaskTitleRefresh refines the fallback title without blocking task
// creation, confirmation, or continuation on a model round-trip.
func (s *Service) scheduleTaskTitleRefresh(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, fallbackTitle string) {
	if s == nil || s.titleGenerator == nil || s.runEngine == nil {
		return
	}
	taskID := strings.TrimSpace(task.TaskID)
	fallbackTitle = strings.TrimSpace(fallbackTitle)
	if taskID == "" || fallbackTitle == "" {
		return
	}
	refreshToken, ok := s.runEngine.ReserveTitleRefresh(taskID, fallbackTitle)
	if !ok {
		return
	}
	intentValue := cloneMap(taskIntent)
	go func() {
		intentName := strings.TrimSpace(stringValue(intentValue, "name", ""))
		result := s.titleGenerator.GenerateTaskSubjectResult(context.Background(), snapshot, intentName, fallbackTitle)
		s.appendTaskTitleGenerationAudit(task, intentName, fallbackTitle, result)
		if result.Title == "" || result.Title == fallbackTitle {
			return
		}
		updatedTask, ok := s.runEngine.UpdateTitleIfCurrent(taskID, fallbackTitle, refreshToken, result.Title)
		if !ok {
			return
		}
		// Async title refinement must emit a live task.updated notification because
		// it happens after the original RPC response has usually already drained.
		s.publishRuntimeNotification(taskID, "task.updated", map[string]any{
			"task_id":    updatedTask.TaskID,
			"session_id": nonEmptySessionID(updatedTask.SessionID),
			"status":     updatedTask.Status,
		})
	}()
}

// refreshTitleAfterGovernance starts model-backed title refinement only after a
// task has cleared queueing, confirmation, and governance gates for real
// execution. Title prettification must not export raw task context before those
// owner-controlled boundaries decide the task may proceed.
func (s *Service) refreshTitleAfterGovernance(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any) {
	if s == nil {
		return
	}
	s.scheduleTaskTitleRefresh(task, snapshot, taskIntent, task.Title)
}

func nonEmptySessionID(sessionID string) any {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	return sessionID
}

// appendTaskTitleGenerationAudit keeps post-governance title refresh visible in
// the same task-centric audit/token summary path as other model-backed work.
func (s *Service) appendTaskTitleGenerationAudit(task runengine.TaskRecord, intentName string, fallbackTitle string, result titlegen.TaskSubjectResult) {
	if s == nil || s.runEngine == nil || result.Invocation == nil {
		return
	}
	record := map[string]any{
		"audit_id":   fmt.Sprintf("audit_title_%s_%d", task.TaskID, time.Now().UTC().UnixNano()),
		"task_id":    task.TaskID,
		"run_id":     task.RunID,
		"type":       "model",
		"action":     "title.generate",
		"summary":    "generate compact task title",
		"target":     firstNonEmptyString(intentName, "task_title"),
		"result":     "success",
		"created_at": time.Now().UTC().Format(time.RFC3339Nano),
		"metadata": map[string]any{
			"fallback_title":  fallbackTitle,
			"generated_title": result.Title,
			"provider":        result.Invocation.Provider,
			"model_id":        result.Invocation.ModelID,
			"request_id":      result.Invocation.RequestID,
			"latency_ms":      result.Invocation.LatencyMS,
		},
	}
	_, _ = s.runEngine.AppendAuditData(task.TaskID, []map[string]any{record}, titleGenerationTokenUsage(*result.Invocation))
}

func titleGenerationTokenUsage(invocation model.InvocationRecord) map[string]any {
	return map[string]any{
		"input_tokens":   invocation.Usage.InputTokens,
		"output_tokens":  invocation.Usage.OutputTokens,
		"total_tokens":   invocation.Usage.TotalTokens,
		"estimated_cost": 0.0,
		"request_id":     invocation.RequestID,
		"provider":       invocation.Provider,
		"model_id":       invocation.ModelID,
		"latency_ms":     invocation.LatencyMS,
	}
}
