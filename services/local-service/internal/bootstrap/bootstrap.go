// 该文件负责本地服务依赖装配与启动初始化。
package bootstrap

import (
	"context"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
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

type App struct {
	server *rpc.Server
}

func New(cfg config.Config) (*App, error) {
	pathPolicy, err := platform.NewLocalPathPolicy(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	_ = audit.NewService()
	_ = checkpoint.NewService()
	_ = storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	_ = platform.NewLocalFileSystemAdapter(pathPolicy)
	_ = platform.LocalExecutionBackend{}

	orchestratorService := orchestrator.NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(cfg.Model),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	return &App{server: rpc.NewServer(cfg.RPC, orchestratorService)}, nil
}

func (a *App) Start(ctx context.Context) error {
	return a.server.Start(ctx)
}
