package orchestrator

import (
	"errors"
	"fmt"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	taskcontext "github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

// NotepadList returns lightweight notepad items from storage without promoting
// them to formal tasks.
func (s *Service) NotepadList(params map[string]any) (map[string]any, error) {
	group := stringValue(params, "group", "upcoming")
	limit := intValue(params, "limit", 20)
	offset := intValue(params, "offset", 0)
	items, total := s.runEngine.NotepadItems(group, limit, offset)
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// NotepadUpdate persists one notepad item update while leaving task creation to
// NotepadConvertToTask.
func (s *Service) NotepadUpdate(params map[string]any) (map[string]any, error) {
	itemID := stringValue(params, "item_id", "")
	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}

	action := stringValue(params, "action", "")
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	updatedItem, refreshGroups, deletedItemID, handled, err := s.runEngine.UpdateNotepadItem(itemID, action)
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, fmt.Errorf("notepad item not found: %s", itemID)
	}

	response := map[string]any{
		"notepad_item":    any(nil),
		"refresh_groups":  refreshGroups,
		"deleted_item_id": nil,
	}
	if updatedItem != nil {
		response["notepad_item"] = s.runEngine.ProtocolNotepadItem(updatedItem)
	}
	if deletedItemID != "" {
		response["deleted_item_id"] = deletedItemID
	}
	return response, nil
}

// NotepadConvertToTask promotes one notepad item into the formal task/run
// workflow and returns the same task payload shape as normal task creation.
func (s *Service) NotepadConvertToTask(params map[string]any) (map[string]any, error) {
	itemID := stringValue(params, "item_id", "")
	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}
	if !boolValue(params, "confirmed", false) {
		return nil, fmt.Errorf("confirmed must be true to convert notepad item")
	}

	item, handled, claimErr := s.runEngine.ClaimNotepadItemTask(itemID)
	if claimErr != nil {
		return nil, claimErr
	}
	if !handled {
		return nil, fmt.Errorf("notepad item not found: %s", itemID)
	}
	claimed := true
	defer func() {
		if claimed {
			s.runEngine.ReleaseNotepadItemClaim(itemID)
		}
	}()

	snapshot := notepadSnapshot(item, currentRuntimeWorkspaceRoot(s.executor))
	suggestion := s.intent.Suggest(snapshot, nil, false)
	suggestion = s.normalizeSuggestedIntentForAvailability(snapshot, suggestion, false)
	suggestion.TaskTitle = notepadTaskTitle(snapshot, suggestion)
	task := s.createNotepadTask(snapshot, suggestion)
	linkedItem, ok := s.runEngine.LinkNotepadItemTask(itemID, task.TaskID)
	if !ok {
		linkErr := fmt.Errorf("failed to link notepad item to task: %s", itemID)
		if rollbackErr := s.runEngine.DeleteTask(task.TaskID); rollbackErr != nil {
			return nil, errors.Join(linkErr, fmt.Errorf("rollback task %s: %w", task.TaskID, rollbackErr))
		}
		return nil, linkErr
	}
	claimed = false
	publishedTaskStart := false
	if !suggestion.RequiresConfirm {
		// Direct-execution note conversions must publish task ownership before
		// queue/governance/execution starts so the shared RPC stream can attach
		// live loop.* notifications to this request the same way task.start does.
		s.publishTaskStart(task.TaskID, task.SessionID, requestTraceID(params))
		publishedTaskStart = true
	}
	response, err := s.finishNotepadTask(snapshot, suggestion, task)
	if err != nil {
		if publishedTaskStart {
			return s.failPublishedNotepadTask(linkedItem, task, suggestion.Intent, err)
		}
		return nil, s.rollbackLinkedNotepadTask(itemID, task.TaskID, err)
	}
	if !publishedTaskStart {
		s.publishTaskStart(task.TaskID, task.SessionID, requestTraceID(params))
	}

	response["notepad_item"] = s.runEngine.ProtocolNotepadItem(linkedItem)
	response["refresh_groups"] = []string{stringValue(linkedItem, "bucket", "upcoming")}
	return response, nil
}

// rollbackLinkedNotepadTask compensates the note->task backlink before deleting
// the provisional task so failed conversions do not leave stale dashboard links.
func (s *Service) rollbackLinkedNotepadTask(itemID, taskID string, cause error) error {
	if _, ok := s.runEngine.UnlinkNotepadItemTask(itemID, taskID); !ok {
		cause = errors.Join(cause, fmt.Errorf("rollback notepad link %s -> %s", itemID, taskID))
	}
	if rollbackErr := s.runEngine.DeleteTask(taskID); rollbackErr != nil {
		cause = errors.Join(cause, fmt.Errorf("rollback task %s: %w", taskID, rollbackErr))
	}
	return cause
}

// failPublishedNotepadTask keeps externally visible note conversions queryable
// when late orchestration setup fails after task.start has already been emitted.
// Once transports can subscribe to the task id, deleting it would leave a
// dangling runtime object on the stream side and a missing task in storage.
// The note stays linked to that failed task so task-centric recovery flows can
// still navigate back to the originating notepad item without guessing state.
func (s *Service) failPublishedNotepadTask(linkedItem map[string]any, task runengine.TaskRecord, taskIntent map[string]any, cause error) (map[string]any, error) {
	impactScope := s.buildImpactScope(task, s.buildPendingExecution(task, taskIntent))
	bubbleText := "任务启动失败，请稍后再试。"
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", bubbleText, task.UpdatedAt.Format(dateTimeLayout))
	failedTask, ok := s.runEngine.FailTaskExecution(task.TaskID, "task_start_failed", "execution_error", bubbleText, impactScope, bubble)
	if !ok {
		return nil, errors.Join(cause, ErrTaskNotFound)
	}
	auditRecord := s.writeGovernanceAuditRecord(failedTask.TaskID, failedTask.RunID, "execution", "start_task", bubbleText, impactScopeTarget(impactScope, targetPathFromIntent(taskIntent)), "failed")
	failedTask = s.appendAuditData(failedTask, compactAuditRecords(auditRecord), nil)

	response := buildTaskEntryResponse(failedTask, bubble, nil)
	response["notepad_item"] = s.runEngine.ProtocolNotepadItem(linkedItem)
	response["refresh_groups"] = []string{stringValue(linkedItem, "bucket", "upcoming")}
	return response, nil
}

func (s *Service) createNotepadTask(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) runengine.TaskRecord {
	status := taskStatusForSuggestion(suggestion.RequiresConfirm)
	currentStep := currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent)
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		RequestSource:     snapshot.Source,
		RequestTrigger:    snapshot.Trigger,
		Title:             suggestion.TaskTitle,
		SourceType:        "todo",
		Status:            status,
		Intent:            suggestion.Intent,
		PreferredDelivery: suggestion.DirectDeliveryType,
		CurrentStep:       currentStep,
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline(status, currentStep),
		Snapshot:          snapshot,
	})
	s.attachMemoryReadPlans(task.TaskID, task.RunID, snapshot, suggestion.Intent)
	return task
}

func (s *Service) finishNotepadTask(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, task runengine.TaskRecord) (map[string]any, error) {
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForStart(snapshot, suggestion, previewClarificationHits(s, task, snapshot, suggestion), snapshot.SessionReplyLanguage), task.StartedAt.Format(dateTimeLayout))
	if suggestion.RequiresConfirm {
		task = s.persistTaskPresentation(task, bubble)
		return buildTaskEntryResponse(task, bubble, nil), nil
	}

	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
		return nil, queueErr
	} else if queued {
		return buildTaskEntryResponse(queuedTask, queueBubble, nil), nil
	}

	governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(task, suggestion.Intent)
	if governanceErr != nil {
		return nil, governanceErr
	}
	if handled {
		return governedResponse, nil
	}
	task = governedTask

	deliveryResult := map[string]any(nil)
	var execErr error
	task, bubble, deliveryResult, _, execErr = s.executeTask(task, snapshot, suggestion.Intent)
	if execErr != nil {
		return nil, execErr
	}
	return buildTaskEntryResponse(task, bubble, deliveryResult), nil
}
