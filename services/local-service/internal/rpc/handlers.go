// Package rpc routes stable JSON-RPC methods into the main orchestrator.
package rpc

import (
	"errors"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

// registerHandlers binds stable agent.* JSON-RPC methods to orchestrator entry
// points.
func (s *Server) registerHandlers() {
	s.handlers = map[string]methodHandler{
		"agent.input.submit":                   s.handleAgentInputSubmit,
		"agent.task.start":                     s.handleAgentTaskStart,
		"agent.task.confirm":                   s.handleAgentTaskConfirm,
		"agent.task.artifact.list":             s.handleAgentTaskArtifactList,
		"agent.task.artifact.open":             s.handleAgentTaskArtifactOpen,
		"agent.delivery.open":                  s.handleAgentDeliveryOpen,
		"agent.recommendation.get":             s.handleAgentRecommendationGet,
		"agent.recommendation.feedback.submit": s.handleAgentRecommendationFeedbackSubmit,
		"agent.task.list":                      s.handleAgentTaskList,
		"agent.task.detail.get":                s.handleAgentTaskDetailGet,
		"agent.task.events.list":               s.handleAgentTaskEventsList,
		"agent.task.steer":                     s.handleAgentTaskSteer,
		"agent.task.control":                   s.handleAgentTaskControl,
		"agent.task_inspector.config.get":      s.handleAgentTaskInspectorConfigGet,
		"agent.task_inspector.config.update":   s.handleAgentTaskInspectorConfigUpdate,
		"agent.task_inspector.run":             s.handleAgentTaskInspectorRun,
		"agent.notepad.list":                   s.handleAgentNotepadList,
		"agent.notepad.update":                 s.handleAgentNotepadUpdate,
		"agent.notepad.convert_to_task":        s.handleAgentNotepadConvertToTask,
		"agent.dashboard.overview.get":         s.handleAgentDashboardOverviewGet,
		"agent.dashboard.module.get":           s.handleAgentDashboardModuleGet,
		"agent.mirror.overview.get":            s.handleAgentMirrorOverviewGet,
		"agent.security.summary.get":           s.handleAgentSecuritySummaryGet,
		"agent.security.audit.list":            s.handleAgentSecurityAuditList,
		"agent.security.pending.list":          s.handleAgentSecurityPendingList,
		"agent.security.restore_points.list":   s.handleAgentSecurityRestorePointsList,
		"agent.security.restore.apply":         s.handleAgentSecurityRestoreApply,
		"agent.security.respond":               s.handleAgentSecurityRespond,
		"agent.settings.get":                   s.handleAgentSettingsGet,
		"agent.settings.update":                s.handleAgentSettingsUpdate,
		"agent.plugin.runtime.list":            s.handleAgentPluginRuntimeList,
		"agent.plugin.list":                    s.handleAgentPluginList,
		"agent.plugin.detail.get":              s.handleAgentPluginDetailGet,
	}
}

// handleAgentTaskArtifactList handles agent.task.artifact.list.
func (s *Server) handleAgentTaskArtifactList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskArtifactList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskArtifactOpen handles agent.task.artifact.open.
func (s *Server) handleAgentTaskArtifactOpen(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskArtifactOpen(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentDeliveryOpen handles agent.delivery.open.
func (s *Server) handleAgentDeliveryOpen(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.DeliveryOpen(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentInputSubmit handles agent.input.submit.
func (s *Server) handleAgentInputSubmit(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SubmitInput(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskStart handles agent.task.start.
func (s *Server) handleAgentTaskStart(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.StartTask(sanitizeTaskStartParams(params))
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskConfirm handles agent.task.confirm.
func (s *Server) handleAgentTaskConfirm(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.ConfirmTask(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentRecommendationGet handles agent.recommendation.get.
func (s *Server) handleAgentRecommendationGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.RecommendationGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentRecommendationFeedbackSubmit handles
// agent.recommendation.feedback.submit.
func (s *Server) handleAgentRecommendationFeedbackSubmit(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.RecommendationFeedbackSubmit(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskList handles agent.task.list.
func (s *Server) handleAgentTaskList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskDetailGet handles agent.task.detail.get.
func (s *Server) handleAgentTaskDetailGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskDetailGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskEventsList handles agent.task.events.list.
func (s *Server) handleAgentTaskEventsList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskEventsList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskSteer handles agent.task.steer.
func (s *Server) handleAgentTaskSteer(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskSteer(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskControl handles agent.task.control.
func (s *Server) handleAgentTaskControl(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskControl(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskInspectorConfigGet handles agent.task_inspector.config.get.
func (s *Server) handleAgentTaskInspectorConfigGet(params map[string]any) (any, *rpcError) {
	_ = params
	data, err := s.orchestrator.TaskInspectorConfigGet()
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskInspectorConfigUpdate handles
// agent.task_inspector.config.update.
func (s *Server) handleAgentTaskInspectorConfigUpdate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskInspectorConfigUpdate(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskInspectorRun handles agent.task_inspector.run.
func (s *Server) handleAgentTaskInspectorRun(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskInspectorRun(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentNotepadList handles agent.notepad.list.
func (s *Server) handleAgentNotepadList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.NotepadList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentNotepadUpdate handles agent.notepad.update.
func (s *Server) handleAgentNotepadUpdate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.NotepadUpdate(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentNotepadConvertToTask handles agent.notepad.convert_to_task.
func (s *Server) handleAgentNotepadConvertToTask(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.NotepadConvertToTask(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentDashboardOverviewGet handles agent.dashboard.overview.get.
func (s *Server) handleAgentDashboardOverviewGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.DashboardOverviewGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentDashboardModuleGet handles agent.dashboard.module.get.
func (s *Server) handleAgentDashboardModuleGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.DashboardModuleGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentMirrorOverviewGet handles agent.mirror.overview.get.
func (s *Server) handleAgentMirrorOverviewGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.MirrorOverviewGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecuritySummaryGet handles agent.security.summary.get.
func (s *Server) handleAgentSecuritySummaryGet(params map[string]any) (any, *rpcError) {
	_ = params
	data, err := s.orchestrator.SecuritySummaryGet()
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecurityAuditList handles agent.security.audit.list.
func (s *Server) handleAgentSecurityAuditList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityAuditList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecurityPendingList handles agent.security.pending.list.
func (s *Server) handleAgentSecurityPendingList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityPendingList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecurityRestorePointsList handles
// agent.security.restore_points.list.
func (s *Server) handleAgentSecurityRestorePointsList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityRestorePointsList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecurityRestoreApply handles agent.security.restore.apply.
func (s *Server) handleAgentSecurityRestoreApply(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityRestoreApply(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecurityRespond handles agent.security.respond.
func (s *Server) handleAgentSecurityRespond(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityRespond(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSettingsGet handles agent.settings.get.
func (s *Server) handleAgentSettingsGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SettingsGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSettingsUpdate handles agent.settings.update.
func (s *Server) handleAgentSettingsUpdate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SettingsUpdate(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentPluginRuntimeList handles agent.plugin.runtime.list.
func (s *Server) handleAgentPluginRuntimeList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.PluginRuntimeList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentPluginList handles agent.plugin.list.
func (s *Server) handleAgentPluginList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.PluginList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentPluginDetailGet handles agent.plugin.detail.get.
func (s *Server) handleAgentPluginDetailGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.PluginDetailGet(params)
	return wrapOrchestratorResult(data, err)
}

// sanitizeTaskStartParams strips any client-supplied intent payload so
// agent.task.start continues to flow through the authoritative suggestion path.
func sanitizeTaskStartParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return nil
	}

	sanitized := make(map[string]any, len(params))
	for key, value := range params {
		if strings.TrimSpace(key) == "intent" {
			continue
		}
		sanitized[key] = value
	}
	return sanitized
}

// wrapOrchestratorResult maps orchestrator return values into the shared RPC
// success/error envelope.
// It does not correct business logic; it only freezes protocol-facing error
// codes, messages, and trace metadata.
func wrapOrchestratorResult(data any, err error) (any, *rpcError) {
	if err == nil {
		return data, nil
	}

	if errors.Is(err, orchestrator.ErrTaskNotFound) {
		return nil, &rpcError{
			Code:    1001001,
			Message: "TASK_NOT_FOUND",
			Detail:  err.Error(),
			TraceID: "trace_task_not_found",
		}
	}
	if errors.Is(err, orchestrator.ErrArtifactNotFound) {
		return nil, &rpcError{
			Code:    1005002,
			Message: "ARTIFACT_NOT_FOUND",
			Detail:  err.Error(),
			TraceID: "trace_artifact_not_found",
		}
	}
	if errors.Is(err, orchestrator.ErrTaskStatusInvalid) {
		return nil, &rpcError{
			Code:    1001004,
			Message: "TASK_STATUS_INVALID",
			Detail:  err.Error(),
			TraceID: "trace_task_status_invalid",
		}
	}
	if errors.Is(err, orchestrator.ErrTaskAlreadyFinished) {
		return nil, &rpcError{
			Code:    1001005,
			Message: "TASK_ALREADY_FINISHED",
			Detail:  err.Error(),
			TraceID: "trace_task_already_finished",
		}
	}
	if errors.Is(err, orchestrator.ErrStorageQueryFailed) {
		return nil, &rpcError{
			Code:    1005001,
			Message: "SQLITE_WRITE_FAILED",
			Detail:  err.Error(),
			TraceID: "trace_storage_query_failed",
		}
	}
	if errors.Is(err, storage.ErrStructuredStoreUnavailable) || errors.Is(err, storage.ErrDatabasePathRequired) {
		return nil, &rpcError{
			Code:    1005001,
			Message: "SQLITE_WRITE_FAILED",
			Detail:  err.Error(),
			TraceID: "trace_sqlite_write_failed",
		}
	}
	if errors.Is(err, orchestrator.ErrStrongholdAccessFailed) {
		return nil, &rpcError{
			Code:    1005004,
			Message: "STRONGHOLD_ACCESS_FAILED",
			Detail:  err.Error(),
			TraceID: "trace_stronghold_access_failed",
		}
	}
	if errors.Is(err, storage.ErrStrongholdAccessFailed) || errors.Is(err, storage.ErrStrongholdUnavailable) || errors.Is(err, storage.ErrSecretStoreAccessFailed) || errors.Is(err, storage.ErrSecretNotFound) || errors.Is(err, model.ErrClientNotConfigured) || errors.Is(err, model.ErrSecretSourceFailed) || errors.Is(err, model.ErrSecretNotFound) {
		return nil, &rpcError{
			Code:    1005004,
			Message: "STRONGHOLD_ACCESS_FAILED",
			Detail:  err.Error(),
			TraceID: "trace_stronghold_access_failed",
		}
	}
	if errors.Is(err, orchestrator.ErrRecoveryPointNotFound) {
		return nil, &rpcError{
			Code:    1005006,
			Message: "RECOVERY_POINT_NOT_FOUND",
			Detail:  err.Error(),
			TraceID: "trace_recovery_point_not_found",
		}
	}
	if errors.Is(err, model.ErrModelProviderRequired) || errors.Is(err, model.ErrModelProviderUnsupported) {
		return nil, &rpcError{
			Code:    1008001,
			Message: "MODEL_PROVIDER_NOT_FOUND",
			Detail:  err.Error(),
			TraceID: "trace_model_provider_not_found",
		}
	}
	if errors.Is(err, model.ErrToolCallingNotSupported) || errors.Is(err, model.ErrOpenAIAPIKeyRequired) || errors.Is(err, model.ErrOpenAIEndpointRequired) || errors.Is(err, model.ErrOpenAIModelIDRequired) || errors.Is(err, model.ErrOpenAIHTTPStatus) || errors.Is(err, model.ErrOpenAIRequestFailed) || errors.Is(err, model.ErrOpenAIRequestTimeout) || errors.Is(err, model.ErrOpenAIResponseInvalid) {
		return nil, &rpcError{
			Code:    1008002,
			Message: "MODEL_NOT_ALLOWED",
			Detail:  err.Error(),
			TraceID: "trace_model_not_allowed",
		}
	}

	return nil, &rpcError{
		Code:    errInvalidParams,
		Message: "INVALID_PARAMS",
		Detail:  err.Error(),
		TraceID: "trace_orchestrator_error",
	}
}
