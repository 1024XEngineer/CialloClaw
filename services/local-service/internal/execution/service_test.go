package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/agentloop"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

type stubModelClient struct {
	output                 string
	err                    error
	toolCalls              []model.ToolCallResult
	generateToolCallsCount int
}

func (s *stubModelClient) GenerateText(_ context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
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

func (s *stubModelClient) GenerateToolCalls(_ context.Context, request model.ToolCallRequest) (model.ToolCallResult, error) {
	s.generateToolCallsCount++
	if s.err != nil {
		return model.ToolCallResult{}, s.err
	}
	if len(s.toolCalls) == 0 {
		return model.ToolCallResult{
			RequestID:  "req_tool_final",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: request.Input,
		}, nil
	}
	result := s.toolCalls[0]
	s.toolCalls = s.toolCalls[1:]
	return result, nil
}

func newTestExecutionService(t *testing.T, output string) (*Service, string) {
	t.Helper()
	return newTestExecutionServiceWithConfig(t, serviceconfig.ModelConfig{}, output)
}

func newTestExecutionServiceWithConfig(t *testing.T, cfg serviceconfig.ModelConfig, output string) (*Service, string) {
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
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		model.NewService(cfg, &stubModelClient{output: output}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		plugin.NewService(),
	), workspaceRoot
}

func newTestExecutionServiceWithModelClient(t *testing.T, client model.Client) (*Service, string) {
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
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		model.NewService(serviceconfig.ModelConfig{}, client),
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
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		model.NewService(serviceconfig.ModelConfig{}, &stubModelClient{output: output}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		plugin.NewService(),
	), workspaceRoot
}

func newTestExecutionServiceWithWorkers(t *testing.T, output string, playwright tools.PlaywrightSidecarClient, ocr tools.OCRWorkerClient, media tools.MediaWorkerClient) (*Service, string) {
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
	if err := sidecarclient.RegisterOCRTools(toolRegistry); err != nil {
		t.Fatalf("register ocr tools: %v", err)
	}
	if err := sidecarclient.RegisterMediaTools(toolRegistry); err != nil {
		t.Fatalf("register media tools: %v", err)
	}
	toolExecutor := tools.NewToolExecutor(toolRegistry)

	return NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		playwright,
		ocr,
		media,
		model.NewService(serviceconfig.ModelConfig{}, &stubModelClient{output: output}),
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

func TestExecuteAgentLoopReadsFileBeforeReturningAnswer(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{
			{
				RequestID: "req_loop_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{
					{
						Name:      "read_file",
						Arguments: map[string]any{"path": "notes/source.txt"},
					},
				},
			},
			{
				RequestID:  "req_loop_2",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I checked the file and extracted the key takeaway.",
			},
		},
	}
	service, workspaceRoot := newTestExecutionServiceWithModelClient(t, modelClient)
	sourcePath := filepath.Join(workspaceRoot, "notes", "source.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("Important launch note"), 0o644); err != nil {
		t.Fatalf("seed source file: %v", err)
	}

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop",
		RunID:        "run_loop",
		Title:        "Loop test",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Please inspect the note and tell me the takeaway."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop result",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.Content != "I checked the file and extracted the key takeaway." {
		t.Fatalf("unexpected loop output: %s", result.Content)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one executed tool call, got %+v", result.ToolCalls)
	}
	if result.ToolCalls[0].ToolName != "read_file" {
		t.Fatalf("expected read_file tool call, got %+v", result.ToolCalls[0])
	}
	if result.ToolCalls[0].Output["loop_round"] != 1 {
		t.Fatalf("expected first tool call to be annotated with loop round, got %+v", result.ToolCalls[0].Output)
	}
	if result.ModelInvocation["request_id"] != "req_loop_2" {
		t.Fatalf("expected final planning turn metadata, got %+v", result.ModelInvocation)
	}
}

func TestCompactAgentLoopHistoryKeepsRecentObservations(t *testing.T) {
	history := []string{
		"Tool read_file succeeded. Summary: {\"path\":\"notes/1.md\",\"excerpt\":\"alpha alpha alpha alpha alpha\"}",
		"Tool read_file succeeded. Summary: {\"path\":\"notes/2.md\",\"excerpt\":\"beta beta beta beta beta\"}",
		"Tool page_read succeeded. Summary: {\"url\":\"https://example.com\",\"title\":\"Example\"}",
	}

	compacted := compactAgentLoopHistory(history, 120, 1)
	if len(compacted) != 2 {
		t.Fatalf("expected one compressed summary plus one recent item, got %+v", compacted)
	}
	if !strings.Contains(compacted[0], "Compressed earlier observations") {
		t.Fatalf("expected compacted head summary, got %+v", compacted)
	}
	if compacted[1] != history[2] {
		t.Fatalf("expected most recent observation to stay verbatim, got %+v", compacted)
	}
}

func TestExecuteAgentLoopHonorsConfiguredMaxToolIterations(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{
			{
				RequestID: "req_loop_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			},
			{
				RequestID: "req_loop_2",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			},
			{
				RequestID: "req_loop_3",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			},
		},
	}
	cfg := serviceconfig.ModelConfig{MaxToolIterations: 2}
	service, workspaceRoot := newTestExecutionServiceWithModelClientAndConfig(t, cfg, modelClient)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_limit",
		RunID:        "run_loop_limit",
		Title:        "Loop limit test",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the notes directory and keep going."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop result",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("expected loop to stop after two configured iterations, got %+v", result.ToolCalls)
	}
	if result.ModelInvocation["request_id"] != "req_loop_2" {
		t.Fatalf("expected last recorded invocation to come from second turn, got %+v", result.ModelInvocation)
	}
}

func TestExecuteAgentLoopPersistsRuntimeEventsAndStopReason(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{
			{
				RequestID: "req_loop_runtime_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "."}}},
			},
			{
				RequestID:  "req_loop_runtime_2",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Loop runtime finished cleanly.",
			},
		},
	}
	loopStore := storage.NewService(nil).LoopRuntimeStore()
	service, _ := newTestExecutionServiceWithModelClient(t, modelClient)
	service = service.WithLoopRuntimeStore(loopStore)

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_runtime",
		RunID:        "run_loop_runtime",
		Title:        "Loop runtime persistence",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the workspace and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop runtime result",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Content != "Loop runtime finished cleanly." {
		t.Fatalf("unexpected loop runtime result: %+v", result)
	}
	events, total, err := loopStore.ListEvents(context.Background(), "task_loop_runtime", 20, 0)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if total == 0 || len(events) == 0 {
		t.Fatal("expected persisted loop events")
	}
	foundCompleted := false
	for _, event := range events {
		if event.Type == "loop.completed" {
			foundCompleted = true
			break
		}
	}
	if !foundCompleted {
		t.Fatalf("expected loop.completed event in %+v", events)
	}
}

func TestExecuteAgentLoopRetriesPlannerOnceBeforeFailing(t *testing.T) {
	modelClient := &stubModelClient{err: errors.New("temporary planner error")}
	service, _ := newTestExecutionServiceWithModelClient(t, modelClient)
	_, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_retry_planner",
		RunID:        "run_loop_retry_planner",
		Title:        "Loop retry planner",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the workspace and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop retry result",
	})
	if err == nil {
		t.Fatal("expected planner retry path to still surface final error")
	}
}

func TestExecuteAgentLoopHonorsConfiguredPlannerRetryBudget(t *testing.T) {
	modelClient := &stubModelClient{err: errors.New("temporary planner error")}
	cfg := serviceconfig.ModelConfig{PlannerRetryBudget: 2}
	service, _ := newTestExecutionServiceWithModelClientAndConfig(t, cfg, modelClient)
	_, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_retry_budget",
		RunID:        "run_loop_retry_budget",
		Title:        "Loop retry budget",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the workspace and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop retry budget result",
	})
	if err == nil {
		t.Fatal("expected planner retry budget path to still surface final error")
	}
	if modelClient.generateToolCallsCount != 3 {
		t.Fatalf("expected planner to be attempted three times, got %d", modelClient.generateToolCallsCount)
	}
}

func TestExecuteAgentLoopConsumesActiveRunSteeringBetweenRounds(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{
			{
				RequestID: "req_loop_active_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			},
			{
				RequestID:  "req_loop_active_2",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Loop runtime finished after follow-up steering.",
			},
		},
	}
	service, workspaceRoot := newTestExecutionServiceWithModelClient(t, modelClient)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	pollCount := 0
	service = service.WithSteeringPoller(func(_ string) []string {
		pollCount++
		if pollCount == 2 {
			return []string{"Also include the newly added summary section."}
		}
		return nil
	})

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_active_steer",
		RunID:        "run_loop_active_steer",
		Title:        "Loop active steering",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the notes directory and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop result",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Content != "Loop runtime finished after follow-up steering." {
		t.Fatalf("unexpected active steering result: %+v", result)
	}
	if len(modelClient.toolCalls) != 0 {
		t.Fatalf("expected both tool call responses to be consumed, got %+v", modelClient.toolCalls)
	}
	if pollCount < 2 {
		t.Fatalf("expected active-run steering poller to run between rounds, got %d", pollCount)
	}
}

func TestRunStatusFromStopReasonTreatsToolRetryExhaustedAsFailed(t *testing.T) {
	if status := runStatusFromStopReason(agentloop.StopReasonToolRetryExhausted); status != "failed" {
		t.Fatalf("expected tool retry exhausted to map to failed, got %q", status)
	}
}

func newTestExecutionServiceWithModelClientAndConfig(t *testing.T, cfg serviceconfig.ModelConfig, client model.Client) (*Service, string) {
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
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		model.NewService(cfg, client),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		plugin.NewService(),
	), workspaceRoot
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
		TaskID:               "task_005",
		RunID:                "run_005",
		Title:                "页面读取",
		Intent:               map[string]any{"name": "page_read", "arguments": map[string]any{"url": "https://example.com"}},
		Snapshot:             contextsvc.TaskContextSnapshot{InputType: "text", Text: "请读取页面"},
		DeliveryType:         "bubble",
		ResultTitle:          "页面读取结果",
		ApprovalGranted:      true,
		ApprovedOperation:    "page_read",
		ApprovedTargetObject: "https://example.com",
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
		TaskID:               "task_006",
		RunID:                "run_006",
		Title:                "页面搜索",
		Intent:               map[string]any{"name": "page_search", "arguments": map[string]any{"url": "https://example.com", "query": "example", "limit": 3}},
		Snapshot:             contextsvc.TaskContextSnapshot{InputType: "text", Text: "请搜索页面"},
		DeliveryType:         "bubble",
		ResultTitle:          "页面搜索结果",
		ApprovalGranted:      true,
		ApprovedOperation:    "page_search",
		ApprovedTargetObject: "https://example.com",
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

func TestExecuteDirectSidecarPageInteractUsesToolExecutor(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", stubPlaywrightClient{interactResult: tools.BrowserPageInteractResult{
		Title:          "Interactive Page",
		TextContent:    "interaction complete",
		ActionsApplied: 2,
		Source:         "playwright_sidecar",
	}})

	result, err := service.Execute(context.Background(), Request{
		TaskID:               "task_006a",
		RunID:                "run_006a",
		Title:                "页面操作",
		Intent:               map[string]any{"name": "page_interact", "arguments": map[string]any{"url": "https://example.com", "actions": []any{map[string]any{"type": "click", "selector": "button"}}}},
		Snapshot:             contextsvc.TaskContextSnapshot{InputType: "text", Text: "请点击按钮"},
		DeliveryType:         "bubble",
		ResultTitle:          "页面操作结果",
		ApprovalGranted:      true,
		ApprovedOperation:    "page_interact",
		ApprovedTargetObject: "https://example.com",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.ToolName != "page_interact" {
		t.Fatalf("expected page_interact tool, got %s", result.ToolName)
	}
	if result.ToolOutput["actions_applied"] != 2 {
		t.Fatalf("expected action count in tool output, got %+v", result.ToolOutput)
	}
}

func TestExecuteDirectSidecarStructuredDOMUsesToolExecutor(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", stubPlaywrightClient{structuredResult: tools.BrowserStructuredDOMResult{
		Title:    "Structured Page",
		Headings: []string{"Heading A"},
		Links:    []string{"Link A"},
		Buttons:  []string{"Submit"},
		Inputs:   []string{"email"},
		Source:   "playwright_sidecar",
	}})

	result, err := service.Execute(context.Background(), Request{
		TaskID:               "task_006b",
		RunID:                "run_006b",
		Title:                "结构化页面",
		Intent:               map[string]any{"name": "structured_dom", "arguments": map[string]any{"url": "https://example.com"}},
		Snapshot:             contextsvc.TaskContextSnapshot{InputType: "text", Text: "请提取页面结构"},
		DeliveryType:         "bubble",
		ResultTitle:          "页面结构结果",
		ApprovalGranted:      true,
		ApprovedOperation:    "structured_dom",
		ApprovedTargetObject: "https://example.com",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.ToolName != "structured_dom" {
		t.Fatalf("expected structured_dom tool, got %s", result.ToolName)
	}
	if result.ToolOutput["summary_output"] == nil {
		t.Fatalf("expected summary output, got %+v", result.ToolOutput)
	}
}

func TestExecuteDirectOCRAndMediaToolsUseWorkerClients(t *testing.T) {
	ocrPath := "notes/demo.txt"
	framesDir := "frames"
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: ocrPath, Text: "hello from ocr", Language: "plain_text", PageCount: 1, Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{framesResult: tools.MediaFrameExtractResult{InputPath: "clips/demo.mp4", OutputDir: framesDir, FramePaths: []string{filepath.Join(framesDir, "frame-001.jpg")}, FrameCount: 1, Source: "media_worker_frames"}}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	ocrResult, err := service.Execute(context.Background(), Request{
		TaskID:       "task_006c",
		RunID:        "run_006c",
		Title:        "提取文本",
		Intent:       map[string]any{"name": "extract_text", "arguments": map[string]any{"path": ocrPath}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请提取文本"},
		DeliveryType: "bubble",
		ResultTitle:  "提取结果",
	})
	if err != nil {
		t.Fatalf("extract_text execute failed: %v", err)
	}
	if ocrResult.ToolName != "extract_text" || !strings.Contains(ocrResult.BubbleText, "hello from ocr") {
		t.Fatalf("unexpected extract_text result: %+v", ocrResult)
	}
	mediaResult, err := service.Execute(context.Background(), Request{
		TaskID:       "task_006d",
		RunID:        "run_006d",
		Title:        "抽取视频帧",
		Intent:       map[string]any{"name": "extract_frames", "arguments": map[string]any{"path": "clips/demo.mp4", "output_dir": framesDir, "limit": 1.0}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请抽取视频帧"},
		DeliveryType: "bubble",
		ResultTitle:  "抽帧结果",
	})
	if err != nil {
		t.Fatalf("extract_frames execute failed: %v", err)
	}
	if mediaResult.ToolName != "extract_frames" || mediaResult.ToolOutput["frame_count"] != 1 {
		t.Fatalf("unexpected extract_frames result: %+v", mediaResult)
	}
}

func TestExecuteDirectSidecarPageReadFailureReturnsMappedToolTrace(t *testing.T) {
	service, _ := newTestExecutionServiceWithPlaywright(t, "unused", stubPlaywrightClient{err: tools.ErrPlaywrightSidecarFailed})

	result, err := service.Execute(context.Background(), Request{
		TaskID:               "task_007",
		RunID:                "run_007",
		Title:                "页面读取失败",
		Intent:               map[string]any{"name": "page_read", "arguments": map[string]any{"url": "https://example.com"}},
		Snapshot:             contextsvc.TaskContextSnapshot{InputType: "text", Text: "请读取页面"},
		DeliveryType:         "bubble",
		ResultTitle:          "页面读取结果",
		ApprovalGranted:      true,
		ApprovedOperation:    "page_read",
		ApprovedTargetObject: "https://example.com",
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
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		model.NewService(serviceconfig.ModelConfig{}, &stubModelClient{err: errors.New("provider unavailable")}),
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
	if assessment.RiskLevel != string(risk.RiskLevelYellow) {
		t.Fatalf("expected page_read yellow risk level, got %+v", assessment)
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
	if !assessment.ApprovalRequired {
		t.Fatalf("expected page_search to require approval, got %+v", assessment)
	}
}

func TestResolveToolExecutionSupportsWorkerAndInteractiveIntents(t *testing.T) {
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), sidecarclient.NewNoopOCRWorkerClient(), sidecarclient.NewNoopMediaWorkerClient())
	tests := []struct {
		name     string
		request  Request
		wantTool string
		wantKey  string
	}{
		{name: "page_interact", request: Request{Intent: map[string]any{"name": "page_interact", "arguments": map[string]any{"url": "https://example.com", "actions": []any{map[string]any{"type": "click", "selector": "button"}}}}}, wantTool: "page_interact", wantKey: "url"},
		{name: "structured_dom", request: Request{Intent: map[string]any{"name": "structured_dom", "arguments": map[string]any{"url": "https://example.com"}}}, wantTool: "structured_dom", wantKey: "url"},
		{name: "extract_text", request: Request{Intent: map[string]any{"name": "extract_text", "arguments": map[string]any{"path": "notes/demo.txt"}}}, wantTool: "extract_text", wantKey: "path"},
		{name: "transcode_media", request: Request{Intent: map[string]any{"name": "transcode_media", "arguments": map[string]any{"path": "clips/demo.mov", "output_path": "clips/demo.mp4", "format": "mp4"}}}, wantTool: "transcode_media", wantKey: "output_path"},
		{name: "extract_frames", request: Request{Intent: map[string]any{"name": "extract_frames", "arguments": map[string]any{"path": "clips/demo.mov", "output_dir": "frames", "limit": 2.0}}}, wantTool: "extract_frames", wantKey: "output_dir"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			toolName, input, ok := service.resolveToolExecution(test.request, map[string]any{"payload": map[string]any{}}, "")
			if !ok || toolName != test.wantTool {
				t.Fatalf("expected %s tool resolution, got tool=%s ok=%v input=%+v", test.wantTool, toolName, ok, input)
			}
			if _, exists := input[test.wantKey]; !exists {
				t.Fatalf("expected input key %s, got %+v", test.wantKey, input)
			}
		})
	}
}

func TestResolveGovernanceToolExecutionSupportsWorkerAndInteractiveIntents(t *testing.T) {
	service, workspaceRoot := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), sidecarclient.NewNoopOCRWorkerClient(), sidecarclient.NewNoopMediaWorkerClient())
	tests := []struct {
		name     string
		request  Request
		wantTool string
	}{
		{name: "page_interact", request: Request{TaskID: "task_001", RunID: "run_001", DeliveryType: "bubble", ResultTitle: "页面交互结果", Intent: map[string]any{"name": "page_interact", "arguments": map[string]any{"url": "https://example.com", "actions": []any{map[string]any{"type": "click", "selector": "button"}}}}}, wantTool: "page_interact"},
		{name: "structured_dom", request: Request{TaskID: "task_002", RunID: "run_002", DeliveryType: "bubble", ResultTitle: "结构化结果", Intent: map[string]any{"name": "structured_dom", "arguments": map[string]any{"url": "https://example.com"}}}, wantTool: "structured_dom"},
		{name: "ocr_pdf", request: Request{TaskID: "task_003", RunID: "run_003", DeliveryType: "bubble", ResultTitle: "OCR 结果", Intent: map[string]any{"name": "ocr_pdf", "arguments": map[string]any{"path": "docs/demo.pdf", "language": "eng"}}}, wantTool: "ocr_pdf"},
		{name: "normalize_recording", request: Request{TaskID: "task_004", RunID: "run_004", DeliveryType: "bubble", ResultTitle: "归一化结果", Intent: map[string]any{"name": "normalize_recording", "arguments": map[string]any{"path": "clips/demo.mov", "output_path": "clips/demo.mp4"}}}, wantTool: "normalize_recording"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			toolName, input, execCtx, ok, err := service.resolveGovernanceToolExecution(test.request)
			if err != nil {
				t.Fatalf("resolveGovernanceToolExecution returned error: %v", err)
			}
			if !ok || toolName != test.wantTool || execCtx == nil {
				t.Fatalf("expected governance tool %s, got tool=%s ok=%v ctx=%+v", test.wantTool, toolName, ok, execCtx)
			}
			if execCtx.WorkspacePath != workspaceRoot {
				t.Fatalf("expected workspace root in tool context, got %q", execCtx.WorkspacePath)
			}
			if len(input) == 0 {
				t.Fatalf("expected tool input, got %+v", input)
			}
		})
	}
}

func TestExecutionWorkerHelpersCoverArtifactsRecoveryAndTrace(t *testing.T) {
	artifacts := toolArtifactsFromResult("task_001", &tools.ToolExecutionResult{Artifacts: []tools.ArtifactRef{{ArtifactType: "generated_file", Title: "demo.mp4", Path: "clips/demo.mp4", MimeType: "video/mp4"}}})
	if len(artifacts) != 1 || artifacts[0]["path"] != "clips/demo.mp4" {
		t.Fatalf("unexpected tool artifacts: %+v", artifacts)
	}
	if artifacts[0]["artifact_id"] == "" {
		t.Fatalf("expected runtime tool artifact to receive a stable artifact_id, got %+v", artifacts)
	}
	if workspacePathFromDeliveryResult(nil) != "workspace" {
		t.Fatalf("expected default workspace path")
	}
	if workspacePathFromDeliveryResult(map[string]any{"payload": map[string]any{"path": "notes/demo.md"}}) != "notes/demo.md" {
		t.Fatalf("expected workspace payload path to normalize")
	}
	if checkpointObjectPath("demo.txt") != "workspace/demo.txt" {
		t.Fatalf("expected checkpoint object path to be workspace-relative")
	}
	if firstNonEmptyRecoveryPoint(map[string]any{"id": "primary"}, map[string]any{"id": "fallback"})["id"] != "primary" {
		t.Fatalf("expected primary recovery point to win")
	}
	if firstNonEmptyRecoveryPoint(nil, map[string]any{"id": "fallback"})["id"] != "fallback" {
		t.Fatalf("expected fallback recovery point")
	}
	result := &Result{ToolCalls: []tools.ToolCallRecord{{ToolName: "extract_text", Input: map[string]any{"path": "notes/demo.txt"}, Output: map[string]any{"text": "hello"}}}}
	assignLatestToolTrace(result, latestToolCall(result.ToolCalls))
	enrichToolTrace(result, map[string]any{"worker": "ocr_worker"})
	enrichLatestToolCall(result, map[string]any{"worker": "ocr_worker"})
	if result.ToolName != "extract_text" || result.ToolOutput["worker"] != "ocr_worker" {
		t.Fatalf("unexpected enriched result: %+v", result)
	}
	if result.ToolCalls[0].Output["worker"] != "ocr_worker" {
		t.Fatalf("expected latest tool call enrichment, got %+v", result.ToolCalls[0].Output)
	}
	cloned := cloneMap(map[string]any{"nested": map[string]any{"value": "demo"}, "items": []map[string]any{{"path": "notes/demo.txt"}}})
	if cloned["nested"].(map[string]any)["value"] != "demo" || cloned["items"].([]map[string]any)[0]["path"] != "notes/demo.txt" {
		t.Fatalf("unexpected cloned map: %+v", cloned)
	}
	if len(cloneMapSlice([]map[string]any{{"path": "notes/demo.txt"}})) != 1 {
		t.Fatalf("expected cloneMapSlice to clone one item")
	}
}

func TestBuildExecutionInputAndFileSectionCoverFileBranches(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes", "demo.txt"), []byte("worker file content"), 0o644); err != nil {
		t.Fatalf("write demo file: %v", err)
	}
	section := service.fileSection("notes/demo.txt")
	if !strings.Contains(section, "worker file content") {
		t.Fatalf("expected file content section, got %s", section)
	}
	missingSection := service.fileSection("notes/missing.txt")
	if !strings.Contains(missingSection, "读取失败") {
		t.Fatalf("expected missing file section, got %s", missingSection)
	}
	service.fileSystem = nil
	if section := service.fileSection("notes/demo.txt"); section != "文件: notes/demo.txt" {
		t.Fatalf("expected no-filesystem branch, got %s", section)
	}
	service, _ = newTestExecutionService(t, "unused")
	inputText := service.buildExecutionInput(contextsvc.TaskContextSnapshot{SelectionText: "选中文本", Text: "输入文本", ErrorText: "错误信息", Files: []string{"notes/demo.txt"}, PageTitle: "Page", PageURL: "https://example.com", AppName: "Desktop"})
	for _, fragment := range []string{"选中文本", "输入文本", "错误信息", "页面上下文"} {
		if !strings.Contains(inputText, fragment) {
			t.Fatalf("expected execution input to contain %q, got %s", fragment, inputText)
		}
	}
}

func TestToolBubbleTextAndGovernanceHelpersSupportNewWorkerFlows(t *testing.T) {
	bubbleText := toolBubbleText("extract_text", &tools.ToolExecutionResult{SummaryOutput: map[string]any{"content_preview": "hello ocr"}})
	if bubbleText != "hello ocr" {
		t.Fatalf("expected content preview bubble text, got %s", bubbleText)
	}
	searchBubble := toolBubbleText("page_search", &tools.ToolExecutionResult{SummaryOutput: map[string]any{"query": "demo", "match_count": 3}})
	if !strings.Contains(searchBubble, "关键词") {
		t.Fatalf("expected search bubble text, got %s", searchBubble)
	}
	if governanceTargetObject("page_interact", map[string]any{"url": "https://example.com"}, &tools.ToolExecuteContext{WorkspacePath: "/workspace"}) != "https://example.com" {
		t.Fatalf("expected page_interact governance target url")
	}
	if governanceTargetObject("extract_text", map[string]any{"path": "notes/demo.txt"}, &tools.ToolExecuteContext{WorkspacePath: "/workspace"}) != "notes/demo.txt" {
		t.Fatalf("expected file-based governance target path")
	}
	if governanceTargetObject("transcode_media", map[string]any{"path": "clips/demo.mov", "output_path": "exports/demo.mp4"}, &tools.ToolExecuteContext{WorkspacePath: "/workspace"}) != "exports/demo.mp4" {
		t.Fatalf("expected media governance target to follow output_path")
	}
	if governanceTargetObject("extract_frames", map[string]any{"path": "clips/demo.mov", "output_dir": "exports/frames"}, &tools.ToolExecuteContext{WorkspacePath: "/workspace"}) != "exports/frames" {
		t.Fatalf("expected frame extraction governance target to follow output_dir")
	}
	if approvedTargetObject(map[string]any{"name": "page_interact", "arguments": map[string]any{"url": "https://example.com"}}, "/workspace") != "https://example.com" {
		t.Fatalf("expected webpage intent to preserve approved url target")
	}
	if approvedTargetObject(map[string]any{"name": "transcode_media", "arguments": map[string]any{"path": "clips/demo.mov", "output_path": "exports/demo.mp4"}}, "/workspace") != "/workspace/exports/demo.mp4" {
		t.Fatalf("expected media intent approval target to follow output_path")
	}
}

func TestPrepareWriteFileRecoveryPointAndWorkspaceHelpers(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes", "demo.txt"), []byte("demo content"), 0o644); err != nil {
		t.Fatalf("write demo file: %v", err)
	}
	recoveryPoint, err := service.prepareWriteFileRecoveryPoint(context.Background(), Request{TaskID: "task_007"}, "write_file", map[string]any{"path": "notes/demo.txt"})
	if err != nil {
		t.Fatalf("prepareWriteFileRecoveryPoint returned error: %v", err)
	}
	if recoveryPoint["recovery_point_id"] == "" {
		t.Fatalf("expected recovery point id, got %+v", recoveryPoint)
	}
	if result, err := service.prepareWriteFileRecoveryPoint(context.Background(), Request{TaskID: "task_007"}, "read_file", map[string]any{"path": "notes/demo.txt"}); err != nil || result != nil {
		t.Fatalf("expected non write_file tool to skip recovery point, got result=%+v err=%v", result, err)
	}
	if resolveWorkspaceRoot(service.fileSystem) != workspaceRoot {
		t.Fatalf("expected resolved workspace root %q, got %q", workspaceRoot, resolveWorkspaceRoot(service.fileSystem))
	}
}

type stubExecutionCapability struct {
	result tools.CommandExecutionResult
	err    error
}

type stubPlaywrightClient struct {
	readResult       tools.BrowserPageReadResult
	searchResult     tools.BrowserPageSearchResult
	interactResult   tools.BrowserPageInteractResult
	structuredResult tools.BrowserStructuredDOMResult
	err              error
}

type stubOCRWorkerClient struct {
	result tools.OCRTextResult
	err    error
}

type stubMediaWorkerClient struct {
	transcodeResult tools.MediaTranscodeResult
	framesResult    tools.MediaFrameExtractResult
	err             error
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

func (s stubPlaywrightClient) InteractPage(_ context.Context, url string, _ []map[string]any) (tools.BrowserPageInteractResult, error) {
	if s.err != nil {
		return tools.BrowserPageInteractResult{}, s.err
	}
	result := s.interactResult
	if result.URL == "" {
		result.URL = url
	}
	return result, nil
}

func (s stubPlaywrightClient) StructuredDOM(_ context.Context, url string) (tools.BrowserStructuredDOMResult, error) {
	if s.err != nil {
		return tools.BrowserStructuredDOMResult{}, s.err
	}
	result := s.structuredResult
	if result.URL == "" {
		result.URL = url
	}
	return result, nil
}

func (s stubOCRWorkerClient) ExtractText(_ context.Context, _ string) (tools.OCRTextResult, error) {
	if s.err != nil {
		return tools.OCRTextResult{}, s.err
	}
	return s.result, nil
}

func (s stubOCRWorkerClient) OCRImage(_ context.Context, _ string, _ string) (tools.OCRTextResult, error) {
	if s.err != nil {
		return tools.OCRTextResult{}, s.err
	}
	return s.result, nil
}

func (s stubOCRWorkerClient) OCRPDF(_ context.Context, _ string, _ string) (tools.OCRTextResult, error) {
	if s.err != nil {
		return tools.OCRTextResult{}, s.err
	}
	return s.result, nil
}

func (s stubMediaWorkerClient) TranscodeMedia(_ context.Context, _, _, _ string) (tools.MediaTranscodeResult, error) {
	if s.err != nil {
		return tools.MediaTranscodeResult{}, s.err
	}
	return s.transcodeResult, nil
}

func (s stubMediaWorkerClient) NormalizeRecording(_ context.Context, _, _ string) (tools.MediaTranscodeResult, error) {
	if s.err != nil {
		return tools.MediaTranscodeResult{}, s.err
	}
	return s.transcodeResult, nil
}

func (s stubMediaWorkerClient) ExtractFrames(_ context.Context, _, _ string, _ float64, _ int) (tools.MediaFrameExtractResult, error) {
	if s.err != nil {
		return tools.MediaFrameExtractResult{}, s.err
	}
	return s.framesResult, nil
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
