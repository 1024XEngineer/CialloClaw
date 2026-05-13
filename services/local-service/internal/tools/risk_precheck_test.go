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

func (s riskPlatformStub) ReadDir(path string) ([]fs.DirEntry, error) {
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
	if result.Reason != "normal" {
		t.Fatalf("expected normal reason, got %+v", result)
	}
}

func TestDefaultRiskPrecheckerWriteFileInsideWorkspaceCreateFlow(t *testing.T) {
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
	if result.RiskLevel != RiskLevelGreen || result.Deny || result.ApprovalRequired || result.CheckpointRequired {
		t.Fatalf("unexpected precheck result: %+v", result)
	}
	if files := result.ImpactScope["files"].([]string); len(files) != 1 || files[0] != "/workspace/report.txt" {
		t.Fatalf("expected write_file impact scope to include target file, got %+v", result.ImpactScope)
	}
}

func TestDefaultRiskPrecheckerWriteFileOverwriteRequiresApproval(t *testing.T) {
	prechecker := DefaultRiskPrechecker{}
	within := true
	exists := true
	result, err := prechecker.Precheck(context.Background(), RiskPrecheckInput{
		Metadata: ToolMetadata{Name: "write_file", DisplayName: "Write", Source: ToolSourceBuiltin},
		ToolName: "write_file",
		Input:    map[string]any{"path": "report.txt"},
		Workspace: WorkspaceBoundaryInfo{
			WorkspacePath: "/workspace",
			TargetPath:    "/workspace/report.txt",
			Within:        &within,
			Exists:        &exists,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelYellow || result.Deny || !result.ApprovalRequired || !result.CheckpointRequired {
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
		Input:    map[string]any{"command": "rm -rf /tmp/demo", "working_dir": "/workspace"},
		Workspace: WorkspaceBoundaryInfo{
			WorkspacePath: "/workspace",
			TargetPath:    "/workspace",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelRed || !result.Deny {
		t.Fatalf("unexpected precheck result: %+v", result)
	}
}

func TestDefaultRiskPrecheckerExecCommandRequiresApprovalAndImpactScope(t *testing.T) {
	prechecker := DefaultRiskPrechecker{}
	within := true
	result, err := prechecker.Precheck(context.Background(), RiskPrecheckInput{
		Metadata: ToolMetadata{Name: "exec_command", DisplayName: "Exec", Source: ToolSourceBuiltin},
		ToolName: "exec_command",
		Input:    map[string]any{"command": "git status", "working_dir": "notes"},
		Workspace: WorkspaceBoundaryInfo{
			WorkspacePath: "/workspace",
			TargetPath:    "/workspace/notes",
			Within:        &within,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelYellow || !result.ApprovalRequired || !result.CheckpointRequired {
		t.Fatalf("unexpected precheck result: %+v", result)
	}
	files := result.ImpactScope["files"].([]string)
	if len(files) != 1 || files[0] != "/workspace/notes" {
		t.Fatalf("expected exec_command impact scope to include working dir, got %+v", result.ImpactScope)
	}
}

func TestBuildRiskPrecheckInputPageReadUsesURLWithoutWorkspaceBoundary(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "page_read", DisplayName: "Page Read", Source: ToolSourceSidecar},
		"page_read",
		execCtx,
		map[string]any{"url": "https://93.184.216.34/page"},
	)
	if input.Workspace.TargetPath != "https://93.184.216.34/page" {
		t.Fatalf("expected URL target path, got %+v", input.Workspace)
	}
	if input.Workspace.Within != nil {
		t.Fatalf("expected page_read not to perform workspace boundary check, got %+v", input.Workspace)
	}

	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	webpages := result.ImpactScope["webpages"].([]string)
	if len(webpages) != 1 || webpages[0] != "https://93.184.216.34/page" {
		t.Fatalf("expected webpage impact scope, got %+v", result.ImpactScope)
	}
	if result.RiskLevel != RiskLevelGreen || result.ApprovalRequired {
		t.Fatalf("expected page_read to stay low risk, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputBrowserSnapshotCapturesAppScope(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_snapshot", DisplayName: "Browser Snapshot", Source: ToolSourceSidecar},
		"browser_snapshot",
		execCtx,
		map[string]any{"attach": map[string]any{"browser_kind": "chrome", "target": map[string]any{"url": "https://example.com/page"}}},
	)
	if input.Workspace.TargetPath != "https://example.com/page" {
		t.Fatalf("expected browser snapshot target to use attached page URL, got %+v", input.Workspace)
	}
	if input.Workspace.Within != nil {
		t.Fatalf("expected browser snapshot to skip workspace boundary checks, got %+v", input.Workspace)
	}
	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelGreen || result.ApprovalRequired {
		t.Fatalf("expected browser snapshot to stay low risk, got %+v", result)
	}
	apps := result.ImpactScope["apps"].([]string)
	if len(apps) != 1 || apps[0] != "chrome" {
		t.Fatalf("expected browser snapshot app scope, got %+v", result.ImpactScope)
	}
}

func TestBuildRiskPrecheckInputPageReadRequiresApprovalForLoopbackTarget(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "page_read", DisplayName: "Page Read", Source: ToolSourceSidecar},
		"page_read",
		execCtx,
		map[string]any{"url": "http://127.0.0.1:8080/admin"},
	)
	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelYellow || !result.ApprovalRequired {
		t.Fatalf("expected loopback page_read to require approval, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputBrowserNavigateUsesDestinationURL(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_navigate", DisplayName: "Browser Navigate", Source: ToolSourceSidecar},
		"browser_navigate",
		execCtx,
		map[string]any{
			"url":    "https://example.com/next",
			"attach": map[string]any{"browser_kind": "edge", "target": map[string]any{"url": "https://example.com/current"}},
		},
	)
	if input.Workspace.TargetPath != "https://example.com/next" {
		t.Fatalf("expected browser navigate target to prefer destination URL, got %+v", input.Workspace)
	}
	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelGreen || result.ApprovalRequired {
		t.Fatalf("expected browser navigate public target to stay low risk, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputBrowserNavigateRequiresApprovalForLoopbackTarget(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_navigate", DisplayName: "Browser Navigate", Source: ToolSourceSidecar},
		"browser_navigate",
		execCtx,
		map[string]any{
			"url":    "http://127.0.0.1:8080/admin",
			"attach": map[string]any{"browser_kind": "edge", "target": map[string]any{"url": "https://example.com/current"}},
		},
	)
	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelYellow || !result.ApprovalRequired {
		t.Fatalf("expected loopback browser navigate to require approval, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputBrowserTabsListStaysLowRisk(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_tabs_list", DisplayName: "Browser Tabs List", Source: ToolSourceSidecar},
		"browser_tabs_list",
		execCtx,
		map[string]any{"attach": map[string]any{"browser_kind": "chrome"}},
	)
	if input.Workspace.TargetPath != "" {
		t.Fatalf("expected browser_tabs_list without explicit target to keep empty governance target, got %+v", input.Workspace)
	}
	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelGreen || result.ApprovalRequired {
		t.Fatalf("expected browser_tabs_list to stay low risk, got %+v", result)
	}
	apps := result.ImpactScope["apps"].([]string)
	if len(apps) != 1 || apps[0] != "chrome" {
		t.Fatalf("expected browser_tabs_list app scope, got %+v", result.ImpactScope)
	}
}

func TestBuildRiskPrecheckInputBrowserTabsListEndpointOverrideRequiresApproval(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_tabs_list", DisplayName: "Browser Tabs List", Source: ToolSourceSidecar},
		"browser_tabs_list",
		execCtx,
		map[string]any{"attach": map[string]any{"browser_kind": "chrome", "endpoint_url": "http://127.0.0.1:9333"}},
	)
	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelYellow || !result.ApprovalRequired {
		t.Fatalf("expected browser_tabs_list endpoint override to require approval, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputBrowserTabFocusStaysLowRisk(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_tab_focus", DisplayName: "Browser Tab Focus", Source: ToolSourceSidecar},
		"browser_tab_focus",
		execCtx,
		map[string]any{"attach": map[string]any{"browser_kind": "chrome", "target": map[string]any{"page_index": 2}}},
	)
	if input.Workspace.TargetPath != "browser_tab:2" {
		t.Fatalf("expected browser_tab_focus target to use stable attach selector, got %+v", input.Workspace)
	}
	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelGreen || result.ApprovalRequired {
		t.Fatalf("expected browser_tab_focus to stay low risk, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputBrowserInteractUsesAttachTarget(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_interact", DisplayName: "Browser Interact", Source: ToolSourceSidecar},
		"browser_interact",
		execCtx,
		map[string]any{
			"attach":  map[string]any{"target": map[string]any{"page_index": 2}},
			"actions": []any{map[string]any{"type": "click", "selector": "#continue"}},
		},
	)
	if input.Workspace.TargetPath != "browser_tab:2" {
		t.Fatalf("expected browser interact target to use stable attach selector, got %+v", input.Workspace)
	}
	if input.Workspace.Within != nil {
		t.Fatalf("expected browser interact to skip workspace boundary checks, got %+v", input.Workspace)
	}

	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	webpages := result.ImpactScope["webpages"].([]string)
	if len(webpages) != 1 || webpages[0] != "browser_tab:2" {
		t.Fatalf("expected browser interact impact scope to include stable attach target, got %+v", result.ImpactScope)
	}
	if result.RiskLevel != RiskLevelYellow || !result.ApprovalRequired {
		t.Fatalf("expected browser interact to require approval, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputBrowserInteractRejectsTitleOnlyGovernanceTarget(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "browser_interact", DisplayName: "Browser Interact", Source: ToolSourceSidecar},
		"browser_interact",
		execCtx,
		map[string]any{
			"attach":  map[string]any{"target": map[string]any{"title_contains": "Example Docs"}},
			"actions": []any{map[string]any{"type": "click", "selector": "#continue"}},
		},
	)
	if input.Workspace.TargetPath != "" {
		t.Fatalf("expected title-only browser interact target to stay empty, got %+v", input.Workspace)
	}
}

func TestBuildRiskPrecheckInputUsesMediaOutputTarget(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "transcode_media", DisplayName: "Transcode", Source: ToolSourceWorker},
		"transcode_media",
		execCtx,
		map[string]any{"path": "clips/demo.mov", "output_path": "/workspace/exports/demo.mp4"},
	)
	if input.Workspace.TargetPath != "/workspace/exports/demo.mp4" {
		t.Fatalf("expected media precheck target to use output_path, got %+v", input.Workspace)
	}
	if input.Workspace.Within == nil || !*input.Workspace.Within {
		t.Fatalf("expected media output path to stay inside workspace, got %+v", input.Workspace)
	}

	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	files := result.ImpactScope["files"].([]string)
	if len(files) != 1 || files[0] != "/workspace/exports/demo.mp4" {
		t.Fatalf("expected media impact scope to include output target, got %+v", result.ImpactScope)
	}
}

func TestBuildRiskPrecheckInputDeniesMediaOutputOutsideWorkspace(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/workspace"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "extract_frames", DisplayName: "Extract Frames", Source: ToolSourceWorker},
		"extract_frames",
		execCtx,
		map[string]any{"path": "clips/demo.mov", "output_dir": "/outside/frames"},
	)
	if input.Workspace.TargetPath != "/outside/frames" {
		t.Fatalf("expected frame extraction target to use output_dir, got %+v", input.Workspace)
	}
	if input.Workspace.Within == nil || *input.Workspace.Within {
		t.Fatalf("expected out-of-workspace frame extraction to be detected, got %+v", input.Workspace)
	}

	result, err := DefaultRiskPrechecker{}.Precheck(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != RiskLevelRed || !result.Deny {
		t.Fatalf("expected out-of-workspace media write to be denied, got %+v", result)
	}
}

func TestBuildRiskPrecheckInputWriteFileMarksToolPolicyPathOutsideWorkspace(t *testing.T) {
	execCtx := &ToolExecuteContext{
		WorkspacePath: "/workspace",
		Platform:      riskPlatformStub{workspacePath: "/tmp"},
	}
	input := BuildRiskPrecheckInput(
		ToolMetadata{Name: "write_file", DisplayName: "Write File", Source: ToolSourceBuiltin},
		"write_file",
		execCtx,
		map[string]any{"path": "/tmp/Desktop/report.md"},
	)
	if input.Workspace.TargetPath != "/tmp/Desktop/report.md" {
		t.Fatalf("expected write_file target path to preserve resolved tool path, got %+v", input.Workspace)
	}
	if input.Workspace.Within == nil || *input.Workspace.Within {
		t.Fatalf("expected write_file target outside workspace to be detected even when platform allows it, got %+v", input.Workspace)
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
