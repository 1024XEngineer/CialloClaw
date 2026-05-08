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

type recordingHook struct {
	afterRounds []PersistedRound
	afterTools  []tools.ToolCallRecord
}

func (h *recordingHook) BeforeRound(_ context.Context, _ PersistedRound, plannerInput string) (string, error) {
	return plannerInput, nil
}

func (h *recordingHook) AfterRound(_ context.Context, round PersistedRound) error {
	h.afterRounds = append(h.afterRounds, round)
	return nil
}

func (h *recordingHook) BeforeTool(_ context.Context, _ PersistedRound, call model.ToolInvocation) (model.ToolInvocation, error) {
	return call, nil
}

func (h *recordingHook) AfterTool(_ context.Context, _ PersistedRound, record tools.ToolCallRecord, _ string) error {
	h.afterTools = append(h.afterTools, record)
	return nil
}

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
	if !strings.Contains(plannerInputs[1], "补充要求：") {
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
	if !strings.Contains(plannerInput, "默认使用中文回答；只有在用户明确要求其他语言时才切换。") {
		t.Fatalf("expected planner input to pin the default response language, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "最终答复先给结论，保持精简，不要堆砌客套话。") {
		t.Fatalf("expected planner input to pin the concise response policy, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "当前可用能力：") {
		t.Fatalf("expected planner input to include available tools, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "- read_file: Read a file from the workspace. 必填参数：path") {
		t.Fatalf("expected planner input to include read_file capability, got %q", plannerInput)
	}
	if !strings.Contains(plannerInput, "- page_search: Search for text on a page. 必填参数：url, query") {
		t.Fatalf("expected planner input to include page_search capability, got %q", plannerInput)
	}
}

func TestBuildPlannerInputOmitsToolSectionWithoutTools(t *testing.T) {
	plannerInput, compactedHistory := buildPlannerInput(
		"Just answer directly.",
		nil,
		nil,
		0,
		0,
	)

	if len(compactedHistory) != 0 {
		t.Fatalf("expected no compacted history, got %+v", compactedHistory)
	}
	if strings.Contains(plannerInput, "当前可用能力：") {
		t.Fatalf("expected planner input without tools to omit capability section, got %q", plannerInput)
	}
}

func TestAppendCapabilityReminderInputPreservesLanguageContract(t *testing.T) {
	reminderInput := appendCapabilityReminderInput("Please answer in English.", []model.ToolDefinition{{Name: "read_file"}})

	if !strings.Contains(reminderInput, "按用户要求的语言给出简洁答复；若用户未指定语言，默认中文。") {
		t.Fatalf("expected reminder input to preserve the user language contract, got %q", reminderInput)
	}
	if strings.Contains(reminderInput, "简洁中文答复") {
		t.Fatalf("expected reminder input to avoid forcing Chinese replies, got %q", reminderInput)
	}
}

func TestShouldRetryForCapabilityReminderRequiresExplicitUnquotedDenial(t *testing.T) {
	fileToolDefinitions := []model.ToolDefinition{{Name: "read_file"}}
	pageToolDefinitions := []model.ToolDefinition{{Name: "page_read"}}
	customFileToolDefinitions := []model.ToolDefinition{{Name: "workspace_reader", Description: "Read files from the workspace."}}
	customPageToolDefinitions := []model.ToolDefinition{{Name: "browser_fetch", Description: "Read web pages by URL."}}
	tests := []struct {
		name      string
		output    string
		tools     []model.ToolDefinition
		wantRetry bool
	}{
		{name: "explicit_english_denial", output: "I cannot access workspace files in this environment.", tools: fileToolDefinitions, wantRetry: true},
		{name: "apology_prefix", output: "Sorry, I can't browse websites from here.", tools: pageToolDefinitions, wantRetry: true},
		{name: "apology_but_prefix", output: "Sorry, but I can't browse websites from here.", tools: pageToolDefinitions, wantRetry: true},
		{name: "context_prefixed_english_denial", output: "Unfortunately, in this environment I cannot access workspace files.", tools: fileToolDefinitions, wantRetry: true},
		{name: "role_prefixed_english_denial", output: "As an AI, I can't access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_prefixed_english_denial", output: "As an AI assistant, I can't access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_label_english_denial", output: "Assistant: I can't access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_newline_english_denial", output: "As an AI assistant\nI can't access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_dash_english_denial", output: "Assistant - I can't access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_period_english_denial", output: "As an AI assistant. I can't access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "currently_prefixed_english_denial", output: "I cannot currently access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "currently_access_denial", output: "I do not currently have access to workspace files.", tools: fileToolDefinitions, wantRetry: true},
		{name: "direct_access_denial", output: "I don't have direct access to workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "directly_prefixed_english_denial", output: "I cannot directly access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "softened_prefix", output: "I still cannot access workspace files in this environment.", tools: fileToolDefinitions, wantRetry: true},
		{name: "bare_english_denial", output: "Cannot access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "bare_english_unable", output: "Unable to access workspace files from here.", tools: fileToolDefinitions, wantRetry: true},
		{name: "bare_chinese_denial", output: "无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "role_prefixed_chinese_denial", output: "作为 AI，我无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_prefixed_chinese_denial", output: "作为 AI 助手，我无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_label_chinese_denial", output: "助手：我无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_newline_chinese_denial", output: "作为 AI 助手\n我无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_dash_chinese_denial", output: "助手 - 我无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "assistant_period_chinese_denial", output: "助手。 我无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "softened_chinese_denial", output: "我现在无法访问当前工作区文件。", tools: fileToolDefinitions, wantRetry: true},
		{name: "denial_with_answer", output: "I cannot access workspace files directly, but based on the text you provided, the error means the file path is missing.", tools: fileToolDefinitions, wantRetry: false},
		{name: "denial_with_workaround", output: "I cannot access the file directly; please paste it here and I can help analyze it.", tools: fileToolDefinitions, wantRetry: false},
		{name: "denial_with_question_workaround", output: "I can't access the file directly. Can you paste it here so I can help analyze it?", tools: fileToolDefinitions, wantRetry: false},
		{name: "denial_with_check_if", output: "I can't access the repo directly. Check whether foo is nil before use.", tools: fileToolDefinitions, wantRetry: false},
		{name: "denial_with_likely_fix", output: "I can't access the repo directly, but the likely fix is to nil-check the pointer before use.", tools: fileToolDefinitions, wantRetry: false},
		{name: "denial_with_root_cause", output: "I can't access the repo directly. The root cause is likely a missing nil check before use.", tools: fileToolDefinitions, wantRetry: false},
		{name: "denial_with_so_heres", output: "I can't access the repo directly, so here's the likely fix: nil-check the pointer before use.", tools: fileToolDefinitions, wantRetry: false},
		{name: "non_capability_chinese_limit_with_answer", output: "我不能查看图片，但可以帮你分析你贴出来的内容。", tools: fileToolDefinitions, wantRetry: false},
		{name: "quoted_error_text", output: "\"I cannot access workspace files in this environment\" usually means the runtime did not expose file tools.", tools: fileToolDefinitions, wantRetry: false},
		{name: "analysis_prefix", output: "The error \"I cannot access workspace files in this environment\" usually means the runtime did not expose file tools.", tools: fileToolDefinitions, wantRetry: false},
		{name: "page_denial_without_page_tools", output: "I can't browse websites from here.", tools: fileToolDefinitions, wantRetry: false},
		{name: "page_denial_with_custom_page_tools", output: "I can't browse websites from here.", tools: customPageToolDefinitions, wantRetry: true},
		{name: "file_denial_with_custom_file_tools", output: "I cannot access workspace files in this environment.", tools: customFileToolDefinitions, wantRetry: true},
		{name: "generic_denial_without_capability_match", output: "I can't do that here.", tools: fileToolDefinitions, wantRetry: false},
		{name: "no_tools", output: "I cannot access workspace files in this environment.", tools: nil, wantRetry: false},
		{name: "empty_output", output: "   ", tools: fileToolDefinitions, wantRetry: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldRetryForCapabilityReminder(test.output, test.tools); got != test.wantRetry {
				t.Fatalf("shouldRetryForCapabilityReminder(%q) = %v, want %v", test.output, got, test.wantRetry)
			}
		})
	}
}

func TestRunDoesNotRetryOrdinaryDirectAnswers(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	plannerInputs := []string{}
	executeCalls := 0
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		return model.ToolCallResult{
			RequestID:  "req_direct_answer",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "这是直接回答，不需要调用工具。",
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		executeCalls++
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_direct_answer",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_direct_answer",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "这是直接回答，不需要调用工具。" {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if executeCalls != 0 {
		t.Fatalf("expected no tool execution for direct answer, got %d", executeCalls)
	}
	if len(plannerInputs) != 1 {
		t.Fatalf("expected a single planner round for direct answer, got %+v", plannerInputs)
	}
	if countRetryReason(result.Events, "capability_reminder") != 0 {
		t.Fatalf("expected no capability reminder retry for direct answer, got %+v", result.Events)
	}
}

func TestRunDirectAnswerCallsHookAfterRound(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	hook := &recordingHook{}
	request.Hook = hook
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID:  "req_direct_answer_hook",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Direct answer with hook.",
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_direct_answer_hook",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_direct_answer_hook",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "Direct answer with hook." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if len(hook.afterRounds) != 1 {
		t.Fatalf("expected hook.AfterRound to observe the final direct-answer round, got %+v", hook.afterRounds)
	}
	if hook.afterRounds[0].StopReason != StopReasonCompleted {
		t.Fatalf("expected hook.AfterRound to receive completed stop reason, got %+v", hook.afterRounds[0])
	}
}

func TestRunPlannerFailureCallsHookAfterRound(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	hook := &recordingHook{}
	request.Hook = hook
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "unused", tools.ToolCallRecord{
			ToolCallID: "tool_call_planner_failure_hook",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_planner_failure_hook",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusSucceeded,
			Output:     map[string]any{"loop_round": round},
		}
	}
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{}, model.ErrOpenAIRequestTimeout
	}

	_, handled, err := runtime.Run(context.Background(), request)
	if err == nil {
		t.Fatal("expected planner error")
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if len(hook.afterRounds) != 1 {
		t.Fatalf("expected hook.AfterRound on planner failure, got %+v", hook.afterRounds)
	}
	if hook.afterRounds[0].StopReason != StopReasonPlannerError {
		t.Fatalf("expected planner_error stop reason in hook, got %+v", hook.afterRounds[0])
	}
}

func TestRunDoesNotRetryCapabilityReminderWithoutRemainingTurns(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 1
	plannerInputs := []string{}
	executeCalls := 0
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		return model.ToolCallResult{
			RequestID:  "req_capability_last_turn",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "I cannot access workspace files in this environment.",
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		executeCalls++
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_capability_last_turn",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_last_turn",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "I cannot access workspace files in this environment." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected completed stop reason on final denial turn, got %s", result.StopReason)
	}
	if executeCalls != 0 {
		t.Fatalf("expected no tool execution when reminder retry is unavailable, got %d", executeCalls)
	}
	if len(plannerInputs) != 1 {
		t.Fatalf("expected a single planner round without remaining turns, got %+v", plannerInputs)
	}
	if countRetryReason(result.Events, "capability_reminder") != 0 {
		t.Fatalf("expected no capability reminder retry without remaining turns, got %+v", result.Events)
	}
	if len(result.Rounds) != 1 || result.Rounds[0].StopReason != StopReasonCompleted {
		t.Fatalf("expected the denial round to be persisted as completed, got %+v", result.Rounds)
	}
}

func TestRunAllowsDirectAnswersWhenAllToolsAreDisallowed(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.AllowedTool = func(string) bool { return false }
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_disallowed_tools_direct_answer",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_disallowed_tools_direct_answer",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusSucceeded,
			Output:     map[string]any{"loop_round": round},
		}
	}
	plannerInputs := []string{}
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		if len(req.Tools) != 0 {
			t.Fatalf("expected no planner-visible tools when all tools are disallowed, got %+v", req.Tools)
		}
		return model.ToolCallResult{
			RequestID:  "req_disallowed_tools_direct_answer",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Direct answer without tools.",
		}, nil
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if len(plannerInputs) != 1 {
		t.Fatalf("expected one planner round when all tools are disallowed, got %+v", plannerInputs)
	}
	if strings.Contains(plannerInputs[0], "当前可用能力：") {
		t.Fatalf("expected planner input to omit unavailable capabilities, got %q", plannerInputs[0])
	}
	if result.OutputText != "Direct answer without tools." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected completed stop reason for direct answer without tools, got %s", result.StopReason)
	}
	if countRetryReason(result.Events, "capability_reminder") != 0 {
		t.Fatalf("expected no capability reminder retry without any allowed tools, got %+v", result.Events)
	}
}

func TestRunAllowsDirectAnswersWithoutPlannerTools(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.ToolDefinitions = nil
	request.ExecuteTool = nil
	plannerInputs := []string{}
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		if len(req.Tools) != 0 {
			t.Fatalf("expected planner request without tools, got %+v", req.Tools)
		}
		return model.ToolCallResult{
			RequestID:  "req_no_tools_direct_answer",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Direct answer with no planner tools.",
		}, nil
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if len(plannerInputs) != 1 {
		t.Fatalf("expected one planner round without tools, got %+v", plannerInputs)
	}
	if strings.Contains(plannerInputs[0], "当前可用能力：") {
		t.Fatalf("expected planner input without tools to omit capability section, got %q", plannerInputs[0])
	}
	if result.OutputText != "Direct answer with no planner tools." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected completed stop reason for no-tool direct answer, got %s", result.StopReason)
	}
}

func TestRunAllowsDirectAnswersWithoutAuditBuilder(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.BuildAuditRecord = nil
	request.ToolDefinitions = nil
	request.ExecuteTool = nil
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID:  "req_no_audit_builder",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Direct answer without audit builder.",
		}, nil
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "Direct answer without audit builder." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected completed stop reason without audit builder, got %s", result.StopReason)
	}
}

func TestRunAllowsDirectAnswersWhenExecutorIsMissing(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.ExecuteTool = nil
	plannerInputs := []string{}
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		if len(req.Tools) != 0 {
			t.Fatalf("expected planner-visible tools to be suppressed when executor is missing, got %+v", req.Tools)
		}
		return model.ToolCallResult{
			RequestID:  "req_missing_executor_direct_answer",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Direct answer without executor.",
		}, nil
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if len(plannerInputs) != 1 {
		t.Fatalf("expected one planner round without executor, got %+v", plannerInputs)
	}
	if strings.Contains(plannerInputs[0], "当前可用能力：") {
		t.Fatalf("expected planner input to omit tools when executor is missing, got %q", plannerInputs[0])
	}
	if result.OutputText != "Direct answer without executor." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected completed stop reason without executor, got %s", result.StopReason)
	}
}

func TestRunStopsWithNoSupportedToolsWhenPlannerCallsToolWithoutExecutor(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.ExecuteTool = nil
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		if len(req.Tools) != 0 {
			t.Fatalf("expected planner-visible tools to be suppressed when executor is missing, got %+v", req.Tools)
		}
		return model.ToolCallResult{
			RequestID: "req_missing_executor_tool_call",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/source.txt"}}},
		}, nil
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected fallback output when planner calls tools without executor, got %+v", result)
	}
	if result.StopReason != StopReasonNoSupportedTools {
		t.Fatalf("expected no_supported_tools stop reason, got %s", result.StopReason)
	}
	if len(result.Rounds) != 1 || result.Rounds[0].StopReason != StopReasonNoSupportedTools {
		t.Fatalf("expected persisted no_supported_tools round, got %+v", result.Rounds)
	}
}

func TestRunReturnsPlannerAnswerWhenExecutorMissingAndPlannerAlsoAnswers(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.ExecuteTool = nil
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		if len(req.Tools) != 0 {
			t.Fatalf("expected planner-visible tools to be suppressed when executor is missing, got %+v", req.Tools)
		}
		return model.ToolCallResult{
			RequestID:  "req_missing_executor_mixed_answer",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "You should nil-check the pointer before use.",
			ToolCalls:  []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/source.txt"}}},
		}, nil
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "You should nil-check the pointer before use." {
		t.Fatalf("expected planner direct answer to win when executor is missing, got %+v", result)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected completed stop reason when planner already answered, got %s", result.StopReason)
	}
}

func TestRunStopsWithNoSupportedToolsOnLastDisallowedToolRound(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 1
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
	request.AllowedTool = func(name string) bool { return name == "read_file" }
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID: "req_last_turn_disallowed_tool",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_last_turn_disallowed_tool",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_last_turn_disallowed_tool",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonNoSupportedTools {
		t.Fatalf("expected no_supported_tools stop reason on final disallowed-tool round, got %s", result.StopReason)
	}
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected fallback output on final disallowed-tool round, got %+v", result)
	}
}

func TestRunReturnsPlannerAnswerWhenAllChosenToolsAreDisallowed(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
	request.AllowedTool = func(name string) bool { return name == "read_file" }
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID:  "req_disallowed_tools_with_answer",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Here is what to change: nil-check the pointer before use.",
			ToolCalls:  []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_disallowed_tools_with_answer",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_disallowed_tools_with_answer",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "Here is what to change: nil-check the pointer before use." {
		t.Fatalf("expected planner answer to be preserved when all chosen tools are disallowed, got %+v", result)
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected completed stop reason when planner already answered, got %s", result.StopReason)
	}
}

func TestRunUsesFallbackWhenToolRoundOnlyHasPlannerChatter(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 1
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID:  "req_tool_round_with_chatter",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "I will inspect the file before answering.",
			ToolCalls:  []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/source.txt"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_round_with_chatter",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_tool_round_with_chatter",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonMaxIterations {
		t.Fatalf("expected max_iterations_reached when tool-round chatter cannot finish the loop, got %s", result.StopReason)
	}
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected fallback output instead of tool-round planner chatter, got %+v", result)
	}
}

func TestRunUsesFallbackWhenOnlyPriorToolRoundHadPlannerText(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	plannerCalls := 0
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerCalls++
		switch plannerCalls {
		case 1:
			return model.ToolCallResult{
				RequestID:  "req_tool_round_answer_then_empty_1",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I will inspect the file before answering.",
				ToolCalls:  []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/source.txt"}}},
			}, nil
		default:
			return model.ToolCallResult{
				RequestID:  "req_tool_round_answer_then_empty_2",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "",
			}, nil
		}
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_round_answer_then_empty",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_tool_round_answer_then_empty",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected fallback output when only a prior tool round had planner text, got %+v", result)
	}
	if result.StopReason != StopReasonNeedUserInput {
		t.Fatalf("expected need_user_input when final round has no answer, got %s", result.StopReason)
	}
}

func TestRunDoesNotPromoteMixedDenialTextToFallbackOutput(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 1
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID:  "req_mixed_denial_tool_round",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "I cannot access workspace files from here.",
			ToolCalls:  []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/source.txt"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_mixed_denial_tool_round",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_mixed_denial_tool_round",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonMaxIterations {
		t.Fatalf("expected max_iterations_reached stop reason after single mixed tool round, got %s", result.StopReason)
	}
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected plain denial text not to replace fallback output, got %+v", result)
	}
}

func TestRunRetriesCapabilityReminderForDenialPlusDisallowedToolPlan(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
	request.AllowedTool = func(name string) bool { return name == "read_file" }
	plannerInputs := []string{}
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		switch len(plannerInputs) {
		case 1:
			return model.ToolCallResult{
				RequestID:  "req_denial_disallowed_tool_1",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I cannot access workspace files from here.",
				ToolCalls:  []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			}, nil
		default:
			return model.ToolCallResult{
				RequestID:  "req_denial_disallowed_tool_2",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Recovered after reminder.",
			}, nil
		}
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_denial_disallowed_tool",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_denial_disallowed_tool",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "Recovered after reminder." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if len(plannerInputs) != 2 {
		t.Fatalf("expected denial+disallowed tool plan to retry once, got %+v", plannerInputs)
	}
	if countRetryReason(result.Events, "capability_reminder") != 1 {
		t.Fatalf("expected exactly one capability reminder retry event, got %+v", result.Events)
	}
}

func TestRunTimedOutToolCallsHooksBeforeReturning(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	hook := &recordingHook{}
	request.Hook = hook
	request.MaxTurns = 1
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID: "req_tool_timeout_hook",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/retry.txt"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "tool timeout", tools.ToolCallRecord{
			ToolCallID: "tool_call_timeout_hook",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_timeout_hook",
			ToolName:   call.Name,
			Status:     tools.ToolCallStatusTimeout,
			Output:     map[string]any{"loop_round": round},
		}
	}

	result, handled, err := runtime.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonToolRetryExhausted {
		t.Fatalf("expected tool_retry_exhausted stop reason, got %s", result.StopReason)
	}
	if len(hook.afterTools) != 1 {
		t.Fatalf("expected hook.AfterTool on timeout path, got %+v", hook.afterTools)
	}
	if len(hook.afterRounds) != 1 || hook.afterRounds[0].StopReason != StopReasonToolRetryExhausted {
		t.Fatalf("expected hook.AfterRound on timeout path, got %+v", hook.afterRounds)
	}
}

func TestRunRepeatedToolExitCallsHookAfterRound(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	hook := &recordingHook{}
	request.Hook = hook
	request.RepeatedToolBudget = 1
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID: "req_repeated_tool_hook",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed the same directory again.", tools.ToolCallRecord{
			ToolCallID: "tool_call_repeated_tool_hook",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_repeated_tool_hook",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonRepeatedToolChoice {
		t.Fatalf("expected dead_loop_detected stop reason, got %s", result.StopReason)
	}
	if len(hook.afterRounds) != 2 {
		t.Fatalf("expected hook.AfterRound for both persisted rounds, got %+v", hook.afterRounds)
	}
	if hook.afterRounds[1].StopReason != StopReasonRepeatedToolChoice {
		t.Fatalf("expected final hook round to capture repeated-tool stop reason, got %+v", hook.afterRounds[1])
	}
}

func TestRunDoesNotTreatDifferentMultiToolRoundsAsRepeatedSingleToolLoops(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.RepeatedToolBudget = 1
	plannerCalls := 0
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerCalls++
		switch plannerCalls {
		case 1:
			return model.ToolCallResult{
				RequestID: "req_multi_round_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{
					{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}},
					{Name: "list_dir", Arguments: map[string]any{"path": "notes"}},
				},
			}, nil
		case 2:
			return model.ToolCallResult{
				RequestID: "req_multi_round_2",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{
					{Name: "page_search", Arguments: map[string]any{"url": "https://example.com", "query": "notes"}},
					{Name: "list_dir", Arguments: map[string]any{"path": "notes"}},
				},
			}, nil
		default:
			return model.ToolCallResult{
				RequestID:  "req_multi_round_final",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Finished after distinct multi-tool rounds.",
			}, nil
		}
	}
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}, {Name: "list_dir"}, {Name: "page_search"}}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_multi_round",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_multi_round",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected distinct multi-tool rounds to avoid repeated-tool stop reason, got %s", result.StopReason)
	}
	if result.OutputText != "Finished after distinct multi-tool rounds." {
		t.Fatalf("unexpected output text: %+v", result)
	}
}

func TestRunDoesNotTreatSameToolWithDifferentArgumentsAsRepeatedLoop(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.RepeatedToolBudget = 1
	plannerCalls := 0
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerCalls++
		switch plannerCalls {
		case 1:
			return model.ToolCallResult{
				RequestID: "req_same_tool_args_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}},
			}, nil
		case 2:
			return model.ToolCallResult{
				RequestID: "req_same_tool_args_2",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/b.txt"}}},
			}, nil
		default:
			return model.ToolCallResult{
				RequestID:  "req_same_tool_args_final",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Finished after different file reads.",
			}, nil
		}
	}
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_same_tool_args",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_same_tool_args",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonCompleted {
		t.Fatalf("expected different file arguments to avoid repeated-tool stop reason, got %s", result.StopReason)
	}
}

func TestRunStopsImmediatelyForDisallowedToolChoicesWithoutAnswer(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.RepeatedToolBudget = 1
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}}
	request.AllowedTool = func(name string) bool { return name == "read_file" }
	plannerCalls := 0
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerCalls++
		return model.ToolCallResult{
			RequestID: "req_disallowed_tool_repeat",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_disallowed_tool_repeat",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_disallowed_tool_repeat",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonNoSupportedTools {
		t.Fatalf("expected disallowed tool choices without answer to stop as no_supported_tools, got %s", result.StopReason)
	}
}

func TestRunDetectsOscillatingToolLoops(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 4
	request.RepeatedToolBudget = 1
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}, {Name: "list_dir"}}
	plannerCalls := 0
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerCalls++
		signatures := [][]model.ToolInvocation{
			{{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}},
			{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			{{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}},
			{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
		}
		return model.ToolCallResult{
			RequestID: "req_oscillating_loop",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: signatures[plannerCalls-1],
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_oscillating_loop",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_oscillating_loop",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonRepeatedToolChoice {
		t.Fatalf("expected oscillating A-B-A-B loop to trigger dead-loop detection, got %s", result.StopReason)
	}
}

func TestRunDetectsOscillatingAllowedToolsDespiteDisallowedNoise(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 4
	request.RepeatedToolBudget = 1
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}, {Name: "list_dir"}}
	plannerCalls := 0
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerCalls++
		plans := [][]model.ToolInvocation{
			{{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}, {Name: "page_search", Arguments: map[string]any{"url": "https://example.com", "query": "a"}}},
			{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}, {Name: "page_search", Arguments: map[string]any{"url": "https://example.com", "query": "b"}}},
			{{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}, {Name: "page_search", Arguments: map[string]any{"url": "https://example.com", "query": "c"}}},
			{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}, {Name: "page_search", Arguments: map[string]any{"url": "https://example.com", "query": "d"}}},
		}
		return model.ToolCallResult{
			RequestID: "req_oscillating_loop_with_noise",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: plans[plannerCalls-1],
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_oscillating_loop_with_noise",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_oscillating_loop_with_noise",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonRepeatedToolChoice {
		t.Fatalf("expected disallowed noise to avoid masking oscillating allowed-tool loop, got %s", result.StopReason)
	}
}

func TestRunDetectsReorderedEquivalentMultiToolLoops(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.MaxTurns = 4
	request.RepeatedToolBudget = 1
	request.ToolDefinitions = []model.ToolDefinition{{Name: "read_file"}, {Name: "list_dir"}}
	plannerCalls := 0
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerCalls++
		plans := [][]model.ToolInvocation{
			{{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}, {Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}, {Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}},
			{{Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}, {Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}, {Name: "read_file", Arguments: map[string]any{"path": "notes/a.txt"}}},
		}
		return model.ToolCallResult{
			RequestID: "req_reordered_equivalent_loop",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: plans[plannerCalls-1],
		}, nil
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		return "Observed " + call.Name, tools.ToolCallRecord{
			ToolCallID: "tool_call_reordered_equivalent_loop",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_reordered_equivalent_loop",
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
		t.Fatal("expected request to be handled")
	}
	if result.StopReason != StopReasonRepeatedToolChoice {
		t.Fatalf("expected reordered equivalent multi-tool rounds to trigger dead-loop detection, got %s", result.StopReason)
	}
}

func TestRunRetriesWhenPlannerClaimsCapabilitiesAreUnavailable(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	plannerInputs := []string{}
	executeCalls := 0
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		switch len(plannerInputs) {
		case 1:
			return model.ToolCallResult{
				RequestID:  "req_capability_retry_1",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I cannot access workspace files in this environment.",
			}, nil
		case 2:
			return model.ToolCallResult{
				RequestID: "req_capability_retry_2",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/source.txt"}}},
			}, nil
		default:
			return model.ToolCallResult{
				RequestID:  "req_capability_retry_3",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Done after using the available tool.",
			}, nil
		}
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		executeCalls++
		return "Read the requested file.", tools.ToolCallRecord{
			ToolCallID: "tool_call_capability_retry",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_02",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "Done after using the available tool." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if executeCalls != 1 {
		t.Fatalf("expected one tool execution after capability reminder, got %d", executeCalls)
	}
	if len(result.Rounds) != 3 {
		t.Fatalf("expected all planner rounds to be persisted, got %+v", result.Rounds)
	}
	if result.Rounds[0].PlannerOutput != "I cannot access workspace files in this environment." || result.Rounds[0].OutputSummary != "I cannot access workspace files in this environment." {
		t.Fatalf("expected first persisted round to capture the denial output, got %+v", result.Rounds[0])
	}
	if len(plannerInputs) != 3 {
		t.Fatalf("expected three planner rounds, got %+v", plannerInputs)
	}
	if !strings.Contains(plannerInputs[1], "能力提醒：") {
		t.Fatalf("expected second planner input to include capability reminder, got %q", plannerInputs[1])
	}
	if countEventType(result.Events, "loop.round.completed") != 3 {
		t.Fatalf("expected one completed event per persisted planner round, got %+v", result.Events)
	}
	if !strings.Contains(plannerInputs[1], "当前这轮已经开放下列工具能力。") {
		t.Fatalf("expected second planner input to restate tool availability, got %q", plannerInputs[1])
	}
	if !strings.Contains(plannerInputs[1], "按用户要求的语言给出简洁答复；若用户未指定语言，默认中文。") {
		t.Fatalf("expected second planner input to preserve language contract, got %q", plannerInputs[1])
	}
	if strings.Contains(plannerInputs[1], "简洁中文答复") {
		t.Fatalf("expected second planner input to avoid forcing Chinese replies, got %q", plannerInputs[1])
	}
	if countRetryReason(result.Events, "capability_reminder") != 1 {
		t.Fatalf("expected exactly one capability reminder retry event, got %+v", result.Events)
	}
}

func TestRunDoesNotRepeatCapabilityReminderAfterSecondDenial(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	plannerInputs := []string{}
	executeCalls := 0
	request.GenerateToolCalls = func(_ context.Context, req model.ToolCallRequest) (model.ToolCallResult, error) {
		plannerInputs = append(plannerInputs, req.Input)
		switch len(plannerInputs) {
		case 1:
			return model.ToolCallResult{
				RequestID:  "req_capability_retry_stop_1",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I cannot access workspace files in this environment.",
			}, nil
		default:
			return model.ToolCallResult{
				RequestID:  "req_capability_retry_stop_2",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I still cannot access workspace files in this environment.",
			}, nil
		}
	}
	request.ExecuteTool = func(_ context.Context, call model.ToolInvocation, round int) (string, tools.ToolCallRecord) {
		executeCalls++
		return "unexpected tool execution", tools.ToolCallRecord{
			ToolCallID: "tool_call_capability_retry_stop",
			TaskID:     request.TaskID,
			RunID:      request.RunID,
			StepID:     "step_loop_02",
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
		t.Fatal("expected request to be handled")
	}
	if result.OutputText != "I still cannot access workspace files in this environment." {
		t.Fatalf("unexpected output text: %+v", result)
	}
	if executeCalls != 0 {
		t.Fatalf("expected no tool execution after repeated denial, got %d", executeCalls)
	}
	if len(result.Rounds) != 2 {
		t.Fatalf("expected both denial rounds to be persisted, got %+v", result.Rounds)
	}
	if len(plannerInputs) != 2 {
		t.Fatalf("expected capability reminder flow to stop after one retry, got %+v", plannerInputs)
	}
	if !strings.Contains(plannerInputs[1], "能力提醒：") {
		t.Fatalf("expected second planner input to include capability reminder, got %q", plannerInputs[1])
	}
	if countEventType(result.Events, "loop.round.completed") != 2 {
		t.Fatalf("expected one completed event per denial round, got %+v", result.Events)
	}
	if countRetryReason(result.Events, "capability_reminder") != 1 {
		t.Fatalf("expected exactly one capability reminder retry event, got %+v", result.Events)
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
			RequestID:  "req_tool_retry",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "I will inspect the retry file first.",
			ToolCalls:  []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/retry.txt"}}},
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
			RequestID:  "req_tool_failure",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "I will inspect the failing file first.",
			ToolCalls:  []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/fail.txt"}}},
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
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected fallback output after max-turn tool failure, got %+v", result)
	}
}

func TestRunStopsAfterRepeatedToolChoices(t *testing.T) {
	runtime := NewRuntime()
	request := testRuntimeRequest()
	request.RepeatedToolBudget = 1
	request.GenerateToolCalls = func(_ context.Context, _ model.ToolCallRequest) (model.ToolCallResult, error) {
		return model.ToolCallResult{
			RequestID:  "req_dead_loop",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "I will list the notes directory first.",
			ToolCalls:  []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
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
	if result.OutputText != request.FallbackOutput {
		t.Fatalf("expected fallback output after repeated tool choices, got %+v", result)
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

func countRetryReason(events []LifecycleEvent, reason string) int {
	count := 0
	for _, event := range events {
		if event.Type != "loop.retrying" {
			continue
		}
		if value, ok := event.Payload["reason"].(string); ok && value == reason {
			count++
		}
	}
	return count
}
