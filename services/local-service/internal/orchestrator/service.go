package orchestrator

import (
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type Service struct {
	context   *contextsvc.Service
	intent    *intent.Service
	runEngine *runengine.Engine
	delivery  *delivery.Service
	memory    *memory.Service
	risk      *risk.Service
	model     *model.Service
	tools     *tools.Registry
	plugin    *plugin.Service
}

func NewService(
	context *contextsvc.Service,
	intent *intent.Service,
	runEngine *runengine.Engine,
	delivery *delivery.Service,
	memory *memory.Service,
	risk *risk.Service,
	model *model.Service,
	tools *tools.Registry,
	plugin *plugin.Service,
) *Service {
	return &Service{
		context:   context,
		intent:    intent,
		runEngine: runEngine,
		delivery:  delivery,
		memory:    memory,
		risk:      risk,
		model:     model,
		tools:     tools,
		plugin:    plugin,
	}
}

func (s *Service) Snapshot() map[string]any {
	return map[string]any{
		"context_source": s.context.Snapshot()["source"],
		"intent_state":   s.intent.Analyze("bootstrap"),
		"task_status":    s.runEngine.CurrentTaskStatus(),
		"run_state":      s.runEngine.CurrentState(),
		"delivery_type":  s.delivery.DefaultResultType(),
		"memory_backend": s.memory.RetrievalBackend(),
		"risk_level":     s.risk.DefaultLevel(),
		"model":          s.model.Descriptor(),
		"tool_count":     len(s.tools.Names()),
		"primary_worker": s.plugin.Workers()[0],
	}
}
