// Package orchestrator assembles the owner-4 task-centric backend workflow.
package orchestrator

import (
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/titlegen"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

// Service owns the task-centric backend boundary behind JSON-RPC adapters.
// It keeps task/run mapping, delivery assembly, risk governance, memory hooks,
// and query hydration behind one dependency graph without starting transports.
type Service struct {
	context          *contextsvc.Service
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

// NewService returns a minimally usable orchestrator with required runtime
// dependencies and default optional services. It does not start background work;
// bootstrap may replace optional collaborators through With* methods before use.
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
		context:          context,
		intent:           intent,
		runEngine:        runEngine,
		delivery:         delivery,
		memory:           memory,
		risk:             risk,
		model:            model,
		tools:            tools,
		plugin:           plugin,
		audit:            audit.NewService(),
		recommendation:   recommendation.NewService(),
		traceEval:        traceeval.NewService(nil, nil),
		inspector:        taskinspector.NewService(nil),
		titleGenerator:   titlegen.NewService(model),
		executionTimeout: defaultTaskExecutionTimeout,
		runtimeTaps:      map[uint64]func(taskID, method string, params map[string]any){},
		taskStartTaps:    map[uint64]func(taskID, sessionID, traceID string){},
	}
}

// WithAudit replaces the default audit service when bootstrap owns a shared
// audit store. A nil value leaves the default in-memory audit service intact.
func (s *Service) WithAudit(auditService *audit.Service) *Service {
	if auditService != nil {
		s.audit = auditService
	}
	return s
}

// WithExecutor attaches the execution service used by the main task loop and
// wires runtime notifications plus steering polls back into task state.
func (s *Service) WithExecutor(executorService *execution.Service) *Service {
	s.executor = executorService
	if executorService != nil {
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
	return s
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

// WithStorage attaches shared storage for governance and query-side hydration.
func (s *Service) WithStorage(storageService *storage.Service) *Service {
	if storageService != nil {
		s.storage = storageService
	}
	return s
}

// WithTraceEval replaces the default trace/eval recorder used after execution.
// A nil value keeps the default recorder so task execution remains available.
func (s *Service) WithTraceEval(traceEvalService *traceeval.Service) *Service {
	if traceEvalService != nil {
		s.traceEval = traceEvalService
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
