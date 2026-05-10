package rpc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

type stubLoopModelClient struct {
	toolResult       model.ToolCallResult
	generateToolWait chan struct{}
	generateToolSeen chan struct{}
}

// selectiveWaitLoopModelClient only applies the blocking tool-call gate to one
// task so stream-serialization tests can distinguish per-task locking from
// unrelated concurrent requests.
type selectiveWaitLoopModelClient struct {
	stubLoopModelClient
	blockedTaskID string
}

func (s *stubLoopModelClient) GenerateText(_ context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	return model.GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  "req_loop_text",
		Provider:   "openai_responses",
		ModelID:    "gpt-5.4",
		OutputText: "loop fallback output",
	}, nil
}

func (s *stubLoopModelClient) GenerateToolCalls(_ context.Context, request model.ToolCallRequest) (model.ToolCallResult, error) {
	if s.generateToolSeen != nil {
		select {
		case <-s.generateToolSeen:
		default:
			close(s.generateToolSeen)
		}
	}
	if s.generateToolWait != nil {
		<-s.generateToolWait
	}
	result := s.toolResult
	if strings.TrimSpace(result.OutputText) == "" && len(result.ToolCalls) == 0 {
		result.OutputText = request.Input
	}
	if result.RequestID == "" {
		result.RequestID = "req_loop_tools"
	}
	if result.Provider == "" {
		result.Provider = "openai_responses"
	}
	if result.ModelID == "" {
		result.ModelID = "gpt-5.4"
	}
	return result, nil
}

func (s *selectiveWaitLoopModelClient) GenerateToolCalls(ctx context.Context, request model.ToolCallRequest) (model.ToolCallResult, error) {
	if strings.TrimSpace(s.blockedTaskID) == "" || request.TaskID == s.blockedTaskID {
		return s.stubLoopModelClient.GenerateToolCalls(ctx, request)
	}

	result := s.toolResult
	if strings.TrimSpace(result.OutputText) == "" && len(result.ToolCalls) == 0 {
		result.OutputText = request.Input
	}
	if result.RequestID == "" {
		result.RequestID = "req_loop_tools"
	}
	if result.Provider == "" {
		result.Provider = "openai_responses"
	}
	if result.ModelID == "" {
		result.ModelID = "gpt-5.4"
	}
	return result, nil
}

type testStorageAdapter struct {
	databasePath string
}

type stubExecutionCapability struct {
	result tools.CommandExecutionResult
	err    error
}

func (s stubExecutionCapability) RunCommand(_ context.Context, _ string, _ []string, _ string) (tools.CommandExecutionResult, error) {
	if s.err != nil {
		return tools.CommandExecutionResult{}, s.err
	}
	return s.result, nil
}

func (a testStorageAdapter) DatabasePath() string {
	return a.databasePath
}

func (a testStorageAdapter) SecretStorePath() string {
	if a.databasePath == "" {
		return ""
	}
	return a.databasePath + ".stronghold"
}

func newTestServer() *Server {
	server, _, _ := newTestServerWithDependencies(nil, nil, nil)
	return server
}

func newTestServerWithModelClient(client model.Client) *Server {
	server, _, _ := newTestServerWithDependencies(client, nil, nil)
	return server
}

func newTestServerWithStorage(storageService *storage.Service) *Server {
	server, _, _ := newTestServerWithDependencies(nil, storageService, nil)
	return server
}

func newTestServerWithTaskInspector(inspectorService *taskinspector.Service) *Server {
	server, _, _ := newTestServerWithDependencies(nil, nil, inspectorService)
	return server
}

func newTestServerWithDependencies(client model.Client, storageService *storage.Service, inspectorService *taskinspector.Service) (*Server, *tools.Registry, *plugin.Service) {
	toolRegistry := tools.NewRegistry()
	_ = builtin.RegisterBuiltinTools(toolRegistry)
	_ = sidecarclient.RegisterPlaywrightTools(toolRegistry)
	_ = sidecarclient.RegisterOCRTools(toolRegistry)
	_ = sidecarclient.RegisterMediaTools(toolRegistry)
	toolExecutor := tools.NewToolExecutor(toolRegistry)
	pathPolicy, _ := platform.NewLocalPathPolicy(filepath.Join("workspace", "rpc-test"))
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	pluginService := plugin.NewService()
	executionService := execution.NewService(
		fileSystem,
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		model.NewService(serviceconfig.ModelConfig{Provider: "openai_responses", ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses"}, client),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		pluginService,
	)
	orch, err := orchestrator.NewService(orchestrator.Deps{
		Context:   taskcontext.NewCaptureService(),
		Intent:    intent.NewService(),
		RunEngine: runengine.NewEngine(),
		Delivery:  delivery.NewService(),
		Memory:    memory.NewService(),
		Risk:      risk.NewService(),
		Model: model.NewService(serviceconfig.ModelConfig{
			Provider: "openai_responses",
			ModelID:  "gpt-5.4",
			Endpoint: "https://api.openai.com/v1/responses",
		}),
		Tools:     toolRegistry,
		Plugin:    pluginService,
		Executor:  executionService,
		Storage:   storageService,
		Inspector: inspectorService,
	})
	if err != nil {
		panic(err)
	}

	server := NewServer(serviceconfig.RPCConfig{
		Transport:        "named_pipe",
		NamedPipeName:    `\\.\pipe\cialloclaw-rpc-test`,
		DebugHTTPAddress: ":0",
	}, orch)
	server.now = func() time.Time {
		return time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	}
	return server, toolRegistry, pluginService
}

func rpcRequestMeta(traceID string) map[string]any {
	return map[string]any{
		"trace_id":    traceID,
		"client_time": "2026-05-10T00:00:00Z",
	}
}

func startTaskForTest(s *orchestrator.Service, params map[string]any) (map[string]any, error) {
	response, err := s.StartTask(orchestrator.StartTaskRequestFromParams(params))
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func mustMarshal(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request params: %v", err)
	}
	return encoded
}

func numericValue(t *testing.T, value any) int {
	t.Helper()
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		t.Fatalf("expected numeric value, got %#v", value)
		return 0
	}
}
