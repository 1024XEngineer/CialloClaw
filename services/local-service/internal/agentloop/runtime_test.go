package agentloop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestRunMergesSteeringMessagesIntoLaterPlannerRounds(t *testing.T) {
	runtime := NewRuntime()
	plannerInputs := []string{}
	pollCount := 0
	request := testRuntimeRequest()
	request.PollSteering = func(_ context.Context, _ string) []string {
		pollCount++
		if pollCount == 2 {
			return []string{"Also include the latest summary.", "Keep the answer concise."}
		}
		return nil
	}
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		if len(plannerInputs) == 1 {
			return model.ToolCallResult{
				RequestID: "req_round_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			}, nil
		}
		return model.ToolCallResult{
			RequestID:  "req_round_2",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Final answer after steering.",
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed workspace notes directory.", tools.ToolCallRecord{
			ToolCallID: "tool_call_round_1",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_01",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusSucceeded,
			Output:     map[string]any{"loop_round": round},
		}
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected agent loop request to be handled")
	}
	if result.OutputText != "Final answer after steering." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if len(plannerInputs) != 2 {
		t.Fatalf("expected two planner rounds, got %d", len(plannerInputs))
	}
	if !strings.Contains(plannerInputs[1], "Follow-up steering:") {
		t.Fatalf("expected second planner input to include steering section, got %q", plannerInputs[1])
	}
	if !strings.Contains(plannerInputs[1], "Also include the latest summary.") || !strings.Contains(plannerInputs[1], "Keep the answer concise.") {
		t.Fatalf("expected second planner input to include every steering message, got %q", plannerInputs[1])
	}
	if !hasEventType(result.Events, "task.steered") {
		t.Fatalf("expected task.steered event in %+v", result.Events)
	}
}

func TestRunCompactsHistoryBeforeLaterPlannerRounds(t *testing.T) {
	runtime := NewRuntime()
	plannerInputs := []string{}
	request := testRuntimeRequest()
	request.CompressChars = 80
	request.KeepRecent = 1
	toolNames := []string{"read_file", "list_dir", "read_file"}
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		switch len(plannerInputs) {
		case 1, 2, 3:
			return model.ToolCallResult{
				RequestID: "req_round_tool",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: toolNames[len(plannerInputs)-1], Arguments: map[string]any{"path": "notes/source.txt"}}},
			}, nil
		default:
			return model.ToolCallResult{
				RequestID:  "req_round_final",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Finished after compaction.",
			}, nil
		}
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return strings.Repeat("Observation ", 12) + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_compact",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_compact",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusSucceeded,
		}
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if !hasEventType(result.Events, "loop.compacted") {
		t.Fatalf("expected loop.compacted event in %+v", result.Events)
	}
	if len(plannerInputs) < 4 || !strings.Contains(plannerInputs[3], "Compressed earlier observations") {
		t.Fatalf("expected compacted planner input on later round, got %+v", plannerInputs)
	}
}

func TestBuildPlannerInputIncludesToolCatalogAndConcisePolicy(t *testing.T) {
	plannerInput, compactedHistory := buildPlannerInput(
		"Inspect workspace notes and answer.",
		nil,
		[]model.ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a file from the workspace.",
				InputSchema: map[string]any{
					"required": []string{"path"},
				},
			},
			{
				Name:        "page_search",
				Description: "Search for text on a page.",
				InputSchema: map[string]any{
					"required": []any{"url", "query"},
				},
			},
		},
		0,
		0,
	)

	if len(compactedHistory) != 0 {
		t.Fatalf("expected no compacted history, got %+v", compactedHistory)
	}
	if !strings.Contains(plannerInput, "Always respond in Chinese unless the user explicitly asks for another language.") {
		t.Fatalf("expected planner input to pin the default response language, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "Keep final answers concise, lead with the result, and avoid filler.") {
		t.Fatalf("expected planner input to pin the concise response policy, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "Available tools:") {
		t.Fatalf("expected planner input to include available tools, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "- read_file: Read a file from the workspace. Required inputs: path") {
		t.Fatalf("expected planner input to include read_file capability, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "- page_search: Search for text on a page. Required inputs: url, query") {
		t.Fatalf("expected planner input to include page_search capability, got %q", plannerInput)
	}
}

func TestRunRetriesPlannerUpToConfiguredBudget(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.PlannerRetryBudget = 2
	attempts := 0
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
	request.AllowedTool = func(string) bool { return true }
	request.ExecuteTool = func(context.Context, model.ToolInvocation, int) (string, tools.ToolCallRecord) {
		return "unused", tools.ToolCallRecord{ToolName: "read_file", Status: tools.ToolCallStatusSucceeded}
	}
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		attempts++
		return model.ToolCallResult{}, model.ErrOpenAIRequestTimeout
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err == nil {
		t.Fatalf("expected planner error to be returned, got result=%+v handled=%v", result, handled)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if attempts != 3 {
		t.Fatalf("expected planner to be attempted three times, got %d", attempts)
	}
	if result.StopReason != StopReasonPlannerError {
		t.Fatalf("expected planner_error stop reason, got %s", result.StopReason)
	}
	if countEventType(result.Events, "loop.retrying") != 2 {
		t.Fatalf("expected two retry events, got %+v", result.Events)
	}
	if !hasEventType(result.Events, "loop.failed") {
		t.Fatalf("expected loop.failed event in %+v", result.Events)
	}
}

func TestRunStopsPlannerRetriesForNonRetryableErrors(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.PlannerRetryBudget = 2
	attempts := 0
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
	request.AllowedTool = func(string) bool { return true }
	request.ExecuteTool = func(context.Context, model.ToolInvocation, int) (string, tools.ToolCallRecord) {
		return "unused", tools.ToolCallRecord{ToolName: "read_file", Status: tools.ToolCallStatusSucceeded}
	}
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		attempts++
		return model.ToolCallResult{}, &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "bad request"}
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err == nil {
		t.Fatalf("expected planner error to be returned, got result=%+v handled=%v", result, handled)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if attempts != 1 {
		t.Fatalf("expected non-retryable planner error to stop immediately, got %d attempts", attempts)
	}
	if countEventType(result.Events, "loop.retrying") != 0 {
		t.Fatalf("expected no retry event for non-retryable planner error, got %+v", result.Events)
	}
	if result.StopReason != StopReasonPlannerError {
		t.Fatalf("expected planner_error stop reason, got %s", result.StopReason)
	}
	if !hasEventType(result.Events, "loop.failed") {
		t.Fatalf("expected loop.failed event in %+v", result.Events)
	}
}

func TestRunRetriesPlannerForRateLimitAndProviderFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "rate_limit", err: &model.OpenAIHTTPStatusError{StatusCode: 429, Message: "rate limited"}},
		{name: "provider_5xx", err: &model.OpenAIHTTPStatusError{StatusCode: 503, Message: "service unavailable"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := NewRuntime()
			request := testRuntimeRequest()
			request.PlannerRetryBudget = 2
			attempts := 0
			request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
			request.AllowedTool = func(string) bool { return true }
			request.ExecuteTool = func(context.Context, model.ToolInvocation, int) (string, tools.ToolCallRecord) {
				return "unused", tools.ToolCallRecord{ToolName: "read_file", Status: tools.ToolCallStatusSucceeded}
			}
			request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
				attempts++
				return model.ToolCallResult{}, test.err
			}

			result, handled, err := runtime.Run(context.Background(), request)
			if err == nil {
				t.Fatalf("expected planner error to be returned, got result=%+v handled=%v", result, handled)
			}
			if !handled {
				t.Fatal("expected request to be handled")
			}
			if attempts != 3 {
				t.Fatalf("expected retryable planner error to use full retry budget, got %d attempts", attempts)
			}
			if countEventType(result.Events, "loop.retrying") != 2 {
				t.Fatalf("expected retry events for retryable planner error, got %+v", result.Events)
			}
		})
	}
}

func TestRunRetriesTimedOutToolUpToConfiguredBudget(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 1
	request.ToolRetryBudget = 2
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID: "req_tool_retry",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/retry.txt"}}},
		}, nil
	}
	attempts := 0
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		attempts++
		return "tool timeout", tools.ToolCallRecord{
			ToolCallID: "tool_call_timeout",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_timeout",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusTimeout,
			Output:     map[string]any{"loop_round": round, "attempt": attempts},
		}
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if attempts != 3 {
		t.Fatalf("expected tool to be attempted three times, got %d", attempts)
	}
	if result.StopReason != StopReasonToolRetryExhausted {
		t.Fatalf("expected tool_retry_exhausted stop reason, got %s", result.StopReason)
	}
	if countEventType(result.Events, "loop.retrying") != 2 {
		t.Fatalf("expected two tool retry events, got %+v", result.Events)
	}
	if !hasEventType(result.Events, "loop.failed") {
		t.Fatalf("expected loop.failed event, got %+v", result.Events)
	}
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected fallback output after timeout exhaustion, got %+v", result)
	}
}

func TestRunDoesNotRetryNonTimeoutToolFailures(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 1
	request.ToolRetryBudget = 2
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID: "req_tool_failure",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/fail.txt"}}},
		}, nil
	}
	attempts := 0
	executionCode := tools.ToolErrorCodeExecutionFailed
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		attempts++
		return "tool failed", tools.ToolCallRecord{
			ToolCallID: "tool_call_failed",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_failed",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusFailed,
			ErrorCode:  &executionCode,
			Output:     map[string]any{"loop_round": round, "attempt": attempts},
		}
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if attempts != 1 {
		t.Fatalf("expected non-timeout tool failure to avoid in-round retries, got %d attempts", attempts)
	}
	if countEventType(result.Events, "loop.retrying") != 0 {
		t.Fatalf("expected no retry event for non-timeout tool failure, got %+v", result.Events)
	}
	if len(result.Rounds) != 1 || result.Rounds[0].LoopRound != 1 {
		t.Fatalf("expected one persisted round snapshot, got %+v", result.Rounds)
	}
	if result.Rounds[0].ToolCallRecord.Status != tools.ToolCallStatusFailed {
		t.Fatalf("expected failed tool record to remain in round history, got %+v", result.Rounds[0])
	}
	if result.StopReason != StopReasonMaxIterations {
		t.Fatalf("expected single-round failure path to end with max_iterations_reached, got %+v", result.StopReason)
	}
}

func TestRunStopsAfterRepeatedToolChoices(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.RepeatedToolBudget = 1
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID: "req_dead_loop",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed the same directory again.", tools.ToolCallRecord{
			ToolCallID: "tool_call_dead_loop",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_dead_loop",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusSucceeded,
			Output:     map[string]any{"loop_round": round},
		}
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonRepeatedToolChoice {
		t.Fatalf("expected dead_loop_detected stop reason, got %s", result.StopReason)
	}
	if len(result.Rounds) != 2 {
		t.Fatalf("expected second round to stop the dead loop, got %+v", result.Rounds)
	}
	if !hasEventType(result.Events, "loop.failed") {
		t.Fatalf("expected loop.failed event in %+v", result.Events)
	}
}

func TestPlannerRetryReasonClassifiesRetryableErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReason string
		wantRetry  bool
	}{
		{name: "timeout", err: model.ErrOpenAIRequestTimeout, wantReason: "timeout", wantRetry: true},
		{name: "rate_limited", err: &model.OpenAIHTTPStatusError{StatusCode: 429}, wantReason: "rate_limited", wantRetry: true},
		{name: "provider_5xx", err: &model.OpenAIHTTPStatusError{StatusCode: 503}, wantReason: "provider_unavailable", wantRetry: true},
		{name: "validation_4xx", err: &model.OpenAIHTTPStatusError{StatusCode: 400}, wantReason: "non_retryable_status", wantRetry: false},
		{name: "generic_failure", err: errors.New("planner failed"), wantReason: "non_retryable_error", wantRetry: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := plannerRetryReason(test.err); got != test.wantReason {
				t.Fatalf("plannerRetryReason() = %q, want %q", got, test.wantReason)
			}
			if got := shouldRetryPlannerError(test.err); got != test.wantRetry {
				t.Fatalf("shouldRetryPlannerError() = %v, want %v", got, test.wantRetry)
			}
		})
	}
}

func TestToolRetryReasonOnlyRetriesTimeouts(t *testing.T) {
	timeoutCode := tools.ToolErrorCodeTimeout
	executionCode := tools.ToolErrorCodeExecutionFailed
	validationCode := tools.ToolErrorCodeOutputInvalid
	tests := []struct {
		name       string
		record     tools.ToolCallRecord
		wantReason string
		wantRetry  bool
	}{
		{name: "timeout_status", record: tools.ToolCallRecord{Status: tools.ToolCallStatusTimeout, ErrorCode: &timeoutCode}, wantReason: "timeout", wantRetry: true},
		{name: "execution_failed", record: tools.ToolCallRecord{Status: tools.ToolCallStatusFailed, ErrorCode: &executionCode}, wantReason: "non_retryable_failure", wantRetry: false},
		{name: "validation_failed", record: tools.ToolCallRecord{Status: tools.ToolCallStatusFailed, ErrorCode: &validationCode}, wantReason: "validation", wantRetry: false},
		{name: "plain_failure", record: tools.ToolCallRecord{Status: tools.ToolCallStatusFailed}, wantReason: "non_retryable", wantRetry: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := toolRetryReason(test.record); got != test.wantReason {
				t.Fatalf("toolRetryReason() = %q, want %q", got, test.wantReason)
			}
			if got := shouldRetryToolRecord(test.record); got != test.wantRetry {
				t.Fatalf("shouldRetryToolRecord() = %v, want %v", got, test.wantRetry)
			}
		})
	}
}

func TestCompactHistoryKeepsRecentItemsWhenThresholdExceeded(t *testing.T) {
	history := []string{
		"first observation with alpha alpha alpha alpha",
		"second observation with beta beta beta beta",
		"third observation with gamma gamma gamma gamma",
	}
	compacted := compactHistory(history, 60, 1)
	if len(compacted) != 2 {
		t.Fatalf("expected summary plus one recent item, got %+v", compacted)
	}
	if !strings.Contains(compacted[0], "Compressed earlier observations") {
		t.Fatalf("expected compacted head summary, got %+v", compacted)
	}
	if compacted[1] != history[2] {
		t.Fatalf("expected most recent history item to remain verbatim, got %+v", compacted)
	}
}

func TestCompactHistoryReturnsOriginalWhenWithinThreshold(t *testing.T) {
	history := []string{"alpha", "beta"}
	compacted := compactHistory(history, 200, 1)
	if len(compacted) != len(history) {
		t.Fatalf("expected original history length, got %+v", compacted)
	}
	if compacted[0] != "alpha" || compacted[1] != "beta" {
		t.Fatalf("expected original history to stay unchanged, got %+v", compacted)
	}
}

func testRuntimeRequest() Request {
	now := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	return Request{
		TaskID:          "task_runtime_test",
		RunID:           "run_runtime_test",
		Intent:          map[string]any{"name": defaultIntentName, "arguments": map[string]any{}},
		InputText:       "Inspect the workspace and answer.",
		ResultTitle:     "Runtime result",
		FallbackOutput:  "Fallback output",
		ToolDefinitions: []model.ToolDefinition{{Name: "read_file"}, {Name: "list_dir"}},
		AllowedTool:     func(string) bool { return true },
		BuildAuditRecord: func(context.Context, *model.InvocationRecord) (map[string]any, error) {
			return map[string]any{"status": "recorded"}, nil
		},
		Now: func() time.Time {
			now = now.Add(time.Second)
			return now
		},
	}
}

func hasEventType(events []LifecycleEvent, eventType string) bool {
	return countEventType(events, eventType) > 0
}

func countEventType(events []LifecycleEvent, eventType string) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}
