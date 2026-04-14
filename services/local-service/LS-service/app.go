package main

import (
	"context"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

type App struct {
	server       *rpc.Server
	storage      *storage.Service
	toolRegistry *tools.Registry
	toolExecutor *tools.ToolExecutor
	playwright   *sidecarclient.PlaywrightSidecarRuntime
}

func newApp(cfg serviceconfig.Config, externalCfg externalModelConfig) (*App, error) {
	pathPolicy, err := platform.NewLocalPathPolicy(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	storageService := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	auditService := audit.NewService(storageService.AuditWriter())
	checkpointService := checkpoint.NewService(storageService.RecoveryPointWriter())
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executionBackend := platform.LocalExecutionBackend{}
	osCapability := platform.NewLocalOSCapabilityAdapter()
	playwrightRuntime, err := sidecarclient.NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := playwrightRuntime.Start(); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	playwrightClient := playwrightRuntime.Client()
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		_ = storageService.Close()
		_ = playwrightRuntime.Stop()
		return nil, err
	}
	if err := sidecarclient.RegisterPlaywrightTools(toolRegistry); err != nil {
		_ = storageService.Close()
		_ = playwrightRuntime.Stop()
		return nil, err
	}
	toolExecutor := tools.NewToolExecutor(
		toolRegistry,
		tools.WithToolCallRecorder(tools.NewToolCallRecorder(storageService.ToolCallSink())),
	)

	modelClient, err := newChatCompletionsClient(externalCfg)
	if err != nil {
		_ = storageService.Close()
		_ = playwrightRuntime.Stop()
		return nil, err
	}
	modelService := model.NewService(serviceconfig.ModelConfig{
		Provider:            lsServiceProvider,
		ModelID:             externalCfg.Model,
		Endpoint:            externalCfg.BaseURL,
		APIKey:              externalCfg.APIKey,
		SingleTaskLimit:     cfg.Model.SingleTaskLimit,
		DailyLimit:          cfg.Model.DailyLimit,
		BudgetAutoDowngrade: cfg.Model.BudgetAutoDowngrade,
	}, modelClient)

	deliveryService := delivery.NewService()
	pluginService := plugin.NewService()
	executionService := execution.NewService(fileSystem, executionBackend, playwrightClient, modelService, auditService, checkpointService, deliveryService, toolRegistry, toolExecutor, pluginService)
	inspectorService := taskinspector.NewService(fileSystem)
	runEngine, err := runengine.NewEngineWithStore(storageService.TaskRunStore())
	if err != nil {
		_ = storageService.Close()
		_ = playwrightRuntime.Stop()
		return nil, err
	}

	orchestratorService := orchestrator.NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runEngine,
		deliveryService,
		memory.NewServiceFromStorage(storageService.MemoryStore(), storageService.Capabilities().MemoryRetrievalBackend),
		risk.NewService(),
		modelService,
		toolRegistry,
		pluginService,
	).WithAudit(auditService).WithExecutor(executionService).WithStorage(storageService).WithTaskInspector(inspectorService)

	return &App{
		server:       rpc.NewServer(cfg.RPC, orchestratorService),
		storage:      storageService,
		toolRegistry: toolRegistry,
		toolExecutor: toolExecutor,
		playwright:   playwrightRuntime,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	return a.server.Start(ctx)
}

func (a *App) Close() error {
	if a.playwright != nil {
		_ = a.playwright.Stop()
	}
	if a.storage == nil {
		return nil
	}
	return a.storage.Close()
}
