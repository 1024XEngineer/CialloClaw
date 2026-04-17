// Orchestrator service tests cover the main task flow and RPC-facing integration points.
package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
	_ "modernc.org/sqlite"
)

// TestServiceStartTaskAndConfirmFlow 验证确认后的普通任务会继续执行并完成交付。
type stubModelClient struct {
	output string
}

type failingExecutionBackend struct {
	err error
}

func (b failingExecutionBackend) RunCommand(_ context.Context, _ string, _ []string, _ string) (tools.CommandExecutionResult, error) {
	return tools.CommandExecutionResult{}, b.err
}

type successfulExecutionBackend struct {
	result tools.CommandExecutionResult
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

func (b successfulExecutionBackend) RunCommand(_ context.Context, _ string, _ []string, _ string) (tools.CommandExecutionResult, error) {
	if b.result.ExitCode == 0 && b.result.Stdout == "" && b.result.Stderr == "" {
		return tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}, nil
	}
	return b.result, nil
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

type failingCheckpointWriter struct {
	err error
}

func (w failingCheckpointWriter) WriteRecoveryPoint(_ context.Context, _ checkpoint.RecoveryPoint) error {
	return w.err
}

type failingTodoStore struct {
	base       storage.TodoStore
	replaceErr error
}

type countingTaskRunStore struct {
	base      storage.TaskRunStore
	loadCalls int
}

func (s failingTodoStore) ReplaceTodoState(ctx context.Context, items []storage.TodoItemRecord, rules []storage.RecurringRuleRecord) error {
	if s.replaceErr != nil {
		return s.replaceErr
	}
	if s.base == nil {
		return nil
	}
	return s.base.ReplaceTodoState(ctx, items, rules)
}

func (s failingTodoStore) LoadTodoState(ctx context.Context) ([]storage.TodoItemRecord, []storage.RecurringRuleRecord, error) {
	if s.base == nil {
		return nil, nil, nil
	}
	return s.base.LoadTodoState(ctx)
}

func (s *countingTaskRunStore) AllocateIdentifier(ctx context.Context, prefix string) (string, error) {
	return s.base.AllocateIdentifier(ctx, prefix)
}

func (s *countingTaskRunStore) DeleteTaskRun(ctx context.Context, taskID string) error {
	return s.base.DeleteTaskRun(ctx, taskID)
}

func (s *countingTaskRunStore) SaveTaskRun(ctx context.Context, record storage.TaskRunRecord) error {
	return s.base.SaveTaskRun(ctx, record)
}

func (s *countingTaskRunStore) LoadTaskRuns(ctx context.Context) ([]storage.TaskRunRecord, error) {
	s.loadCalls++
	return s.base.LoadTaskRuns(ctx)
}

func (s stubModelClient) GenerateText(_ context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	return model.GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  "req_test",
		Provider:   "openai_responses",
		ModelID:    "gpt-5.4",
		OutputText: s.output,
		Usage: model.TokenUsage{
			InputTokens:  12,
			OutputTokens: 24,
			TotalTokens:  36,
		},
		LatencyMS: 42,
	}, nil
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func querySQLiteCount(t *testing.T, databasePath, query string, args ...any) int {
	t.Helper()
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("query sqlite count failed: %v", err)
	}
	return count
}

func newTestServiceWithExecution(t *testing.T, modelOutput string) (*Service, string) {
	return newTestServiceWithExecutionAndPlaywright(t, modelOutput, platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient())
}

func newTestServiceWithExecutionOptions(t *testing.T, modelOutput string, executionBackend tools.ExecutionCapability, checkpointWriter checkpoint.Writer) (*Service, string) {
	return newTestServiceWithExecutionAndPlaywright(t, modelOutput, executionBackend, checkpointWriter, sidecarclient.NewNoopPlaywrightSidecarClient())
}

func newTestServiceWithExecutionAndPlaywright(t *testing.T, modelOutput string, executionBackend tools.ExecutionCapability, checkpointWriter checkpoint.Writer, playwrightClient tools.PlaywrightSidecarClient) (*Service, string) {
	return newTestServiceWithExecutionWorkers(t, modelOutput, executionBackend, checkpointWriter, playwrightClient, sidecarclient.NewNoopOCRWorkerClient(), sidecarclient.NewNoopMediaWorkerClient())
}

func newTestServiceWithExecutionWorkers(t *testing.T, modelOutput string, executionBackend tools.ExecutionCapability, checkpointWriter checkpoint.Writer, playwrightClient tools.PlaywrightSidecarClient, ocrClient tools.OCRWorkerClient, mediaClient tools.MediaWorkerClient) (*Service, string) {
	t.Helper()

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "service.db")))
	t.Cleanup(func() { _ = storageService.Close() })
	if checkpointWriter == nil {
		checkpointWriter = storageService.RecoveryPointWriter()
	}
	modelService := model.NewService(modelConfig(), stubModelClient{output: modelOutput})
	auditService := audit.NewService(storageService.AuditWriter())
	deliveryService := delivery.NewService()
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
	pluginService := plugin.NewService()
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executor := execution.NewService(fileSystem, executionBackend, playwrightClient, ocrClient, mediaClient, modelService, auditService, checkpoint.NewService(checkpointWriter), deliveryService, toolRegistry, toolExecutor, pluginService)

	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		mustNewStoredEngine(t, storageService.TaskRunStore()),
		deliveryService,
		memory.NewServiceFromStorage(storageService.MemoryStore(), storageService.Capabilities().MemoryRetrievalBackend),
		risk.NewService(),
		modelService,
		toolRegistry,
		pluginService,
	).WithAudit(auditService).WithStorage(storageService).WithExecutor(executor).WithTaskInspector(taskinspector.NewService(fileSystem))

	return service, workspaceRoot
}

func mustNewStoredEngine(t *testing.T, taskStore storage.TaskRunStore) *runengine.Engine {
	t.Helper()
	engine, err := runengine.NewEngineWithStore(taskStore)
	if err != nil {
		t.Fatalf("new stored engine: %v", err)
	}
	return engine
}

func newTestService() *Service {
	return NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)
}

func mutateRuntimeTask(t *testing.T, engine *runengine.Engine, taskID string, mutate func(record *runengine.TaskRecord)) {
	t.Helper()

	engineValue := reflect.ValueOf(engine).Elem()
	muField := engineValue.FieldByName("mu")
	mu := (*sync.RWMutex)(unsafe.Pointer(muField.UnsafeAddr()))
	mu.Lock()
	defer mu.Unlock()

	tasksField := engineValue.FieldByName("tasks")
	tasks := reflect.NewAt(tasksField.Type(), unsafe.Pointer(tasksField.UnsafeAddr())).Elem()
	recordValue := tasks.MapIndex(reflect.ValueOf(taskID))
	if !recordValue.IsValid() || recordValue.IsNil() {
		t.Fatalf("expected runtime task %s to exist", taskID)
	}
	record := recordValue.Interface().(*runengine.TaskRecord)
	mutate(record)
}

func replaceTaskRunStore(t *testing.T, service *storage.Service, store storage.TaskRunStore) {
	t.Helper()

	serviceValue := reflect.ValueOf(service).Elem()
	field := serviceValue.FieldByName("taskRunStore")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(store))
}

func TestServiceStartTaskAndConfirmFlow(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "这里是一段需要解释的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	startedTask := startResult["task"].(map[string]any)
	if startedTask["status"] != "confirming_intent" {
		t.Fatalf("expected confirming_intent status, got %v", startedTask["status"])
	}

	taskID := startedTask["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	confirmedTask := confirmResult["task"].(map[string]any)
	if confirmedTask["status"] != "completed" {
		t.Fatalf("expected completed status after confirmation, got %v", confirmedTask["status"])
	}

	deliveryResult, ok := confirmResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected confirmation flow to return delivery_result")
	}
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected explain intent to deliver by bubble, got %v", deliveryResult["type"])
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected confirmed task to remain available in runtime")
	}
	if record.Status != "completed" {
		t.Fatalf("expected runtime task to be completed, got %s", record.Status)
	}
	if len(record.MemoryWritePlans) == 0 {
		t.Fatal("expected confirmation flow to attach memory write plans")
	}
	if record.DeliveryResult == nil {
		t.Fatal("expected confirmation flow to persist delivery result")
	}
}

func TestServiceSubmitInputKeepsUnknownShortTextInIntentConfirmation(t *testing.T) {
	service := newTestService()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_text",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected unknown short text to remain in confirming_intent, got %v", task["status"])
	}
	intentValue, ok := task["intent"].(map[string]any)
	if !ok || len(intentValue) != 0 {
		t.Fatalf("expected unknown short text task to keep empty intent payload, got %+v", task["intent"])
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["text"] != "我还不确定你想如何处理这段内容，请确认目标。" {
		t.Fatalf("expected neutral confirmation prompt, got %v", bubble["text"])
	}
	if result["delivery_result"] != nil {
		t.Fatalf("expected no delivery result before intent is confirmed, got %+v", result["delivery_result"])
	}
	if _, ok := service.runEngine.GetTask(task["task_id"].(string)); !ok {
		t.Fatal("expected task to remain available in runtime")
	}
}

func TestServiceSubmitInputRoutesClearCommandToAgentLoopWithoutForcedConfirmation(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Translated note ready.")

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_clear_command",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Translate this note into English",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["status"] != "completed" {
		t.Fatalf("expected clear command to execute directly, got %v", task["status"])
	}
	intentValue, ok := task["intent"].(map[string]any)
	if !ok || intentValue["name"] != "agent_loop" {
		t.Fatalf("expected clear command to route through agent_loop, got %+v", task["intent"])
	}
	deliveryResult, ok := result["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected direct command to return delivery_result")
	}
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected short command to prefer bubble delivery, got %v", deliveryResult["type"])
	}
}

func TestServiceSubmitInputUsesSuggestedWorkspaceDeliveryForLongAgentLoopInput(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "Long-form result body.")

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_long_command",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please review the following document notes and prepare a detailed deliverable:\nLine one explains the rollout plan.\nLine two adds implementation details.\nLine three adds follow-up tasks.",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	deliveryResult, ok := result["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected long direct command to return delivery_result")
	}
	if deliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected long agent loop command to prefer workspace_document, got %v", deliveryResult["type"])
	}
	payload := deliveryResult["payload"].(map[string]any)
	outputPath := payload["path"].(string)
	if outputPath == "" {
		t.Fatal("expected workspace delivery to carry a path")
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, strings.TrimPrefix(outputPath, "workspace/"))); err != nil {
		t.Fatalf("expected workspace delivery file to exist, got %v", err)
	}
}

func TestServiceSubmitInputQueuesDirectAgentLoopTaskBehindSameSessionWork(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Queued task output.")

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_serial",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please write this into a file after authorization.",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path":           "workspace_document",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("first start task failed: %v", err)
	}
	if firstResult["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected first task to wait for authorization, got %+v", firstResult["task"])
	}

	secondResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_serial",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Translate this note into English",
		},
	})
	if err != nil {
		t.Fatalf("second submit input failed: %v", err)
	}

	secondTask := secondResult["task"].(map[string]any)
	if secondTask["status"] != "blocked" {
		t.Fatalf("expected second task to be queued as blocked, got %+v", secondTask)
	}
	if secondTask["current_step"] != "session_queue" {
		t.Fatalf("expected queued task current_step=session_queue, got %+v", secondTask)
	}
	if secondResult["delivery_result"] != nil {
		t.Fatalf("expected queued task not to return delivery_result yet, got %+v", secondResult["delivery_result"])
	}
}

func TestServiceConfirmTaskQueuesCorrectedTaskBehindSameSessionWork(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Queued confirm output.")

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_confirm_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please write this into a file after authorization.",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path":           "workspace_document",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("first start task failed: %v", err)
	}
	if firstResult["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected first task to wait for authorization, got %+v", firstResult["task"])
	}

	secondResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_confirm_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "ok",
		},
	})
	if err != nil {
		t.Fatalf("second submit input failed: %v", err)
	}
	secondTaskID := secondResult["task"].(map[string]any)["task_id"].(string)

	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   secondTaskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name":      "agent_loop",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}
	confirmedTask := confirmResult["task"].(map[string]any)
	if confirmedTask["status"] != "blocked" || confirmedTask["current_step"] != "session_queue" {
		t.Fatalf("expected corrected task to queue behind active session work, got %+v", confirmedTask)
	}
	if confirmResult["delivery_result"] != nil {
		t.Fatalf("expected queued corrected task not to return delivery_result, got %+v", confirmResult["delivery_result"])
	}
}

func TestServiceTaskControlCancelQueuedTaskDoesNotResumeWhileSessionBusy(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Queued cancel output.")

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_cancel_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please write this into a file after authorization.",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path":           "workspace_document",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("first start task failed: %v", err)
	}
	firstTaskID := firstResult["task"].(map[string]any)["task_id"].(string)

	secondResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_cancel_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Translate this note into English",
		},
	})
	if err != nil {
		t.Fatalf("second submit input failed: %v", err)
	}
	secondTaskID := secondResult["task"].(map[string]any)["task_id"].(string)

	thirdResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_cancel_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Summarize this release note for me",
		},
	})
	if err != nil {
		t.Fatalf("third submit input failed: %v", err)
	}
	thirdTaskID := thirdResult["task"].(map[string]any)["task_id"].(string)

	if _, err := service.TaskControl(map[string]any{
		"task_id": secondTaskID,
		"action":  "cancel",
	}); err != nil {
		t.Fatalf("cancel queued task failed: %v", err)
	}

	thirdTask, ok := service.runEngine.GetTask(thirdTaskID)
	if !ok {
		t.Fatal("expected third task to remain available in runtime")
	}
	if thirdTask.Status != "blocked" || thirdTask.CurrentStep != "session_queue" {
		t.Fatalf("expected later queued task to remain queued while first task still owns the session, got %+v", thirdTask)
	}

	firstTask, ok := service.runEngine.GetTask(firstTaskID)
	if !ok || firstTask.Status != "waiting_auth" {
		t.Fatalf("expected first task to remain the active session owner, got %+v ok=%v", firstTask, ok)
	}
}

func TestServiceSecurityRespondResumesQueuedTaskWithOriginalSnapshot(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Snapshot resume output.")

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_snapshot_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please write this into a file after authorization.",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path":           "workspace_document",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("first start task failed: %v", err)
	}
	firstTaskID := firstResult["task"].(map[string]any)["task_id"].(string)

	secondResult, err := service.StartTask(map[string]any{
		"session_id": "sess_snapshot_queue",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "Selected source text",
		},
		"context": map[string]any{
			"selection": map[string]any{
				"text": "Selected source text",
			},
			"files": []any{"workspace/docs/input.md"},
			"page": map[string]any{
				"title":    "Release Notes",
				"url":      "https://example.com/release",
				"app_name": "browser",
			},
		},
		"intent": map[string]any{
			"name":      "agent_loop",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("second start task failed: %v", err)
	}
	secondTaskID := secondResult["task"].(map[string]any)["task_id"].(string)

	if _, err := service.SecurityRespond(map[string]any{
		"task_id":       firstTaskID,
		"approval_id":   "appr_snapshot_queue",
		"decision":      "allow_once",
		"remember_rule": false,
	}); err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	secondTask, ok := service.runEngine.GetTask(secondTaskID)
	if !ok {
		t.Fatal("expected resumed task to remain available")
	}
	if secondTask.Status != "completed" {
		t.Fatalf("expected queued task to resume and complete, got %+v", secondTask)
	}
	if secondTask.Snapshot.SelectionText != "Selected source text" {
		t.Fatalf("expected selection text to survive queue resume, got %+v", secondTask.Snapshot)
	}
	if len(secondTask.Snapshot.Files) != 1 || secondTask.Snapshot.Files[0] != "workspace/docs/input.md" {
		t.Fatalf("expected file list to survive queue resume, got %+v", secondTask.Snapshot)
	}
	if secondTask.Snapshot.PageTitle != "Release Notes" || secondTask.Snapshot.PageURL != "https://example.com/release" {
		t.Fatalf("expected page metadata to survive queue resume, got %+v", secondTask.Snapshot)
	}
}

func TestServiceSecurityRespondResumesQueuedSessionTask(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Queued task resumed output.")

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_resume_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please write this into a file after authorization.",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path":           "workspace_document",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("first start task failed: %v", err)
	}
	firstTaskID := firstResult["task"].(map[string]any)["task_id"].(string)

	secondResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_resume_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Translate this note into English",
		},
	})
	if err != nil {
		t.Fatalf("second submit input failed: %v", err)
	}
	secondTaskID := secondResult["task"].(map[string]any)["task_id"].(string)

	if _, err := service.SecurityRespond(map[string]any{
		"task_id":       firstTaskID,
		"approval_id":   "appr_resume_queue",
		"decision":      "allow_once",
		"remember_rule": false,
	}); err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	secondTask, ok := service.runEngine.GetTask(secondTaskID)
	if !ok {
		t.Fatal("expected queued second task to remain available in runtime")
	}
	if secondTask.Status != "completed" {
		t.Fatalf("expected queued second task to resume and complete, got %+v", secondTask)
	}
	if secondTask.CurrentStep != "return_result" {
		t.Fatalf("expected resumed task to finish through return_result, got %+v", secondTask)
	}
}

func TestServiceConfirmTaskRejectsUnknownIntentWithoutCorrection(t *testing.T) {
	service := newTestService()

	startResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_confirm",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	task := confirmResult["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected task to remain in confirming_intent when no corrected intent is provided, got %v", task["status"])
	}
	bubble := confirmResult["bubble_message"].(map[string]any)
	if bubble["text"] != "请先明确告诉我你希望执行的处理方式。" {
		t.Fatalf("expected clarification bubble, got %v", bubble["text"])
	}
	if confirmResult["delivery_result"] != nil {
		t.Fatalf("expected no delivery result while intent is still missing, got %+v", confirmResult["delivery_result"])
	}
}

func TestServiceConfirmTaskKeepsUnknownIntentInConfirmationWhenRejected(t *testing.T) {
	service := newTestService()

	startResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_cancel",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	task := confirmResult["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected rejected unknown intent task to remain in confirming_intent, got %v", task["status"])
	}
	if task["intent"] != nil {
		intentValue, ok := task["intent"].(map[string]any)
		if !ok || len(intentValue) != 0 {
			t.Fatalf("expected rejected unknown intent task to clear its current intent, got %+v", task["intent"])
		}
	}
	bubble := confirmResult["bubble_message"].(map[string]any)
	if bubble["text"] != "这不是我该做的处理方式。请重新说明你的目标，或给我一个更准确的处理意图。" {
		t.Fatalf("expected reconfirm bubble, got %v", bubble["text"])
	}
	if confirmResult["delivery_result"] != nil {
		t.Fatalf("expected rejected unknown intent task not to return delivery_result, got %+v", confirmResult["delivery_result"])
	}
}

func TestServiceConfirmTaskRewritesPlaceholderTitleAfterCorrection(t *testing.T) {
	service := newTestService()

	startResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_title",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name":      "translate",
			"arguments": map[string]any{"target_language": "en"},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	task := confirmResult["task"].(map[string]any)
	if task["title"] != "翻译：你好" {
		t.Fatalf("expected corrected intent to rewrite placeholder title, got %v", task["title"])
	}
}

func TestServiceConfirmTaskIgnoresCorrectedIntentWhenConfirmedTrue(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Explained content.")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_confirm_ignore_correction",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "这里是一段需要解释的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	startTask := startResult["task"].(map[string]any)

	taskID := startTask["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
		"corrected_intent": map[string]any{
			"name": "translate",
			"arguments": map[string]any{
				"target_language": "en",
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	task := confirmResult["task"].(map[string]any)
	intentValue, ok := task["intent"].(map[string]any)
	if !ok || !reflect.DeepEqual(intentValue, startTask["intent"]) {
		t.Fatalf("expected confirm=true to keep the original task intent, got %+v", task["intent"])
	}
	if task["title"] != startTask["title"] {
		t.Fatalf("expected confirm=true to keep the original title, got %v", task["title"])
	}
}

// TestServiceConfirmTaskRejectsOutOfPhaseRequest ensures stale confirm requests
// cannot rewrite tasks that already moved beyond the confirmation phase.
func TestServiceConfirmTaskRejectsOutOfPhaseRequest(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_confirm_out_of_phase",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "请生成一个文件版本",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("seed confirm task failed: %v", err)
	}

	recordedTask, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected seeded task to remain available")
	}
	originalTitle := recordedTask.Title
	originalIntent := cloneMap(recordedTask.Intent)

	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
	})
	if !errors.Is(err, ErrTaskStatusInvalid) {
		t.Fatalf("expected out-of-phase confirm to return ErrTaskStatusInvalid, got %v", err)
	}

	recordedTask, ok = service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain available after rejected confirm")
	}
	if recordedTask.Title != originalTitle {
		t.Fatalf("expected out-of-phase confirm not to rewrite title, got %q want %q", recordedTask.Title, originalTitle)
	}
	if !reflect.DeepEqual(recordedTask.Intent, originalIntent) {
		t.Fatalf("expected out-of-phase confirm not to rewrite intent, got %+v want %+v", recordedTask.Intent, originalIntent)
	}
}

func TestTaskInspectorRunAggregatesRuntimeState(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "inspector output")
	now := time.Now().UTC()
	dueToday := now.Add(15 * time.Minute)
	if dueToday.Day() != now.Day() {
		dueToday = now.Add(1 * time.Minute)
	}

	service.runEngine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_today",
			"title":            "translate release notes",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           dueToday.Format(time.RFC3339),
			"agent_suggestion": "translate",
		},
	})

	todosDir := filepath.Join(workspaceRoot, "todos")
	if err := os.MkdirAll(todosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(todosDir, "inbox.md"), []byte("- [ ] review task\n- [x] archive task\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	result, err := service.TaskInspectorRun(map[string]any{
		"reason":         "startup_scan",
		"target_sources": []any{"workspace/todos"},
	})
	if err != nil {
		t.Fatalf("TaskInspectorRun returned error: %v", err)
	}

	inspectionID, ok := result["inspection_id"].(string)
	if !ok || !strings.HasPrefix(inspectionID, "insp_") {
		t.Fatalf("expected runtime inspection_id, got %+v", result["inspection_id"])
	}

	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary payload, got %+v", result["summary"])
	}
	if summary["parsed_files"] != 1 {
		t.Fatalf("expected parsed_files to reflect workspace scan, got %+v", summary)
	}
	if summary["identified_items"] != 1 {
		t.Fatalf("expected identified_items to reflect source-backed open notes, got %+v", summary)
	}
	if summary["due_today"] != 0 {
		t.Fatalf("expected source-backed notes to replace runtime due buckets after scan, got %+v", summary)
	}

	suggestions, ok := result["suggestions"].([]string)
	if !ok || len(suggestions) == 0 {
		t.Fatalf("expected runtime suggestions, got %+v", result["suggestions"])
	}

	items, total := service.runEngine.NotepadItems("", 10, 0)
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected inspector run to sync parsed notes into runtime, total=%d len=%d", total, len(items))
	}
	if items[0]["item_id"] == "todo_today" && items[1]["item_id"] == "todo_today" {
		t.Fatalf("expected source-backed notes to replace prior runtime sample, got %+v", items)
	}
}

func TestTaskInspectorRunClearsStaleSourceBackedNotesWhenFilesEmpty(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "inspector clear")
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_stale_source",
		"title":   "stale source note",
		"bucket":  "upcoming",
		"status":  "normal",
		"type":    "one_time",
	}})
	todosDir := filepath.Join(workspaceRoot, "todos")
	if err := os.MkdirAll(todosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(todosDir, "empty.md"), []byte("# no checklist items here\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := service.TaskInspectorRun(map[string]any{"target_sources": []any{"workspace/todos"}}); err != nil {
		t.Fatalf("TaskInspectorRun returned error: %v", err)
	}
	items, total := service.runEngine.NotepadItems("", 10, 0)
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected source-backed sync to clear stale runtime notes when source is empty, total=%d len=%d items=%+v", total, len(items), items)
	}
}

func TestServiceNotepadListReturnsRuntimeItemsByBucket(t *testing.T) {
	service := newTestService()
	now := time.Now().UTC()
	service.runEngine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":                "todo_today",
			"title":                  "translate daily notes",
			"bucket":                 "upcoming",
			"status":                 "normal",
			"type":                   "todo_item",
			"due_at":                 now.Add(2 * time.Hour).Format(time.RFC3339),
			"agent_suggestion":       "translate",
			"note_text":              "Bring the daily notes into English for the external sync.",
			"prerequisite":           "Confirm the final Chinese source text first.",
			"repeat_rule":            nil,
			"next_occurrence_at":     nil,
			"recent_instance_status": nil,
			"effective_scope":        nil,
			"ended_at":               nil,
			"related_resources": []map[string]any{
				{
					"resource_id":   "todo_today_resource",
					"label":         "Daily note draft",
					"path":          "workspace/daily.md",
					"resource_type": "file",
				},
			},
		},
		{
			"item_id":          "todo_later",
			"title":            "rewrite later draft",
			"bucket":           "later",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(48 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "rewrite",
		},
	})

	result, err := service.NotepadList(map[string]any{
		"group":  "upcoming",
		"limit":  float64(20),
		"offset": float64(0),
	})
	if err != nil {
		t.Fatalf("notepad list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one upcoming notepad item, got %d", len(items))
	}
	if items[0]["item_id"] != "todo_today" {
		t.Fatalf("expected runtime list to keep todo_today, got %+v", items[0])
	}
	if items[0]["status"] != "due_today" {
		t.Fatalf("expected runtime list to normalize due_today status, got %v", items[0]["status"])
	}
	if items[0]["note_text"] != "Bring the daily notes into English for the external sync." {
		t.Fatalf("expected note_text to survive list response, got %+v", items[0]["note_text"])
	}
	resources, ok := items[0]["related_resources"].([]map[string]any)
	if !ok || len(resources) != 1 || resources[0]["resource_id"] != "todo_today_resource" {
		t.Fatalf("expected related_resources to survive list response, got %+v", items[0]["related_resources"])
	}
}

func TestServiceNotepadConvertToTaskUsesRuntimeItemWithoutClosingTodo(t *testing.T) {
	service := newTestService()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	service.runEngine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_translate",
			"title":            "translate the meeting notes",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(3 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "translate into English",
		},
	})

	result, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_translate",
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("notepad convert failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["title"] != "translate the meeting notes" {
		t.Fatalf("expected converted task title to come from runtime notepad item, got %v", task["title"])
	}
	if task["source_type"] != "todo" {
		t.Fatalf("expected converted task source_type todo, got %v", task["source_type"])
	}

	intentValue := task["intent"].(map[string]any)
	if intentValue["name"] != "translate" {
		t.Fatalf("expected runtime notepad conversion to infer translate intent, got %v", intentValue["name"])
	}

	taskID := task["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected converted task to remain available in runtime")
	}
	if len(record.MemoryReadPlans) == 0 {
		t.Fatal("expected converted task to attach memory read plans")
	}

	sourceItem := result["notepad_item"].(map[string]any)
	if sourceItem["linked_task_id"] != taskID {
		t.Fatalf("expected convert_to_task to return linked source item, got %+v", sourceItem)
	}
	refreshGroups := result["refresh_groups"].([]string)
	if len(refreshGroups) != 1 || refreshGroups[0] != "upcoming" {
		t.Fatalf("expected refresh_groups to point at updated bucket, got %+v", refreshGroups)
	}

	upcomingItems, total := service.runEngine.NotepadItems("upcoming", 10, 0)
	if total != 1 || len(upcomingItems) != 1 {
		t.Fatalf("expected converted todo item to stay open until task finishes, total=%d len=%d", total, len(upcomingItems))
	}
	if upcomingItems[0]["item_id"] != "todo_translate" || upcomingItems[0]["status"] == "completed" {
		t.Fatalf("expected notepad item to remain open, got %+v", upcomingItems[0])
	}
	if upcomingItems[0]["linked_task_id"] != taskID {
		t.Fatalf("expected runtime notepad item to keep linked_task_id, got %+v", upcomingItems[0])
	}
}

func TestServiceNotepadConvertToTaskRequiresConfirmedFlag(t *testing.T) {
	service := newTestService()
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_confirm",
		"title":   "translate release draft",
		"bucket":  "upcoming",
		"status":  "normal",
		"type":    "todo_item",
	}})

	_, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_confirm",
		"confirmed": false,
	})
	if err == nil {
		t.Fatal("expected convert_to_task to reject unconfirmed requests")
	}
	if err.Error() != "confirmed must be true to convert notepad item" {
		t.Fatalf("expected confirmed validation error, got %v", err)
	}

	items, total := service.runEngine.NotepadItems("upcoming", 10, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected notepad item to remain untouched after rejected convert, total=%d len=%d", total, len(items))
	}
}

func TestServiceNotepadConvertToTaskRejectsAlreadyLinkedItem(t *testing.T) {
	service := newTestService()
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id":        "todo_linked",
		"title":          "already linked note",
		"bucket":         "upcoming",
		"status":         "normal",
		"type":           "todo_item",
		"linked_task_id": "task_existing",
	}})

	_, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_linked",
		"confirmed": true,
	})
	if err == nil {
		t.Fatal("expected convert_to_task to reject already linked item")
	}
	if err.Error() != "notepad item is already linked to task: task_existing" {
		t.Fatalf("expected linked item error, got %v", err)
	}
}

func TestServiceNotepadConvertToTaskRejectsInFlightClaim(t *testing.T) {
	service := newTestService()
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_claimed",
		"title":   "claimed note",
		"bucket":  "upcoming",
		"status":  "normal",
		"type":    "todo_item",
	}})
	if _, handled, err := service.runEngine.ClaimNotepadItemTask("todo_claimed"); err != nil || !handled {
		t.Fatalf("expected runtime claim to succeed before convert, handled=%v err=%v", handled, err)
	}

	_, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_claimed",
		"confirmed": true,
	})
	if err == nil {
		t.Fatal("expected convert_to_task to reject in-flight claim")
	}
	if err.Error() != "notepad item is already being converted: todo_claimed" {
		t.Fatalf("expected in-flight conversion error, got %v", err)
	}
}

func TestServiceNotepadConvertToTaskRollsBackTaskWhenLinkPersistenceFails(t *testing.T) {
	taskStore := storage.NewInMemoryTaskRunStore()
	engine, err := runengine.NewEngineWithStore(taskStore)
	if err != nil {
		t.Fatalf("new stored engine failed: %v", err)
	}
	baseTodoStore := storage.NewInMemoryTodoStore()
	if err := engine.WithTodoStore(baseTodoStore); err != nil {
		t.Fatalf("attach base todo store failed: %v", err)
	}
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_link_failure",
		"title":   "convert with failing note persistence",
		"bucket":  "upcoming",
		"status":  "normal",
		"type":    "todo_item",
	}})
	if err := engine.WithTodoStore(failingTodoStore{
		base:       baseTodoStore,
		replaceErr: errors.New("todo replace failed"),
	}); err != nil {
		t.Fatalf("swap to failing todo store failed: %v", err)
	}

	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		engine,
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	_, err = service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_link_failure",
		"confirmed": true,
	})
	if err == nil {
		t.Fatal("expected convert_to_task to fail when note persistence fails")
	}
	if !strings.Contains(err.Error(), "failed to link notepad item to task: todo_link_failure") {
		t.Fatalf("expected link failure in error message, got %v", err)
	}

	if tasks, total := engine.ListTasks("unfinished", "updated_at", "desc", 10, 0); total != 0 || len(tasks) != 0 {
		t.Fatalf("expected rollback to remove runtime task, total=%d tasks=%+v", total, tasks)
	}
	persisted, loadErr := taskStore.LoadTaskRuns(context.Background())
	if loadErr != nil {
		t.Fatalf("load persisted task runs failed: %v", loadErr)
	}
	if len(persisted) != 0 {
		t.Fatalf("expected rollback to remove persisted task run, got %+v", persisted)
	}

	item, ok := engine.NotepadItem("todo_link_failure")
	if !ok {
		t.Fatal("expected note to remain available after rollback")
	}
	if linkedTaskID := stringValue(item, "linked_task_id", ""); linkedTaskID != "" {
		t.Fatalf("expected note to remain unlinked after rollback, got %+v", item)
	}
}

func TestServiceNotepadUpdateReturnsUpdatedItemAndRefreshGroups(t *testing.T) {
	service := newTestService()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_later_update",
		"title":   "move later note",
		"bucket":  "later",
		"status":  "normal",
		"type":    "todo_item",
		"due_at":  now.Add(48 * time.Hour).Format(time.RFC3339),
	}})

	result, err := service.NotepadUpdate(map[string]any{
		"item_id": "todo_later_update",
		"action":  "move_upcoming",
	})
	if err != nil {
		t.Fatalf("notepad update failed: %v", err)
	}

	updatedItem := result["notepad_item"].(map[string]any)
	if updatedItem["bucket"] != "upcoming" {
		t.Fatalf("expected updated item bucket upcoming, got %+v", updatedItem)
	}
	refreshGroups := result["refresh_groups"].([]string)
	if len(refreshGroups) != 2 || refreshGroups[0] != "later" || refreshGroups[1] != "upcoming" {
		t.Fatalf("expected refresh_groups to include source and target buckets, got %+v", refreshGroups)
	}
}

func TestServiceNotepadUpdateReturnsDeletedItemID(t *testing.T) {
	service := newTestService()
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_delete_rpc",
		"title":   "delete me",
		"bucket":  "closed",
		"status":  "completed",
		"type":    "todo_item",
	}})

	result, err := service.NotepadUpdate(map[string]any{
		"item_id": "todo_delete_rpc",
		"action":  "delete",
	})
	if err != nil {
		t.Fatalf("notepad delete failed: %v", err)
	}
	if result["notepad_item"] != nil {
		t.Fatalf("expected deleted item payload to be nil, got %+v", result["notepad_item"])
	}
	if result["deleted_item_id"] != "todo_delete_rpc" {
		t.Fatalf("expected deleted_item_id in response, got %+v", result["deleted_item_id"])
	}
}

func TestServiceExecutionAuditIDsStayUniqueAcrossToolAndTaskRecords(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "runtime output")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_audit",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "解释一下这段内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	recordedTask, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain available")
	}
	if len(recordedTask.AuditRecords) == 0 {
		t.Fatal("expected task audit records to be appended")
	}

	firstTaskAuditID, _ := recordedTask.AuditRecords[0]["audit_id"].(string)
	if firstTaskAuditID == "" {
		t.Fatalf("expected persisted task audit id, got %+v", recordedTask.AuditRecords[0])
	}
	if firstTaskAuditID == "audit_001" {
		t.Fatalf("expected shared audit service to advance ids before task audit persistence, got %q", firstTaskAuditID)
	}
}

func TestServiceRecommendationGetUsesRuntimeTaskState(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_recommend",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "tiny note",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskTitle := startResult["task"].(map[string]any)["title"].(string)
	result, err := service.RecommendationGet(map[string]any{
		"source": "floating_ball",
		"scene":  "hover",
		"context": map[string]any{
			"page_title": "Dashboard",
			"app_name":   "desktop",
		},
	})
	if err != nil {
		t.Fatalf("recommendation get failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) == 0 {
		t.Fatal("expected runtime recommendation items")
	}
	if !strings.Contains(items[0]["text"].(string), taskTitle) {
		t.Fatalf("expected recommendation text to reference runtime task title, got %v", items[0]["text"])
	}
}

func TestServiceRecommendationGetUsesPerceptionSignals(t *testing.T) {
	service := newTestService()
	service.runEngine.ReplaceNotepadItems(nil)
	result, err := service.RecommendationGet(map[string]any{
		"source": "floating_ball",
		"scene":  "hover",
		"context": map[string]any{
			"page_title":          "Release Checklist",
			"app_name":            "browser",
			"clipboard_text":      "请 translate this paragraph into English before sharing externally.",
			"visible_text":        "Warning: release notes are incomplete.",
			"dwell_millis":        18000,
			"copy_count":          1,
			"window_switch_count": 3,
			"last_action":         "copy",
		},
	})
	if err != nil {
		t.Fatalf("recommendation get failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) == 0 {
		t.Fatal("expected recommendation items from perception signals")
	}
	if items[0]["intent"].(map[string]any)["name"] != "translate" {
		t.Fatalf("expected copy behavior to prioritize translate, got %+v", items[0])
	}
}

func TestMemoryQueryFromSnapshotKeepsExplicitTaskInputAheadOfClipboard(t *testing.T) {
	snapshot := contextsvc.TaskContextSnapshot{
		Text:          "explicit task input",
		ClipboardText: "stale copied content",
		VisibleText:   "visible page context",
	}
	if memoryQueryFromSnapshot(snapshot) != "explicit task input" {
		t.Fatalf("expected explicit task text to outrank clipboard, got %q", memoryQueryFromSnapshot(snapshot))
	}
}

func TestServiceRecommendationFeedbackSubmitAppliesCooldown(t *testing.T) {
	service := newTestService()
	params := map[string]any{
		"source": "floating_ball",
		"scene":  "selected_text",
		"context": map[string]any{
			"page_title":     "Article",
			"app_name":       "desktop",
			"selection_text": "This paragraph should be translated before publishing externally.",
		},
	}

	first, err := service.RecommendationGet(params)
	if err != nil {
		t.Fatalf("recommendation get failed: %v", err)
	}
	items := first["items"].([]map[string]any)
	if len(items) == 0 {
		t.Fatal("expected recommendation items before feedback")
	}

	feedbackResult, err := service.RecommendationFeedbackSubmit(map[string]any{
		"recommendation_id": items[0]["recommendation_id"],
		"feedback":          "negative",
	})
	if err != nil {
		t.Fatalf("recommendation feedback submit failed: %v", err)
	}
	if feedbackResult["applied"] != true {
		t.Fatalf("expected recommendation feedback to apply, got %+v", feedbackResult)
	}

	second, err := service.RecommendationGet(params)
	if err != nil {
		t.Fatalf("second recommendation get failed: %v", err)
	}
	if second["cooldown_hit"] != true {
		t.Fatalf("expected cooldown hit after negative feedback, got %+v", second)
	}
	if len(second["items"].([]map[string]any)) != 0 {
		t.Fatalf("expected cooldown hit to suppress recommendation items, got %+v", second["items"])
	}
}

func TestServiceSubmitInputWithFilesDoesNotWaitForInput(t *testing.T) {
	service := newTestService()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_files",
		"source":     "floating_ball",
		"input": map[string]any{
			"files": []any{"workspace/notes.md"},
		},
		"context": map[string]any{
			"page": map[string]any{
				"title":    "Workspace",
				"app_name": "desktop",
			},
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["status"] == "waiting_input" {
		t.Fatalf("expected file input to enter task flow instead of waiting_input, got %+v", task)
	}
	if task["source_type"] != "dragged_file" {
		t.Fatalf("expected file input to map to dragged_file source_type, got %v", task["source_type"])
	}
}

// TestServiceSubmitInputEmptyTextReturnsWaitingInput 验证空文本提交会进入 waiting_input。
func TestServiceSubmitInputEmptyTextReturnsWaitingInput(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type":       "text",
			"text":       "   ",
			"input_mode": "text",
		},
		"context": map[string]any{},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["status"] != "waiting_input" {
		t.Fatalf("expected waiting_input status, got %v", task["status"])
	}
	if task["current_step"] != "collect_input" {
		t.Fatalf("expected collect_input current_step, got %v", task["current_step"])
	}
	if task["intent"] != nil {
		intentValue, ok := task["intent"].(map[string]any)
		if !ok || len(intentValue) != 0 {
			t.Fatalf("expected waiting_input task to keep empty intent, got %v", task["intent"])
		}
	}

	bubble := result["bubble_message"].(map[string]any)
	if bubble["type"] != "status" {
		t.Fatalf("expected waiting_input bubble type status, got %v", bubble["type"])
	}

	taskID := task["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected waiting_input task to exist in runtime")
	}
	if record.FinishedAt != nil {
		t.Fatal("expected waiting_input task to keep finished_at nil")
	}
	if len(record.MemoryReadPlans) != 0 || len(record.MemoryWritePlans) != 0 {
		t.Fatal("expected waiting_input task not to attach memory handoff plans")
	}
}

// TestServiceDirectStartBuildsMemoryAndDeliveryHandoffs 验证ServiceDirectStartBuildsMemoryAndDeliveryHandoffs。
func TestServiceDirectStartBuildsMemoryAndDeliveryHandoffs(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "直接总结这段文字",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to exist in runtime")
	}
	if len(record.MemoryReadPlans) == 0 || len(record.MemoryWritePlans) == 0 {
		t.Fatal("expected memory handoff plans to be attached")
	}
	if record.StorageWritePlan == nil || len(record.ArtifactPlans) == 0 {
		t.Fatal("expected delivery handoff plans to be attached")
	}
	if record.FinishedAt == nil {
		t.Fatal("expected direct completion flow to set finished_at only after completion")
	}

	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected notifications to be available")
	}
	hasDeliveryReady := false
	for _, notification := range notifications {
		if notification.Method == "delivery.ready" {
			hasDeliveryReady = true
			break
		}
	}
	if !hasDeliveryReady {
		t.Fatal("expected delivery.ready notification to be queued")
	}
}

// TestServiceStartTaskWaitingAuthDoesNotSetFinishedAt 验证等待授权前不会提前写入 finished_at。
func TestServiceStartTaskRespectsPreferredDelivery(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "direct summarize with bubble delivery",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
		"delivery": map[string]any{
			"preferred": "bubble",
			"fallback":  "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	deliveryResult, ok := startResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected direct start to return delivery_result")
	}
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected preferred bubble delivery, got %v", deliveryResult["type"])
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected direct start task to exist in runtime")
	}
	if record.PreferredDelivery != "bubble" {
		t.Fatalf("expected runtime task to persist preferred delivery, got %q", record.PreferredDelivery)
	}
	if record.FallbackDelivery != "workspace_document" {
		t.Fatalf("expected runtime task to persist fallback delivery, got %q", record.FallbackDelivery)
	}
	if record.StorageWritePlan != nil || len(record.ArtifactPlans) != 0 {
		t.Fatal("expected bubble delivery not to create document persistence plans")
	}
}

func TestServiceSubmitInputRespectsPreferredDelivery(t *testing.T) {
	service := newTestService()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "translate this line",
		},
		"options": map[string]any{
			"confirm_required":   false,
			"preferred_delivery": "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	deliveryResult, ok := result["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected submit input to return delivery_result")
	}
	if deliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected preferred workspace_document delivery, got %v", deliveryResult["type"])
	}

	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		t.Fatal("expected delivery_result payload")
	}
	if payload["path"] == nil {
		t.Fatal("expected workspace_document delivery to include payload path")
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected submit input task to exist in runtime")
	}
	if record.PreferredDelivery != "workspace_document" {
		t.Fatalf("expected runtime task to persist preferred delivery, got %q", record.PreferredDelivery)
	}
}

func TestServiceConfirmTaskRespectsStoredPreferredDelivery(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "selected text for confirmation flow",
		},
		"delivery": map[string]any{
			"preferred": "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	deliveryResult, ok := confirmResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected confirm flow to return delivery_result")
	}
	if deliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected stored preferred workspace_document delivery, got %v", deliveryResult["type"])
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected confirmed task to exist in runtime")
	}
	if record.PreferredDelivery != "workspace_document" {
		t.Fatalf("expected runtime task to keep preferred delivery, got %q", record.PreferredDelivery)
	}
	if record.DeliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected runtime delivery result to use workspace_document, got %v", record.DeliveryResult["type"])
	}
}

func TestServiceStartTaskWaitingAuthDoesNotSetFinishedAt(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "file_drop",
		"input": map[string]any{
			"type":  "file",
			"files": []any{"workspace/input.md"},
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	startedTask := startResult["task"].(map[string]any)
	if startedTask["status"] != "waiting_auth" {
		t.Fatalf("expected waiting_auth status, got %v", startedTask["status"])
	}
	if startedTask["finished_at"] != nil {
		t.Fatalf("expected waiting_auth task to keep finished_at nil, got %v", startedTask["finished_at"])
	}

	taskID := startedTask["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.FinishedAt != nil {
		t.Fatal("expected runtime waiting_auth task to keep finished_at nil")
	}
}

// TestServiceConfirmCanEnterWaitingAuth 验证ServiceConfirmCanEnterWaitingAuth。
func TestServiceConfirmCanEnterWaitingAuth(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "这里是一段需要确认处理方式的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	confirmedTask := confirmResult["task"].(map[string]any)
	if confirmedTask["status"] != "waiting_auth" {
		t.Fatalf("expected waiting_auth status, got %v", confirmedTask["status"])
	}
	if confirmedTask["intent"].(map[string]any)["name"] != "write_file" {
		t.Fatalf("expected corrected intent to be persisted before waiting auth, got %v", confirmedTask["intent"])
	}

	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected notifications to exist for waiting task")
	}
	hasApprovalPending := false
	for _, notification := range notifications {
		if notification.Method == "approval.pending" {
			hasApprovalPending = true
			break
		}
	}
	if !hasApprovalPending {
		t.Fatal("expected approval.pending notification to be queued")
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after entering waiting_auth")
	}
	if record.Intent["name"] != "write_file" {
		t.Fatalf("expected runtime task intent to be updated before waiting auth, got %v", record.Intent)
	}
}

// TestServiceSecurityRespondAllowOnceResumesAndCompletes 验证授权通过后任务会继续执行并完成交付。
func TestServiceSecurityRespondAllowOnceResumesAndCompletes(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "需要授权后继续执行的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_001",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	responseTask := respondResult["task"].(map[string]any)
	if responseTask["status"] != "completed" {
		t.Fatalf("expected response task to reflect finalized completion, got %v", responseTask["status"])
	}
	responseBubble := respondResult["bubble_message"].(map[string]any)
	if responseBubble["type"] != "result" {
		t.Fatalf("expected security respond to return the final result bubble, got %v", responseBubble["type"])
	}
	impactScope := respondResult["impact_scope"].(map[string]any)
	files := impactScope["files"].([]string)
	if len(files) != 1 || files[0] != "workspace/文件写入结果.md" {
		t.Fatalf("expected impact scope files to stay within workspace-relative paths, got %v", files)
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after authorization")
	}
	if record.Status != "completed" {
		t.Fatalf("expected runtime task to complete after resume, got %s", record.Status)
	}
	if record.Authorization == nil {
		t.Fatal("expected authorization record to be stored on runtime task")
	}

	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected notifications to remain available after authorization")
	}
	hasProcessingUpdate := false
	hasDeliveryReady := false
	for _, notification := range notifications {
		if notification.Method == "task.updated" {
			if notification.Params["status"] == "processing" {
				hasProcessingUpdate = true
			}
		}
		if notification.Method == "delivery.ready" {
			hasDeliveryReady = true
		}
	}
	if !hasProcessingUpdate || !hasDeliveryReady {
		t.Fatal("expected resumed processing and delivery notifications to be queued")
	}
	if record.PendingExecution != nil {
		t.Fatal("expected pending execution plan to be cleared after successful authorization")
	}
}

// TestServiceSecurityRespondDenyOnceCancelsTask 验证拒绝授权后任务会结束。
func TestServiceSecurityRespondRespectsFallbackDelivery(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "authorization flow with delivery fallback",
		},
		"delivery": map[string]any{
			"preferred": "unsupported_delivery",
			"fallback":  "bubble",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style":                 "key_points",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	_, err = service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_001",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to exist in runtime after authorization")
	}
	if record.DeliveryResult["type"] != "bubble" {
		t.Fatalf("expected fallback bubble delivery after authorization, got %v", record.DeliveryResult["type"])
	}
	if record.StorageWritePlan != nil || len(record.ArtifactPlans) != 0 {
		t.Fatal("expected bubble fallback delivery not to create document persistence plans")
	}
}

func TestServiceSecurityRespondDenyOnceCancelsTask(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "需要授权后继续执行的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_001",
		"decision":      "deny_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	responseTask := respondResult["task"].(map[string]any)
	if responseTask["status"] != "cancelled" {
		t.Fatalf("expected cancelled task in deny response, got %v", responseTask["status"])
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after denial")
	}
	if record.Status != "cancelled" {
		t.Fatalf("expected runtime task to be cancelled after denial, got %s", record.Status)
	}
	if record.Authorization == nil {
		t.Fatal("expected denial decision to be stored as authorization record")
	}
	if record.PendingExecution != nil {
		t.Fatal("expected pending execution plan to be cleared after denial")
	}
}

func TestServiceStartTaskWriteFileOverwriteWaitsForApproval(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "unused")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes", "output.md"), []byte("旧内容"), 0o644); err != nil {
		t.Fatalf("seed output file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_overwrite",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请覆盖该文件",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": "notes/output.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	task := startResult["task"].(map[string]any)
	if task["status"] != "waiting_auth" {
		t.Fatalf("expected waiting_auth for overwrite risk, got %+v", task)
	}
	if task["risk_level"] != "yellow" {
		t.Fatalf("expected yellow overwrite risk, got %+v", task)
	}
	pendingPlan, ok := service.runEngine.PendingExecutionPlan(task["task_id"].(string))
	if !ok {
		t.Fatal("expected pending execution plan for overwrite task")
	}
	impactScope := pendingPlan["impact_scope"].(map[string]any)
	if impactScope["overwrite_or_delete_risk"] != true {
		t.Fatalf("expected overwrite_or_delete_risk=true, got %+v", impactScope)
	}
}

func TestServiceStartTaskExecCommandWaitsForApproval(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "unused")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec_cmd",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "执行命令",
		},
		"intent": map[string]any{
			"name": "exec_command",
			"arguments": map[string]any{
				"command": "cmd",
				"args":    []any{"/c", "echo", "ok"},
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	task := startResult["task"].(map[string]any)
	if task["status"] != "waiting_auth" {
		t.Fatalf("expected waiting_auth for exec_command, got %+v", task)
	}
	if task["risk_level"] != "yellow" {
		t.Fatalf("expected yellow risk for safe exec_command, got %+v", task)
	}
	pendingPlan, ok := service.runEngine.PendingExecutionPlan(task["task_id"].(string))
	if !ok {
		t.Fatal("expected pending execution plan for exec_command")
	}
	files := pendingPlan["impact_scope"].(map[string]any)["files"].([]string)
	if len(files) != 1 || !strings.Contains(files[0], filepath.Base(workspaceRoot)) {
		t.Fatalf("expected impact scope to include workspace root, got %+v", pendingPlan)
	}
}

func TestServiceStartTaskOutOfWorkspaceWriteIsBlocked(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "unused")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_outside",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "越界写入",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": "../secret.txt",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	task := startResult["task"].(map[string]any)
	if task["status"] != "cancelled" {
		t.Fatalf("expected cancelled task after out-of-workspace deny, got %+v", task)
	}
	record, ok := service.runEngine.GetTask(task["task_id"].(string))
	if !ok {
		t.Fatal("expected blocked task to remain in runtime")
	}
	if stringValue(record.SecuritySummary, "security_status", "") != "intercepted" {
		t.Fatalf("expected intercepted security status, got %+v", record.SecuritySummary)
	}
	if len(record.AuditRecords) == 0 {
		t.Fatal("expected blocked task to record audit trail")
	}
}

func TestServiceSecurityRespondAllowOnceReturnsStructuredExecutionFailure(t *testing.T) {
	service, _ := newTestServiceWithExecutionOptions(t, "unused", failingExecutionBackend{err: errors.New("runner unavailable")}, nil)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec_fail",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "执行命令",
		},
		"intent": map[string]any{
			"name": "exec_command",
			"arguments": map[string]any{
				"command": "cmd",
				"args":    []any{"/c", "echo", "ok"},
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_exec_fail",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "failed" {
		t.Fatalf("expected failed task after execution error, got %+v", respondResult)
	}
	if respondResult["delivery_result"] != nil {
		t.Fatalf("expected no delivery result on execution failure, got %+v", respondResult)
	}
	bubble := respondResult["bubble_message"].(map[string]any)
	if !strings.Contains(stringValue(bubble, "text", ""), "执行失败") {
		t.Fatalf("expected failure bubble, got %+v", bubble)
	}
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected failed task to remain in runtime")
	}
	if stringValue(record.SecuritySummary, "security_status", "") != "execution_error" {
		t.Fatalf("expected execution_error security status, got %+v", record.SecuritySummary)
	}
	if len(record.AuditRecords) == 0 {
		t.Fatal("expected failed execution to append audit records")
	}
	for _, auditRecord := range record.AuditRecords {
		if auditRecord["action"] == "publish_result" {
			t.Fatalf("expected failed execution not to publish delivery audit, got %+v", record.AuditRecords)
		}
	}
}

func TestServiceSecurityRespondAllowOnceExecCommandCompletesAfterApproval(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecutionOptions(t, "unused", successfulExecutionBackend{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}}, nil)
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("mkdir workspace root: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec_allow",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "执行命令",
		},
		"intent": map[string]any{
			"name": "exec_command",
			"arguments": map[string]any{
				"command": "cmd",
				"args":    []any{"/c", "echo", "ok"},
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_exec_allow",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected completed task after approved exec_command, got %+v", respondResult)
	}
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after approved exec_command")
	}
	if record.LatestToolCall["tool_name"] != "exec_command" {
		t.Fatalf("expected exec_command tool trace, got %+v", record.LatestToolCall)
	}
}

func TestServiceSecurityRespondAllowOnceCompletesDerivedWriteFileAfterApproval(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "新的文档内容")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_derived_write",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请总结成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style":                 "key_points",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if startResult["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected derived write flow to wait for auth, got %+v", startResult)
	}

	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_derived_write",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected completed task after approved derived write_file, got %+v", respondResult)
	}
}

func TestServiceSecurityRespondAllowOnceReturnsStructuredRecoveryFailure(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecutionOptions(t, "unused", platform.LocalExecutionBackend{}, failingCheckpointWriter{err: errors.New("checkpoint unavailable")})
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes", "output.md"), []byte("旧内容"), 0o644); err != nil {
		t.Fatalf("seed output file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_recovery_fail",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请覆盖该文件",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": "notes/output.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_recovery_fail",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "failed" {
		t.Fatalf("expected failed task after recovery preparation error, got %+v", respondResult)
	}
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected recovery-failed task to remain in runtime")
	}
	if stringValue(record.SecuritySummary, "security_status", "") != "execution_error" {
		t.Fatalf("expected execution_error security status for recovery failure, got %+v", record.SecuritySummary)
	}
	if len(record.AuditRecords) == 0 {
		t.Fatal("expected recovery failure to append audit records")
	}
	lastAudit := record.AuditRecords[len(record.AuditRecords)-1]
	if lastAudit["action"] != "create_recovery_point" {
		t.Fatalf("expected recovery failure audit action, got %+v", lastAudit)
	}
}

// modelConfig 处理当前模块的相关逻辑。
func TestServiceTaskListSupportsSortParams(t *testing.T) {
	service := newTestService()

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "first finished task",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start first task failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	secondResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "second finished task",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start second task failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "started_at",
		"sort_order": "asc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) < 2 {
		t.Fatalf("expected at least two finished tasks, got %d", len(items))
	}

	firstTaskID := firstResult["task"].(map[string]any)["task_id"].(string)
	secondTaskID := secondResult["task"].(map[string]any)["task_id"].(string)
	if items[0]["task_id"] != firstTaskID || items[1]["task_id"] != secondTaskID {
		t.Fatalf("expected started_at asc order %s -> %s, got %v -> %v", firstTaskID, secondTaskID, items[0]["task_id"], items[1]["task_id"])
	}
}

func TestServiceTaskListFallsBackToStoredTaskRuns(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored task list")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_001",
		SessionID:   "sess_stored",
		RunID:       "run_stored_001",
		Title:       "stored finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 9, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 9, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 9, 6, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) != 1 || items[0]["task_id"] != "task_stored_001" {
		t.Fatalf("expected storage-backed task list item, got %+v", items)
	}
}

func TestServiceTaskListDoesNotFallbackWhenOffsetExceedsRuntimePage(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "runtime paging")

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_page",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime finished task",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start runtime task failed: %v", err)
	}
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_extra",
		SessionID:   "sess_page",
		RunID:       "run_stored_extra",
		Title:       "stored finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 14, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 14, 6, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(100),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) != 0 {
		t.Fatalf("expected empty page beyond runtime total, got %+v", items)
	}
	page := listResult["page"].(map[string]any)
	if page["total"] != 1 {
		t.Fatalf("expected runtime total to stay unchanged, got %+v", page)
	}
}

func TestServiceTaskListClampsPagingParams(t *testing.T) {
	service := newTestService()

	for index := 0; index < 25; index++ {
		_, err := service.StartTask(map[string]any{
			"session_id": fmt.Sprintf("sess_clamp_%02d", index),
			"source":     "floating_ball",
			"trigger":    "hover_text_input",
			"input": map[string]any{
				"type": "text",
				"text": fmt.Sprintf("task %02d for task list clamp", index),
			},
			"intent": map[string]any{
				"name": "write_file",
				"arguments": map[string]any{
					"require_authorization": true,
				},
			},
		})
		if err != nil {
			t.Fatalf("start task %d failed: %v", index, err)
		}
	}

	result, err := service.TaskList(map[string]any{
		"group":      "unfinished",
		"limit":      float64(0),
		"offset":     float64(-5),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list with clamped defaults failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 20 {
		t.Fatalf("expected zero limit to clamp to default page size 20, got %d", len(items))
	}
	page := result["page"].(map[string]any)
	if page["limit"] != 20 {
		t.Fatalf("expected clamped page limit 20, got %+v", page)
	}
	if page["offset"] != 0 {
		t.Fatalf("expected negative offset to clamp to 0, got %+v", page)
	}

	largeResult, err := service.TaskList(map[string]any{
		"group":      "unfinished",
		"limit":      float64(999),
		"offset":     float64(0),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list with large limit failed: %v", err)
	}

	largeItems := largeResult["items"].([]map[string]any)
	if len(largeItems) != 25 {
		t.Fatalf("expected large limit to return all 25 tasks after clamping to 100, got %d", len(largeItems))
	}
	largePage := largeResult["page"].(map[string]any)
	if largePage["limit"] != 100 {
		t.Fatalf("expected oversized limit to clamp to 100, got %+v", largePage)
	}
}

func TestServiceTaskListFallbackMatchesRuntimeUnknownGroupSemantics(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored unknown group")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_unfinished",
		SessionID:   "sess_group",
		RunID:       "run_stored_unfinished",
		Title:       "stored unfinished task",
		SourceType:  "hover_input",
		Status:      "processing",
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("save unfinished task run failed: %v", err)
	}
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_finished",
		SessionID:   "sess_group",
		RunID:       "run_stored_finished",
		Title:       "stored finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 10, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 15, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 15, 16, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save finished task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "unknown_group",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) != 1 || items[0]["task_id"] != "task_stored_unfinished" {
		t.Fatalf("expected unknown group fallback to match runtime unfinished semantics, got %+v", items)
	}
}

func TestServiceTaskListFallbackMatchesRuntimeSortTieBreaker(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored sort tie")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	finishedAt := time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC)
	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_sort_older_update",
		SessionID:   "sess_sort",
		RunID:       "run_sort_old",
		Title:       "older update task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 30, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 40, 0, 0, time.UTC),
		FinishedAt:  timePointer(finishedAt),
	})
	if err != nil {
		t.Fatalf("save first task run failed: %v", err)
	}
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_sort_newer_update",
		SessionID:   "sess_sort",
		RunID:       "run_sort_new",
		Title:       "newer update task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 35, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 50, 0, 0, time.UTC),
		FinishedAt:  timePointer(finishedAt),
	})
	if err != nil {
		t.Fatalf("save second task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "finished_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) < 2 || items[0]["task_id"] != "task_sort_newer_update" || items[1]["task_id"] != "task_sort_older_update" {
		t.Fatalf("expected fallback sort tie-breaker to prefer newer updated_at, got %+v", items)
	}
}

func TestServiceDashboardOverviewUsesRuntimeAggregation(t *testing.T) {
	service := newTestService()

	completedResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for dashboard overview",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start completed task failed: %v", err)
	}

	waitingResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "waiting authorization task for dashboard overview",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start waiting auth task failed: %v", err)
	}

	result, err := service.DashboardOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	focusSummary := overview["focus_summary"].(map[string]any)
	waitingTaskID := waitingResult["task"].(map[string]any)["task_id"].(string)
	if focusSummary["task_id"] != waitingTaskID {
		t.Fatalf("expected focus summary to point at latest unfinished task %s, got %v", waitingTaskID, focusSummary["task_id"])
	}
	if focusSummary["status"] != "waiting_auth" {
		t.Fatalf("expected focus summary status waiting_auth, got %v", focusSummary["status"])
	}

	trustSummary := overview["trust_summary"].(map[string]any)
	if trustSummary["pending_authorizations"] != 1 {
		t.Fatalf("expected one pending authorization, got %v", trustSummary["pending_authorizations"])
	}
	if trustSummary["has_restore_point"] != true {
		t.Fatalf("expected completed task to provide restore point, got %v", trustSummary["has_restore_point"])
	}
	if trustSummary["workspace_path"] != "workspace" {
		t.Fatalf("expected workspace-relative path in trust summary, got %v", trustSummary["workspace_path"])
	}

	quickActions := overview["quick_actions"].([]string)
	if len(quickActions) == 0 || quickActions[0] != "处理待授权操作" {
		t.Fatalf("expected dashboard quick actions to prioritize authorization handling, got %v", quickActions)
	}

	highValueSignals := overview["high_value_signal"].([]string)
	if len(highValueSignals) == 0 {
		t.Fatal("expected runtime-derived high value signals")
	}
	perceptionResult, err := service.DashboardOverviewGet(map[string]any{
		"include": []any{"high_value_signal"},
		"context": map[string]any{
			"clipboard_text":      "请 translate this paragraph into English before sharing externally.",
			"page_title":          "Release Checklist",
			"visible_text":        "Warning: release notes are incomplete.",
			"dwell_millis":        15000,
			"copy_count":          1,
			"window_switch_count": 3,
		},
	})
	if err != nil {
		t.Fatalf("DashboardOverviewGet with perception context returned error: %v", err)
	}
	perceptionSignals := strings.Join(perceptionResult["overview"].(map[string]any)["high_value_signal"].([]string), " ")
	if !strings.Contains(perceptionSignals, "复制行为") || !strings.Contains(perceptionSignals, "切换页面或窗口") {
		t.Fatalf("expected dashboard to surface perception-derived high value signals, got %s", perceptionSignals)
	}

	completedTaskID := completedResult["task"].(map[string]any)["task_id"].(string)
	if completedTaskID == waitingTaskID {
		t.Fatal("expected completed and waiting tasks to be distinct runtime records")
	}
}

func TestServiceTaskDetailGetExposesActiveApprovalAnchor(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "task detail should expose waiting approval anchor",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}

	approvalRequest, ok := detailResult["approval_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected approval_request anchor, got %+v", detailResult["approval_request"])
	}
	if approvalRequest["task_id"] != taskID {
		t.Fatalf("expected approval_request to stay anchored to task %s, got %+v", taskID, approvalRequest)
	}

	securitySummary, ok := detailResult["security_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected security_summary payload, got %+v", detailResult["security_summary"])
	}
	if securitySummary["pending_authorizations"] != 1 {
		t.Fatalf("expected pending_authorizations to collapse to 1, got %+v", securitySummary["pending_authorizations"])
	}
	if securitySummary["latest_restore_point"] != nil {
		t.Fatalf("expected latest_restore_point to stay nil without restore anchor, got %+v", securitySummary["latest_restore_point"])
	}
}

func TestServiceTaskDetailGetDropsStaleApprovalAnchorOutsideWaitingAuth(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail_stale",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "stale approval anchors must not leak into task detail",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if _, ok := service.runEngine.GetTask(taskID); !ok {
		t.Fatal("expected task to remain available in runtime")
	}
	mutateRuntimeTask(t, service.runEngine, taskID, func(runtimeRecord *runengine.TaskRecord) {
		runtimeRecord.ApprovalRequest = map[string]any{
			"approval_id": "appr_stale",
			"task_id":     taskID,
			"risk_level":  "red",
		}
		runtimeRecord.SecuritySummary["pending_authorizations"] = 1
	})

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	if detailResult["approval_request"] != nil {
		t.Fatalf("expected stale approval_request to be dropped, got %+v", detailResult["approval_request"])
	}

	securitySummary := detailResult["security_summary"].(map[string]any)
	if securitySummary["pending_authorizations"] != 0 {
		t.Fatalf("expected stale pending_authorizations to collapse to 0, got %+v", securitySummary["pending_authorizations"])
	}
}

func TestServiceTaskDetailGetDropsApprovalAnchorWithMismatchedTaskID(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail_bad_task",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "mismatched approval anchors must not leak into task detail",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	mutateRuntimeTask(t, service.runEngine, taskID, func(runtimeRecord *runengine.TaskRecord) {
		runtimeRecord.ApprovalRequest["task_id"] = "task_other"
	})

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	if detailResult["approval_request"] != nil {
		t.Fatalf("expected mismatched approval_request to be dropped, got %+v", detailResult["approval_request"])
	}

	securitySummary := detailResult["security_summary"].(map[string]any)
	if securitySummary["pending_authorizations"] != 0 {
		t.Fatalf("expected mismatched pending_authorizations to collapse to 0, got %+v", securitySummary["pending_authorizations"])
	}
}

func TestServiceTaskDetailGetDropsApprovalAnchorWhenStatusIsNotPending(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail_bad_status",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "non-pending approval anchors must not leak into task detail",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	mutateRuntimeTask(t, service.runEngine, taskID, func(runtimeRecord *runengine.TaskRecord) {
		runtimeRecord.ApprovalRequest["status"] = "approved"
	})

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	if detailResult["approval_request"] != nil {
		t.Fatalf("expected non-pending approval_request to be dropped, got %+v", detailResult["approval_request"])
	}
}

func TestServiceDashboardOverviewRespectsIncludeFilter(t *testing.T) {
	service := newTestService()

	result, err := service.DashboardOverviewGet(map[string]any{
		"include": []any{"focus_summary", "quick_actions"},
	})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	if _, ok := overview["focus_summary"]; !ok {
		t.Fatal("expected focus_summary field to be present")
	}
	if _, ok := overview["quick_actions"]; !ok {
		t.Fatal("expected quick_actions field to be present")
	}
	if overview["trust_summary"] != nil {
		t.Fatalf("expected trust_summary placeholder to be nil when not requested, got %+v", overview["trust_summary"])
	}
	globalState, ok := overview["global_state"].(map[string]any)
	if !ok || len(globalState) != 0 {
		t.Fatalf("expected global_state placeholder to be empty map when not requested, got %+v", overview["global_state"])
	}
	highValueSignal, ok := overview["high_value_signal"].([]string)
	if !ok || len(highValueSignal) != 0 {
		t.Fatalf("expected high_value_signal placeholder to be empty slice when not requested, got %+v", overview["high_value_signal"])
	}
}

func TestServiceDashboardOverviewFocusModeNarrowsSecondaryData(t *testing.T) {
	service := newTestService()

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_focus",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for focus mode",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start completed task failed: %v", err)
	}

	_, err = service.StartTask(map[string]any{
		"session_id": "sess_focus",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "waiting authorization task for focus mode",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start waiting auth task failed: %v", err)
	}

	result, err := service.DashboardOverviewGet(map[string]any{
		"focus_mode": true,
	})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	quickActions := overview["quick_actions"].([]string)
	for _, action := range quickActions {
		if action == "查看最近结果" {
			t.Fatalf("expected focus mode to drop secondary quick action, got %v", quickActions)
		}
	}
	highValueSignals := overview["high_value_signal"].([]string)
	if len(highValueSignals) > 2 {
		t.Fatalf("expected focus mode to narrow signal list, got %v", highValueSignals)
	}
}

func TestServiceDashboardOverviewFallsBackToStoredTaskRuns(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored dashboard overview")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_dashboard_waiting",
		SessionID:   "sess_overview",
		RunID:       "run_dashboard_waiting",
		Title:       "stored waiting authorization task",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 14, 18, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 18, 5, 0, 0, time.UTC),
		ApprovalRequest: map[string]any{
			"approval_id": "appr_dashboard_001",
			"task_id":     "task_dashboard_waiting",
			"risk_level":  "yellow",
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	})
	if err != nil {
		t.Fatalf("save waiting task run failed: %v", err)
	}

	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_dashboard_finished",
		SessionID:   "sess_overview",
		RunID:       "run_dashboard_finished",
		Title:       "stored finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 18, 10, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 18, 15, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 18, 16, 0, 0, time.UTC)),
		SecuritySummary: map[string]any{
			"latest_restore_point": map[string]any{
				"recovery_point_id": "rp_dashboard_001",
				"task_id":           "task_dashboard_finished",
				"summary":           "stored restore point",
			},
		},
		DeliveryResult: map[string]any{
			"type": "workspace_document",
			"payload": map[string]any{
				"path": "workspace/dashboard-overview.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("save finished task run failed: %v", err)
	}

	result, err := service.DashboardOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	focusSummary := overview["focus_summary"].(map[string]any)
	if focusSummary["task_id"] != "task_dashboard_waiting" {
		t.Fatalf("expected storage-backed focus summary to target waiting task, got %+v", focusSummary)
	}
	if focusSummary["status"] != "waiting_auth" {
		t.Fatalf("expected storage-backed focus summary status waiting_auth, got %+v", focusSummary)
	}
	trustSummary := overview["trust_summary"].(map[string]any)
	if trustSummary["pending_authorizations"] != 1 {
		t.Fatalf("expected storage-backed pending authorization count, got %+v", trustSummary)
	}
	if trustSummary["has_restore_point"] != true {
		t.Fatalf("expected storage-backed restore point signal, got %+v", trustSummary)
	}
	quickActions := overview["quick_actions"].([]string)
	if len(quickActions) == 0 || quickActions[0] != "处理待授权操作" {
		t.Fatalf("expected storage-backed quick actions to prioritize authorization handling, got %+v", quickActions)
	}
	highValueSignals := overview["high_value_signal"].([]string)
	if len(highValueSignals) == 0 {
		t.Fatal("expected storage-backed dashboard signals")
	}
}

func TestServiceDashboardOverviewResortsMergedRuntimeAndStoredTasks(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "merged dashboard overview")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	runtimeResult, err := service.StartTask(map[string]any{
		"session_id": "sess_merge_overview",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime task should not win when stored task is newer",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start runtime task failed: %v", err)
	}
	runtimeTask := runtimeResult["task"].(map[string]any)
	runtimeUpdatedAt, err := time.Parse(dateTimeLayout, runtimeTask["updated_at"].(string))
	if err != nil {
		t.Fatalf("parse runtime updated_at failed: %v", err)
	}

	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_dashboard_waiting_newer",
		SessionID:   "sess_merge_overview",
		RunID:       "run_dashboard_waiting_newer",
		Title:       "stored waiting task should become focus",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		StartedAt:   runtimeUpdatedAt.Add(-5 * time.Minute),
		UpdatedAt:   runtimeUpdatedAt.Add(1 * time.Minute),
		ApprovalRequest: map[string]any{
			"approval_id": "appr_dashboard_newer",
			"task_id":     "task_dashboard_waiting_newer",
			"risk_level":  "yellow",
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	})
	if err != nil {
		t.Fatalf("save newer waiting task run failed: %v", err)
	}

	result, err := service.DashboardOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	focusSummary := overview["focus_summary"].(map[string]any)
	if focusSummary["task_id"] != "task_dashboard_waiting_newer" {
		t.Fatalf("expected merged overview to re-sort and focus the newer stored task, got %+v", focusSummary)
	}
	if focusSummary["task_id"] == runtimeResult["task"].(map[string]any)["task_id"] {
		t.Fatalf("expected newer stored task to outrank runtime task in merged overview, got %+v", focusSummary)
	}
	trustSummary := overview["trust_summary"].(map[string]any)
	if trustSummary["pending_authorizations"] != 2 {
		t.Fatalf("expected merged overview to count runtime and stored pending authorizations, got %+v", trustSummary)
	}
}

func TestServiceDashboardOverviewLoadsStoredTasksOncePerRequest(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "single storage scan")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	countingStore := &countingTaskRunStore{base: service.storage.TaskRunStore()}
	replaceTaskRunStore(t, service.storage, countingStore)

	if _, err := service.StartTask(map[string]any{
		"session_id": "sess_overview_count",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime task for overview count",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	}); err != nil {
		t.Fatalf("start runtime task failed: %v", err)
	}

	if err := countingStore.SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_dashboard_count",
		SessionID:   "sess_overview_count",
		RunID:       "run_dashboard_count",
		Title:       "stored task for overview count",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 12, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 12, 6, 0, 0, time.UTC)),
	}); err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	if _, err := service.DashboardOverviewGet(map[string]any{}); err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	if countingStore.loadCalls != 1 {
		t.Fatalf("expected dashboard overview to load stored tasks once per request, got %d", countingStore.loadCalls)
	}
}

func TestServiceMirrorOverviewUsesRuntimeMirrorReferences(t *testing.T) {
	service := newTestService()

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "mirror overview should reuse runtime memory plans",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	result, err := service.MirrorOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("mirror overview failed: %v", err)
	}

	memoryReferences := result["memory_references"].([]map[string]any)
	if len(memoryReferences) == 0 {
		t.Fatal("expected runtime-derived mirror references")
	}
	if !strings.HasPrefix(memoryReferences[0]["memory_id"].(string), "mem_") {
		t.Fatalf("expected memory reference to come from runtime plans, got %v", memoryReferences[0]["memory_id"])
	}

	historySummary := result["history_summary"].([]string)
	if len(historySummary) == 0 {
		t.Fatal("expected history summary to be derived from runtime tasks")
	}

	profile := result["profile"].(map[string]any)
	if profile["preferred_output"] != "workspace_document" {
		t.Fatalf("expected profile to infer workspace_document preference, got %v", profile["preferred_output"])
	}
}

func TestServiceStartTaskWritesRealMemorySummary(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "交付结果里包含 project alpha 的关键结论。")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_memory_write",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请总结 project alpha 的进展",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected completed task to remain in runtime")
	}
	if len(record.MirrorReferences) == 0 {
		t.Fatalf("expected real mirror reference after memory write, got %+v", record)
	}
	if !strings.HasPrefix(record.MirrorReferences[0]["memory_id"].(string), "memsum_") {
		t.Fatalf("expected real memory summary id, got %+v", record.MirrorReferences)
	}
	if querySQLiteCount(t, service.storage.DatabasePath(), `SELECT COUNT(1) FROM memory_summaries WHERE task_id = ?`, taskID) != 1 {
		t.Fatalf("expected one persisted memory summary for task %s", taskID)
	}
}

func TestServiceStartTaskHitsRealMemoryAndRecordsRetrievalHit(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "输出延续了 project alpha markdown bullets 风格。")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.memory.WriteSummary(context.Background(), memory.MemorySummary{
		MemorySummaryID: "mem_seed_001",
		TaskID:          "task_seed_001",
		RunID:           "run_seed_001",
		Summary:         "project alpha prefers markdown bullets and concise structure",
		CreatedAt:       time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("seed memory summary failed: %v", err)
	}

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_memory_hit",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请按 project alpha markdown bullets 总结这段内容",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected completed task to remain in runtime")
	}
	hitFound := false
	writeFound := false
	for _, reference := range record.MirrorReferences {
		memoryID := reference["memory_id"]
		if memoryID == "mem_seed_001" {
			hitFound = true
		}
		if memoryIDString, ok := memoryID.(string); ok && strings.HasPrefix(memoryIDString, "memsum_") {
			writeFound = true
		}
	}
	if !hitFound || !writeFound {
		t.Fatalf("expected both retrieval hit and writeback references, got %+v", record.MirrorReferences)
	}
	if querySQLiteCount(t, service.storage.DatabasePath(), `SELECT COUNT(1) FROM retrieval_hits WHERE task_id = ? AND memory_id = ?`, taskID, "mem_seed_001") != 1 {
		t.Fatalf("expected persisted retrieval hit for task %s", taskID)
	}
	if querySQLiteCount(t, service.storage.DatabasePath(), `SELECT COUNT(1) FROM memory_summaries WHERE task_id = ?`, taskID) != 1 {
		t.Fatalf("expected persisted memory summary for task %s", taskID)
	}

	mirrorResult, err := service.MirrorOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("mirror overview failed: %v", err)
	}
	memoryReferences := mirrorResult["memory_references"].([]map[string]any)
	seenSeed := false
	for _, reference := range memoryReferences {
		if reference["memory_id"] == "mem_seed_001" {
			seenSeed = true
			break
		}
	}
	if !seenSeed {
		t.Fatalf("expected mirror overview to expose real retrieval hit, got %+v", memoryReferences)
	}
}

func TestServiceMirrorOverviewFallsBackToStoredFinishedTasks(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored mirror overview")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_mirror_stored",
		SessionID:   "sess_stored",
		RunID:       "run_mirror_stored",
		Title:       "stored mirror task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 11, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 11, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 11, 6, 0, 0, time.UTC)),
		MirrorReferences: []map[string]any{{
			"memory_id": "mem_stored_001",
			"reason":    "stored memory hit",
			"summary":   "stored mirror reference",
		}},
		DeliveryResult: map[string]any{
			"type": "workspace_document",
			"payload": map[string]any{
				"path": "workspace/stored-result.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	result, err := service.MirrorOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("mirror overview failed: %v", err)
	}

	memoryReferences := result["memory_references"].([]map[string]any)
	if len(memoryReferences) != 1 || memoryReferences[0]["memory_id"] != "mem_stored_001" {
		t.Fatalf("expected storage-backed mirror references, got %+v", memoryReferences)
	}
	historySummary := result["history_summary"].([]string)
	if len(historySummary) == 0 {
		t.Fatal("expected storage-backed mirror history summary")
	}
	profile := result["profile"].(map[string]any)
	if profile["preferred_output"] != "workspace_document" {
		t.Fatalf("expected storage-backed mirror profile to infer workspace_document, got %+v", profile)
	}
}

func TestServiceSecuritySummaryUsesRuntimeTaskState(t *testing.T) {
	service := newTestService()

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for security summary restore point",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start completed task failed: %v", err)
	}

	waitingResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "security summary intercepted task",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start intercepted task failed: %v", err)
	}

	waitingTaskID := waitingResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.SecurityRespond(map[string]any{
		"task_id":       waitingTaskID,
		"approval_id":   "appr_001",
		"decision":      "deny_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	if summary["security_status"] != "intercepted" {
		t.Fatalf("expected intercepted security status from runtime task state, got %v", summary["security_status"])
	}
	if summary["pending_authorizations"] != 0 {
		t.Fatalf("expected no pending authorizations after denial, got %v", summary["pending_authorizations"])
	}
	if summary["latest_restore_point"] == nil {
		t.Fatal("expected latest restore point to come from completed runtime task")
	}

	tokenCostSummary := summary["token_cost_summary"].(map[string]any)
	if tokenCostSummary["budget_auto_downgrade"] != true {
		t.Fatalf("expected token summary to reflect settings snapshot, got %v", tokenCostSummary["budget_auto_downgrade"])
	}
}

func TestServiceSecuritySummaryIncludesRuntimeTokenUsage(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed token summary")

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "collect runtime token summary",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected completed runtime task to remain available")
	}
	if len(record.AuditRecords) == 0 {
		t.Fatal("expected executor-backed task to carry audit records")
	}

	securityResult, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	tokenCostSummary := securityResult["summary"].(map[string]any)["token_cost_summary"].(map[string]any)
	if tokenCostSummary["current_task_tokens"] != 36 {
		t.Fatalf("expected current_task_tokens to reflect runtime usage, got %+v", tokenCostSummary)
	}
	if tokenCostSummary["today_tokens"] != 36 {
		t.Fatalf("expected today_tokens to reflect runtime usage, got %+v", tokenCostSummary)
	}
	if tokenCostSummary["budget_auto_downgrade"] != true {
		t.Fatalf("expected budget_auto_downgrade to remain true, got %+v", tokenCostSummary)
	}
}

func TestServiceSecuritySummaryFallsBackToStoredRecoveryPoint(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_001",
		TaskID:          "task_external",
		Summary:         "stored recovery point",
		CreatedAt:       "2026-04-08T10:00:00Z",
		Objects:         []string{"workspace/result.md"},
	})
	if err != nil {
		t.Fatalf("write recovery point failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	latestRestorePoint := summary["latest_restore_point"].(map[string]any)
	if latestRestorePoint["recovery_point_id"] != "rp_001" {
		t.Fatalf("expected storage-backed recovery point, got %+v", latestRestorePoint)
	}
}

func TestServiceSecuritySummaryFallsBackToStoredTaskRuns(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored security summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_security_finished",
		SessionID:   "sess_stored",
		RunID:       "run_security_finished",
		Title:       "stored security task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 13, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 13, 6, 0, 0, time.UTC)),
		SecuritySummary: map[string]any{
			"security_status": "recoverable",
			"latest_restore_point": map[string]any{
				"recovery_point_id": "rp_security_001",
				"task_id":           "task_security_finished",
				"summary":           "stored security recovery point",
				"created_at":        "2026-04-14T13:06:00Z",
				"objects":           []string{"workspace/security.md"},
			},
		},
		TokenUsage: map[string]any{
			"total_tokens":   88,
			"estimated_cost": 0.42,
		},
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	if summary["security_status"] != "recoverable" {
		t.Fatalf("expected storage-backed security status, got %+v", summary)
	}
	latestRestorePoint := summary["latest_restore_point"].(map[string]any)
	if latestRestorePoint["recovery_point_id"] != "rp_security_001" {
		t.Fatalf("expected storage-backed recovery point from task run, got %+v", latestRestorePoint)
	}
	tokenCostSummary := summary["token_cost_summary"].(map[string]any)
	if tokenCostSummary["current_task_tokens"] != 88 {
		t.Fatalf("expected storage-backed token usage, got %+v", tokenCostSummary)
	}
}

func TestServiceSecuritySummaryCountsStoredPendingAuthorizations(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored waiting auth")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_waiting_auth_stored",
		SessionID:   "sess_waiting",
		RunID:       "run_waiting_auth_stored",
		Title:       "stored waiting auth task",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 14, 17, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 17, 5, 0, 0, time.UTC),
		ApprovalRequest: map[string]any{
			"approval_id": "appr_waiting_001",
			"task_id":     "task_waiting_auth_stored",
			"risk_level":  "yellow",
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	})
	if err != nil {
		t.Fatalf("save waiting auth task run failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	if summary["pending_authorizations"] != 1 {
		t.Fatalf("expected stored waiting_auth task to count as pending authorization, got %+v", summary)
	}
}

func TestServiceDashboardModuleCountsRuntimeAndStoredPendingAuthorizations(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "dashboard mixed waiting auth")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	runtimeResult, err := service.StartTask(map[string]any{
		"session_id": "sess_dashboard_module_mixed",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime waiting authorization for dashboard module",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start runtime waiting task failed: %v", err)
	}

	runtimeTaskID := runtimeResult["task"].(map[string]any)["task_id"].(string)
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_dashboard_module_waiting_stored",
		SessionID:   "sess_dashboard_module_mixed",
		RunID:       "run_dashboard_module_waiting_stored",
		Title:       "stored waiting auth task for dashboard module",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 14, 18, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 18, 5, 0, 0, time.UTC),
		ApprovalRequest: map[string]any{
			"approval_id": "appr_dashboard_module_stored",
			"task_id":     "task_dashboard_module_waiting_stored",
			"risk_level":  "yellow",
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	})
	if err != nil {
		t.Fatalf("save stored waiting auth task run failed: %v", err)
	}

	moduleResult, err := service.DashboardModuleGet(map[string]any{
		"module": "security",
		"tab":    "audit",
	})
	if err != nil {
		t.Fatalf("dashboard module get failed: %v", err)
	}

	highlights := moduleResult["highlights"].([]string)
	foundPendingHighlight := false
	for _, highlight := range highlights {
		if strings.Contains(highlight, "当前仍有 2 个待授权任务等待处理。") {
			foundPendingHighlight = true
			break
		}
	}
	if !foundPendingHighlight {
		t.Fatalf("expected merged pending authorization highlight for runtime task %s, got %+v", runtimeTaskID, highlights)
	}
}

func TestServiceSecuritySummaryCountsRuntimeAndStoredPendingAuthorizations(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "security mixed waiting auth")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	runtimeResult, err := service.StartTask(map[string]any{
		"session_id": "sess_security_summary_mixed",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime waiting authorization for security summary",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start runtime waiting task failed: %v", err)
	}

	runtimeTaskID := runtimeResult["task"].(map[string]any)["task_id"].(string)
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_security_waiting_stored",
		SessionID:   "sess_security_summary_mixed",
		RunID:       "run_security_waiting_stored",
		Title:       "stored waiting auth task for security summary",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 14, 19, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 19, 5, 0, 0, time.UTC),
		ApprovalRequest: map[string]any{
			"approval_id": "appr_security_stored",
			"task_id":     "task_security_waiting_stored",
			"risk_level":  "yellow",
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	})
	if err != nil {
		t.Fatalf("save stored waiting auth task run failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	if summary["pending_authorizations"] != 2 {
		t.Fatalf("expected merged pending authorizations for runtime task %s, got %+v", runtimeTaskID, summary)
	}
	if summary["security_status"] != "pending_confirmation" {
		t.Fatalf("expected pending_confirmation status when merged pending authorizations remain, got %+v", summary)
	}
}

func TestServiceSecurityPendingListMergesRuntimeAndStoredPendingAuthorizations(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "security pending mixed waiting auth")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	runtimeResult, err := service.StartTask(map[string]any{
		"session_id": "sess_security_pending_list_mixed",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime waiting authorization for pending list",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start runtime waiting task failed: %v", err)
	}

	runtimeTask := runtimeResult["task"].(map[string]any)
	runtimeUpdatedAt, err := time.Parse(dateTimeLayout, runtimeTask["updated_at"].(string))
	if err != nil {
		t.Fatalf("parse runtime updated_at failed: %v", err)
	}

	if err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_security_pending_list_stored",
		SessionID:   "sess_security_pending_list_mixed",
		RunID:       "run_security_pending_list_stored",
		Title:       "stored waiting auth task for pending list",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "red",
		StartedAt:   runtimeUpdatedAt.Add(-5 * time.Minute),
		UpdatedAt:   runtimeUpdatedAt.Add(1 * time.Minute),
		ApprovalRequest: map[string]any{
			"approval_id":    "appr_security_pending_list_stored",
			"task_id":        "task_security_pending_list_stored",
			"operation_name": "write_file",
			"target_object":  "/workspace/security.txt",
			"reason":         "Stored high-risk write still needs authorization.",
			"status":         "pending",
			"risk_level":     "red",
			"created_at":     runtimeUpdatedAt.Add(1 * time.Minute).Format(time.RFC3339Nano),
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	}); err != nil {
		t.Fatalf("save stored waiting auth task run failed: %v", err)
	}

	result, err := service.SecurityPendingList(map[string]any{
		"limit":  float64(20),
		"offset": float64(0),
	})
	if err != nil {
		t.Fatalf("security pending list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 2 {
		t.Fatalf("expected merged pending authorization list to return two items, got %+v", items)
	}
	if items[0]["task_id"] != "task_security_pending_list_stored" {
		t.Fatalf("expected newer stored pending task to lead merged list, got %+v", items)
	}

	page := result["page"].(map[string]any)
	if page["total"] != 2 || page["has_more"] != false {
		t.Fatalf("expected merged pending list page metadata, got %+v", page)
	}
}

func TestServiceSecurityPendingListPaginatesMergedPendingAuthorizations(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "security pending pagination")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	runtimeResult, err := service.StartTask(map[string]any{
		"session_id": "sess_security_pending_list_page",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime waiting authorization for pending pagination",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start runtime waiting task failed: %v", err)
	}

	runtimeTask := runtimeResult["task"].(map[string]any)
	runtimeUpdatedAt, err := time.Parse(dateTimeLayout, runtimeTask["updated_at"].(string))
	if err != nil {
		t.Fatalf("parse runtime updated_at failed: %v", err)
	}

	if err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_security_pending_page_stored",
		SessionID:   "sess_security_pending_list_page",
		RunID:       "run_security_pending_page_stored",
		Title:       "stored waiting auth task for pending pagination",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		StartedAt:   runtimeUpdatedAt.Add(-5 * time.Minute),
		UpdatedAt:   runtimeUpdatedAt.Add(1 * time.Minute),
		ApprovalRequest: map[string]any{
			"approval_id":    "appr_security_pending_page_stored",
			"task_id":        "task_security_pending_page_stored",
			"operation_name": "write_file",
			"target_object":  "/workspace/pending.txt",
			"reason":         "Stored task should occupy the first merged page slot.",
			"status":         "pending",
			"risk_level":     "yellow",
			"created_at":     runtimeUpdatedAt.Add(1 * time.Minute).Format(time.RFC3339Nano),
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	}); err != nil {
		t.Fatalf("save stored waiting auth task run failed: %v", err)
	}

	result, err := service.SecurityPendingList(map[string]any{
		"limit":  float64(1),
		"offset": float64(1),
	})
	if err != nil {
		t.Fatalf("security pending list with pagination failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one paged pending authorization item, got %+v", items)
	}
	if items[0]["task_id"] != runtimeTask["task_id"] {
		t.Fatalf("expected offset page to return runtime task after stored task, got %+v", items)
	}

	page := result["page"].(map[string]any)
	if page["limit"] != 1 || page["offset"] != 1 || page["total"] != 2 || page["has_more"] != false {
		t.Fatalf("expected paged merged pending list metadata, got %+v", page)
	}
}

func TestServiceDashboardModuleHighlightsIncludeAuditTrail(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "dashboard audit trail")

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "build dashboard audit trail",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	moduleResult, err := service.DashboardModuleGet(map[string]any{
		"module": "security",
		"tab":    "audit",
	})
	if err != nil {
		t.Fatalf("dashboard module get failed: %v", err)
	}

	highlights := moduleResult["highlights"].([]string)
	foundAuditHighlight := false
	for _, highlight := range highlights {
		if strings.Contains(highlight, "generate_text") || strings.Contains(highlight, "publish_result") || strings.Contains(highlight, "write_file") {
			foundAuditHighlight = true
			break
		}
	}
	if !foundAuditHighlight {
		t.Fatalf("expected dashboard highlights to expose runtime audit trail, got %+v", highlights)
	}
}

func TestServiceDashboardModuleFallsBackToStoredTaskRuns(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored dashboard module")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_dashboard_finished",
		SessionID:   "sess_stored",
		RunID:       "run_dashboard_finished",
		Title:       "stored dashboard task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 12, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 12, 6, 0, 0, time.UTC)),
		DeliveryResult: map[string]any{
			"type": "workspace_document",
			"payload": map[string]any{
				"path": "workspace/dashboard.md",
			},
		},
		Artifacts: []map[string]any{{
			"artifact_id":      "art_dashboard_finished",
			"task_id":          "task_dashboard_finished",
			"artifact_type":    "generated_doc",
			"title":            "dashboard.md",
			"path":             "workspace/dashboard.md",
			"mime_type":        "text/markdown",
			"delivery_type":    "workspace_document",
			"delivery_payload": map[string]any{"path": "workspace/dashboard.md", "task_id": "task_dashboard_finished"},
			"created_at":       "2026-04-14T12:06:00Z",
		}},
		AuditRecords: []map[string]any{{
			"audit_id":   "audit_dashboard_001",
			"task_id":    "task_dashboard_finished",
			"action":     "write_file",
			"summary":    "stored dashboard audit",
			"created_at": "2026-04-14T12:06:00Z",
			"result":     "success",
			"target":     "workspace/dashboard.md",
		}},
	})
	if err != nil {
		t.Fatalf("save finished task run failed: %v", err)
	}
	err = service.storage.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_dashboard_finished",
		TaskID:              "task_dashboard_finished",
		ArtifactType:        "generated_doc",
		Title:               "dashboard.md",
		Path:                "workspace/dashboard.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/dashboard.md","task_id":"task_dashboard_finished"}`,
		CreatedAt:           "2026-04-14T12:06:00Z",
	}})
	if err != nil {
		t.Fatalf("save dashboard artifact failed: %v", err)
	}

	moduleResult, err := service.DashboardModuleGet(map[string]any{
		"module": "security",
		"tab":    "audit",
	})
	if err != nil {
		t.Fatalf("dashboard module get failed: %v", err)
	}

	summary := moduleResult["summary"].(map[string]any)
	if summary["completed_tasks"] != 1 {
		t.Fatalf("expected storage-backed completed task count, got %+v", summary)
	}
	if summary["generated_outputs"] != 1 {
		t.Fatalf("expected storage-backed generated output count, got %+v", summary)
	}
	highlights := moduleResult["highlights"].([]string)
	if len(highlights) == 0 {
		t.Fatal("expected storage-backed dashboard highlights")
	}
	foundArtifactHighlight := false
	for _, highlight := range highlights {
		if strings.Contains(highlight, "workspace/dashboard.md") {
			foundArtifactHighlight = true
			break
		}
	}
	if !foundArtifactHighlight {
		t.Fatalf("expected dashboard highlights to mention artifact-backed path, got %+v", highlights)
	}
}

func TestServiceSecurityAuditListFallsBackToStoredAuditRecords(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.AuditWriter().WriteAuditRecord(context.Background(), audit.Record{
		AuditID:   "audit_001",
		TaskID:    "task_external",
		Type:      "file",
		Action:    "write_file",
		Summary:   "stored audit record",
		Target:    "workspace/result.md",
		Result:    "success",
		CreatedAt: "2026-04-08T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("write audit record failed: %v", err)
	}

	result, err := service.SecurityAuditList(map[string]any{"task_id": "task_external", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security audit list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["audit_id"] != "audit_001" {
		t.Fatalf("expected storage-backed audit record, got %+v", items)
	}
}

func TestServiceSecurityAuditListRequiresTaskID(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	_, err := service.SecurityAuditList(map[string]any{"limit": 20, "offset": 0})
	if err == nil || err.Error() != "task_id is required" {
		t.Fatalf("expected task_id required error, got %v", err)
	}
}

func TestServiceSecurityRestorePointsListFallsBackToStoredRecoveryPoints(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_001",
		TaskID:          "task_external",
		Summary:         "stored recovery point",
		CreatedAt:       "2026-04-08T10:00:00Z",
		Objects:         []string{"workspace/result.md"},
	})
	if err != nil {
		t.Fatalf("write recovery point failed: %v", err)
	}

	result, err := service.SecurityRestorePointsList(map[string]any{"task_id": "task_external", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security restore points list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["recovery_point_id"] != "rp_001" {
		t.Fatalf("expected storage-backed recovery point, got %+v", items)
	}
	if items[0]["task_id"] != "task_external" {
		t.Fatalf("expected task_external recovery point, got %+v", items[0])
	}
	objects := items[0]["objects"].([]string)
	if len(objects) != 1 || objects[0] != "workspace/result.md" {
		t.Fatalf("expected recovery point objects to round-trip, got %+v", objects)
	}
	page := result["page"].(map[string]any)
	if page["total"] != 1 {
		t.Fatalf("expected total=1, got %+v", page)
	}
}

func TestServiceSecurityRestorePointsListWithoutStorageReturnsEmptyPage(t *testing.T) {
	service := newTestService()

	result, err := service.SecurityRestorePointsList(map[string]any{"limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security restore points list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 0 {
		t.Fatalf("expected empty restore point list, got %+v", items)
	}
	page := result["page"].(map[string]any)
	if page["total"] != 0 {
		t.Fatalf("expected empty page metadata, got %+v", page)
	}
}

func TestServiceTaskDetailGetFallsBackToStoredRecoveryPointForTask(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "task detail restore point fallback",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	err = service.storage.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_task_detail",
		TaskID:          taskID,
		Summary:         "stored recovery point for task detail",
		CreatedAt:       "2026-04-08T10:02:00Z",
		Objects:         []string{"workspace/result.md"},
	})
	if err != nil {
		t.Fatalf("write recovery point failed: %v", err)
	}

	result, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	securitySummary := result["security_summary"].(map[string]any)
	latestRestorePoint := securitySummary["latest_restore_point"].(map[string]any)
	if latestRestorePoint["recovery_point_id"] != "rp_task_detail" {
		t.Fatalf("expected storage-backed restore point in task detail, got %+v", latestRestorePoint)
	}
}

func TestServiceTaskDetailGetDropsMismatchedSummaryRecoveryPointBeforeStorageFallback(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary mismatch")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec_mismatch",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "task detail restore point mismatch fallback",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	mutateRuntimeTask(t, service.runEngine, taskID, func(runtimeRecord *runengine.TaskRecord) {
		runtimeRecord.SecuritySummary["latest_restore_point"] = map[string]any{
			"recovery_point_id": "rp_wrong_task",
			"task_id":           "task_other",
			"summary":           "wrong task restore point",
			"created_at":        "2026-04-08T10:01:00Z",
			"objects":           []string{"workspace/wrong.md"},
		}
	})
	if err := service.storage.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_task_detail_fallback",
		TaskID:          taskID,
		Summary:         "stored recovery point for fallback",
		CreatedAt:       "2026-04-08T10:02:00Z",
		Objects:         []string{"workspace/result.md"},
	}); err != nil {
		t.Fatalf("write recovery point failed: %v", err)
	}

	result, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	securitySummary := result["security_summary"].(map[string]any)
	latestRestorePoint := securitySummary["latest_restore_point"].(map[string]any)
	if latestRestorePoint["recovery_point_id"] != "rp_task_detail_fallback" {
		t.Fatalf("expected storage fallback after mismatched summary restore point, got %+v", latestRestorePoint)
	}
}

func TestServiceSecurityRestoreApplyRestoresWorkspaceAndReturnsFormalResult(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "新的内容")
	originalPath := filepath.Join(workspaceRoot, "notes", "output.md")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(originalPath, []byte("旧的内容"), 0o644); err != nil {
		t.Fatalf("seed original file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请覆盖该文件",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": "notes/output.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":     taskID,
		"approval_id": taskID,
		"decision":    "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected write_file task to complete after authorization, got %+v", respondResult)
	}

	pointsResult, err := service.SecurityRestorePointsList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security restore points list failed: %v", err)
	}
	points := pointsResult["items"].([]map[string]any)
	if len(points) == 0 {
		t.Fatal("expected completed write_file task to persist recovery point")
	}

	applyResult, err := service.SecurityRestoreApply(map[string]any{
		"task_id":           taskID,
		"recovery_point_id": points[0]["recovery_point_id"],
	})
	if err != nil {
		t.Fatalf("security restore apply failed: %v", err)
	}
	if applyResult["task"].(map[string]any)["status"] != "waiting_auth" || applyResult["applied"] != false {
		t.Fatalf("expected restore apply to require authorization first, got %+v", applyResult)
	}
	contentBeforeApproval, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read file before restore approval: %v", err)
	}
	if !strings.Contains(string(contentBeforeApproval), "新的内容") {
		t.Fatalf("expected restore request not to mutate workspace before approval, got %q", string(contentBeforeApproval))
	}
	respondApplyResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_restore_apply",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond for restore apply failed: %v", err)
	}
	if respondApplyResult["applied"] != true {
		t.Fatalf("expected approved restore apply success, got %+v", respondApplyResult)
	}
	auditRecord := respondApplyResult["audit_record"].(map[string]any)
	if auditRecord["action"] != "restore_apply" || auditRecord["result"] != "success" {
		t.Fatalf("expected restore audit success, got %+v", auditRecord)
	}
	bubble := respondApplyResult["bubble_message"].(map[string]any)
	if !strings.Contains(stringValue(bubble, "text", ""), "恢复点") {
		t.Fatalf("expected bubble message to mention recovery point, got %+v", bubble)
	}
	restoredContent, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(restoredContent) != "旧的内容" {
		t.Fatalf("expected restore apply to recover original content, got %q", string(restoredContent))
	}
	updatedTask, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain available after restore")
	}
	if stringValue(updatedTask.SecuritySummary, "security_status", "") != "recovered" {
		t.Fatalf("expected recovered security status, got %+v", updatedTask.SecuritySummary)
	}
}

func TestServiceSecurityRestoreApplyReturnsStructuredFailure(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "新的内容")
	originalPath := filepath.Join(workspaceRoot, "notes", "output.md")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(originalPath, []byte("旧的内容"), 0o644); err != nil {
		t.Fatalf("seed original file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请覆盖该文件",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": "notes/output.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":     taskID,
		"approval_id": taskID,
		"decision":    "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected write_file task to complete after authorization, got %+v", respondResult)
	}
	pointsResult, err := service.SecurityRestorePointsList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security restore points list failed: %v", err)
	}
	points := pointsResult["items"].([]map[string]any)
	if len(points) == 0 {
		t.Fatal("expected completed write_file task to persist recovery point")
	}
	backupPath := filepath.Join(workspaceRoot, ".recovery_points", points[0]["recovery_point_id"].(string), "notes", "output.md")
	if err := os.Remove(backupPath); err != nil {
		t.Fatalf("remove backup snapshot: %v", err)
	}

	applyResult, err := service.SecurityRestoreApply(map[string]any{
		"task_id":           taskID,
		"recovery_point_id": points[0]["recovery_point_id"],
	})
	if err != nil {
		t.Fatalf("security restore apply returned rpc error unexpectedly: %v", err)
	}
	if applyResult["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected restore apply to wait for auth before execution, got %+v", applyResult)
	}
	applyResult, err = service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_restore_apply_failure",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond for restore apply failure failed: %v", err)
	}
	if applyResult["applied"] != false {
		t.Fatalf("expected restore apply failure result, got %+v", applyResult)
	}
	auditRecord := applyResult["audit_record"].(map[string]any)
	if auditRecord["result"] != "failed" {
		t.Fatalf("expected failed restore audit record, got %+v", auditRecord)
	}
	bubble := applyResult["bubble_message"].(map[string]any)
	if !strings.Contains(stringValue(bubble, "text", ""), "恢复失败") {
		t.Fatalf("expected failure bubble message, got %+v", bubble)
	}
	updatedTask, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain available after failed restore")
	}
	if stringValue(updatedTask.SecuritySummary, "security_status", "") != "execution_error" {
		t.Fatalf("expected execution_error security status, got %+v", updatedTask.SecuritySummary)
	}
}

func TestServiceSecurityRestoreApplySupportsPersistedTaskFallback(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "新的内容")
	originalPath := filepath.Join(workspaceRoot, "notes", "output.md")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(originalPath, []byte("旧的内容"), 0o644); err != nil {
		t.Fatalf("seed original file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请覆盖该文件",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": "notes/output.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{"task_id": taskID, "approval_id": taskID, "decision": "allow_once"})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected task to complete before persisted fallback, got %+v", respondResult)
	}
	if _, err := service.TaskDetailGet(map[string]any{"task_id": taskID}); err != nil {
		t.Fatalf("task detail get before persisted fallback failed: %v", err)
	}
	pointsResult, err := service.SecurityRestorePointsList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security restore points list failed: %v", err)
	}
	points := pointsResult["items"].([]map[string]any)
	if len(points) == 0 {
		t.Fatal("expected recovery point to exist")
	}
	service.runEngine = runengine.NewEngine()

	applyResult, err := service.SecurityRestoreApply(map[string]any{"task_id": taskID, "recovery_point_id": points[0]["recovery_point_id"]})
	if err != nil {
		t.Fatalf("security restore apply failed with persisted task fallback: %v", err)
	}
	if applyResult["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected restore apply fallback to wait for auth, got %+v", applyResult)
	}
	applyResult, err = service.SecurityRespond(map[string]any{"task_id": taskID, "approval_id": "appr_restore_apply_persisted", "decision": "allow_once"})
	if err != nil {
		t.Fatalf("security respond failed with persisted fallback: %v", err)
	}
	if applyResult["applied"] != true {
		t.Fatalf("expected restore apply success with persisted task fallback, got %+v", applyResult)
	}
}

func TestServiceTaskDetailGetPreservesStableContractShape(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "task detail delivery")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "collect detail view payload",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}

	if _, ok := detailResult["delivery_result"]; ok {
		t.Fatalf("expected task detail response not to expose undeclared delivery_result field, got %+v", detailResult["delivery_result"])
	}
	if _, ok := detailResult["audit_records"]; ok {
		t.Fatalf("expected task detail response not to expose undeclared audit_records field, got %+v", detailResult["audit_records"])
	}
	if detailResult["task"].(map[string]any)["task_id"] != taskID {
		t.Fatalf("expected task detail task_id to match request, got %+v", detailResult["task"])
	}
	artifacts, ok := detailResult["artifacts"].([]map[string]any)
	if !ok || len(artifacts) != 0 {
		t.Fatalf("expected empty artifact collection array, got %+v", detailResult["artifacts"])
	}
	mirrorReferences, ok := detailResult["mirror_references"].([]map[string]any)
	if !ok || len(mirrorReferences) != 0 {
		t.Fatalf("expected empty mirror reference collection array, got %+v", detailResult["mirror_references"])
	}
	if _, ok := detailResult["timeline"].([]map[string]any); !ok {
		t.Fatalf("expected timeline to stay an array, got %+v", detailResult["timeline"])
	}
}

func TestServiceTaskDetailGetNormalizesProtocolCollections(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "task detail protocol collections")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail_protocol",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "collect normalized detail payload",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	task, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected runtime task to exist")
	}
	if _, ok := service.runEngine.SetPresentation(taskID, task.BubbleMessage, task.DeliveryResult, []map[string]any{{
		"artifact_id":      "art_detail_protocol_001",
		"task_id":          taskID,
		"artifact_type":    "generated_doc",
		"title":            "detail-protocol.md",
		"path":             "workspace/detail-protocol.md",
		"mime_type":        "text/markdown",
		"delivery_type":    "workspace_document",
		"delivery_payload": map[string]any{"path": "workspace/detail-protocol.md", "task_id": taskID},
		"created_at":       "2026-04-15T10:00:00Z",
	}}); !ok {
		t.Fatal("expected task presentation update to succeed")
	}
	if _, ok := service.runEngine.SetMirrorReferences(taskID, []map[string]any{{
		"memory_id": "mem_protocol_001",
		"reason":    "detail normalization",
		"summary":   "normalized reference",
		"source":    "runtime",
	}}); !ok {
		t.Fatal("expected mirror reference update to succeed")
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}

	artifacts := detailResult["artifacts"].([]map[string]any)
	if len(artifacts) != 1 {
		t.Fatalf("expected one normalized artifact, got %+v", artifacts)
	}
	artifact := artifacts[0]
	if artifact["artifact_id"] != "art_detail_protocol_001" || artifact["mime_type"] != "text/markdown" {
		t.Fatalf("expected formal artifact fields to survive normalization, got %+v", artifact)
	}
	if _, ok := artifact["delivery_type"]; ok {
		t.Fatalf("expected detail artifact to omit undeclared delivery_type, got %+v", artifact)
	}
	if _, ok := artifact["delivery_payload"]; ok {
		t.Fatalf("expected detail artifact to omit undeclared delivery_payload, got %+v", artifact)
	}

	mirrorReferences := detailResult["mirror_references"].([]map[string]any)
	if len(mirrorReferences) != 1 {
		t.Fatalf("expected one normalized mirror reference, got %+v", mirrorReferences)
	}
	if _, ok := mirrorReferences[0]["source"]; ok {
		t.Fatalf("expected mirror reference to omit undeclared source field, got %+v", mirrorReferences[0])
	}
}

func TestServiceTaskDetailGetFallsBackToStoredTaskRun(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored task detail")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_detail",
		SessionID:   "sess_stored",
		RunID:       "run_stored_detail",
		Title:       "stored detail task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 10, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 10, 6, 0, 0, time.UTC)),
		Timeline: []storage.TaskStepSnapshot{{
			StepID:        "step_deliver_result",
			TaskID:        "task_stored_detail",
			Name:          "deliver_result",
			Status:        "completed",
			OrderIndex:    1,
			InputSummary:  "stored input",
			OutputSummary: "stored output",
		}},
		Artifacts: []map[string]any{{
			"artifact_id":      "art_task_stored_detail",
			"task_id":          "task_stored_detail",
			"artifact_type":    "generated_doc",
			"title":            "stored-detail.md",
			"path":             "workspace/stored-detail.md",
			"mime_type":        "text/markdown",
			"delivery_type":    "workspace_document",
			"delivery_payload": map[string]any{"path": "workspace/stored-detail.md", "task_id": "task_stored_detail"},
			"created_at":       "2026-04-14T10:06:00Z",
		}},
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}
	err = service.storage.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_task_stored_detail",
		TaskID:              "task_stored_detail",
		ArtifactType:        "generated_doc",
		Title:               "stored-detail.md",
		Path:                "workspace/stored-detail.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/stored-detail.md","task_id":"task_stored_detail"}`,
		CreatedAt:           "2026-04-14T10:06:00Z",
	}})
	if err != nil {
		t.Fatalf("save artifact failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": "task_stored_detail"})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}

	task := detailResult["task"].(map[string]any)
	if task["task_id"] != "task_stored_detail" || task["title"] != "stored detail task" {
		t.Fatalf("expected storage-backed task detail, got %+v", task)
	}
	timeline := detailResult["timeline"].([]map[string]any)
	if len(timeline) != 1 || timeline[0]["name"] != "deliver_result" {
		t.Fatalf("expected storage-backed timeline, got %+v", timeline)
	}
	artifacts := detailResult["artifacts"].([]map[string]any)
	if len(artifacts) != 1 || artifacts[0]["artifact_id"] != "art_task_stored_detail" {
		t.Fatalf("expected storage-backed artifacts, got %+v", artifacts)
	}
}

func TestServiceTaskArtifactListReturnsStoredArtifacts(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "artifact list")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_list_001",
		TaskID:              "task_artifact_list",
		ArtifactType:        "generated_doc",
		Title:               "artifact-list.md",
		Path:                "workspace/artifact-list.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/artifact-list.md","task_id":"task_artifact_list"}`,
		CreatedAt:           "2026-04-14T10:00:00Z",
	}})
	if err != nil {
		t.Fatalf("save artifacts failed: %v", err)
	}
	result, err := service.TaskArtifactList(map[string]any{"task_id": "task_artifact_list", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task artifact list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["artifact_id"] != "art_list_001" {
		t.Fatalf("expected stored artifact list item, got %+v", items)
	}
	if _, ok := items[0]["delivery_type"]; ok {
		t.Fatalf("expected artifact list item to omit undeclared delivery_type, got %+v", items[0])
	}
}

func TestServiceTaskArtifactListUsesStorePaginationBeyondHundred(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "artifact pagination")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	records := make([]storage.ArtifactRecord, 0, 120)
	for index := 0; index < 120; index++ {
		records = append(records, storage.ArtifactRecord{
			ArtifactID:          fmt.Sprintf("art_page_%03d", index),
			TaskID:              "task_artifact_page",
			ArtifactType:        "generated_doc",
			Title:               fmt.Sprintf("artifact-%03d.md", index),
			Path:                fmt.Sprintf("workspace/artifact-%03d.md", index),
			MimeType:            "text/markdown",
			DeliveryType:        "workspace_document",
			DeliveryPayloadJSON: fmt.Sprintf(`{"path":"workspace/artifact-%03d.md","task_id":"task_artifact_page"}`, index),
			CreatedAt:           time.Date(2026, 4, 14, 10, 0, index, 0, time.UTC).Format(time.RFC3339),
		})
	}
	if err := service.storage.ArtifactStore().SaveArtifacts(context.Background(), records); err != nil {
		t.Fatalf("save artifacts failed: %v", err)
	}
	result, err := service.TaskArtifactList(map[string]any{"task_id": "task_artifact_page", "limit": 20, "offset": 100})
	if err != nil {
		t.Fatalf("task artifact list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	page := result["page"].(map[string]any)
	if len(items) != 20 {
		t.Fatalf("expected 20 paged artifacts, got %d", len(items))
	}
	if page["total"] != 120 {
		t.Fatalf("expected full artifact total, got %+v", page)
	}
}

func TestServiceTaskArtifactOpenFindsStoredArtifactBeyondFirstHundred(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "artifact open pagination")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	records := make([]storage.ArtifactRecord, 0, 120)
	for index := 0; index < 120; index++ {
		records = append(records, storage.ArtifactRecord{
			ArtifactID:          fmt.Sprintf("art_open_page_%03d", index),
			TaskID:              "task_artifact_open_page",
			ArtifactType:        "generated_doc",
			Title:               fmt.Sprintf("artifact-open-%03d.md", index),
			Path:                fmt.Sprintf("workspace/artifact-open-%03d.md", index),
			MimeType:            "text/markdown",
			DeliveryType:        "open_file",
			DeliveryPayloadJSON: fmt.Sprintf(`{"path":"workspace/artifact-open-%03d.md","task_id":"task_artifact_open_page"}`, index),
			CreatedAt:           time.Date(2026, 4, 14, 10, 0, index, 0, time.UTC).Format(time.RFC3339),
		})
	}
	if err := service.storage.ArtifactStore().SaveArtifacts(context.Background(), records); err != nil {
		t.Fatalf("save artifacts failed: %v", err)
	}
	result, err := service.TaskArtifactOpen(map[string]any{"task_id": "task_artifact_open_page", "artifact_id": "art_open_page_000"})
	if err != nil {
		t.Fatalf("task artifact open failed: %v", err)
	}
	if result["artifact"].(map[string]any)["artifact_id"] != "art_open_page_000" {
		t.Fatalf("expected artifact beyond first hundred to resolve, got %+v", result)
	}
}

func TestServiceTaskArtifactOpenReturnsStableOpenPayload(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "artifact open")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_open_001",
		TaskID:              "task_artifact_open",
		ArtifactType:        "generated_doc",
		Title:               "artifact-open.md",
		Path:                "workspace/artifact-open.md",
		MimeType:            "text/markdown",
		DeliveryType:        "open_file",
		DeliveryPayloadJSON: `{"path":"workspace/artifact-open.md","task_id":"task_artifact_open"}`,
		CreatedAt:           "2026-04-14T10:05:00Z",
	}})
	if err != nil {
		t.Fatalf("save artifact failed: %v", err)
	}
	result, err := service.TaskArtifactOpen(map[string]any{"task_id": "task_artifact_open", "artifact_id": "art_open_001"})
	if err != nil {
		t.Fatalf("task artifact open failed: %v", err)
	}
	if result["open_action"] != "open_file" {
		t.Fatalf("expected open_file action, got %+v", result)
	}
	deliveryResult := result["delivery_result"].(map[string]any)
	if deliveryResult["type"] != "open_file" {
		t.Fatalf("expected open_file delivery result, got %+v", deliveryResult)
	}
	payload := result["resolved_payload"].(map[string]any)
	if payload["path"] != "workspace/artifact-open.md" {
		t.Fatalf("expected resolved payload path, got %+v", payload)
	}
	artifact := result["artifact"].(map[string]any)
	if _, ok := artifact["delivery_type"]; ok {
		t.Fatalf("expected opened artifact to omit undeclared delivery_type, got %+v", artifact)
	}
}

func TestServiceTaskArtifactOpenReturnsArtifactNotFoundWhenTaskExists(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "artifact not found")
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_artifact_not_found",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请整理成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.TaskArtifactOpen(map[string]any{"task_id": taskID, "artifact_id": "art_missing"})
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("expected ErrArtifactNotFound, got %v", err)
	}
}

func TestServiceStartTaskPersistsArtifactsToStore(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "persist artifact store")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_artifact_persist",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请整理成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	records, total, err := service.storage.ArtifactStore().ListArtifacts(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("list persisted artifacts failed: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("expected one persisted artifact, got total=%d records=%+v", total, records)
	}
	if records[0].DeliveryType != "workspace_document" {
		t.Fatalf("expected persisted workspace_document artifact, got %+v", records[0])
	}
}

func TestServiceDeliveryOpenReturnsTaskDeliveryResult(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "delivery open task")
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_delivery_open",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请整理成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	result, err := service.DeliveryOpen(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("delivery open failed: %v", err)
	}
	if result["open_action"] != "workspace_document" {
		t.Fatalf("expected workspace_document open action, got %+v", result)
	}
	payload := result["resolved_payload"].(map[string]any)
	if payload["task_id"] != taskID {
		t.Fatalf("expected payload to carry task_id, got %+v", payload)
	}
}

func TestServiceDeliveryOpenReturnsArtifactDeliveryResult(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "delivery open artifact")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_delivery_open_001",
		TaskID:              "task_delivery_open",
		ArtifactType:        "generated_doc",
		Title:               "delivery-open.md",
		Path:                "workspace/delivery-open.md",
		MimeType:            "text/markdown",
		DeliveryType:        "open_file",
		DeliveryPayloadJSON: `{"path":"workspace/delivery-open.md","task_id":"task_delivery_open"}`,
		CreatedAt:           "2026-04-14T10:10:00Z",
	}})
	if err != nil {
		t.Fatalf("save artifact failed: %v", err)
	}
	result, err := service.DeliveryOpen(map[string]any{"task_id": "task_delivery_open", "artifact_id": "art_delivery_open_001"})
	if err != nil {
		t.Fatalf("delivery open artifact failed: %v", err)
	}
	if result["open_action"] != "open_file" {
		t.Fatalf("expected open_file action, got %+v", result)
	}
	if result["artifact"].(map[string]any)["artifact_id"] != "art_delivery_open_001" {
		t.Fatalf("expected artifact payload, got %+v", result)
	}
	if _, ok := result["artifact"].(map[string]any)["delivery_type"]; ok {
		t.Fatalf("expected delivery-open artifact to omit undeclared delivery_type, got %+v", result["artifact"])
	}
}

func TestTaskArtifactHelpersCoverFallbackBranches(t *testing.T) {
	if got := inferArtifactDeliveryType(map[string]any{"path": "workspace/file.md"}); got != "open_file" {
		t.Fatalf("expected path-backed artifact to infer open_file, got %q", got)
	}
	if got := inferArtifactDeliveryType(map[string]any{"title": "no-path"}); got != "task_detail" {
		t.Fatalf("expected no-path artifact to infer task_detail, got %q", got)
	}
	result := normalizeDeliveryOpenResult(nil, map[string]any{"payload": map[string]any{}}, "task_001")
	if result["type"] != "task_detail" || result["title"] != "任务交付结果" || result["preview_text"] != "任务交付结果" {
		t.Fatalf("expected defaulted delivery result fields, got %+v", result)
	}
}

func TestServiceTaskArtifactListFallsBackToRuntimeArtifactsWhenStoreEmpty(t *testing.T) {
	service := newTestService()
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_runtime_artifact",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请整理成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	task, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected runtime task to exist")
	}
	_, _ = service.runEngine.SetPresentation(taskID, task.BubbleMessage, task.DeliveryResult, []map[string]any{{
		"artifact_id":      "art_runtime_001",
		"task_id":          taskID,
		"artifact_type":    "generated_doc",
		"title":            "runtime.md",
		"path":             "workspace/runtime.md",
		"mime_type":        "text/markdown",
		"delivery_type":    "workspace_document",
		"delivery_payload": map[string]any{"path": "workspace/runtime.md", "task_id": taskID},
	}})
	result, err := service.TaskArtifactList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task artifact list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["artifact_id"] != "art_runtime_001" {
		t.Fatalf("expected runtime artifact fallback to return item, got %+v", items)
	}
	if _, ok := items[0]["delivery_type"]; ok {
		t.Fatalf("expected runtime artifact fallback item to omit undeclared delivery_type, got %+v", items[0])
	}
}

func TestServiceRuntimeArtifactsBackfillStableArtifactIdentifiersWhenMissing(t *testing.T) {
	service := newTestService()
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_runtime_missing_artifact_id",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "只检查运行态 artifact id",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	task, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected runtime task to exist")
	}
	_, _ = service.runEngine.SetPresentation(taskID, task.BubbleMessage, task.DeliveryResult, []map[string]any{{
		"artifact_id":      "",
		"task_id":          taskID,
		"artifact_type":    "generated_file",
		"title":            "runtime-output.txt",
		"path":             "workspace/runtime-output.txt",
		"mime_type":        "text/plain",
		"delivery_type":    "open_file",
		"delivery_payload": map[string]any{"path": "workspace/runtime-output.txt", "task_id": taskID},
	}})

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	detailArtifacts := detailResult["artifacts"].([]map[string]any)
	if len(detailArtifacts) != 1 {
		t.Fatalf("expected one runtime detail artifact, got %+v", detailArtifacts)
	}
	artifactID, ok := detailArtifacts[0]["artifact_id"].(string)
	if !ok || artifactID == "" {
		t.Fatalf("expected runtime detail artifact to receive a stable id, got %+v", detailArtifacts[0])
	}

	listResult, err := service.TaskArtifactList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task artifact list failed: %v", err)
	}
	items := listResult["items"].([]map[string]any)
	if len(items) != 1 || items[0]["artifact_id"] != artifactID {
		t.Fatalf("expected runtime artifact list to reuse generated id, got %+v", items)
	}

	openResult, err := service.TaskArtifactOpen(map[string]any{"task_id": taskID, "artifact_id": artifactID})
	if err != nil {
		t.Fatalf("task artifact open failed: %v", err)
	}
	if openResult["artifact"].(map[string]any)["artifact_id"] != artifactID {
		t.Fatalf("expected artifact open to resolve generated runtime id, got %+v", openResult)
	}
}

func TestServiceTaskControlRejectsInvalidStatusTransition(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "this task still requires confirmation",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.TaskControl(map[string]any{
		"task_id": taskID,
		"action":  "pause",
	})
	if !errors.Is(err, ErrTaskStatusInvalid) {
		t.Fatalf("expected pause from confirming_intent to return ErrTaskStatusInvalid, got %v", err)
	}
}

func TestSettingsGetIncludesSecretConfigurationAvailability(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings secret availability")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	result, err := service.SettingsGet(map[string]any{"scope": "all"})
	if err != nil {
		t.Fatalf("settings get failed: %v", err)
	}
	dataLog := result["settings"].(map[string]any)["data_log"].(map[string]any)
	if dataLog["provider_api_key_configured"] != false {
		t.Fatalf("expected unset provider key flag, got %+v", dataLog)
	}
	if err := service.storage.SecretStore().PutSecret(context.Background(), storage.SecretRecord{
		Namespace: "model",
		Key:       service.model.Provider() + "_api_key",
		Value:     "secret-key",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed secret store failed: %v", err)
	}
	result, err = service.SettingsGet(map[string]any{"scope": "all"})
	if err != nil {
		t.Fatalf("settings get with secret failed: %v", err)
	}
	dataLog = result["settings"].(map[string]any)["data_log"].(map[string]any)
	if dataLog["provider_api_key_configured"] != true {
		t.Fatalf("expected configured provider key flag, got %+v", dataLog)
	}
}

func TestSettingsGetReturnsStrongholdErrorWhenSecretStoreUnreadable(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings secret error")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.Close(); err != nil {
		t.Fatalf("close storage failed: %v", err)
	}
	_, err := service.SettingsGet(map[string]any{"scope": "all"})
	if !errors.Is(err, ErrStrongholdAccessFailed) {
		t.Fatalf("expected ErrStrongholdAccessFailed, got %v", err)
	}
}

func TestSettingsUpdatePersistsSecretOutsideRegularSettings(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings secret persist")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	result, err := service.SettingsUpdate(map[string]any{
		"data_log": map[string]any{
			"provider":              "openai",
			"budget_auto_downgrade": false,
			"api_key":               "persisted-secret-key",
		},
	})
	if err != nil {
		t.Fatalf("settings update failed: %v", err)
	}
	stored, err := service.storage.SecretStore().GetSecret(context.Background(), "model", service.model.Provider()+"_api_key")
	if err != nil {
		t.Fatalf("expected stored secret, got %v", err)
	}
	if stored.Value != "persisted-secret-key" {
		t.Fatalf("unexpected stored secret: %+v", stored)
	}
	effectiveSettings := result["effective_settings"].(map[string]any)
	dataLog := effectiveSettings["data_log"].(map[string]any)
	if _, exists := dataLog["api_key"]; exists {
		t.Fatalf("expected api_key to stay out of regular settings path, got %+v", dataLog)
	}
	if dataLog["provider_api_key_configured"] != true {
		t.Fatalf("expected configured flag in settings response, got %+v", dataLog)
	}
}

func TestSettingsUpdatePersistsSecretForRequestedProvider(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings provider secret persist")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	_, err := service.SettingsUpdate(map[string]any{
		"data_log": map[string]any{
			"provider":              "anthropic",
			"budget_auto_downgrade": true,
			"api_key":               "anthropic-secret-key",
		},
	})
	if err != nil {
		t.Fatalf("settings update failed: %v", err)
	}
	stored, err := service.storage.SecretStore().GetSecret(context.Background(), "model", "anthropic_api_key")
	if err != nil {
		t.Fatalf("expected anthropic secret to be stored, got %v", err)
	}
	if stored.Value != "anthropic-secret-key" {
		t.Fatalf("unexpected stored anthropic secret: %+v", stored)
	}
	_, err = service.storage.SecretStore().GetSecret(context.Background(), "model", service.model.Provider()+"_api_key")
	if !errors.Is(err, storage.ErrSecretNotFound) {
		t.Fatalf("expected default provider secret to remain unset, got %v", err)
	}
	result, err := service.SettingsGet(map[string]any{"scope": "data_log"})
	if err != nil {
		t.Fatalf("settings get failed: %v", err)
	}
	dataLog := result["settings"].(map[string]any)["data_log"].(map[string]any)
	if dataLog["provider"] != "anthropic" || dataLog["provider_api_key_configured"] != true {
		t.Fatalf("expected settings get to reflect anthropic provider secret, got %+v", dataLog)
	}
}

func TestSettingsUpdateReturnsStrongholdErrorWithoutStorage(t *testing.T) {
	service := newTestService()
	_, err := service.SettingsUpdate(map[string]any{
		"data_log": map[string]any{
			"provider": "openai",
			"api_key":  "sk-test",
		},
	})
	if !errors.Is(err, ErrStrongholdAccessFailed) {
		t.Fatalf("expected ErrStrongholdAccessFailed, got %v", err)
	}
}

func TestServiceTaskControlRequiresTaskID(t *testing.T) {
	service := newTestService()

	_, err := service.TaskControl(map[string]any{
		"action": "pause",
	})
	if err == nil || err.Error() != "task_id is required" {
		t.Fatalf("expected task_id required error, got %v", err)
	}
}

func TestServiceTaskControlRequiresAction(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "task control needs action",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.TaskControl(map[string]any{
		"task_id": taskID,
	})
	if err == nil || err.Error() != "action is required" {
		t.Fatalf("expected action required error, got %v", err)
	}
}

func TestServiceTaskControlRejectsFinishedTaskOperations(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for task control error mapping",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.TaskControl(map[string]any{
		"task_id": taskID,
		"action":  "cancel",
	})
	if !errors.Is(err, ErrTaskAlreadyFinished) {
		t.Fatalf("expected cancel on completed task to return ErrTaskAlreadyFinished, got %v", err)
	}
}

func TestServiceTaskControlReturnsUpdatedTaskAndBubbleForWaitingAuthCancel(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_task_control_payload",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "task control should return stable payload",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	result, err := service.TaskControl(map[string]any{
		"task_id":   taskID,
		"action":    "cancel",
		"arguments": map[string]any{"reason": "user_cancelled_from_dashboard"},
	})
	if err != nil {
		t.Fatalf("task control failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["task_id"] != taskID {
		t.Fatalf("expected task control to keep task_id %s, got %+v", taskID, task)
	}
	if task["status"] != "cancelled" {
		t.Fatalf("expected cancelled task after task.control cancel, got %+v", task)
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["task_id"] != taskID || bubble["type"] != "status" {
		t.Fatalf("expected stable status bubble payload, got %+v", bubble)
	}
	if bubble["text"] != "任务已取消" {
		t.Fatalf("expected cancel bubble text, got %+v", bubble)
	}

	recordedTask, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected cancelled task to remain available in runtime")
	}
	if recordedTask.Status != "cancelled" || recordedTask.CurrentStep != "task_cancelled" {
		t.Fatalf("expected runtime task to stay aligned with task.control payload, got %+v", recordedTask)
	}
}

func TestServiceTaskEventsListReturnsNormalizedLoopEvents(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "loop event list")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime store to be wired")
	}
	if err := service.storage.LoopRuntimeStore().SaveEvents(context.Background(), []storage.EventRecord{{
		EventID:     "evt_loop_list_001",
		RunID:       "run_loop_list_001",
		TaskID:      "task_loop_list_001",
		StepID:      "step_loop_list_001",
		Type:        "loop.completed",
		Level:       "info",
		PayloadJSON: `{"stop_reason":"completed"}`,
		CreatedAt:   "2026-04-17T10:00:00Z",
	}}); err != nil {
		t.Fatalf("save loop events failed: %v", err)
	}

	result, err := service.TaskEventsList(map[string]any{"task_id": "task_loop_list_001", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task events list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["type"] != "loop.completed" {
		t.Fatalf("expected normalized loop event item, got %+v", items)
	}
	page := result["page"].(map[string]any)
	if page["total"] != 1 {
		t.Fatalf("expected total 1, got %+v", page)
	}
}

func TestServiceTaskSteerPersistsFollowUpMessage(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "task steer")
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_task_steer",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please write this into a file after authorization.",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)

	result, err := service.TaskSteer(map[string]any{"task_id": taskID, "message": "Also include a short summary section."})
	if err != nil {
		t.Fatalf("task steer failed: %v", err)
	}
	if result["task"].(map[string]any)["task_id"] != taskID {
		t.Fatalf("expected steered task id %s, got %+v", taskID, result)
	}
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected steered task to remain in runtime")
	}
	if len(record.SteeringMessages) != 1 || record.SteeringMessages[0] != "Also include a short summary section." {
		t.Fatalf("expected steering message to persist, got %+v", record.SteeringMessages)
	}
	if record.LatestEvent["type"] != "task.steered" {
		t.Fatalf("expected latest event task.steered, got %+v", record.LatestEvent)
	}
}

func TestServiceTaskListIncludesLoopStopReason(t *testing.T) {
	service := newTestService()
	for index := 0; index < 2; index++ {
		_, err := service.StartTask(map[string]any{
			"session_id": fmt.Sprintf("sess_loop_stop_%02d", index),
			"source":     "floating_ball",
			"trigger":    "hover_text_input",
			"input": map[string]any{
				"type": "text",
				"text": fmt.Sprintf("task %02d", index),
			},
			"intent": map[string]any{
				"name": "write_file",
				"arguments": map[string]any{
					"require_authorization": true,
				},
			},
		})
		if err != nil {
			t.Fatalf("start task %d failed: %v", index, err)
		}
	}
	items, total := service.runEngine.ListTasks("unfinished", "updated_at", "desc", 20, 0)
	if total == 0 {
		t.Fatal("expected tasks to exist")
	}
	updated, ok := service.runEngine.RecordLoopLifecycle(items[0].TaskID, "loop.failed", "tool_retry_exhausted", map[string]any{"stop_reason": "tool_retry_exhausted"})
	if !ok {
		t.Fatal("expected loop lifecycle update to succeed")
	}
	result, err := service.TaskList(map[string]any{"group": "unfinished", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}
	listed := result["items"].([]map[string]any)
	if listed[0]["task_id"] != updated.TaskID || listed[0]["loop_stop_reason"] != "tool_retry_exhausted" {
		t.Fatalf("expected task list to expose loop stop reason, got %+v", listed[0])
	}
}

func TestServiceStartTaskWithExecutorWritesWorkspaceDocument(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "第一点\n第二点\n第三点")

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请整理成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	deliveryResult := result["delivery_result"].(map[string]any)
	payload := deliveryResult["payload"].(map[string]any)
	outputPath := payload["path"].(string)
	if outputPath == "" {
		t.Fatal("expected workspace document delivery to carry a payload path")
	}

	content, err := os.ReadFile(filepath.Join(workspaceRoot, strings.TrimPrefix(outputPath, "workspace/")))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !strings.Contains(string(content), "# 处理结果") {
		t.Fatalf("expected written file to contain title header, got %s", string(content))
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "write_file" {
		t.Fatalf("expected runtime task to record write_file tool call, got %v", record.LatestToolCall["tool_name"])
	}
	output, ok := record.LatestToolCall["output"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest tool call output map, got %+v", record.LatestToolCall)
		if output["summary_output"] == nil {
			t.Fatalf("expected write_file tool output to include summary_output, got %+v", output)
		}
		if output["model_invocation"] == nil {
			t.Fatalf("expected latest tool call to include model invocation, got %+v", output)
		}
		if output["audit_record"] == nil {
			t.Fatalf("expected latest tool call to include audit record, got %+v", output)
		}
		if output["recovery_point"] != nil {
			t.Fatalf("expected no recovery_point for create flow, got %+v", output)
		}
	}
}

func TestServiceStartTaskWithExecutorReturnsGeneratedBubble(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "这段内容主要在解释当前问题的原因和处理方向。")

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请解释这段内容",
		},
		"intent": map[string]any{
			"name":      "explain",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	bubble := result["bubble_message"].(map[string]any)
	if bubble["text"] != "这段内容主要在解释当前问题的原因和处理方向。" {
		t.Fatalf("expected bubble text to use generated output, got %v", bubble["text"])
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "generate_text" {
		t.Fatalf("expected runtime task to record generate_text tool call, got %v", record.LatestToolCall["tool_name"])
	}
	output, ok := record.LatestToolCall["output"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest tool call output map, got %+v", record.LatestToolCall)
	}
	if output["model_invocation"] == nil {
		t.Fatalf("expected latest tool call to include model invocation, got %+v", output)
	}
	if output["audit_record"] == nil {
		t.Fatalf("expected latest tool call to include audit record, got %+v", output)
	}
}

func TestServiceStartTaskWithExecutorDeliversPageReadBubble(t *testing.T) {
	service, _ := newTestServiceWithExecutionAndPlaywright(t, "unused", platform.LocalExecutionBackend{}, nil, stubPlaywrightClient{readResult: tools.BrowserPageReadResult{
		Title:       "Example Domain",
		TextContent: "This domain is for use in illustrative examples in documents.",
		MIMEType:    "text/html",
		TextType:    "text/html",
		Source:      "playwright_sidecar",
	}})

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_page_read",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请读取这个网页",
		},
		"intent": map[string]any{
			"name": "page_read",
			"arguments": map[string]any{
				"url": "https://example.com",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	if result["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected page_read task to wait for authorization, got %+v", result)
	}
	result, err = service.SecurityRespond(map[string]any{
		"task_id":     result["task"].(map[string]any)["task_id"],
		"approval_id": result["task"].(map[string]any)["task_id"],
		"decision":    "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	deliveryResult := result["delivery_result"].(map[string]any)
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected bubble delivery result, got %+v", deliveryResult)
	}
	bubble := result["bubble_message"].(map[string]any)
	if !strings.Contains(bubble["text"].(string), "illustrative examples") {
		t.Fatalf("expected bubble text to contain page preview, got %+v", bubble)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "page_read" {
		t.Fatalf("expected runtime task to record page_read tool call, got %v", record.LatestToolCall["tool_name"])
	}
	output, ok := record.LatestToolCall["output"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest tool call output map, got %+v", record.LatestToolCall)
	}
	if output["title"] != "Example Domain" {
		t.Fatalf("expected page_read tool output title to be recorded, got %+v", output)
	}
	if output["content_preview"] == nil {
		t.Fatalf("expected page_read tool output preview to be recorded, got %+v", output)
	}
}

func TestServiceStartTaskWithExecutorDeliversPageSearchBubble(t *testing.T) {
	service, _ := newTestServiceWithExecutionAndPlaywright(t, "unused", platform.LocalExecutionBackend{}, nil, stubPlaywrightClient{searchResult: tools.BrowserPageSearchResult{
		Matches:    []string{"Keyword beta lives here"},
		MatchCount: 1,
		Source:     "playwright_sidecar",
	}})

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_page_search",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请搜索这个网页",
		},
		"intent": map[string]any{
			"name": "page_search",
			"arguments": map[string]any{
				"url":   "https://example.com",
				"query": "beta",
				"limit": 2,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	if result["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected page_search task to wait for authorization, got %+v", result)
	}
	result, err = service.SecurityRespond(map[string]any{
		"task_id":     result["task"].(map[string]any)["task_id"],
		"approval_id": result["task"].(map[string]any)["task_id"],
		"decision":    "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	deliveryResult := result["delivery_result"].(map[string]any)
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected bubble delivery result, got %+v", deliveryResult)
	}
	bubble := result["bubble_message"].(map[string]any)
	if !strings.Contains(bubble["text"].(string), "关键词") {
		t.Fatalf("expected page_search bubble summary, got %+v", bubble)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "page_search" {
		t.Fatalf("expected runtime task to record page_search tool call, got %+v", record.LatestToolCall)
	}
}

func TestServiceWorkerToolWritesToolCallEventNotification(t *testing.T) {
	service, _ := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), stubOCRWorkerClient{result: tools.OCRTextResult{Path: "notes/demo.txt", Text: "hello from ocr", Language: "plain_text", PageCount: 1, Source: "ocr_worker_text"}}, sidecarclient.NewNoopMediaWorkerClient())
	result, err := service.StartTask(map[string]any{
		"session_id": "sess_ocr_extract",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请提取文本",
		},
		"intent": map[string]any{
			"name": "extract_text",
			"arguments": map[string]any{
				"path": "notes/demo.txt",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "extract_text" {
		t.Fatalf("expected extract_text latest tool call, got %+v", record.LatestToolCall)
	}
	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected task notifications")
	}
	foundToolCallEvent := false
	for _, notification := range notifications {
		if notification.Method != "tool_call.completed" {
			continue
		}
		toolCall, _ := notification.Params["tool_call"].(map[string]any)
		eventPayload, _ := notification.Params["event"].(map[string]any)
		payload, _ := eventPayload["payload"].(map[string]any)
		if toolCall["tool_name"] == "extract_text" && payload["source"] == "ocr_worker_text" {
			foundToolCallEvent = true
		}
	}
	if !foundToolCallEvent {
		t.Fatal("expected tool_call.completed notification to be queued for OCR worker")
	}
}

func TestServiceMediaWorkerPropagatesArtifactsAndWorkerEventPayload(t *testing.T) {
	service, _ := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), sidecarclient.NewNoopOCRWorkerClient(), stubMediaWorkerClient{transcodeResult: tools.MediaTranscodeResult{InputPath: "clips/demo.mov", OutputPath: "clips/demo.mp4", Format: "mp4", Source: "media_worker_ffmpeg"}})
	result, err := service.StartTask(map[string]any{
		"session_id": "sess_media_transcode",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请转码视频",
		},
		"intent": map[string]any{
			"name": "transcode_media",
			"arguments": map[string]any{
				"path":        "clips/demo.mov",
				"output_path": "clips/demo.mp4",
				"format":      "mp4",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "transcode_media" {
		t.Fatalf("expected transcode_media latest tool call, got %+v", record.LatestToolCall)
	}
	toolOutput, _ := record.LatestToolCall["output"].(map[string]any)
	if toolOutput["output_path"] != "clips/demo.mp4" {
		t.Fatalf("expected media worker output path in tool call, got %+v", toolOutput)
	}
	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected task notifications")
	}
	foundToolCallEvent := false
	for _, notification := range notifications {
		if notification.Method != "tool_call.completed" {
			continue
		}
		eventPayload, _ := notification.Params["event"].(map[string]any)
		payload, _ := eventPayload["payload"].(map[string]any)
		if payload["source"] == "media_worker_ffmpeg" && payload["output_path"] == "clips/demo.mp4" {
			foundToolCallEvent = true
		}
	}
	if !foundToolCallEvent {
		t.Fatalf("expected media worker tool_call.completed notification with output metadata, got %+v", notifications)
	}
}

func TestServiceStartTaskWithExecutorPageReadFailureUsesUnifiedError(t *testing.T) {
	service, _ := newTestServiceWithExecutionAndPlaywright(t, "unused", platform.LocalExecutionBackend{}, nil, stubPlaywrightClient{err: tools.ErrPlaywrightSidecarFailed})

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_page_read_fail",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请读取这个网页",
		},
		"intent": map[string]any{
			"name": "page_read",
			"arguments": map[string]any{
				"url": "https://example.com",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task should return task-centric failure result, got %v", err)
	}
	if result["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected page_read failure task to wait for authorization, got %+v", result)
	}
	result, err = service.SecurityRespond(map[string]any{
		"task_id":     result["task"].(map[string]any)["task_id"],
		"approval_id": result["task"].(map[string]any)["task_id"],
		"decision":    "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond should surface task-centric failure result, got %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected failed task to remain in runtime")
	}
	if record.Status != "failed" {
		t.Fatalf("expected failed status, got %+v", record)
	}
	if record.LatestToolCall["tool_name"] != "page_read" {
		t.Fatalf("expected runtime task to record page_read failure, got %+v", record.LatestToolCall)
	}
	if record.LatestToolCall["error_code"] != tools.ToolErrorCodePlaywrightSidecarFail {
		t.Fatalf("expected unified sidecar error code, got %+v", record.LatestToolCall)
	}
}

func TestServiceStartTaskWithRealLocalPageReadDelivery(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Local Acceptance Page</title></head><body><p>Local acceptance page verifies end to end page read delivery.</p><p>Keyword beta lives here.</p></body></html>`))
	})}
	defer server.Close()
	go func() {
		_ = server.Serve(listener)
	}()

	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := sidecarclient.NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	if err := runtime.Start(); err != nil {
		t.Skipf("playwright runtime unavailable in test environment: %v", err)
	}
	defer runtime.Stop()

	service, _ := newTestServiceWithExecutionAndPlaywright(t, "unused", platform.LocalExecutionBackend{}, nil, runtime.Client())
	result, err := service.StartTask(map[string]any{
		"session_id": "sess_real_page_read",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请读取本地网页",
		},
		"intent": map[string]any{
			"name": "page_read",
			"arguments": map[string]any{
				"url": "http://" + listener.Addr().String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	if result["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected real page_read task to wait for authorization, got %+v", result)
	}
	result, err = service.SecurityRespond(map[string]any{
		"task_id":     result["task"].(map[string]any)["task_id"],
		"approval_id": result["task"].(map[string]any)["task_id"],
		"decision":    "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	deliveryResult := result["delivery_result"].(map[string]any)
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected bubble delivery result, got %+v", deliveryResult)
	}
	bubble := result["bubble_message"].(map[string]any)
	if !strings.Contains(bubble["text"].(string), "Local acceptance page") {
		t.Fatalf("expected real local page preview in bubble, got %+v", bubble)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "page_read" {
		t.Fatalf("expected runtime task to record page_read tool call, got %+v", record.LatestToolCall)
	}
	if record.LatestEvent["type"] != "delivery.ready" {
		t.Fatalf("expected delivery.ready latest event, got %+v", record.LatestEvent)
	}
}

func modelConfig() serviceconfig.ModelConfig {
	return serviceconfig.ModelConfig{
		Provider: "openai_responses",
		ModelID:  "gpt-5.4",
		Endpoint: "https://api.openai.com/v1/responses",
	}
}
