package rpc

import (
	"encoding/json"
	"testing"
)

func TestDispatchReturnsMaintenanceApprovalRequest(t *testing.T) {
	server := newTestServer()
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-maintenance-approval"`),
		Method:  "agent.task.history.delete",
		Params:  mustMarshal(t, map[string]any{}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	approval, ok := data["approval_request"].(map[string]any)
	if !ok || approval["operation_name"] != "task_history_delete" {
		t.Fatalf("expected maintenance approval payload, got %+v", data)
	}
}
