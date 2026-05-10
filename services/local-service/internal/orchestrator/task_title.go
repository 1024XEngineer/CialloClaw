package orchestrator

import (
	"context"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
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
func (s *Service) scheduleTaskTitleRefresh(taskID string, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, fallbackTitle string) {
	if s == nil || s.titleGenerator == nil || s.runEngine == nil {
		return
	}
	taskID = strings.TrimSpace(taskID)
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
		generatedTitle := s.titleGenerator.GenerateTaskSubject(context.Background(), snapshot, intentName, fallbackTitle)
		if generatedTitle == "" || generatedTitle == fallbackTitle {
			return
		}
		updatedTask, ok := s.runEngine.UpdateTitleIfCurrent(taskID, fallbackTitle, refreshToken, generatedTitle)
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
	s.scheduleTaskTitleRefresh(task.TaskID, snapshot, taskIntent, task.Title)
}

func nonEmptySessionID(sessionID string) any {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	return sessionID
}
