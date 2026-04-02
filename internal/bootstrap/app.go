package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"cialloclaw/internal/action/executor"
	"cialloclaw/internal/action/tooling"
	"cialloclaw/internal/cognition/agents"
	"cialloclaw/internal/cognition/processors"
	"cialloclaw/internal/config"
	"cialloclaw/internal/llm"
	memorymanager "cialloclaw/internal/memory"
	"cialloclaw/internal/perception/clipboard"
	todosource "cialloclaw/internal/perception/todo"
	"cialloclaw/internal/protocol"
	"cialloclaw/internal/runtime/eventbus"
	"cialloclaw/internal/runtime/id"
	"cialloclaw/internal/state/approval"
	logstate "cialloclaw/internal/state/log"
	"cialloclaw/internal/state/memory"
	"cialloclaw/internal/state/session"
	"cialloclaw/internal/state/task"
	"cialloclaw/internal/ui/httpui"
)

type App struct {
	root         string
	configMu     sync.RWMutex
	Config       config.Config
	Logger       *slog.Logger
	Bus          eventbus.Bus
	SessionRepo  session.Repository
	TaskRepo     task.Repository
	ApprovalRepo approval.Repository
	MemoryRepo   memory.Repository
	LogRepo      logstate.Repository
	Tools        *tooling.Registry
	Agents       *agents.Registry
	Server       *httpui.Server
	Clipboard    *clipboard.Source
	TodoSource   *todosource.Source

	sessionID string
}

func NewApp(root string) (*App, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app := &App{
		root:         root,
		Config:       cfg,
		Logger:       logger,
		Bus:          eventbus.New(),
		SessionRepo:  session.NewFileRepository(cfg.DataDir),
		TaskRepo:     task.NewFileRepository(cfg.DataDir),
		ApprovalRepo: approval.NewFileRepository(cfg.DataDir),
		MemoryRepo:   memory.NewFileRepository(cfg.DataDir),
		LogRepo:      logstate.NewFileRepository(cfg.DataDir),
		Tools:        tooling.NewRegistry(),
		Agents:       agents.NewRegistry(),
	}
	if err := app.registerTools(); err != nil {
		return nil, err
	}
	if err := app.registerAgents(); err != nil {
		return nil, err
	}
	if err := app.ensureSession(context.Background()); err != nil {
		return nil, err
	}
	app.registerSubscribers()
	app.Server = httpui.New(cfg.HTTPAddr, app.Bus, app.CurrentSessionID, app.CurrentConfig, app.UpdateLLMConfig, app.SessionRepo, app.TaskRepo, app.ApprovalRepo, app.MemoryRepo, app.LogRepo, app.Tools, cfg.TodoRoots)
	app.Clipboard = clipboard.New(app.Bus, app.CurrentSessionID, cfg.ClipboardPollInterval)
	app.TodoSource = todosource.New(app.Bus, app.CurrentSessionID, app.MemoryRepo, cfg.TodoRoots, cfg.TodoPollInterval)
	return app, nil
}

func (a *App) Start(ctx context.Context) error {
	if err := a.Server.Start(ctx); err != nil {
		return err
	}
	if err := a.Clipboard.Start(ctx); err != nil {
		return err
	}
	if err := a.TodoSource.Start(ctx); err != nil {
		return err
	}
	_ = a.Bus.Publish(ctx, protocol.NewEvent(protocol.EventTypeServiceStarted, "runtime.bootstrap", a.CurrentSessionID(), protocol.PriorityHigh, map[string]any{
		"http_addr": a.Config.HTTPAddr,
	}))
	a.Logger.Info("cialloclaw started", "addr", a.Config.HTTPAddr)
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	_ = a.TodoSource.Stop(ctx)
	_ = a.Clipboard.Stop(ctx)
	_ = a.Server.Stop(ctx)
	_ = a.Bus.Publish(ctx, protocol.NewEvent(protocol.EventTypeServiceStopped, "runtime.shutdown", a.CurrentSessionID(), protocol.PriorityHigh, nil))
	sessionItem, err := a.SessionRepo.GetByID(ctx, a.CurrentSessionID())
	if err == nil && sessionItem != nil {
		now := time.Now()
		sessionItem.Status = session.StatusClosed
		sessionItem.EndedAt = &now
		sessionItem.LastActiveAt = now
		_ = a.SessionRepo.Update(ctx, sessionItem)
	}
	return nil
}

func (a *App) CurrentSessionID() string {
	return a.sessionID
}

func (a *App) CurrentConfig() config.Config {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.Config
}

func (a *App) CurrentLLMConfig() config.LLMConfig {
	return a.CurrentConfig().LLM
}

func (a *App) UpdateLLMConfig(llmConfig config.LLMConfig) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	next := a.Config
	next.LLM.BaseURL = llmConfig.BaseURL
	next.LLM.Model = llmConfig.Model
	if llmConfig.APIKey != "" {
		next.LLM.APIKey = llmConfig.APIKey
	}
	if err := config.Save(a.root, next); err != nil {
		return err
	}
	a.Config = next
	return nil
}

func (a *App) registerTools() error {
	if err := a.Tools.Register(tooling.NewContentTool(llm.NewClient(a.CurrentLLMConfig))); err != nil {
		return err
	}
	return a.Tools.Register(tooling.NewTodoScanTool())
}

func (a *App) registerAgents() error {
	return a.Agents.Register(agents.NewAssistant())
}

func (a *App) ensureSession(ctx context.Context) error {
	active, err := a.SessionRepo.ListActive(ctx)
	if err != nil {
		return err
	}
	var current *session.Session
	if len(active) > 0 {
		current = active[0]
		current.Status = session.StatusActive
		current.LastActiveAt = time.Now()
		if err := a.SessionRepo.Update(ctx, current); err != nil {
			return err
		}
	} else {
		hostname, _ := os.Hostname()
		now := time.Now()
		current = &session.Session{
			ID:              id.New("session"),
			UserID:          "local-user",
			DeviceID:        hostname,
			Status:          session.StatusActive,
			StartedAt:       now,
			LastActiveAt:    now,
			WorkingMemoryID: id.New("wm"),
			TraceRootID:     id.New("trace"),
			Metadata:        map[string]string{"ui": "http"},
		}
		if err := a.SessionRepo.Create(ctx, current); err != nil {
			return err
		}
	}
	if wm, err := a.MemoryRepo.GetWorkingSnapshot(ctx, current.ID); err != nil {
		return err
	} else if wm == nil {
		if err := a.MemoryRepo.SaveWorkingSnapshot(ctx, &memory.WorkingMemory{
			ID:        current.WorkingMemoryID,
			SessionID: current.ID,
			UpdatedAt: time.Now(),
		}); err != nil {
			return err
		}
	}
	a.sessionID = current.ID
	return nil
}

func (a *App) registerSubscribers() {
	mustSubscribe(a.Bus, &processors.ContextProcessor{
		Bus:         a.Bus,
		SessionRepo: a.SessionRepo,
		MemoryRepo:  a.MemoryRepo,
		LogRepo:     a.LogRepo,
		SessionID:   a.CurrentSessionID(),
	})
	mustSubscribe(a.Bus, &processors.IntentClassifier{
		Bus:       a.Bus,
		LogRepo:   a.LogRepo,
		SessionID: a.CurrentSessionID(),
	})
	mustSubscribe(a.Bus, &processors.TaskPlanner{
		Bus:         a.Bus,
		SessionRepo: a.SessionRepo,
		TaskRepo:    a.TaskRepo,
		MemoryRepo:  a.MemoryRepo,
		LogRepo:     a.LogRepo,
		SessionID:   a.CurrentSessionID(),
	})
	mustSubscribe(a.Bus, &processors.Director{
		Bus:      a.Bus,
		TaskRepo: a.TaskRepo,
		Agents:   a.Agents,
		LogRepo:  a.LogRepo,
	})
	mustSubscribe(a.Bus, &processors.VideoSuggestionProcessor{
		Bus:          a.Bus,
		ApprovalRepo: a.ApprovalRepo,
		SessionRepo:  a.SessionRepo,
		MemoryRepo:   a.MemoryRepo,
		LogRepo:      a.LogRepo,
		SessionID:    a.CurrentSessionID(),
	})
	mustSubscribe(a.Bus, &executor.AssignmentExecutor{
		Bus:         a.Bus,
		TaskRepo:    a.TaskRepo,
		SessionRepo: a.SessionRepo,
		MemoryRepo:  a.MemoryRepo,
		LogRepo:     a.LogRepo,
		Agents:      a.Agents,
		Tools:       a.Tools,
	})
	mustSubscribe(a.Bus, &executor.ToolExecutor{
		Bus:      a.Bus,
		Tools:    a.Tools,
		LogRepo:  a.LogRepo,
		TaskRepo: a.TaskRepo,
	})
	mustSubscribe(a.Bus, &executor.ResultHandler{
		Bus:         a.Bus,
		TaskRepo:    a.TaskRepo,
		SessionRepo: a.SessionRepo,
		MemoryRepo:  a.MemoryRepo,
		LogRepo:     a.LogRepo,
	})
	mustSubscribe(a.Bus, &executor.ApprovalResponder{
		Bus:          a.Bus,
		ApprovalRepo: a.ApprovalRepo,
		SessionRepo:  a.SessionRepo,
		MemoryRepo:   a.MemoryRepo,
		LogRepo:      a.LogRepo,
	})
	mustSubscribe(a.Bus, &memorymanager.Manager{
		DataDir:    a.Config.DataDir,
		MemoryRepo: a.MemoryRepo,
		TaskRepo:   a.TaskRepo,
		LogRepo:    a.LogRepo,
	})
}

func mustSubscribe(bus eventbus.Bus, sub eventbus.Subscriber) {
	if err := bus.Subscribe(sub); err != nil {
		panic(err)
	}
}
