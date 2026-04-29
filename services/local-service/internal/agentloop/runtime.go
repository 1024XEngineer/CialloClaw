package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
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
	if len(request.ToolDefinitions) == 0 || request.ExecuteTool == nil {
		return Result{StopReason: StopReasonNoSupportedTools}, true, nil
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
	var latestInvocation *model.InvocationRecord
	repeatedToolName := ""
	repeatedToolCount := 0

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
		plannerInput, compactedHistory := buildPlannerInput(activeInputText, history, request.ToolDefinitions, request.CompressChars, request.KeepRecent)
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
				Tools:  request.ToolDefinitions,
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
			stopReason := StopReasonCompleted
			if outputText == "" {
				outputText = request.FallbackOutput
				stopReason = StopReasonNeedUserInput
			}
			auditRecord, err := request.BuildAuditRecord(ctx, latestInvocation)
			if err != nil {
				return Result{}, true, err
			}
			round.Status = "completed"
			round.CompletedAt = request.Now()
			round.StopReason = stopReason
			round.OutputSummary = truncateText(singleLineSummary(outputText), 160)
			rounds = append(rounds, round)
			events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(stopReason)}))
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

		observations := make([]string, 0, len(plan.ToolCalls))
		for _, call := range plan.ToolCalls {
			if request.Hook != nil {
				updatedCall, err := request.Hook.BeforeTool(ctx, round, call)
				if err != nil {
					return Result{}, true, err
				}
				call = updatedCall
			}
			toolName := strings.TrimSpace(call.Name)
			if request.AllowedTool != nil && !request.AllowedTool(toolName) {
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
			}
			if record.Status == tools.ToolCallStatusTimeout {
				round.Status = "completed"
				round.CompletedAt = request.Now()
				round.StopReason = StopReasonToolRetryExhausted
				round.OutputSummary = truncateText(singleLineSummary(request.FallbackOutput), 160)
				rounds = append(rounds, round)
				events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonToolRetryExhausted)}))
				events = appendEvent(events, request, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonToolRetryExhausted), "tool_name": toolName}))
				auditRecord, auditErr := request.BuildAuditRecord(ctx, latestInvocation)
				if auditErr != nil {
					return Result{}, true, auditErr
				}
				return Result{
					OutputText:      request.FallbackOutput,
					ToolCalls:       allToolCalls,
					ModelInvocation: invocationRecordMap(latestInvocation),
					AuditRecord:     auditRecord,
					Events:          events,
					Rounds:          rounds,
					StopReason:      StopReasonToolRetryExhausted,
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

		if round.ToolName != "" {
			if round.ToolName == repeatedToolName {
				repeatedToolCount++
			} else {
				repeatedToolName = round.ToolName
				repeatedToolCount = 1
			}
			if repeatedToolCount > request.RepeatedToolBudget {
				round.Status = "completed"
				round.CompletedAt = request.Now()
				round.StopReason = StopReasonRepeatedToolChoice
				round.OutputSummary = truncateText(singleLineSummary(request.FallbackOutput), 160)
				rounds = append(rounds, round)
				events = appendEvent(events, request, newEventForRound(round, "loop.round.completed", map[string]any{"attempt_index": round.AttemptIndex, "segment_kind": round.SegmentKind, "loop_round": round.LoopRound, "stop_reason": string(StopReasonRepeatedToolChoice)}))
				events = appendEvent(events, request, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonRepeatedToolChoice), "tool_name": round.ToolName}))
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
		OutputText:      request.FallbackOutput,
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
	sections := []string{
		"You are the planning step of a desktop agent loop.",
		"Always respond in Chinese unless the user explicitly asks for another language.",
		"Decide whether to answer directly or call one of the provided tools.",
		"If one of the listed tools can help, prefer using it instead of saying the task cannot be done.",
		"Keep final answers concise, lead with the result, and avoid filler.",
		"Use tools only when they materially improve the answer.",
		"Never invent file contents, directory entries, or page contents.",
		"If the task is already clear and no tool is required, return the final answer directly.",
	}
	if capabilityLines := buildToolCapabilityLines(toolDefinitions); len(capabilityLines) > 0 {
		sections = append(sections, "", "Available tools:")
		sections = append(sections, capabilityLines...)
	}
	sections = append(sections, "", "User context:", strings.TrimSpace(inputText))
	if len(compressedHistory) > 0 {
		sections = append(sections, "", "Observed tool results:")
		sections = append(sections, compressedHistory...)
	}
	return strings.Join(sections, "\n"), compressedHistory
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
			separator := ". "
			if strings.HasSuffix(line, ".") || strings.HasSuffix(line, "!") || strings.HasSuffix(line, "?") {
				separator = " "
			}
			line += separator + "Required inputs: " + strings.Join(requiredFields, ", ")
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
	return strings.TrimSpace(inputText) + "\n\nFollow-up steering:\n" + strings.Join(steeringLines, "\n")
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
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	if limit <= 3 {
		return trimmed[:limit]
	}
	return trimmed[:limit-3] + "..."
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

func marshalJSON(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(payload)
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
