// 该文件负责本地服务依赖装配与启动初始化。
package bootstrap

import (
	"context"
	"errors"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

// App 定义当前模块的数据结构。
type App struct {
	server       *rpc.Server
	storage      *storage.Service
	toolRegistry *tools.Registry
	toolExecutor *tools.ToolExecutor
	playwright   *sidecarclient.PlaywrightSidecarRuntime
	ocr          *sidecarclient.OCRWorkerRuntime
	media        *sidecarclient.MediaWorkerRuntime
}

// New 创建并返回当前能力。
func New(cfg config.Config) (*App, error) {
	pathPolicy, err := platform.NewLocalPathPolicy(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	storageService := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	auditService := audit.NewService(storageService.AuditWriter())
	checkpointService := checkpoint.NewService(storageService.RecoveryPointWriter())
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executionBackend := platform.NewControlledExecutionBackend(cfg.WorkspaceRoot)
	osCapability := platform.NewLocalOSCapabilityAdapter()
	playwrightRuntime, err := sidecarclient.NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		playwrightRuntime = sidecarclient.NewUnavailablePlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	} else {
		if err := playwrightRuntime.Start(); err != nil {
			playwrightRuntime = sidecarclient.NewUnavailablePlaywrightSidecarRuntime(plugin.NewService(), osCapability)
		}
	}
	playwrightClient := playwrightRuntime.Client()
	ocrRuntime, err := sidecarclient.NewOCRWorkerRuntime(osCapability)
	if err != nil {
		ocrRuntime = sidecarclient.NewUnavailableOCRWorkerRuntime(osCapability)
	} else {
		if err := ocrRuntime.Start(); err != nil {
			ocrRuntime = sidecarclient.NewUnavailableOCRWorkerRuntime(osCapability)
		}
	}
	ocrClient := ocrRuntime.Client()
	mediaRuntime, err := sidecarclient.NewMediaWorkerRuntime(osCapability)
	if err != nil {
		mediaRuntime = sidecarclient.NewUnavailableMediaWorkerRuntime(osCapability)
	} else {
		if err := mediaRuntime.Start(); err != nil {
			mediaRuntime = sidecarclient.NewUnavailableMediaWorkerRuntime(osCapability)
		}
	}
	mediaClient := mediaRuntime.Client()
	screenClient := sidecarclient.NewLocalScreenCaptureClient(fileSystem)
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		return nil, err
	}
	if err := sidecarclient.RegisterPlaywrightTools(toolRegistry); err != nil {
		return nil, err
	}
	if err := sidecarclient.RegisterOCRTools(toolRegistry); err != nil {
		return nil, err
	}
	if err := sidecarclient.RegisterMediaTools(toolRegistry); err != nil {
		return nil, err
	}
	toolExecutor := tools.NewToolExecutor(
		toolRegistry,
		tools.WithToolCallRecorder(tools.NewToolCallRecorder(storageService.ToolCallSink())),
	)

	modelService, err := model.NewServiceFromConfig(model.ServiceConfig{
		ModelConfig:  cfg.Model,
		SecretSource: model.NewStaticSecretSource(storageService),
	})
	if err != nil {
		if errors.Is(err, model.ErrSecretSourceFailed) && (errors.Is(err, model.ErrSecretNotFound) || errors.Is(err, storage.ErrSecretNotFound)) {
			modelService = model.NewService(cfg.Model)
		} else {
			_ = storageService.Close()
			return nil, err
		}
	}

	deliveryService := delivery.NewService()
	pluginService := plugin.NewService()
	traceEvalService := traceeval.NewService(storageService.TraceStore(), storageService.EvalStore())
	executionService := execution.NewService(fileSystem, executionBackend, playwrightClient, ocrClient, mediaClient, screenClient, modelService, auditService, checkpointService, deliveryService, toolRegistry, toolExecutor, pluginService).
		WithArtifactStore(storageService.ArtifactStore()).
		WithLoopRuntimeStore(storageService.LoopRuntimeStore())
	inspectorService := taskinspector.NewService(fileSystem)
	runEngine, err := runengine.NewEngineWithStore(storageService.TaskRunStore())
	if err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := runEngine.WithTodoStore(storageService.TodoStore()); err != nil {
		_ = storageService.Close()
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
	).WithAudit(auditService).WithExecutor(executionService).WithStorage(storageService).WithTaskInspector(inspectorService).WithTraceEval(traceEvalService)

	return &App{
		server:       rpc.NewServer(cfg.RPC, orchestratorService),
		storage:      storageService,
		toolRegistry: toolRegistry,
		toolExecutor: toolExecutor,
		playwright:   playwrightRuntime,
		ocr:          ocrRuntime,
		media:        mediaRuntime,
	}, nil
}

// Start 启动当前能力。
func (a *App) Start(ctx context.Context) error {
	return a.server.Start(ctx)
}

func (a *App) Close() error {
	if a.playwright != nil {
		_ = a.playwright.Stop()
	}
	if a.ocr != nil {
		_ = a.ocr.Stop()
	}
	if a.media != nil {
		_ = a.media.Stop()
	}
	if a.storage == nil {
		return nil
	}

	return a.storage.Close()
}
