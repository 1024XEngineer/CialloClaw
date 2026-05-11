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
	intentValue := cloneMap(task.Intent)
	if !confirmed && len(correctedIntent) > 0 {
		intentValue = correctedIntent
	}
	if !confirmed && len(correctedIntent) == 0 {
		updatedTask, err := s.revertTaskToIntentConfirmation(task)
		if err != nil {
			return nil, err
		}
		snapshot := snapshotFromTask(updatedTask)
		clarificationText := rejectedIntentClarificationText(snapshot)
		if clarificationHits := s.previewMemoryContext(updatedTask.TaskID, updatedTask.RunID, snapshot); len(clarificationHits) > 0 {
			clarificationText = clarificationText + " " + clarificationBubbleText(map[string]any{}, snapshot, clarificationHits)
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
		clarificationText := missingIntentClarificationText(snapshot)
		if clarificationHits := s.previewMemoryContext(task.TaskID, task.RunID, snapshot); len(clarificationHits) > 0 {
			clarificationText = clarificationText + " " + clarificationBubbleText(map[string]any{}, snapshot, clarificationHits)
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
	updatedTitle := s.intent.Suggest(snapshotFromTask(task), intentValue, false).TaskTitle

	bubbleText := confirmationAcceptedText(snapshotFromTask(task))
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
	snapshot := snapshotFromTask(updatedTask)

	updatedTask, resultBubble, deliveryResult, _, err := s.executeTask(updatedTask, snapshot, intentValue)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  resultBubble,
		"delivery_result": optionalFormalDeliveryResult(deliveryResult),
	}, nil
}

func confirmationAcceptedText(snapshot taskcontext.TaskContextSnapshot) string {
	if isEnglishClarificationSnapshot(snapshot) {
		return "Got it. I am starting with your updated goal."
	}
	return "已按新的要求开始处理"
}

func isEnglishClarificationSnapshot(snapshot taskcontext.TaskContextSnapshot) bool {
	return clarificationReplyLanguage(snapshot) == languagepolicy.ReplyLanguageEnglish
}

func rejectedIntentClarificationText(snapshot taskcontext.TaskContextSnapshot) string {
	if isEnglishClarificationSnapshot(snapshot) {
		return "That is not the right handling path. Please restate your goal or give me a more accurate intent."
	}
	return "这不是我该做的处理方式。请重新说明你的目标，或给我一个更准确的处理意图。"
}

func missingIntentClarificationText(snapshot taskcontext.TaskContextSnapshot) string {
	if isEnglishClarificationSnapshot(snapshot) {
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
