package rpc

import "testing"

func TestHandlerWrappersCoverRecommendationInspectorDashboardAndSecurityMethods(t *testing.T) {
	server := newTestServer()
	handlerCalls := []struct {
		name   string
		invoke func() (any, *rpcError)
	}{
		{name: "recommendation.get", invoke: func() (any, *rpcError) { return server.handleAgentRecommendationGet(map[string]any{}) }},
		{name: "recommendation.feedback.submit", invoke: func() (any, *rpcError) { return server.handleAgentRecommendationFeedbackSubmit(map[string]any{}) }},
		{name: "task_inspector.config.get", invoke: func() (any, *rpcError) { return server.handleAgentTaskInspectorConfigGet(nil) }},
		{name: "task_inspector.config.update", invoke: func() (any, *rpcError) {
			return server.handleAgentTaskInspectorConfigUpdate(map[string]any{"task_sources": []any{"D:/workspace/todos"}, "inspection_interval": map[string]any{"unit": "minute", "value": 10}})
		}},
		{name: "task_inspector.run", invoke: func() (any, *rpcError) { return server.handleAgentTaskInspectorRun(map[string]any{}) }},
		{name: "notepad.list", invoke: func() (any, *rpcError) { return server.handleAgentNotepadList(map[string]any{}) }},
		{name: "notepad.convert_to_task", invoke: func() (any, *rpcError) {
			return server.handleAgentNotepadConvertToTask(map[string]any{"item_id": "missing"})
		}},
		{name: "dashboard.overview.get", invoke: func() (any, *rpcError) { return server.handleAgentDashboardOverviewGet(map[string]any{}) }},
		{name: "dashboard.module.get", invoke: func() (any, *rpcError) { return server.handleAgentDashboardModuleGet(map[string]any{"module": "task"}) }},
		{name: "mirror.overview.get", invoke: func() (any, *rpcError) { return server.handleAgentMirrorOverviewGet(map[string]any{}) }},
		{name: "security.summary.get", invoke: func() (any, *rpcError) { return server.handleAgentSecuritySummaryGet(nil) }},
		{name: "security.pending.list", invoke: func() (any, *rpcError) { return server.handleAgentSecurityPendingList(map[string]any{}) }},
		{name: "security.respond", invoke: func() (any, *rpcError) {
			return server.handleAgentSecurityRespond(map[string]any{"task_id": "missing", "decision": "approve"})
		}},
		{name: "settings.model.validate", invoke: func() (any, *rpcError) { return server.handleAgentSettingsModelValidate(map[string]any{}) }},
	}
	for _, call := range handlerCalls {
		data, rpcErr := call.invoke()
		if data == nil && rpcErr == nil {
			t.Fatalf("expected %s handler to return either data or rpc error", call.name)
		}
		if rpcErr != nil && rpcErr.TraceID == "" {
			t.Fatalf("expected %s handler rpc error to include trace id, got %+v", call.name, rpcErr)
		}
	}
}
