package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/languagepolicy"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const defaultIntentName = "agent_loop"

// StopReason describes why one agent loop ended. It is persisted and emitted so
// query and dashboard surfaces can distinguish completion, governance pauses,
// dead loops, and retry exhaustion.
type StopReason string

const (
	StopReasonCompleted          StopReason = "completed"
	StopReasonNeedAuthorization  StopReason = "need_authorization"
	StopReasonNeedUserInput      StopReason = "need_user_input"
	StopReasonMaxIterations      StopReason = "max_iterations_reached"
	StopReasonPlannerError       StopReason = "planner_error"
	StopReasonToolRetryExhausted StopReason = "tool_retry_exhausted"
	StopReasonNoSupportedTools   StopReason = "no_supported_tools"
	StopReasonRepeatedToolChoice StopReason = "dead_loop_detected"
)

// LifecycleEvent captures the task-centric loop events that should flow into
// the run/step/event compatibility chain.
type LifecycleEvent struct {
	Type      string
	Level     string
	StepID    string
	Payload   map[string]any
	CreatedAt time.Time
}

// DeliveryRecord captures one normalized delivery_result snapshot before the
// orchestrator maps it back to the task-centric outward response.
type DeliveryRecord struct {
	DeliveryResultID string
	TaskID           string
	Type             string
	Title            string
	Payload          map[string]any
	PreviewText      string
	CreatedAt        time.Time
}

// Hook allows loop callers to inspect and optionally adjust planning and tool
// execution data without reaching into execution internals.
type Hook interface {
	BeforeRound(ctx context.Context, round PersistedRound, plannerInput string) (string, error)
	AfterRound(ctx context.Context, round PersistedRound) error
	BeforeTool(ctx context.Context, round PersistedRound, call model.ToolInvocation) (model.ToolInvocation, error)
	AfterTool(ctx context.Context, round PersistedRound, record tools.ToolCallRecord, observation string) error
}

// PersistedRound describes one persisted loop step compatible with the
// `steps` table planned in docs/data-design.md.
type PersistedRound struct {
	StepID         string
	RunID          string
	TaskID         string
	AttemptIndex   int
	SegmentKind    string
	LoopRound      int
	Name           string
	Status         string
	InputSummary   string
	OutputSummary  string
	StartedAt      time.Time
	CompletedAt    time.Time
	StopReason     StopReason
	PlannerInput   string
	PlannerOutput  string
	ToolName       string
	Observation    string
	ToolCallRecord tools.ToolCallRecord
}

// Result is the structured output of one full loop run.
type Result struct {
	OutputText       string
	ToolCalls        []tools.ToolCallRecord
	ModelInvocation  map[string]any
	AuditRecord      map[string]any
	Events           []LifecycleEvent
	Rounds           []PersistedRound
	DeliveryRecord   *DeliveryRecord
	StopReason       StopReason
	CompactedHistory []string
}

// Request describes the minimum execution-time dependencies and data that the
// dedicated loop runtime needs.
type Request struct {
	TaskID             string
	RunID              string
	Intent             map[string]any
	AttemptIndex       int
	SegmentKind        string
	InputText          string
	ResultTitle        string
	FallbackOutput     string
	ToolDefinitions    []model.ToolDefinition
	AllowedTool        func(name string) bool
	PollSteering       func(context.Context, string) []string
	GenerateToolCalls  func(context.Context, model.ToolCallRequest) (model.ToolCallResult, error)
	ExecuteTool        func(context.Context, model.ToolInvocation, int) (string, tools.ToolCallRecord)
	BuildAuditRecord   func(context.Context, *model.InvocationRecord) (map[string]any, error)
	MaxTurns           int
	Timeout            time.Duration
	CompressChars      int
	KeepRecent         int
	RepeatedToolBudget int
	PlannerRetryBudget int
	ToolRetryBudget    int
	Hook               Hook
	// EmitEvent mirrors lifecycle events as soon as they happen so transports can
	// stream loop progress while the enclosing RPC call is still running.
	EmitEvent func(LifecycleEvent)
	Now       func() time.Time
}

// Runtime executes a bounded ReAct-style loop with structured round state,
// compaction, and explicit stop reasons.
type Runtime struct{}

// NewRuntime builds one reusable agent loop runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}

// Run executes the loop for a single task/run pair.
func (r *Runtime) Run(ctx context.Context, request Request) (Result, bool, error) {
	if !isAgentLoopIntent(request.Intent) || request.GenerateToolCalls == nil {
		return Result{}, false, nil
	}
	availableToolDefinitions := filterAllowedToolDefinitions(request.ToolDefinitions, request.AllowedTool)
	availableToolNames := toolDefinitionNameSet(availableToolDefinitions)
	if request.ExecuteTool == nil {
		availableToolDefinitions = nil
		availableToolNames = nil
	}

	if request.MaxTurns <= 0 {
		request.MaxTurns = 4
	}
	if request.AttemptIndex <= 0 {
		request.AttemptIndex = 1
	}
	if strings.TrimSpace(request.SegmentKind) == "" {
		request.SegmentKind = "initial"
	}
	if request.KeepRecent < 0 {
		request.KeepRecent = 0
	}
	if request.RepeatedToolBudget <= 0 {
		request.RepeatedToolBudget = 2
	}
	if request.PlannerRetryBudget <= 0 {
		request.PlannerRetryBudget = 1
	}
	if request.ToolRetryBudget <= 0 {
		request.ToolRetryBudget = 1
	}
	if request.Now == nil {
		request.Now = time.Now
	}
	if request.BuildAuditRecord == nil {
		request.BuildAuditRecord = func(context.Context, *model.InvocationRecord) (map[string]any, error) {
			return nil, nil
		}
	}
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	activeInputText := request.InputText
	history := []string{}
	allToolCalls := []tools.ToolCallRecord{}
	rounds := []PersistedRound{}
	events := []LifecycleEvent{}
	events = appendEvent(events, request, newEvent(request, "loop.started", map[string]any{"status": "processing"}))
	lastPlannerOutputText := ""
	var latestInvocation *model.InvocationRecord
	roundToolHistory := []string{}
	// capabilityReminderUsed keeps the recovery heuristic bounded. Once the
	// planner has been reminded about the exposed tool surface, later denials
	// should surface normally instead of spinning the planner forever.
	capabilityReminderUsed := false

	for turn := 0; turn < request.MaxTurns; turn++ {
		if request.PollSteering != nil {
			steeringMessages := request.PollSteering(ctx, request.TaskID)
			if len(steeringMessages) > 0 {
				// Keep every accepted steering message in the planner prompt so
				// later rounds do not silently discard earlier guidance.
				activeInputText = appendSteeringInput(activeInputText, steeringMessages)
				events = appendEvent(events, request, newEvent(request, "task.steered", map[string]any{
					"task_id":  request.TaskID,
					"messages": append([]string(nil), steeringMessages...),
				}))
			}
		}
		plannerInput, compactedHistory := buildPlannerInput(activeInputText, history, availableToolDefinitions, request.CompressChars, request.KeepRecent)
		round := PersistedRound{
			StepID:        fmt.Sprintf("step_loop_%02d", turn+1),
			RunID:         request.RunID,
			TaskID:        request.TaskID,
			AttemptIndex:  request.AttemptIndex,
			SegmentKind:   request.SegmentKind,
			LoopRound:     turn + 1,
			Name:          "agent_loop_round",
			Status:        "running",
			InputSummary:  truncateText(singleLineSummary(plannerInput), 160),
			StartedAt:     request.Now(),
			PlannerInput:  plannerInput,
			StopReason:    "",
			PlannerOutput: "",
		}
		if request.Hook != nil {
			updatedInput, err := request.Hook.BeforeRound(ctx, round, plannerInput)
			if err != nil {
				return Result{}, true, err
			}
			plannerInput = updatedInput
			round.PlannerInput = plannerInput
			round.InputSummary = truncateText(singleLineSummary(plannerInput), 160)
		}
		events = appendEvent(events, request, newEventForRound(round, "loop.round.started", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound}))

		var plan model.ToolCallResult
		var err error
		for attempt := 0; attempt <= request.PlannerRetryBudget; attempt++ {
			plan, err = request.GenerateToolCalls(ctx, model.ToolCallRequest{
				TaskID: request.TaskID,
				RunID:  request.RunID,
				Input:  plannerInput,
				Tools:  availableToolDefinitions,
			})
			if err == nil {
				break
			}
			if attempt < request.PlannerRetryBudget && shouldRetryPlannerError(err) {
				events = appendEvent(events, request, newEventForRound(round, "loop.retrying", map[string]any{
					"attempt_index": round.AttemptIndex,
					"segment_kind":  round.SegmentKind,
					"loop_round":    round.LoopRound,
					"phase":         "planner",
					"attempt":       attempt + 1,
					"reason":        plannerRetryReason(err),
					"error":         err.Error(),
				}))
				continue
			}
			break
		}
		if err != nil {
			round.Status = "failed"
			round.CompletedAt = request.Now()
			round.StopReason = StopReasonPlannerError
			round.OutputSummary = truncateText(singleLineSummary(err.Error()), 160)
			rounds = append(rounds, round)
			events = appendEvent(events, request, newEventForRound(round, "loop.failed", map[string]any{
				"attempt_index": round.AttemptIndex,
				"segment_kind":  round.SegmentKind,
				"loop_round":    round.LoopRound,
				"stop_reason":   string(StopReasonPlannerError),
				"error":         err.Error(),
			}))
			if request.Hook != nil {
				if err := request.Hook.AfterRound(ctx, round); err != nil {
					return Result{}, true, err
				}
			}
			return Result{
				ToolCalls:       allToolCalls,
				ModelInvocation: invocationRecordMap(latestInvocation),
				Events:          events,
				Rounds:          rounds,
				StopReason:      StopReasonPlannerError,
			}, true, fmt.Errorf("agent loop planning turn %d: %w", turn+1, err)
		}

		latestInvocation = &model.InvocationRecord{
			TaskID:    request.TaskID,
			RunID:     request.RunID,
			RequestID: plan.RequestID,
			Provider:  plan.Provider,
			ModelID:   plan.ModelID,
			Usage:     plan.Usage,
			LatencyMS: plan.LatencyMS,
		}
		round.PlannerOutput = truncateText(singleLineSummary(plan.OutputText), 240)
		plannerOutputText := strings.TrimSpace(plan.OutputText)
		// Planner text from a tool-call round is a note for the next round. It is
		// only a final answer when the planner returns without tool calls.
		if len(plan.ToolCalls) == 0 && shouldCarryPlannerOutputText(plannerOutputText) {
			lastPlannerOutputText = plannerOutputText
		}

		if len(compactedHistory) < len(history) {
			events = appendEvent(events, request, newEventForRound(round, "loop.compacted", map[string]any{
				"attempt_index":       round.AttemptIndex,
				"segment_kind":        round.SegmentKind,
				"loop_round":          round.LoopRound,
				"history_before":      len(history),
				"history_after":       len(compactedHistory),
				"compaction_strategy": "history_summary",
			}))
		}

		if len(plan.ToolCalls) == 0 {
			outputText := strings.TrimSpace(plan.OutputText)
			if !capabilityReminderUsed && turn+1 < request.MaxTurns && shouldRetryForCapabilityReminder(outputText, availableToolDefinitions) {
				capabilityReminderUsed = true
				round.Status = "completed"
				round.CompletedAt = request.Now()
				round.StopReason = StopReasonCompleted
				round.OutputSummary = truncateText(singleLineSummary(outputText), 160)
				rounds = append(rounds, round)
				events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonCompleted)}))
				if request.Hook != nil {
					if err := request.Hook.AfterRound(ctx, round); err != nil {
						return Result{}, true, err
					}
				}
				activeInputText = appendCapabilityReminderInput(activeInputText, availableToolDefinitions)
				// Retry this heuristic only once. Repeated denials after an explicit
				// reminder should return to the caller so the loop stays observable.
				events = appendEvent(events, request, newEventForRound(round, "loop.retrying", map[string]any{
					"attempt_index": round.AttemptIndex,
					"segment_kind":  round.SegmentKind,
					"loop_round":    round.LoopRound,
					"phase":         "planner",
					"attempt":       1,
					"reason":        "capability_reminder",
					"output_text":   truncateText(singleLineSummary(outputText), 160),
				}))
				continue
			}
			stopReason := StopReasonCompleted
			if outputText == "" {
				if strings.TrimSpace(lastPlannerOutputText) != "" {
					outputText = lastPlannerOutputText
				} else {
					outputText = request.FallbackOutput
					stopReason = StopReasonNeedUserInput
				}
			}
			round.Status = "completed"
			round.CompletedAt = request.Now()
			round.StopReason = stopReason
			round.OutputSummary = truncateText(singleLineSummary(outputText), 160)
			rounds = append(rounds, round)
			events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(stopReason)}))
			if request.Hook != nil {
				if err := request.Hook.AfterRound(ctx, round); err != nil {
					return Result{}, true, err
				}
			}
			auditRecord, err := request.BuildAuditRecord(ctx, latestInvocation)
			if err != nil {
				return Result{}, true, err
			}
			events = appendEvent(events, request, newEvent(request, "loop.completed", map[string]any{"stop_reason": string(stopReason)}))
			return Result{
				OutputText:      outputText,
				ToolCalls:       allToolCalls,
				ModelInvocation: invocationRecordMap(latestInvocation),
				AuditRecord:     auditRecord,
				Events:          events,
				Rounds:          rounds,
				StopReason:      stopReason,
			}, true, nil
		}

		if len(availableToolNames) == 0 {
			round.Status = "completed"
			round.CompletedAt = request.Now()
			round.StopReason = StopReasonNoSupportedTools
			round.OutputSummary = truncateText(singleLineSummary(request.FallbackOutput), 160)
			rounds = append(rounds, round)
			events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonNoSupportedTools)}))
			events = appendEvent(events, request, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonNoSupportedTools)}))
			if request.Hook != nil {
				if err := request.Hook.AfterRound(ctx, round); err != nil {
					return Result{}, true, err
				}
			}
			auditRecord, err := request.BuildAuditRecord(ctx, latestInvocation)
			if err != nil {
				return Result{}, true, err
			}
			return Result{
				OutputText:      request.FallbackOutput,
				ToolCalls:       allToolCalls,
				ModelInvocation: invocationRecordMap(latestInvocation),
				AuditRecord:     auditRecord,
				Events:          events,
				Rounds:          rounds,
				StopReason:      StopReasonNoSupportedTools,
			}, true, nil
		}

		observations := make([]string, 0, len(plan.ToolCalls)+1)
		roundExecutedToolSignatures := make([]string, 0, len(plan.ToolCalls))
		executedToolCount := 0
		if shouldCarryPlannerOutputText(plannerOutputText) {
			observations = append(observations, "Planner note: "+plannerOutputText)
		}
		for _, call := range plan.ToolCalls {
			if request.Hook != nil {
				updatedCall, err := request.Hook.BeforeTool(ctx, round, call)
				if err != nil {
					return Result{}, true, err
				}
				call = updatedCall
			}
			toolName := strings.TrimSpace(call.Name)
			if _, ok := availableToolNames[toolName]; !ok {
				observation := fmt.Sprintf("Tool %s is not allowed in the current agent loop.", toolName)
				observations = append(observations, observation)
				round.ToolName = toolName
				round.Observation = observation
				continue
			}

			observation, record := request.ExecuteTool(ctx, call, turn+1)
			for attempt := 0; attempt < request.ToolRetryBudget && shouldRetryToolRecord(record); attempt++ {
				events = appendEvent(events, request, newEventForRound(round, "loop.retrying", map[string]any{
					"attempt_index": round.AttemptIndex,
					"segment_kind":  round.SegmentKind,
					"loop_round":    round.LoopRound,
					"phase":         "tool",
					"attempt":       attempt + 1,
					"tool_name":     toolName,
					"reason":        toolRetryReason(record),
				}))
				observation, record = request.ExecuteTool(ctx, call, turn+1)
			}
			if record.ToolName != "" {
				allToolCalls = append(allToolCalls, record)
				round.ToolCallRecord = record
				round.ToolName = record.ToolName
				executedToolCount++
				roundExecutedToolSignatures = append(roundExecutedToolSignatures, toolInvocationSignature(call))
			}
			if record.Status == tools.ToolCallStatusTimeout {
				round.Status = "completed"
				round.CompletedAt = request.Now()
				round.StopReason = StopReasonToolRetryExhausted
				round.Observation = truncateText(singleLineSummary(observation), 240)
				round.OutputSummary = truncateText(singleLineSummary(bestEffortLoopOutput(request.FallbackOutput, lastPlannerOutputText)), 160)
				rounds = append(rounds, round)
				events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonToolRetryExhausted)}))
				events = appendEvent(events, request, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonToolRetryExhausted), "tool_name": toolName}))
				if request.Hook != nil {
					if err := request.Hook.AfterTool(ctx, round, record, observation); err != nil {
						return Result{}, true, err
					}
					if err := request.Hook.AfterRound(ctx, round); err != nil {
						return Result{}, true, err
					}
				}
				auditRecord, auditErr := request.BuildAuditRecord(ctx, latestInvocation)
				if auditErr != nil {
					return Result{}, true, auditErr
				}
				return Result{
					OutputText:      bestEffortLoopOutput(request.FallbackOutput, lastPlannerOutputText),
					ToolCalls:       allToolCalls,
					ModelInvocation: invocationRecordMap(latestInvocation),
					AuditRecord:     auditRecord,
					Events:          events,
					Rounds:          rounds,
					StopReason:      StopReasonToolRetryExhausted,
				}, true, nil
			}
			if toolCallNeedsAuthorization(record) {
				round.Status = "completed"
				round.CompletedAt = request.Now()
				round.StopReason = StopReasonNeedAuthorization
				round.Observation = truncateText(singleLineSummary(observation), 240)
				round.OutputSummary = truncateText(singleLineSummary(observation), 160)
				rounds = append(rounds, round)
				events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonNeedAuthorization)}))
				if request.Hook != nil {
					if err := request.Hook.AfterTool(ctx, round, record, observation); err != nil {
						return Result{}, true, err
					}
					if err := request.Hook.AfterRound(ctx, round); err != nil {
						return Result{}, true, err
					}
				}
				auditRecord, auditErr := request.BuildAuditRecord(ctx, latestInvocation)
				if auditErr != nil {
					return Result{}, true, auditErr
				}
				return Result{
					OutputText:      observation,
					ToolCalls:       allToolCalls,
					ModelInvocation: invocationRecordMap(latestInvocation),
					AuditRecord:     auditRecord,
					Events:          events,
					Rounds:          rounds,
					StopReason:      StopReasonNeedAuthorization,
				}, true, nil
			}
			observations = append(observations, observation)
			round.Observation = truncateText(singleLineSummary(observation), 240)
			events = appendEvent(events, request, newEventForRound(round, "tool_call.observed", map[string]any{
				"attempt_index": round.AttemptIndex,
				"segment_kind":  round.SegmentKind,
				"loop_round":    round.LoopRound,
				"tool_name":     round.ToolName,
				"observation":   round.Observation,
			}))
			if request.Hook != nil {
				if err := request.Hook.AfterTool(ctx, round, record, observation); err != nil {
					return Result{}, true, err
				}
			}
		}

		if executedToolCount == 0 {
			if plannerOutputText != "" {
				if !capabilityReminderUsed && turn+1 < request.MaxTurns && shouldRetryForCapabilityReminder(plannerOutputText, availableToolDefinitions) {
					capabilityReminderUsed = true
					round.Status = "completed"
					round.CompletedAt = request.Now()
					round.StopReason = StopReasonCompleted
					round.OutputSummary = truncateText(singleLineSummary(plannerOutputText), 160)
					rounds = append(rounds, round)
					events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonCompleted)}))
					if request.Hook != nil {
						if err := request.Hook.AfterRound(ctx, round); err != nil {
							return Result{}, true, err
						}
					}
					activeInputText = appendCapabilityReminderInput(activeInputText, availableToolDefinitions)
					events = appendEvent(events, request, newEventForRound(round, "loop.retrying", map[string]any{
						"attempt_index": round.AttemptIndex,
						"segment_kind":  round.SegmentKind,
						"loop_round":    round.LoopRound,
						"phase":         "planner",
						"attempt":       1,
						"reason":        "capability_reminder",
						"output_text":   truncateText(singleLineSummary(plannerOutputText), 160),
					}))
					continue
				}
			}
			// A round that selected tools is not a final answer, even when every
			// selected tool was filtered out before execution.
			round.Status = "completed"
			round.CompletedAt = request.Now()
			round.StopReason = StopReasonNoSupportedTools
			round.OutputSummary = truncateText(singleLineSummary(request.FallbackOutput), 160)
			rounds = append(rounds, round)
			events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonNoSupportedTools)}))
			events = appendEvent(events, request, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonNoSupportedTools)}))
			if request.Hook != nil {
				if err := request.Hook.AfterRound(ctx, round); err != nil {
					return Result{}, true, err
				}
			}
			auditRecord, err := request.BuildAuditRecord(ctx, latestInvocation)
			if err != nil {
				return Result{}, true, err
			}
			return Result{
				OutputText:      request.FallbackOutput,
				ToolCalls:       allToolCalls,
				ModelInvocation: invocationRecordMap(latestInvocation),
				AuditRecord:     auditRecord,
				Events:          events,
				Rounds:          rounds,
				StopReason:      StopReasonNoSupportedTools,
			}, true, nil
		}

		sort.Strings(roundExecutedToolSignatures)
		if roundToolSignature := strings.Join(roundExecutedToolSignatures, ","); roundToolSignature != "" {
			roundToolHistory = append(roundToolHistory, roundToolSignature)
			if repeatedToolPatternExceeded(roundToolHistory, request.RepeatedToolBudget) {
				round.Status = "completed"
				round.CompletedAt = request.Now()
				round.StopReason = StopReasonRepeatedToolChoice
				round.OutputSummary = truncateText(singleLineSummary(bestEffortLoopOutput(request.FallbackOutput, lastPlannerOutputText)), 160)
				rounds = append(rounds, round)
				events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonRepeatedToolChoice)}))
				events = appendEvent(events, request, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonRepeatedToolChoice), "tool_name": round.ToolName}))
				if request.Hook != nil {
					if err := request.Hook.AfterRound(ctx, round); err != nil {
						return Result{}, true, err
					}
				}
				auditRecord, err := request.BuildAuditRecord(ctx, latestInvocation)
				if err != nil {
					return Result{}, true, err
				}
				return Result{
					OutputText:      bestEffortLoopOutput(request.FallbackOutput, lastPlannerOutputText),
					ToolCalls:       allToolCalls,
					ModelInvocation: invocationRecordMap(latestInvocation),
					AuditRecord:     auditRecord,
					Events:          events,
					Rounds:          rounds,
					StopReason:      StopReasonRepeatedToolChoice,
				}, true, nil
			}
		}
		history = append(history, observations...)
		round.Status = "completed"
		round.CompletedAt = request.Now()
		round.StopReason = StopReasonCompleted
		round.OutputSummary = truncateText(singleLineSummary(strings.Join(observations, " | ")), 160)
		rounds = append(rounds, round)
		events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonCompleted)}))
		if request.Hook != nil {
			if err := request.Hook.AfterRound(ctx, round); err != nil {
				return Result{}, true, err
			}
		}
	}

	auditRecord, err := request.BuildAuditRecord(ctx, latestInvocation)
	if err != nil {
		return Result{}, true, err
	}
	events = appendEvent(events, request, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonMaxIterations)}))
	return Result{
		OutputText:      bestEffortLoopOutput(request.FallbackOutput, lastPlannerOutputText),
		ToolCalls:       allToolCalls,
		ModelInvocation: invocationRecordMap(latestInvocation),
		AuditRecord:     auditRecord,
		Events:          events,
		Rounds:          rounds,
		StopReason:      StopReasonMaxIterations,
	}, true, nil
}

func isAgentLoopIntent(taskIntent map[string]any) bool {
	return strings.TrimSpace(stringValue(taskIntent, "name", "")) == defaultIntentName
}

func buildPlannerInput(inputText string, history []string, toolDefinitions []model.ToolDefinition, compressChars, keepRecent int) (string, []string) {
	compressedHistory := compactHistory(history, compressChars, keepRecent)
	english := languagepolicy.PreferredReplyLanguage(inputText) == languagepolicy.ReplyLanguageEnglish
	sections := plannerInstructionSections(english)
	if capabilityLines := buildToolCapabilityLines(toolDefinitions); len(capabilityLines) > 0 {
		sections = append(sections, "", plannerCapabilityHeading(english))
		sections = append(sections, capabilityLines...)
	}
	sections = append(sections, "", plannerContextHeading(english), strings.TrimSpace(inputText))
	if len(compressedHistory) > 0 {
		sections = append(sections, "", plannerHistoryHeading(english))
		sections = append(sections, compressedHistory...)
	}
	return strings.Join(sections, "\n"), compressedHistory
}

func plannerInstructionSections(english bool) []string {
	if english {
		return []string{
			"You are the planning round for a desktop agent.",
			"Use English for this request unless the user explicitly asks for another language.",
			"First decide whether you can answer directly; call tools only when they will clearly improve the result.",
			"If an available tool can help finish the task, prefer using the tool instead of claiming you cannot do it.",
			"Lead with the conclusion, stay concise, and avoid filler.",
			"Do not invent file contents, directory entries, or webpage contents.",
			"If the task is already clear and does not need tools, give the final answer directly.",
		}
	}
	return []string{
		"你是桌面 Agent 的规划轮次。",
		"默认使用中文回答；只有在用户明确要求其他语言时才切换。",
		"先判断能否直接回答；只有在工具能明显提升结果时才调用工具。",
		"如果当前已开放的工具能帮助完成任务，优先调用工具，不要先说自己做不到。",
		"最终答复先给结论，保持精简，不要堆砌客套话。",
		"不要编造文件内容、目录项或网页内容。",
		"如果任务已经足够清晰且不需要工具，直接给最终答复。",
	}
}

func plannerCapabilityHeading(english bool) string {
	if english {
		return "Available tools:"
	}
	return "当前可用能力："
}

func plannerContextHeading(english bool) string {
	if english {
		return "User context:"
	}
	return "用户上下文："
}

func plannerHistoryHeading(english bool) string {
	if english {
		return "Observed tool results:"
	}
	return "已观察到的工具结果："
}

func filterAllowedToolDefinitions(toolDefinitions []model.ToolDefinition, allowedTool func(string) bool) []model.ToolDefinition {
	if len(toolDefinitions) == 0 {
		return nil
	}
	if allowedTool == nil {
		return append([]model.ToolDefinition(nil), toolDefinitions...)
	}

	allowed := make([]model.ToolDefinition, 0, len(toolDefinitions))
	for _, definition := range toolDefinitions {
		name := strings.TrimSpace(definition.Name)
		if name == "" || !allowedTool(name) {
			continue
		}
		allowed = append(allowed, definition)
	}
	return allowed
}

func toolDefinitionNameSet(toolDefinitions []model.ToolDefinition) map[string]struct{} {
	result := make(map[string]struct{}, len(toolDefinitions))
	for _, definition := range toolDefinitions {
		name := strings.TrimSpace(definition.Name)
		if name == "" {
			continue
		}
		result[name] = struct{}{}
	}
	return result
}

func toolInvocationSignature(call model.ToolInvocation) string {
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return ""
	}
	encoded, err := json.Marshal(call.Arguments)
	if err != nil {
		return name
	}
	return name + ":" + string(encoded)
}

func repeatedToolPatternExceeded(roundToolHistory []string, budget int) bool {
	if budget < 0 || len(roundToolHistory) < 2 {
		return false
	}
	maxPatternLen := len(roundToolHistory) / 2
	if maxPatternLen == 0 {
		return false
	}
	for patternLen := 1; patternLen <= maxPatternLen; patternLen++ {
		if trailingPatternRepeats(roundToolHistory, patternLen) > budget {
			return true
		}
	}
	return false
}

func trailingPatternRepeats(roundToolHistory []string, patternLen int) int {
	if patternLen <= 0 || len(roundToolHistory) < patternLen {
		return 0
	}
	pattern := roundToolHistory[len(roundToolHistory)-patternLen:]
	repeats := 1
	for start := len(roundToolHistory) - (2 * patternLen); start >= 0; start -= patternLen {
		if !equalStringSlices(roundToolHistory[start:start+patternLen], pattern) {
			break
		}
		repeats++
	}
	return repeats
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func bestEffortLoopOutput(fallbackOutput, plannerOutputText string) string {
	if trimmed := strings.TrimSpace(plannerOutputText); trimmed != "" {
		return trimmed
	}
	return fallbackOutput
}

func shouldCarryPlannerOutputText(outputText string) bool {
	trimmed := strings.TrimSpace(outputText)
	if trimmed == "" {
		return false
	}
	normalized := normalizeCapabilityReminderDenial(trimmed)
	if normalized == "" || looksLikeAnalyzedCapabilityText(normalized) {
		return true
	}
	if hasCapabilityDenialPrefix(normalized) {
		return looksLikeCapabilityDenialWithAnswer(normalized)
	}
	return true
}

func buildToolCapabilityLines(toolDefinitions []model.ToolDefinition) []string {
	lines := make([]string, 0, len(toolDefinitions))
	for _, definition := range toolDefinitions {
		name := strings.TrimSpace(definition.Name)
		if name == "" {
			continue
		}

		line := "- " + name
		if description := strings.TrimSpace(definition.Description); description != "" {
			line += ": " + description
		}
		if requiredFields := toolRequiredFields(definition.InputSchema); len(requiredFields) > 0 {
			separator := "。"
			if strings.HasSuffix(line, ".") || strings.HasSuffix(line, "!") || strings.HasSuffix(line, "?") || strings.HasSuffix(line, "。") || strings.HasSuffix(line, "！") || strings.HasSuffix(line, "？") {
				separator = " "
			}
			line += separator + "必填参数：" + strings.Join(requiredFields, ", ")
		}
		lines = append(lines, line)
	}
	return lines
}

func toolRequiredFields(schema map[string]any) []string {
	requiredValue, ok := schema["required"]
	if !ok {
		return nil
	}

	switch typed := requiredValue.(type) {
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			value, ok := item.(string)
			if !ok {
				continue
			}
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	default:
		return nil
	}
}

func shouldRetryForCapabilityReminder(outputText string, toolDefinitions []model.ToolDefinition) bool {
	// Only plain-text capability denials should trigger the reminder retry. This
	// keeps the recovery path narrow so a genuine refusal does not become an
	// unbounded planner loop just because tools were available in the run.
	if len(toolDefinitions) == 0 {
		return false
	}

	trimmed := strings.TrimSpace(outputText)
	if trimmed == "" {
		return false
	}
	// Quoted or block-quoted text often appears in explain/analyze flows where
	// the model is describing an error string instead of refusing the task.
	if startsWithQuotedCapabilityText(trimmed) {
		return false
	}

	normalized := normalizeCapabilityReminderDenial(trimmed)
	if normalized == "" {
		return false
	}
	if looksLikeAnalyzedCapabilityText(normalized) {
		return false
	}
	if !capabilityReminderMatchesAvailableTools(normalized, toolDefinitions) {
		return false
	}

	if !hasCapabilityDenialPrefix(normalized) {
		return false
	}
	return !looksLikeCapabilityDenialWithAnswer(normalized)
}

func hasCapabilityDenialPrefix(normalized string) bool {
	denialSignals := []string{
		"i cannot access",
		"i can't access",
		"i am unable to access",
		"i'm unable to access",
		"i do not have access",
		"i don't have access",
		"i cannot read files",
		"i can't read files",
		"i cannot browse",
		"i can't browse",
		"i do not have the ability",
		"i don't have the ability",
		"i lack the ability",
		"cannot access",
		"can't access",
		"unable to access",
		"do not have access",
		"don't have access",
		"cannot read files",
		"can't read files",
		"cannot browse",
		"can't browse",
		"do not have the ability",
		"don't have the ability",
		"lack the ability",
		"我没有这个能力",
		"我没有这些能力",
		"我没有能力",
		"我无法访问",
		"我不能访问",
		"我无法读取",
		"我不能读取",
		"我无法查看",
		"我不能查看",
		"我做不到",
		"没有这个能力",
		"没有这些能力",
		"没有能力",
		"无法访问",
		"不能访问",
		"无法读取",
		"不能读取",
		"无法查看",
		"不能查看",
		"做不到",
	}
	for _, signal := range denialSignals {
		if strings.HasPrefix(normalized, signal) {
			return true
		}
	}
	return false
}

func capabilityReminderMatchesAvailableTools(outputText string, toolDefinitions []model.ToolDefinition) bool {
	hasFileTool := false
	hasPageTool := false
	for _, definition := range toolDefinitions {
		switch toolDefinitionCapabilityKind(definition) {
		case "file":
			hasFileTool = true
		case "page":
			hasPageTool = true
		}
	}
	switch deniedCapabilityKind(outputText) {
	case "page":
		return hasPageTool
	case "file":
		return hasFileTool
	default:
		return false
	}
}

func toolDefinitionCapabilityKind(definition model.ToolDefinition) string {
	searchable := strings.ToLower(strings.TrimSpace(definition.Name + " " + definition.Description))
	pageKeywords := []string{"page", "web", "browser", "url", "site", "网页", "网站", "页面"}
	for _, keyword := range pageKeywords {
		if strings.Contains(searchable, keyword) {
			return "page"
		}
	}
	fileKeywords := []string{"file", "dir", "directory", "workspace", "repo", "repository", "文件", "目录", "工作区", "仓库"}
	for _, keyword := range fileKeywords {
		if strings.Contains(searchable, keyword) {
			return "file"
		}
	}
	return "generic"
}

func deniedCapabilityKind(outputText string) string {
	pageKeywords := []string{"browse", "website", "web page", "webpage", "url", "网页", "网站", "页面"}
	for _, keyword := range pageKeywords {
		if strings.Contains(outputText, keyword) {
			return "page"
		}
	}
	fileKeywords := []string{"workspace", "file", "files", "directory", "repo", "repository", "working tree", "文件", "工作区", "目录", "仓库"}
	for _, keyword := range fileKeywords {
		if strings.Contains(outputText, keyword) {
			return "file"
		}
	}
	return "generic"
}

func looksLikeCapabilityDenialWithAnswer(outputText string) bool {
	answerMarkers := []string{
		", but based on",
		" but based on",
		", but if you",
		" but if you",
		", but you can",
		" but you can",
		" however, based on",
		" however, if you",
		"please paste",
		"can you paste",
		"could you paste",
		"please upload",
		"can you upload",
		"could you upload",
		"paste the",
		"upload the",
		"share the",
		"can you share",
		"could you share",
		"send the",
		"i can help analyze",
		"i can help explain",
		"i can still help",
		"likely fix",
		"the fix is",
		"the likely fix is",
		"the likely cause is",
		"the root cause is",
		"root cause is",
		"the issue is",
		"the problem is",
		"check whether",
		"check if",
		"verify whether",
		"verify if",
		"look for",
		"inspect whether",
		"so here's",
		"so here is",
		"here's",
		"here is",
		"what to change",
		"change is",
		"you should",
		"try ",
		"但根据你提供",
		"不过根据你提供",
		"但基于你提供",
		"不过基于你提供",
		"但如果你提供",
		"不过如果你提供",
		"但你可以",
		"不过你可以",
		"但可以",
		"不过可以",
		"但我可以",
		"不过我可以",
		"根据你提供",
		"基于你提供",
		"请粘贴",
		"请上传",
		"能把",
		"可以把",
		"方便把",
		"把内容贴",
		"把文件上传",
		"我可以帮你分析",
		"我可以帮你解释",
		"我可以继续帮你",
		"可以帮你分析",
		"可以帮你解释",
		"你贴出来",
		"可能的修复",
		"修复方式",
		"问题在于",
		"原因是",
		"根因是",
		"可以尝试",
		"建议你",
		"检查是否",
		"确认是否",
		"看看是否",
		"这里是",
		"这里要改",
		"需要修改",
	}
	for _, marker := range answerMarkers {
		if strings.Contains(outputText, marker) {
			return true
		}
	}
	return false
}

func looksLikeAnalyzedCapabilityText(outputText string) bool {
	analysisPrefixes := []string{
		"the error",
		"this error",
		"error message",
		"the message",
		"this message",
		"错误信息",
		"这个错误",
		"这条错误",
		"报错",
	}
	for _, prefix := range analysisPrefixes {
		if strings.HasPrefix(outputText, prefix) {
			return true
		}
	}
	return false
}

func startsWithQuotedCapabilityText(outputText string) bool {
	return strings.HasPrefix(outputText, "\"") ||
		strings.HasPrefix(outputText, "'") ||
		strings.HasPrefix(outputText, "`") ||
		strings.HasPrefix(outputText, "“") ||
		strings.HasPrefix(outputText, "‘") ||
		strings.HasPrefix(outputText, ">")
}

func normalizeCapabilityReminderDenial(outputText string) string {
	normalized := strings.ToLower(strings.TrimSpace(outputText))
	if normalized == "" {
		return ""
	}
	normalized = stripCapabilityRoleLeadIn(normalized)

	leadIns := []string{
		"sorry,",
		"sorry，",
		"sorry",
		"sorry but",
		"i'm sorry,",
		"i'm sorry，",
		"i'm sorry",
		"i am sorry,",
		"i am sorry，",
		"i am sorry",
		"unfortunately,",
		"unfortunately，",
		"unfortunately",
		"抱歉，",
		"抱歉,",
		"抱歉",
		"对不起，",
		"对不起,",
		"对不起",
		"不好意思，",
		"不好意思,",
		"不好意思",
		"as an ai,",
		"as an ai，",
		"as an ai",
		"as an ai assistant\n",
		"as an ai assistant\r\n",
		"as an ai assistant.",
		"作为 ai，",
		"作为 ai,",
		"作为 ai",
		"作为 ai 助手\n",
		"作为 ai 助手\r\n",
		"作为 ai 助手。",
		"assistant.",
		"assistant。",
		"assistant-",
		"assistant -",
		"assistant\n",
		"assistant\r\n",
		"assistant",
		"assistant:",
		"assistant：",
		"model.",
		"model。",
		"model-",
		"model -",
		"model",
		"model:",
		"model：",
		"助手.",
		"助手。",
		"助手-",
		"助手 -",
		"助手\n",
		"助手\r\n",
		"助手",
		"助手:",
		"助手：",
		"作为ai，",
		"作为ai,",
		"作为ai",
	}
	for {
		stripped := false
		for _, leadIn := range leadIns {
			if strings.HasPrefix(normalized, leadIn) {
				normalized = strings.TrimSpace(strings.TrimPrefix(normalized, leadIn))
				normalized = strings.TrimLeft(normalized, " \t\r\n-*•:：")
				stripped = true
			}
		}
		if !stripped {
			break
		}
	}
	normalized = stripCapabilityRoleLeadIn(normalized)

	softenedPrefixes := [][2]string{
		{"i still ", "i "},
		{"i currently ", "i "},
		{"i cannot currently ", "i cannot "},
		{"i can't currently ", "i can't "},
		{"i do not currently have access", "i do not have access"},
		{"i don't currently have access", "i don't have access"},
		{"i do not have direct access", "i do not have access"},
		{"i don't have direct access", "i don't have access"},
		{"i cannot directly ", "i cannot "},
		{"i can't directly ", "i can't "},
		{"but ", ""},
		{"in this environment ", ""},
		{"from here ", ""},
		{"here ", ""},
		{"cannot directly ", "cannot "},
		{"can't directly ", "can't "},
		{"do not have direct access", "do not have access"},
		{"don't have direct access", "don't have access"},
		{"still ", ""},
		{"currently ", ""},
		{"我现在", "我"},
		{"我仍然", "我"},
		{"我还是", "我"},
		{"我无法直接", "我无法"},
		{"我不能直接", "我不能"},
		{"但是", ""},
		{"但", ""},
		{"在这个环境里", ""},
		{"在这个环境中", ""},
		{"当前环境下", ""},
		{"无法直接", "无法"},
		{"不能直接", "不能"},
		{"没有直接", "没有"},
		{"现在", ""},
		{"仍然", ""},
		{"还是", ""},
		{"目前", ""},
		{"暂时", ""},
	}
	for {
		stripped := false
		for _, prefix := range softenedPrefixes {
			if strings.HasPrefix(normalized, prefix[0]) {
				normalized = strings.TrimSpace(prefix[1] + strings.TrimPrefix(normalized, prefix[0]))
				stripped = true
			}
		}
		if !stripped {
			break
		}
	}
	return strings.TrimLeft(normalized, " \t\r\n-*•:：")
}

func stripCapabilityRoleLeadIn(normalized string) string {
	rolePrefixes := []string{"as an ai", "as an ai assistant", "as a language model", "as an assistant", "assistant", "model", "作为 ai", "作为ai", "作为 ai 助手", "作为ai助手", "作为语言模型", "助手"}
	for _, prefix := range rolePrefixes {
		if !strings.HasPrefix(normalized, prefix) {
			continue
		}
		rest := normalized[len(prefix):]
		if index := strings.IndexAny(rest, ",，:：-.。"); index >= 0 && len(prefix)+index < 96 {
			_, size := utf8.DecodeRuneInString(rest[index:])
			candidate := strings.TrimLeft(strings.TrimSpace(rest[index+size:]), " \t\r\n-*•:：")
			if hasCapabilityDenialPrefix(candidate) {
				return candidate
			}
		}
		if index := strings.IndexAny(rest, "\r\n"); index >= 0 && len(prefix)+index < 96 {
			candidate := strings.TrimLeft(strings.TrimSpace(rest[index+1:]), " \t\r\n-*•:：")
			if hasCapabilityDenialPrefix(candidate) {
				return candidate
			}
		}
		if index := strings.Index(rest, " i"); index >= 0 && len(prefix)+index < 96 {
			candidate := strings.TrimLeft(strings.TrimSpace(rest[index+1:]), " \t\r\n-*•:：")
			if hasCapabilityDenialPrefix(candidate) {
				return candidate
			}
		}
		if index := strings.Index(rest, "我"); index >= 0 && len(prefix)+index < 96 {
			candidate := strings.TrimLeft(strings.TrimSpace(rest[index:]), " \t\r\n-*•:：")
			if hasCapabilityDenialPrefix(candidate) {
				return candidate
			}
		}
	}
	return normalized
}

func appendCapabilityReminderInput(inputText string, toolDefinitions []model.ToolDefinition) string {
	// Append the reminder to the original user input so the next planner round
	// keeps the task context while restating the bounded tool surface.
	sections := []string{
		strings.TrimSpace(inputText),
		"",
		"能力提醒：",
		"- 当前这轮已经开放下列工具能力。",
		"- 在回答“做不到”或“无法访问”之前，先判断这些工具是否适用。",
		"- 如果工具能帮助完成任务，就调用工具；否则按用户要求的语言给出简洁答复；若用户未指定语言，默认中文。",
	}
	if capabilityLines := buildToolCapabilityLines(toolDefinitions); len(capabilityLines) > 0 {
		sections = append(sections, "当前可用能力：")
		sections = append(sections, capabilityLines...)
	}
	return strings.TrimSpace(strings.Join(sections, "\n"))
}

func appendSteeringInput(inputText string, steeringMessages []string) string {
	if len(steeringMessages) == 0 {
		return inputText
	}
	steeringLines := make([]string, 0, len(steeringMessages))
	for _, item := range steeringMessages {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		steeringLines = append(steeringLines, "- "+trimmed)
	}
	if len(steeringLines) == 0 {
		return inputText
	}
	return strings.TrimSpace(inputText) + "\n\n补充要求：\n" + strings.Join(steeringLines, "\n")
}

func compactHistory(history []string, compressChars, keepRecent int) []string {
	if len(history) == 0 {
		return nil
	}
	if compressChars <= 0 || keepRecent < 0 {
		return append([]string(nil), history...)
	}

	normalized := make([]string, 0, len(history))
	totalChars := 0
	for _, item := range history {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
		totalChars += len(trimmed)
	}
	if len(normalized) == 0 || totalChars <= compressChars || len(normalized) <= keepRecent {
		return normalized
	}
	if keepRecent > len(normalized) {
		keepRecent = len(normalized)
	}
	headCount := len(normalized) - keepRecent
	headSummary := summarizeHistory(normalized[:headCount], compressChars/2)
	result := make([]string, 0, keepRecent+1)
	if headSummary != "" {
		result = append(result, headSummary)
	}
	result = append(result, normalized[headCount:]...)
	return result
}

func summarizeHistory(history []string, maxChars int) string {
	if len(history) == 0 || maxChars <= 0 {
		return ""
	}
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Compressed earlier observations (%d items):", len(history)))
	for index, item := range history {
		snippet := singleLineSummary(item)
		entry := "\n- " + truncateText(snippet, 160)
		if builder.Len()+len(entry) > maxChars {
			remaining := len(history) - index
			if remaining > 0 {
				builder.WriteString(fmt.Sprintf("\n- ... %d more observations omitted", remaining))
			}
			break
		}
		builder.WriteString(entry)
	}
	return builder.String()
}

func newEvent(request Request, eventType string, payload map[string]any) LifecycleEvent {
	return LifecycleEvent{
		Type:      eventType,
		Level:     "info",
		Payload:   cloneMap(payload),
		CreatedAt: request.Now(),
	}
}

func appendEvent(events []LifecycleEvent, request Request, event LifecycleEvent) []LifecycleEvent {
	if request.EmitEvent != nil {
		request.EmitEvent(event)
	}
	return append(events, event)
}

func newEventForRound(round PersistedRound, eventType string, payload map[string]any) LifecycleEvent {
	return LifecycleEvent{
		Type:      eventType,
		Level:     "info",
		StepID:    round.StepID,
		Payload:   cloneMap(payload),
		CreatedAt: firstNonZeroTime(round.CompletedAt, round.StartedAt),
	}
}

func firstNonZeroTime(primary, fallback time.Time) time.Time {
	if !primary.IsZero() {
		return primary
	}
	return fallback
}

func invocationRecordMap(record *model.InvocationRecord) map[string]any {
	if record == nil {
		return nil
	}
	return record.Map()
}

func truncateText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	return textutil.TruncateGraphemes(trimmed, limit)
}

func singleLineSummary(value string) string {
	lines := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, " ")
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneMap(typed)
		case []map[string]any:
			cloned := make([]map[string]any, 0, len(typed))
			for _, item := range typed {
				cloned = append(cloned, cloneMap(item))
			}
			result[key] = cloned
		default:
			result[key] = value
		}
	}
	return result
}

func stringValue(input map[string]any, key, fallback string) string {
	if input == nil {
		return fallback
	}
	value, ok := input[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// shouldRetryPlannerError keeps planner retries narrow and deterministic so
// runtime behavior stays explainable across timeout, rate-limit, and hard-fail
// branches.
func shouldRetryPlannerError(err error) bool {
	if err == nil {
		return false
	}
	switch plannerRetryReason(err) {
	case "timeout", "rate_limited", "provider_unavailable":
		return true
	default:
		return false
	}
}

func plannerRetryReason(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, model.ErrOpenAIRequestTimeout) {
		return "timeout"
	}
	var statusErr *model.OpenAIHTTPStatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.StatusCode == http.StatusTooManyRequests:
			return "rate_limited"
		case statusErr.StatusCode >= 500 && statusErr.StatusCode <= 599:
			return "provider_unavailable"
		default:
			return "non_retryable_status"
		}
	}
	return "non_retryable_error"
}

func shouldRetryToolRecord(record tools.ToolCallRecord) bool {
	return toolRetryReason(record) == "timeout"
}

func toolCallNeedsAuthorization(record tools.ToolCallRecord) bool {
	if record.ErrorCode != nil && *record.ErrorCode == tools.ToolErrorCodeApprovalRequired {
		return true
	}
	if record.Status != tools.ToolCallStatusFailed {
		return false
	}
	value, ok := record.Output["approval_required"].(bool)
	return ok && value
}

func toolRetryReason(record tools.ToolCallRecord) string {
	if record.Status == tools.ToolCallStatusTimeout {
		return "timeout"
	}
	if record.Status == tools.ToolCallStatusFailed && record.ErrorCode != nil {
		switch *record.ErrorCode {
		case tools.ToolErrorCodeExecutionFailed, tools.ToolErrorCodeWorkerNotAvailable, tools.ToolErrorCodePlaywrightSidecarFail, tools.ToolErrorCodeOCRWorkerFailed, tools.ToolErrorCodeMediaWorkerFailed:
			return "non_retryable_failure"
		case tools.ToolErrorCodeOutputInvalid, tools.ToolErrorCodeNotFound:
			return "validation"
		}
	}
	return "non_retryable"
}
