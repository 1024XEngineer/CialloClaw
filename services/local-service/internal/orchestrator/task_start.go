package orchestrator

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

// StartTask creates the formal task/run mapping from an explicit object or an
// inferred intent. Object-only starts stay in confirmation unless the caller
// supplied enough instruction to enter governance and execution immediately.
func (s *Service) StartTask(request StartTaskRequest) (TaskEntryResponse, error) {
	response, err := s.startTask(request.paramsMap())
	if err != nil {
		return TaskEntryResponse{}, err
	}
	return newTaskEntryResponse(response), nil
}

func (s *Service) startTask(params map[string]any) (map[string]any, error) {
	flow := s.prepareStartTaskFlow(params)
	if response, handled, err := s.maybeContinueStartTask(&flow); err != nil || handled {
		return response, err
	}
	if response, handled, err := s.maybeHandleExplicitScreenStart(flow); err != nil || handled {
		return response, err
	}

	flow.Suggestion = s.suggestStartTaskIntent(flow)
	if response, handled, err := s.maybeHandleSuggestedScreenStart(flow); err != nil || handled {
		return response, err
	}

	flow.PreferredDelivery, flow.FallbackDelivery = startTaskDeliveryPreference(flow)
	task := s.createTaskFromEntryFlow(flow)
	return s.finishStartTask(flow, task)
}

type taskEntryFlow struct {
	Params               map[string]any
	Snapshot             taskcontext.TaskContextSnapshot
	ExplicitIntent       map[string]any
	Options              map[string]any
	ConfirmRequired      bool
	ForceConfirmRequired bool
	Suggestion           intent.Suggestion
	PreferredDelivery    string
	FallbackDelivery     string
}

func (s *Service) prepareStartTaskFlow(params map[string]any) taskEntryFlow {
	snapshot := s.context.Capture(params)
	explicitIntent := mapValue(params, "intent")
	options := mapValue(params, "options")
	forceConfirmRequired := boolValue(options, "confirm_required", false)

	return taskEntryFlow{
		Params:               params,
		Snapshot:             snapshot,
		ExplicitIntent:       explicitIntent,
		Options:              options,
		ConfirmRequired:      taskStartConfirmRequired(snapshot, explicitIntent, forceConfirmRequired),
		ForceConfirmRequired: forceConfirmRequired,
	}
}

func (s *Service) maybeContinueStartTask(flow *taskEntryFlow) (map[string]any, bool, error) {
	response, handled, resolvedSessionID, err := s.maybeContinueExistingTask(flow.Params, flow.Snapshot, flow.ExplicitIntent, taskContinuationOptions{
		ConfirmRequired:      flow.ConfirmRequired,
		ForceConfirmRequired: flow.ForceConfirmRequired,
	})
	if err != nil || handled {
		return response, handled, err
	}
	if strings.TrimSpace(resolvedSessionID) != "" {
		flow.Params = withResolvedSessionID(flow.Params, resolvedSessionID)
	}
	return nil, false, nil
}

func (s *Service) maybeHandleExplicitScreenStart(flow taskEntryFlow) (map[string]any, bool, error) {
	return s.handleScreenAnalyzeStart(flow.Params, flow.Snapshot, flow.ExplicitIntent)
}

func (s *Service) suggestStartTaskIntent(flow taskEntryFlow) intent.Suggestion {
	suggestion := s.intent.Suggest(flow.Snapshot, flow.ExplicitIntent, flow.ConfirmRequired)
	fallbackConfirmRequired := flow.ConfirmRequired
	// Screen inference already carries its own authorization boundary; only an
	// explicit caller request should turn an unavailable screen path back into
	// intent confirmation.
	if stringValue(suggestion.Intent, "name", "") == "screen_analyze" && !flow.ForceConfirmRequired {
		fallbackConfirmRequired = suggestion.RequiresConfirm
	}
	return s.normalizeSuggestedIntentForAvailability(flow.Snapshot, suggestion, fallbackConfirmRequired)
}

func (s *Service) maybeHandleSuggestedScreenStart(flow taskEntryFlow) (map[string]any, bool, error) {
	return s.handleScreenAnalyzeSuggestion(flow.Params, flow.Snapshot, flow.Suggestion)
}

func startTaskDeliveryPreference(flow taskEntryFlow) (string, string) {
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromStart(flow.Params)
	if len(flow.ExplicitIntent) == 0 && !flow.Suggestion.RequiresConfirm {
		return mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, flow.Suggestion.DirectDeliveryType)
	}
	return preferredDelivery, fallbackDelivery
}

func (s *Service) createTaskFromEntryFlow(flow taskEntryFlow) runengine.TaskRecord {
	status := taskStatusForSuggestion(flow.Suggestion.RequiresConfirm)
	currentStep := currentStepForSuggestion(flow.Suggestion.RequiresConfirm, flow.Suggestion.Intent)
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(flow.Params, "session_id", ""),
		RequestSource:     stringValue(flow.Params, "source", ""),
		RequestTrigger:    stringValue(flow.Params, "trigger", ""),
		Title:             flow.Suggestion.TaskTitle,
		SourceType:        flow.Suggestion.TaskSourceType,
		Status:            status,
		Intent:            flow.Suggestion.Intent,
		PreferredDelivery: flow.PreferredDelivery,
		FallbackDelivery:  flow.FallbackDelivery,
		CurrentStep:       currentStep,
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline(status, currentStep),
		Snapshot:          flow.Snapshot,
	})
	s.publishTaskStart(task.TaskID, task.SessionID, requestTraceID(flow.Params))
	s.attachMemoryReadPlans(task.TaskID, task.RunID, flow.Snapshot, flow.Suggestion.Intent)
	return task
}

func (s *Service) finishStartTask(flow taskEntryFlow, task runengine.TaskRecord) (map[string]any, error) {
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(flow.Suggestion.RequiresConfirm), bubbleTextForStart(flow.Suggestion), task.StartedAt.Format(dateTimeLayout))
	if flow.Suggestion.RequiresConfirm {
		task = s.persistTaskPresentation(task, bubble)
		return buildTaskEntryResponse(task, bubble, nil), nil
	}

	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
		return nil, queueErr
	} else if queued {
		return buildTaskEntryResponse(queuedTask, queueBubble, nil), nil
	}

	governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(task, flow.Suggestion.Intent)
	if governanceErr != nil {
		return nil, governanceErr
	}
	if handled {
		return governedResponse, nil
	}
	task = governedTask

	deliveryResult := map[string]any(nil)
	var execErr error
	task, bubble, deliveryResult, _, execErr = s.executeTask(task, flow.Snapshot, flow.Suggestion.Intent)
	if execErr != nil {
		return nil, execErr
	}
	return buildTaskEntryResponse(task, bubble, deliveryResult), nil
}

// taskStartConfirmRequired keeps confirmation as an explicit pre-execution gate.
// Object-based task starts with their own instruction can enter the Agent Loop
// directly, while bare objects still stop for intent confirmation.
func taskStartConfirmRequired(snapshot taskcontext.TaskContextSnapshot, explicitIntent map[string]any, forceConfirm bool) bool {
	if forceConfirm {
		return true
	}
	if len(explicitIntent) > 0 {
		return false
	}
	return !taskStartHasExplicitGoal(snapshot)
}

func taskStartHasExplicitGoal(snapshot taskcontext.TaskContextSnapshot) bool {
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
			return presentation.Text(presentation.MessageBubbleInputConfirmUnknown, nil)
		}
		return confirmIntentText(suggestion.Intent)
	}
	return suggestion.ResultBubbleText
}

// bubbleTextForStart returns the bubble text for agent.task.start flows.
func bubbleTextForStart(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return presentation.Text(presentation.MessageBubbleStartConfirmUnknown, nil)
		}
		return confirmIntentText(suggestion.Intent)
	}
	return suggestion.ResultBubbleText
}

func confirmIntentText(taskIntent map[string]any) string {
	switch stringValue(taskIntent, "name", "") {
	case "translate":
		return presentation.Text(presentation.MessageBubbleConfirmTranslate, nil)
	case "rewrite":
		return presentation.Text(presentation.MessageBubbleConfirmRewrite, nil)
	case "explain":
		return presentation.Text(presentation.MessageBubbleConfirmExplain, nil)
	case "summarize":
		return presentation.Text(presentation.MessageBubbleConfirmSummarize, nil)
	case "write_file":
		return presentation.Text(presentation.MessageBubbleConfirmWriteFile, nil)
	default:
		return presentation.Text(presentation.MessageBubbleConfirmDefault, nil)
	}
}

// initialTimeline creates the first timeline step for a new task and derives
// whether that step starts as pending or running.
func initialTimeline(status, currentStep string) []runengine.TaskStepRecord {
	stepStatus := "running"
	if status == "confirming_intent" || status == "waiting_input" {
		stepStatus = "pending"
	}

	outputSummary := presentation.Text(presentation.MessageTimelineWaiting, nil)
	if status == "waiting_input" {
		outputSummary = presentation.Text(presentation.MessageTimelineWaitingInput, nil)
	}

	return []runengine.TaskStepRecord{
		{
			StepID:        fmt.Sprintf("step_%s", currentStep),
			Name:          currentStep,
			Status:        stepStatus,
			OrderIndex:    1,
			InputSummary:  presentation.Text(presentation.MessageTimelineInputSeen, nil),
			OutputSummary: outputSummary,
		},
	}
}

func deliveryPreferenceFromStart(params map[string]any) (string, string) {
	deliveryOptions := mapValue(params, "delivery")
	return stringValue(deliveryOptions, "preferred", ""), stringValue(deliveryOptions, "fallback", "")
}
