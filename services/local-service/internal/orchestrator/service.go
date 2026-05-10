// Package orchestrator assembles the owner-4 task-centric backend workflow.
package orchestrator

import (
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/recommendation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/titlegen"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

// Service owns the task-centric backend boundary behind JSON-RPC adapters.
// It keeps task/run mapping, delivery assembly, risk governance, memory hooks,
// and query hydration behind one dependency graph without starting transports.
type Service struct {
	context          *taskcontext.CaptureService
	intent           *intent.Service
	runEngine        *runengine.Engine
	delivery         *delivery.Service
	memory           *memory.Service
	risk             *risk.Service
	model            *model.Service
	tools            *tools.Registry
	plugin           *plugin.Service
	audit            *audit.Service
	recommendation   *recommendation.Service
	traceEval        *traceeval.Service
	executor         *execution.Service
	inspector        *taskinspector.Service
	storage          *storage.Service
	titleGenerator   *titlegen.Service
	modelMu          sync.RWMutex
	runtimeMu        sync.RWMutex
	executionTimeout time.Duration
	runtimeNextID    uint64
	runtimeTaps      map[uint64]func(taskID, method string, params map[string]any)
	taskStartTaps    map[uint64]func(taskID, sessionID, traceID string)
}

// NewService builds the task-centric orchestrator from one explicit dependency
// graph. Required collaborators must be present in deps before construction,
// while nil optional collaborators fall back to bootstrap-safe defaults here.
func NewService(deps Deps) (*Service, error) {
	if err := deps.Validate(); err != nil {
		return nil, err
	}
	service := &Service{
		context:          deps.Context,
		intent:           deps.Intent,
		runEngine:        deps.RunEngine,
		delivery:         deps.Delivery,
		memory:           deps.Memory,
		risk:             deps.Risk,
		model:            deps.Model,
		tools:            deps.Tools,
		plugin:           deps.Plugin,
		audit:            deps.auditService(),
		recommendation:   deps.recommendationService(),
		traceEval:        deps.traceEvalService(),
		inspector:        deps.inspectorService(),
		titleGenerator:   titlegen.NewService(deps.Model),
		storage:          deps.Storage,
		executionTimeout: deps.resolvedExecutionTimeout(),
		runtimeTaps:      map[uint64]func(taskID, method string, params map[string]any){},
		taskStartTaps:    map[uint64]func(taskID, sessionID, traceID string){},
	}
	service.attachExecutor(deps.Executor)
	return service, nil
}

// attachExecutor wires runtime notifications plus steering polls back into task
// state. The dependency is explicit at construction time because production
// task execution should not rely on a later mutation step.
func (s *Service) attachExecutor(executorService *execution.Service) {
	s.executor = executorService
	if executorService == nil {
		return
	}
	executorService.WithNotificationEmitter(func(taskID, method string, params map[string]any) {
		s.publishRuntimeNotification(taskID, method, params)
		_, _ = s.runEngine.EmitRuntimeNotification(taskID, method, params)
	}).WithSteeringPoller(func(taskID string) []string {
		messages, ok := s.runEngine.DrainSteeringMessages(taskID)
		if !ok {
			return nil
		}
		return messages
	})
}

// WithTaskInspector replaces the default inspector service used by inspection
// RPCs. A nil value keeps the default no-storage inspector.
func (s *Service) WithTaskInspector(inspectorService *taskinspector.Service) *Service {
	if inspectorService != nil {
		s.inspector = inspectorService
	}
	return s
}

// WithTitleGenerator replaces the default runtime title generator so every
// title-producing path shares the same model-backed policy.
func (s *Service) WithTitleGenerator(generator *titlegen.Service) *Service {
	if generator != nil {
		s.titleGenerator = generator
	}
	return s
}

// Snapshot returns the minimal debug summary for health endpoints. It is a
// read-only view and must not become the task, delivery, or governance truth.
func (s *Service) Snapshot() map[string]any {
	pendingApprovals, pendingTotal := s.runEngine.PendingApprovalRequests(100, 0)
	primaryWorker := ""
	if s.plugin != nil {
		if workers := s.plugin.Workers(); len(workers) > 0 {
			primaryWorker = workers[0]
		}
	}
	return map[string]any{
		"context_source":          s.context.Snapshot()["source"],
		"intent_state":            s.intent.Analyze("bootstrap"),
		"task_status":             s.runEngine.CurrentTaskStatus(),
		"run_state":               s.runEngine.CurrentState(),
		"delivery_type":           s.delivery.DefaultResultType(),
		"memory_backend":          s.memory.RetrievalBackend(),
		"risk_level":              s.risk.DefaultLevel(),
		"model":                   s.currentModelDescriptor(),
		"tool_count":              len(s.tools.Names()),
		"primary_worker":          primaryWorker,
		"pending_approvals":       pendingTotal,
		"latest_approval_request": firstMapOrNil(pendingApprovals),
	}
}

// RunEngine exposes the attached runtime engine for transport-layer tests and
// debug wiring that need to seed notifications or inspect task state.
func (s *Service) RunEngine() *runengine.Engine {
	return s.runEngine
}
