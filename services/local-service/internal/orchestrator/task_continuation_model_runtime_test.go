package orchestrator

import (
	"testing"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func TestModelTaskContinuationDecisionUsesCurrentModelAccessor(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "{\"decision\":\"new_task\",\"task_id\":\"\",\"reason\":\"fresh work\"}")
	service.ReplaceModel(nil)
	decision, ok := service.modelTaskContinuationDecision(contextsvc.TaskContextSnapshot{InputType: "text", Text: "new task"}, nil, taskContinuationContext{})
	if ok || decision != (taskContinuationDecision{}) {
		t.Fatalf("expected nil runtime model to skip model continuation path, got decision=%+v ok=%v", decision, ok)
	}

	service, _ = newTestServiceWithExecution(t, "{\"decision\":\"continue\",\"task_id\":\"task_001\",\"reason\":\"same task\"}")
	decision, ok = service.modelTaskContinuationDecision(contextsvc.TaskContextSnapshot{InputType: "text", Text: "follow up"}, nil, taskContinuationContext{Candidates: []runengine.TaskRecord{{TaskID: "task_001"}}})
	if !ok || decision.Decision != "continue" || decision.TaskID != "task_001" {
		t.Fatalf("expected current model accessor to keep continuation classification working, got decision=%+v ok=%v", decision, ok)
	}
	if service.currentModel() == nil || service.currentModel().Provider() != model.OpenAIResponsesProvider {
		t.Fatalf("expected runtime model to remain canonical, got %+v", service.currentModel())
	}
}
