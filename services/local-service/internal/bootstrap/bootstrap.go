// 该文件负责本地服务依赖装配与启动初始化。
package bootstrap

import (
	"context"
	"os"
	"strings"

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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// App 定义当前模块的数据结构。
type App struct {
	server  *rpc.Server
	storage *storage.Service
}

// New 创建并返回当前能力。
func New(cfg config.Config) (*App, error) {
	pathPolicy, err := platform.NewLocalPathPolicy(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	_ = audit.NewService()
	_ = checkpoint.NewService()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	_ = platform.LocalExecutionBackend{}

	modelService := model.NewService(cfg.Model)
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey != "" {
		if configuredModelService, err := model.NewServiceFromConfig(model.ServiceConfig{
			ModelConfig: cfg.Model,
			APIKey:      apiKey,
		}); err == nil {
			modelService = configuredModelService
		}
	}

	deliveryService := delivery.NewService()
	toolRegistry := tools.NewRegistry()
	pluginService := plugin.NewService()
	executionService := execution.NewService(fileSystem, modelService, deliveryService, toolRegistry, pluginService)
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
	).WithExecutor(executionService)

	return &App{server: rpc.NewServer(cfg.RPC, orchestratorService), storage: storageService}, nil
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
