package orchestrator

import (
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

// ConfirmTask applies a user decision to a task that is still waiting for
// intent confirmation. It may keep clarification open, apply a corrected intent,
// or confirm the stored intent before governance and delivery planning continue.
func (s *Service) ConfirmTask(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	if task.Status != "confirming_intent" {
		return nil, ErrTaskStatusInvalid
	}
	confirmed := boolValue(params, "confirmed", false)
	correctedIntent := mapValue(params, "corrected_intent")
	correctionText := strings.TrimSpace(stringValue(params, "correction_text", ""))
	if err := validateTaskConfirmCorrectionPayload(confirmed, correctedIntent, correctionText); err != nil {
		return nil, err
	}
	if !confirmed && correctionText != "" {
		return s.reinferTaskIntentFromCorrection(task, correctionText)
	}
	snapshot := snapshotFromTask(task)
	intentValue := cloneMap(task.Intent)
	updatedTitle := task.Title
	if !confirmed && len(correctedIntent) > 0 {
		if normalizedIntent, ok := normalizeCorrectionIntent(correctedIntent); ok {
			suggestion := s.normalizedTaskConfirmSuggestion(snapshot, normalizedIntent, false)
			intentValue = suggestion.Intent
			updatedTitle = suggestion.TaskTitle
		} else {
			updatedTask, err := s.revertTaskToIntentConfirmation(task)
			if err != nil {
				return nil, err
			}
			bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleConfirmRejected, nil), updatedTask.UpdatedAt.Format(dateTimeLayout))
			if presentedTask, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
				updatedTask = presentedTask
			} else {
				return nil, ErrTaskNotFound
			}
			return map[string]any{
				"task":            taskMap(updatedTask),
				"bubble_message":  bubble,
				"delivery_result": nil,
			}, nil
		}
	}
	if !confirmed && len(correctedIntent) == 0 {
		updatedTask, err := s.revertTaskToIntentConfirmation(task)
		if err != nil {
			return nil, err
		}
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleConfirmRejected, nil), updatedTask.UpdatedAt.Format(dateTimeLayout))
		if presentedTask, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			updatedTask = presentedTask
		} else {
			return nil, ErrTaskNotFound
		}
		return map[string]any{
			"task":            taskMap(updatedTask),
			"bubble_message":  bubble,
			"delivery_result": nil,
		}, nil
	}
	if strings.TrimSpace(stringValue(intentValue, "name", "")) == "" {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleConfirmMissingIntent, nil), task.UpdatedAt.Format(dateTimeLayout))
		if updatedTask, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			return map[string]any{
				"task":            taskMap(updatedTask),
				"bubble_message":  bubble,
				"delivery_result": nil,
			}, nil
		}
		return nil, ErrTaskNotFound
	}
	if confirmed {
		updatedTitle = s.intent.Suggest(snapshot, intentValue, false).TaskTitle
	}

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleConfirmStarted, nil), task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.UpdateIntent(task.TaskID, updatedTitle, intentValue)
	if !ok {
		return nil, ErrTaskNotFound
	}
	s.attachMemoryReadPlans(updatedTask.TaskID, updatedTask.RunID, snapshotFromTask(updatedTask), intentValue)
	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(updatedTask); queueErr != nil {
		return nil, queueErr
	} else if queued {
		return map[string]any{
			"task":            taskMap(queuedTask),
			"bubble_message":  queueBubble,
			"delivery_result": nil,
		}, nil
	}
	governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(updatedTask, intentValue)
	if governanceErr != nil {
		return nil, governanceErr
	}
	if handled {
		return governedResponse, nil
	}
	updatedTask = governedTask

	updatedTask, ok = s.runEngine.ConfirmTask(task.TaskID, updatedTitle, intentValue, bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}
	executionSnapshot := snapshotFromTask(updatedTask)

	updatedTask, resultBubble, deliveryResult, _, err := s.executeTask(updatedTask, executionSnapshot, intentValue)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  resultBubble,
		"delivery_result": optionalFormalDeliveryResult(deliveryResult),
	}, nil
}

func (s *Service) revertTaskToIntentConfirmation(task runengine.TaskRecord) (runengine.TaskRecord, error) {
	updatedTask, ok := s.runEngine.UpdateIntent(task.TaskID, confirmationTitleFromTask(task), nil)
	if !ok {
		return runengine.TaskRecord{}, ErrTaskNotFound
	}
	return updatedTask, nil
}
