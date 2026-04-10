// 该文件负责本地服务依赖装配与启动初始化。
package bootstrap

import (
	"context"

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
)

// App 定义当前模块的数据结构。
type App struct {
	server       *rpc.Server
	storage      *storage.Service
	toolRegistry *tools.Registry
	toolExecutor *tools.ToolExecutor
}

// New 创建并返回当前能力。
func New(cfg config.Config) (*App, error) {
	pathPolicy, err := platform.NewLocalPathPolicy(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	auditService := audit.NewService()
	checkpointService := checkpoint.NewService()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executionBackend := platform.LocalExecutionBackend{}
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		return nil, err
	}
	toolExecutor := tools.NewToolExecutor(
		toolRegistry,
		tools.WithToolCallRecorder(tools.NewToolCallRecorder(storageService.ToolCallSink())),
	)

	modelService, err := model.NewServiceFromConfig(model.ServiceConfig{
		ModelConfig: cfg.Model,
	})
	if err != nil {
		_ = storageService.Close()
		return nil, err
	}

	deliveryService := delivery.NewService()
	pluginService := plugin.NewService()
	executionService := execution.NewService(fileSystem, executionBackend, modelService, auditService, checkpointService, deliveryService, toolRegistry, toolExecutor, pluginService)
	inspectorService := taskinspector.NewService(fileSystem)
	runEngine, err := runengine.NewEngineWithStore(storageService.TaskRunStore())
	if err != nil {
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
	).WithAudit(auditService).WithExecutor(executionService).WithTaskInspector(inspectorService)

	return &App{
		server:       rpc.NewServer(cfg.RPC, orchestratorService),
		storage:      storageService,
		toolRegistry: toolRegistry,
		toolExecutor: toolExecutor,
	}, nil
}

// Start 启动当前能力。
func (a *App) Start(ctx context.Context) error {
	return a.server.Start(ctx)
}

func (a *App) Close() error {
	if a.storage == nil {
		return nil
	}

	return a.storage.Close()
}
