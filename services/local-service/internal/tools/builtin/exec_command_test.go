package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubExecutionCapability struct {
	lastCommand    string
	lastArgs       []string
	lastWorkingDir string
	result         tools.CommandExecutionResult
	err            error
}

func (s *stubExecutionCapability) RunCommand(_ context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error) {
	s.lastCommand = command
	s.lastArgs = append([]string(nil), args...)
	s.lastWorkingDir = workingDir
	if s.err != nil {
		return tools.CommandExecutionResult{}, s.err
	}
	return s.result, nil
}

func TestExecCommandToolExecuteSuccess(t *testing.T) {
	execution := &stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "line1\nline2", Stderr: "", ExitCode: 0}}
	tool := NewExecCommandTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution}, map[string]any{
		"command":     "echo",
		"args":        []any{"hello", "world"},
		"working_dir": "/workspace",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if execution.lastCommand != "echo" || execution.lastWorkingDir != "/workspace" {
		t.Fatalf("unexpected execution inputs: %+v", execution)
	}
	if result.RawOutput["exit_code"] != 0 {
		t.Fatalf("unexpected raw output: %+v", result.RawOutput)
	}
	if result.SummaryOutput["stdout_preview"] != "line1\nline2" {
		t.Fatalf("unexpected summary output: %+v", result.SummaryOutput)
	}
}

func TestExecCommandToolValidateFailure(t *testing.T) {
	tool := NewExecCommandTool()

	if err := tool.Validate(map[string]any{"command": ""}); err == nil {
		t.Fatal("expected validate error")
	}
}

func TestExecCommandToolReturnsAdapterError(t *testing.T) {
	execution := &stubExecutionCapability{err: errors.New("runner unavailable")}
	tool := NewExecCommandTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution}, map[string]any{
		"command": "echo",
	})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected ErrToolExecutionFailed, got %v", err)
	}
}

func TestExecCommandToolRequiresExecutionAdapter(t *testing.T) {
	tool := NewExecCommandTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{}, map[string]any{"command": "echo"})
	if !errors.Is(err, tools.ErrCapabilityDenied) {
		t.Fatalf("expected ErrCapabilityDenied, got %v", err)
	}
}
