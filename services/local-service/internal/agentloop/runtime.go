package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
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

// PersistedRound describes one persisted loop step compatible with the
// `steps` table planned in docs/data-design.md.
type PersistedRound struct {
	StepID         string
	RunID          string
	TaskID         string
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
	InputText          string
	ResultTitle        string
	FallbackOutput     string
	ToolDefinitions    []model.ToolDefinition
	AllowedTool        func(name string) bool
	GenerateToolCalls  func(context.Context, model.ToolCallRequest) (model.ToolCallResult, error)
	ExecuteTool        func(context.Context, model.ToolInvocation, int) (string, tools.ToolCallRecord)
	BuildAuditRecord   func(context.Context, *model.InvocationRecord) (map[string]any, error)
	MaxTurns           int
	CompressChars      int
	KeepRecent         int
	RepeatedToolBudget int
	Now                func() time.Time
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
	if request.KeepRecent < 0 {
		request.KeepRecent = 0
	}
	if request.RepeatedToolBudget <= 0 {
		request.RepeatedToolBudget = 2
	}
	if request.Now == nil {
		request.Now = time.Now
	}

	history := []string{}
	allToolCalls := []tools.ToolCallRecord{}
	rounds := []PersistedRound{}
	events := []LifecycleEvent{newEvent(request, "loop.started", map[string]any{"status": "processing"})}
	var latestInvocation *model.InvocationRecord
	repeatedToolName := ""
	repeatedToolCount := 0

	for turn := 0; turn < request.MaxTurns; turn++ {
		plannerInput, compactedHistory := buildPlannerInput(request.InputText, history, request.CompressChars, request.KeepRecent)
		round := PersistedRound{
			StepID:        fmt.Sprintf("step_loop_%02d", turn+1),
			RunID:         request.RunID,
			TaskID:        request.TaskID,
			LoopRound:     turn + 1,
			Name:          "agent_loop_round",
			Status:        "running",
			InputSummary:  truncateText(singleLineSummary(plannerInput), 160),
			StartedAt:     request.Now(),
			PlannerInput:  plannerInput,
			StopReason:    "",
			PlannerOutput: "",
		}
		events = append(events, newEventForRound(round, "loop.round.started", map[string]any{"loop_round": round.LoopRound}))

		plan, err := request.GenerateToolCalls(ctx, model.ToolCallRequest{
			TaskID: request.TaskID,
			RunID:  request.RunID,
			Input:  plannerInput,
			Tools:  request.ToolDefinitions,
		})
		if err != nil {
			round.Status = "failed"
			round.CompletedAt = request.Now()
			round.StopReason = StopReasonPlannerError
			round.OutputSummary = truncateText(singleLineSummary(err.Error()), 160)
			rounds = append(rounds, round)
			events = append(events, newEventForRound(round, "loop.failed", map[string]any{
				"loop_round":  round.LoopRound,
				"stop_reason": string(StopReasonPlannerError),
				"error":       err.Error(),
			}))
			return Result{}, true, fmt.Errorf("agent loop planning turn %d: %w", turn+1, err)
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
			events = append(events, newEventForRound(round, "loop.compacted", map[string]any{
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
			events = append(events,
				newEventForRound(round, "loop.round.completed", map[string]any{"loop_round": round.LoopRound, "stop_reason": string(stopReason)}),
				newEvent(request, "loop.completed", map[string]any{"stop_reason": string(stopReason)}),
			)
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
			toolName := strings.TrimSpace(call.Name)
			if request.AllowedTool != nil && !request.AllowedTool(toolName) {
				observation := fmt.Sprintf("Tool %s is not allowed in the current agent loop.", toolName)
				observations = append(observations, observation)
				round.ToolName = toolName
				round.Observation = observation
				continue
			}

			observation, record := request.ExecuteTool(ctx, call, turn+1)
			if record.ToolName != "" {
				allToolCalls = append(allToolCalls, record)
				round.ToolCallRecord = record
				round.ToolName = record.ToolName
			}
			observations = append(observations, observation)
			round.Observation = truncateText(singleLineSummary(observation), 240)
			events = append(events, newEventForRound(round, "tool_call.observed", map[string]any{
				"loop_round":  round.LoopRound,
				"tool_name":   round.ToolName,
				"observation": round.Observation,
			}))
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
				events = append(events,
					newEventForRound(round, "loop.round.completed", map[string]any{"loop_round": round.LoopRound, "stop_reason": string(StopReasonRepeatedToolChoice)}),
					newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonRepeatedToolChoice), "tool_name": round.ToolName}),
				)
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
		events = append(events, newEventForRound(round, "loop.round.completed", map[string]any{"loop_round": round.LoopRound, "stop_reason": string(StopReasonCompleted)}))
	}

	auditRecord, err := request.BuildAuditRecord(ctx, latestInvocation)
	if err != nil {
		return Result{}, true, err
	}
	events = append(events, newEvent(request, "loop.failed", map[string]any{"stop_reason": string(StopReasonMaxIterations)}))
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

func buildPlannerInput(inputText string, history []string, compressChars, keepRecent int) (string, []string) {
	compressedHistory := compactHistory(history, compressChars, keepRecent)
	sections := []string{
		"You are the planning step of a desktop agent loop.",
		"Decide whether to answer directly or call one of the provided tools.",
		"Use tools only when they materially improve the answer.",
		"Never invent file contents, directory entries, or page contents.",
		"If the task is already clear and no tool is required, return the final answer directly.",
		"",
		"User context:",
		strings.TrimSpace(inputText),
	}
	if len(compressedHistory) > 0 {
		sections = append(sections, "", "Observed tool results:")
		sections = append(sections, compressedHistory...)
	}
	return strings.Join(sections, "\n"), compressedHistory
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
