package orchestrator

import (
	"context"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

// SubmitInput adapts free-form user input into the task-centric execution path.
// It captures context, derives an intent suggestion, then either waits for more
// input, asks for confirmation, or creates the task/run pair for execution.
func (s *Service) SubmitInput(params map[string]any) (map[string]any, error) {
	snapshot := s.context.Capture(params)
	options := mapValue(params, "options")
	confirmRequired := boolValue(options, "confirm_required", false)
	if response, handled, resolvedSessionID, err := s.maybeContinueExistingTask(params, snapshot, nil, taskContinuationOptions{
		ConfirmRequired:      confirmRequired,
		ForceConfirmRequired: confirmRequired,
	}); err != nil {
		return nil, err
	} else if handled {
		return response, nil
	} else if strings.TrimSpace(resolvedSessionID) != "" {
		params = withResolvedSessionID(params, resolvedSessionID)
	}
	suggestion := s.intent.Suggest(snapshot, nil, confirmRequired)
	suggestion = s.normalizeSuggestedIntentForAvailability(snapshot, suggestion, confirmRequired)
	if handledResponse, handled, err := s.handleScreenAnalyzeSuggestion(params, snapshot, suggestion); err != nil {
		return nil, err
	} else if handled {
		return handledResponse, nil
	}
	if decision, ok := s.routeUnanchoredSubmitInput(context.Background(), snapshot, suggestion, confirmRequired); ok {
		if decision.Route == inputRouteSocialChat {
			return s.socialChatInputResponse(decision), nil
		}
		suggestion = applyInputRouteDecision(suggestion, decision)
	}
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromSubmit(params)
	if !suggestion.RequiresConfirm {
		preferredDelivery, fallbackDelivery = mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, suggestion.DirectDeliveryType)
	}
	if s.intent.AnalyzeSnapshot(snapshot) == "waiting_input" {
		task := s.runEngine.CreateTask(runengine.CreateTaskInput{
			SessionID:         stringValue(params, "session_id", ""),
			RequestSource:     stringValue(params, "source", ""),
			RequestTrigger:    stringValue(params, "trigger", ""),
			Title:             "等待补充输入",
			SourceType:        suggestion.TaskSourceType,
			Status:            "waiting_input",
			Intent:            nil,
			PreferredDelivery: preferredDelivery,
			FallbackDelivery:  fallbackDelivery,
			CurrentStep:       "collect_input",
			RiskLevel:         s.risk.DefaultLevel(),
			Timeline:          initialTimeline("waiting_input", "collect_input"),
			Snapshot:          snapshot,
		})

		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "请先告诉我你希望我处理什么内容。", task.StartedAt.Format(dateTimeLayout))
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}

		return map[string]any{
			"task":            taskMap(task),
			"bubble_message":  bubble,
			"delivery_result": nil,
		}, nil
	}

	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(params, "session_id", ""),
		RequestSource:     stringValue(params, "source", ""),
		RequestTrigger:    stringValue(params, "trigger", ""),
		Title:             suggestion.TaskTitle,
		SourceType:        suggestion.TaskSourceType,
		Status:            taskStatusForSuggestion(suggestion.RequiresConfirm),
		Intent:            suggestion.Intent,
		PreferredDelivery: preferredDelivery,
		FallbackDelivery:  fallbackDelivery,
		CurrentStep:       currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent),
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline(taskStatusForSuggestion(suggestion.RequiresConfirm), currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent)),
		Snapshot:          snapshot,
	})
	s.publishTaskStart(task.TaskID, task.SessionID, requestTraceID(params))
	s.attachMemoryReadPlans(task.TaskID, task.RunID, snapshot, suggestion.Intent)

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForInput(suggestion), task.StartedAt.Format(dateTimeLayout))
	deliveryResult := map[string]any(nil)
	if !suggestion.RequiresConfirm {
		if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
			return nil, queueErr
		} else if queued {
			task = queuedTask
			bubble = queueBubble
		} else {
			governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(task, suggestion.Intent)
			if governanceErr != nil {
				return nil, governanceErr
			}
			if handled {
				return governedResponse, nil
			}
			task = governedTask
			var execErr error
			task, bubble, deliveryResult, _, execErr = s.executeTask(task, snapshot, suggestion.Intent)
			if execErr != nil {
				return nil, execErr
			}
		}
	} else {
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}
	}

	response := map[string]any{
		"task":            taskMap(task),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}
	if deliveryResult != nil {
		response["delivery_result"] = deliveryResult
	}

	return response, nil
}

// deliveryPreferenceFromSubmit reads delivery preferences from
// agent.input.submit. Submit uses options.* while agent.task.start uses a
// dedicated delivery object, so the orchestrator keeps both decoders separate
// and normalizes them before any execution or approval plan is built.
func deliveryPreferenceFromSubmit(params map[string]any) (string, string) {
	options := mapValue(params, "options")
	return stringValue(options, "preferred_delivery", ""), ""
}
