package tools

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubTool struct {
	meta          ToolMetadata
	validateErr   error
	executeResult *ToolResult
	executeErr    error
	executeDelay  time.Duration
	executeCalled bool
	lastExecCtx   *ToolExecuteContext
}

func (s *stubTool) Metadata() ToolMetadata {
	return s.meta
}

func (s *stubTool) Validate(_ map[string]any) error {
	return s.validateErr
}

func (s *stubTool) Execute(ctx context.Context, execCtx *ToolExecuteContext, _ map[string]any) (*ToolResult, error) {
	s.executeCalled = true
	s.lastExecCtx = execCtx
	if s.executeDelay > 0 {
		select {
		case <-time.After(s.executeDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.executeResult, s.executeErr
}

func newExecutorForTest(tool Tool, sink ToolCallSink) *ToolExecutor {
	reg := NewRegistry()
	if tool != nil {
		if err := reg.Register(tool); err != nil {
			panic(err)
		}
	}
	return NewToolExecutor(reg, WithToolCallRecorder(NewToolCallRecorder(sink)))
}

func TestToolExecutorNormalExecution(t *testing.T) {
	sink := &InMemoryToolCallSink{}
	tool := &stubTool{
		meta:          ToolMetadata{Name: "ok_tool", DisplayName: "OK", Source: ToolSourceBuiltin, TimeoutSec: 5},
		executeResult: &ToolResult{Output: map[string]any{"path": "demo.txt", "content": map[string]any{"hidden": true}}},
	}
	exec := newExecutorForTest(tool, sink)

	execCtx := &ToolExecuteContext{TaskID: "task_1", RunID: "run_1", StepID: "step_1"}
	result, err := exec.ExecuteToolWithContext(context.Background(), execCtx, "ok_tool", map[string]any{"path": "demo.txt", "blob": []any{1, 2, 3}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata.Name != "ok_tool" {
		t.Fatalf("expected metadata name, got %+v", result.Metadata)
	}
	if result.Precheck == nil || result.Precheck.RiskLevel != RiskLevelGreen {
		t.Fatalf("expected green precheck, got %+v", result.Precheck)
	}
	if result.ToolCall.Status != ToolCallStatusSucceeded {
		t.Fatalf("expected succeeded tool call, got %q", result.ToolCall.Status)
	}
	if got := result.SummaryOutput["content"].(map[string]any)["type"]; got != "object" {
		t.Fatalf("expected summarized output, got %+v", result.SummaryOutput)
	}
	if tool.lastExecCtx == nil || tool.lastExecCtx.Timeout != 5*time.Second || tool.lastExecCtx.Cancel == nil {
		t.Fatalf("expected execution context timeout and cancel to be set, got %+v", tool.lastExecCtx)
	}
	records := sink.Snapshot()
	if len(records) != 2 {
		t.Fatalf("expected 2 recorded states, got %d", len(records))
	}
	if records[0].Status != ToolCallStatusStarted || records[1].Status != ToolCallStatusSucceeded {
		t.Fatalf("unexpected status flow: %+v", records)
	}
	if got := records[0].Input["blob"].(map[string]any)["type"]; got != "array" {
		t.Fatalf("expected summarized input, got %+v", records[0].Input)
	}
	if records[1].ErrorCode != nil {
		t.Fatalf("expected nil error code, got %+v", records[1].ErrorCode)
	}
}

func TestToolExecutorValidateFailure(t *testing.T) {
	sink := &InMemoryToolCallSink{}
	exec := newExecutorForTest(&stubTool{
		meta:        ToolMetadata{Name: "bad_input", DisplayName: "Bad Input", Source: ToolSourceBuiltin},
		validateErr: errors.New("missing path"),
	}, sink)

	result, err := exec.ExecuteToolWithContext(context.Background(), &ToolExecuteContext{}, "bad_input", map[string]any{"path": ""})
	if !errors.Is(err, ErrToolValidationFailed) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if result.ToolCall.Status != ToolCallStatusFailed {
		t.Fatalf("expected failed status, got %q", result.ToolCall.Status)
	}
	if result.ToolCall.ErrorCode != nil {
		t.Fatalf("expected nil mapped error code, got %+v", result.ToolCall.ErrorCode)
	}
}

func TestToolExecutorToolNotFound(t *testing.T) {
	sink := &InMemoryToolCallSink{}
	exec := NewToolExecutor(NewRegistry(), WithToolCallRecorder(NewToolCallRecorder(sink)))

	result, err := exec.ExecuteTool(context.Background(), "missing_tool", map[string]any{"foo": "bar"})
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("expected tool not found, got %v", err)
	}
	if result.ToolCall.Status != ToolCallStatusFailed {
		t.Fatalf("expected failed tool call, got %q", result.ToolCall.Status)
	}
	if result.ToolCall.ErrorCode == nil || *result.ToolCall.ErrorCode != ToolErrorCodeNotFound {
		t.Fatalf("expected tool not found code, got %+v", result.ToolCall.ErrorCode)
	}
}

func TestToolExecutorExecuteError(t *testing.T) {
	sink := &InMemoryToolCallSink{}
	exec := newExecutorForTest(&stubTool{
		meta:       ToolMetadata{Name: "fail_tool", DisplayName: "Fail", Source: ToolSourceBuiltin, TimeoutSec: 5},
		executeErr: errors.New("boom"),
	}, sink)

	result, err := exec.ExecuteTool(context.Background(), "fail_tool", map[string]any{"foo": "bar"})
	if !errors.Is(err, ErrToolExecutionFailed) {
		t.Fatalf("expected execution failed, got %v", err)
	}
	if result.Error == nil || result.Error.Code != ToolErrorCodeExecutionFailed {
		t.Fatalf("expected execution failed code, got %+v", result.Error)
	}
	if result.ToolCall.Status != ToolCallStatusFailed {
		t.Fatalf("expected failed tool call, got %q", result.ToolCall.Status)
	}
}

func TestToolExecutorTimeout(t *testing.T) {
	sink := &InMemoryToolCallSink{}
	exec := newExecutorForTest(&stubTool{
		meta:         ToolMetadata{Name: "slow_tool", DisplayName: "Slow", Source: ToolSourceBuiltin, TimeoutSec: 1},
		executeDelay: 1500 * time.Millisecond,
	}, sink)

	result, err := exec.ExecuteTool(context.Background(), "slow_tool", map[string]any{"foo": "bar"})
	if !errors.Is(err, ErrToolExecutionTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if result.Error == nil || result.Error.Code != ToolErrorCodeTimeout {
		t.Fatalf("expected timeout code, got %+v", result.Error)
	}
	if result.ToolCall.Status != ToolCallStatusTimeout {
		t.Fatalf("expected timeout status, got %q", result.ToolCall.Status)
	}
	if result.ToolCall.ErrorCode == nil || *result.ToolCall.ErrorCode != ToolErrorCodeTimeout {
		t.Fatalf("expected timeout tool call code, got %+v", result.ToolCall.ErrorCode)
	}
}

func TestToolExecutorLegacyExecuteCompatibility(t *testing.T) {
	exec := newExecutorForTest(&stubTool{
		meta:          ToolMetadata{Name: "legacy_tool", DisplayName: "Legacy", Source: ToolSourceBuiltin},
		executeResult: &ToolResult{Output: map[string]any{"ok": true}},
	}, nil)

	result, err := exec.Execute(context.Background(), "legacy_tool", nil, map[string]any{"ok": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ToolName != "legacy_tool" || result.Duration <= 0 {
		t.Fatalf("unexpected legacy result: %+v", result)
	}
}

func TestToolExecutorResolveTool(t *testing.T) {
	tool := &stubTool{meta: ToolMetadata{Name: "resolve_me", DisplayName: "Resolve", Source: ToolSourceBuiltin}}
	exec := newExecutorForTest(tool, nil)

	resolved, err := exec.ResolveTool("resolve_me")
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if resolved.Metadata().Name != "resolve_me" {
		t.Fatalf("unexpected resolved tool: %+v", resolved.Metadata())
	}
}

func TestToolExecuteContextFields(t *testing.T) {
	ctx := &ToolExecuteContext{
		TaskID:        "task_001",
		RunID:         "run_001",
		StepID:        "step_001",
		TraceID:       "trace_001",
		WorkspacePath: "D:/CialloClawWorkspace",
		Timeout:       10 * time.Second,
	}
	if ctx.TaskID != "task_001" || ctx.RunID != "run_001" {
		t.Fatalf("unexpected context fields: %+v", ctx)
	}
}

func TestToolExecutorPrecheckToolWithContextReturnsAssessment(t *testing.T) {
	exec := newExecutorForTest(&stubTool{
		meta: ToolMetadata{Name: "write_file", DisplayName: "Write File", Source: ToolSourceBuiltin},
	}, nil)

	resultMeta, precheck, err := exec.PrecheckToolWithContext(context.Background(), &ToolExecuteContext{
		WorkspacePath: "D:/workspace",
	}, "write_file", map[string]any{"path": "notes/a.md"})
	if err != nil {
		t.Fatalf("PrecheckToolWithContext returned error: %v", err)
	}
	if resultMeta.Name != "write_file" {
		t.Fatalf("expected metadata for write_file, got %+v", resultMeta)
	}
	if precheck == nil {
		t.Fatal("expected precheck result")
	}
	if precheck.RiskLevel == "" {
		t.Fatalf("expected risk level to be populated, got %+v", precheck)
	}
}

func TestApprovalBypassAllowedUsesStoredOperationAndTarget(t *testing.T) {
	execCtx := &ToolExecuteContext{
		ApprovalGranted:      true,
		ApprovedOperation:    "write_file",
		ApprovedTargetObject: "workspace/notes/a.md",
	}
	precheckInput := RiskPrecheckInput{
		Workspace: WorkspaceBoundaryInfo{
			WorkspacePath: "D:/repo/workspace",
			TargetPath:    "D:/repo/workspace/notes/a.md",
		},
	}
	if !approvalBypassAllowed(execCtx, "write_file", precheckInput) {
		t.Fatal("expected approval bypass to allow normalized stored target")
	}
	if approvalBypassAllowed(execCtx, "exec_command", precheckInput) {
		t.Fatal("expected approval bypass to reject mismatched operation")
	}

	execCtx.ApprovedOperation = ""
	execCtx.ApprovedTargetObject = ""
	if !approvalBypassAllowed(execCtx, "write_file", precheckInput) {
		t.Fatal("expected blank approved target to allow granted approval")
	}
}

func TestNormalizeApprovalTargetHandlesWorkspaceForms(t *testing.T) {
	workspaceRoot := "D:/repo/workspace"
	inputs := []string{
		"workspace/notes/a.md",
		"./notes/a.md",
		"D:/repo/workspace/notes/a.md",
		"notes/a.md",
	}
	for _, input := range inputs {
		if got := normalizeApprovalTarget(input, workspaceRoot); got != "notes/a.md" {
			t.Fatalf("expected normalized path notes/a.md for %q, got %q", input, got)
		}
	}
	if got := normalizeApprovalTarget(workspaceRoot, workspaceRoot); got != "." {
		t.Fatalf("expected workspace root to normalize to '.', got %q", got)
	}
}
