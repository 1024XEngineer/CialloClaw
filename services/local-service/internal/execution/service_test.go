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
	if result.ToolOutput["recovery_point"] != nil {
		t.Fatalf("expected no recovery point for create flow, got %+v", result.ToolOutput)
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
	if _, err := os.Stat(filepath.Join(workspaceRoot, "notes", "output.md")); err != nil {
		t.Fatalf("expected write_file bubble path to still write file, got %v", err)
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

type stubExecutionCapability struct {
	result tools.CommandExecutionResult
	err    error
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
