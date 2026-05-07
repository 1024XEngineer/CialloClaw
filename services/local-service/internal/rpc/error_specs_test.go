package rpc

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestRPCErrorFromOrchestratorErrorUsesDeclaredSpecs(t *testing.T) {
	tests := []struct {
		name        string
		give        error
		wantCode    int
		wantMessage string
		wantTraceID string
	}{
		{name: "task not found", give: orchestrator.ErrTaskNotFound, wantCode: 1001001, wantMessage: "TASK_NOT_FOUND", wantTraceID: "trace_task_not_found"},
		{name: "artifact not found", give: orchestrator.ErrArtifactNotFound, wantCode: 1005002, wantMessage: "ARTIFACT_NOT_FOUND", wantTraceID: "trace_artifact_not_found"},
		{name: "task status invalid", give: orchestrator.ErrTaskStatusInvalid, wantCode: 1001004, wantMessage: "TASK_STATUS_INVALID", wantTraceID: "trace_task_status_invalid"},
		{name: "task already finished", give: orchestrator.ErrTaskAlreadyFinished, wantCode: 1001005, wantMessage: "TASK_ALREADY_FINISHED", wantTraceID: "trace_task_already_finished"},
		{name: "orchestrator storage query", give: orchestrator.ErrStorageQueryFailed, wantCode: 1005001, wantMessage: "SQLITE_WRITE_FAILED", wantTraceID: "trace_storage_query_failed"},
		{name: "structured store unavailable", give: storage.ErrStructuredStoreUnavailable, wantCode: 1005001, wantMessage: "SQLITE_WRITE_FAILED", wantTraceID: "trace_sqlite_write_failed"},
		{name: "orchestrator stronghold", give: orchestrator.ErrStrongholdAccessFailed, wantCode: 1005004, wantMessage: "STRONGHOLD_ACCESS_FAILED", wantTraceID: "trace_stronghold_access_failed"},
		{name: "storage stronghold", give: storage.ErrStrongholdUnavailable, wantCode: 1005004, wantMessage: "STRONGHOLD_ACCESS_FAILED", wantTraceID: "trace_stronghold_access_failed"},
		{name: "recovery point not found", give: orchestrator.ErrRecoveryPointNotFound, wantCode: 1005006, wantMessage: "RECOVERY_POINT_NOT_FOUND", wantTraceID: "trace_recovery_point_not_found"},
		{name: "model provider missing", give: model.ErrModelProviderUnsupported, wantCode: 1008001, wantMessage: "MODEL_PROVIDER_NOT_FOUND", wantTraceID: "trace_model_provider_not_found"},
		{name: "tool output invalid", give: tools.ErrToolOutputInvalid, wantCode: 1003004, wantMessage: "TOOL_OUTPUT_INVALID", wantTraceID: "trace_tool_output_invalid"},
		{name: "inspection outside workspace", give: taskinspector.ErrInspectionSourceOutsideWorkspace, wantCode: 1004003, wantMessage: "WORKSPACE_BOUNDARY_DENIED", wantTraceID: "trace_workspace_boundary_denied"},
		{name: "inspection filesystem unavailable", give: taskinspector.ErrInspectionFileSystemUnavailable, wantCode: 1007006, wantMessage: "INSPECTION_FILESYSTEM_UNAVAILABLE", wantTraceID: "trace_inspection_filesystem_unavailable"},
		{name: "inspection source not found", give: taskinspector.ErrInspectionSourceNotFound, wantCode: 1007007, wantMessage: "INSPECTION_SOURCE_NOT_FOUND", wantTraceID: "trace_inspection_source_not_found"},
		{name: "inspection source unreadable", give: taskinspector.ErrInspectionSourceUnreadable, wantCode: 1007008, wantMessage: "INSPECTION_SOURCE_UNREADABLE", wantTraceID: "trace_inspection_source_unreadable"},
		{name: "model runtime unavailable", give: model.ErrOpenAIRequestTimeout, wantCode: 1008003, wantMessage: "MODEL_RUNTIME_UNAVAILABLE", wantTraceID: "trace_model_runtime_unavailable"},
		{name: "model not allowed", give: model.ErrOpenAIEndpointRequired, wantCode: 1008002, wantMessage: "MODEL_NOT_ALLOWED", wantTraceID: "trace_model_not_allowed"},
		{name: "wrapped sentinel", give: fmt.Errorf("wrapped: %w", orchestrator.ErrTaskNotFound), wantCode: 1001001, wantMessage: "TASK_NOT_FOUND", wantTraceID: "trace_task_not_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rpcErrorFromOrchestratorError(tt.give)
			if got.Code != tt.wantCode || got.Message != tt.wantMessage || got.TraceID != tt.wantTraceID {
				t.Fatalf("rpc error = code:%d message:%s trace:%s, want code:%d message:%s trace:%s", got.Code, got.Message, got.TraceID, tt.wantCode, tt.wantMessage, tt.wantTraceID)
			}
			if got.Detail != tt.give.Error() {
				t.Fatalf("detail = %q, want %q", got.Detail, tt.give.Error())
			}
		})
	}
}

func TestRPCErrorFromOrchestratorErrorFallsBackToInvalidParams(t *testing.T) {
	got := rpcErrorFromOrchestratorError(errors.New("unknown orchestrator failure"))
	if got.Code != errInvalidParams || got.Message != "INVALID_PARAMS" || got.TraceID != "trace_orchestrator_error" {
		t.Fatalf("fallback rpc error = code:%d message:%s trace:%s", got.Code, got.Message, got.TraceID)
	}
}
