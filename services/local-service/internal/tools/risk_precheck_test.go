package tools

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

type riskPlatformStub struct {
	workspacePath string
}

func (s riskPlatformStub) Join(elem ...string) string {
	return strings.Join(elem, "/")
}

func (s riskPlatformStub) Abs(path string) (string, error) {
	if strings.HasPrefix(path, "/") {
		return path, nil
	}
	return s.workspacePath + "/" + path, nil
}

func (s riskPlatformStub) EnsureWithinWorkspace(path string) (string, error) {
	if path == s.workspacePath || strings.HasPrefix(path, s.workspacePath+"/") {
		return path, nil
	}
	return "", errors.New("outside workspace")
}

func (s riskPlatformStub) Stat(path string) (fs.FileInfo, error) {
	return nil, fs.ErrNotExist
}

func (s riskPlatformStub) ReadFile(path string) ([]byte, error) {
	return nil, nil
}

func (s riskPlatformStub) WriteFile(path string, content []byte) error {
	return nil
}

func TestDefaultRiskPrecheckerReadFileLowRisk(t *testing.T) {
	prechecker := DefaultRiskPrechecker{}
	result, err := prechecker.Precheck(context.Background(), RiskPrecheckInput{
		Metadata: ToolMetadata{Name: "read_file", DisplayName: "Read", Source: ToolSourceBuiltin},
		ToolName: "read_file",
		Input:    map[string]any{"path": "demo.txt"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelGreen || result.Deny || result.ApprovalRequired || result.CheckpointRequired {
		t.Fatalf("unexpected precheck result: %+v", result)
	}
}

func TestDefaultRiskPrecheckerWriteFileInsideWorkspace(t *testing.T) {
	prechecker := DefaultRiskPrechecker{}
	within := true
	result, err := prechecker.Precheck(context.Background(), RiskPrecheckInput{
		Metadata: ToolMetadata{Name: "write_file", DisplayName: "Write", Source: ToolSourceBuiltin},
		ToolName: "write_file",
		Input:    map[string]any{"path": "report.txt"},
		Workspace: WorkspaceBoundaryInfo{
			WorkspacePath: "/workspace",
			TargetPath:    "/workspace/report.txt",
			Within:        &within,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelYellow || result.Deny || result.ApprovalRequired || !result.CheckpointRequired {
		t.Fatalf("unexpected precheck result: %+v", result)
	}
}

func TestDefaultRiskPrecheckerWriteFileOutsideWorkspaceDenied(t *testing.T) {
	prechecker := DefaultRiskPrechecker{}
	within := false
	result, err := prechecker.Precheck(context.Background(), RiskPrecheckInput{
		Metadata: ToolMetadata{Name: "write_file", DisplayName: "Write", Source: ToolSourceBuiltin},
		ToolName: "write_file",
		Input:    map[string]any{"path": "../secret.txt"},
		Workspace: WorkspaceBoundaryInfo{
			WorkspacePath: "/workspace",
			TargetPath:    "/secret.txt",
			Within:        &within,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelRed || !result.Deny || result.DenyReason == "" {
		t.Fatalf("unexpected precheck result: %+v", result)
	}
}

func TestDefaultRiskPrecheckerExecCommandHighRisk(t *testing.T) {
	prechecker := DefaultRiskPrechecker{}
	result, err := prechecker.Precheck(context.Background(), RiskPrecheckInput{
		Metadata: ToolMetadata{Name: "exec_command", DisplayName: "Exec", Source: ToolSourceBuiltin},
		ToolName: "exec_command",
		Input:    map[string]any{"command": "rm -rf /tmp/demo"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelRed || !result.Deny {
		t.Fatalf("unexpected precheck result: %+v", result)
	}
}

func TestToolExecutorBlocksDeniedPrecheck(t *testing.T) {
	sink := &InMemoryToolCallSink{}
	tool := &stubTool{
		meta: ToolMetadata{Name: "write_file", DisplayName: "Write", Source: ToolSourceBuiltin, TimeoutSec: 5},
	}
	exec := newExecutorForTest(tool, sink)

	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	result, err := exec.ExecuteToolWithContext(context.Background(), execCtx, "write_file", map[string]any{"path": "/outside/report.txt"})
	if !errors.Is(err, ErrWorkspaceBoundaryDenied) {
		t.Fatalf("expected workspace boundary error, got %v", err)
	}
	if tool.executeCalled {
		t.Fatal("expected tool execution to be skipped")
	}
	if result.Precheck == nil || !result.Precheck.Deny {
		t.Fatalf("expected denied precheck, got %+v", result.Precheck)
	}
	if result.Error == nil || result.Error.Code != ToolErrorCodeWorkspaceDenied {
		t.Fatalf("expected workspace denied code, got %+v", result.Error)
	}
	if result.ToolCall.Status != ToolCallStatusFailed {
		t.Fatalf("expected failed tool call, got %q", result.ToolCall.Status)
	}
	records := sink.Snapshot()
	if len(records) != 2 {
		t.Fatalf("expected 2 recorded states, got %d", len(records))
	}
	if records[1].ErrorCode == nil || *records[1].ErrorCode != ToolErrorCodeWorkspaceDenied {
		t.Fatalf("expected workspace denied error code, got %+v", records[1].ErrorCode)
	}
}

func TestToolExecutorBlocksApprovalRequiredPrecheck(t *testing.T) {
	sink := &InMemoryToolCallSink{}
	tool := &stubTool{
		meta: ToolMetadata{Name: "exec_command", DisplayName: "Exec", Source: ToolSourceBuiltin, TimeoutSec: 5},
	}
	exec := newExecutorForTest(tool, sink)

	result, err := exec.ExecuteToolWithContext(context.Background(), &ToolExecuteContext{}, "exec_command", map[string]any{"command": "powershell Get-Process"})
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected approval required error, got %v", err)
	}
	if tool.executeCalled {
		t.Fatal("expected tool execution to be skipped")
	}
	if result.Precheck == nil || !result.Precheck.ApprovalRequired {
		t.Fatalf("expected approval-required precheck, got %+v", result.Precheck)
	}
	if result.Error == nil || result.Error.Code != ToolErrorCodeApprovalRequired {
		t.Fatalf("expected approval required code, got %+v", result.Error)
	}
}
