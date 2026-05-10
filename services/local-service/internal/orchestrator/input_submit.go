package orchestrator

import (
	"context"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

// SubmitInput adapts free-form user input into the task-centric execution path.
// It captures context, derives an intent suggestion, then either waits for more
// input, asks for confirmation, or creates the task/run pair for execution.
func (s *Service) SubmitInput(request SubmitInputRequest) (TaskEntryResponse, error) {
	return s.SubmitInputFromParams(request.ProtocolParamsMap())
}

// SubmitInputFromParams lets the RPC layer reuse the normalized protocol map it
// already validated so submit-input requests avoid an extra DTO-to-map bounce.
func (s *Service) SubmitInputFromParams(params map[string]any) (TaskEntryResponse, error) {
	response, err := s.submitInput(params)
	if err != nil {
		return TaskEntryResponse{}, err
	}
	return newTaskEntryResponse(response)
}

func (s *Service) submitInput(params map[string]any) (map[string]any, error) {
	flow := s.prepareInputSubmitFlow(params)
	if response, handled, err := s.maybeContinueInputSubmit(&flow); err != nil || handled {
		return response, err
	}
	if response, handled, err := s.maybeHandleSuggestedInputScreen(flow); err != nil || handled {
		return response, err
	}
	if response, handled := s.maybeRouteUnanchoredInput(&flow); handled {
		return response, nil
	}
	flow.PreferredDelivery, flow.FallbackDelivery = inputSubmitDeliveryPreference(flow)
	if response, handled, err := s.maybeCreateWaitingInputTask(flow); err != nil || handled {
		return response, err
	}

	task := s.createTaskFromEntryFlow(flow)
	return s.finishInputSubmit(flow, task)
}

func (s *Service) prepareInputSubmitFlow(params map[string]any) taskEntryFlow {
	snapshot := s.context.Capture(params)
	options := mapValue(params, "options")
	confirmRequired := boolValue(options, "confirm_required", false)
	suggestion := s.intent.Suggest(snapshot, nil, confirmRequired)
	return taskEntryFlow{
		Params:               params,
		Snapshot:             snapshot,
		Options:              options,
		ConfirmRequired:      confirmRequired,
		ForceConfirmRequired: confirmRequired,
		Suggestion:           s.normalizeSuggestedIntentForAvailability(snapshot, suggestion, confirmRequired),
	}
}

func (s *Service) maybeContinueInputSubmit(flow *taskEntryFlow) (map[string]any, bool, error) {
	response, handled, resolvedSessionID, err := s.maybeContinueExistingTask(flow.Params, flow.Snapshot, nil, taskContinuationOptions{
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

func (s *Service) maybeHandleSuggestedInputScreen(flow taskEntryFlow) (map[string]any, bool, error) {
	return s.handleScreenAnalyzeSuggestion(flow.Params, flow.Snapshot, flow.Suggestion)
}

func (s *Service) maybeRouteUnanchoredInput(flow *taskEntryFlow) (map[string]any, bool) {
	decision, ok := s.routeUnanchoredSubmitInput(context.Background(), flow.Snapshot, flow.Suggestion, flow.ConfirmRequired)
	if !ok {
		return nil, false
	}
	if decision.Route == inputRouteSocialChat {
		return s.socialChatInputResponse(decision), true
	}
	flow.Suggestion = applyInputRouteDecision(flow.Suggestion, decision)
	return nil, false
}

func inputSubmitDeliveryPreference(flow taskEntryFlow) (string, string) {
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromSubmit(flow.Params)
	if !flow.Suggestion.RequiresConfirm {
		return mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, flow.Suggestion.DirectDeliveryType)
	}
	return preferredDelivery, fallbackDelivery
}

func (s *Service) maybeCreateWaitingInputTask(flow taskEntryFlow) (map[string]any, bool, error) {
	if s.intent.AnalyzeSnapshot(flow.Snapshot) != "waiting_input" {
		return nil, false, nil
	}
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(flow.Params, "session_id", ""),
		RequestSource:     stringValue(flow.Params, "source", ""),
		RequestTrigger:    stringValue(flow.Params, "trigger", ""),
		Title:             presentation.Text(presentation.MessageTaskTitleWaitingInput, nil),
		SourceType:        flow.Suggestion.TaskSourceType,
		Status:            "waiting_input",
		Intent:            nil,
		PreferredDelivery: flow.PreferredDelivery,
		FallbackDelivery:  flow.FallbackDelivery,
		CurrentStep:       "collect_input",
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline("waiting_input", "collect_input"),
		Snapshot:          flow.Snapshot,
	})

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleInputNeedGoal, nil), task.StartedAt.Format(dateTimeLayout))
	task = s.persistTaskPresentation(task, bubble)
	return buildTaskEntryResponse(task, bubble, nil), true, nil
}

func (s *Service) finishInputSubmit(flow taskEntryFlow, task runengine.TaskRecord) (map[string]any, error) {
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(flow.Suggestion.RequiresConfirm), bubbleTextForInput(flow.Suggestion), task.StartedAt.Format(dateTimeLayout))
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

// deliveryPreferenceFromSubmit reads delivery preferences from
// agent.input.submit. Submit uses options.* while agent.task.start uses a
// dedicated delivery object, so the orchestrator keeps both decoders separate
// and normalizes them before any execution or approval plan is built.
func deliveryPreferenceFromSubmit(params map[string]any) (string, string) {
	options := mapValue(params, "options")
	return stringValue(options, "preferred_delivery", ""), ""
}
