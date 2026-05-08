package orchestrator

import (
	"fmt"
	"strings"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

// StartTask creates the formal task/run mapping from an explicit object or an
// inferred intent. Object-only starts stay in confirmation unless the caller
// supplied enough instruction to enter governance and execution immediately.
func (s *Service) StartTask(params map[string]any) (map[string]any, error) {
	snapshot := s.context.Capture(params)
	explicitIntent := mapValue(params, "intent")
	options := mapValue(params, "options")
	forceConfirmRequired := boolValue(options, "confirm_required", false)
	confirmRequired := taskStartConfirmRequired(snapshot, explicitIntent, forceConfirmRequired)
	if response, handled, resolvedSessionID, err := s.maybeContinueExistingTask(params, snapshot, explicitIntent, taskContinuationOptions{
		ConfirmRequired:      confirmRequired,
		ForceConfirmRequired: forceConfirmRequired,
	}); err != nil {
		return nil, err
	} else if handled {
		return response, nil
	} else if strings.TrimSpace(resolvedSessionID) != "" {
		params = withResolvedSessionID(params, resolvedSessionID)
	}
	if handledResponse, handled, err := s.handleScreenAnalyzeStart(params, snapshot, explicitIntent); err != nil {
		return nil, err
	} else if handled {
		return handledResponse, nil
	}
	suggestion := s.intent.Suggest(snapshot, explicitIntent, confirmRequired)
	fallbackConfirmRequired := confirmRequired
	// Screen inference already carries its own authorization boundary; only an
	// explicit caller request should turn an unavailable screen path back into
	// intent confirmation.
	if stringValue(suggestion.Intent, "name", "") == "screen_analyze" && !forceConfirmRequired {
		fallbackConfirmRequired = suggestion.RequiresConfirm
	}
	suggestion = s.normalizeSuggestedIntentForAvailability(snapshot, suggestion, fallbackConfirmRequired)
	if handledResponse, handled, err := s.handleScreenAnalyzeSuggestion(params, snapshot, suggestion); err != nil {
		return nil, err
	} else if handled {
		return handledResponse, nil
	}
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromStart(params)
	if len(explicitIntent) == 0 && !suggestion.RequiresConfirm {
		preferredDelivery, fallbackDelivery = mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, suggestion.DirectDeliveryType)
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

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForStart(suggestion), task.StartedAt.Format(dateTimeLayout))
	response := map[string]any{
		"task":            taskMap(task),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}

	if suggestion.RequiresConfirm {
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
			response["task"] = taskMap(task)
		}
		return response, nil
	}

	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
		return nil, queueErr
	} else if queued {
		response["task"] = taskMap(queuedTask)
		response["bubble_message"] = queueBubble
		return response, nil
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
	response["task"] = taskMap(task)
	response["bubble_message"] = bubble
	if len(deliveryResult) > 0 {
		response["delivery_result"] = deliveryResult
	} else {
		response["delivery_result"] = nil
	}
	return response, nil
}

// taskStartConfirmRequired keeps confirmation as an explicit pre-execution gate.
// Object-based task starts with their own instruction can enter the Agent Loop
// directly, while bare objects still stop for intent confirmation.
func taskStartConfirmRequired(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, forceConfirm bool) bool {
	if forceConfirm {
		return true
	}
	if len(explicitIntent) > 0 {
		return false
	}
	return !taskStartHasExplicitGoal(snapshot)
}

func taskStartHasExplicitGoal(snapshot contextsvc.TaskContextSnapshot) bool {
	switch snapshot.InputType {
	case "file":
		return strings.TrimSpace(snapshot.Text) != ""
	default:
		return false
	}
}

// taskStatusForSuggestion derives the initial task_status from the suggestion
// confirmation requirement.
func taskStatusForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "confirming_intent"
	}
	return "processing"
}

// currentStepForSuggestion derives the initial current_step from the suggested
// intent.
func currentStepForSuggestion(requiresConfirm bool, taskIntent map[string]any) string {
	if requiresConfirm {
		return "intent_confirmation"
	}
	if stringValue(taskIntent, "name", "") == "agent_loop" {
		return "agent_loop"
	}
	return "generate_output"
}

// bubbleTypeForSuggestion selects the outward-facing bubble type for the
// suggestion result.
func bubbleTypeForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "intent_confirm"
	}
	return "result"
}

// bubbleTextForInput returns the bubble text for agent.input.submit flows.
func bubbleTextForInput(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return "我还不确定你想如何处理这段内容，请确认目标。"
		}
		return confirmIntentText(suggestion.Intent)
	}
	return suggestion.ResultBubbleText
}

// bubbleTextForStart returns the bubble text for agent.task.start flows.
func bubbleTextForStart(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return "我还不确定你想如何处理当前对象，请先确认。"
		}
		return confirmIntentText(suggestion.Intent)
	}
	return suggestion.ResultBubbleText
}

func confirmIntentText(taskIntent map[string]any) string {
	switch stringValue(taskIntent, "name", "") {
	case "translate":
		return "你是想翻译这段内容吗？"
	case "rewrite":
		return "你是想改写这段内容吗？"
	case "explain":
		return "你是想解释这段内容吗？"
	case "summarize":
		return "你是想总结这段内容吗？"
	case "write_file":
		return "你是想把结果整理成文档吗？"
	default:
		return "请确认你希望我如何处理当前内容。"
	}
}

// initialTimeline creates the first timeline step for a new task and derives
// whether that step starts as pending or running.
func initialTimeline(status, currentStep string) []runengine.TaskStepRecord {
	stepStatus := "running"
	if status == "confirming_intent" || status == "waiting_input" {
		stepStatus = "pending"
	}

	outputSummary := "等待继续处理"
	if status == "waiting_input" {
		outputSummary = "等待用户补充输入"
	}

	return []runengine.TaskStepRecord{
		{
			StepID:        fmt.Sprintf("step_%s", currentStep),
			Name:          currentStep,
			Status:        stepStatus,
			OrderIndex:    1,
			InputSummary:  "已识别到当前任务对象",
			OutputSummary: outputSummary,
		},
	}
}

func deliveryPreferenceFromStart(params map[string]any) (string, string) {
	deliveryOptions := mapValue(params, "delivery")
	return stringValue(deliveryOptions, "preferred", ""), stringValue(deliveryOptions, "fallback", "")
}
