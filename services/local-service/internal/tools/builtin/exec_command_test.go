package builtin

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
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

type stubExecCommandPlatform struct {
	workspaceRoot string
	outOfScope    map[string]bool
}

func newStubExecCommandPlatform(workspaceRoot string) *stubExecCommandPlatform {
	return &stubExecCommandPlatform{workspaceRoot: workspaceRoot, outOfScope: make(map[string]bool)}
}

func (s *stubExecCommandPlatform) Join(elem ...string) string { return filepath.Join(elem...) }

func (s *stubExecCommandPlatform) Abs(path string) (string, error) {
	if isStubAbsolutePath(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(s.workspaceRoot, path), nil
}

func (s *stubExecCommandPlatform) EnsureWithinWorkspace(path string) (string, error) {
	clean := filepath.Clean(path)
	if s.outOfScope[clean] {
		return "", errors.New("outside workspace")
	}
	if isStubAbsolutePath(clean) {
		return clean, nil
	}
	return filepath.Join(s.workspaceRoot, clean), nil
}

func (s *stubExecCommandPlatform) ReadDir(path string) ([]fs.DirEntry, error)  { return nil, nil }
func (s *stubExecCommandPlatform) ReadFile(path string) ([]byte, error)        { return nil, fs.ErrNotExist }
func (s *stubExecCommandPlatform) WriteFile(path string, content []byte) error { return nil }
func (s *stubExecCommandPlatform) Stat(path string) (fs.FileInfo, error)       { return nil, fs.ErrNotExist }

func TestExecCommandToolExecuteSuccess(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	execution := &stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "line1\nline2", Stderr: "", ExitCode: 0}}
	platform := newStubExecCommandPlatform(workspace)
	tool := NewExecCommandTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution, Platform: platform, WorkspacePath: workspace}, map[string]any{
		"command":     "echo",
		"args":        []any{"hello", "world"},
		"working_dir": "notes",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if execution.lastCommand != "echo" || execution.lastWorkingDir != filepath.Join(workspace, "notes") {
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
	workspace := filepath.Clean("D:/workspace")
	platform := newStubExecCommandPlatform(workspace)
	execution := &stubExecutionCapability{err: errors.New("runner unavailable")}
	tool := NewExecCommandTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution, Platform: platform, WorkspacePath: workspace}, map[string]any{
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

func TestExecCommandToolRejectsOutsideWorkspaceWorkingDir(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubExecCommandPlatform(workspace)
	outside := filepath.Clean("D:/outside")
	platform.outOfScope[outside] = true
	execution := &stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}}
	tool := NewExecCommandTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution, Platform: platform, WorkspacePath: workspace}, map[string]any{
		"command":     "echo",
		"working_dir": outside,
	})
	if !errors.Is(err, tools.ErrWorkspaceBoundaryDenied) {
		t.Fatalf("expected ErrWorkspaceBoundaryDenied, got %v", err)
	}
}

func TestExecCommandToolUsesWorkspacePathByDefault(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubExecCommandPlatform(workspace)
	execution := &stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}}
	tool := NewExecCommandTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution, Platform: platform, WorkspacePath: workspace}, map[string]any{
		"command": "echo",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if execution.lastWorkingDir != workspace {
		t.Fatalf("expected workspace path default, got %q", execution.lastWorkingDir)
	}
}

func TestExecCommandToolTruncatesLargeOutputs(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubExecCommandPlatform(workspace)
	largeStdout := strings.Repeat("a", commandOutputRawLimit+10)
	largeStderr := strings.Repeat("b", commandOutputRawLimit+5)
	execution := &stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: largeStdout, Stderr: largeStderr, ExitCode: 0}}
	tool := NewExecCommandTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution, Platform: platform, WorkspacePath: workspace}, map[string]any{
		"command": "echo",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := result.RawOutput["stdout"].(string); len(got) != commandOutputRawLimit {
		t.Fatalf("expected truncated stdout length %d, got %d", commandOutputRawLimit, len(got))
	}
	if got := result.RawOutput["stderr"].(string); len(got) != commandOutputRawLimit {
		t.Fatalf("expected truncated stderr length %d, got %d", commandOutputRawLimit, len(got))
	}
	if result.RawOutput["stdout_truncated"] != true || result.RawOutput["stderr_truncated"] != true {
		t.Fatalf("expected truncation flags, got %+v", result.RawOutput)
	}
	if result.RawOutput["stdout_bytes"] != len(largeStdout) || result.RawOutput["stderr_bytes"] != len(largeStderr) {
		t.Fatalf("expected original byte counts, got %+v", result.RawOutput)
	}
}

func TestExecCommandToolIncludesSandboxMetadata(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubExecCommandPlatform(workspace)
	execution := &stubExecutionCapability{result: tools.CommandExecutionResult{
		Stdout:           "ok",
		ExitCode:         0,
		ExecutionBackend: "docker_sandbox",
		SandboxContainer: "sandbox-test-1",
		SandboxImage:     "docker.io/library/golang:1.26-bookworm",
	}}
	tool := NewExecCommandTool()
	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{Execution: execution, Platform: platform, WorkspacePath: workspace}, map[string]any{
		"command": "go",
		"args":    []any{"test", "./..."},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["execution_backend"] != "docker_sandbox" {
		t.Fatalf("expected execution backend metadata, got %+v", result.RawOutput)
	}
	sandbox, ok := result.RawOutput["sandbox"].(map[string]any)
	if !ok {
		t.Fatalf("expected sandbox metadata map, got %+v", result.RawOutput)
	}
	if sandbox["container_name"] != "sandbox-test-1" || sandbox["image"] != "docker.io/library/golang:1.26-bookworm" {
		t.Fatalf("unexpected sandbox metadata: %+v", sandbox)
	}
	if result.SummaryOutput["execution_backend"] != "docker_sandbox" {
		t.Fatalf("expected summary output to carry backend metadata, got %+v", result.SummaryOutput)
	}
}
