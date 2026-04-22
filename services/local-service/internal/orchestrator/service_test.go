// Orchestrator service tests cover the main task flow and RPC-facing integration points.
package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
	_ "modernc.org/sqlite"
)

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

type localHTTPPlaywrightClient struct{}

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

func (localHTTPPlaywrightClient) ReadPage(_ context.Context, url string) (tools.BrowserPageReadResult, error) {
	response, err := http.Get(url)
	if err != nil {
		return tools.BrowserPageReadResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return tools.BrowserPageReadResult{}, err
	}
	text := string(body)
	title := ""
	lower := strings.ToLower(text)
	start := strings.Index(lower, "<title>")
	end := strings.Index(lower, "</title>")
	if start >= 0 && end > start+len("<title>") {
		title = text[start+len("<title>") : end]
	}
	return tools.BrowserPageReadResult{
		URL:         url,
		Title:       title,
		TextContent: text,
		MIMEType:    response.Header.Get("Content-Type"),
		TextType:    "html",
		Source:      "local_http_playwright_client",
	}, nil
}

func (localHTTPPlaywrightClient) SearchPage(_ context.Context, url, query string, _ int) (tools.BrowserPageSearchResult, error) {
	return tools.BrowserPageSearchResult{}, tools.ErrPlaywrightSidecarFailed
}

func (localHTTPPlaywrightClient) InteractPage(_ context.Context, _ string, _ []map[string]any) (tools.BrowserPageInteractResult, error) {
	return tools.BrowserPageInteractResult{}, tools.ErrPlaywrightSidecarFailed
}

func (localHTTPPlaywrightClient) StructuredDOM(_ context.Context, _ string) (tools.BrowserStructuredDOMResult, error) {
	return tools.BrowserStructuredDOMResult{}, tools.ErrPlaywrightSidecarFailed
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

type failingEvalSnapshotStore struct {
	err error
}

type failingApprovalRequestStore struct {
	base storage.ApprovalRequestStore
	err  error
}

type failingAuthorizationRecordStore struct {
	base storage.AuthorizationRecordStore
	err  error
}

func (s failingEvalSnapshotStore) WriteEvalSnapshot(context.Context, storage.EvalSnapshotRecord) error {
	return s.err
}

func (s failingEvalSnapshotStore) ListEvalSnapshots(context.Context, string, int, int) ([]storage.EvalSnapshotRecord, int, error) {
	return nil, 0, s.err
}

func (s failingApprovalRequestStore) WriteApprovalRequest(ctx context.Context, record storage.ApprovalRequestRecord) error {
	if s.err != nil {
		return s.err
	}
	if s.base == nil {
		return nil
	}
	return s.base.WriteApprovalRequest(ctx, record)
}

func (s failingApprovalRequestStore) UpdateApprovalRequestStatus(ctx context.Context, approvalID string, status string, updatedAt string) error {
	if s.err != nil {
		return s.err
	}
	if s.base == nil {
		return nil
	}
	return s.base.UpdateApprovalRequestStatus(ctx, approvalID, status, updatedAt)
}

func (s failingApprovalRequestStore) ListApprovalRequests(ctx context.Context, taskID string, limit, offset int) ([]storage.ApprovalRequestRecord, int, error) {
	if s.base == nil {
		return nil, 0, nil
	}
	return s.base.ListApprovalRequests(ctx, taskID, limit, offset)
}

func (s failingApprovalRequestStore) ListPendingApprovalRequests(ctx context.Context, limit, offset int) ([]storage.ApprovalRequestRecord, int, error) {
	if s.base == nil {
		return nil, 0, nil
	}
	return s.base.ListPendingApprovalRequests(ctx, limit, offset)
}

func (s failingAuthorizationRecordStore) WriteAuthorizationRecord(ctx context.Context, record storage.AuthorizationRecordRecord) error {
	if s.err != nil {
		return s.err
	}
	if s.base == nil {
		return nil
	}
	return s.base.WriteAuthorizationRecord(ctx, record)
}

func (s failingAuthorizationRecordStore) WriteAuthorizationDecision(ctx context.Context, record storage.AuthorizationRecordRecord, approvalStatus string, updatedAt string) error {
	if s.err != nil {
		return s.err
	}
	if s.base == nil {
		return nil
	}
	return s.base.WriteAuthorizationDecision(ctx, record, approvalStatus, updatedAt)
}

func (s failingAuthorizationRecordStore) ListAuthorizationRecords(ctx context.Context, taskID string, limit, offset int) ([]storage.AuthorizationRecordRecord, int, error) {
	if s.base == nil {
		return nil, 0, nil
	}
	return s.base.ListAuthorizationRecords(ctx, taskID, limit, offset)
}

type countingTaskRunStore struct {
	base      storage.TaskRunStore
	loadCalls int
}

type countingTaskStore struct {
	base      storage.TaskStore
	listCalls int
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

func (s *countingTaskStore) WriteTask(ctx context.Context, record storage.TaskRecord) error {
	return s.base.WriteTask(ctx, record)
}

func (s *countingTaskStore) DeleteTask(ctx context.Context, taskID string) error {
	return s.base.DeleteTask(ctx, taskID)
}

func (s *countingTaskStore) GetTask(ctx context.Context, taskID string) (storage.TaskRecord, error) {
	return s.base.GetTask(ctx, taskID)
}

func (s *countingTaskStore) ListTasks(ctx context.Context, limit, offset int) ([]storage.TaskRecord, int, error) {
	s.listCalls++
	return s.base.ListTasks(ctx, limit, offset)
}

func (s *countingTaskStore) ListTasksBySession(ctx context.Context, sessionID string, limit, offset int) ([]storage.TaskRecord, int, error) {
	return s.base.ListTasksBySession(ctx, sessionID, limit, offset)
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

func intPtr(value int) *int {
	return &value
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
	toolExecutor := tools.NewToolExecutor(toolRegistry, tools.WithToolCallRecorder(tools.NewToolCallRecorder(storageService.ToolCallSink())))
	pluginService := plugin.NewService()
	seedTestExtensionAssets(t, storageService, pluginService)
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executor := execution.NewService(fileSystem, executionBackend, playwrightClient, ocrClient, mediaClient, sidecarclient.NewLocalScreenCaptureClient(fileSystem), modelService, auditService, checkpoint.NewService(checkpointWriter), deliveryService, toolRegistry, toolExecutor, pluginService).WithArtifactStore(storageService.ArtifactStore()).WithExtensionAssetCatalog(storageService)

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
	).WithAudit(auditService).WithStorage(storageService).WithExecutor(executor).WithTaskInspector(taskinspector.NewService(fileSystem)).WithTraceEval(traceeval.NewService(storageService.TraceStore(), storageService.EvalStore()))

	return service, workspaceRoot
}

func newTestServiceWithModelService(t *testing.T, modelService *model.Service) (*Service, string, *storage.Service) {
	t.Helper()

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "service.db")))
	t.Cleanup(func() { _ = storageService.Close() })
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
	toolExecutor := tools.NewToolExecutor(toolRegistry, tools.WithToolCallRecorder(tools.NewToolCallRecorder(storageService.ToolCallSink())))
	pluginService := plugin.NewService()
	seedTestExtensionAssets(t, storageService, pluginService)
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executor := execution.NewService(fileSystem, platform.LocalExecutionBackend{}, sidecarclient.NewNoopPlaywrightSidecarClient(), sidecarclient.NewNoopOCRWorkerClient(), sidecarclient.NewNoopMediaWorkerClient(), sidecarclient.NewLocalScreenCaptureClient(fileSystem), modelService, auditService, checkpoint.NewService(storageService.RecoveryPointWriter()), deliveryService, toolRegistry, toolExecutor, pluginService).WithArtifactStore(storageService.ArtifactStore()).WithExtensionAssetCatalog(storageService)

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
	).WithAudit(auditService).WithStorage(storageService).WithExecutor(executor).WithTaskInspector(taskinspector.NewService(fileSystem)).WithTraceEval(traceeval.NewService(storageService.TraceStore(), storageService.EvalStore()))

	return service, workspaceRoot, storageService
}

func seedTestExtensionAssets(t *testing.T, storageService *storage.Service, pluginService *plugin.Service) {
	t.Helper()
	if err := storageService.EnsureBuiltinExecutionAssets(context.Background()); err != nil {
		t.Fatalf("ensure builtin execution assets: %v", err)
	}
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

func mustNewStoredEngine(t *testing.T, taskStore storage.TaskRunStore) *runengine.Engine {
	t.Helper()
	engine, err := runengine.NewEngineWithStore(taskStore)
	if err != nil {
		t.Fatalf("new stored engine: %v", err)
	}
	return engine
}

type storageTestAdapter struct {
	databasePath string
}

func (s storageTestAdapter) DatabasePath() string {
	return s.databasePath
}

func (s storageTestAdapter) SecretStorePath() string {
	if s.databasePath == "" {
		return ""
	}
	return s.databasePath + ".stronghold"
}

func newTestService() *Service {
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		panic(err)
	}
	if err := sidecarclient.RegisterPlaywrightTools(toolRegistry); err != nil {
		panic(err)
	}
	if err := sidecarclient.RegisterOCRTools(toolRegistry); err != nil {
		panic(err)
	}
	if err := sidecarclient.RegisterMediaTools(toolRegistry); err != nil {
		panic(err)
	}
	return NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		toolRegistry,
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

func replaceRuntimeClock(t *testing.T, engine *runengine.Engine, clock func() time.Time) {
	t.Helper()

	engineValue := reflect.ValueOf(engine).Elem()
	field := engineValue.FieldByName("now")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(clock))
}

func replaceTaskRunStore(t *testing.T, service *storage.Service, store storage.TaskRunStore) {
	t.Helper()

	serviceValue := reflect.ValueOf(service).Elem()
	field := serviceValue.FieldByName("taskRunStore")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(store))
}

func replaceTaskStore(t *testing.T, service *storage.Service, store storage.TaskStore) {
	t.Helper()

	serviceValue := reflect.ValueOf(service).Elem()
	field := serviceValue.FieldByName("taskStore")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(store))
}

func replaceApprovalRequestStore(t *testing.T, service *storage.Service, store storage.ApprovalRequestStore) {
	t.Helper()

	serviceValue := reflect.ValueOf(service).Elem()
	field := serviceValue.FieldByName("approvalRequestStore")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(store))
}

func replaceAuthorizationRecordStore(t *testing.T, service *storage.Service, store storage.AuthorizationRecordStore) {
	t.Helper()

	serviceValue := reflect.ValueOf(service).Elem()
	field := serviceValue.FieldByName("authorizationRecordStore")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(store))
}

func replaceSecretStore(t *testing.T, service *storage.Service, store storage.SecretStore) {
	t.Helper()

	serviceValue := reflect.ValueOf(service).Elem()
	field := serviceValue.FieldByName("secretStore")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(store))
}

// TestServiceStartTaskAndConfirmFlow verifies that a confirmed standard task
// continues execution and completes delivery.
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

func TestServiceSubmitInputUsesConfiguredOpenAIResponsesClient(t *testing.T) {
	type capturedRequest struct {
		Model      string        `json:"model"`
		Input      string        `json:"input"`
		Tools      []interface{} `json:"tools"`
		ToolChoice string        `json:"tool_choice"`
	}

	var captured capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if got := r.Header.Get("Authorization"); got != "Bearer formal-mainline-key" {
			t.Fatalf("authorization header mismatch: got %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("parse request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_formal_mainline","model":"gpt-5.4","output_text":"Configured Responses output.","usage":{"input_tokens":5,"output_tokens":7,"total_tokens":12}}`))
	}))
	defer server.Close()

	modelService, err := model.NewServiceFromConfig(model.ServiceConfig{
		ModelConfig: serviceconfig.ModelConfig{
			Provider: model.OpenAIResponsesProvider,
			ModelID:  "gpt-5.4",
			Endpoint: server.URL,
		},
		APIKey: "formal-mainline-key",
	})
	if err != nil {
		t.Fatalf("new model service from config: %v", err)
	}

	service, _, _ := newTestServiceWithModelService(t, modelService)
	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_formal_mainline",
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
		t.Fatalf("expected configured model path to complete task, got %+v", task)
	}
	if captured.Model != "gpt-5.4" {
		t.Fatalf("expected configured model id to reach responses client, got %+v", captured)
	}
	if captured.ToolChoice != "auto" || len(captured.Tools) == 0 {
		t.Fatalf("expected configured responses request to include tool-calling metadata, got %+v", captured)
	}
	if !strings.Contains(captured.Input, "Translate this note into English") {
		t.Fatalf("expected user request to reach responses client, got %+v", captured)
	}
	deliveryResult, ok := result["delivery_result"].(map[string]any)
	if !ok || deliveryResult["type"] != "bubble" {
		t.Fatalf("expected configured model path to return bubble delivery, got %+v", result["delivery_result"])
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

func TestServiceSecurityRespondResumesQueuedScreenAnalyzeTaskThroughApproval(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_local_0001/frame_0001.png", Text: "fatal build error", Language: "eng", Source: "ocr_worker_text"}}
	service, workspaceRoot := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "inputs"), 0o755); err != nil {
		t.Fatalf("mkdir inputs failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "inputs", "screen.png"), []byte("fake screen capture"), 0o644); err != nil {
		t.Fatalf("write screen input failed: %v", err)
	}

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_screen_queue",
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
		"session_id": "sess_screen_queue",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请分析屏幕中的错误",
		},
		"intent": map[string]any{
			"name": "screen_analyze",
			"arguments": map[string]any{
				"path": "inputs/screen.png",
			},
		},
	})
	if err != nil {
		t.Fatalf("second start task failed: %v", err)
	}
	secondTaskID := secondResult["task"].(map[string]any)["task_id"].(string)
	if secondResult["task"].(map[string]any)["status"] != "blocked" {
		t.Fatalf("expected queued screen task to stay blocked before approval is created, got %+v", secondResult["task"])
	}

	if _, err := service.SecurityRespond(map[string]any{
		"task_id":       firstTaskID,
		"approval_id":   "appr_screen_queue_first",
		"decision":      "allow_once",
		"remember_rule": false,
	}); err != nil {
		t.Fatalf("security respond failed for first task: %v", err)
	}

	secondTask, ok := service.runEngine.GetTask(secondTaskID)
	if !ok {
		t.Fatal("expected queued screen task to remain available in runtime")
	}
	if secondTask.Status != "waiting_auth" || secondTask.CurrentStep != "waiting_authorization" {
		t.Fatalf("expected queued screen task to re-enter waiting authorization, got %+v", secondTask)
	}
	if len(secondTask.ApprovalRequest) == 0 || stringValue(secondTask.PendingExecution, "kind", "") != "screen_analysis" {
		t.Fatalf("expected queued screen task to rebuild approval state, got %+v", secondTask)
	}

	screenResult, err := service.SecurityRespond(map[string]any{
		"task_id":       secondTaskID,
		"approval_id":   "appr_screen_queue_second",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed for queued screen task: %v", err)
	}
	if screenResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected queued screen task to complete after approval, got %+v", screenResult["task"])
	}
	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": secondTaskID})
	if err != nil {
		t.Fatalf("task detail get for queued screen task failed: %v", err)
	}
	if detailResult["approval_request"] != nil {
		t.Fatalf("expected completed queued screen task to clear approval_request, got %+v", detailResult["approval_request"])
	}
	authorizationRecord, ok := detailResult["authorization_record"].(map[string]any)
	if !ok || authorizationRecord["task_id"] != secondTaskID || authorizationRecord["decision"] != "allow_once" {
		t.Fatalf("expected queued screen task detail to retain authorization record, got %+v", detailResult["authorization_record"])
	}
	auditRecord, ok := detailResult["audit_record"].(map[string]any)
	if !ok || auditRecord["task_id"] != secondTaskID {
		t.Fatalf("expected queued screen task detail to retain latest audit record, got %+v", detailResult["audit_record"])
	}
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 1 {
		t.Fatalf("expected queued screen task detail to retain one formal citation, got %+v", citations)
	}
	if !strings.Contains(stringValue(citations[0], "label", ""), "error_evidence") {
		t.Fatalf("expected queued screen task citation to preserve evidence role, got %+v", citations[0])
	}
	securitySummary := detailResult["security_summary"].(map[string]any)
	if securitySummary["latest_restore_point"] == nil {
		t.Fatalf("expected queued screen task detail to retain recovery point summary, got %+v", securitySummary)
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

func TestExecuteTaskPersistsTraceAndEvalSnapshots(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed trace summary")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_trace_eval",
		Title:       "trace eval task",
		SourceType:  "floating_ball",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	updated, _, _, _, err := service.executeTask(task, contextsvc.TaskContextSnapshot{InputType: "text", Text: "please summarize this content"}, map[string]any{"name": "summarize", "arguments": map[string]any{}})
	if err != nil {
		t.Fatalf("executeTask failed: %v", err)
	}
	traces, total, err := service.storage.TraceStore().ListTraceRecords(context.Background(), updated.TaskID, 10, 0)
	if err != nil || total != 1 || len(traces) != 1 {
		t.Fatalf("expected one trace record, total=%d len=%d err=%v", total, len(traces), err)
	}
	if traces[0].ReviewResult != "passed" {
		t.Fatalf("expected passing trace review result, got %+v", traces[0])
	}
	if traces[0].AssetRefsJSON == "" {
		t.Fatalf("expected trace record to keep extension asset refs, got %+v", traces[0])
	}
	evals, total, err := service.storage.EvalStore().ListEvalSnapshots(context.Background(), updated.TaskID, 10, 0)
	if err != nil || total != 1 || len(evals) != 1 {
		t.Fatalf("expected one eval snapshot, total=%d len=%d err=%v", total, len(evals), err)
	}
	if evals[0].Status != "passed" {
		t.Fatalf("expected passing eval snapshot status, got %+v", evals[0])
	}
	if evals[0].AssetRefsJSON == "" {
		t.Fatalf("expected eval snapshot to keep extension asset refs, got %+v", evals[0])
	}
}

func TestMaybeEscalateHumanLoopBlocksTask(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "unused")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl",
		Title:       "doom loop task",
		SourceType:  "floating_ball",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "agent_loop",
		RiskLevel:   "yellow",
	})
	capture, err := service.traceEval.Capture(traceeval.CaptureInput{
		TaskID:     task.TaskID,
		RunID:      task.RunID,
		IntentName: "agent_loop",
		Snapshot:   contextsvc.TaskContextSnapshot{Text: "keep trying"},
		ToolCalls: []tools.ToolCallRecord{
			{ToolName: "read_file", Output: map[string]any{"loop_round": 3}},
			{ToolName: "read_file", Output: map[string]any{"loop_round": 3}},
			{ToolName: "read_file", Output: map[string]any{"loop_round": 3}},
		},
		DurationMS: 500,
	})
	if err != nil {
		t.Fatalf("trace capture failed: %v", err)
	}
	escalated, bubble, ok := service.maybeEscalateHumanLoop(task, capture)
	if !ok {
		t.Fatal("expected human escalation to block task")
	}
	if escalated.Status != "blocked" || escalated.CurrentStep != "human_in_loop" {
		t.Fatalf("expected blocked human_in_loop task, got %+v", escalated)
	}
	if bubble == nil || !strings.Contains(bubble["text"].(string), "Doom Loop") {
		t.Fatalf("expected escalation bubble to mention doom loop, got %+v", bubble)
	}
}

func TestServiceTaskControlResumeExecutesHumanLoopTask(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Recovered after review.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_resume",
		Title:       "总结：Please summarize this after review",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: contextsvc.TaskContextSnapshot{
			Text:      "Please summarize this after review",
			InputType: "text",
			Trigger:   "hover_text_input",
		},
	})
	taskID := task.TaskID
	if _, ok := service.runEngine.EscalateHumanLoop(taskID, map[string]any{"reason": "doom_loop", "status": "pending"}, map[string]any{"task_id": taskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}

	result, err := service.TaskControl(map[string]any{
		"task_id": taskID,
		"action":  "resume",
		"arguments": map[string]any{
			"review": map[string]any{
				"decision":    "approve",
				"reviewer_id": "reviewer_001",
				"notes":       "looks safe to continue",
			},
		},
	})
	if err != nil {
		t.Fatalf("task control resume failed: %v", err)
	}
	updatedTask := result["task"].(map[string]any)
	if updatedTask["status"] != "completed" {
		t.Fatalf("expected human loop resume to finish task, got %+v", updatedTask)
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["type"] != "result" || !strings.Contains(bubble["text"].(string), "workspace/") {
		t.Fatalf("expected resumed execution to return result bubble, got %+v", bubble)
	}
	record, ok := service.runEngine.GetTask(taskID)
	if !ok || record.PendingExecution != nil {
		t.Fatalf("expected resumed task to clear pending execution, got %+v", record)
	}
}

func TestExecutionSegmentKindClassifiesInitialResumeAndRestart(t *testing.T) {
	initialTask := runengine.TaskRecord{RunID: "run_same", Status: "processing", ExecutionAttempt: 1}
	initialProcessing := runengine.TaskRecord{RunID: "run_same", Status: "processing", ExecutionAttempt: 1}
	if segment := executionSegmentKind(initialTask, initialProcessing); segment != executionSegmentInitial {
		t.Fatalf("expected initial segment, got %s", segment)
	}
	if attempt := executionAttemptIndex(initialTask, initialProcessing); attempt != 1 {
		t.Fatalf("expected initial attempt index 1, got %d", attempt)
	}

	resumedTask := runengine.TaskRecord{RunID: "run_same", Status: "paused", ExecutionAttempt: 1}
	resumedProcessing := runengine.TaskRecord{RunID: "run_same", Status: "processing", ExecutionAttempt: 1}
	if segment := executionSegmentKind(resumedTask, resumedProcessing); segment != executionSegmentResume {
		t.Fatalf("expected resume segment, got %s", segment)
	}
	if attempt := executionAttemptIndex(resumedTask, resumedProcessing); attempt != 1 {
		t.Fatalf("expected resume to stay in first attempt, got %d", attempt)
	}

	restartedTask := runengine.TaskRecord{RunID: "run_before_restart", Status: "completed", ExecutionAttempt: 1}
	restartedProcessing := runengine.TaskRecord{RunID: "run_after_restart", Status: "processing", ExecutionAttempt: 2}
	if segment := executionSegmentKind(restartedTask, restartedProcessing); segment != executionSegmentRestart {
		t.Fatalf("expected restart segment, got %s", segment)
	}
	if attempt := executionAttemptIndex(restartedTask, restartedProcessing); attempt != 2 {
		t.Fatalf("expected restart to increment attempt index, got %d", attempt)
	}

	restartedAgainTask := runengine.TaskRecord{RunID: "run_after_restart", Status: "completed", ExecutionAttempt: 2}
	restartedAgainProcessing := runengine.TaskRecord{RunID: "run_after_restart_2", Status: "processing", ExecutionAttempt: 3}
	if attempt := executionAttemptIndex(restartedAgainTask, restartedAgainProcessing); attempt != 3 {
		t.Fatalf("expected second restart to reach attempt index 3, got %d", attempt)
	}

	legacyRestartProcessing := runengine.TaskRecord{RunID: "run_after_restart_3", Status: "processing"}
	if attempt := executionAttemptIndex(restartedAgainTask, legacyRestartProcessing); attempt != 3 {
		t.Fatalf("expected fallback attempt inference to reach 3, got %d", attempt)
	}
}

func TestServiceTaskControlResumeConsumesHumanLoopPendingPayload(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Recovered after review.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_payload",
		Title:       "总结：Please summarize this after review",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: contextsvc.TaskContextSnapshot{
			Text:      "Please summarize this after review",
			InputType: "text",
			Trigger:   "hover_text_input",
		},
	})
	if _, ok := service.runEngine.EscalateHumanLoop(task.TaskID, map[string]any{
		"reason":           "doom_loop",
		"status":           "pending",
		"suggested_action": "review_and_replan",
	}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}
	result, err := service.TaskControl(map[string]any{
		"task_id": task.TaskID,
		"action":  "resume",
		"arguments": map[string]any{
			"review": map[string]any{
				"decision":    "approve",
				"reviewer_id": "reviewer_002",
				"notes":       "approved to continue",
			},
		},
	})
	if err != nil {
		t.Fatalf("resume task failed: %v", err)
	}
	if result["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected resumed task to complete after consuming escalation payload, got %+v", result)
	}
	record, ok := service.runEngine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected resumed task in runtime")
	}
	if record.PendingExecution != nil {
		t.Fatalf("expected orchestrator resume path to consume pending escalation payload, got %+v", record.PendingExecution)
	}
}

func TestServiceTaskControlResumeHumanLoopReplanReturnsToIntentConfirmation(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Recovered after review.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_replan",
		Title:       "总结：Please summarize this after review",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: contextsvc.TaskContextSnapshot{
			Text:      "Please summarize this after review",
			InputType: "text",
			Trigger:   "hover_text_input",
		},
	})
	if _, ok := service.runEngine.EscalateHumanLoop(task.TaskID, map[string]any{
		"reason":           "doom_loop",
		"status":           "pending",
		"suggested_action": "review_and_replan",
	}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}
	result, err := service.TaskControl(map[string]any{
		"task_id": task.TaskID,
		"action":  "resume",
		"arguments": map[string]any{
			"review": map[string]any{
				"decision":         "replan",
				"reviewer_id":      "reviewer_003",
				"notes":            "change the intent before continuing",
				"corrected_intent": map[string]any{"name": "translate", "arguments": map[string]any{"target_language": "en"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("resume with replan failed: %v", err)
	}
	updatedTask := result["task"].(map[string]any)
	if updatedTask["status"] != "confirming_intent" || updatedTask["current_step"] != "confirming_intent" {
		t.Fatalf("expected replan decision to return task to confirming_intent, got %+v", updatedTask)
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["type"] != "status" || !strings.Contains(bubble["text"].(string), "重新规划") {
		t.Fatalf("expected replan bubble to request new confirmation, got %+v", bubble)
	}
	record, ok := service.runEngine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected replanned task in runtime")
	}
	if record.PendingExecution != nil {
		t.Fatalf("expected replan path to clear pending escalation payload, got %+v", record.PendingExecution)
	}
	if stringValue(record.Intent, "name", "") != "translate" {
		t.Fatalf("expected corrected intent to be stored for replan, got %+v", record.Intent)
	}
}

func TestServiceTaskControlResumeHumanLoopReplanClearsAuthorizationBeforeReconfirm(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Recovered after review.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_replan_authorized",
		Title:       "写入：Please update the workspace file after review",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "write_file", "arguments": map[string]any{"target_path": "workspace/original.md"}},
		CurrentStep: "authorized_execution",
		RiskLevel:   "yellow",
		Snapshot: contextsvc.TaskContextSnapshot{
			Text:      "Please update the workspace file after review",
			InputType: "text",
			Trigger:   "hover_text_input",
		},
	})
	if _, ok := service.runEngine.ResolveAuthorization(task.TaskID, map[string]any{"decision": "allow_once"}, map[string]any{"files": []string{"workspace/original.md"}}); !ok {
		t.Fatal("expected authorization record to be stored before human review")
	}
	if _, ok := service.runEngine.EscalateHumanLoop(task.TaskID, map[string]any{
		"reason":           "doom_loop",
		"status":           "pending",
		"suggested_action": "review_and_replan",
	}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}

	result, err := service.TaskControl(map[string]any{
		"task_id": task.TaskID,
		"action":  "resume",
		"arguments": map[string]any{
			"review": map[string]any{
				"decision": "replan",
				"corrected_intent": map[string]any{
					"name": "write_file",
					"arguments": map[string]any{
						"target_path":           "workspace/replanned.md",
						"require_authorization": true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("resume with replan failed: %v", err)
	}
	if result["task"].(map[string]any)["status"] != "confirming_intent" {
		t.Fatalf("expected replan decision to return task to confirming_intent, got %+v", result["task"])
	}

	record, ok := service.runEngine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected replanned task in runtime")
	}
	if record.Authorization != nil || record.ImpactScope != nil {
		t.Fatalf("expected replan to clear prior authorization state, got %+v", record)
	}

	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   task.TaskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task after replan failed: %v", err)
	}
	if confirmResult["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected corrected intent to require fresh authorization, got %+v", confirmResult["task"])
	}

	record, ok = service.runEngine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after reconfirm")
	}
	if record.Authorization != nil {
		t.Fatalf("expected prior authorization record to stay cleared until fresh approval, got %+v", record.Authorization)
	}
	if len(record.ApprovalRequest) == 0 {
		t.Fatalf("expected reconfirmed task to create a new approval request, got %+v", record)
	}
}

func TestServiceTaskControlResumeHumanLoopRequiresReviewDecision(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Recovered after review.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_missing_review",
		Title:       "总结：Please summarize this after review",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	if _, ok := service.runEngine.EscalateHumanLoop(task.TaskID, map[string]any{
		"reason":           "doom_loop",
		"status":           "pending",
		"suggested_action": "review_and_replan",
	}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}
	_, err := service.TaskControl(map[string]any{"task_id": task.TaskID, "action": "resume"})
	if err == nil || !strings.Contains(err.Error(), "review decision is required") {
		t.Fatalf("expected missing review decision to block resume, got %v", err)
	}
	record, ok := service.runEngine.GetTask(task.TaskID)
	if !ok || record.Status != "blocked" || record.CurrentStep != "human_in_loop" {
		t.Fatalf("expected task to remain blocked in human review, got %+v", record)
	}
}

func TestServiceTaskControlResumeHumanLoopReplanRequiresCorrectedIntent(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Recovered after review.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_missing_replan_intent",
		Title:       "总结：Please summarize this after review",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	if _, ok := service.runEngine.EscalateHumanLoop(task.TaskID, map[string]any{
		"reason":           "doom_loop",
		"status":           "pending",
		"suggested_action": "review_and_replan",
	}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}
	_, err := service.TaskControl(map[string]any{
		"task_id": task.TaskID,
		"action":  "resume",
		"arguments": map[string]any{
			"review": map[string]any{
				"decision":    "replan",
				"reviewer_id": "reviewer_004",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "review.corrected_intent is required") {
		t.Fatalf("expected missing corrected_intent to block replan resume, got %v", err)
	}
}

func TestServiceTaskControlResumeHumanLoopIgnoresTopLevelReviewPayload(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Recovered after review.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_top_level_review",
		Title:       "总结：Please summarize this after review",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	if _, ok := service.runEngine.EscalateHumanLoop(task.TaskID, map[string]any{
		"reason":           "doom_loop",
		"status":           "pending",
		"suggested_action": "review_and_replan",
	}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}
	_, err := service.TaskControl(map[string]any{
		"task_id": task.TaskID,
		"action":  "resume",
		"review": map[string]any{
			"decision":    "approve",
			"reviewer_id": "reviewer_005",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "review decision is required") {
		t.Fatalf("expected top-level review payload to be ignored by stable contract, got %v", err)
	}
}

func TestServiceTaskControlResumePausedTaskDoesNotReexecute(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Should not rerun.")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_pause_resume",
		Title:       "总结：paused task",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	if _, err := service.TaskControl(map[string]any{"task_id": task.TaskID, "action": "pause"}); err != nil {
		t.Fatalf("pause task failed: %v", err)
	}
	result, err := service.TaskControl(map[string]any{"task_id": task.TaskID, "action": "resume"})
	if err != nil {
		t.Fatalf("resume task failed: %v", err)
	}
	updatedTask := result["task"].(map[string]any)
	if updatedTask["status"] != "processing" || updatedTask["current_step"] != "generate_output" {
		t.Fatalf("expected plain resume to restore processing without rerun, got %+v", updatedTask)
	}
	if result["bubble_message"].(map[string]any)["type"] != "status" {
		t.Fatalf("expected plain resume to keep status bubble, got %+v", result["bubble_message"])
	}
	record, ok := service.runEngine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected resumed task to remain in runtime")
	}
	if record.DeliveryResult != nil || record.FinishedAt != nil {
		t.Fatalf("expected paused resume not to implicitly rerun task, got %+v", record)
	}
	if record.PendingExecution != nil {
		t.Fatalf("expected paused resume not to create pending execution, got %+v", record.PendingExecution)
	}
}

func TestMaybeEscalateHumanLoopSkipsSideEffectingExecutionAttempt(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "unused")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_side_effect",
		Title:       "doom loop write task",
		SourceType:  "floating_ball",
		Status:      "processing",
		Intent:      map[string]any{"name": "write_file"},
		CurrentStep: "generate_output",
		RiskLevel:   "yellow",
	})
	capture, err := service.traceEval.Capture(traceeval.CaptureInput{
		TaskID:     task.TaskID,
		RunID:      task.RunID,
		IntentName: "write_file",
		Snapshot:   contextsvc.TaskContextSnapshot{Text: "keep rewriting"},
		ToolCalls: []tools.ToolCallRecord{
			{ToolName: "write_file", Input: map[string]any{"path": "workspace/out.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
			{ToolName: "write_file", Input: map[string]any{"path": "workspace/out.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
			{ToolName: "write_file", Input: map[string]any{"path": "workspace/out.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
		},
	})
	if err != nil {
		t.Fatalf("trace capture failed: %v", err)
	}
	escalated, bubble, ok := service.maybeEscalateHumanLoop(task, capture, execution.Result{
		ToolCalls: []tools.ToolCallRecord{{ToolName: "write_file", Input: map[string]any{"path": "workspace/out.md"}, Status: tools.ToolCallStatusSucceeded}},
	})
	if ok || bubble != nil || escalated.TaskID != "" {
		t.Fatalf("expected side-effecting attempt to skip human-loop escalation, got task=%+v bubble=%+v ok=%v", escalated, bubble, ok)
	}
	record, ok := service.runEngine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected original task to remain unchanged")
	}
	if record.Status != "processing" || record.CurrentStep != "generate_output" {
		t.Fatalf("expected side-effecting skip not to mutate runtime task, got %+v", record)
	}
}

func TestMaybeEscalateHumanLoopAllowsReadOnlyToolLoops(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "unused")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_hitl_read_only",
		Title:       "doom loop read task",
		SourceType:  "floating_ball",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "agent_loop",
		RiskLevel:   "yellow",
	})
	capture, err := service.traceEval.Capture(traceeval.CaptureInput{
		TaskID:     task.TaskID,
		RunID:      task.RunID,
		IntentName: "agent_loop",
		Snapshot:   contextsvc.TaskContextSnapshot{Text: "keep reading"},
		ToolCalls: []tools.ToolCallRecord{
			{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
			{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
			{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
		},
	})
	if err != nil {
		t.Fatalf("trace capture failed: %v", err)
	}
	escalated, bubble, ok := service.maybeEscalateHumanLoop(task, capture, execution.Result{
		ToolCalls: []tools.ToolCallRecord{{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)}},
	})
	if !ok {
		t.Fatal("expected read-only doom-loop attempt to keep human-loop escalation")
	}
	if escalated.Status != "blocked" || bubble == nil {
		t.Fatalf("expected blocked task with escalation bubble, got task=%+v bubble=%+v", escalated, bubble)
	}
	if plan, ok := service.runEngine.PendingExecutionPlan(task.TaskID); !ok || stringValue(plan, "kind", "") != "human_in_loop" {
		t.Fatalf("expected pending escalation plan for read-only loop, got %+v ok=%v", plan, ok)
	}
}

func TestCaptureExecutionTraceSurfacesRecordFailure(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "unused")
	service.traceEval = traceeval.NewService(storage.NewService(nil).TraceStore(), failingEvalSnapshotStore{err: errors.New("eval persistence failed")})
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_trace_fail",
		Title:       "trace persistence failure",
		SourceType:  "floating_ball",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	_, err := service.captureExecutionTrace(task, contextsvc.TaskContextSnapshot{Text: "capture me"}, task.Intent, execution.Result{Content: "done"}, nil)
	if err == nil || !strings.Contains(err.Error(), "eval persistence failed") {
		t.Fatalf("expected trace persistence failure to surface, got %v", err)
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

// TestServiceSubmitInputEmptyTextReturnsWaitingInput verifies that empty text
// submissions enter waiting_input.
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

// TestServiceDirectStartBuildsMemoryAndDeliveryHandoffs verifies direct starts
// attach memory and delivery handoffs.
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

// TestServiceStartTaskRespectsPreferredDelivery verifies direct starts preserve
// preferred and fallback delivery settings.
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

func TestServiceStartTaskPersistsFormalReadFileSampleChain(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "unused")
	readPath := filepath.Join(workspaceRoot, "notes", "source.txt")
	if err := os.MkdirAll(filepath.Dir(readPath), 0o755); err != nil {
		t.Fatalf("create notes dir: %v", err)
	}
	if err := os.WriteFile(readPath, []byte("hello from formal sample chain"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_read_file_sample",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请读取这个文件",
		},
		"intent": map[string]any{
			"name": "read_file",
			"arguments": map[string]any{
				"path": "notes/source.txt",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)

	toolCallsResult, err := service.TaskToolCallsList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task tool calls list failed: %v", err)
	}
	toolCalls := toolCallsResult["items"].([]map[string]any)
	if len(toolCalls) != 1 || toolCalls[0]["tool_name"] != "read_file" {
		t.Fatalf("expected one persisted read_file tool call, got %+v", toolCalls)
	}
	if mapValue(toolCalls[0], "input")["path"] != "notes/source.txt" {
		t.Fatalf("expected persisted read_file path, got %+v", toolCalls[0])
	}

	eventsResult, err := service.TaskEventsList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task events list failed: %v", err)
	}
	events := eventsResult["items"].([]map[string]any)
	if len(events) != 2 {
		t.Fatalf("expected tool_call.completed plus delivery.ready, got %+v", events)
	}
	foundToolCompleted := false
	foundDeliveryReady := false
	for _, event := range events {
		switch event["type"] {
		case "tool_call.completed":
			foundToolCompleted = true
		case "delivery.ready":
			foundDeliveryReady = true
		}
	}
	if !foundToolCompleted || !foundDeliveryReady {
		t.Fatalf("expected persisted read_file runtime events, got %+v", events)
	}

	deliveryRecord, ok, err := service.storage.LoopRuntimeStore().GetLatestDeliveryResult(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get latest delivery result failed: %v", err)
	}
	if !ok || deliveryRecord.Type != "bubble" || !strings.Contains(deliveryRecord.PreviewText, "hello from formal sample chain") {
		t.Fatalf("expected persisted direct delivery result, ok=%v record=%+v", ok, deliveryRecord)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	runtimeSummary := detailResult["runtime_summary"].(map[string]any)
	if runtimeSummary["events_count"] != 2 || runtimeSummary["latest_event_type"] != "delivery.ready" {
		t.Fatalf("expected task detail runtime summary to prefer formal event chain, got %+v", runtimeSummary)
	}
	deliveryResult := detailResult["delivery_result"].(map[string]any)
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected task detail to expose formal delivery_result, got %+v", deliveryResult)
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

// TestServiceConfirmCanEnterWaitingAuth verifies confirm flows can enter
// waiting_auth.
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

func TestServiceConfirmWaitingAuthPersistsApprovalRequestRecord(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "approval persistence output")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_approval_store",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "persist approval request before execution",
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

	items, total, err := service.storage.ApprovalRequestStore().ListApprovalRequests(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("list approval requests failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one persisted approval request, got total=%d items=%+v", total, items)
	}
	if items[0].TaskID != taskID || items[0].Status != "pending" {
		t.Fatalf("expected pending approval request for task %s, got %+v", taskID, items[0])
	}
	if items[0].OperationName != "write_file" {
		t.Fatalf("expected write_file approval request, got %+v", items[0])
	}
}

func TestServiceConfirmTaskReturnsStorageErrorWhenApprovalPersistenceFails(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "approval persistence failure output")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	originalStore := service.storage.ApprovalRequestStore()
	defer replaceApprovalRequestStore(t, service.storage, originalStore)
	replaceApprovalRequestStore(t, service.storage, failingApprovalRequestStore{base: originalStore, err: errors.New("approval store unavailable")})

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_approval_store_failure",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "persist approval request before execution",
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
	if err == nil || !errors.Is(err, ErrStorageQueryFailed) {
		t.Fatalf("expected ErrStorageQueryFailed from approval persistence, got %v", err)
	}
}

// TestServiceSecurityRespondAllowOnceResumesAndCompletes verifies allow-once
// resumes execution and completes delivery.
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

// TestServiceSecurityRespondRespectsFallbackDelivery verifies authorization
// resume honors fallback delivery resolution.
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

func TestServiceSecurityRespondPersistsAuthorizationRecord(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "authorization persistence output")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_authorization_store",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "persist authorization decision after approval",
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

	_, err = service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_auth_store",
		"decision":      "allow_once",
		"remember_rule": true,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	items, total, err := service.storage.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("list authorization records failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one persisted authorization record, got total=%d items=%+v", total, items)
	}
	if items[0].TaskID != taskID || items[0].Decision != "allow_once" || !items[0].RememberRule {
		t.Fatalf("unexpected authorization record: %+v", items[0])
	}
	approvalItems, approvalTotal, err := service.storage.ApprovalRequestStore().ListApprovalRequests(context.Background(), taskID, 20, 0)
	if err != nil || approvalTotal != 1 || len(approvalItems) != 1 {
		t.Fatalf("expected one approval request after authorization, got total=%d items=%+v err=%v", approvalTotal, approvalItems, err)
	}
	if approvalItems[0].Status != "approved" {
		t.Fatalf("expected resolved approval request to be marked approved, got %+v", approvalItems[0])
	}
}

func TestServiceSecurityRespondReturnsStorageErrorWhenAuthorizationPersistenceFails(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "authorization persistence failure output")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	originalStore := service.storage.AuthorizationRecordStore()
	defer replaceAuthorizationRecordStore(t, service.storage, originalStore)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_authorization_store_failure",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "persist authorization decision after approval",
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

	replaceAuthorizationRecordStore(t, service.storage, failingAuthorizationRecordStore{base: originalStore, err: errors.New("authorization store unavailable")})
	_, err = service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"decision":      "allow_once",
		"remember_rule": true,
	})
	if err == nil || !errors.Is(err, ErrStorageQueryFailed) {
		t.Fatalf("expected ErrStorageQueryFailed from authorization persistence, got %v", err)
	}
}

func TestServiceSecurityRespondRejectsOutOfPhaseAuthorizationPersistence(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "authorization out of phase output")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes", "output.md"), []byte("old content"), 0o644); err != nil {
		t.Fatalf("seed output file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_authorization_out_of_phase",
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
	if _, err := service.SecurityRespond(map[string]any{"task_id": taskID, "decision": "allow_once"}); err != nil {
		t.Fatalf("first security respond failed: %v", err)
	}
	if _, err := service.SecurityRespond(map[string]any{"task_id": taskID, "decision": "allow_once"}); !errors.Is(err, ErrTaskStatusInvalid) {
		t.Fatalf("expected repeated out-of-phase respond to return ErrTaskStatusInvalid, got %v", err)
	}

	items, total, err := service.storage.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("list authorization records failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected repeated out-of-phase respond to keep one persisted authorization record, got total=%d items=%+v", total, items)
	}
}

func TestServiceSecurityRespondKeepsAuthorizationHistoryAcrossMultipleCycles(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "history persistence output")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes", "output.md"), []byte("old content"), 0o644); err != nil {
		t.Fatalf("seed output file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_authorization_history",
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

	if _, err := service.SecurityRespond(map[string]any{"task_id": taskID, "approval_id": taskID, "decision": "allow_once"}); err != nil {
		t.Fatalf("security respond for initial write failed: %v", err)
	}
	pointsResult, err := service.SecurityRestorePointsList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security restore points list failed: %v", err)
	}
	points := pointsResult["items"].([]map[string]any)
	if len(points) == 0 {
		t.Fatal("expected restore point to exist for second approval cycle")
	}
	if _, err := service.SecurityRestoreApply(map[string]any{"task_id": taskID, "recovery_point_id": points[0]["recovery_point_id"]}); err != nil {
		t.Fatalf("security restore apply failed: %v", err)
	}
	if _, err := service.SecurityRespond(map[string]any{"task_id": taskID, "approval_id": "appr_restore_apply_history", "decision": "allow_once"}); err != nil {
		t.Fatalf("security respond for restore apply failed: %v", err)
	}

	items, total, err := service.storage.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("list authorization history failed: %v", err)
	}
	if total < 2 || len(items) < 2 {
		t.Fatalf("expected at least two authorization records, got total=%d items=%+v", total, items)
	}
	if items[0].AuthorizationRecordID == items[1].AuthorizationRecordID {
		t.Fatalf("expected unique authorization record ids across approval cycles, got %+v", items)
	}
	if items[0].ApprovalID == "" || items[1].ApprovalID == "" {
		t.Fatalf("expected authorization history to keep approval ids, got %+v", items)
	}
	approvalItems, approvalTotal, err := service.storage.ApprovalRequestStore().ListApprovalRequests(context.Background(), taskID, 20, 0)
	if err != nil || approvalTotal < 2 || len(approvalItems) < 2 {
		t.Fatalf("expected approval history for both cycles, got total=%d items=%+v err=%v", approvalTotal, approvalItems, err)
	}
	for _, item := range approvalItems {
		if item.Status == "pending" {
			t.Fatalf("expected all resolved approvals to be non-pending, got %+v", approvalItems)
		}
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

// TestServiceTaskListSupportsSortParams verifies task list sorting parameters.
func TestServiceTaskListSupportsSortParams(t *testing.T) {
	service := newTestService()
	baseTime := time.Date(2026, 4, 19, 9, 0, 0, 0, time.UTC)
	times := []time.Time{baseTime, baseTime.Add(10 * time.Millisecond)}
	index := 0
	replaceRuntimeClock(t, service.runEngine, func() time.Time {
		if index >= len(times) {
			return times[len(times)-1]
		}
		current := times[index]
		index++
		return current
	})

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

func TestServiceTaskListPrefersStructuredTaskStoreFallback(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored task list from structured store")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              "task_structured_001",
		SessionID:           "sess_structured",
		RunID:               "run_structured_001",
		Title:               "structured finished task",
		SourceType:          "hover_input",
		Status:              "completed",
		IntentName:          "summarize",
		IntentArgumentsJSON: `{"style":"key_points"}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "green",
		StartedAt:           time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 9, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 15, 9, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), "task_structured_001", []storage.TaskStepRecord{{
		StepID:        "step_structured_001",
		TaskID:        "task_structured_001",
		Name:          "deliver_result",
		Status:        "completed",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "structured output",
		CreatedAt:     time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 15, 9, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace structured task steps failed: %v", err)
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
	if len(items) != 1 || items[0]["task_id"] != "task_structured_001" {
		t.Fatalf("expected structured task store fallback item, got %+v", items)
	}
	intent := items[0]["intent"].(map[string]any)
	arguments := intent["arguments"].(map[string]any)
	if arguments["style"] != "key_points" {
		t.Fatalf("expected structured task list to preserve intent arguments, got %+v", intent)
	}
}

func TestServiceTaskListKeepsLegacyTaskRunsAlongsideStructuredTasks(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "merged storage task list")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_structured_merged",
		SessionID:   "sess_structured_merged",
		RunID:       "run_structured_merged",
		Title:       "legacy structured snapshot title",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 16, 9, 4, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 16, 9, 6, 0, 0, time.UTC)),
	}); err != nil {
		t.Fatalf("save structured compatibility task run failed: %v", err)
	}
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              "task_structured_merged",
		SessionID:           "sess_structured_merged",
		RunID:               "run_structured_merged",
		Title:               "structured finished task",
		SourceType:          "hover_input",
		Status:              "completed",
		IntentName:          "summarize",
		IntentArgumentsJSON: `{"style":"key_points"}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "green",
		StartedAt:           time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 16, 9, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 16, 9, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_legacy_only_001",
		SessionID:   "sess_legacy_only",
		RunID:       "run_legacy_only_001",
		Title:       "legacy finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 15, 9, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 15, 9, 6, 0, 0, time.UTC)),
	}); err != nil {
		t.Fatalf("save legacy-only task run failed: %v", err)
	}
	// Emulate an upgraded database where this older task still exists only in
	// task_runs because it predates the first-class tasks table rollout.
	if err := service.storage.TaskStore().DeleteTask(context.Background(), "task_legacy_only_001"); err != nil {
		t.Fatalf("delete structured legacy-only task failed: %v", err)
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
	if len(items) != 2 {
		t.Fatalf("expected merged structured and legacy task list items, got %+v", items)
	}
	itemsByID := map[string]map[string]any{}
	for _, item := range items {
		itemsByID[item["task_id"].(string)] = item
	}
	if itemsByID["task_structured_merged"]["title"] != "structured finished task" {
		t.Fatalf("expected structured task row to stay authoritative, got %+v", itemsByID["task_structured_merged"])
	}
	if itemsByID["task_legacy_only_001"]["title"] != "legacy finished task" {
		t.Fatalf("expected legacy-only task run to remain visible, got %+v", itemsByID["task_legacy_only_001"])
	}
	page := listResult["page"].(map[string]any)
	if page["total"] != 2 {
		t.Fatalf("expected merged storage total 2, got %+v", page)
	}
}

func TestServiceTaskListUsesStructuredStoreUnlimitedPaginationInMemoryFallback(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task list unlimited in-memory")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	for index := 0; index < 25; index++ {
		if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
			TaskID:              fmt.Sprintf("task_structured_%03d", index),
			SessionID:           "sess_structured",
			RunID:               fmt.Sprintf("run_structured_%03d", index),
			Title:               fmt.Sprintf("structured task %03d", index),
			SourceType:          "hover_input",
			Status:              "completed",
			IntentName:          "summarize",
			IntentArgumentsJSON: `{"style":"key_points"}`,
			PreferredDelivery:   "workspace_document",
			FallbackDelivery:    "bubble",
			CurrentStep:         "deliver_result",
			CurrentStepStatus:   "completed",
			RiskLevel:           "green",
			StartedAt:           time.Date(2026, 4, 15, 9, 0, index, 0, time.UTC).Format(time.RFC3339Nano),
			UpdatedAt:           time.Date(2026, 4, 15, 9, 5, index, 0, time.UTC).Format(time.RFC3339Nano),
			FinishedAt:          time.Date(2026, 4, 15, 9, 6, index, 0, time.UTC).Format(time.RFC3339Nano),
		}); err != nil {
			t.Fatalf("write structured task %d failed: %v", index, err)
		}
	}
	items, total, ok := service.listTasksFromStructuredStorage("finished", "updated_at", "desc", 0, 0)
	if !ok || total != 25 || len(items) != 25 {
		t.Fatalf("expected unlimited structured storage pagination to return all tasks, got ok=%v total=%d len=%d", ok, total, len(items))
	}
}

func TestServiceTaskListMergesStructuredStorageWhenOffsetExceedsRuntimePage(t *testing.T) {
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
	if page["total"] != 2 {
		t.Fatalf("expected structured and runtime totals to merge, got %+v", page)
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

func TestServiceTaskDetailGetIncludesRuntimeSummary(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "task detail runtime summary")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime store to be wired")
	}
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail_runtime_summary",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "task detail should expose runtime summary",
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
	task, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain available in runtime")
	}
	if _, ok := service.runEngine.RecordLoopLifecycle(taskID, "loop.failed", "tool_retry_exhausted", map[string]any{"stop_reason": "tool_retry_exhausted"}); !ok {
		t.Fatal("expected loop lifecycle update to succeed")
	}
	if err := service.storage.LoopRuntimeStore().SaveEvents(context.Background(), []storage.EventRecord{{
		EventID:     "evt_detail_runtime_001",
		RunID:       task.RunID,
		TaskID:      taskID,
		StepID:      fmt.Sprintf("%s_step_loop_01", task.RunID),
		Type:        "loop.failed",
		Level:       "error",
		PayloadJSON: `{"stop_reason":"tool_retry_exhausted"}`,
		CreatedAt:   "2026-04-18T11:00:00Z",
	}, {
		EventID:     "evt_detail_runtime_002",
		RunID:       "run_previous_attempt",
		TaskID:      taskID,
		StepID:      "run_previous_attempt_step_loop_01",
		Type:        "loop.round.completed",
		Level:       "info",
		PayloadJSON: `{"stop_reason":"completed"}`,
		CreatedAt:   "2026-04-18T10:59:00Z",
	}}); err != nil {
		t.Fatalf("save runtime events failed: %v", err)
	}
	if _, ok := service.runEngine.AppendSteeringMessage(taskID, "Also include a short summary section.", nil); !ok {
		t.Fatal("expected steering append to succeed")
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	runtimeSummary, ok := detailResult["runtime_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected runtime_summary payload, got %+v", detailResult["runtime_summary"])
	}
	if runtimeSummary["loop_stop_reason"] != "tool_retry_exhausted" {
		t.Fatalf("expected loop_stop_reason in runtime_summary, got %+v", runtimeSummary)
	}
	if runtimeSummary["events_count"] != 2 {
		t.Fatalf("expected task-level events_count 2, got %+v", runtimeSummary)
	}
	if runtimeSummary["latest_event_type"] != "loop.failed" {
		t.Fatalf("expected latest_event_type loop.failed, got %+v", runtimeSummary)
	}
	if runtimeSummary["active_steering_count"] != 1 {
		t.Fatalf("expected active_steering_count 1, got %+v", runtimeSummary)
	}
	if _, ok := runtimeSummary["latest_failure_code"]; !ok {
		t.Fatalf("expected runtime_summary latest_failure_code field, got %+v", runtimeSummary)
	}
	if _, ok := runtimeSummary["observation_signals"]; !ok {
		t.Fatalf("expected runtime_summary observation_signals field, got %+v", runtimeSummary)
	}
}

func TestServiceTaskDetailGetKeepsRuntimeSummaryScopedToRuntimeEvents(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "task detail runtime event scope")
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail_runtime_scope",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "task detail runtime summary should ignore non-runtime latest events",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if _, ok := service.runEngine.AppendSteeringMessage(taskID, "Please prioritize the action items.", nil); !ok {
		t.Fatal("expected steering append to succeed")
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	runtimeSummary, ok := detailResult["runtime_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected runtime_summary payload, got %+v", detailResult["runtime_summary"])
	}
	if runtimeSummary["events_count"] != 0 {
		t.Fatalf("expected task-level events_count 0, got %+v", runtimeSummary)
	}
	if runtimeSummary["latest_event_type"] != nil {
		t.Fatalf("expected latest_event_type to stay nil without runtime events, got %+v", runtimeSummary)
	}
	if runtimeSummary["active_steering_count"] != 1 {
		t.Fatalf("expected active_steering_count 1, got %+v", runtimeSummary)
	}
}

func TestServiceTaskDetailGetIncludesFailureSummaryForFailedScreenTask(t *testing.T) {
	service := newTestService()
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         "sess_screen_failure_detail",
		Title:             "查看当前屏幕：Build Dashboard",
		SourceType:        "screen_capture",
		Status:            "processing",
		Intent:            map[string]any{"name": "screen_analyze"},
		PreferredDelivery: "bubble",
		FallbackDelivery:  "bubble",
		CurrentStep:       "generate_output",
		RiskLevel:         "yellow",
		Timeline:          initialTimeline("processing", "generate_output"),
		Snapshot: contextsvc.TaskContextSnapshot{
			PageTitle:     "Build Dashboard",
			VisibleText:   "Fatal build error",
			ScreenSummary: "release validation failed",
		},
	})
	updatedTask, _ := service.failExecutionTask(task, task.Intent, execution.Result{
		Artifacts: []map[string]any{{
			"artifact_id":   "art_screen_failure_detail",
			"task_id":       task.TaskID,
			"artifact_type": "screen_capture",
			"title":         "build-dashboard.png",
			"path":          "workspace/build-dashboard.png",
			"mime_type":     "image/png",
		}},
		ToolOutput: map[string]any{
			"citation_seed": map[string]any{
				"artifact_id":       "art_screen_failure_detail",
				"artifact_type":     "screen_capture",
				"evidence_role":     "error_evidence",
				"ocr_excerpt":       "Fatal build error",
				"screen_session_id": "screen_session_failure_detail",
			},
		},
	}, tools.ErrOCRWorkerFailed)

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": updatedTask.TaskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	runtimeSummary := detailResult["runtime_summary"].(map[string]any)
	if runtimeSummary["latest_failure_code"] != "OCR_WORKER_FAILED" {
		t.Fatalf("expected failed screen task to expose formal latest_failure_code, got %+v", runtimeSummary)
	}
	if runtimeSummary["latest_failure_category"] != "screen_ocr" {
		t.Fatalf("expected failed screen task to expose latest_failure_category, got %+v", runtimeSummary)
	}
	if runtimeSummary["latest_failure_summary"] == nil {
		t.Fatalf("expected failed screen task to expose latest_failure_summary, got %+v", runtimeSummary)
	}
	observationSignals := runtimeSummary["observation_signals"].([]string)
	if !reflect.DeepEqual(observationSignals, []string{"screen_summary", "visible_text", "page_title"}) {
		t.Fatalf("expected failed screen task to expose stable observation signals, got %+v", runtimeSummary)
	}
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 1 || citations[0]["source_ref"] != "art_screen_failure_detail" {
		t.Fatalf("expected failed screen task to retain formal citations, got %+v", citations)
	}
}

func TestServiceTaskDetailGetIncludesFailureSummaryForFailedModelTask(t *testing.T) {
	service := newTestService()
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         "sess_model_failure_detail",
		Title:             "总结当前报错",
		SourceType:        "hover_input",
		Status:            "processing",
		Intent:            map[string]any{"name": "summarize", "arguments": map[string]any{}},
		PreferredDelivery: "bubble",
		FallbackDelivery:  "bubble",
		CurrentStep:       "generate_output",
		RiskLevel:         "green",
		Timeline:          initialTimeline("processing", "generate_output"),
		Snapshot: contextsvc.TaskContextSnapshot{
			Text:      "Provider failure should remain visible in runtime summary.",
			InputType: "text",
		},
	})
	updatedTask, _ := service.failExecutionTask(task, task.Intent, execution.Result{}, errors.Join(model.ErrSecretSourceFailed, model.ErrSecretNotFound))

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": updatedTask.TaskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	runtimeSummary := detailResult["runtime_summary"].(map[string]any)
	if runtimeSummary["latest_failure_code"] != "STRONGHOLD_ACCESS_FAILED" {
		t.Fatalf("expected failed model task to expose formal latest_failure_code, got %+v", runtimeSummary)
	}
	if runtimeSummary["latest_failure_category"] != "model_credentials" {
		t.Fatalf("expected failed model task to expose latest_failure_category, got %+v", runtimeSummary)
	}
	summary, _ := runtimeSummary["latest_failure_summary"].(string)
	if !strings.Contains(summary, "模型凭证") {
		t.Fatalf("expected failed model task to expose credentials-focused summary, got %+v", runtimeSummary)
	}
}

func TestClassifyModelFailureUsesFormalCodes(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		wantCode string
		wantType string
	}{
		{
			name:     "unsupported provider",
			err:      model.ErrModelProviderUnsupported,
			wantCode: "MODEL_PROVIDER_NOT_FOUND",
			wantType: "model_provider",
		},
		{
			name:     "missing provider config",
			err:      model.ErrModelProviderRequired,
			wantCode: "MODEL_PROVIDER_NOT_FOUND",
			wantType: "model_provider",
		},
		{
			name:     "tool calling unsupported",
			err:      model.ErrToolCallingNotSupported,
			wantCode: "MODEL_NOT_ALLOWED",
			wantType: "model_capability",
		},
		{
			name:     "endpoint missing",
			err:      model.ErrOpenAIEndpointRequired,
			wantCode: "MODEL_NOT_ALLOWED",
			wantType: "model_configuration",
		},
		{
			name:     "api key missing",
			err:      model.ErrOpenAIAPIKeyRequired,
			wantCode: "STRONGHOLD_ACCESS_FAILED",
			wantType: "model_credentials",
		},
		{
			name:     "provider timeout",
			err:      model.ErrOpenAIRequestTimeout,
			wantCode: "MODEL_RUNTIME_UNAVAILABLE",
			wantType: "model_runtime",
		},
		{
			name:     "provider rate limited",
			err:      &model.OpenAIHTTPStatusError{StatusCode: 429, Message: "rate limited"},
			wantCode: "MODEL_RUNTIME_UNAVAILABLE",
			wantType: "model_runtime",
		},
		{
			name:     "provider rejects request",
			err:      &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "bad request"},
			wantCode: "MODEL_NOT_ALLOWED",
			wantType: "model_configuration",
		},
		{
			name:     "output invalid",
			err:      tools.ErrToolOutputInvalid,
			wantCode: "TOOL_OUTPUT_INVALID",
			wantType: "model_output",
		},
		{
			name:     "secret resolution unavailable",
			err:      errors.Join(model.ErrSecretSourceFailed, model.ErrSecretNotFound),
			wantCode: "STRONGHOLD_ACCESS_FAILED",
			wantType: "model_credentials",
		},
		{
			name:     "missing client",
			err:      model.ErrClientNotConfigured,
			wantCode: "STRONGHOLD_ACCESS_FAILED",
			wantType: "model_credentials",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			failureCode, failureCategory := classifyModelFailure(testCase.err)
			if failureCode != testCase.wantCode || failureCategory != testCase.wantType {
				t.Fatalf("expected %s/%s, got %s/%s", testCase.wantCode, testCase.wantType, failureCode, failureCategory)
			}
		})
	}
}

func TestBuildBudgetFailureAuditTracksFormalModelFailures(t *testing.T) {
	service := newTestService()
	auditRecord := service.buildBudgetFailureAudit(runengine.TaskRecord{TaskID: "task_budget_failure_audit", RunID: "run_budget_failure_audit"}, errors.Join(model.ErrOpenAIEndpointRequired, model.ErrOpenAIRequestTimeout))
	if auditRecord == nil {
		t.Fatal("expected formal model failure to produce budget failure audit")
	}
	if stringValue(auditRecord, "action", "") != "budget_auto_downgrade.failure_signal" {
		t.Fatalf("expected budget failure audit action, got %+v", auditRecord)
	}
	if !strings.Contains(stringValue(auditRecord, "reason", ""), model.ErrOpenAIEndpointRequired.Error()) {
		t.Fatalf("expected budget failure audit reason to retain model config detail, got %+v", auditRecord)
	}
}

func TestLatestTaskFailurePrefersStructuredFailureMetadataOverBudgetSignals(t *testing.T) {
	task := runengine.TaskRecord{
		TaskID: "task_failure_signal_priority",
		Status: "failed",
		RunID:  "run_failure_signal_priority",
		AuditRecords: []map[string]any{{
			"type":    "execution",
			"action":  "execute_task",
			"result":  "failed",
			"summary": "OCR worker failed while analyzing the current screen.",
			"metadata": map[string]any{
				"failure_code":     "OCR_WORKER_FAILED",
				"failure_category": "screen_ocr",
			},
		}, {
			"category": "budget_auto_downgrade",
			"action":   "budget_auto_downgrade.failure_signal",
			"result":   "failed",
			"reason":   model.ErrClientNotConfigured.Error(),
		}},
	}

	failureCode, failureCategory, failureSummary := latestTaskFailure(task)
	if failureCode != "OCR_WORKER_FAILED" {
		t.Fatalf("expected latestTaskFailure to keep structured failure_code, got %q", failureCode)
	}
	if failureCategory != "screen_ocr" {
		t.Fatalf("expected latestTaskFailure to keep structured failure_category, got %q", failureCategory)
	}
	if !strings.Contains(failureSummary, "OCR worker failed") {
		t.Fatalf("expected latestTaskFailure to keep structured failure summary, got %q", failureSummary)
	}
}

func TestBuildTaskCitationsPreservesDistinctFormalReferencesForSameArtifact(t *testing.T) {
	task := runengine.TaskRecord{
		TaskID: "task_screen_multi_citation",
		RunID:  "run_screen_multi_citation",
	}
	artifacts := []map[string]any{{
		"artifact_id":   "art_screen_multi_citation",
		"task_id":       task.TaskID,
		"artifact_type": "screen_capture",
		"title":         "screen.png",
		"path":          "workspace/screen.png",
		"mime_type":     "image/png",
	}}
	toolCalls := []tools.ToolCallRecord{
		{
			Output: map[string]any{
				"citation_seed": map[string]any{
					"artifact_id":   "art_screen_multi_citation",
					"artifact_type": "screen_capture",
					"evidence_role": "error_evidence",
					"ocr_excerpt":   "Fatal build error",
				},
			},
		},
		{
			Output: map[string]any{
				"citation_seed": map[string]any{
					"artifact_id":   "art_screen_multi_citation",
					"artifact_type": "screen_capture",
					"evidence_role": "page_context",
					"ocr_excerpt":   "Release dashboard",
				},
			},
		},
		{
			Output: map[string]any{
				"citation_seed": map[string]any{
					"artifact_id":   "art_screen_multi_citation",
					"artifact_type": "screen_capture",
					"evidence_role": "error_evidence",
					"ocr_excerpt":   "Fatal build error",
				},
			},
		},
	}

	citations := buildTaskCitations(task, toolCalls, nil, nil, artifacts)
	if len(citations) != 2 {
		t.Fatalf("expected exact duplicate seeds to collapse but distinct formal references to remain, got %+v", citations)
	}
	if citations[0]["source_ref"] != "art_screen_multi_citation" || citations[1]["source_ref"] != "art_screen_multi_citation" {
		t.Fatalf("expected both citations to reference the same artifact, got %+v", citations)
	}
	if citations[0]["citation_id"] == citations[1]["citation_id"] {
		t.Fatalf("expected distinct formal references on the same artifact to keep unique citation ids, got %+v", citations)
	}
	if citations[0]["artifact_id"] != "art_screen_multi_citation" || citations[1]["artifact_type"] != "screen_capture" {
		t.Fatalf("expected structured citation metadata to survive deduping, got %+v", citations)
	}
	if citations[0]["excerpt_text"] == citations[1]["excerpt_text"] {
		t.Fatalf("expected distinct citations to preserve their excerpt text, got %+v", citations)
	}
}

func TestServiceAttachFormalCitationsPersistsFirstClassCitationFallback(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "persist formal citations")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime storage to be wired")
	}
	task := runengine.TaskRecord{
		TaskID: "task_persist_citations",
		RunID:  "run_persist_citations",
	}
	artifacts := []map[string]any{{
		"artifact_id":   "art_persist_citations",
		"task_id":       task.TaskID,
		"artifact_type": "screen_capture",
		"title":         "persist-screen.png",
		"path":          "workspace/persist-screen.png",
		"mime_type":     "image/png",
	}}
	toolCalls := []tools.ToolCallRecord{{
		Output: map[string]any{
			"citation_seed": map[string]any{
				"artifact_id":       "art_persist_citations",
				"artifact_type":     "screen_capture",
				"evidence_role":     "error_evidence",
				"ocr_excerpt":       "Fatal build error",
				"screen_session_id": "screen_sess_persist",
			},
		},
	}}

	service.attachFormalCitations(task, task, toolCalls, nil, map[string]any{
		"payload": map[string]any{"task_id": task.TaskID},
	}, artifacts)

	citations, err := service.storage.LoopRuntimeStore().ListTaskCitations(context.Background(), task.TaskID)
	if err != nil {
		t.Fatalf("list first-class citations failed: %v", err)
	}
	if len(citations) != 1 {
		t.Fatalf("expected one persisted citation, got %+v", citations)
	}
	if citations[0].ArtifactID != "art_persist_citations" || citations[0].EvidenceRole != "error_evidence" {
		t.Fatalf("expected persisted citation metadata to survive, got %+v", citations[0])
	}
	if citations[0].ScreenSessionID != "screen_sess_persist" || citations[0].ExcerptText != "Fatal build error" {
		t.Fatalf("expected persisted citation evidence fields to survive, got %+v", citations[0])
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
	originalStore := service.storage.TaskStore()
	defer func() {
		replaceTaskStore(t, service.storage, originalStore)
		if service.storage != nil {
			_ = service.storage.Close()
		}
	}()

	countingStore := &countingTaskStore{base: service.storage.TaskStore()}
	replaceTaskStore(t, service.storage, countingStore)

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

	if err := countingStore.WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              "task_dashboard_count",
		SessionID:           "sess_overview_count",
		RunID:               "run_dashboard_count",
		Title:               "stored task for overview count",
		SourceType:          "hover_input",
		Status:              "completed",
		IntentName:          "summarize",
		IntentArgumentsJSON: `{"style":"key_points"}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "green",
		StartedAt:           time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 14, 12, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 14, 12, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}

	if _, err := service.DashboardOverviewGet(map[string]any{}); err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	if countingStore.listCalls != 1 {
		t.Fatalf("expected dashboard overview to load structured tasks once per request, got %d", countingStore.listCalls)
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

func TestServiceStartTaskHandlesControlledScreenAnalyzeIntent(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_local_0001/frame_0001.png", Text: "fatal build error", Language: "eng", Source: "ocr_worker_text"}}
	service, workspaceRoot := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "inputs"), 0o755); err != nil {
		t.Fatalf("mkdir inputs failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "inputs", "screen.png"), []byte("fake screen capture"), 0o644); err != nil {
		t.Fatalf("write screen input failed: %v", err)
	}
	result, err := service.StartTask(map[string]any{
		"session_id": "sess_screen_task",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请分析屏幕中的错误",
		},
		"intent": map[string]any{
			"name": "screen_analyze",
			"arguments": map[string]any{
				"path": "inputs/screen.png",
			},
		},
	})
	if err != nil {
		t.Fatalf("start screen analyze task failed: %v", err)
	}
	task := result["task"].(map[string]any)
	if task["source_type"] != "screen_capture" {
		t.Fatalf("expected screen source_type, got %+v", task)
	}
	if task["status"] != "waiting_auth" {
		t.Fatalf("expected screen analyze task to require authorization first, got %+v", task)
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["type"] != "status" {
		t.Fatalf("expected waiting authorization status bubble, got %+v", bubble)
	}
	approvalRequests, total := service.runEngine.PendingApprovalRequests(20, 0)
	if total != 1 || len(approvalRequests) != 1 {
		t.Fatalf("expected one pending approval request, got total=%d items=%+v", total, approvalRequests)
	}
	record, exists := service.runEngine.GetTask(task["task_id"].(string))
	if !exists || record.Status != "waiting_auth" {
		t.Fatalf("expected runtime screen task to wait for auth, got %+v", record)
	}
	if record.ApprovalRequest == nil || record.PendingExecution == nil {
		t.Fatalf("expected approval request and pending execution, got %+v", record)
	}
	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":  task["task_id"],
		"decision": "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond allow_once failed: %v", err)
	}
	respondTask := respondResult["task"].(map[string]any)
	if respondTask["status"] != "completed" {
		t.Fatalf("expected authorized screen task to complete, got %+v", respondTask)
	}
	record, exists = service.runEngine.GetTask(task["task_id"].(string))
	if !exists || record.Status != "completed" {
		t.Fatalf("expected controlled screen task to complete, got %+v", record)
	}
	if len(record.Artifacts) != 1 || record.Artifacts[0]["artifact_type"] != "screen_capture" {
		t.Fatalf("expected one screen artifact in runtime task, got %+v", record.Artifacts)
	}
	if record.Authorization == nil || record.Authorization["decision"] != "allow_once" {
		t.Fatalf("expected authorization record to be stored, got %+v", record.Authorization)
	}
	artifacts, total, err := service.storage.ArtifactStore().ListArtifacts(context.Background(), task["task_id"].(string), 20, 0)
	if err != nil {
		t.Fatalf("list persisted artifacts failed: %v", err)
	}
	if total != 1 || len(artifacts) != 1 {
		t.Fatalf("expected one persisted screen artifact, total=%d len=%d", total, len(artifacts))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(artifacts[0].DeliveryPayloadJSON), &payload); err != nil {
		t.Fatalf("decode persisted screen payload failed: %v", err)
	}
	if payload["screen_session_id"] == "" || payload["capture_mode"] != "screenshot" || payload["retention_policy"] == "" || payload["evidence_role"] != "error_evidence" {
		t.Fatalf("expected persisted artifact payload to retain screen metadata, got %+v", payload)
	}
	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": task["task_id"]})
	if err != nil {
		t.Fatalf("task detail get for screen task failed: %v", err)
	}
	authorizationRecord, ok := detailResult["authorization_record"].(map[string]any)
	if !ok || authorizationRecord["decision"] != "allow_once" {
		t.Fatalf("expected task detail to expose latest authorization_record, got %+v", detailResult["authorization_record"])
	}
	auditRecord, ok := detailResult["audit_record"].(map[string]any)
	if !ok || auditRecord["task_id"] != task["task_id"] {
		t.Fatalf("expected task detail to expose latest audit_record, got %+v", detailResult["audit_record"])
	}
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 1 {
		t.Fatalf("expected one formal citation for screen task, got %+v", citations)
	}
	if citations[0]["source_type"] != "file" || !strings.Contains(stringValue(citations[0], "label", ""), "error_evidence") {
		t.Fatalf("expected citation to preserve artifact-backed screen evidence metadata, got %+v", citations[0])
	}
	if citations[0]["artifact_type"] != "screen_capture" || citations[0]["evidence_role"] != "error_evidence" {
		t.Fatalf("expected citation to expose structured screen evidence role and artifact type, got %+v", citations[0])
	}
	if citations[0]["excerpt_text"] == nil || citations[0]["screen_session_id"] == nil {
		t.Fatalf("expected citation to expose OCR excerpt and screen session metadata, got %+v", citations[0])
	}
	deliveryResult, ok := detailResult["delivery_result"].(map[string]any)
	if !ok || stringValue(deliveryResult, "preview_text", "") == "" {
		t.Fatalf("expected task detail to expose formal delivery_result, got %+v", detailResult["delivery_result"])
	}
	record, exists = service.runEngine.GetTask(task["task_id"].(string))
	if !exists || len(record.Citations) != 1 {
		t.Fatalf("expected runtime task to retain one formal citation, got %+v", record)
	}
}

func TestServiceStartTaskInfersScreenAnalyzeFromVisualErrorRequest(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_local_0001/frame_0001.png", Text: "fatal build error", Language: "eng", Source: "ocr_worker_text"}}
	service, _ := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_screen_infer_start",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "帮我看看这个页面的报错",
			"page_context": map[string]any{
				"title":        "Build Dashboard",
				"url":          "https://example.com/build",
				"app_name":     "Chrome",
				"window_title": "Browser - Build Dashboard",
				"visible_text": "Fatal build error: missing release asset",
			},
		},
		"context": map[string]any{
			"screen_summary": "release validation failed on current screen",
		},
	})
	if err != nil {
		t.Fatalf("start inferred screen analyze task failed: %v", err)
	}
	task := result["task"].(map[string]any)
	if task["source_type"] != "screen_capture" {
		t.Fatalf("expected screen_capture source type, got %+v", task)
	}
	if task["status"] != "waiting_auth" {
		t.Fatalf("expected inferred screen analyze task to wait for auth, got %+v", task)
	}
	intentValue := task["intent"].(map[string]any)
	if intentValue["name"] != "screen_analyze" {
		t.Fatalf("expected screen_analyze intent, got %+v", intentValue)
	}
	arguments := intentValue["arguments"].(map[string]any)
	if arguments["evidence_role"] != "error_evidence" || arguments["page_title"] != "Build Dashboard" {
		t.Fatalf("expected inferred visual arguments to be preserved, got %+v", arguments)
	}
	record, exists := service.runEngine.GetTask(task["task_id"].(string))
	if !exists || record.PendingExecution == nil {
		t.Fatalf("expected runtime task to keep pending execution for inferred screen task, got %+v", record)
	}
	if stringValue(record.PendingExecution, "source_path", "") != "" {
		t.Fatalf("expected inferred screen task to authorize current screen instead of an existing file, got %+v", record.PendingExecution)
	}
	if stringValue(record.PendingExecution, "target_object", "") != "Build Dashboard" {
		t.Fatalf("expected inferred screen target to use page context, got %+v", record.PendingExecution)
	}
}

func TestServiceStartTaskInfersScreenAnalyzeTitleFromScreenSummaryWhenTitlesAreMissing(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_local_0001/frame_0001.png", Text: "fatal build error", Language: "eng", Source: "ocr_worker_text"}}
	service, _ := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_screen_summary_title",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "看看当前屏幕上哪里出错了",
		},
		"context": map[string]any{
			"screen": map[string]any{
				"summary":      "release validation failed before publish",
				"visible_text": "fatal build error",
			},
		},
	})
	if err != nil {
		t.Fatalf("start inferred screen task failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if !strings.Contains(stringValue(task, "title", ""), "release validation") {
		t.Fatalf("expected screen summary to drive inferred task title, got %+v", task)
	}
	intentValue := task["intent"].(map[string]any)
	arguments := intentValue["arguments"].(map[string]any)
	if arguments["screen_summary"] != "release validation failed before publish" || arguments["visible_text"] != "fatal build error" {
		t.Fatalf("expected inferred intent to preserve screen summary and visible text, got %+v", arguments)
	}
}

func TestServiceSubmitInputInfersScreenAnalyzeFromVisualErrorRequest(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_local_0001/frame_0001.png", Text: "fatal build error", Language: "eng", Source: "ocr_worker_text"}}
	service, _ := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_screen_infer_submit",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type":       "text",
			"text":       "看看当前屏幕上的报错",
			"input_mode": "text",
		},
		"context": map[string]any{
			"page": map[string]any{
				"title":        "Release Checklist",
				"url":          "https://example.com/release",
				"app_name":     "Chrome",
				"window_title": "Browser - Release Checklist",
				"visible_text": "Warning: release notes are incomplete.",
			},
			"screen_summary": "release checklist shows blocking warning",
		},
	})
	if err != nil {
		t.Fatalf("submit inferred screen analyze task failed: %v", err)
	}
	task := result["task"].(map[string]any)
	if task["status"] != "waiting_auth" || task["source_type"] != "screen_capture" {
		t.Fatalf("expected submit input to route into waiting screen authorization, got %+v", task)
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["type"] != "status" {
		t.Fatalf("expected waiting authorization status bubble, got %+v", bubble)
	}
}

func TestServiceStartTaskFallsBackWhenScreenCapabilityUnavailable(t *testing.T) {
	service := newTestService()

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_screen_capability_fallback",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "帮我看看这个页面的报错",
			"page_context": map[string]any{
				"title":        "Build Dashboard",
				"window_title": "Browser - Build Dashboard",
				"visible_text": "Fatal build error: missing release asset",
			},
		},
		"context": map[string]any{
			"screen_summary": "release validation failed on current screen",
		},
	})
	if err != nil {
		t.Fatalf("start task with unavailable screen capability failed: %v", err)
	}
	task := result["task"].(map[string]any)
	if task["source_type"] == "screen_capture" {
		t.Fatalf("expected unavailable screen capability to avoid screen_capture task, got %+v", task)
	}
	intentValue := task["intent"].(map[string]any)
	if intentValue["name"] != "agent_loop" {
		t.Fatalf("expected fallback to agent_loop, got %+v", intentValue)
	}
	if task["status"] != "completed" {
		t.Fatalf("expected fallback task to continue through normal flow, got %+v", task)
	}
	if task["current_step"] == "waiting_authorization" {
		t.Fatalf("expected fallback task to avoid visual waiting_auth flow, got %+v", task)
	}
}

func TestServiceSubmitInputFallbackKeepsExplicitConfirmationWhenScreenCapabilityUnavailable(t *testing.T) {
	service := newTestService()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_screen_capability_confirm_fallback",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type":       "text",
			"text":       "帮我看看这个页面的报错",
			"input_mode": "text",
		},
		"context": map[string]any{
			"page": map[string]any{
				"title":        "Build Dashboard",
				"window_title": "Browser - Build Dashboard",
				"visible_text": "Fatal build error: missing release asset",
			},
			"screen_summary": "release validation failed on current screen",
		},
		"options": map[string]any{
			"confirm_required": true,
		},
	})
	if err != nil {
		t.Fatalf("submit input with unavailable screen capability failed: %v", err)
	}
	task := result["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected fallback task to preserve confirming_intent, got %+v", task)
	}
	if task["current_step"] != "intent_confirmation" {
		t.Fatalf("expected fallback task to wait for confirmation, got %+v", task)
	}
	intentValue := task["intent"].(map[string]any)
	if intentValue["name"] != "agent_loop" {
		t.Fatalf("expected unavailable screen capability to downgrade into agent_loop, got %+v", intentValue)
	}
	if result["delivery_result"] != nil {
		t.Fatalf("expected confirming fallback to skip direct delivery, got %+v", result["delivery_result"])
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["type"] != "intent_confirm" {
		t.Fatalf("expected confirmation bubble for downgraded task, got %+v", bubble)
	}
}

func TestSecurityRespondScreenAnalyzeFailureReconcilesTaskState(t *testing.T) {
	ocrStub := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "temp/screen_local_0001/frame_0001.png", Text: "fatal build error", Language: "eng", Source: "ocr_worker_text"}}
	service, _ := newTestServiceWithExecutionWorkers(t, "unused", platform.LocalExecutionBackend{}, nil, sidecarclient.NewNoopPlaywrightSidecarClient(), ocrStub, sidecarclient.NewNoopMediaWorkerClient())
	result, err := service.StartTask(map[string]any{
		"session_id": "sess_screen_fail",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请分析屏幕中的错误",
		},
		"intent": map[string]any{
			"name": "screen_analyze",
			"arguments": map[string]any{
				"path": "inputs/missing-screen.png",
			},
		},
	})
	if err != nil {
		t.Fatalf("start screen analyze task failed: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":  taskID,
		"decision": "allow_once",
	})
	if err != nil {
		t.Fatalf("security respond allow_once failed: %v", err)
	}
	respondTask := respondResult["task"].(map[string]any)
	if respondTask["status"] != "failed" {
		t.Fatalf("expected failed task after approved screen capture error, got %+v", respondTask)
	}
	bubble := respondResult["bubble_message"].(map[string]any)
	if bubble["type"] != "status" {
		t.Fatalf("expected failure status bubble, got %+v", bubble)
	}
	record, exists := service.runEngine.GetTask(taskID)
	if !exists || record.Status != "failed" || record.PendingExecution != nil {
		t.Fatalf("expected runtime task to reconcile to failed terminal state, got %+v", record)
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

func TestServiceBudgetAutoDowngradeSwitchesWorkspaceDeliveryToBubble(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed downgrade output")
	runtimeEvents := make([]map[string]any, 0)
	unsubscribe := service.SubscribeRuntimeNotifications(func(taskID, method string, params map[string]any) {
		runtimeEvents = append(runtimeEvents, map[string]any{
			"task_id": taskID,
			"method":  method,
			"params":  cloneMap(params),
		})
	})
	defer unsubscribe()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_budget_downgrade",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": strings.Repeat("long task content ", 20),
		},
		"options": map[string]any{
			"confirm_required":   false,
			"preferred_delivery": "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected downgraded task to remain in runtime")
	}
	_, ok = service.runEngine.AppendAuditData(taskID, nil, map[string]any{"total_tokens": 96, "estimated_cost": 0.12})
	if !ok {
		t.Fatal("expected token usage update to succeed")
	}
	record, ok = service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected downgraded task to remain in runtime after token usage update")
	}
	updatedTask, bubble, deliveryResult, _, err := service.executeTask(record, snapshotFromTask(record), record.Intent)
	if err != nil {
		t.Fatalf("executeTask failed after token usage pressure: %v", err)
	}
	if bubble == nil || bubble["type"] != "result" {
		t.Fatalf("expected downgraded execution to keep a result bubble, got %+v", bubble)
	}

	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected budget downgrade to switch delivery to bubble, got %+v", deliveryResult)
	}
	record, ok = service.runEngine.GetTask(updatedTask.TaskID)
	if !ok {
		t.Fatal("expected downgraded task to remain in runtime")
	}
	if record.DeliveryResult["type"] != "bubble" {
		t.Fatalf("expected runtime delivery result to use downgraded bubble delivery, got %+v", record.DeliveryResult)
	}
	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatalf("expected budget downgrade notifications to remain buffered")
	}
	foundDowngradeEvent := false
	for _, notification := range notifications {
		if notification.Method != "budget.downgrade.applied" {
			continue
		}
		eventPayload := mapValue(mapValue(notification.Params, "event"), "payload")
		if eventPayload["trigger_reason"] != "budget_pressure" {
			t.Fatalf("expected budget downgrade payload to explain budget pressure, got %+v", notification.Params)
		}
		foundDowngradeEvent = true
		break
	}
	if !foundDowngradeEvent {
		t.Fatalf("expected one budget.downgrade.applied notification, got %+v", notifications)
	}
	foundLiveRuntimeEvent := false
	for _, event := range runtimeEvents {
		if event["method"] != "budget.downgrade.applied" {
			continue
		}
		foundLiveRuntimeEvent = true
		break
	}
	if !foundLiveRuntimeEvent {
		t.Fatalf("expected live runtime subscribers to receive budget.downgrade.applied, got %+v", runtimeEvents)
	}
	if len(record.AuditRecords) == 0 || stringValue(record.AuditRecords[len(record.AuditRecords)-1], "action", "") != "budget_auto_downgrade.applied" {
		t.Fatalf("expected budget downgrade audit record, got %+v", record.AuditRecords)
	}
	if record.SecuritySummary["budget_auto_downgrade_applied"] != true {
		t.Fatalf("expected completed task security summary to retain downgrade marker, got %+v", record.SecuritySummary)
	}
	if record.SecuritySummary["budget_auto_downgrade_reason"] != "budget_pressure" {
		t.Fatalf("expected completed task security summary to retain downgrade reason, got %+v", record.SecuritySummary)
	}
	if stringValue(mapValue(record.SecuritySummary, "budget_auto_downgrade_trace"), "trigger_reason", "") != "budget_pressure" {
		t.Fatalf("expected completed task security summary to retain downgrade trace, got %+v", record.SecuritySummary)
	}
}

func TestServiceBudgetAutoDowngradeProviderUnavailableDisablesExpensiveTools(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed provider unavailable")
	if _, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider":              "unsupported_provider",
			"budget_auto_downgrade": true,
		},
	}); err != nil {
		t.Fatalf("settings update failed: %v", err)
	}

	task := runengine.TaskRecord{
		TaskID:            "task_budget_provider_unavailable",
		SessionID:         "sess_budget_provider_unavailable",
		RunID:             "run_budget_provider_unavailable",
		Title:             "provider unavailable task",
		SourceType:        "hover_input",
		Status:            "processing",
		Intent:            map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		PreferredDelivery: "workspace_document",
		FallbackDelivery:  "bubble",
		CurrentStep:       "generate_output",
		RiskLevel:         "green",
		StartedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	decision := service.evaluateBudgetAutoDowngrade(task, task.Intent)
	if !decision.Applied || decision.TriggerReason != "provider_unavailable" {
		t.Fatalf("expected provider unavailable downgrade decision, got %+v", decision)
	}
	updatedTask, updatedSnapshot, updatedIntent := service.applyBudgetAutoDowngrade(task, contextsvc.TaskContextSnapshot{Text: strings.Repeat("provider unavailable ", 20)}, task.Intent, decision)
	if updatedTask.PreferredDelivery != "bubble" || updatedTask.FallbackDelivery != "bubble" {
		t.Fatalf("expected provider unavailable downgrade to force bubble delivery, got task=%+v", updatedTask)
	}
	if mapValue(updatedIntent, "arguments")["disable_tool_calls"] != true {
		t.Fatalf("expected provider unavailable downgrade to disable expensive tool calls, got %+v", updatedIntent)
	}
	if len(updatedSnapshot.Text) == 0 {
		t.Fatalf("expected snapshot text to survive downgrade mutation, got %+v", updatedSnapshot)
	}
}

func TestServiceBudgetAutoDowngradeDisabledKeepsWorkspaceDelivery(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed no downgrade")
	if _, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"budget_auto_downgrade": false,
		},
	}); err != nil {
		t.Fatalf("settings update failed: %v", err)
	}

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_budget_disabled",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": strings.Repeat("long task content ", 20),
		},
		"options": map[string]any{
			"confirm_required":   false,
			"preferred_delivery": "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}
	deliveryResult := result["delivery_result"].(map[string]any)
	if deliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected disabled budget downgrade to preserve workspace_document delivery, got %+v", deliveryResult)
	}
}

func TestServiceBudgetAutoDowngradeUsesFailurePressureSignal(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed failure pressure")
	task := runengine.TaskRecord{
		TaskID:            "task_budget_failure_pressure",
		SessionID:         "sess_budget_failure_pressure",
		RunID:             "run_budget_failure_pressure",
		Title:             "failure pressure task",
		SourceType:        "hover_input",
		Status:            "processing",
		Intent:            map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		PreferredDelivery: "workspace_document",
		FallbackDelivery:  "bubble",
		CurrentStep:       "generate_output",
		RiskLevel:         "green",
		StartedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		AuditRecords: []map[string]any{{
			"category": "budget_auto_downgrade",
			"action":   "budget_auto_downgrade.failure_signal",
			"result":   "failed",
			"reason":   model.ErrClientNotConfigured.Error(),
		}, {
			"category": "budget_auto_downgrade",
			"action":   "budget_auto_downgrade.failure_signal",
			"result":   "failed",
			"reason":   model.ErrModelProviderUnsupported.Error(),
		}},
	}
	decision := service.evaluateBudgetAutoDowngrade(task, task.Intent)
	if !decision.Applied || decision.TriggerReason != "failure_pressure" {
		t.Fatalf("expected failure pressure downgrade decision, got %+v", decision)
	}
	if !containsString(decision.DegradeActions, "skip_expensive_tools") {
		t.Fatalf("expected failure pressure decision to disable expensive tools, got %+v", decision)
	}
	if decision.Trace == nil || decision.Trace["failure_signal_window"] != 2 {
		t.Fatalf("expected failure pressure trace to expose configured policy window, got %+v", decision.Trace)
	}
}

func TestServiceFailExecutionTaskAppendsBudgetFailureSignal(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed failure signal")
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_budget_failure_audit",
		Title:       "failure signal task",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: contextsvc.TaskContextSnapshot{
			Text:      "provider failure should leave budget signal",
			InputType: "text",
			Trigger:   "hover_text_input",
		},
	})
	updatedTask, bubble := service.failExecutionTask(task, task.Intent, execution.Result{}, model.ErrClientNotConfigured)
	if bubble == nil {
		t.Fatal("expected failure bubble")
	}
	foundBudgetFailure := false
	for _, record := range updatedTask.AuditRecords {
		if stringValue(record, "category", "") != "budget_auto_downgrade" {
			continue
		}
		if stringValue(record, "result", "") != "failed" {
			continue
		}
		foundBudgetFailure = true
		break
	}
	if !foundBudgetFailure {
		t.Fatalf("expected failExecutionTask to append budget failure signal, got %+v", updatedTask.AuditRecords)
	}
}

func TestServiceBudgetFallbackSuccessStillAppendsFailureSignal(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed fallback success signal")
	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_budget_fallback_signal",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": strings.Repeat("fallback signal content ", 12),
		},
		"options": map[string]any{
			"confirm_required": false,
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	mutateRuntimeTask(t, service.runEngine, taskID, func(record *runengine.TaskRecord) {
		record.AuditRecords = append(record.AuditRecords, map[string]any{
			"category": "budget_auto_downgrade",
			"action":   "budget_auto_downgrade.failure_signal",
			"result":   "failed",
			"reason":   model.ErrClientNotConfigured.Error(),
		})
	})
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after fallback success")
	}
	foundBudgetFailure := false
	for _, auditRecord := range record.AuditRecords {
		if stringValue(auditRecord, "action", "") != "budget_auto_downgrade.failure_signal" {
			continue
		}
		foundBudgetFailure = true
		break
	}
	if !foundBudgetFailure {
		t.Fatalf("expected fallback success path to retain budget failure signal, got %+v", record.AuditRecords)
	}
}

func TestServiceBudgetAutoDowngradeUsesConfiguredPolicyThresholds(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed configured thresholds")
	_, _, _, needRestart, err := service.runEngine.UpdateSettings(map[string]any{
		"models": map[string]any{
			"budget_auto_downgrade": true,
			"budget_policy": map[string]any{
				"failure_signal_window":     3,
				"token_pressure_threshold":  120,
				"cost_pressure_threshold":   0.20,
				"planner_retry_budget":      2,
				"expensive_tool_categories": []any{"command", "browser_mutation", "media_heavy"},
			},
		},
	})
	if err != nil {
		t.Fatalf("update settings failed: %v", err)
	}
	if needRestart {
		t.Fatal("expected nested budget policy update to remain immediate")
	}
	task := runengine.TaskRecord{
		TaskID:            "task_budget_policy_thresholds",
		SessionID:         "sess_budget_policy_thresholds",
		RunID:             "run_budget_policy_thresholds",
		Title:             "configured thresholds task",
		SourceType:        "hover_input",
		Status:            "processing",
		Intent:            map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		PreferredDelivery: "workspace_document",
		FallbackDelivery:  "bubble",
		CurrentStep:       "generate_output",
		RiskLevel:         "green",
		StartedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		TokenUsage: map[string]any{
			"total_tokens":   96,
			"estimated_cost": 0.08,
		},
	}
	decision := service.evaluateBudgetAutoDowngrade(task, task.Intent)
	if decision.Applied {
		t.Fatalf("expected configured thresholds to suppress downgrade below custom limits, got %+v", decision)
	}
	task.TokenUsage["total_tokens"] = 140
	decision = service.evaluateBudgetAutoDowngrade(task, task.Intent)
	if !decision.Applied || decision.Trace == nil || decision.Trace["planner_retry_budget"] != 2 || decision.Trace["failure_signal_window"] != 3 {
		t.Fatalf("expected configured thresholds and trace metadata to be respected, got %+v", decision)
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

func TestServiceSecurityPendingListFallsBackToApprovalRequestStore(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "security pending approval store fallback")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.ApprovalRequestStore().WriteApprovalRequest(context.Background(), storage.ApprovalRequestRecord{
		ApprovalID:      "appr_store_only_001",
		TaskID:          "task_store_only_001",
		OperationName:   "restore_apply",
		RiskLevel:       "red",
		TargetObject:    "workspace/result.md",
		Reason:          "stored approval request should backfill pending list",
		Status:          "pending",
		ImpactScopeJSON: `{"files":["workspace/result.md"]}`,
		CreatedAt:       "2026-04-18T10:00:00Z",
		UpdatedAt:       "2026-04-18T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("write approval request failed: %v", err)
	}

	result, err := service.SecurityPendingList(map[string]any{"limit": float64(20), "offset": float64(0)})
	if err != nil {
		t.Fatalf("security pending list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one storage-backed pending authorization, got %+v", items)
	}
	if items[0]["task_id"] != "task_store_only_001" || items[0]["operation_name"] != "restore_apply" {
		t.Fatalf("unexpected storage-backed pending authorization item: %+v", items[0])
	}
	impactScope, ok := items[0]["impact_scope"].(map[string]any)
	if !ok || len(impactScope) == 0 {
		t.Fatalf("expected impact_scope to be restored from approval store, got %+v", items[0])
	}
	page := result["page"].(map[string]any)
	if page["total"] != 1 || page["has_more"] != false {
		t.Fatalf("expected storage-backed page metadata, got %+v", page)
	}
}

func TestServiceSecurityPendingListIgnoresResolvedApprovalStoreRecords(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "security pending approval resolved fallback")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.ApprovalRequestStore().WriteApprovalRequest(context.Background(), storage.ApprovalRequestRecord{
		ApprovalID:      "appr_store_resolved_001",
		TaskID:          "task_store_resolved_001",
		OperationName:   "restore_apply",
		RiskLevel:       "red",
		TargetObject:    "workspace/result.md",
		Reason:          "resolved approval request should not backfill pending list",
		Status:          "approved",
		ImpactScopeJSON: `{"files":["workspace/result.md"]}`,
		CreatedAt:       "2026-04-18T10:00:00Z",
		UpdatedAt:       "2026-04-18T10:01:00Z",
	})
	if err != nil {
		t.Fatalf("write resolved approval request failed: %v", err)
	}

	result, err := service.SecurityPendingList(map[string]any{"limit": float64(20), "offset": float64(0)})
	if err != nil {
		t.Fatalf("security pending list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 0 {
		t.Fatalf("expected resolved approval record to stay out of pending list, got %+v", items)
	}
	page := result["page"].(map[string]any)
	if page["total"] != 0 || page["has_more"] != false {
		t.Fatalf("expected empty page metadata after filtering resolved approvals, got %+v", page)
	}
}

func TestServiceSecurityRestoreApplyReturnsStorageErrorWhenApprovalPersistenceFails(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "restore approval persistence failure output")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	originalStore := service.storage.ApprovalRequestStore()
	defer replaceApprovalRequestStore(t, service.storage, originalStore)
	originalPath := filepath.Join("notes", "output.md")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, originalPath), []byte("old content"), 0o644); err != nil {
		t.Fatalf("seed output file: %v", err)
	}

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_restore_store_failure",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请覆盖该文件",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": originalPath,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if _, err := service.SecurityRespond(map[string]any{"task_id": taskID, "decision": "allow_once"}); err != nil {
		t.Fatalf("security respond failed: %v", err)
	}
	pointsResult, err := service.SecurityRestorePointsList(map[string]any{"task_id": taskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security restore points list failed: %v", err)
	}
	points := pointsResult["items"].([]map[string]any)
	if len(points) == 0 {
		t.Fatal("expected restore point to exist")
	}

	replaceApprovalRequestStore(t, service.storage, failingApprovalRequestStore{base: originalStore, err: errors.New("approval store unavailable")})
	_, err = service.SecurityRestoreApply(map[string]any{"task_id": taskID, "recovery_point_id": points[0]["recovery_point_id"]})
	if err == nil || !errors.Is(err, ErrStorageQueryFailed) {
		t.Fatalf("expected ErrStorageQueryFailed from restore apply persistence, got %v", err)
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
	approvalItems, approvalTotal, err := service.storage.ApprovalRequestStore().ListApprovalRequests(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("list persisted approval requests failed: %v", err)
	}
	if approvalTotal < 1 || len(approvalItems) == 0 {
		t.Fatalf("expected persisted approval request for restore apply, got total=%d items=%+v", approvalTotal, approvalItems)
	}
	foundRestoreApply := false
	for _, item := range approvalItems {
		if item.OperationName == "restore_apply" && item.Status == "pending" {
			foundRestoreApply = true
			break
		}
	}
	if !foundRestoreApply {
		t.Fatalf("expected restore_apply approval request to be persisted, got %+v", approvalItems)
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

	if _, ok := detailResult["delivery_result"]; !ok {
		t.Fatal("expected task detail response to expose formal delivery_result field")
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
	citations, ok := detailResult["citations"].([]map[string]any)
	if !ok || len(citations) != 0 {
		t.Fatalf("expected empty citation collection array, got %+v", detailResult["citations"])
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
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 0 {
		t.Fatalf("expected no citations for generic detail payload, got %+v", citations)
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

func TestServiceTaskDetailGetFallsBackToTaskRunCitationsForScreenTask(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored screen detail citations")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if service.storage.TaskStore() == nil {
		t.Fatal("expected task store to be wired")
	}
	if err := service.storage.TaskStore().DeleteTask(context.Background(), "task_stored_screen_detail"); err != nil && !storage.IsTaskRecordNotFound(err) {
		t.Fatalf("delete structured task shadow failed: %v", err)
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_screen_detail",
		SessionID:   "sess_stored_screen",
		RunID:       "run_stored_screen_detail",
		Title:       "stored screen detail task",
		SourceType:  "screen_capture",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 20, 10, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 20, 10, 6, 0, 0, time.UTC)),
		Artifacts: []map[string]any{{
			"artifact_id":      "art_stored_screen_detail",
			"task_id":          "task_stored_screen_detail",
			"artifact_type":    "screen_capture",
			"title":            "stored-screen.png",
			"path":             "workspace/stored-screen.png",
			"mime_type":        "image/png",
			"delivery_type":    "open_file",
			"delivery_payload": map[string]any{"path": "workspace/stored-screen.png", "task_id": "task_stored_screen_detail", "evidence_role": "error_evidence"},
			"created_at":       "2026-04-20T10:06:00Z",
		}},
		Citations: []map[string]any{{
			"citation_id": "cit_task_stored_screen_detail_" + stableCitationIdentity("task_stored_screen_detail", "file", "art_stored_screen_detail", map[string]any{
				"artifact_id":   "art_stored_screen_detail",
				"artifact_type": "screen_capture",
				"evidence_role": "error_evidence",
				"ocr_excerpt":   "fatal build error",
			}),
			"task_id":     "task_stored_screen_detail",
			"run_id":      "run_stored_screen_detail",
			"source_type": "file",
			"source_ref":  "art_stored_screen_detail",
			"label":       "error_evidence | screen_capture | fatal build error",
		}},
	})
	if err != nil {
		t.Fatalf("save task run with citations failed: %v", err)
	}
	if err := service.runEngine.DeleteTask("task_stored_screen_detail"); err != nil && !errors.Is(err, runengine.ErrTaskNotFound) {
		t.Fatalf("delete runtime task shadow failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": "task_stored_screen_detail"})
	if err != nil {
		t.Fatalf("task detail get with stored citations failed: %v", err)
	}
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 1 || citations[0]["source_ref"] != "art_stored_screen_detail" {
		t.Fatalf("expected storage-backed citation to round-trip through task detail, got %+v", citations)
	}
}

func TestServiceTaskDetailGetPrefersStructuredTaskStoreFallback(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task detail")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              "task_structured_detail",
		SessionID:           "sess_structured",
		RunID:               "run_structured_detail",
		Title:               "structured detail task",
		SourceType:          "hover_input",
		Status:              "completed",
		IntentName:          "summarize",
		IntentArgumentsJSON: `{"style":"key_points"}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "green",
		StartedAt:           time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 10, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 15, 10, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), "task_structured_detail", []storage.TaskStepRecord{{
		StepID:        "step_structured_detail",
		TaskID:        "task_structured_detail",
		Name:          "deliver_result",
		Status:        "completed",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "structured output",
		CreatedAt:     time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 15, 10, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace structured task steps failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": "task_structured_detail"})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	task := detailResult["task"].(map[string]any)
	if task["task_id"] != "task_structured_detail" || task["title"] != "structured detail task" {
		t.Fatalf("expected structured task detail fallback, got %+v", task)
	}
	timeline := detailResult["timeline"].([]map[string]any)
	if len(timeline) != 1 || timeline[0]["step_id"] != "step_structured_detail" {
		t.Fatalf("expected structured timeline fallback, got %+v", timeline)
	}
	intent := task["intent"].(map[string]any)
	arguments := intent["arguments"].(map[string]any)
	if arguments["style"] != "key_points" {
		t.Fatalf("expected structured task detail to preserve intent arguments, got %+v", intent)
	}
}

func TestServiceTaskDetailGetStructuredFallbackUsesSessionAndRunStores(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured session run detail")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.SessionStore().WriteSession(context.Background(), storage.SessionRecord{
		SessionID: "sess_structured_link",
		Title:     "Structured Session Title",
		Status:    "idle",
		CreatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt: time.Date(2026, 4, 15, 10, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("write session failed: %v", err)
	}
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              "task_structured_session_run",
		SessionID:           "sess_structured_link",
		RunID:               "run_structured_session_run",
		Title:               "",
		SourceType:          "hover_input",
		Status:              "processing",
		IntentName:          "summarize",
		IntentArgumentsJSON: `{"style":"key_points"}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "generate_output",
		CurrentStepStatus:   "processing",
		RiskLevel:           "green",
		StartedAt:           time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 10, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.LoopRuntimeStore().SaveRun(context.Background(), storage.RunRecord{
		RunID:      "run_structured_session_run",
		TaskID:     "task_structured_session_run",
		SessionID:  "sess_structured_link",
		Status:     "processing",
		IntentName: "summarize",
		StartedAt:  time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:  time.Date(2026, 4, 15, 10, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		StopReason: "paused_by_session",
	}); err != nil {
		t.Fatalf("write run failed: %v", err)
	}
	result, err := service.TaskDetailGet(map[string]any{"task_id": "task_structured_session_run"})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	task := result["task"].(map[string]any)
	if task["title"] != "Structured Session Title" {
		t.Fatalf("expected session store to backfill task title, got %+v", task)
	}
	if task["loop_stop_reason"] != "paused_by_session" {
		t.Fatalf("expected run store to backfill loop stop reason, got %+v", task)
	}
}

func TestServiceTaskDetailGetPrefersRuntimeStateWhenStructuredRowIsStale(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "runtime detail beats stale storage")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	runtimeTask := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_runtime_freshness",
		Title:       "Runtime freshness task",
		SourceType:  "hover_input",
		Status:      "confirming_intent",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		CurrentStep: "confirming_intent",
		RiskLevel:   "green",
	})
	runtimeTask, ok := service.runEngine.BeginExecution(runtimeTask.TaskID, "generate_output", "Working on the response.")
	if !ok {
		t.Fatal("expected runtime task to begin execution")
	}
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              runtimeTask.TaskID,
		SessionID:           runtimeTask.SessionID,
		RunID:               runtimeTask.RunID,
		Title:               runtimeTask.Title,
		SourceType:          runtimeTask.SourceType,
		Status:              "confirming_intent",
		IntentName:          "summarize",
		IntentArgumentsJSON: `{"style":"key_points"}`,
		PreferredDelivery:   runtimeTask.PreferredDelivery,
		FallbackDelivery:    runtimeTask.FallbackDelivery,
		CurrentStep:         "confirming_intent",
		CurrentStepStatus:   "pending",
		RiskLevel:           runtimeTask.RiskLevel,
		StartedAt:           runtimeTask.StartedAt.Format(time.RFC3339Nano),
		UpdatedAt:           runtimeTask.StartedAt.Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("write stale structured task failed: %v", err)
	}
	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": runtimeTask.TaskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	task := detailResult["task"].(map[string]any)
	if task["status"] != "processing" || task["current_step"] != "generate_output" {
		t.Fatalf("expected runtime task state to override stale storage, got %+v", task)
	}
}

func TestServiceTaskDetailGetStructuredFallbackBackfillsTaskRunEvidence(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured screen detail evidence")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	taskID := "task_structured_screen_evidence"
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              taskID,
		SessionID:           "sess_structured_screen",
		RunID:               "run_structured_screen_evidence",
		Title:               "structured screen evidence task",
		SourceType:          "screen_capture",
		Status:              "failed",
		IntentName:          "screen_analyze",
		IntentArgumentsJSON: `{"language":"eng"}`,
		PreferredDelivery:   "bubble",
		FallbackDelivery:    "bubble",
		CurrentStep:         "generate_output",
		CurrentStepStatus:   "failed",
		RiskLevel:           "yellow",
		StartedAt:           time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 15, 13, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SnapshotJSON:        "{invalid-json}",
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), taskID, []storage.TaskStepRecord{{
		StepID:        "step_structured_screen_evidence",
		TaskID:        taskID,
		Name:          "generate_output",
		Status:        "failed",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "structured output",
		CreatedAt:     time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace structured task steps failed: %v", err)
	}
	if err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      taskID,
		SessionID:   "sess_structured_screen",
		RunID:       "run_structured_screen_evidence",
		Title:       "structured screen evidence task",
		SourceType:  "screen_capture",
		Status:      "failed",
		Intent:      map[string]any{"name": "screen_analyze", "arguments": map[string]any{"language": "eng"}},
		CurrentStep: "generate_output",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 15, 13, 6, 0, 0, time.UTC)),
		DeliveryResult: map[string]any{
			"type":         "task_detail",
			"title":        "屏幕分析结果",
			"preview_text": "fatal build error",
			"payload": map[string]any{
				"task_id": taskID,
				"path":    "workspace/structured-screen.png",
				"url":     nil,
			},
		},
		Citations: []map[string]any{{
			"citation_id": "cit_" + taskID + "_" + stableCitationIdentity(taskID, "file", "art_structured_screen_evidence", map[string]any{
				"artifact_id":   "art_structured_screen_evidence",
				"artifact_type": "screen_capture",
				"evidence_role": "error_evidence",
				"ocr_excerpt":   "fatal build error",
			}),
			"task_id":       taskID,
			"run_id":        "run_structured_screen_evidence",
			"source_type":   "file",
			"source_ref":    "art_structured_screen_evidence",
			"label":         "error_evidence | screen_capture | fatal build error",
			"artifact_id":   "art_structured_screen_evidence",
			"artifact_type": "screen_capture",
			"evidence_role": "error_evidence",
			"excerpt_text":  "fatal build error",
		}},
		AuditRecords: []map[string]any{{
			"audit_id":   "audit_structured_screen_evidence",
			"task_id":    taskID,
			"type":       "execution",
			"action":     "execute_task",
			"summary":    "OCR worker failed while analyzing the screen.",
			"target":     "workspace/structured-screen.png",
			"result":     "failed",
			"created_at": "2026-04-15T13:05:00Z",
			"metadata": map[string]any{
				"failure_code":     "OCR_WORKER_FAILED",
				"failure_category": "screen_ocr",
			},
		}},
	}); err != nil {
		t.Fatalf("save task run with structured fallback evidence failed: %v", err)
	}
	if err := service.runEngine.DeleteTask(taskID); err != nil && !errors.Is(err, runengine.ErrTaskNotFound) {
		t.Fatalf("delete runtime task shadow failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	runtimeSummary := detailResult["runtime_summary"].(map[string]any)
	if runtimeSummary["latest_failure_code"] != "OCR_WORKER_FAILED" || runtimeSummary["latest_failure_category"] != "screen_ocr" {
		t.Fatalf("expected structured fallback to backfill failure metadata, got %+v", runtimeSummary)
	}
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 1 || citations[0]["source_ref"] != "art_structured_screen_evidence" {
		t.Fatalf("expected structured fallback to backfill citations, got %+v", citations)
	}
	if citations[0]["excerpt_text"] != "fatal build error" || citations[0]["evidence_role"] != "error_evidence" {
		t.Fatalf("expected structured fallback to preserve citation metadata, got %+v", citations[0])
	}
	deliveryResult, ok := detailResult["delivery_result"].(map[string]any)
	if !ok || deliveryResult["preview_text"] != "fatal build error" {
		t.Fatalf("expected structured fallback to backfill delivery result, got %+v", detailResult["delivery_result"])
	}
}

func TestServiceTaskDetailGetStructuredFallbackReadsFormalDeliveryFromLoopStore(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task detail delivery fallback")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime storage to be wired")
	}
	taskID := "task_structured_delivery_only"
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              taskID,
		SessionID:           "sess_structured_delivery",
		RunID:               "run_structured_delivery_only",
		Title:               "structured delivery only task",
		SourceType:          "screen_capture",
		Status:              "completed",
		IntentName:          "screen_analyze",
		IntentArgumentsJSON: `{"language":"eng"}`,
		PreferredDelivery:   "task_detail",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "yellow",
		StartedAt:           time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 15, 14, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SnapshotJSON:        "{invalid-json}",
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), taskID, []storage.TaskStepRecord{{
		StepID:        "step_structured_delivery_only",
		TaskID:        taskID,
		Name:          "deliver_result",
		Status:        "completed",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "structured output",
		CreatedAt:     time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace structured task steps failed: %v", err)
	}
	if err := service.storage.LoopRuntimeStore().SaveDeliveryResult(context.Background(), storage.DeliveryResultRecord{
		DeliveryResultID: "delivery_" + taskID,
		TaskID:           taskID,
		Type:             "task_detail",
		Title:            "结构化交付结果",
		PayloadJSON:      `{"task_id":"` + taskID + `","path":"workspace/structured-delivery.md","url":null}`,
		PreviewText:      "formal delivery from loop store",
		CreatedAt:        "2026-04-15T14:06:00Z",
	}); err != nil {
		t.Fatalf("save loop runtime delivery result failed: %v", err)
	}
	if err := service.runEngine.DeleteTask(taskID); err != nil && !errors.Is(err, runengine.ErrTaskNotFound) {
		t.Fatalf("delete runtime task shadow failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	deliveryResult, ok := detailResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured fallback to read delivery_result from loop store, got %+v", detailResult["delivery_result"])
	}
	if deliveryResult["title"] != "结构化交付结果" || deliveryResult["preview_text"] != "formal delivery from loop store" {
		t.Fatalf("unexpected loop-store delivery result: %+v", deliveryResult)
	}
	payload := deliveryResult["payload"].(map[string]any)
	if payload["path"] != "workspace/structured-delivery.md" || payload["task_id"] != taskID {
		t.Fatalf("expected loop-store delivery payload to stay intact, got %+v", payload)
	}
	if _, ok := payload["url"]; !ok || payload["url"] != nil {
		t.Fatalf("expected loop-store delivery payload to expose missing url as null, got %+v", payload)
	}
}

func TestServiceTaskDetailGetStructuredFallbackReadsFormalCitationsFromLoopStore(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task detail citation fallback")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime storage to be wired")
	}
	taskID := "task_structured_citation_only"
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              taskID,
		SessionID:           "sess_structured_citation",
		RunID:               "run_structured_citation_only",
		Title:               "structured citation only task",
		SourceType:          "screen_capture",
		Status:              "completed",
		IntentName:          "screen_analyze",
		IntentArgumentsJSON: `{"language":"eng"}`,
		PreferredDelivery:   "task_detail",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "yellow",
		StartedAt:           time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 15, 14, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SnapshotJSON:        "{invalid-json}",
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), taskID, []storage.TaskStepRecord{{
		StepID:        "step_structured_citation_only",
		TaskID:        taskID,
		Name:          "deliver_result",
		Status:        "completed",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "structured output",
		CreatedAt:     time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace structured task steps failed: %v", err)
	}
	if err := service.storage.LoopRuntimeStore().ReplaceTaskCitations(context.Background(), taskID, []storage.CitationRecord{{
		CitationID:      "cit_" + taskID,
		TaskID:          taskID,
		RunID:           "run_structured_citation_only",
		SourceType:      "file",
		SourceRef:       "art_structured_citation_only",
		Label:           "error_evidence | screen_capture | fatal build error",
		ArtifactID:      "art_structured_citation_only",
		ArtifactType:    "screen_capture",
		EvidenceRole:    "error_evidence",
		ExcerptText:     "fatal build error",
		ScreenSessionID: "screen_sess_citation_only",
		OrderIndex:      0,
	}}); err != nil {
		t.Fatalf("save loop runtime citations failed: %v", err)
	}
	if err := service.runEngine.DeleteTask(taskID); err != nil && !errors.Is(err, runengine.ErrTaskNotFound) {
		t.Fatalf("delete runtime task shadow failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 1 {
		t.Fatalf("expected loop-store citations to backfill task detail, got %+v", citations)
	}
	if citations[0]["artifact_id"] != "art_structured_citation_only" || citations[0]["source_ref"] != "art_structured_citation_only" {
		t.Fatalf("expected loop-store citation artifact fields to survive, got %+v", citations[0])
	}
	if citations[0]["evidence_role"] != "error_evidence" || citations[0]["excerpt_text"] != "fatal build error" {
		t.Fatalf("expected loop-store citation evidence metadata to survive, got %+v", citations[0])
	}
	if citations[0]["screen_session_id"] != "screen_sess_citation_only" {
		t.Fatalf("expected loop-store citation screen session id to survive, got %+v", citations[0])
	}
}

func TestServiceTaskDetailGetStructuredFallbackPrefersFirstClassDeliveryAndCitationsOverSnapshot(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task detail mixed state precedence")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime storage to be wired")
	}
	taskID := "task_structured_mixed_state"
	finishedAt := time.Date(2026, 4, 15, 14, 6, 0, 0, time.UTC)
	snapshotJSONBytes, err := json.Marshal(storage.TaskRunRecord{
		TaskID:            taskID,
		SessionID:         "sess_structured_mixed_state",
		RunID:             "run_structured_mixed_state",
		Title:             "structured mixed state task",
		SourceType:        "screen_capture",
		Status:            "completed",
		Intent:            map[string]any{"name": "screen_analyze", "arguments": map[string]any{"language": "eng"}},
		PreferredDelivery: "task_detail",
		FallbackDelivery:  "bubble",
		CurrentStep:       "deliver_result",
		RiskLevel:         "yellow",
		StartedAt:         time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC),
		UpdatedAt:         time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC),
		FinishedAt:        &finishedAt,
		Timeline: []storage.TaskStepSnapshot{{
			StepID:        "step_structured_mixed_state_snapshot",
			TaskID:        taskID,
			Name:          "deliver_result",
			Status:        "completed",
			OrderIndex:    1,
			InputSummary:  "snapshot input",
			OutputSummary: "snapshot output",
		}},
		DeliveryResult: map[string]any{
			"type":         "task_detail",
			"title":        "snapshot delivery",
			"preview_text": "stale snapshot delivery",
			"payload":      map[string]any{"task_id": taskID, "path": "workspace/stale-snapshot.md", "url": nil},
		},
		Citations: []map[string]any{{
			"citation_id": "cit_snapshot_" + taskID,
			"task_id":     taskID,
			"run_id":      "run_structured_mixed_state",
			"source_type": "file",
			"source_ref":  "art_snapshot_only",
			"label":       "snapshot evidence",
			"artifact_id": "art_snapshot_only",
		}},
	})
	if err != nil {
		t.Fatalf("marshal snapshot json failed: %v", err)
	}
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              taskID,
		SessionID:           "sess_structured_mixed_state",
		RunID:               "run_structured_mixed_state",
		Title:               "structured mixed state task",
		SourceType:          "screen_capture",
		Status:              "completed",
		IntentName:          "screen_analyze",
		IntentArgumentsJSON: `{"language":"eng"}`,
		PreferredDelivery:   "task_detail",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "yellow",
		StartedAt:           time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          finishedAt.Format(time.RFC3339Nano),
		SnapshotJSON:        string(snapshotJSONBytes),
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.LoopRuntimeStore().SaveDeliveryResult(context.Background(), storage.DeliveryResultRecord{
		DeliveryResultID: "delivery_" + taskID,
		TaskID:           taskID,
		Type:             "task_detail",
		Title:            "first-class delivery",
		PayloadJSON:      `{"task_id":"` + taskID + `","path":"workspace/formal-delivery.md","url":null}`,
		PreviewText:      "newer first-class delivery",
		CreatedAt:        "2026-04-15T14:07:00Z",
	}); err != nil {
		t.Fatalf("save first-class delivery result failed: %v", err)
	}
	if err := service.storage.LoopRuntimeStore().ReplaceTaskCitations(context.Background(), taskID, []storage.CitationRecord{{
		CitationID:      "cit_" + taskID,
		TaskID:          taskID,
		RunID:           "run_structured_mixed_state",
		SourceType:      "file",
		SourceRef:       "art_first_class",
		Label:           "error_evidence | screen_capture | formal excerpt",
		ArtifactID:      "art_first_class",
		ArtifactType:    "screen_capture",
		EvidenceRole:    "error_evidence",
		ExcerptText:     "formal excerpt",
		ScreenSessionID: "screen_sess_first_class",
		OrderIndex:      0,
	}}); err != nil {
		t.Fatalf("save first-class citations failed: %v", err)
	}
	if err := service.runEngine.DeleteTask(taskID); err != nil && !errors.Is(err, runengine.ErrTaskNotFound) {
		t.Fatalf("delete runtime task shadow failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	deliveryResult, ok := detailResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatalf("expected mixed-state detail to expose formal delivery_result, got %+v", detailResult["delivery_result"])
	}
	if deliveryResult["title"] != "first-class delivery" || deliveryResult["preview_text"] != "newer first-class delivery" {
		t.Fatalf("expected first-class delivery to override snapshot delivery, got %+v", deliveryResult)
	}
	payload := deliveryResult["payload"].(map[string]any)
	if payload["path"] != "workspace/formal-delivery.md" {
		t.Fatalf("expected first-class delivery payload to override snapshot payload, got %+v", payload)
	}
	citations := detailResult["citations"].([]map[string]any)
	if len(citations) != 1 {
		t.Fatalf("expected first-class citations to override snapshot citations, got %+v", citations)
	}
	if citations[0]["source_ref"] != "art_first_class" || citations[0]["artifact_id"] != "art_first_class" {
		t.Fatalf("expected first-class citation source to override snapshot citation, got %+v", citations[0])
	}
	if citations[0]["screen_session_id"] != "screen_sess_first_class" || citations[0]["excerpt_text"] != "formal excerpt" {
		t.Fatalf("expected first-class citation metadata to override snapshot citation, got %+v", citations[0])
	}
}

func TestServiceTaskDetailGetStructuredFallbackNormalizesSparseDeliveryPayloadKeys(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task detail sparse delivery fallback")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime storage to be wired")
	}
	taskID := "task_structured_delivery_sparse"
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:            taskID,
		SessionID:         "sess_structured_delivery_sparse",
		RunID:             "run_structured_delivery_sparse",
		Title:             "structured sparse delivery task",
		SourceType:        "screen_capture",
		Status:            "completed",
		IntentName:        "screen_analyze",
		PreferredDelivery: "task_detail",
		FallbackDelivery:  "bubble",
		CurrentStep:       "deliver_result",
		CurrentStepStatus: "completed",
		RiskLevel:         "yellow",
		StartedAt:         time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:         time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:        time.Date(2026, 4, 15, 14, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SnapshotJSON:      "{invalid-json}",
	}); err != nil {
		t.Fatalf("write sparse structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), taskID, []storage.TaskStepRecord{{
		StepID:        "step_structured_delivery_sparse",
		TaskID:        taskID,
		Name:          "deliver_result",
		Status:        "completed",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "structured output",
		CreatedAt:     time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace sparse structured task steps failed: %v", err)
	}
	if err := service.storage.LoopRuntimeStore().SaveDeliveryResult(context.Background(), storage.DeliveryResultRecord{
		DeliveryResultID: "delivery_" + taskID,
		TaskID:           taskID,
		Type:             "task_detail",
		Title:            "结构化稀疏交付结果",
		PayloadJSON:      `{}`,
		PreviewText:      "sparse formal delivery from loop store",
		CreatedAt:        "2026-04-15T14:06:00Z",
	}); err != nil {
		t.Fatalf("save sparse loop runtime delivery result failed: %v", err)
	}
	if err := service.runEngine.DeleteTask(taskID); err != nil && !errors.Is(err, runengine.ErrTaskNotFound) {
		t.Fatalf("delete sparse runtime task shadow failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	deliveryResult, ok := detailResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured fallback to read sparse delivery_result from loop store, got %+v", detailResult["delivery_result"])
	}
	payload := deliveryResult["payload"].(map[string]any)
	if _, ok := payload["path"]; !ok || payload["path"] != nil {
		t.Fatalf("expected sparse loop-store delivery payload to expose missing path as null, got %+v", payload)
	}
	if _, ok := payload["url"]; !ok || payload["url"] != nil {
		t.Fatalf("expected sparse loop-store delivery payload to expose missing url as null, got %+v", payload)
	}
	if payload["task_id"] != taskID {
		t.Fatalf("expected sparse loop-store delivery payload to backfill task_id, got %+v", payload)
	}
}

func TestServiceTaskDetailGetStructuredFallbackRehydratesApprovalRequest(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task detail approval")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	taskID := "task_structured_waiting_auth"
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              taskID,
		SessionID:           "sess_structured",
		RunID:               "run_structured_waiting_auth",
		Title:               "structured waiting auth task",
		SourceType:          "hover_input",
		Status:              "waiting_auth",
		IntentName:          "write_file",
		IntentArgumentsJSON: `{"require_authorization":true}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "waiting_authorization",
		CurrentStepStatus:   "pending",
		RiskLevel:           "yellow",
		StartedAt:           time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 11, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SnapshotJSON:        "{invalid-json}",
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), taskID, []storage.TaskStepRecord{{
		StepID:        "step_waiting_auth",
		TaskID:        taskID,
		Name:          "waiting_authorization",
		Status:        "pending",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "waiting for approval",
		CreatedAt:     time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 15, 11, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace structured task steps failed: %v", err)
	}
	if err := service.storage.ApprovalRequestStore().WriteApprovalRequest(context.Background(), storage.ApprovalRequestRecord{
		ApprovalID:      "appr_structured_waiting_auth",
		TaskID:          taskID,
		OperationName:   "write_file",
		RiskLevel:       "yellow",
		TargetObject:    "workspace/document.md",
		Reason:          "structured fallback should restore approval state",
		Status:          "pending",
		ImpactScopeJSON: `{"files":["workspace/document.md"]}`,
		CreatedAt:       "2026-04-15T11:05:00Z",
		UpdatedAt:       "2026-04-15T11:05:00Z",
	}); err != nil {
		t.Fatalf("write approval request failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	approvalRequest, ok := detailResult["approval_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured fallback approval request, got %+v", detailResult["approval_request"])
	}
	if approvalRequest["approval_id"] != "appr_structured_waiting_auth" {
		t.Fatalf("unexpected structured fallback approval request: %+v", approvalRequest)
	}
	if detailResult["authorization_record"] != nil {
		t.Fatalf("expected structured fallback waiting_auth task to omit authorization_record, got %+v", detailResult["authorization_record"])
	}
	if detailResult["audit_record"] != nil {
		t.Fatalf("expected structured fallback waiting_auth task to omit audit_record, got %+v", detailResult["audit_record"])
	}
	securitySummary := detailResult["security_summary"].(map[string]any)
	if securitySummary["pending_authorizations"] != 1 || securitySummary["security_status"] != "pending_confirmation" {
		t.Fatalf("expected structured fallback security summary to reflect pending approval, got %+v", securitySummary)
	}
}

func TestServiceTaskDetailGetStructuredFallbackRehydratesAuthorizationAndAudit(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured task detail governance")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	taskID := "task_structured_screen_governance"
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              taskID,
		SessionID:           "sess_structured_governance",
		RunID:               "run_structured_screen_governance",
		Title:               "structured screen governance task",
		SourceType:          "screen_capture",
		Status:              "completed",
		IntentName:          "screen_analyze",
		IntentArgumentsJSON: `{"language":"eng"}`,
		PreferredDelivery:   "bubble",
		FallbackDelivery:    "bubble",
		CurrentStep:         "deliver_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "yellow",
		StartedAt:           time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 16, 9, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		FinishedAt:          time.Date(2026, 4, 16, 9, 6, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SnapshotJSON:        "{invalid-json}",
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.TaskStepStore().ReplaceTaskSteps(context.Background(), taskID, []storage.TaskStepRecord{{
		StepID:        "step_structured_screen_governance",
		TaskID:        taskID,
		Name:          "deliver_result",
		Status:        "completed",
		OrderIndex:    1,
		InputSummary:  "structured input",
		OutputSummary: "screen analysis complete",
		CreatedAt:     time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:     time.Date(2026, 4, 16, 9, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}); err != nil {
		t.Fatalf("replace structured task steps failed: %v", err)
	}
	if err := service.storage.AuthorizationRecordStore().WriteAuthorizationRecord(context.Background(), storage.AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_structured_screen_governance",
		TaskID:                taskID,
		ApprovalID:            "appr_structured_screen_governance",
		Decision:              "allow_once",
		Operator:              "user",
		RememberRule:          false,
		CreatedAt:             "2026-04-16T09:04:00Z",
	}); err != nil {
		t.Fatalf("write structured authorization record failed: %v", err)
	}
	if err := service.storage.AuditStore().WriteAuditRecord(context.Background(), audit.Record{
		AuditID:   "audit_structured_screen_governance",
		TaskID:    taskID,
		Type:      "execution",
		Action:    "execute_task",
		Summary:   "screen analysis completed with stored evidence.",
		Target:    "workspace/stored-screen.png",
		Result:    "success",
		CreatedAt: "2026-04-16T09:05:00Z",
	}); err != nil {
		t.Fatalf("write structured audit record failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}
	authorizationRecord := detailResult["authorization_record"].(map[string]any)
	if authorizationRecord["authorization_record_id"] != "auth_structured_screen_governance" {
		t.Fatalf("expected structured fallback authorization record, got %+v", authorizationRecord)
	}
	auditRecord := detailResult["audit_record"].(map[string]any)
	if auditRecord["audit_id"] != "audit_structured_screen_governance" {
		t.Fatalf("expected structured fallback audit record, got %+v", auditRecord)
	}
}

func TestNormalizeTaskDetailAuthorizationRecordCoercesLegacyDecisions(t *testing.T) {
	allowRecord := normalizeTaskDetailAuthorizationRecord("task_auth_detail", map[string]any{
		"authorization_record_id": "auth_allow",
		"task_id":                 "task_auth_detail",
		"approval_id":             "appr_allow",
		"decision":                "allow_always",
		"remember_rule":           true,
		"operator":                "user",
		"created_at":              "2026-04-20T16:00:00Z",
	})
	if allowRecord == nil || allowRecord["decision"] != "allow_once" {
		t.Fatalf("expected legacy allow decision to coerce to protocol enum, got %+v", allowRecord)
	}

	denyRecord := normalizeTaskDetailAuthorizationRecord("task_auth_detail", map[string]any{
		"authorization_record_id": "auth_deny",
		"task_id":                 "task_auth_detail",
		"approval_id":             "appr_deny",
		"decision":                "deny_always",
		"remember_rule":           false,
		"operator":                "user",
		"created_at":              "2026-04-20T16:01:00Z",
	})
	if denyRecord == nil || denyRecord["decision"] != "deny_once" {
		t.Fatalf("expected legacy deny decision to coerce to protocol enum, got %+v", denyRecord)
	}

	invalidRecord := normalizeTaskDetailAuthorizationRecord("task_auth_detail", map[string]any{
		"authorization_record_id": "auth_invalid",
		"task_id":                 "task_auth_detail",
		"approval_id":             "appr_invalid",
		"decision":                "manual_override",
		"remember_rule":           false,
		"operator":                "user",
		"created_at":              "2026-04-20T16:02:00Z",
	})
	if invalidRecord != nil {
		t.Fatalf("expected unsupported decision to be dropped from task detail, got %+v", invalidRecord)
	}
}

func TestServiceSecuritySummaryCountsStructuredFallbackPendingAuthorizations(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "structured security summary pending auth")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	taskID := "task_structured_security_waiting_auth"
	if err := service.storage.TaskStore().WriteTask(context.Background(), storage.TaskRecord{
		TaskID:              taskID,
		SessionID:           "sess_structured_security",
		RunID:               "run_structured_security_waiting_auth",
		Title:               "structured waiting auth security task",
		SourceType:          "hover_input",
		Status:              "waiting_auth",
		IntentName:          "write_file",
		IntentArgumentsJSON: `{"require_authorization":true}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "waiting_authorization",
		CurrentStepStatus:   "pending",
		RiskLevel:           "yellow",
		StartedAt:           time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		UpdatedAt:           time.Date(2026, 4, 15, 12, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		SnapshotJSON:        "{invalid-json}",
	}); err != nil {
		t.Fatalf("write structured task failed: %v", err)
	}
	if err := service.storage.ApprovalRequestStore().WriteApprovalRequest(context.Background(), storage.ApprovalRequestRecord{
		ApprovalID:      "appr_structured_security_waiting_auth",
		TaskID:          taskID,
		OperationName:   "write_file",
		RiskLevel:       "yellow",
		TargetObject:    "workspace/security.md",
		Reason:          "structured fallback should count pending authorization",
		Status:          "pending",
		ImpactScopeJSON: `{"files":["workspace/security.md"]}`,
		CreatedAt:       "2026-04-15T12:05:00Z",
		UpdatedAt:       "2026-04-15T12:05:00Z",
	}); err != nil {
		t.Fatalf("write approval request failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}
	summary := result["summary"].(map[string]any)
	if summary["pending_authorizations"] != 1 || summary["security_status"] != "pending_confirmation" {
		t.Fatalf("expected structured fallback pending authorization to appear in security summary, got %+v", summary)
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
	models := result["settings"].(map[string]any)["models"].(map[string]any)
	credentials := models["credentials"].(map[string]any)
	if credentials["provider_api_key_configured"] != false {
		t.Fatalf("expected unset provider key flag, got %+v", credentials)
	}
	stronghold := credentials["stronghold"].(map[string]any)
	if stronghold["backend"] == "" || stronghold["available"] != true {
		t.Fatalf("expected stronghold status metadata, got %+v", stronghold)
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
	models = result["settings"].(map[string]any)["models"].(map[string]any)
	credentials = models["credentials"].(map[string]any)
	if credentials["provider_api_key_configured"] != true {
		t.Fatalf("expected configured provider key flag, got %+v", credentials)
	}
}

func TestSettingsGetWithoutStorageStillReturnsStrongholdStatus(t *testing.T) {
	service := newTestService()
	service.storage = nil
	result, err := service.SettingsGet(map[string]any{"scope": "all"})
	if err != nil {
		t.Fatalf("settings get failed: %v", err)
	}
	models := result["settings"].(map[string]any)["models"].(map[string]any)
	stronghold := models["credentials"].(map[string]any)["stronghold"].(map[string]any)
	if stronghold["backend"] != "none" || stronghold["available"] != false || stronghold["formal_store"] != false {
		t.Fatalf("expected degraded settings get to still expose stronghold defaults, got %+v", stronghold)
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

func TestSettingsGetUnrelatedScopeIgnoresSecretStoreOutage(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings general scope")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.Close(); err != nil {
		t.Fatalf("close storage failed: %v", err)
	}
	result, err := service.SettingsGet(map[string]any{"scope": "general"})
	if err != nil {
		t.Fatalf("expected unrelated settings scope to ignore secret outage, got %v", err)
	}
	settings := result["settings"].(map[string]any)
	if _, ok := settings["general"].(map[string]any); !ok {
		t.Fatalf("expected general scope payload, got %+v", settings)
	}
}

func TestSettingsUpdatePersistsSecretOutsideRegularSettings(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings secret persist")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	result, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
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
	models := effectiveSettings["models"].(map[string]any)
	if _, exists := models["api_key"]; exists {
		t.Fatalf("expected api_key to stay out of regular settings path, got %+v", models)
	}
	if models["provider_api_key_configured"] != true {
		t.Fatalf("expected configured flag in settings response, got %+v", models)
	}
	if _, exists := models["stronghold"]; !exists {
		t.Fatalf("expected stronghold status in settings response, got %+v", models)
	}
}

func TestSettingsUpdateReturnsLeafModelKeys(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings leaf model keys")
	result, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider": "anthropic",
			"base_url": "https://example.invalid/v1",
			"model":    "claude-test",
		},
	})
	if err != nil {
		t.Fatalf("settings update failed: %v", err)
	}
	updatedKeys := result["updated_keys"].([]string)
	expectedKeys := []string{"models.base_url", "models.model", "models.provider"}
	if !reflect.DeepEqual(updatedKeys, expectedKeys) {
		t.Fatalf("expected leaf model updated keys, got %+v", updatedKeys)
	}
}

func TestNormalizeSettingsSnapshotAccumulatesLegacyDataLogFields(t *testing.T) {
	normalized := normalizeSettingsSnapshot(map[string]any{
		"data_log": map[string]any{
			"provider":              "openai",
			"budget_auto_downgrade": true,
			"base_url":              "https://example.invalid/v1",
			"model":                 "gpt-test",
		},
	})
	models := normalized["models"].(map[string]any)
	credentials := models["credentials"].(map[string]any)
	if models["provider"] != "openai" {
		t.Fatalf("expected legacy provider to map into models provider, got %+v", models)
	}
	if credentials["budget_auto_downgrade"] != true || credentials["base_url"] != "https://example.invalid/v1" || credentials["model"] != "gpt-test" {
		t.Fatalf("expected legacy data_log fields to accumulate into models.credentials, got %+v", credentials)
	}
}

func TestSettingsUpdatePersistsSecretForRequestedProvider(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings provider secret persist")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	_, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
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
	result, err := service.SettingsGet(map[string]any{"scope": "models"})
	if err != nil {
		t.Fatalf("settings get failed: %v", err)
	}
	models := result["settings"].(map[string]any)["models"].(map[string]any)
	credentials := models["credentials"].(map[string]any)
	if models["provider"] != "anthropic" || credentials["provider_api_key_configured"] != true {
		t.Fatalf("expected settings get to reflect anthropic provider secret, got models=%+v credentials=%+v", models, credentials)
	}
}

func TestSettingsUpdateDeletesProviderSecretWithoutLeakingValue(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings delete secret")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.SecretStore().PutSecret(context.Background(), storage.SecretRecord{
		Namespace: "model",
		Key:       service.model.Provider() + "_api_key",
		Value:     "secret-key",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed secret store failed: %v", err)
	}
	result, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider":       "openai",
			"delete_api_key": true,
		},
	})
	if err != nil {
		t.Fatalf("settings update delete failed: %v", err)
	}
	_, err = service.storage.SecretStore().GetSecret(context.Background(), "model", service.model.Provider()+"_api_key")
	if !errors.Is(err, storage.ErrSecretNotFound) {
		t.Fatalf("expected secret to be deleted, got %v", err)
	}
	models := result["effective_settings"].(map[string]any)["models"].(map[string]any)
	if _, exists := models["api_key"]; exists {
		t.Fatalf("expected settings delete response to stay redacted, got %+v", models)
	}
	if models["provider_api_key_configured"] != false {
		t.Fatalf("expected delete to clear configured flag, got %+v", models)
	}
}

func TestSettingsUpdateReturnsStrongholdErrorWhenStoreUnavailable(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings stronghold unavailable")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	originalStorage := service.storage
	defer func() {
		if service.storage != nil {
			_ = service.storage.Close()
		}
	}()
	if err := originalStorage.Close(); err != nil {
		t.Fatalf("close original storage failed: %v", err)
	}
	service.storage = storage.NewService(nil)
	replaceSecretStore(t, service.storage, storage.UnavailableSecretStore{})
	_, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"api_key":  "secret-key",
		},
	})
	if !errors.Is(err, ErrStrongholdAccessFailed) {
		t.Fatalf("expected ErrStrongholdAccessFailed, got %v", err)
	}
}

func TestSettingsUpdateReturnsStrongholdErrorWithoutStorage(t *testing.T) {
	service := newTestService()
	_, err := service.SettingsUpdate(map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"api_key":  "sk-test",
		},
	})
	if !errors.Is(err, ErrStrongholdAccessFailed) {
		t.Fatalf("expected ErrStrongholdAccessFailed, got %v", err)
	}
}

func TestSettingsUpdateUnrelatedScopeIgnoresSecretStoreOutage(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "settings unrelated update")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.Close(); err != nil {
		t.Fatalf("close storage failed: %v", err)
	}
	result, err := service.SettingsUpdate(map[string]any{
		"general": map[string]any{
			"language": "zh-CN",
		},
	})
	if err != nil {
		t.Fatalf("expected unrelated settings update to ignore secret outage, got %v", err)
	}
	effectiveSettings := result["effective_settings"].(map[string]any)
	if _, ok := effectiveSettings["general"].(map[string]any); !ok {
		t.Fatalf("expected general effective settings payload, got %+v", effectiveSettings)
	}
	if _, exists := effectiveSettings["models"]; exists {
		t.Fatalf("expected unrelated settings update to avoid attaching model metadata, got %+v", effectiveSettings)
	}
}

func TestServicePluginRuntimeListReturnsStructuredState(t *testing.T) {
	service := newTestService()
	service.plugin.MarkRuntimeStarting(plugin.RuntimeKindWorker, "ocr_worker")
	service.plugin.MarkRuntimeHealthy(plugin.RuntimeKindWorker, "ocr_worker")
	service.plugin.MarkRuntimeFailed(plugin.RuntimeKindSidecar, "playwright_sidecar", errors.New("sidecar failed"))
	result, err := service.PluginRuntimeList(map[string]any{})
	if err != nil {
		t.Fatalf("plugin runtime list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	metrics := result["metrics"].([]map[string]any)
	events := result["events"].([]map[string]any)
	if len(items) == 0 || len(metrics) == 0 || len(events) == 0 {
		t.Fatalf("expected runtime query to return items/metrics/events, got %+v", result)
	}
	foundFailedSidecar := false
	for _, item := range items {
		if item["name"] == "playwright_sidecar" && fmt.Sprint(item["health"]) == string(plugin.RuntimeHealthFailed) {
			foundFailedSidecar = true
			break
		}
	}
	if !foundFailedSidecar {
		t.Fatalf("expected runtime query to expose failed sidecar state, got %+v", items)
	}
}

func TestServicePluginListReturnsStructuredCatalog(t *testing.T) {
	service := newTestService()
	service.plugin.MarkRuntimeHealthy(plugin.RuntimeKindWorker, "ocr_worker")
	service.plugin.MarkRuntimeFailed(plugin.RuntimeKindSidecar, "playwright_sidecar", errors.New("sidecar failed"))

	result, err := service.PluginList(map[string]any{
		"query":  "ocr",
		"kinds":  []any{"worker"},
		"health": []any{"healthy"},
		"page": map[string]any{
			"limit":  10,
			"offset": 0,
		},
	})
	if err != nil {
		t.Fatalf("plugin list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["plugin_id"] != "ocr" {
		t.Fatalf("expected plugin list to return filtered ocr plugin, got %+v", items)
	}
	runtime, ok := service.plugin.RuntimeState(plugin.RuntimeKindWorker, "ocr_worker")
	if !ok {
		t.Fatalf("expected ocr worker runtime to exist")
	}
	capabilities := items[0]["capabilities"].([]map[string]any)
	if len(capabilities) != len(runtime.Capabilities) {
		t.Fatalf("expected plugin list item to expose all registered capabilities, got %+v", capabilities)
	}
	for _, capability := range capabilities {
		toolName := capability["tool_name"].(string)
		tool, err := service.tools.Get(toolName)
		if err != nil {
			t.Fatalf("expected capability %q to resolve from registry: %v", toolName, err)
		}
		metadata := tool.Metadata()
		if capability["display_name"] != metadata.DisplayName || capability["description"] != metadata.Description || capability["source"] != string(metadata.Source) || capability["risk_hint"] != metadata.RiskHint {
			t.Fatalf("expected plugin list capability to mirror registry metadata for %q, got %+v", toolName, capability)
		}
	}
	if len(items[0]["runtimes"].([]map[string]any)) == 0 {
		t.Fatalf("expected plugin list item to expose runtimes, got %+v", items[0])
	}
	page := result["page"].(map[string]any)
	if page["total"] != 1 || page["has_more"] != false {
		t.Fatalf("expected plugin list page metadata, got %+v", page)
	}
}

func TestServicePluginDetailGetReturnsStructuredContracts(t *testing.T) {
	service := newTestService()
	service.plugin.MarkRuntimeHealthy(plugin.RuntimeKindWorker, "ocr_worker")

	result, err := service.PluginDetailGet(map[string]any{
		"plugin_id":       "ocr",
		"include_runtime": true,
		"include_metrics": true,
		"include_events":  true,
	})
	if err != nil {
		t.Fatalf("plugin detail get failed: %v", err)
	}
	pluginValue := result["plugin"].(map[string]any)
	if pluginValue["plugin_id"] != "ocr" || pluginValue["display_name"] != "OCR Worker" {
		t.Fatalf("expected structured plugin detail header, got %+v", pluginValue)
	}
	runtimes := result["runtimes"].([]map[string]any)
	if len(runtimes) != 1 || runtimes[0]["name"] != "ocr_worker" {
		t.Fatalf("expected plugin detail runtimes for ocr worker, got %+v", runtimes)
	}
	metrics := result["metrics"].([]map[string]any)
	if len(metrics) != 1 || metrics[0]["name"] != "ocr_worker" {
		t.Fatalf("expected plugin detail metrics for ocr worker, got %+v", metrics)
	}
	events := result["recent_events"].([]map[string]any)
	if len(events) == 0 {
		t.Fatalf("expected plugin detail events, got %+v", events)
	}
	runtime, ok := service.plugin.RuntimeState(plugin.RuntimeKindWorker, "ocr_worker")
	if !ok {
		t.Fatalf("expected ocr worker runtime to exist")
	}
	tools := result["tools"].([]map[string]any)
	if len(tools) != len(runtime.Capabilities) {
		t.Fatalf("expected plugin detail tools to match declared runtime capabilities, got %+v", tools)
	}
	for _, item := range tools {
		toolName := item["tool_name"].(string)
		tool, err := service.tools.Get(toolName)
		if err != nil {
			t.Fatalf("expected plugin detail tool %q to resolve from registry: %v", toolName, err)
		}
		metadata := tool.Metadata()
		if item["display_name"] != metadata.DisplayName || item["description"] != metadata.Description || item["source"] != string(metadata.Source) || item["risk_hint"] != metadata.RiskHint || item["timeout_sec"] != metadata.TimeoutSec || item["supports_dry_run"] != metadata.SupportsDryRun {
			t.Fatalf("expected plugin detail tool to mirror registry metadata for %q, got %+v", toolName, item)
		}
		inputContract := item["input_contract"].(map[string]any)
		if inputContract["schema_ref"] != metadata.InputSchemaRef {
			t.Fatalf("expected input contract schema ref to mirror registry metadata for %q, got %+v", toolName, inputContract)
		}
		outputContract := item["output_contract"].(map[string]any)
		if outputContract["schema_ref"] != metadata.OutputSchemaRef {
			t.Fatalf("expected output contract schema ref to mirror registry metadata for %q, got %+v", toolName, outputContract)
		}
		deliveryMapping := item["delivery_mapping"].(map[string]any)
		if deliveryMapping["emits_tool_call"] != true {
			t.Fatalf("expected delivery mapping to preserve tool call emission for %q, got %+v", toolName, deliveryMapping)
		}
	}
}

func TestServiceSnapshotUsesStablePrimaryWorker(t *testing.T) {
	service := newTestService()
	snapshot := service.Snapshot()
	if snapshot["primary_worker"] != "playwright_worker" {
		t.Fatalf("expected snapshot primary_worker to use stable declaration order, got %+v", snapshot)
	}
}

func TestDashboardModuleGetIncludesPluginRuntimeSummary(t *testing.T) {
	service := newTestService()
	service.plugin.MarkRuntimeHealthy(plugin.RuntimeKindWorker, "ocr_worker")
	service.plugin.MarkRuntimeUnavailable(plugin.RuntimeKindWorker, "media_worker", "missing")
	service.plugin.MarkRuntimeFailed(plugin.RuntimeKindSidecar, "playwright_sidecar", errors.New("boom"))
	result, err := service.DashboardModuleGet(map[string]any{"module": "mirror", "tab": "daily_summary"})
	if err != nil {
		t.Fatalf("dashboard module get failed: %v", err)
	}
	summary := result["summary"].(map[string]any)["plugin_runtime"].(map[string]any)
	if summary["healthy"] != 1 || summary["failed"] != 1 || summary["unavailable"] != 1 {
		t.Fatalf("expected dashboard module summary to expose plugin runtime counts, got %+v", summary)
	}
}

func TestDashboardModuleGetTasksIncludesFocusRuntimeSummary(t *testing.T) {
	service := newTestService()
	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_dashboard_tasks_runtime",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "dashboard task runtime summary",
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
	if _, ok := service.runEngine.AppendSteeringMessage(taskID, "Also include a one-line recap.", nil); !ok {
		t.Fatal("expected steering message to persist for dashboard focus task")
	}
	if _, ok := service.runEngine.RecordLoopLifecycle(taskID, "loop.retrying", "planner_timeout", map[string]any{"stop_reason": "planner_timeout"}); !ok {
		t.Fatal("expected loop lifecycle to update focus task")
	}

	moduleResult, err := service.DashboardModuleGet(map[string]any{
		"module": "tasks",
		"tab":    "focus",
	})
	if err != nil {
		t.Fatalf("dashboard module get failed: %v", err)
	}

	summary := moduleResult["summary"].(map[string]any)
	if summary["waiting_auth_tasks"] != 1 {
		t.Fatalf("expected one waiting_auth task in summary, got %+v", summary)
	}
	focusRuntimeSummary, ok := summary["focus_runtime_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected focus_runtime_summary map, got %+v", summary["focus_runtime_summary"])
	}
	if focusRuntimeSummary["latest_event_type"] != "loop.retrying" {
		t.Fatalf("expected latest_event_type loop.retrying, got %+v", focusRuntimeSummary)
	}
	if focusRuntimeSummary["active_steering_count"] != 1 {
		t.Fatalf("expected active steering count 1, got %+v", focusRuntimeSummary)
	}

	highlights := moduleResult["highlights"].([]string)
	joined := strings.Join(highlights, " ")
	if !strings.Contains(joined, "最近运行事件：loop.retrying") {
		t.Fatalf("expected runtime event highlight, got %+v", highlights)
	}
	if !strings.Contains(joined, "当前仍有 1 条追加要求待消费") {
		t.Fatalf("expected steering highlight, got %+v", highlights)
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

func TestServiceTaskEventsListSupportsRunAndTypeFilters(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "loop event filters")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime store to be wired")
	}
	if err := service.storage.LoopRuntimeStore().SaveEvents(context.Background(), []storage.EventRecord{
		{EventID: "evt_loop_filter_001", RunID: "run_loop_filter_a", TaskID: "task_loop_filter_001", StepID: "step_a", Type: "loop.round.started", Level: "info", PayloadJSON: `{}`, CreatedAt: "2026-04-17T10:00:00Z"},
		{EventID: "evt_loop_filter_002", RunID: "run_loop_filter_b", TaskID: "task_loop_filter_001", StepID: "step_b", Type: "loop.failed", Level: "error", PayloadJSON: `{}`, CreatedAt: "2026-04-17T10:01:00Z"},
	}); err != nil {
		t.Fatalf("save loop filter events failed: %v", err)
	}

	result, err := service.TaskEventsList(map[string]any{"task_id": "task_loop_filter_001", "run_id": "run_loop_filter_b", "type": "loop.failed", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task events list with filters failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["run_id"] != "run_loop_filter_b" || items[0]["type"] != "loop.failed" {
		t.Fatalf("expected filtered loop event, got %+v", items)
	}
}

func TestServiceTaskEventsListSupportsTimeWindowFilters(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "loop event time filters")
	if service.storage == nil || service.storage.LoopRuntimeStore() == nil {
		t.Fatal("expected loop runtime store to be wired")
	}
	if err := service.storage.LoopRuntimeStore().SaveEvents(context.Background(), []storage.EventRecord{
		{EventID: "evt_loop_time_001", RunID: "run_loop_time_a", TaskID: "task_loop_time_001", StepID: "step_a", Type: "loop.round.started", Level: "info", PayloadJSON: `{}`, CreatedAt: "2026-04-17T10:00:00Z"},
		{EventID: "evt_loop_time_002", RunID: "run_loop_time_b", TaskID: "task_loop_time_001", StepID: "step_b", Type: "loop.failed", Level: "error", PayloadJSON: `{}`, CreatedAt: "2026-04-17T10:05:00Z"},
	}); err != nil {
		t.Fatalf("save loop time filter events failed: %v", err)
	}

	result, err := service.TaskEventsList(map[string]any{
		"task_id":         "task_loop_time_001",
		"created_at_from": "2026-04-17T10:04:00Z",
		"created_at_to":   "2026-04-17T10:06:00Z",
		"limit":           20,
		"offset":          0,
	})
	if err != nil {
		t.Fatalf("task events list with time filters failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["run_id"] != "run_loop_time_b" {
		t.Fatalf("expected time-filtered loop event, got %+v", items)
	}
}

func TestServiceTaskToolCallsListReturnsPersistedToolCalls(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "tool call list")
	if service.storage == nil || service.storage.ToolCallStore() == nil {
		t.Fatal("expected tool call store to be wired")
	}
	err := service.storage.ToolCallStore().SaveToolCall(context.Background(), tools.ToolCallRecord{
		ToolCallID: "tool_call_list_001",
		RunID:      "run_tool_call_list_001",
		TaskID:     "task_tool_call_list_001",
		ToolName:   "read_file",
		Status:     tools.ToolCallStatusSucceeded,
		Input:      map[string]any{"path": "notes/source.txt"},
		Output:     map[string]any{"path": "notes/source.txt", "summary_output": map[string]any{"path": "notes/source.txt"}},
		DurationMS: 12,
	})
	if err != nil {
		t.Fatalf("save tool call failed: %v", err)
	}

	result, err := service.TaskToolCallsList(map[string]any{"task_id": "task_tool_call_list_001", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task tool calls list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["tool_name"] != "read_file" {
		t.Fatalf("expected persisted read_file tool call, got %+v", items)
	}
	page := result["page"].(map[string]any)
	if page["total"] != 1 {
		t.Fatalf("expected total 1, got %+v", page)
	}
}

func TestServiceTaskToolCallsListSupportsRunFilter(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "tool call list filters")
	if service.storage == nil || service.storage.ToolCallStore() == nil {
		t.Fatal("expected tool call store to be wired")
	}
	for _, record := range []tools.ToolCallRecord{
		{ToolCallID: "tool_call_filter_001", RunID: "run_filter_a", TaskID: "task_tool_call_filter_001", ToolName: "read_file", Status: tools.ToolCallStatusSucceeded, DurationMS: 5},
		{ToolCallID: "tool_call_filter_002", RunID: "run_filter_b", TaskID: "task_tool_call_filter_001", ToolName: "read_file", Status: tools.ToolCallStatusFailed, DurationMS: 7},
	} {
		if err := service.storage.ToolCallStore().SaveToolCall(context.Background(), record); err != nil {
			t.Fatalf("save filtered tool call failed: %v", err)
		}
	}

	result, err := service.TaskToolCallsList(map[string]any{"task_id": "task_tool_call_filter_001", "run_id": "run_filter_b", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task tool calls list with run filter failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["run_id"] != "run_filter_b" || items[0]["status"] != string(tools.ToolCallStatusFailed) {
		t.Fatalf("expected filtered tool call, got %+v", items)
	}
}

func TestServiceTaskToolCallsListNormalizesProtocolStatuses(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "tool call list statuses")
	if service.storage == nil || service.storage.ToolCallStore() == nil {
		t.Fatal("expected tool call store to be wired")
	}
	if err := service.storage.ToolCallStore().SaveToolCall(context.Background(), tools.ToolCallRecord{
		ToolCallID: "tool_call_status_001",
		RunID:      "run_tool_call_status_001",
		TaskID:     "task_tool_call_status_001",
		ToolName:   "read_file",
		Status:     tools.ToolCallStatusStarted,
		DurationMS: 3,
	}); err != nil {
		t.Fatalf("save tool call status failed: %v", err)
	}

	result, err := service.TaskToolCallsList(map[string]any{"task_id": "task_tool_call_status_001", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task tool calls list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["status"] != "running" {
		t.Fatalf("expected outward running status, got %+v", items)
	}
}

func TestServiceTaskToolCallsListFallsBackToCompatibilityLatestToolCall(t *testing.T) {
	service := newTestService()
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         "sess_tool_call_compat",
		Title:             "compat tool call",
		SourceType:        "floating_ball",
		Status:            "processing",
		Intent:            map[string]any{"name": "read_file", "arguments": map[string]any{"path": "notes/source.txt"}},
		PreferredDelivery: "bubble",
		FallbackDelivery:  "bubble",
		CurrentStep:       "generate_output",
		RiskLevel:         "green",
		Timeline:          initialTimeline("processing", "generate_output"),
	})
	if _, ok := service.runEngine.RecordToolCallLifecycle(task.TaskID, "read_file", "succeeded", map[string]any{"path": "notes/source.txt"}, map[string]any{"path": "notes/source.txt", "content_preview": "compat preview"}, 14, nil); !ok {
		t.Fatal("expected compatibility tool call to be recorded")
	}

	result, err := service.TaskToolCallsList(map[string]any{"task_id": task.TaskID, "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("task tool calls list failed: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["tool_name"] != "read_file" || mapValue(items[0], "output")["content_preview"] != "compat preview" {
		t.Fatalf("expected compatibility fallback tool call, got %+v", items)
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

	service, _ := newTestServiceWithExecutionAndPlaywright(t, "unused", platform.LocalExecutionBackend{}, nil, localHTTPPlaywrightClient{})
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

func TestLocalHTTPPlaywrightClientReadPageFetchesLocalHTML(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Local Acceptance Page</title></head><body><p>Local acceptance page verifies direct local http page reads.</p></body></html>`))
	})}
	defer server.Close()
	go func() {
		_ = server.Serve(listener)
	}()

	client := localHTTPPlaywrightClient{}
	result, err := client.ReadPage(context.Background(), "http://"+listener.Addr().String())
	if err != nil {
		t.Fatalf("ReadPage returned error: %v", err)
	}
	if result.Title != "Local Acceptance Page" {
		t.Fatalf("expected local title to be parsed, got %+v", result)
	}
	if !strings.Contains(result.TextContent, "direct local http page reads") {
		t.Fatalf("expected local page content to be captured, got %+v", result)
	}
	if result.Source != "local_http_playwright_client" {
		t.Fatalf("expected local http source marker, got %+v", result)
	}
}

func modelConfig() serviceconfig.ModelConfig {
	return serviceconfig.ModelConfig{
		Provider: "openai_responses",
		ModelID:  "gpt-5.4",
		Endpoint: "https://api.openai.com/v1/responses",
	}
}
