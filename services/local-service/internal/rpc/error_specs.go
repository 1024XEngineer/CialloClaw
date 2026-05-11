package rpc

import (
	"errors"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type orchestratorErrorSpec struct {
	targets []error
	match   func(error) bool
	code    int
	message string
	traceID string
}

var orchestratorErrorSpecs = []orchestratorErrorSpec{
	{
		targets: []error{orchestrator.ErrTaskNotFound},
		code:    1001001,
		message: "TASK_NOT_FOUND",
		traceID: "trace_task_not_found",
	},
	{
		targets: []error{orchestrator.ErrArtifactNotFound},
		code:    1005002,
		message: "ARTIFACT_NOT_FOUND",
		traceID: "trace_artifact_not_found",
	},
	{
		targets: []error{orchestrator.ErrTaskStatusInvalid},
		code:    1001004,
		message: "TASK_STATUS_INVALID",
		traceID: "trace_task_status_invalid",
	},
	{
		targets: []error{orchestrator.ErrTaskAlreadyFinished},
		code:    1001005,
		message: "TASK_ALREADY_FINISHED",
		traceID: "trace_task_already_finished",
	},
	{
		targets: []error{orchestrator.ErrStorageQueryFailed},
		code:    1005001,
		message: "SQLITE_WRITE_FAILED",
		traceID: "trace_storage_query_failed",
	},
	{
		targets: []error{storage.ErrStructuredStoreUnavailable, storage.ErrDatabasePathRequired},
		code:    1005001,
		message: "SQLITE_WRITE_FAILED",
		traceID: "trace_sqlite_write_failed",
	},
	{
		targets: []error{orchestrator.ErrStrongholdAccessFailed},
		code:    1005004,
		message: "STRONGHOLD_ACCESS_FAILED",
		traceID: "trace_stronghold_access_failed",
	},
	{
		targets: []error{storage.ErrStrongholdAccessFailed, storage.ErrStrongholdUnavailable, storage.ErrSecretStoreAccessFailed},
		code:    1005004,
		message: "STRONGHOLD_ACCESS_FAILED",
		traceID: "trace_stronghold_access_failed",
	},
	{
		targets: []error{orchestrator.ErrRecoveryPointNotFound},
		code:    1005006,
		message: "RECOVERY_POINT_NOT_FOUND",
		traceID: "trace_recovery_point_not_found",
	},
	{
		targets: []error{model.ErrModelProviderRequired, model.ErrModelProviderUnsupported},
		code:    1008001,
		message: "MODEL_PROVIDER_NOT_FOUND",
		traceID: "trace_model_provider_not_found",
	},
	{
		targets: []error{tools.ErrToolOutputInvalid},
		code:    1003004,
		message: "TOOL_OUTPUT_INVALID",
		traceID: "trace_tool_output_invalid",
	},
	{
		targets: []error{taskinspector.ErrInspectionSourceOutsideWorkspace},
		code:    1004003,
		message: "WORKSPACE_BOUNDARY_DENIED",
		traceID: "trace_workspace_boundary_denied",
	},
	{
		targets: []error{taskinspector.ErrInspectionFileSystemUnavailable},
		code:    1007006,
		message: "INSPECTION_FILESYSTEM_UNAVAILABLE",
		traceID: "trace_inspection_filesystem_unavailable",
	},
	{
		targets: []error{taskinspector.ErrInspectionSourceNotFound},
		code:    1007007,
		message: "INSPECTION_SOURCE_NOT_FOUND",
		traceID: "trace_inspection_source_not_found",
	},
	{
		targets: []error{taskinspector.ErrInspectionSourceUnreadable},
		code:    1007008,
		message: "INSPECTION_SOURCE_UNREADABLE",
		traceID: "trace_inspection_source_unreadable",
	},
	{
		match:   model.IsProviderRuntimeUnavailable,
		code:    1008003,
		message: "MODEL_RUNTIME_UNAVAILABLE",
		traceID: "trace_model_runtime_unavailable",
	},
	{
		targets: []error{
			model.ErrClientNotConfigured,
			model.ErrToolCallingNotSupported,
			model.ErrOpenAIAPIKeyRequired,
			model.ErrOpenAIEndpointRequired,
			model.ErrOpenAIModelIDRequired,
			model.ErrSecretSourceFailed,
		},
		code:    1008002,
		message: "MODEL_NOT_ALLOWED",
		traceID: "trace_model_not_allowed",
	},
}

// rpcErrorFromOrchestratorError preserves the protocol-facing error contract
// while keeping the mapping data out of handler control flow.
func rpcErrorFromOrchestratorError(err error) *rpcError {
	for _, spec := range orchestratorErrorSpecs {
		if spec.matches(err) {
			return spec.rpcError(err)
		}
	}

	return &rpcError{
		Code:    errInvalidParams,
		Message: "INVALID_PARAMS",
		Detail:  err.Error(),
		TraceID: "trace_orchestrator_error",
	}
}

func (s orchestratorErrorSpec) matches(err error) bool {
	if s.match != nil && s.match(err) {
		return true
	}
	for _, target := range s.targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func (s orchestratorErrorSpec) rpcError(err error) *rpcError {
	return &rpcError{
		Code:    s.code,
		Message: s.message,
		Detail:  err.Error(),
		TraceID: s.traceID,
	}
}
