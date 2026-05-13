package bootstrap

import (
	"context"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/rpc"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

type coreDeps struct {
	storageService             *storage.Service
	auditService               *audit.Service
	checkpointService          *checkpoint.Service
	fileSystem                 platform.FileSystemAdapter
	toolFileSystem             platform.FileSystemAdapter
	executionBackend           tools.ExecutionCapability
	osCapability               platform.OSCapabilityAdapter
	pluginService              *plugin.Service
	resolvedModelConfig        config.ModelConfig
	placeholderModelConfig     config.ModelConfig
	persistedModelRouteChanged bool
}

type runtimeDeps struct {
	sidecars     sidecarRuntimes
	screenClient tools.ScreenCaptureClient
	toolRegistry *tools.Registry
	toolExecutor *tools.ToolExecutor
	modelService *model.Service
}

type serviceDeps struct {
	deliveryService     *delivery.Service
	traceEvalService    *traceeval.Service
	executionService    *execution.Service
	inspectorService    *taskinspector.Service
	runEngine           *runengine.Engine
	orchestratorService *orchestrator.Service
}

// buildCoreDeps assembles storage, filesystem, and governance dependencies
// that every later bootstrap stage relies on.
func buildCoreDeps(cfg config.Config) (coreDeps, error) {
	pathPolicy, err := newLocalPathPolicyForBootstrap(cfg.WorkspaceRoot)
	if err != nil {
		return coreDeps{}, err
	}
	toolPathPolicy, err := platform.NewLocalToolPathPolicy(cfg.WorkspaceRoot)
	if err != nil {
		return coreDeps{}, err
	}

	storageService := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	resolvedModelConfig, placeholderModelConfig, persistedModelRouteChanged, err := loadBootstrapModelConfig(cfg.Model, storageService.SettingsStore())
	if err != nil {
		_ = storageService.Close()
		return coreDeps{}, err
	}

	auditService := audit.NewService(storageService.AuditWriter())
	checkpointService := checkpoint.NewService(storageService.RecoveryPointWriter())
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	toolFileSystem := platform.NewLocalFileSystemAdapter(toolPathPolicy)
	executionBackend := platform.NewControlledExecutionBackend(cfg.WorkspaceRoot)
	osCapability := platform.NewLocalOSCapabilityAdapter()
	pluginService := plugin.NewService()
	if err := storageService.EnsureBuiltinExecutionAssets(context.Background()); err != nil {
		_ = storageService.Close()
		return coreDeps{}, err
	}
	if err := persistPluginManifests(context.Background(), storageService, pluginService); err != nil {
		_ = storageService.Close()
		return coreDeps{}, err
	}

	return coreDeps{
		storageService:             storageService,
		auditService:               auditService,
		checkpointService:          checkpointService,
		fileSystem:                 fileSystem,
		toolFileSystem:             toolFileSystem,
		executionBackend:           executionBackend,
		osCapability:               osCapability,
		pluginService:              pluginService,
		resolvedModelConfig:        resolvedModelConfig,
		placeholderModelConfig:     placeholderModelConfig,
		persistedModelRouteChanged: persistedModelRouteChanged,
	}, nil
}

// buildRuntimes assembles tool and model runtimes after core persistence is
// available, keeping external worker startup separate from service wiring.
func buildRuntimes(core coreDeps) (runtimeDeps, error) {
	sidecars := buildSidecarRuntimes(core.pluginService, core.osCapability)
	toolRegistry := tools.NewRegistry()
	if err := registerBuiltinToolsForBootstrap(toolRegistry); err != nil {
		return runtimeDeps{}, err
	}
	if err := registerPlaywrightToolsForBootstrap(toolRegistry); err != nil {
		return runtimeDeps{}, err
	}
	if err := registerOCRToolsForBootstrap(toolRegistry); err != nil {
		return runtimeDeps{}, err
	}
	if err := registerMediaToolsForBootstrap(toolRegistry); err != nil {
		return runtimeDeps{}, err
	}

	toolExecutor := tools.NewToolExecutor(
		toolRegistry,
		tools.WithToolCallRecorder(tools.NewToolCallRecorder(core.storageService.ToolCallSink())),
	)

	modelService, err := newModelServiceFromConfigForBootstrap(model.ServiceConfig{
		ModelConfig:  core.resolvedModelConfig,
		SecretSource: model.NewStaticSecretSource(core.storageService),
	})
	if err != nil {
		if shouldFallbackBootstrapModelService(err, core.persistedModelRouteChanged) {
			modelService = model.NewService(core.placeholderModelConfig)
		} else {
			return runtimeDeps{}, err
		}
	}

	return runtimeDeps{
		sidecars:     sidecars,
		screenClient: sidecarclient.NewLocalScreenCaptureClient(core.fileSystem),
		toolRegistry: toolRegistry,
		toolExecutor: toolExecutor,
		modelService: modelService,
	}, nil
}

// buildServices wires the task-centric services once core persistence and
// external runtimes are stable.
func buildServices(core coreDeps, runtimes runtimeDeps) (serviceDeps, error) {
	deliveryService := delivery.NewService()
	traceEvalService := traceeval.NewService(core.storageService.TraceStore(), core.storageService.EvalStore())
	executionService := execution.NewService(
		core.fileSystem,
		core.executionBackend,
		runtimes.sidecars.playwright.Client(),
		runtimes.sidecars.ocr.Client(),
		runtimes.sidecars.media.Client(),
		runtimes.screenClient,
		runtimes.modelService,
		core.auditService,
		core.checkpointService,
		deliveryService,
		runtimes.toolRegistry,
		runtimes.toolExecutor,
		core.pluginService,
	).WithArtifactStore(core.storageService.ArtifactStore()).
		WithToolPlatform(core.toolFileSystem).
		WithLoopRuntimeStore(core.storageService.LoopRuntimeStore()).
		WithExtensionAssetCatalog(core.storageService)
	inspectorService := taskinspector.NewService(core.fileSystem)

	runEngine, err := runengine.NewEngineWithStore(core.storageService.TaskRunStore())
	if err != nil {
		return serviceDeps{}, err
	}
	if err := runEngine.WithTodoStore(core.storageService.TodoStore()); err != nil {
		return serviceDeps{}, err
	}
	if err := runEngine.WithSettingsStore(core.storageService.SettingsStore()); err != nil {
		return serviceDeps{}, err
	}
	if err := runEngine.WithSessionStore(core.storageService.SessionStore()); err != nil {
		return serviceDeps{}, err
	}

	orchestratorService, err := orchestrator.NewService(orchestrator.Deps{
		Context:   taskcontext.NewCaptureService(),
		Intent:    intent.NewService(),
		RunEngine: runEngine,
		Delivery:  deliveryService,
		Memory:    memory.NewServiceFromStorage(core.storageService.MemoryStore(), core.storageService.Capabilities().MemoryRetrievalBackend),
		Risk:      risk.NewService(),
		Model:     runtimes.modelService,
		Tools:     runtimes.toolRegistry,
		Plugin:    core.pluginService,
		Audit:     core.auditService,
		TraceEval: traceEvalService,
		Executor:  executionService,
		Inspector: inspectorService,
		Storage:   core.storageService,
	})
	if err != nil {
		return serviceDeps{}, err
	}

	return serviceDeps{
		deliveryService:     deliveryService,
		traceEvalService:    traceEvalService,
		executionService:    executionService,
		inspectorService:    inspectorService,
		runEngine:           runEngine,
		orchestratorService: orchestratorService,
	}, nil
}

func newApp(cfg config.Config, core coreDeps, runtimes runtimeDeps, services serviceDeps) *App {
	return &App{
		server:       rpc.NewServer(cfg.RPC, services.orchestratorService),
		storage:      core.storageService,
		toolRegistry: runtimes.toolRegistry,
		toolExecutor: runtimes.toolExecutor,
		playwright:   runtimes.sidecars.playwright,
		ocr:          runtimes.sidecars.ocr,
		media:        runtimes.sidecars.media,
	}
}
