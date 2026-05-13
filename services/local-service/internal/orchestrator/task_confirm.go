package orchestrator

import (
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/languagepolicy"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
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
	snapshot := snapshotFromTask(task)
	replyLanguage := s.taskConfirmReplyLanguage(snapshot, correctionText)
	intentValue := cloneMap(task.Intent)
	updatedTitle := task.Title
	if !confirmed && correctionText != "" {
		suggestion := s.reinferTaskIntentFromCorrection(task, snapshot, correctionText)
		intentValue = suggestion.Intent
		updatedTitle = suggestion.TaskTitle
	} else if !confirmed && len(correctedIntent) > 0 {
		if normalizedIntent, ok := normalizeTaskConfirmIntent(correctedIntent); ok {
			suggestion := s.normalizedTaskConfirmSuggestion(snapshot, normalizedIntent, false)
			intentValue = suggestion.Intent
			updatedTitle = suggestion.TaskTitle
		} else {
			updatedTask, err := s.revertTaskToIntentConfirmation(task)
			if err != nil {
				return nil, err
			}
			snapshot := snapshotFromTask(updatedTask)
			replyLanguage := s.taskConfirmReplyLanguage(snapshot, correctionText)
			clarificationText := rejectedIntentClarificationText(replyLanguage)
			if clarificationHits := s.clarificationPreviewHits(updatedTask, snapshot); len(clarificationHits) > 0 {
				clarificationText = clarificationText + " " + clarificationBubbleTextForLanguage(map[string]any{}, clarificationHits, replyLanguage)
			}
			bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", clarificationText, updatedTask.UpdatedAt.Format(dateTimeLayout))
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
	} else if !confirmed && len(correctedIntent) == 0 {
		updatedTask, err := s.revertTaskToIntentConfirmation(task)
		if err != nil {
			return nil, err
		}
		snapshot := snapshotFromTask(updatedTask)
		replyLanguage := s.taskConfirmReplyLanguage(snapshot, correctionText)
		clarificationText := rejectedIntentClarificationText(replyLanguage)
		if clarificationHits := s.clarificationPreviewHits(updatedTask, snapshot); len(clarificationHits) > 0 {
			clarificationText = clarificationText + " " + clarificationBubbleTextForLanguage(map[string]any{}, clarificationHits, replyLanguage)
		}
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", clarificationText, updatedTask.UpdatedAt.Format(dateTimeLayout))
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
		snapshot := snapshotFromTask(task)
		replyLanguage := s.taskConfirmReplyLanguage(snapshot, correctionText)
		clarificationText := missingIntentClarificationText(replyLanguage)
		if clarificationHits := s.clarificationPreviewHits(task, snapshot); len(clarificationHits) > 0 {
			clarificationText = clarificationText + " " + clarificationBubbleTextForLanguage(map[string]any{}, clarificationHits, replyLanguage)
		}
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", clarificationText, task.UpdatedAt.Format(dateTimeLayout))
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

	bubbleText := confirmationAcceptedText(replyLanguage)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", bubbleText, task.UpdatedAt.Format(dateTimeLayout))
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
	s.refreshTitleAfterGovernance(updatedTask, executionSnapshot, intentValue)

	updatedTask, resultBubble, deliveryResult, _, err := s.executeTaskWithReplyLanguage(updatedTask, executionSnapshot, intentValue, replyLanguage)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  resultBubble,
		"delivery_result": optionalFormalDeliveryResult(deliveryResult),
	}, nil
}

func confirmationAcceptedText(replyLanguage string) string {
	if isEnglishReplyLanguage(replyLanguage) {
		return "Got it. I am starting with your updated goal."
	}
	return "已按新的要求开始处理"
}

func (s *Service) taskConfirmReplyLanguage(snapshot taskcontext.TaskContextSnapshot, correctionText string) string {
	if trimmed := strings.TrimSpace(correctionText); trimmed != "" {
		return languagepolicy.PreferredReplyLanguage(trimmed)
	}
	return s.confirmationReplyLanguage(snapshot)
}

func isEnglishReplyLanguage(replyLanguage string) bool {
	return replyLanguage == languagepolicy.ReplyLanguageEnglish
}

func rejectedIntentClarificationText(replyLanguage string) string {
	if isEnglishReplyLanguage(replyLanguage) {
		return "That is not the right handling path. Please restate your goal or give me a more accurate intent."
	}
	return "这不是我该做的处理方式。请重新说明你的目标，或给我一个更准确的处理意图。"
}

func missingIntentClarificationText(replyLanguage string) string {
	if isEnglishReplyLanguage(replyLanguage) {
		return "Please tell me clearly what kind of handling you want me to perform first."
	}
	return "请先明确告诉我你希望执行的处理方式。"
}

func (s *Service) revertTaskToIntentConfirmation(task runengine.TaskRecord) (runengine.TaskRecord, error) {
	updatedTask, ok := s.runEngine.UpdateIntent(task.TaskID, confirmationTitleFromTask(task), nil)
	if !ok {
		return runengine.TaskRecord{}, ErrTaskNotFound
	}
	return updatedTask, nil
}
