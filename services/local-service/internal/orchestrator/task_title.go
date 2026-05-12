package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/titlegen"
)

type auxiliaryTokenUsageSummary struct {
	InputTokens   int
	OutputTokens  int
	TotalTokens   int
	EstimatedCost float64
}

func (s auxiliaryTokenUsageSummary) Map() map[string]any {
	return map[string]any{
		"input_tokens":   s.InputTokens,
		"output_tokens":  s.OutputTokens,
		"total_tokens":   s.TotalTokens,
		"estimated_cost": s.EstimatedCost,
	}
}

type taskUpdatedNotification struct {
	TaskID    string
	SessionID any
	Status    string
}

func (n taskUpdatedNotification) Map() map[string]any {
	return map[string]any{
		"task_id":    n.TaskID,
		"session_id": n.SessionID,
		"status":     n.Status,
	}
}

type titleGenerationAuditMetadata struct {
	FallbackTitle  string
	GeneratedTitle string
	Provider       string
	ModelID        string
	RequestID      string
	LatencyMS      int64
}

func (m titleGenerationAuditMetadata) Map() map[string]any {
	return map[string]any{
		"fallback_title":  m.FallbackTitle,
		"generated_title": m.GeneratedTitle,
		"provider":        m.Provider,
		"model_id":        m.ModelID,
		"request_id":      m.RequestID,
		"latency_ms":      m.LatencyMS,
	}
}

type titleGenerationAuditRecord struct {
	AuditID   string
	TaskID    string
	RunID     string
	Type      string
	Action    string
	Summary   string
	Target    string
	Result    string
	CreatedAt string
	Metadata  titleGenerationAuditMetadata
}

func (r titleGenerationAuditRecord) Map() map[string]any {
	return map[string]any{
		"audit_id":   r.AuditID,
		"task_id":    r.TaskID,
		"run_id":     r.RunID,
		"type":       r.Type,
		"action":     r.Action,
		"summary":    r.Summary,
		"target":     r.Target,
		"result":     r.Result,
		"created_at": r.CreatedAt,
		"metadata":   r.Metadata.Map(),
	}
}

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
	currentTask, ok := s.runEngine.GetTask(taskID)
	if !ok || !currentTask.UpdatedAt.Equal(task.UpdatedAt) {
		return
	}
	// Each async refresh needs a fresh reservation so an older goroutine cannot
	// reuse a task-carried token and overwrite a newer context snapshot when the
	// responses arrive out of order. The stale-task guard above keeps older task
	// views from reclaiming ownership after a newer state has already published.
	refreshToken, refreshCtx, ok := s.runEngine.ReserveTitleRefresh(taskID, fallbackTitle)
	if !ok {
		return
	}
	intentValue := cloneMap(taskIntent)
	go func() {
		defer s.runEngine.FinishTitleRefresh(taskID, refreshToken)
		intentName := strings.TrimSpace(stringValue(intentValue, "name", ""))
		result := s.titleGenerator.GenerateTaskSubjectResult(refreshCtx, titlegen.GenerationOwner{
			TaskID: task.TaskID,
			RunID:  task.RunID,
		}, snapshot, intentName, fallbackTitle)
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
		s.publishRuntimeNotification(taskID, "task.updated", taskUpdatedNotification{
			TaskID:    updatedTask.TaskID,
			SessionID: nonEmptySessionID(updatedTask.SessionID),
			Status:    updatedTask.Status,
		}.Map())
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
	summary := "generate compact task title"
	outputResult := "success"
	if !result.Generated {
		summary = "task title model call fell back to local task title"
		outputResult = "fallback"
	}
	record := titleGenerationAuditRecord{
		AuditID:   fmt.Sprintf("audit_title_%s_%d", task.TaskID, time.Now().UTC().UnixNano()),
		TaskID:    task.TaskID,
		RunID:     task.RunID,
		Type:      "model",
		Action:    "title.generate",
		Summary:   summary,
		Target:    firstNonEmptyString(intentName, "task_title"),
		Result:    outputResult,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Metadata: titleGenerationAuditMetadata{
			FallbackTitle:  fallbackTitle,
			GeneratedTitle: result.Title,
			Provider:       result.Invocation.Provider,
			ModelID:        result.Invocation.ModelID,
			RequestID:      result.Invocation.RequestID,
			LatencyMS:      result.Invocation.LatencyMS,
		},
	}
	_, _ = s.runEngine.AppendAuditData(task.TaskID, []map[string]any{record.Map()}, titleGenerationTokenUsage(*result.Invocation))
}

func titleGenerationTokenUsage(invocation model.InvocationRecord) map[string]any {
	// Task-level token totals should include auxiliary title generation, but the
	// representative request metadata must keep pointing at the primary execution
	// request instead of whichever refinement goroutine finished last.
	return auxiliaryTokenUsageSummary{
		InputTokens:   invocation.Usage.InputTokens,
		OutputTokens:  invocation.Usage.OutputTokens,
		TotalTokens:   invocation.Usage.TotalTokens,
		EstimatedCost: 0.0,
	}.Map()
}
