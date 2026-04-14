package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

type stubModelClient struct {
	output string
	err    error
}

func (s stubModelClient) GenerateText(_ context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	if s.err != nil {
		return model.GenerateTextResponse{}, s.err
	}

	return model.GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  "req_test",
		Provider:   "openai_responses",
		ModelID:    "gpt-5.4",
		OutputText: s.output,
	}, nil
}

func newTestExecutionService(t *testing.T, output string) (*Service, string) {
	t.Helper()

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		t.Fatalf("register builtin tools: %v", err)
	}
	toolExecutor := tools.NewToolExecutor(toolRegistry)

	return NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		model.NewService(serviceconfig.ModelConfig{}, stubModelClient{output: output}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		plugin.NewService(),
	), workspaceRoot
}

func newTestExecutionServiceWithPlaywright(t *testing.T, output string, playwright tools.PlaywrightSidecarClient) (*Service, string) {
	t.Helper()

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		t.Fatalf("register builtin tools: %v", err)
	}
	if err := sidecarclient.RegisterPlaywrightTools(toolRegistry); err != nil {
		t.Fatalf("register playwright tools: %v", err)
	}
	toolExecutor := tools.NewToolExecutor(toolRegistry)

	return NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		playwright,
		model.NewService(serviceconfig.ModelConfig{}, stubModelClient{output: output}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		plugin.NewService(),
	), workspaceRoot
}

func registerBuiltinTools(t *testing.T) *tools.Registry {
	t.Helper()

	registry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(registry); err != nil {
		t.Fatalf("RegisterBuiltinTools returned error: %v", err)
	}
	return registry
}

func TestExecuteWorkspaceDocumentWritesFile(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "第一点\n第二点\n第三点")

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_001",
		RunID:        "run_001",
		Title:        "生成文档",
		Intent:       map[string]any{"name": "write_file", "arguments": map[string]any{"target_path": "notes/output.md"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请整理成文档"},
		DeliveryType: "workspace_document",
		ResultTitle:  "文件写入结果",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.ToolName != "write_file" {
		t.Fatalf("expected write_file tool, got %s", result.ToolName)
	}
	executionContext, ok := result.ToolInput["execution_context"].(map[string]any)
	if !ok {
		t.Fatalf("expected execution_context in tool input, got %+v", result.ToolInput)
	}
	if executionContext["intent_name"] != "write_file" {
		t.Fatalf("expected intent_name in execution_context, got %+v", executionContext)
	}
	if result.ToolInput["path"] != "notes/output.md" {
		t.Fatalf("expected tool input path to be preserved, got %+v", result.ToolInput)
	}
	if result.ToolInput["content"] == nil {
		t.Fatalf("expected tool input content to be preserved, got %+v", result.ToolInput)
	}
	if result.ToolOutput["summary_output"] == nil {
		t.Fatalf("expected write_file to flow through ToolExecutor summary output, got %+v", result.ToolOutput)
	}
	if result.ToolOutput["audit_record"] == nil {
		t.Fatalf("expected write_file tool output to include consumed audit record, got %+v", result.ToolOutput)
	}
	recoveryPoint, ok := result.ToolOutput["recovery_point"].(map[string]any)
	if !ok {
		t.Fatalf("expected create flow to emit recovery point metadata, got %+v", result.ToolOutput)
	}
	if objects := recoveryPoint["objects"].([]string); len(objects) != 1 || objects[0] != "workspace/notes/output.md" {
		t.Fatalf("expected create flow recovery point to target workspace/notes/output.md, got %+v", recoveryPoint)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("expected generate_text + write_file tool chain, got %d calls", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ToolName != "generate_text" || result.ToolCalls[1].ToolName != "write_file" {
		t.Fatalf("unexpected tool chain order: %+v", result.ToolCalls)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected one delivery artifact, got %d", len(result.Artifacts))
	}
	if result.Artifacts[0]["artifact_type"] != "generated_doc" {
		t.Fatalf("expected generated_doc artifact, got %+v", result.Artifacts[0])
	}
	if deliveryPayloadPath(result.DeliveryResult) != "workspace/notes/output.md" {
		t.Fatalf("expected explicit workspace output path, got %v", deliveryPayloadPath(result.DeliveryResult))
	}

	writtenPath := filepath.Join(workspaceRoot, "notes", "output.md")
	content, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("read written document: %v", err)
	}
	if !strings.Contains(string(content), "# 文件写入结果") {
		t.Fatalf("expected written document to contain title header, got %s", string(content))
	}
	if !strings.Contains(string(content), "第一点") {
		t.Fatalf("expected written document to contain generated content, got %s", string(content))
	}
}

func TestExecuteWriteFileBubbleConsumesArtifactCandidate(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "第一点\n第二点")

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_001b",
		RunID:        "run_001b",
		Title:        "生成文档",
		Intent:       map[string]any{"name": "write_file", "arguments": map[string]any{"target_path": "notes/output.md"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请整理成文档"},
		DeliveryType: "bubble",
		ResultTitle:  "文件写入结果",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected artifact candidate to be consumed when delivery yields none, got %d artifacts", len(result.Artifacts))
	}
	if result.Artifacts[0]["artifact_type"] != "generated_file" {
		t.Fatalf("expected generated_file artifact from candidate, got %+v", result.Artifacts[0])
	}
	if result.ToolOutput["audit_record"] == nil {
		t.Fatalf("expected audit candidate to be consumed, got %+v", result.ToolOutput)
	}
	if result.ToolOutput["recovery_point"] == nil {
		t.Fatalf("expected create flow to expose recovery point candidate, got %+v", result.ToolOutput)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "notes", "output.md")); err != nil {
		t.Fatalf("expected write_file bubble path to still write file, got %v", err)
	}
}

func TestExecuteWriteFileOverwriteCreatesAndAppliesRecoveryPoint(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "新的内容")
	originalPath := filepath.Join(workspaceRoot, "notes", "output.md")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(originalPath, []byte("旧的内容"), 0o644); err != nil {
		t.Fatalf("seed original file: %v", err)
	}

	result, err := service.Execute(context.Background(), Request{
		TaskID:          "task_restore",
		RunID:           "run_restore",
		Title:           "覆盖文件",
		Intent:          map[string]any{"name": "write_file", "arguments": map[string]any{"target_path": "notes/output.md"}},
		Snapshot:        contextsvc.TaskContextSnapshot{InputType: "text", Text: "请覆盖该文件"},
		DeliveryType:    "workspace_document",
		ResultTitle:     "文件写入结果",
		ApprovalGranted: true,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.RecoveryPoint == nil {
		t.Fatalf("expected overwrite execution to emit recovery point, got %+v", result.ToolOutput)
	}
	if result.ToolOutput["recovery_point"] == nil {
		t.Fatalf("expected tool output to expose recovery point, got %+v", result.ToolOutput)
	}
	overwrittenContent, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if !strings.Contains(string(overwrittenContent), "新的内容") {
		t.Fatalf("expected file to be overwritten, got %q", string(overwrittenContent))
	}

	recoveryPoint := checkpoint.RecoveryPoint{
		RecoveryPointID: result.RecoveryPoint["recovery_point_id"].(string),
		TaskID:          result.RecoveryPoint["task_id"].(string),
		Summary:         result.RecoveryPoint["summary"].(string),
		CreatedAt:       result.RecoveryPoint["created_at"].(string),
		Objects:         result.RecoveryPoint["objects"].([]string),
	}
	applyResult, err := service.ApplyRecoveryPoint(context.Background(), recoveryPoint)
	if err != nil {
		t.Fatalf("apply recovery point failed: %v", err)
	}
	if applyResult.RecoveryPointID != recoveryPoint.RecoveryPointID {
		t.Fatalf("expected recovery point id to round-trip, got %+v", applyResult)
	}
	restoredContent, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(restoredContent) != "旧的内容" {
		t.Fatalf("expected restore to recover original content, got %q", string(restoredContent))
	}
}

func TestExecuteBubbleReturnsGeneratedText(t *testing.T) {
	service, _ := newTestExecutionService(t, "这段内容主要在解释当前问题的原因和处理方向。")

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_002",
		RunID:        "run_002",
		Title:        "解释内容",
		Intent:       map[string]any{"name": "explain", "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text_selection", SelectionText: "需要解释的文本"},
		DeliveryType: "bubble",
		ResultTitle:  "解释结果",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.ToolName != "generate_text" {
		t.Fatalf("expected generate_text tool, got %s", result.ToolName)
	}
	if result.BubbleText != "这段内容主要在解释当前问题的原因和处理方向。" {
		t.Fatalf("expected bubble text to use generated output, got %s", result.BubbleText)
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("expected bubble delivery not to create artifacts, got %d", len(result.Artifacts))
	}
	if result.ToolOutput["model_invocation"] == nil {
		t.Fatalf("expected tool output to include model invocation, got %+v", result.ToolOutput)
	}
	if result.ToolOutput["audit_record"] == nil {
		t.Fatalf("expected tool output to include audit record, got %+v", result.ToolOutput)
	}
	if result.ModelInvocation == nil {
		t.Fatal("expected model invocation to be present")
	}
	if result.AuditRecord == nil {
		t.Fatal("expected audit record to be present")
	}
}

func TestExecuteDirectBuiltinReadFileUsesToolExecutor(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")
	readPath := filepath.Join(workspaceRoot, "notes", "source.txt")
	if err := os.MkdirAll(filepath.Dir(readPath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(readPath, []byte("hello from file"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_003",
		RunID:        "run_003",
		Title:        "读取文件",
		Intent:       map[string]any{"name": "read_file", "arguments": map[string]any{"path": "notes/source.txt"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请读取文件"},
		DeliveryType: "bubble",
		ResultTitle:  "读取结果",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.ToolName != "read_file" {
		t.Fatalf("expected read_file tool, got %s", result.ToolName)
	}
	if result.ToolInput["path"] != "notes/source.txt" {
		t.Fatalf("expected read_file path to be preserved, got %+v", result.ToolInput)
	}
	executionContext, ok := result.ToolInput["execution_context"].(map[string]any)
	if !ok {
		t.Fatalf("expected execution_context in builtin tool input, got %+v", result.ToolInput)
	}
	if executionContext["intent_name"] != "read_file" {
		t.Fatalf("expected read_file intent in execution_context, got %+v", executionContext)
	}
	if result.ToolOutput["summary_output"] == nil {
		t.Fatalf("expected direct builtin execution to include summary_output, got %+v", result.ToolOutput)
	}
	if !strings.Contains(result.BubbleText, "hello from file") {
		t.Fatalf("expected bubble text to include file preview, got %s", result.BubbleText)
	}
	if deliveryType, ok := result.DeliveryResult["type"].(string); !ok || deliveryType != "bubble" {
		t.Fatalf("expected bubble delivery result, got %+v", result.DeliveryResult)
	}
}

func TestExecuteDirectSidecarPageReadUsesToolExecutor(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", stubPlaywrightClient{readResult: tools.BrowserPageReadResult{
		Title:       "Example Page",
		TextContent: "page content from sidecar",
		MIMEType:    "text/html",
		TextType:    "text/html",
		Source:      "playwright_sidecar",
	}})

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_005",
		RunID:        "run_005",
		Title:        "页面读取",
		Intent:       map[string]any{"name": "page_read", "arguments": map[string]any{"url": "https://example.com"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请读取页面"},
		DeliveryType: "bubble",
		ResultTitle:  "页面读取结果",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.ToolName != "page_read" {
		t.Fatalf("expected page_read tool, got %s", result.ToolName)
	}
	if result.ToolOutput["summary_output"] == nil {
		t.Fatalf("expected sidecar tool summary output, got %+v", result.ToolOutput)
	}
	if !strings.Contains(result.BubbleText, "page content from sidecar") {
		t.Fatalf("expected bubble text to include sidecar preview, got %s", result.BubbleText)
	}
}

func TestExecuteDirectSidecarPageSearchUsesToolExecutor(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", stubPlaywrightClient{searchResult: tools.BrowserPageSearchResult{
		Matches:    []string{"example text match"},
		MatchCount: 1,
		Source:     "playwright_sidecar",
	}})

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_006",
		RunID:        "run_006",
		Title:        "页面搜索",
		Intent:       map[string]any{"name": "page_search", "arguments": map[string]any{"url": "https://example.com", "query": "example", "limit": 3}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请搜索页面"},
		DeliveryType: "bubble",
		ResultTitle:  "页面搜索结果",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.ToolName != "page_search" {
		t.Fatalf("expected page_search tool, got %s", result.ToolName)
	}
	if result.ToolOutput["summary_output"] == nil {
		t.Fatalf("expected page_search summary output, got %+v", result.ToolOutput)
	}
	if !strings.Contains(result.BubbleText, "关键词") {
		t.Fatalf("expected bubble text to summarize search result, got %s", result.BubbleText)
	}
	if result.DeliveryResult["type"] != "bubble" {
		t.Fatalf("expected bubble delivery result, got %+v", result.DeliveryResult)
	}
}

func TestExecuteDirectSidecarPageReadFailureReturnsMappedToolTrace(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", stubPlaywrightClient{err: tools.ErrPlaywrightSidecarFailed})

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_007",
		RunID:        "run_007",
		Title:        "页面读取失败",
		Intent:       map[string]any{"name": "page_read", "arguments": map[string]any{"url": "https://example.com"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请读取页面"},
		DeliveryType: "bubble",
		ResultTitle:  "页面读取结果",
	})
	if err == nil {
		t.Fatal("expected page_read execution to fail")
	}
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected wrapped tool execution failure, got %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected failed tool call trace, got %+v", result.ToolCalls)
	}
	if result.ToolCalls[0].ErrorCode == nil || *result.ToolCalls[0].ErrorCode != tools.ToolErrorCodePlaywrightSidecarFail {
		t.Fatalf("expected unified sidecar error code, got %+v", result.ToolCalls[0])
	}
}

func TestExecuteFallsBackWhenModelFails(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		t.Fatalf("register builtin tools: %v", err)
	}
	toolExecutor := tools.NewToolExecutor(toolRegistry)

	service := NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{err: errors.New("provider unavailable")},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		model.NewService(serviceconfig.ModelConfig{}, stubModelClient{err: errors.New("provider unavailable")}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		plugin.NewService(),
	)

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_004",
		RunID:        "run_004",
		Title:        "解释内容",
		Intent:       map[string]any{"name": "explain", "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text_selection", SelectionText: "需要解释的文本"},
		DeliveryType: "bubble",
		ResultTitle:  "解释结果",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(result.BubbleText, "需要解释的文本") {
		t.Fatalf("expected fallback bubble to include normalized input, got %s", result.BubbleText)
	}
}

func TestBuildPromptDoesNotDefaultUnknownIntentToSummarize(t *testing.T) {
	prompt := buildPrompt(Request{Intent: map[string]any{}}, "输入内容:\n你好")

	if strings.Contains(prompt, "请总结以下内容") {
		t.Fatalf("expected unknown intent prompt not to force summarize, got %s", prompt)
	}
	if !strings.Contains(prompt, "如果目标不明确") {
		t.Fatalf("expected unknown intent prompt to ask for clarification behavior, got %s", prompt)
	}
}

func TestFallbackOutputRequestsClarificationWhenIntentMissing(t *testing.T) {
	output := fallbackOutput(Request{Intent: map[string]any{}}, "你好")

	if !strings.Contains(output, "请补充你的目标") {
		t.Fatalf("expected unknown intent fallback to request clarification, got %s", output)
	}
	if strings.Contains(output, "总结结果") {
		t.Fatalf("expected unknown intent fallback not to pretend summarize, got %s", output)
	}
}

func TestAssessGovernanceRequiresAuthorizationForRestoreWrite(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")

	assessment, handled, err := service.AssessGovernance(context.Background(), Request{
		TaskID:       "task_auth_write",
		RunID:        "run_auth_write",
		Intent:       map[string]any{"name": "write_file", "arguments": map[string]any{"target_path": "notes/result.md", "require_authorization": true}},
		DeliveryType: "workspace_document",
		ResultTitle:  "授权写入",
	})
	if err != nil {
		t.Fatalf("AssessGovernance returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected write_file governance path to be handled")
	}
	if !assessment.ApprovalRequired {
		t.Fatalf("expected approval to be required, got %+v", assessment)
	}
	if assessment.OperationName != "write_file" {
		t.Fatalf("expected write_file operation, got %+v", assessment)
	}
	expectedTarget := "notes/result.md"
	if assessment.TargetObject != expectedTarget {
		t.Fatalf("expected target object %q, got %q", expectedTarget, assessment.TargetObject)
	}
	files, _ := assessment.ImpactScope["files"].([]string)
	expectedImpactFile := filepath.Join(workspaceRoot, "notes", "result.md")
	if len(files) != 1 || files[0] != expectedImpactFile {
		t.Fatalf("expected impact scope files to include %q, got %+v", expectedImpactFile, assessment.ImpactScope)
	}
}

func TestAssessGovernanceExecCommandUsesWorkspaceTargetWithoutRecoveryPoint(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")

	assessment, handled, err := service.AssessGovernance(context.Background(), Request{
		TaskID: "task_exec_auth",
		RunID:  "run_exec_auth",
		Intent: map[string]any{"name": "exec_command", "arguments": map[string]any{
			"command":               "git status",
			"working_dir":           workspaceRoot,
			"require_authorization": true,
		}},
	})
	if err != nil {
		t.Fatalf("AssessGovernance returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected exec_command governance path to be handled")
	}
	if assessment.OperationName != "exec_command" || assessment.TargetObject != workspaceRoot {
		t.Fatalf("unexpected exec_command assessment: %+v", assessment)
	}
	if !assessment.ApprovalRequired {
		t.Fatalf("expected exec_command to require approval when flagged, got %+v", assessment)
	}

	recoveryPoint, err := service.prepareGovernanceRecoveryPoint(context.Background(), Request{TaskID: "task_exec_auth"}, workspaceRoot, "exec_command", map[string]any{"working_dir": workspaceRoot})
	if err != nil {
		t.Fatalf("prepareGovernanceRecoveryPoint returned error: %v", err)
	}
	if recoveryPoint != nil {
		t.Fatalf("expected exec_command not to create recovery point, got %+v", recoveryPoint)
	}
}

func TestAssessGovernancePageReadUsesURLTarget(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient())
	assessment, handled, err := service.AssessGovernance(context.Background(), Request{
		TaskID: "task_page_read_auth",
		RunID:  "run_page_read_auth",
		Intent: map[string]any{"name": "page_read", "arguments": map[string]any{
			"url":                   "https://example.com/demo",
			"require_authorization": true,
		}},
		DeliveryType: "bubble",
		ResultTitle:  "网页读取结果",
	})
	if err != nil {
		t.Fatalf("AssessGovernance returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected page_read governance path to be handled")
	}
	if assessment.OperationName != "page_read" || assessment.TargetObject != "https://example.com/demo" {
		t.Fatalf("unexpected page_read assessment: %+v", assessment)
	}
	if !assessment.ApprovalRequired {
		t.Fatalf("expected page_read to require approval when flagged, got %+v", assessment)
	}
	webpages, _ := assessment.ImpactScope["webpages"].([]string)
	if len(webpages) != 1 || webpages[0] != "https://example.com/demo" {
		t.Fatalf("expected webpage impact scope to include target URL, got %+v", assessment.ImpactScope)
	}
}

func TestAssessGovernancePageSearchPreservesQueryInput(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient())
	assessment, handled, err := service.AssessGovernance(context.Background(), Request{
		TaskID: "task_page_search_auth",
		RunID:  "run_page_search_auth",
		Intent: map[string]any{"name": "page_search", "arguments": map[string]any{
			"url":   "https://example.com/search",
			"query": "alpha",
			"limit": 2,
		}},
		DeliveryType: "bubble",
		ResultTitle:  "网页搜索结果",
	})
	if err != nil {
		t.Fatalf("AssessGovernance returned error: %v", err)
	}
	if !handled {
		t.Fatal("expected page_search governance path to be handled")
	}
	if assessment.OperationName != "page_search" || assessment.TargetObject != "https://example.com/search" {
		t.Fatalf("unexpected page_search assessment: %+v", assessment)
	}
	webpages, _ := assessment.ImpactScope["webpages"].([]string)
	if len(webpages) != 1 || webpages[0] != "https://example.com/search" {
		t.Fatalf("expected webpage impact scope to include search URL, got %+v", assessment.ImpactScope)
	}
}

type stubExecutionCapability struct {
	result tools.CommandExecutionResult
	err    error
}

type stubPlaywrightClient struct {
	readResult   tools.BrowserPageReadResult
	searchResult tools.BrowserPageSearchResult
	err          error
}

func (s stubPlaywrightClient) ReadPage(_ context.Context, url string) (tools.BrowserPageReadResult, error) {
	if s.err != nil {
		return tools.BrowserPageReadResult{}, s.err
	}
	result := s.readResult
	if result.URL == "" {
		result.URL = url
	}
	return result, nil
}

func (s stubPlaywrightClient) SearchPage(_ context.Context, url, query string, limit int) (tools.BrowserPageSearchResult, error) {
	if s.err != nil {
		return tools.BrowserPageSearchResult{}, s.err
	}
	result := s.searchResult
	if result.URL == "" {
		result.URL = url
	}
	if result.Query == "" {
		result.Query = query
	}
	if limit > 0 && len(result.Matches) > limit {
		result.Matches = result.Matches[:limit]
		result.MatchCount = len(result.Matches)
	}
	return result, nil
}

func (s stubExecutionCapability) RunCommand(_ context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error) {
	_ = command
	_ = args
	_ = workingDir
	if s.err != nil {
		return tools.CommandExecutionResult{}, s.err
	}
	return s.result, nil
}
