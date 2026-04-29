package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type stubModelClient struct {
	output                 string
	err                    error
	toolCalls              []model.ToolCallResult
	generateToolCallsCount int
	plannerInputs          []string
}

type recordingLoopRuntimeStore struct {
	runs            []storage.RunRecord
	steps           []storage.StepRecord
	events          []storage.EventRecord
	deliveryResults []storage.DeliveryResultRecord
	citationsByTask map[string][]storage.CitationRecord
}

func (s *recordingLoopRuntimeStore) SaveRun(_ context.Context, record storage.RunRecord) error {
	s.runs = append(s.runs, record)
	return nil
}

func (s *recordingLoopRuntimeStore) SaveSteps(_ context.Context, records []storage.StepRecord) error {
	s.steps = append(s.steps, records...)
	return nil
}

func (s *recordingLoopRuntimeStore) SaveEvents(_ context.Context, records []storage.EventRecord) error {
	s.events = append(s.events, records...)
	return nil
}

func (s *recordingLoopRuntimeStore) SaveDeliveryResult(_ context.Context, record storage.DeliveryResultRecord) error {
	s.deliveryResults = append(s.deliveryResults, record)
	return nil
}

func (s *recordingLoopRuntimeStore) GetRun(_ context.Context, runID string) (storage.RunRecord, error) {
	for _, record := range s.runs {
		if record.RunID == runID {
			return record, nil
		}
	}
	return storage.RunRecord{}, sql.ErrNoRows
}

func (s *recordingLoopRuntimeStore) ListDeliveryResults(_ context.Context, taskID string, limit, offset int) ([]storage.DeliveryResultRecord, int, error) {
	items := make([]storage.DeliveryResultRecord, 0, len(s.deliveryResults))
	for index := len(s.deliveryResults) - 1; index >= 0; index-- {
		record := s.deliveryResults[index]
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		items = append(items, record)
	}
	total := len(items)
	if offset >= total {
		return []storage.DeliveryResultRecord{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return append([]storage.DeliveryResultRecord(nil), items[offset:end]...), total, nil
}

func (s *recordingLoopRuntimeStore) ReplaceTaskCitations(_ context.Context, taskID string, records []storage.CitationRecord) error {
	if s.citationsByTask == nil {
		s.citationsByTask = map[string][]storage.CitationRecord{}
	}
	s.citationsByTask[taskID] = append([]storage.CitationRecord(nil), records...)
	return nil
}

func (s *recordingLoopRuntimeStore) GetLatestDeliveryResult(_ context.Context, taskID string) (storage.DeliveryResultRecord, bool, error) {
	var latest storage.DeliveryResultRecord
	found := false
	for _, record := range s.deliveryResults {
		if record.TaskID != taskID {
			continue
		}
		if !found || record.CreatedAt > latest.CreatedAt {
			latest = record
			found = true
		}
	}
	return latest, found, nil
}

func (s *recordingLoopRuntimeStore) ListTaskCitations(_ context.Context, taskID string) ([]storage.CitationRecord, error) {
	return append([]storage.CitationRecord(nil), s.citationsByTask[taskID]...), nil
}

func (s *recordingLoopRuntimeStore) ListEvents(_ context.Context, taskID, runID, eventType, createdAtFrom, createdAtTo string, limit, offset int) ([]storage.EventRecord, int, error) {
	filtered := make([]storage.EventRecord, 0, len(s.events))
	fromTime := time.Time{}
	toTime := time.Time{}
	if createdAtFrom != "" {
		fromTime, _ = time.Parse(time.RFC3339Nano, createdAtFrom)
	}
	if createdAtTo != "" {
		toTime, _ = time.Parse(time.RFC3339Nano, createdAtTo)
	}
	for _, record := range s.events {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		if eventType != "" && record.Type != eventType {
			continue
		}
		recordTime, _ := time.Parse(time.RFC3339Nano, record.CreatedAt)
		if !fromTime.IsZero() && recordTime.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && recordTime.After(toTime) {
			continue
		}
		filtered = append(filtered, record)
	}
	if offset >= len(filtered) {
		return []storage.EventRecord{}, len(filtered), nil
	}
	end := len(filtered)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return append([]storage.EventRecord(nil), filtered[offset:end]...), len(filtered), nil
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
	s.plannerInputs = append(s.plannerInputs, request.Input)
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
	storageService := newTestExecutionStorage(t)
	pluginService := plugin.NewService()
	seedTestExecutionPluginManifests(t, storageService, pluginService)

	return NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		model.NewService(cfg, &stubModelClient{output: output}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		pluginService,
	).WithArtifactStore(storageService.ArtifactStore()).WithExtensionAssetCatalog(storageService), workspaceRoot
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
	storageService := newTestExecutionStorage(t)
	pluginService := plugin.NewService()
	seedTestExecutionPluginManifests(t, storageService, pluginService)

	return NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		model.NewService(serviceconfig.ModelConfig{}, client),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		pluginService,
	).WithArtifactStore(storageService.ArtifactStore()).WithExtensionAssetCatalog(storageService), workspaceRoot
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
	storageService := newTestExecutionStorage(t)
	pluginService := plugin.NewService()
	seedTestExecutionPluginManifests(t, storageService, pluginService)

	return NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		playwright,
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		model.NewService(serviceconfig.ModelConfig{}, &stubModelClient{output: output}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		pluginService,
	).WithArtifactStore(storageService.ArtifactStore()).WithExtensionAssetCatalog(storageService), workspaceRoot
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
	storageService := newTestExecutionStorage(t)
	pluginService := plugin.NewService()
	seedTestExecutionPluginManifests(t, storageService, pluginService)

	return NewService(
		platform.NewLocalFileSystemAdapter(pathPolicy),
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		playwright,
		ocr,
		media,
		sidecarclient.NewNoopScreenCaptureClient(),
		model.NewService(serviceconfig.ModelConfig{}, &stubModelClient{output: output}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		pluginService,
	).WithArtifactStore(storageService.ArtifactStore()).WithExtensionAssetCatalog(storageService), workspaceRoot
}

func newTestExecutionStorage(t *testing.T) *storage.Service {
	t.Helper()
	service := storage.NewService(nil)
	if err := service.EnsureBuiltinExecutionAssets(context.Background()); err != nil {
		t.Fatalf("ensure builtin execution assets: %v", err)
	}
	return service
}

func seedTestExecutionPluginManifests(t *testing.T, storageService *storage.Service, pluginService *plugin.Service) {
	t.Helper()
	runtimeNamesByPluginID := map[string][]string{}
	for _, runtime := range pluginService.RuntimeStates() {
		if runtime.Manifest == nil || runtime.Manifest.PluginID == "" {
			continue
		}
		runtimeNamesByPluginID[runtime.Manifest.PluginID] = append(runtimeNamesByPluginID[runtime.Manifest.PluginID], runtime.Name)
	}
	for _, manifest := range pluginService.Manifests() {
		capabilitiesJSON, err := json.Marshal(manifest.Capabilities)
		if err != nil {
			t.Fatalf("marshal plugin capabilities: %v", err)
		}
		permissionsJSON, err := json.Marshal(manifest.Permissions)
		if err != nil {
			t.Fatalf("marshal plugin permissions: %v", err)
		}
		runtimeNamesJSON, err := json.Marshal(runtimeNamesByPluginID[manifest.PluginID])
		if err != nil {
			t.Fatalf("marshal plugin runtime names: %v", err)
		}
		if err := storageService.PluginManifestStore().WritePluginManifest(context.Background(), storage.PluginManifestRecord{
			PluginID:         manifest.PluginID,
			Name:             manifest.Name,
			Version:          manifest.Version,
			Entry:            manifest.Entry,
			Source:           manifest.Source,
			Summary:          "test manifest",
			CapabilitiesJSON: string(capabilitiesJSON),
			PermissionsJSON:  string(permissionsJSON),
			RuntimeNamesJSON: string(runtimeNamesJSON),
			CreatedAt:        time.Now().UTC().Format(time.RFC3339),
			UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("write plugin manifest: %v", err)
		}
	}
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

func TestExecuteAgentLoopRetriesFalseCapabilityDenialBeforeCallingTool(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{
			{
				RequestID:  "req_loop_capability_retry_1",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I cannot access workspace files in this environment.",
			},
			{
				RequestID: "req_loop_capability_retry_2",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "read_file", Arguments: map[string]any{"path": "notes/source.txt"}}},
			},
			{
				RequestID:  "req_loop_capability_retry_3",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "I checked the file after the retry reminder.",
			},
		},
	}
	service, workspaceRoot := newTestExecutionServiceWithModelClient(t, modelClient)
	sourcePath := filepath.Join(workspaceRoot, "notes", "source.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("Important retry note"), 0o644); err != nil {
		t.Fatalf("seed source file: %v", err)
	}

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_capability_retry",
		RunID:        "run_loop_capability_retry",
		Title:        "Loop capability retry test",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Please inspect the note and tell me the takeaway."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop result",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.Content != "I checked the file after the retry reminder." {
		t.Fatalf("unexpected loop output: %s", result.Content)
	}
	if modelClient.generateToolCallsCount != 3 {
		t.Fatalf("expected three planning turns after capability retry, got %d", modelClient.generateToolCallsCount)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolName != "read_file" {
		t.Fatalf("expected one read_file tool call after capability retry, got %+v", result.ToolCalls)
	}
	if len(modelClient.plannerInputs) < 2 {
		t.Fatalf("expected planner inputs for retry flow, got %+v", modelClient.plannerInputs)
	}
	if !strings.Contains(modelClient.plannerInputs[0], "Available tools:") || !strings.Contains(modelClient.plannerInputs[0], "- read_file") {
		t.Fatalf("expected first planner input to expose runtime capabilities, got %q", modelClient.plannerInputs[0])
	}
	if !strings.Contains(modelClient.plannerInputs[1], "Capability reminder:") {
		t.Fatalf("expected second planner input to include capability reminder, got %q", modelClient.plannerInputs[1])
	}
	if !strings.Contains(modelClient.plannerInputs[1], "The listed tools are available in this run.") {
		t.Fatalf("expected second planner input to restate tool availability, got %q", modelClient.plannerInputs[1])
	}
	if result.ModelInvocation["request_id"] != "req_loop_capability_retry_3" {
		t.Fatalf("expected final model invocation metadata, got %+v", result.ModelInvocation)
	}
}

func TestExecuteAgentLoopRetriesFalseWebCapabilityDenialsBeforeCallingTool(t *testing.T) {
	tests := []struct {
		name           string
		inputText      string
		capabilityLine string
		wantTool       string
		wantOutput     string
		playwright     stubPlaywrightClient
		toolCalls      []model.ToolCallResult
	}{
		{
			name:           "page_read",
			inputText:      "Please inspect https://example.com and summarize the page.",
			capabilityLine: "- page_read",
			wantTool:       "page_read",
			wantOutput:     "I checked the page after the retry reminder.",
			playwright: stubPlaywrightClient{readResult: tools.BrowserPageReadResult{
				Title:       "Example Page",
				TextContent: "example page content",
				Source:      "playwright_sidecar",
			}},
			toolCalls: []model.ToolCallResult{
				{
					RequestID:  "req_loop_page_read_retry_1",
					Provider:   "openai_responses",
					ModelID:    "gpt-5.4",
					OutputText: "I cannot browse websites from here.",
				},
				{
					RequestID: "req_loop_page_read_retry_2",
					Provider:  "openai_responses",
					ModelID:   "gpt-5.4",
					ToolCalls: []model.ToolInvocation{{Name: "page_read", Arguments: map[string]any{"url": "https://example.com"}}},
				},
				{
					RequestID:  "req_loop_page_read_retry_3",
					Provider:   "openai_responses",
					ModelID:    "gpt-5.4",
					OutputText: "I checked the page after the retry reminder.",
				},
			},
		},
		{
			name:           "page_search",
			inputText:      "Please search https://example.com for the word example.",
			capabilityLine: "- page_search",
			wantTool:       "page_search",
			wantOutput:     "I searched the page after the retry reminder.",
			playwright: stubPlaywrightClient{searchResult: tools.BrowserPageSearchResult{
				Matches:    []string{"example result"},
				MatchCount: 1,
				Source:     "playwright_sidecar",
			}},
			toolCalls: []model.ToolCallResult{
				{
					RequestID:  "req_loop_page_search_retry_1",
					Provider:   "openai_responses",
					ModelID:    "gpt-5.4",
					OutputText: "I cannot browse websites from here.",
				},
				{
					RequestID: "req_loop_page_search_retry_2",
					Provider:  "openai_responses",
					ModelID:   "gpt-5.4",
					ToolCalls: []model.ToolInvocation{{Name: "page_search", Arguments: map[string]any{"url": "https://example.com", "query": "example"}}},
				},
				{
					RequestID:  "req_loop_page_search_retry_3",
					Provider:   "openai_responses",
					ModelID:    "gpt-5.4",
					OutputText: "I searched the page after the retry reminder.",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modelClient := &stubModelClient{toolCalls: append([]model.ToolCallResult(nil), test.toolCalls...)}
			service, _ := newTestExecutionServiceWithPlaywright(t, "unused", test.playwright)
			service = service.ReplaceModel(model.NewService(serviceconfig.ModelConfig{}, modelClient))

			result, err := service.Execute(context.Background(), Request{
				TaskID:               "task_loop_" + test.name + "_retry",
				RunID:                "run_loop_" + test.name + "_retry",
				Title:                "Loop " + test.name + " retry test",
				Intent:               map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
				Snapshot:             contextsvc.TaskContextSnapshot{InputType: "text", Text: test.inputText},
				DeliveryType:         "bubble",
				ResultTitle:          "Loop result",
				ApprovalGranted:      true,
				ApprovedOperation:    test.wantTool,
				ApprovedTargetObject: "https://example.com",
			})
			if err != nil {
				t.Fatalf("execute failed: %v", err)
			}

			if result.Content != test.wantOutput {
				t.Fatalf("unexpected loop output: %s", result.Content)
			}
			if modelClient.generateToolCallsCount != 3 {
				t.Fatalf("expected three planning turns after capability retry, got %d", modelClient.generateToolCallsCount)
			}
			if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolName != test.wantTool {
				t.Fatalf("expected one %s tool call after capability retry, got %+v", test.wantTool, result.ToolCalls)
			}
			if result.ToolCalls[0].Output["loop_round"] != 2 {
				t.Fatalf("expected retry-selected tool call to run on round 2, got %+v", result.ToolCalls[0].Output)
			}
			if len(modelClient.plannerInputs) < 2 {
				t.Fatalf("expected planner inputs for retry flow, got %+v", modelClient.plannerInputs)
			}
			if !strings.Contains(modelClient.plannerInputs[0], "Available tools:") || !strings.Contains(modelClient.plannerInputs[0], test.capabilityLine) {
				t.Fatalf("expected first planner input to expose runtime capabilities, got %q", modelClient.plannerInputs[0])
			}
			if !strings.Contains(modelClient.plannerInputs[1], "Capability reminder:") {
				t.Fatalf("expected second planner input to include capability reminder, got %q", modelClient.plannerInputs[1])
			}
			if !strings.Contains(modelClient.plannerInputs[1], "The listed tools are available in this run.") || !strings.Contains(modelClient.plannerInputs[1], test.capabilityLine) {
				t.Fatalf("expected second planner input to restate tool availability, got %q", modelClient.plannerInputs[1])
			}
			if result.ModelInvocation["request_id"] != test.toolCalls[2].RequestID {
				t.Fatalf("expected final model invocation metadata, got %+v", result.ModelInvocation)
			}
		})
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
	events, total, err := loopStore.ListEvents(context.Background(), "task_loop_runtime", "", "", "", "", 20, 0)
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

func TestExecuteAgentLoopRequestsClarificationWhenPlannerReturnsEmptyOutput(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{{
			RequestID:  "req_loop_need_input",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "",
		}},
	}
	service, _ := newTestExecutionServiceWithModelClient(t, modelClient)

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_need_input",
		RunID:        "run_loop_need_input",
		Title:        "Loop need input",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "你好"},
		DeliveryType: "bubble",
		ResultTitle:  "Loop clarification",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.LoopStopReason != string(agentloop.StopReasonNeedUserInput) {
		t.Fatalf("expected need_user_input stop reason, got %+v", result)
	}
	if !strings.Contains(result.Content, "请补充你的目标") {
		t.Fatalf("expected clarification fallback output, got %+v", result)
	}
	if !strings.Contains(result.BubbleText, "请补充你的目标") {
		t.Fatalf("expected clarification bubble text, got %+v", result)
	}
}

func TestPersistAgentLoopRuntimeKeepsResumeSegmentsAndEventsDistinct(t *testing.T) {
	store := &recordingLoopRuntimeStore{}
	service := (&Service{}).WithLoopRuntimeStore(store)

	request := Request{
		TaskID: "task_loop_runtime",
		RunID:  "run_loop_runtime",
		Intent: map[string]any{"name": defaultAgentLoopIntentName},
	}
	initialStart := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	firstResumeStart := initialStart.Add(2 * time.Minute)
	secondResumeStart := firstResumeStart.Add(2 * time.Minute)

	for _, segment := range []struct {
		kind    string
		started time.Time
	}{
		{kind: "initial", started: initialStart},
		{kind: "resume", started: firstResumeStart},
		{kind: "resume", started: secondResumeStart},
	} {
		completedAt := segment.started.Add(15 * time.Second)
		service.persistAgentLoopRuntime(request, agentloop.Result{
			StopReason: agentloop.StopReasonCompleted,
			Rounds: []agentloop.PersistedRound{{
				StepID:        "step_loop_01",
				RunID:         request.RunID,
				TaskID:        request.TaskID,
				AttemptIndex:  1,
				SegmentKind:   segment.kind,
				LoopRound:     1,
				Name:          "agent_loop_round",
				Status:        "completed",
				InputSummary:  "planner input",
				OutputSummary: "planner output",
				StartedAt:     segment.started,
				CompletedAt:   completedAt,
				StopReason:    agentloop.StopReasonCompleted,
			}},
			Events: []agentloop.LifecycleEvent{{
				Type:      "loop.round.completed",
				Level:     "info",
				StepID:    "step_loop_01",
				Payload:   map[string]any{"attempt_index": 1, "segment_kind": segment.kind, "loop_round": 1},
				CreatedAt: completedAt,
			}},
		})
	}

	if len(store.steps) != 3 {
		t.Fatalf("expected three persisted step rows, got %+v", store.steps)
	}
	if len(store.events) != 3 {
		t.Fatalf("expected three persisted event rows, got %+v", store.events)
	}

	seenStepIDs := map[string]struct{}{}
	for _, record := range store.steps {
		if _, exists := seenStepIDs[record.StepID]; exists {
			t.Fatalf("expected unique step ids across persisted segments, got duplicate %q", record.StepID)
		}
		seenStepIDs[record.StepID] = struct{}{}
	}

	seenEventIDs := map[string]struct{}{}
	for index, record := range store.events {
		if _, exists := seenEventIDs[record.EventID]; exists {
			t.Fatalf("expected unique event ids across persisted segments, got duplicate %q", record.EventID)
		}
		seenEventIDs[record.EventID] = struct{}{}
		if record.StepID != store.steps[index].StepID {
			t.Fatalf("expected event step id %q to link to persisted step %q", record.StepID, store.steps[index].StepID)
		}
	}
}

func TestExecuteAgentLoopPersistsPlannerErrors(t *testing.T) {
	modelClient := &stubModelClient{err: errors.New("planner unavailable")}
	loopStore := storage.NewService(nil).LoopRuntimeStore()
	service, _ := newTestExecutionServiceWithModelClient(t, modelClient)
	notifications := []string{}
	service = service.WithLoopRuntimeStore(loopStore).WithNotificationEmitter(func(_ string, method string, _ map[string]any) {
		notifications = append(notifications, method)
	})

	_, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_planner_error",
		RunID:        "run_loop_planner_error",
		Title:        "Loop planner error persistence",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the workspace and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop runtime result",
	})
	if err == nil {
		t.Fatal("expected planner error to surface")
	}
	events, total, listErr := loopStore.ListEvents(context.Background(), "task_loop_planner_error", "", "", "", "", 20, 0)
	if listErr != nil {
		t.Fatalf("ListEvents returned error: %v", listErr)
	}
	if total == 0 || len(events) == 0 {
		t.Fatal("expected persisted loop planner error events")
	}
	foundFailed := false
	for _, event := range events {
		if event.Type == "loop.failed" && strings.Contains(event.PayloadJSON, "planner_error") {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Fatalf("expected loop.failed planner_error event in %+v", events)
	}
	if len(notifications) == 0 {
		t.Fatal("expected runtime notifications for planner error")
	}
}

func TestExecuteAgentLoopRetriesPlannerOnceBeforeFailing(t *testing.T) {
	modelClient := &stubModelClient{err: model.ErrOpenAIRequestTimeout}
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
	if modelClient.generateToolCallsCount != 2 {
		t.Fatalf("expected one retry for timeout planner error, got %d", modelClient.generateToolCallsCount)
	}
}

func TestExecuteBudgetDowngradeFallsBackWhenModelClientUnavailable(t *testing.T) {
	service, _ := newTestExecutionServiceWithModelClient(t, nil)
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_budget_fallback_prompt",
		RunID:        "run_budget_fallback_prompt",
		Title:        "Budget fallback prompt",
		Intent:       map[string]any{"name": "summarize", "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Please summarize this content."},
		DeliveryType: "bubble",
		ResultTitle:  "Budget fallback result",
		BudgetDowngrade: map[string]any{
			"applied":         true,
			"trigger_reason":  "provider_unavailable",
			"degrade_actions": []string{"lightweight_delivery"},
			"summary":         "Budget downgrade fallback applied.",
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result.DeliveryResult["type"] != "bubble" {
		t.Fatalf("expected budget fallback to keep bubble delivery, got %+v", result.DeliveryResult)
	}
	if !strings.Contains(result.Content, "Budget downgrade fallback applied.") {
		t.Fatalf("expected fallback content to include budget downgrade summary, got %q", result.Content)
	}
	if result.ModelInvocation["provider"] != "budget_downgrade_fallback" || result.ModelInvocation["fallback"] != true {
		t.Fatalf("expected fallback model invocation marker, got %+v", result.ModelInvocation)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolName != "generate_text" {
		t.Fatalf("expected fallback execution to preserve generate_text tool call, got %+v", result.ToolCalls)
	}
	if result.ToolCalls[0].Output["token_usage"] == nil {
		t.Fatalf("expected fallback execution to preserve token usage trace in tool call output, got %+v", result.ToolCalls[0].Output)
	}
	if result.BudgetFailure == nil || result.BudgetFailure["action"] != "budget_auto_downgrade.failure_signal" {
		t.Fatalf("expected fallback execution to expose budget failure signal, got %+v", result.BudgetFailure)
	}
}

func TestExecuteBudgetDowngradeAllowsReadOnlyAgentLoopTools(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{{
			RequestID: "req_loop_readonly_allowed",
			Provider:  "openai_responses",
			ModelID:   "gpt-5.4",
			ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "."}}},
		}, {
			RequestID:  "req_loop_readonly_answer",
			Provider:   "openai_responses",
			ModelID:    "gpt-5.4",
			OutputText: "Read-only loop completed without downgrade fallback.",
		}},
	}
	service, _ := newTestExecutionServiceWithModelClient(t, modelClient)
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_budget_allow_readonly_loop",
		RunID:        "run_budget_allow_readonly_loop",
		Title:        "Budget allow readonly loop",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the workspace and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Budget readonly loop result",
		BudgetDowngrade: map[string]any{
			"applied":         true,
			"trigger_reason":  "failure_pressure",
			"degrade_actions": []string{"skip_expensive_tools", "lightweight_delivery"},
			"summary":         "Budget downgrade fallback applied.",
			"trace": map[string]any{
				"expensive_tool_categories": []string{"command", "browser_mutation", "media_heavy"},
			},
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if modelClient.generateToolCallsCount == 0 {
		t.Fatalf("expected read-only agent loop tools to remain available under downgrade")
	}
	if result.ModelInvocation["provider"] == "budget_downgrade_fallback" {
		t.Fatalf("expected read-only loop to avoid hard fallback when cheap tools are allowed, got %+v", result.ModelInvocation)
	}
}

func TestExecuteBudgetDowngradeBlocksExpensiveDirectToolPath(t *testing.T) {
	service, _ := newTestExecutionService(t, "executor-backed direct tool budget fallback")
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_budget_direct_tool",
		RunID:        "run_budget_direct_tool",
		Title:        "Budget direct tool fallback",
		Intent:       map[string]any{"name": "exec_command", "arguments": map[string]any{"command": "dir"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Run an expensive command."},
		DeliveryType: "bubble",
		ResultTitle:  "Budget direct tool result",
		BudgetDowngrade: map[string]any{
			"applied":         true,
			"trigger_reason":  "failure_pressure",
			"degrade_actions": []string{"skip_expensive_tools", "lightweight_delivery"},
			"summary":         "Budget downgrade fallback applied.",
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result.ToolName == "exec_command" {
		t.Fatalf("expected direct expensive tool path to be blocked by budget downgrade, got %+v", result)
	}
	if result.ModelInvocation["provider"] == "budget_downgrade_fallback" {
		t.Fatalf("expected blocked direct tool path to fall back to lightweight generation before hard budget fallback, got %+v", result.ModelInvocation)
	}
	if strings.Contains(result.Content, "Budget downgrade fallback applied.") {
		t.Fatalf("expected blocked direct tool path to avoid hard fallback when lightweight generation succeeds, got %q", result.Content)
	}
}

func TestExecuteBudgetDowngradeFallbackCarriesStructuredTrace(t *testing.T) {
	service, _ := newTestExecutionServiceWithModelClient(t, nil)
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_budget_fallback_trace",
		RunID:        "run_budget_fallback_trace",
		Title:        "Budget fallback trace",
		Intent:       map[string]any{"name": "summarize", "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Please summarize this content."},
		DeliveryType: "bubble",
		ResultTitle:  "Budget fallback trace result",
		BudgetDowngrade: map[string]any{
			"applied":         true,
			"trigger_reason":  "failure_pressure",
			"degrade_actions": []string{"skip_expensive_tools", "lightweight_delivery", "shrink_context"},
			"summary":         "Budget downgrade fallback applied.",
			"trace": map[string]any{
				"failure_signal_window":     2,
				"expensive_tool_categories": []string{"command", "browser_mutation", "media_heavy"},
			},
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result.ModelInvocation["reason"] != "failure_pressure" {
		t.Fatalf("expected fallback invocation reason to remain structured, got %+v", result.ModelInvocation)
	}
}

func TestExecuteBudgetDowngradePreservesFallbackReason(t *testing.T) {
	service, _ := newTestExecutionServiceWithModelClient(t, nil)
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_budget_fallback_reason",
		RunID:        "run_budget_fallback_reason",
		Title:        "Budget fallback reason",
		Intent:       map[string]any{"name": "summarize", "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Please summarize this content."},
		DeliveryType: "bubble",
		ResultTitle:  "Budget fallback reason result",
		BudgetDowngrade: map[string]any{
			"applied":         true,
			"trigger_reason":  "provider_unavailable",
			"degrade_actions": []string{"lightweight_delivery"},
			"summary":         "Budget downgrade fallback applied.",
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result.BudgetFailure == nil || result.BudgetFailure["reason"] != model.ErrClientNotConfigured.Error() {
		t.Fatalf("expected budget failure reason to preserve actual fallback reason, got %+v", result.BudgetFailure)
	}
}

func TestExecuteAgentLoopHonorsConfiguredPlannerRetryBudget(t *testing.T) {
	modelClient := &stubModelClient{err: model.ErrOpenAIRequestTimeout}
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

func TestExecuteAgentLoopDoesNotRetryNonRetryablePlannerErrors(t *testing.T) {
	modelClient := &stubModelClient{err: &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "bad request"}}
	cfg := serviceconfig.ModelConfig{PlannerRetryBudget: 2}
	service, _ := newTestExecutionServiceWithModelClientAndConfig(t, cfg, modelClient)
	_, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_non_retryable_planner",
		RunID:        "run_loop_non_retryable_planner",
		Title:        "Loop non-retryable planner",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the workspace and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop non-retryable result",
	})
	if err == nil {
		t.Fatal("expected non-retryable planner error to surface")
	}
	if modelClient.generateToolCallsCount != 1 {
		t.Fatalf("expected non-retryable planner error to stop after one attempt, got %d", modelClient.generateToolCallsCount)
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
	if len(modelClient.plannerInputs) < 2 {
		t.Fatalf("expected planner inputs for both rounds, got %+v", modelClient.plannerInputs)
	}
	if !strings.Contains(modelClient.plannerInputs[1], "Also include the newly added summary section.") {
		t.Fatalf("expected second planner input to include active steering, got %q", modelClient.plannerInputs[1])
	}
	if !strings.Contains(modelClient.plannerInputs[1], "Inspect the notes directory and answer.") {
		t.Fatalf("expected second planner input to preserve original input, got %q", modelClient.plannerInputs[1])
	}
}

func TestExecuteAgentLoopAppendsMultipleActiveSteeringMessages(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{
			{
				RequestID: "req_loop_multi_1",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			},
			{
				RequestID: "req_loop_multi_2",
				Provider:  "openai_responses",
				ModelID:   "gpt-5.4",
				ToolCalls: []model.ToolInvocation{{Name: "list_dir", Arguments: map[string]any{"path": "notes"}}},
			},
			{
				RequestID:  "req_loop_multi_3",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Loop runtime finished after multiple steering updates.",
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
		switch pollCount {
		case 2:
			return []string{"Keep the original checklist format."}
		case 3:
			return []string{"Also include the new summary section."}
		default:
			return nil
		}
	})

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_loop_multi_steer",
		RunID:        "run_loop_multi_steer",
		Title:        "Loop multiple steering",
		Intent:       map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the notes directory and answer."},
		DeliveryType: "bubble",
		ResultTitle:  "Loop result",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Content != "Loop runtime finished after multiple steering updates." {
		t.Fatalf("unexpected multi-steering result: %+v", result)
	}
	if len(modelClient.plannerInputs) < 3 {
		t.Fatalf("expected planner inputs for three rounds, got %+v", modelClient.plannerInputs)
	}
	if !strings.Contains(modelClient.plannerInputs[2], "Keep the original checklist format.") {
		t.Fatalf("expected final planner input to keep first steering message, got %q", modelClient.plannerInputs[2])
	}
	if !strings.Contains(modelClient.plannerInputs[2], "Also include the new summary section.") {
		t.Fatalf("expected final planner input to include second steering message, got %q", modelClient.plannerInputs[2])
	}
}

func TestExecuteAgentLoopDoesNotDuplicateQueuedSteeringOnFirstRound(t *testing.T) {
	modelClient := &stubModelClient{
		toolCalls: []model.ToolCallResult{
			{
				RequestID:  "req_loop_queued_once",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: "Loop runtime finished with queued steering.",
			},
		},
	}
	service, _ := newTestExecutionServiceWithModelClient(t, modelClient)
	service = service.WithSteeringPoller(func(_ string) []string {
		return []string{"Keep the answer concise."}
	})

	result, err := service.Execute(context.Background(), Request{
		TaskID:           "task_loop_queued_steer",
		RunID:            "run_loop_queued_steer",
		Title:            "Loop queued steering",
		Intent:           map[string]any{"name": defaultAgentLoopIntentName, "arguments": map[string]any{}},
		Snapshot:         contextsvc.TaskContextSnapshot{InputType: "text", Text: "Inspect the task and answer."},
		SteeringMessages: []string{"Keep the answer concise."},
		DeliveryType:     "bubble",
		ResultTitle:      "Loop result",
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Content != "Loop runtime finished with queued steering." {
		t.Fatalf("unexpected queued steering result: %+v", result)
	}
	if len(modelClient.plannerInputs) != 1 {
		t.Fatalf("expected a single planner input, got %+v", modelClient.plannerInputs)
	}
	if count := strings.Count(modelClient.plannerInputs[0], "Keep the answer concise."); count != 1 {
		t.Fatalf("expected queued steering to appear once in the first planner input, got %d occurrences in %q", count, modelClient.plannerInputs[0])
	}
}

func TestRunStatusFromStopReasonTreatsToolRetryExhaustedAsFailed(t *testing.T) {
	if status := runStatusFromStopReason(agentloop.StopReasonToolRetryExhausted); status != "failed" {
		t.Fatalf("expected tool retry exhausted to map to failed, got %q", status)
	}
}

func TestRunStatusFromStopReasonTreatsNeedUserInputAsWaitingInput(t *testing.T) {
	if status := runStatusFromStopReason(agentloop.StopReasonNeedUserInput); status != "waiting_input" {
		t.Fatalf("expected need_user_input to map to waiting_input, got %q", status)
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
		sidecarclient.NewNoopScreenCaptureClient(),
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
	if len(result.ExtensionAssets) < 4 {
		t.Fatalf("expected static execution assets plus plugin manifest refs, got %+v", result.ExtensionAssets)
	}
	foundPluginManifest := false
	for _, asset := range result.ExtensionAssets {
		if asset["asset_kind"] == storage.ExtensionAssetKindPluginManifest && asset["asset_id"] == "playwright" {
			foundPluginManifest = true
			break
		}
	}
	if !foundPluginManifest {
		t.Fatalf("expected page_read execution to attribute playwright plugin manifest, got %+v", result.ExtensionAssets)
	}
	if refs, ok := result.ModelInvocation["extension_asset_refs"].([]map[string]any); !ok || len(refs) != len(result.ExtensionAssets) {
		t.Fatalf("expected model invocation to expose extension asset refs, got %+v", result.ModelInvocation)
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
		sidecarclient.NewNoopScreenCaptureClient(),
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

func TestGenerateOutputWithPromptRejectsFallbackWithoutBudgetDowngrade(t *testing.T) {
	service, _ := newTestExecutionServiceWithModelClient(t, &stubModelClient{output: "   "})
	_, err := service.generateOutputWithPrompt(context.Background(), Request{
		TaskID:      "task_empty_model_output",
		RunID:       "run_empty_model_output",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		ResultTitle: "Empty model output",
	}, "Please summarize this content.")
	if !errors.Is(err, tools.ErrToolOutputInvalid) {
		t.Fatalf("expected empty model output to surface TOOL_OUTPUT_INVALID, got %v", err)
	}
}

func TestScreenCapabilitySnapshotReportsWiringState(t *testing.T) {
	service, _ := newTestExecutionService(t, "screen capability probe")
	snapshot := service.ScreenCapabilitySnapshot()
	if !snapshot.Available {
		t.Fatalf("expected noop screen capability to be wired, got %+v", snapshot)
	}
	if len(snapshot.CaptureModes) != 3 {
		t.Fatalf("expected three capture modes, got %+v", snapshot)
	}

	service.screen = nil
	snapshot = service.ScreenCapabilitySnapshot()
	if snapshot.Available || len(snapshot.CaptureModes) != 0 {
		t.Fatalf("expected nil screen capability to report unavailable, got %+v", snapshot)
	}
}

func TestScreenLifecycleReadyReportsLifecycleManagerWiring(t *testing.T) {
	service, _ := newTestExecutionService(t, "screen lifecycle probe")
	if !service.ScreenLifecycleReady() {
		t.Fatal("expected lifecycle manager to be wired")
	}
	service.lifecycle = nil
	if service.ScreenLifecycleReady() {
		t.Fatal("expected nil lifecycle manager to report unavailable")
	}
}

func TestBuildScreenObservationFlowSucceeds(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_001/frame_001.png", Text: "fatal error at line 3", Language: "eng", Source: "ocr_worker_text"}}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	flow, err := service.buildScreenObservationFlow(context.Background(), "task_screen_002", tools.ScreenFrameCandidate{
		FrameID:         "frame_001",
		ScreenSessionID: "screen_sess_001",
		CaptureMode:     tools.ScreenCaptureModeScreenshot,
		Source:          "voice",
		Path:            "temp/screen_sess_001/frame_001.png",
		CapturedAt:      time.Date(2026, 4, 18, 19, 0, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
	}, "eng", "error_evidence", map[string]any{"region_count": 1})
	if err != nil {
		t.Fatalf("buildScreenObservationFlow returned error: %v", err)
	}
	if flow.OCRInput["path"] != "temp/screen_sess_001/frame_001.png" || flow.OCRResult.Text == "" {
		t.Fatalf("unexpected OCR bridge result: %+v", flow)
	}
	if flow.ObservationSeed["frame_id"] != "frame_001" || flow.Artifact["artifact_type"] != "screen_capture" {
		t.Fatalf("unexpected observation flow result: %+v", flow)
	}
}

func TestBuildScreenObservationFlowUsesMediaFramesForClip(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_clip/frame-001.jpg", Text: "clip extracted frame shows error banner", Language: "eng", Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{framesResult: tools.MediaFrameExtractResult{InputPath: "temp/screen_sess_clip/clip.webm", OutputDir: "temp/screen_sess_clip/frame_extract", FramePaths: []string{"temp/screen_sess_clip/frame_extract/frame-001.jpg"}, FrameCount: 1, Source: "media_worker_frames"}}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	flow, err := service.buildScreenObservationFlow(context.Background(), "task_screen_clip_001", tools.ScreenFrameCandidate{
		FrameID:         "frame_clip_001",
		ScreenSessionID: "screen_sess_clip",
		CaptureMode:     tools.ScreenCaptureModeClip,
		Source:          "voice",
		Path:            "temp/screen_sess_clip/clip.webm",
		CapturedAt:      time.Date(2026, 4, 19, 9, 0, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
		CleanupRequired: true,
	}, "eng", "error_evidence", nil)
	if err != nil {
		t.Fatalf("buildScreenObservationFlow returned error: %v", err)
	}
	if flow.OCRInput["path"] != "temp/screen_sess_clip/frame_extract/frame-001.jpg" {
		t.Fatalf("expected clip OCR input to use extracted frame, got %+v", flow.OCRInput)
	}
	if flow.ObservationSeed["analyzed_path"] != "temp/screen_sess_clip/frame_extract/frame-001.jpg" || flow.ObservationSeed["clip_frame_count"] != 1 {
		t.Fatalf("expected clip observation patch, got %+v", flow.ObservationSeed)
	}
	if len(flow.CleanupPaths) != 3 || flow.CleanupPaths[0] != "temp/screen_sess_clip/clip_normalized.mp4" || flow.CleanupPaths[1] != "temp/screen_sess_clip/frame_extract/frame-001.jpg" || flow.CleanupPaths[2] != "temp/screen_sess_clip/frame_extract" {
		t.Fatalf("expected clip cleanup paths to include extracted frame, got %+v", flow.CleanupPaths)
	}
}

func TestBuildScreenObservationFlowReturnsOCRFailure(t *testing.T) {
	ocrStub := stubOCRWorkerClient{err: tools.ErrOCRWorkerFailed}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	_, err := service.buildScreenObservationFlow(context.Background(), "task_screen_003", tools.ScreenFrameCandidate{
		FrameID:         "frame_002",
		ScreenSessionID: "screen_sess_002",
		CaptureMode:     tools.ScreenCaptureModeScreenshot,
		Source:          "voice",
		Path:            "temp/screen_sess_002/frame_002.png",
		CapturedAt:      time.Date(2026, 4, 18, 19, 30, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
	}, "eng", "error_evidence", nil)
	if !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected OCR worker failure, got %v", err)
	}
}

func TestBuildScreenObservationFlowReturnsClipFramePreparationFailure(t *testing.T) {
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), sidecarclient.NewNoopOCRWorkerClient(), stubMediaWorkerClient{err: tools.ErrMediaWorkerFailed})
	_, err := service.buildScreenObservationFlow(context.Background(), "task_screen_clip_fail", tools.ScreenFrameCandidate{
		FrameID:         "frame_clip_fail",
		ScreenSessionID: "screen_sess_clip_fail",
		CaptureMode:     tools.ScreenCaptureModeClip,
		Source:          "voice",
		Path:            "temp/screen_sess_clip_fail/clip.webm",
		CapturedAt:      time.Date(2026, 4, 19, 9, 30, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
		CleanupRequired: true,
	}, "eng", "error_evidence", nil)
	if !errors.Is(err, tools.ErrMediaWorkerFailed) {
		t.Fatalf("expected clip frame extraction failure to map to media worker error, got %v", err)
	}
}

func TestPrepareScreenOCRInputCoversClipValidationBranches(t *testing.T) {
	t.Run("requires media worker for clip mode", func(t *testing.T) {
		service, _ := newTestExecutionService(t, "unused")
		service.media = nil
		_, err := service.prepareScreenOCRInput(context.Background(), tools.ScreenFrameCandidate{
			FrameID:         "frame_clip_nomedia",
			ScreenSessionID: "screen_sess_clip_nomedia",
			CaptureMode:     tools.ScreenCaptureModeClip,
			Path:            "temp/screen_sess_clip_nomedia/clip.webm",
		}, "eng")
		if !errors.Is(err, tools.ErrMediaWorkerFailed) {
			t.Fatalf("expected clip OCR prep to require media worker, got %v", err)
		}
	})

	t.Run("rejects empty normalized output path", func(t *testing.T) {
		mediaStub := stubMediaWorkerClient{transcodeResult: tools.MediaTranscodeResult{InputPath: "temp/screen_sess_clip_empty/clip.webm"}}
		service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), sidecarclient.NewNoopOCRWorkerClient(), mediaStub)
		_, err := service.prepareScreenOCRInput(context.Background(), tools.ScreenFrameCandidate{
			FrameID:         "frame_clip_empty",
			ScreenSessionID: "screen_sess_clip_empty",
			CaptureMode:     tools.ScreenCaptureModeClip,
			Path:            "   ",
		}, "eng")
		if !errors.Is(err, tools.ErrToolOutputInvalid) {
			t.Fatalf("expected empty normalized path to be rejected, got %v", err)
		}
	})

	t.Run("captures clip metadata and helper fallbacks", func(t *testing.T) {
		mediaStub := stubMediaWorkerClient{
			transcodeResult: tools.MediaTranscodeResult{InputPath: ".webm", OutputPath: ".webm", Source: ""},
			framesResult:    tools.MediaFrameExtractResult{InputPath: "clip_normalized.mp4", OutputDir: "", FramePaths: []string{"clip_frames/frame-001.jpg"}, FrameCount: 1, Source: "media_worker_frames"},
		}
		service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), sidecarclient.NewNoopOCRWorkerClient(), mediaStub)
		prepared, err := service.prepareScreenOCRInput(context.Background(), tools.ScreenFrameCandidate{
			FrameID:         "frame_clip_metadata",
			ScreenSessionID: "screen_sess_clip_metadata",
			CaptureMode:     tools.ScreenCaptureModeClip,
			Path:            ".webm",
			CleanupRequired: true,
		}, "")
		if err != nil {
			t.Fatalf("prepareScreenOCRInput returned error: %v", err)
		}
		if prepared.ObservationPatch["normalized_format"] != "mp4" || prepared.ObservationPatch["media_source"] != "media_worker_frames" {
			t.Fatalf("expected clip OCR prep to fill normalized metadata defaults, got %+v", prepared.ObservationPatch)
		}
		if prepared.ObservationPatch["clip_path"] != ".webm" || prepared.ObservationPatch["frame_output_dir"] != "clip_frames" {
			t.Fatalf("expected clip OCR prep to keep clip and frame output metadata, got %+v", prepared.ObservationPatch)
		}
		if _, ok := prepared.Input["language"]; ok {
			t.Fatalf("expected blank language to stay omitted, got %+v", prepared.Input)
		}
		if got := clipNormalizedOutputPath(".webm"); got != "clip_normalized.mp4" {
			t.Fatalf("expected clipNormalizedOutputPath fallback name, got %q", got)
		}
		if got := clipFrameOutputDir(".webm"); got != "clip_frames" {
			t.Fatalf("expected clipFrameOutputDir fallback name, got %q", got)
		}
		if clipNormalizedOutputPath("") != "" || clipFrameOutputDir("") != "" {
			t.Fatal("expected clip path helpers to tolerate blank input")
		}
		if screenAnalysisCleanupReason(tools.ScreenFrameCandidate{CaptureMode: tools.ScreenCaptureModeScreenshot}) != "screen_analysis_pending_cleanup" {
			t.Fatal("expected screenshot cleanup reason fallback")
		}
	})
}

func TestBuildScreenObservationFlowRejectsInvalidClipFramePath(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "../outside/frame-001.jpg", Text: "clip extracted frame shows error banner", Language: "eng", Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{framesResult: tools.MediaFrameExtractResult{InputPath: "temp/screen_sess_clip_invalid/clip.webm", OutputDir: "temp/screen_sess_clip_invalid/frame_extract", FramePaths: []string{"../outside/frame-001.jpg"}, FrameCount: 1, Source: "media_worker_frames"}}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	_, err := service.buildScreenObservationFlow(context.Background(), "task_screen_clip_invalid", tools.ScreenFrameCandidate{
		FrameID:         "frame_clip_invalid",
		ScreenSessionID: "screen_sess_clip_invalid",
		CaptureMode:     tools.ScreenCaptureModeClip,
		Source:          "voice",
		Path:            "temp/screen_sess_clip_invalid/clip.webm",
		CapturedAt:      time.Date(2026, 4, 19, 9, 45, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
		CleanupRequired: true,
	}, "eng", "error_evidence", nil)
	if !errors.Is(err, tools.ErrToolOutputInvalid) {
		t.Fatalf("expected invalid clip frame path to be rejected, got %v", err)
	}
}

func TestBuildScreenAnalysisResultSucceeds(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_010/frame_010.png", Text: "build failed because dependency lockfile is missing", Language: "eng", Source: "ocr_worker_text"}}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	analysis, err := service.buildScreenAnalysisResult(context.Background(), "task_screen_analysis_001", tools.ScreenFrameCandidate{
		FrameID:         "frame_010",
		ScreenSessionID: "screen_sess_010",
		CaptureMode:     tools.ScreenCaptureModeScreenshot,
		Source:          "voice",
		Path:            "temp/screen_sess_010/frame_010.png",
		CapturedAt:      time.Date(2026, 4, 18, 20, 0, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
	}, "eng", "error_evidence", map[string]any{"region_count": 1})
	if err != nil {
		t.Fatalf("buildScreenAnalysisResult returned error: %v", err)
	}
	if !strings.Contains(analysis.BubbleText, "已分析屏幕内容") || analysis.PreviewText == "" {
		t.Fatalf("expected non-empty bubble/preview, got %+v", analysis)
	}
	if analysis.Artifact["artifact_type"] != "screen_capture" {
		t.Fatalf("expected screen capture artifact, got %+v", analysis.Artifact)
	}
	if analysis.ObservationSummary["frame_id"] != "frame_010" {
		t.Fatalf("expected observation summary to retain frame id, got %+v", analysis.ObservationSummary)
	}
	if analysis.CitationSeed["artifact_id"] == "" || analysis.CitationSeed["ocr_excerpt"] == "" {
		t.Fatalf("expected citation-ready seed, got %+v", analysis.CitationSeed)
	}
}

func TestBuildScreenAnalysisResultKeepsClipCleanupPaths(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_clip/frame-001.jpg", Text: "clip summary text", Language: "eng", Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{framesResult: tools.MediaFrameExtractResult{InputPath: "temp/screen_clip/clip.webm", OutputDir: "temp/screen_clip/frames", FramePaths: []string{"temp/screen_clip/frames/frame-001.jpg", "temp/screen_clip/frames/frame-002.jpg"}, FrameCount: 2, Source: "media_worker_frames"}}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	analysis, err := service.buildScreenAnalysisResult(context.Background(), "task_screen_analysis_clip", tools.ScreenFrameCandidate{
		FrameID:         "frame_clip_010",
		ScreenSessionID: "screen_sess_clip_010",
		CaptureMode:     tools.ScreenCaptureModeClip,
		Source:          "voice",
		Path:            "temp/screen_clip/clip.webm",
		CapturedAt:      time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
		CleanupRequired: true,
	}, "eng", "error_evidence", nil)
	if err != nil {
		t.Fatalf("buildScreenAnalysisResult returned error: %v", err)
	}
	if len(analysis.CleanupPaths) != 4 || analysis.CleanupPaths[0] != "temp/screen_clip/clip_normalized.mp4" {
		t.Fatalf("expected clip cleanup paths to be preserved, got %+v", analysis.CleanupPaths)
	}
	if analysis.ObservationSummary["clip_worker_source"] != "media_worker_frames" {
		t.Fatalf("expected clip observation summary to keep media worker source, got %+v", analysis.ObservationSummary)
	}
}

func TestBuildScreenAnalysisResultFallsBackWhenOCRTextEmpty(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_011/frame_011.png", Text: "   ", Language: "eng", Source: "ocr_worker_text"}}
	service, _ := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	analysis, err := service.buildScreenAnalysisResult(context.Background(), "task_screen_analysis_002", tools.ScreenFrameCandidate{
		FrameID:         "frame_011",
		ScreenSessionID: "screen_sess_011",
		CaptureMode:     tools.ScreenCaptureModeScreenshot,
		Source:          "voice",
		Path:            "temp/screen_sess_011/frame_011.png",
		CapturedAt:      time.Date(2026, 4, 18, 20, 30, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
	}, "eng", "error_evidence", nil)
	if err != nil {
		t.Fatalf("buildScreenAnalysisResult returned error: %v", err)
	}
	if !strings.Contains(analysis.BubbleText, "未识别到可用屏幕文本") {
		t.Fatalf("expected empty OCR summary fallback, got %+v", analysis)
	}
}

func TestExecuteInternalScreenAnalysisReturnsResult(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_020/frame_020.png", Text: "build failed due to missing env file", Language: "eng", Source: "ocr_worker_text"}}
	service, workspaceRoot := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_sess_020"), 0o755); err != nil {
		t.Fatalf("mkdir screen temp dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_sess_020", "frame_020.png"), []byte("fake screen capture"), 0o644); err != nil {
		t.Fatalf("write screen temp file failed: %v", err)
	}
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_screen_exec_001",
		RunID:        "run_screen_exec_001",
		Title:        "分析屏幕截图",
		Intent:       map[string]any{"name": internalScreenAnalyzeIntent, "arguments": map[string]any{"frame_id": "frame_020", "screen_session_id": "screen_sess_020", "path": "temp/screen_sess_020/frame_020.png", "capture_mode": "screenshot", "language": "eng", "evidence_role": "error_evidence"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请分析截图中的错误"},
		DeliveryType: "bubble",
		ResultTitle:  "屏幕分析结果",
	})
	if err != nil {
		t.Fatalf("internal screen analysis execute failed: %v", err)
	}
	if result.ToolName != internalScreenAnalyzeIntent || !strings.Contains(result.BubbleText, "已分析屏幕内容") {
		t.Fatalf("unexpected internal screen analysis result: %+v", result)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Status != tools.ToolCallStatusSucceeded {
		t.Fatalf("expected successful internal screen analysis tool call, got %+v", result.ToolCalls)
	}
	auditCandidate := mapValue(result.ToolCalls[0].Output, "audit_candidate")
	if auditCandidate["action"] != "screen.capture.screenshot_analyze" || auditCandidate["result"] != "success" {
		t.Fatalf("expected successful screen audit candidate on tool call, got %+v", result.ToolCalls[0].Output)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0]["artifact_type"] != "screen_capture" {
		t.Fatalf("expected one screen capture artifact, got %+v", result.Artifacts)
	}
	if result.ToolOutput["observation_summary"] == nil || result.ToolOutput["citation_seed"] == nil {
		t.Fatalf("expected observation and citation outputs, got %+v", result.ToolOutput)
	}
	if result.AuditRecord == nil || result.ToolOutput["audit_record"] == nil {
		t.Fatalf("expected audit record to be attached, got result=%+v", result)
	}
	auditRecord := result.AuditRecord
	if auditRecord["action"] != "screen.capture.screenshot_analyze" {
		t.Fatalf("expected formalized screen audit action, got %+v", auditRecord)
	}
	if auditRecord["target"] != result.Artifacts[0]["path"] {
		t.Fatalf("expected screen audit target to follow promoted artifact path, got audit=%+v artifacts=%+v", auditRecord, result.Artifacts)
	}
	auditMetadata := mapValue(auditRecord, "metadata")
	if auditMetadata["screen_session_id"] == "" || auditMetadata["capture_mode"] != "screenshot" {
		t.Fatalf("expected screen audit metadata, got %+v", auditRecord)
	}
	auditCandidateOutput := mapValue(result.ToolOutput, "audit_candidate")
	if auditCandidateOutput["target"] != result.Artifacts[0]["path"] {
		t.Fatalf("expected audit candidate target to follow promoted artifact path, got %+v", auditCandidateOutput)
	}
	cleanupSummary := mapValue(result.ToolOutput, "cleanup_summary")
	if cleanupSummary["reason"] != "screen_artifact_promoted" || cleanupSummary["deleted_count"] != 1 {
		t.Fatalf("expected cleanup summary to be attached, got %+v", result.ToolOutput)
	}
	traceSummary := mapValue(result.ToolOutput, "trace_summary")
	if traceSummary["kind"] != "screen_analysis" || traceSummary["frame_id"] != "frame_020" {
		t.Fatalf("expected trace summary to be attached, got %+v", result.ToolOutput)
	}
	evalSummary := mapValue(result.ToolOutput, "eval_summary")
	if evalSummary["kind"] != "screen_analysis" || evalSummary["has_artifact"] != true {
		t.Fatalf("expected eval summary to be attached, got %+v", result.ToolOutput)
	}
	cleanupPlan := mapValue(result.ToolOutput, "cleanup_plan")
	if len(cleanupPlan) != 0 {
		t.Fatalf("expected cleanup plan to be cleared after artifact promotion, got %+v", result.ToolOutput)
	}
	cleanupExecuted := mapValue(result.ToolOutput, "cleanup_executed")
	if cleanupExecuted["deleted_count"] != 1 || cleanupExecuted["skipped_count"] != 0 {
		t.Fatalf("expected cleanup execution summary, got %+v", result.ToolOutput)
	}
	persisted := mapValue(result.ToolOutput, "artifact_persisted")
	if persisted["persisted"] != true {
		t.Fatalf("expected artifact persistence result, got %+v", result.ToolOutput)
	}
	artifactPath := result.Artifacts[0]["path"].(string)
	if !strings.HasPrefix(artifactPath, "artifacts/screen/task_screen_exec_001/") {
		t.Fatalf("expected promoted artifact path, got %q", artifactPath)
	}
	recoveryPoint := mapValue(result.ToolOutput, "recovery_point")
	if len(recoveryPoint) != 0 {
		t.Fatalf("expected artifact promotion to clear deferred cleanup recovery semantics, got %+v", result.ToolOutput)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "temp", "screen_sess_020", "frame_020.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected promoted artifact source to be moved out of temp storage, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(artifactPath))); err != nil {
		t.Fatalf("expected promoted artifact file to exist, got %v", err)
	}
	records, total, err := service.artifactStore.ListArtifacts(context.Background(), "task_screen_exec_001", 20, 0)
	if err != nil || total != 1 || len(records) != 1 {
		t.Fatalf("expected persisted screen artifact record, total=%d len=%d err=%v", total, len(records), err)
	}
	if records[0].ArtifactType != "screen_capture" {
		t.Fatalf("expected screen_capture artifact record, got %+v", records[0])
	}
	if records[0].Path != artifactPath || persisted["path"] != artifactPath {
		t.Fatalf("expected persisted artifact path to follow promoted artifact, record=%+v persisted=%+v", records[0], persisted)
	}
}

func TestScreenAuditTargetCandidatePrefersPromotedArtifactPath(t *testing.T) {
	candidate := tools.ScreenFrameCandidate{ScreenSessionID: "screen_sess_target", CaptureMode: tools.ScreenCaptureModeScreenshot, Path: "temp/screen_sess_target/frame_001.png"}
	updated := screenAuditTargetCandidate(candidate, map[string]any{"path": "artifacts/screen/task_demo/frame_001.png"})
	if updated.Path != "artifacts/screen/task_demo/frame_001.png" {
		t.Fatalf("expected promoted artifact path to replace temp audit target, got %+v", updated)
	}
	unchanged := screenAuditTargetCandidate(candidate, nil)
	if unchanged.Path != candidate.Path {
		t.Fatalf("expected missing artifact path to leave audit target unchanged, got %+v", unchanged)
	}
}

func TestExecuteInternalScreenAnalysisRetainsClipFrameCleanupPlan(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_clip_exec/frames/frame-001.jpg", Text: "clip execution summary", Language: "eng", Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{framesResult: tools.MediaFrameExtractResult{InputPath: "temp/screen_clip_exec/clip.webm", OutputDir: "temp/screen_clip_exec/frames", FramePaths: []string{"temp/screen_clip_exec/frames/frame-001.jpg"}, FrameCount: 1, Source: "media_worker_frames"}}
	service, workspaceRoot := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_clip_exec"), 0o755); err != nil {
		t.Fatalf("mkdir clip temp dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_clip_exec", "clip.webm"), []byte("fake clip capture"), 0o644); err != nil {
		t.Fatalf("write clip temp file failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_clip_exec", "frames"), 0o755); err != nil {
		t.Fatalf("mkdir clip frame dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_clip_exec", "frames", "frame-001.jpg"), []byte("fake frame"), 0o644); err != nil {
		t.Fatalf("write clip frame file failed: %v", err)
	}
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_screen_clip_exec",
		RunID:        "run_screen_clip_exec",
		Title:        "分析屏幕录屏",
		Intent:       map[string]any{"name": internalScreenAnalyzeIntent, "arguments": map[string]any{"frame_id": "frame_clip_exec", "screen_session_id": "screen_clip_exec", "path": "temp/screen_clip_exec/clip.webm", "capture_mode": "clip", "language": "eng", "evidence_role": "error_evidence"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请分析录屏中的错误"},
		DeliveryType: "bubble",
		ResultTitle:  "屏幕录屏分析结果",
	})
	if err != nil {
		t.Fatalf("internal clip screen analysis execute failed: %v", err)
	}
	cleanupPlan := mapValue(result.ToolOutput, "cleanup_plan")
	paths := stringSliceValue(cleanupPlan, "paths")
	if len(paths) != 3 || paths[0] != "temp/screen_clip_exec/clip_normalized.mp4" || paths[1] != "temp/screen_clip_exec/frames/frame-001.jpg" || paths[2] != "temp/screen_clip_exec/frames" {
		t.Fatalf("expected clip cleanup plan to retain extracted frame cleanup, got %+v", cleanupPlan)
	}
	hasOCRManifest := false
	hasMediaManifest := false
	for _, asset := range result.ExtensionAssets {
		if asset["asset_kind"] != storage.ExtensionAssetKindPluginManifest {
			continue
		}
		if asset["asset_id"] == "ocr" {
			hasOCRManifest = true
		}
		if asset["asset_id"] == "media" {
			hasMediaManifest = true
		}
	}
	if !hasOCRManifest || !hasMediaManifest {
		t.Fatalf("expected clip execution to attribute both OCR and media plugin manifests, assets=%+v", result.ExtensionAssets)
	}
	cleanupSummary := mapValue(result.ToolOutput, "cleanup_summary")
	if cleanupSummary["deleted_count"] != 1 || cleanupSummary["skipped_count"] != 3 {
		t.Fatalf("expected clip cleanup summary to merge promoted clip artifact and pending frame cleanup, got %+v", cleanupSummary)
	}
	recoveryPoint := mapValue(result.ToolOutput, "recovery_point")
	if recoveryPoint["kind"] != "screen_cleanup" || recoveryPoint["cleanup_status"] != "pending_retry" {
		t.Fatalf("expected clip analysis to keep deferred cleanup recovery semantics, got %+v", recoveryPoint)
	}
	artifactPath := result.Artifacts[0]["path"].(string)
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(artifactPath))); err != nil {
		t.Fatalf("expected promoted clip artifact to exist, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "temp", "screen_clip_exec", "frames", "frame-001.jpg")); err != nil {
		t.Fatalf("expected extracted clip frame to remain pending cleanup, got %v", err)
	}
}

func TestExecuteInternalScreenAnalysisReturnsFailedAuditTrailOnOCRFailure(t *testing.T) {
	ocrStub := stubOCRWorkerClient{err: tools.ErrOCRWorkerFailed}
	service, workspaceRoot := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_fail_exec"), 0o755); err != nil {
		t.Fatalf("mkdir screen failure temp dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_fail_exec", "frame_fail.png"), []byte("fake screen capture"), 0o644); err != nil {
		t.Fatalf("write screen failure temp file failed: %v", err)
	}
	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_screen_exec_fail",
		RunID:        "run_screen_exec_fail",
		Title:        "分析失败截图",
		Intent:       map[string]any{"name": internalScreenAnalyzeIntent, "arguments": map[string]any{"frame_id": "frame_fail", "screen_session_id": "screen_fail_exec", "path": "temp/screen_fail_exec/frame_fail.png", "capture_mode": "screenshot", "language": "eng", "evidence_role": "error_evidence"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请分析截图中的错误"},
		DeliveryType: "bubble",
		ResultTitle:  "屏幕分析失败结果",
	})
	if !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected OCR worker failure, got result=%+v err=%v", result, err)
	}
	if result.ToolName != internalScreenAnalyzeIntent || len(result.ToolCalls) != 1 || result.ToolCalls[0].Status != tools.ToolCallStatusFailed {
		t.Fatalf("expected failed screen analysis tool trace, got %+v", result)
	}
	auditCandidate := mapValue(result.ToolCalls[0].Output, "audit_candidate")
	if auditCandidate["result"] != "failed" || auditCandidate["action"] != "screen.capture.screenshot_analyze" {
		t.Fatalf("expected failed screen audit candidate, got %+v", result.ToolCalls[0].Output)
	}
	cleanupPlan := mapValue(result.ToolOutput, "cleanup_plan")
	if len(stringSliceValue(cleanupPlan, "paths")) != 1 || stringSliceValue(cleanupPlan, "paths")[0] != "temp/screen_fail_exec/frame_fail.png" {
		t.Fatalf("expected failure cleanup plan to retain screenshot temp path, got %+v", result.ToolOutput)
	}
	recoveryPoint := mapValue(result.ToolOutput, "recovery_point")
	if recoveryPoint["kind"] != "screen_cleanup" || recoveryPoint["cleanup_status"] != "pending_retry" {
		t.Fatalf("expected failure recovery point for pending cleanup, got %+v", result.ToolOutput)
	}
	if result.AuditRecord == nil || result.AuditRecord["result"] != "failed" {
		t.Fatalf("expected failed screen audit record, got %+v", result.AuditRecord)
	}
}

func TestExecuteInternalScreenClipAnalysisUsesMediaWorkerOutputs(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_clip_020/clip_020_frames/frame-001.jpg", Text: "release build failed after sign step", Language: "eng", Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{
		transcodeResult: tools.MediaTranscodeResult{InputPath: "temp/screen_sess_clip_020/clip_020.webm", OutputPath: "temp/screen_sess_clip_020/clip_020_normalized.mp4", Format: "mp4", Source: "media_worker_ffmpeg"},
		framesResult:    tools.MediaFrameExtractResult{InputPath: "temp/screen_sess_clip_020/clip_020_normalized.mp4", OutputDir: "temp/screen_sess_clip_020/clip_020_frames", FramePaths: []string{"temp/screen_sess_clip_020/clip_020_frames/frame-001.jpg"}, FrameCount: 1, Source: "media_worker_frames"},
	}
	service, workspaceRoot := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_020"), 0o755); err != nil {
		t.Fatalf("mkdir clip temp dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_020", "clip_020.webm"), []byte("clip data"), 0o644); err != nil {
		t.Fatalf("write clip temp file failed: %v", err)
	}

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_screen_clip_exec_001",
		RunID:        "run_screen_clip_exec_001",
		Title:        "分析录屏片段",
		Intent:       map[string]any{"name": internalScreenAnalyzeIntent, "arguments": map[string]any{"frame_id": "frame_clip_020", "screen_session_id": "screen_sess_clip_020", "path": "temp/screen_sess_clip_020/clip_020.webm", "capture_mode": "clip", "language": "eng", "evidence_role": "error_evidence"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请分析录屏中的错误"},
		DeliveryType: "bubble",
		ResultTitle:  "录屏分析结果",
	})
	if err != nil {
		t.Fatalf("internal screen clip analysis execute failed: %v", err)
	}
	if result.ToolName != internalScreenAnalyzeIntent || !strings.Contains(result.BubbleText, "release build failed") {
		t.Fatalf("unexpected internal screen clip analysis result: %+v", result)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0]["mime_type"] != "video/webm" || !strings.HasPrefix(stringValue(result.Artifacts[0], "path", ""), "artifacts/screen/task_screen_clip_exec_001/") {
		t.Fatalf("expected one persisted clip artifact, got %+v", result.Artifacts)
	}
	auditRecord := result.AuditRecord
	if auditRecord == nil || auditRecord["action"] != "screen.capture.clip_analyze" {
		t.Fatalf("expected clip audit action, got %+v", auditRecord)
	}
	cleanupPlan := mapValue(result.ToolOutput, "cleanup_plan")
	if cleanupPlan["reason"] != "screen_clip_pending_cleanup" {
		t.Fatalf("expected clip cleanup reason, got %+v", cleanupPlan)
	}
	cleanupPaths := stringSliceValue(cleanupPlan, "paths")
	if len(cleanupPaths) != 3 || cleanupPaths[0] != "temp/screen_sess_clip_020/clip_020_normalized.mp4" {
		t.Fatalf("expected clip cleanup plan to track clip, normalized media, and frame outputs, got %+v", cleanupPlan)
	}
	observationSummary := mapValue(result.ToolOutput, "observation_summary")
	if !strings.HasPrefix(stringValue(observationSummary, "clip_path", ""), "artifacts/screen/task_screen_clip_exec_001/") || observationSummary["analysis_frame_path"] != "temp/screen_sess_clip_020/clip_020_frames/frame-001.jpg" || observationSummary["temp_clip_path"] != "temp/screen_sess_clip_020/clip_020.webm" {
		t.Fatalf("expected clip observation summary to include media normalization metadata, got %+v", observationSummary)
	}
	artifactPath := stringValue(result.Artifacts[0], "path", "")
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(artifactPath))); err != nil {
		t.Fatalf("expected clip artifact move to persist durable workspace artifact, got %v", err)
	}
	recoveryPoint := mapValue(result.ToolOutput, "recovery_point")
	if recoveryPoint["summary"] != "screen_cleanup_pending:screen_clip_pending_cleanup" || recoveryPoint["cleanup_status"] != "pending_retry" {
		t.Fatalf("expected clip recovery point semantics, got %+v", recoveryPoint)
	}
	citationSeed := mapValue(result.ToolOutput, "citation_seed")
	if citationSeed["artifact_type"] != "screen_capture" || citationSeed["ocr_excerpt"] == nil {
		t.Fatalf("expected clip citation seed to stay artifact-backed, got %+v", citationSeed)
	}
}

func TestExecuteInternalScreenClipAnalysisRejectsMissingFrames(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_clip_021/clip_021_frames/frame-001.jpg", Text: "unused", Language: "eng", Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{transcodeResult: tools.MediaTranscodeResult{InputPath: "temp/screen_sess_clip_021/clip_021.webm", OutputPath: "temp/screen_sess_clip_021/clip_021_normalized.mp4", Format: "mp4", Source: "media_worker_ffmpeg"}}
	service, workspaceRoot := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_021"), 0o755); err != nil {
		t.Fatalf("mkdir clip failure temp dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_021", "clip_021.webm"), []byte("clip data"), 0o644); err != nil {
		t.Fatalf("write clip failure temp file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_021", "clip_021_normalized.mp4"), []byte("normalized clip data"), 0o644); err != nil {
		t.Fatalf("write normalized clip temp file failed: %v", err)
	}

	_, err := service.Execute(context.Background(), Request{
		TaskID:       "task_screen_clip_exec_002",
		RunID:        "run_screen_clip_exec_002",
		Title:        "分析录屏片段",
		Intent:       map[string]any{"name": internalScreenAnalyzeIntent, "arguments": map[string]any{"frame_id": "frame_clip_021", "screen_session_id": "screen_sess_clip_021", "path": "temp/screen_sess_clip_021/clip_021.webm", "capture_mode": "clip", "language": "eng", "evidence_role": "error_evidence"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请分析录屏中的错误"},
		DeliveryType: "bubble",
		ResultTitle:  "录屏分析结果",
	})
	if !errors.Is(err, tools.ErrToolOutputInvalid) {
		t.Fatalf("expected missing extracted frames to map to tool output invalid, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_021", "clip_021.webm")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected failed clip analysis to cleanup temp clip input, got %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_021", "clip_021_normalized.mp4")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected failed clip analysis to cleanup normalized clip output, got %v", statErr)
	}
}

func TestExecuteInternalScreenClipAnalysisRemovesPromotedArtifactOnArtifactBuildFailure(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_sess_clip_022/clip_022_frames/frame-001.jpg", Text: "unused", Language: "eng", Source: "ocr_worker_text"}}
	mediaStub := stubMediaWorkerClient{
		transcodeResult: tools.MediaTranscodeResult{InputPath: "temp/screen_sess_clip_022/clip_022.webm", OutputPath: "temp/screen_sess_clip_022/clip_022_normalized.mp4", Format: "mp4", Source: "media_worker_ffmpeg"},
		framesResult:    tools.MediaFrameExtractResult{InputPath: "temp/screen_sess_clip_022/clip_022_normalized.mp4", OutputDir: "temp/screen_sess_clip_022/clip_022_frames", FramePaths: []string{"temp/screen_sess_clip_022/clip_022_frames/frame-001.jpg"}, FrameCount: 1, Source: "media_worker_frames"},
	}
	service, workspaceRoot := newTestExecutionServiceWithWorkers(t, "unused", sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, mediaStub)
	service.lifecycle = nil
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_022"), 0o755); err != nil {
		t.Fatalf("mkdir promoted clip temp dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "temp", "screen_sess_clip_022", "clip_022.webm"), []byte("clip data"), 0o644); err != nil {
		t.Fatalf("write promoted clip temp file failed: %v", err)
	}

	_, err := service.Execute(context.Background(), Request{
		TaskID:       "task_screen_clip_exec_003",
		RunID:        "run_screen_clip_exec_003",
		Title:        "分析录屏片段",
		Intent:       map[string]any{"name": internalScreenAnalyzeIntent, "arguments": map[string]any{"frame_id": "frame_clip_022", "screen_session_id": "screen_sess_clip_022", "path": "temp/screen_sess_clip_022/clip_022.webm", "capture_mode": "clip", "language": "eng", "evidence_role": "error_evidence"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请分析录屏中的错误"},
		DeliveryType: "bubble",
		ResultTitle:  "录屏分析结果",
	})
	if err == nil || !strings.Contains(err.Error(), "screen lifecycle manager is required") {
		t.Fatalf("expected artifact build failure to surface lifecycle requirement, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(workspaceRoot, "artifacts", "screen_capture", "task_screen_clip_exec_003", "clip_022.webm")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected failed artifact build to cleanup promoted clip artifact, got %v", statErr)
	}
}

func TestExecuteInternalScreenAnalysisRejectsIncompleteCandidate(t *testing.T) {
	service, _ := newTestExecutionService(t, "unused")
	_, err := service.Execute(context.Background(), Request{
		TaskID:       "task_screen_exec_002",
		RunID:        "run_screen_exec_002",
		Title:        "分析屏幕截图",
		Intent:       map[string]any{"name": internalScreenAnalyzeIntent, "arguments": map[string]any{"screen_session_id": "screen_sess_021"}},
		Snapshot:     contextsvc.TaskContextSnapshot{InputType: "text", Text: "请分析截图中的错误"},
		DeliveryType: "bubble",
		ResultTitle:  "屏幕分析结果",
	})
	if err == nil || !strings.Contains(err.Error(), "screen analysis candidate arguments are incomplete") {
		t.Fatalf("expected incomplete candidate error, got %v", err)
	}
}

func TestExecuteScreenCleanupPlanHandlesSkippedPaths(t *testing.T) {
	service, _ := newTestExecutionService(t, "unused")
	result := service.executeScreenCleanupPlan(map[string]any{
		"reason":           "screen_analysis_pending_cleanup",
		"cleanup_required": true,
		"paths":            []string{"temp/missing.png"},
	})
	if result["deleted_count"] != 0 || result["skipped_count"] != 1 {
		t.Fatalf("expected skipped cleanup result, got %+v", result)
	}
}

func TestScreenHelpersCoverNilAndPendingBranches(t *testing.T) {
	service, _ := newTestExecutionService(t, "unused")
	if got := service.executeScreenCleanupPlan(nil); got != nil {
		t.Fatalf("expected nil cleanup plan to skip execution, got %+v", got)
	}
	if got := service.screenAnalysisCleanupPlan(tools.ScreenFrameCandidate{}, nil); got != nil {
		t.Fatalf("expected no cleanup plan for empty candidate, got %+v", got)
	}
	if got := service.screenAnalysisCleanupSummary(nil); got != nil {
		t.Fatalf("expected no cleanup summary for empty candidate, got %+v", got)
	}
	if got := service.screenAnalysisRecoveryPoint(context.Background(), "task_screen_none", map[string]any{"paths": []string{}}, nil); got != nil {
		t.Fatalf("expected no recovery point without cleanup objects, got %+v", got)
	}
	auditRecord := service.screenAnalysisAuditRecord("task_screen_audit", tools.ScreenFrameCandidate{ScreenSessionID: "screen_sess_extra", CaptureMode: tools.ScreenCaptureModeKeyframe, Source: "voice", Path: "temp/screen_sess_extra/frame.png"}, "screen preview")
	if auditRecord["action"] != "screen.capture.keyframe_analyze" {
		t.Fatalf("expected keyframe audit action, got %+v", auditRecord)
	}
	clipAudit := service.screenAnalysisAuditRecord("task_screen_clip", tools.ScreenFrameCandidate{ScreenSessionID: "screen_sess_clip", CaptureMode: tools.ScreenCaptureModeClip, Source: "voice", Path: "temp/screen_sess_clip/clip.webm"}, "clip preview")
	if clipAudit["action"] != "screen.capture.clip_analyze" {
		t.Fatalf("expected clip audit action, got %+v", clipAudit)
	}
	if got := service.screenAnalysisTraceSummary(tools.ScreenFrameCandidate{}, nil); got != nil {
		t.Fatalf("expected nil trace summary when analysis missing, got %+v", got)
	}
	if got := service.screenAnalysisEvalSummary(tools.ScreenFrameCandidate{}, nil); got != nil {
		t.Fatalf("expected nil eval summary when analysis missing, got %+v", got)
	}
	service.audit = nil
	if got := service.screenAnalysisAuditRecord("task_screen_noaudit", tools.ScreenFrameCandidate{}, "preview"); got != nil {
		t.Fatalf("expected nil audit record when audit service unavailable, got %+v", got)
	}
	service.checkpoint = nil
	if got := service.screenAnalysisRecoveryPoint(context.Background(), "task_screen_norecovery", map[string]any{"paths": []string{"temp/demo.png"}}, map[string]any{"skipped_count": 1, "skipped_paths": []string{"temp/demo.png"}}); got != nil {
		t.Fatalf("expected nil recovery point when checkpoint unavailable, got %+v", got)
	}
	mergedCleanup := mergeScreenCleanupSummaries(map[string]any{"deleted_paths": []string{"temp/a.png"}, "deleted_count": 1}, map[string]any{"skipped_paths": []string{"temp/b.png"}, "skipped_count": 1})
	if mergedCleanup["deleted_count"] != 1 || mergedCleanup["skipped_count"] != 1 {
		t.Fatalf("expected merged cleanup summary to keep both deleted and skipped counts, got %+v", mergedCleanup)
	}
	trimmedPlan := removeScreenCleanupPaths(map[string]any{"paths": []string{"temp/a.png", "temp/b.png"}}, []string{"temp/a.png"})
	if len(stringSliceValue(trimmedPlan, "paths")) != 1 || stringSliceValue(trimmedPlan, "paths")[0] != "temp/b.png" {
		t.Fatalf("expected cleanup plan path removal to keep remaining paths, got %+v", trimmedPlan)
	}
	if caps := internalScreenAnalysisCapabilities(Request{Intent: map[string]any{"arguments": map[string]any{"capture_mode": string(tools.ScreenCaptureModeClip)}}}); len(caps) != 2 || caps[1] != "extract_frames" {
		t.Fatalf("expected clip capture mode to attribute media extraction capability, got %+v", caps)
	}
}

func TestExecutionSmallHelpersCoverPrimitiveBranches(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")
	service.tools = nil
	if got := service.availableToolNames(); got != nil {
		t.Fatalf("expected nil available tools when registry missing, got %+v", got)
	}
	service.plugin = nil
	if got := service.availableWorkers(); got != nil {
		t.Fatalf("expected nil available workers when plugin missing, got %+v", got)
	}
	service.screen = nil
	if got := service.ScreenClient(); got != nil {
		t.Fatalf("expected nil screen client accessor, got %+v", got)
	}
	if got := stringSliceValue(map[string]any{"paths": []any{"a", " ", 1, "b"}}, "paths"); len(got) != 2 || got[1] != "b" {
		t.Fatalf("unexpected string slice coercion: %+v", got)
	}
	if got := intValue(map[string]any{"n": float64(2)}, "n"); got != 2 {
		t.Fatalf("unexpected intValue result: %d", got)
	}
	if got := int64Value(map[string]any{"n": float32(3)}, "n"); got != 3 {
		t.Fatalf("unexpected int64Value result: %d", got)
	}
	if got := marshalEventPayload(map[string]any{"k": "v"}); !strings.Contains(got, "\"k\":\"v\"") {
		t.Fatalf("unexpected marshaled payload: %s", got)
	}
	if got := runStatusFromStopReason(agentloop.StopReasonNeedAuthorization); got != "waiting_auth" {
		t.Fatalf("unexpected run status for need auth: %s", got)
	}
	if got := runStatusFromStopReason(agentloop.StopReasonCompleted); got != "completed" {
		t.Fatalf("unexpected run status for completed: %s", got)
	}
	if got := runStatusFromStopReason(agentloop.StopReason("other")); got != "processing" {
		t.Fatalf("unexpected fallback run status: %s", got)
	}
	if got := resolveWorkspaceRoot(nil); got != "" {
		t.Fatalf("expected empty workspace root for nil filesystem, got %q", got)
	}
	workspacePolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	if got := resolveWorkspaceRoot(platform.NewLocalFileSystemAdapter(workspacePolicy)); !strings.Contains(got, "workspace") {
		t.Fatalf("expected workspace root path, got %q", got)
	}
	modelService := model.NewService(serviceconfig.ModelConfig{PlannerRetryBudget: 3, ToolRetryBudget: 2, ContextCompressChars: 1234, MaxToolIterations: 7})
	service.model = modelService
	if service.agentLoopPlannerRetryBudget() != 3 || service.agentLoopToolRetryBudget() != 2 {
		t.Fatalf("unexpected agent loop retry budgets")
	}
	if service.agentLoopCompressionChars() != 1234 || service.agentLoopMaxTurns() != 7 {
		t.Fatalf("unexpected agent loop model-derived limits")
	}
}

func TestExecuteScreenCleanupPlanDeletesExistingWorkspacePath(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "temp", "screen_sess_030"), 0o755); err != nil {
		t.Fatalf("mkdir temp screen path: %v", err)
	}
	targetPath := filepath.Join(workspaceRoot, "temp", "screen_sess_030", "frame_030.png")
	if err := os.WriteFile(targetPath, []byte("demo"), 0o644); err != nil {
		t.Fatalf("write temp screen file: %v", err)
	}
	result := service.executeScreenCleanupPlan(map[string]any{
		"reason":           "screen_analysis_pending_cleanup",
		"cleanup_required": true,
		"paths":            []string{"temp/screen_sess_030/frame_030.png"},
	})
	if result["deleted_count"] != 1 || result["skipped_count"] != 0 {
		t.Fatalf("expected deleted cleanup result, got %+v", result)
	}
	if _, err := os.Stat(targetPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected target file to be removed, got %v", err)
	}
}

func TestExecuteScreenCleanupPlanRemovesClipFrameDirectories(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")
	frameDir := filepath.Join(workspaceRoot, "temp", "screen_sess_031", "clip_frames")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		t.Fatalf("mkdir clip frame dir: %v", err)
	}
	framePath := filepath.Join(frameDir, "frame_001.jpg")
	if err := os.WriteFile(framePath, []byte("demo"), 0o644); err != nil {
		t.Fatalf("write clip frame file: %v", err)
	}
	result := service.executeScreenCleanupPlan(map[string]any{
		"reason":           "screen_clip_pending_cleanup",
		"cleanup_required": true,
		"paths":            []string{"temp/screen_sess_031/clip_frames"},
	})
	if result["deleted_count"] != 2 || result["skipped_count"] != 0 {
		t.Fatalf("expected recursive clip cleanup result, got %+v", result)
	}
	if _, err := os.Stat(frameDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected clip frame dir to be removed, got %v", err)
	}
}

func TestRemoveCleanupPathCoversNilBlankAndNestedBranches(t *testing.T) {
	service, workspaceRoot := newTestExecutionService(t, "unused")
	if _, err := removeCleanupPath(nil, "temp/clip_frames"); err == nil {
		t.Fatal("expected nil file system cleanup to fail")
	}
	deleted, err := removeCleanupPath(service.fileSystem, "")
	if err != nil || len(deleted) != 0 {
		t.Fatalf("expected blank cleanup path to no-op, got deleted=%+v err=%v", deleted, err)
	}
	nestedDir := filepath.Join(workspaceRoot, "temp", "screen_sess_cleanup", "frames", "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested cleanup dir: %v", err)
	}
	nestedFile := filepath.Join(nestedDir, "frame_001.jpg")
	if err := os.WriteFile(nestedFile, []byte("demo"), 0o644); err != nil {
		t.Fatalf("write nested cleanup file: %v", err)
	}
	deleted, err = removeCleanupPath(service.fileSystem, "temp/screen_sess_cleanup")
	if err != nil {
		t.Fatalf("removeCleanupPath returned error: %v", err)
	}
	if len(deleted) < 4 {
		t.Fatalf("expected recursive cleanup to remove nested file and directories, got %+v", deleted)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "temp", "screen_sess_cleanup")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected recursive cleanup to remove root dir, got %v", err)
	}
}

func TestScreenAnalysisRecoveryPointRefinesPendingCleanupSemantics(t *testing.T) {
	service, _ := newTestExecutionService(t, "unused")
	recoveryPoint := service.screenAnalysisRecoveryPoint(context.Background(), "task_screen_recovery_001", map[string]any{
		"screen_session_id": "screen_sess_099",
		"reason":            "screen_analysis_pending_cleanup",
		"cleanup_required":  true,
		"paths":             []string{"temp/screen_sess_099/frame_099.png"},
	}, map[string]any{
		"reason":        "screen_analysis_pending_cleanup",
		"deleted_paths": []string{},
		"skipped_paths": []string{"temp/screen_sess_099/frame_099.png"},
		"deleted_count": 0,
		"skipped_count": 1,
	})
	if recoveryPoint == nil {
		t.Fatal("expected pending cleanup to yield recovery point")
	}
	if recoveryPoint["summary"] != "screen_cleanup_pending:screen_analysis_pending_cleanup" {
		t.Fatalf("expected refined screen cleanup summary, got %+v", recoveryPoint)
	}
	if recoveryPoint["kind"] != "screen_cleanup" || recoveryPoint["cleanup_status"] != "pending_retry" {
		t.Fatalf("expected refined recovery semantics, got %+v", recoveryPoint)
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

func TestFallbackOutputRequestsClarificationForAgentLoopWhenGoalIsUnderspecified(t *testing.T) {
	output := fallbackOutput(Request{Intent: map[string]any{"name": defaultAgentLoopIntentName}}, "你好")

	if !strings.Contains(output, "请补充你的目标") {
		t.Fatalf("expected agent_loop fallback to request clarification, got %s", output)
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
	screenArtifact, err := screenArtifactFromCandidate("task_screen_001", tools.NewScreenLifecycleManager(), tools.ScreenFrameCandidate{
		FrameID:         "frame_001",
		ScreenSessionID: "screen_sess_001",
		CaptureMode:     tools.ScreenCaptureModeKeyframe,
		Source:          "voice",
		Path:            "temp/screen_sess_001/frame_001.png",
		CapturedAt:      time.Date(2026, 4, 18, 18, 0, 0, 0, time.UTC),
		RetentionPolicy: tools.ScreenRetentionReview,
	}, "error_evidence", map[string]any{"region_count": 2})
	if err != nil {
		t.Fatalf("screenArtifactFromCandidate returned error: %v", err)
	}
	if screenArtifact["artifact_type"] != "screen_capture" || screenArtifact["artifact_id"] == "" {
		t.Fatalf("expected stable screen artifact, got %+v", screenArtifact)
	}
	payload := mapValue(screenArtifact, "delivery_payload")
	if payload["screen_session_id"] != "screen_sess_001" || payload["evidence_role"] != "error_evidence" {
		t.Fatalf("expected screen metadata in delivery payload, got %+v", payload)
	}
	ocrInput, ok := screenOCRInputFromCandidate(tools.ScreenFrameCandidate{Path: "temp/screen_sess_001/frame_001.png"}, "eng")
	if !ok || ocrInput["path"] != "temp/screen_sess_001/frame_001.png" || ocrInput["language"] != "eng" {
		t.Fatalf("unexpected screen OCR input: %+v ok=%v", ocrInput, ok)
	}
	if _, ok := screenOCRInputFromCandidate(tools.ScreenFrameCandidate{}, "eng"); ok {
		t.Fatal("expected empty screen candidate to skip OCR input generation")
	}
	observation := screenObservationSeed(
		tools.ScreenFrameCandidate{
			FrameID:         "frame_001",
			ScreenSessionID: "screen_sess_001",
			CaptureMode:     tools.ScreenCaptureModeScreenshot,
			Source:          "voice",
			Path:            "temp/screen_sess_001/frame_001.png",
			CapturedAt:      time.Date(2026, 4, 18, 18, 30, 0, 0, time.UTC),
		},
		tools.OCRTextResult{Path: "temp/screen_sess_001/frame_001.png", Text: "screen error text example", Language: "eng", Source: "ocr_worker_text"},
	)
	if observation["frame_id"] != "frame_001" || observation["ocr_language"] != "eng" {
		t.Fatalf("unexpected screen observation seed: %+v", observation)
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
	gb18030Content, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte("修复执行输入乱码"))
	if err != nil {
		t.Fatalf("GB18030 encode failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes", "legacy.txt"), gb18030Content, 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}
	section := service.fileSection("notes/demo.txt")
	if !strings.Contains(section, "worker file content") {
		t.Fatalf("expected file content section, got %s", section)
	}
	legacySection := service.fileSection("notes/legacy.txt")
	if !strings.Contains(legacySection, "修复执行输入乱码") || strings.ContainsRune(legacySection, '\uFFFD') {
		t.Fatalf("expected decoded legacy file section, got %s", legacySection)
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

func TestExecutionHelperBranchesAndConfigurationAccessors(t *testing.T) {
	if (*Service)(nil).WithArtifactStore(nil) != nil || (*Service)(nil).WithLoopRuntimeStore(nil) != nil || (*Service)(nil).WithExtensionAssetCatalog(nil) != nil || (*Service)(nil).WithNotificationEmitter(nil) != nil || (*Service)(nil).WithSteeringPoller(nil) != nil {
		t.Fatal("expected nil service receiver helpers to return nil")
	}
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		t.Fatalf("register builtin tools: %v", err)
	}
	service := NewService(
		platform.NewLocalFileSystemAdapter(mustPathPolicy(t)),
		stubExecutionCapability{},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		nil,
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		nil,
		plugin.NewService(),
	)
	if service.agentLoopMaxTurns() != 4 || service.agentLoopCompressionChars() != 2400 || service.agentLoopKeepRecent() != 4 || service.agentLoopPlannerRetryBudget() != 1 || service.agentLoopToolRetryBudget() != 1 {
		t.Fatalf("expected nil-model defaults for agent loop config, got service=%+v", service)
	}
	configuredService := NewService(
		platform.NewLocalFileSystemAdapter(mustPathPolicy(t)),
		stubExecutionCapability{},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		model.NewService(serviceconfig.ModelConfig{MaxToolIterations: 7, ContextCompressChars: 1234, ContextKeepRecent: 5, PlannerRetryBudget: 2, ToolRetryBudget: 3}),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		nil,
		plugin.NewService(),
	)
	if configuredService.agentLoopMaxTurns() != 7 || configuredService.agentLoopCompressionChars() != 1234 || configuredService.agentLoopKeepRecent() != 5 || configuredService.agentLoopPlannerRetryBudget() != 2 || configuredService.agentLoopToolRetryBudget() != 3 {
		t.Fatal("expected configured model limits to be exposed through helper accessors")
	}
	request := Request{TaskID: "task_helper", Intent: map[string]any{"name": "translate", "arguments": map[string]any{"target_language": "English"}}, BudgetDowngrade: map[string]any{"applied": true, "summary": "Downgraded", "trigger_reason": "provider_unavailable", "degrade_actions": []any{"skip_expensive_tools"}, "trace": map[string]any{"planner_retry_budget": 2, "expensive_tool_categories": []any{"command", "filesystem_mutation"}}}}
	if !budgetDowngradeBlocksAgentLoopTools(request) || !budgetDowngradeDisallowsDirectTool(request, "exec_command") || budgetDowngradeDisallowsDirectTool(request, "read_file") {
		t.Fatal("expected budget downgrade helpers to classify expensive tools")
	}
	if budgetPlannerRetryBudget(request, 4) != 2 || len(budgetExpensiveToolCategories(request)) != 2 || budgetToolCategory("write_file") != "filesystem_mutation" || budgetToolCategory("page_interact") != "browser_mutation" || budgetToolCategory("normalize_recording") != "media_heavy" {
		t.Fatal("expected budget helper branches to expose configured categories and overrides")
	}
	trace, ok := budgetDowngradeGenerationFallback(request, "input text", errors.New("provider unavailable"))
	if !ok || trace.GenerationOutput["fallback"] != true || trace.ModelInvocation["fallback"] != true {
		t.Fatalf("expected budget downgrade generation fallback trace, got %+v ok=%v", trace, ok)
	}
	failure := budgetFailureSignal(request, model.ErrClientNotConfigured)
	if failure == nil || failure["category"] != "budget_auto_downgrade" || !isBudgetFailureReason(model.ErrClientNotConfigured.Error()) || normalizeBudgetFailureReason("") != "execution fallback" {
		t.Fatalf("expected budget failure helpers to emit structured failure signal, got %+v", failure)
	}
	if !containsExecutionString([]string{"a", "b"}, "b") || containsExecutionString([]string{"a"}, "c") {
		t.Fatal("expected containsExecutionString to match only exact values")
	}
	if !strings.Contains(buildPrompt(request, "hello"), "翻译成English") || !strings.Contains(buildPrompt(Request{Intent: map[string]any{"name": "rewrite"}}, "hello"), "改写") || !strings.Contains(buildPrompt(Request{Intent: map[string]any{"name": "explain"}}, "hello"), "解释") || !strings.Contains(buildPrompt(Request{Intent: map[string]any{"name": "write_file"}}, "hello"), "保存为文档") || !strings.Contains(buildPrompt(Request{Intent: map[string]any{"name": "summarize"}}, "hello"), "摘要") {
		t.Fatal("expected buildPrompt to cover major intent variants")
	}
	if !strings.Contains(fallbackOutput(request, "hello world"), "翻译结果") || workspaceDocumentContent("", "plain text") == "plain text" || previewTextForOutput("", "bubble") == "" || previewTextForDeliveryType("workspace_document") == "" || truncateBubbleText("") == "" {
		t.Fatal("expected delivery helper functions to provide fallback output text")
	}
	if deliveryPayloadPath(map[string]any{"payload": map[string]any{"path": "workspace/result.md"}}) != "workspace/result.md" || targetPathFromIntent(map[string]any{"arguments": map[string]any{"target_path": "workspace/note.md"}}) != "workspace/note.md" || targetPathFromIntent(map[string]any{"arguments": map[string]any{"target_path": "workspace_document"}}) != "" {
		t.Fatal("expected delivery path helpers to resolve explicit workspace targets")
	}
	if workspaceFSPath("workspace/docs/result.md") != "docs/result.md" || workspaceFSPath("../outside") != "" || workspaceFSPath("workspace") != "." || !isWindowsAbsolutePath("C:/workspace/result.md") {
		t.Fatal("expected workspace path helpers to normalize and guard paths")
	}
	if len(extractHighlights("one. two? three!", 2)) != 2 || firstSentence("one. two") == "" || normalizeWhitespace("  a\n b  ") != "a b" || truncateText("hello world", 5) != "hello..." {
		t.Fatal("expected text helpers to normalize, extract, and truncate text")
	}
	if mapValue(nil, "missing") == nil || stringValue(map[string]any{"name": "  ok  "}, "name", "fallback") != "  ok  " || boolValue(map[string]any{"enabled": true}, "enabled") != true || len(stringSliceValue(map[string]any{"items": []any{" a ", 2, "b"}}, "items")) != 2 {
		t.Fatal("expected primitive execution helpers to tolerate nil maps and decode slices")
	}
	if intValue(map[string]any{"count": int32(3)}, "count") != 3 || intValue(map[string]any{"count": float32(4)}, "count") != 4 || int64Value(map[string]any{"count": float64(5)}, "count") != 5 {
		t.Fatal("expected numeric helper branches to cover multiple number types")
	}
	if invocationRecordMap(nil) != nil {
		t.Fatal("expected invocationRecordMap(nil) to stay nil")
	}
	if agentloopAppendSteeringInput("base", []string{" ", "first", "second"}) == "base" || !isAgentLoopIntent(map[string]any{"name": defaultAgentLoopIntentName}) {
		t.Fatal("expected steering and intent helpers to cover follow-up guidance branches")
	}
	definitions := configuredService.agentLoopToolDefinitions()
	if len(definitions) == 0 {
		t.Fatal("expected agentLoopToolDefinitions to expose a bounded tool set")
	}
	plannerInput := buildAgentLoopPlannerInput("hello", []string{"obs-1", "obs-2", "obs-3"}, 10, 1)
	if !strings.Contains(plannerInput, "Observed tool results") || !strings.Contains(summarizeAgentLoopHistory([]string{"obs-1", "obs-2"}, 20), "Compressed earlier observations") || singleLineSummary("a\n b") != "a b" {
		t.Fatal("expected planner input helpers to compact history")
	}
	annotated := annotateLoopRound(tools.ToolCallRecord{}, 2)
	if annotated.Output["loop_round"] != 2 {
		t.Fatalf("expected annotateLoopRound to attach loop_round, got %+v", annotated)
	}
}

func mustPathPolicy(t *testing.T) *platform.LocalPathPolicy {
	t.Helper()
	policy, err := platform.NewLocalPathPolicy(filepath.Join(t.TempDir(), "workspace"))
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	return policy
}
