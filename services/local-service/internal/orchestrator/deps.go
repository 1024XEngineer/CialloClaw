package orchestrator

import (
	"fmt"
	"strings"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

// Deps declares the full orchestrator dependency graph in one place so callers
// can distinguish required runtime collaborators from optional enrichers.
//
// Required fields must be populated before construction:
// Context, Intent, RunEngine, Delivery, Memory, Risk, Model, Tools, Plugin.
//
// Optional fields fall back to bootstrap-safe defaults when left nil:
// Audit, Recommendation, TraceEval, Inspector, Executor, Storage.
type Deps struct {
	Context *taskcontext.CaptureService
	Intent  *intent.Service

	RunEngine *runengine.Engine
	Delivery  *delivery.Service
	Memory    *memory.Service
	Risk      *risk.Service
	Model     *model.Service
	Tools     *tools.Registry
	Plugin    *plugin.Service

	Audit          *audit.Service
	Recommendation *recommendation.Service
	TraceEval      *traceeval.Service
	Executor       *execution.Service
	Inspector      *taskinspector.Service
	Storage        *storage.Service

	ExecutionTimeout time.Duration
}

// Validate enforces the constructor contract so missing required collaborators
// fail at wiring time instead of surfacing later as unrelated nil dereferences.
func (d Deps) Validate() error {
	missing := make([]string, 0, 9)
	if d.Context == nil {
		missing = append(missing, "Context")
	}
	if d.Intent == nil {
		missing = append(missing, "Intent")
	}
	if d.RunEngine == nil {
		missing = append(missing, "RunEngine")
	}
	if d.Delivery == nil {
		missing = append(missing, "Delivery")
	}
	if d.Memory == nil {
		missing = append(missing, "Memory")
	}
	if d.Risk == nil {
		missing = append(missing, "Risk")
	}
	if d.Model == nil {
		missing = append(missing, "Model")
	}
	if d.Tools == nil {
		missing = append(missing, "Tools")
	}
	if d.Plugin == nil {
		missing = append(missing, "Plugin")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("orchestrator: missing required deps: %s", strings.Join(missing, ", "))
}

func (d Deps) auditService() *audit.Service {
	if d.Audit != nil {
		return d.Audit
	}
	return audit.NewService()
}

func (d Deps) recommendationService() *recommendation.Service {
	if d.Recommendation != nil {
		return d.Recommendation
	}
	return recommendation.NewService()
}

func (d Deps) traceEvalService() *traceeval.Service {
	if d.TraceEval != nil {
		return d.TraceEval
	}
	return traceeval.NewService(nil, nil)
}

func (d Deps) inspectorService() *taskinspector.Service {
	if d.Inspector != nil {
		return d.Inspector
	}
	return taskinspector.NewService(nil)
}

func (d Deps) resolvedExecutionTimeout() time.Duration {
	if d.ExecutionTimeout > 0 {
		return d.ExecutionTimeout
	}
	return defaultTaskExecutionTimeout
}
