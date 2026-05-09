// Package rpc routes stable JSON-RPC methods into the main orchestrator.
package rpc

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"

// registerHandlers binds stable agent.* JSON-RPC methods to their protocol
// decoders and orchestrator entry points.
func (s *Server) registerHandlers() {
	s.handlers = map[string]methodHandler{}
	s.methodSpecs = map[string]methodSpec{}
	for _, method := range s.stableMethodRegistry() {
		s.handlers[method.Name] = method.Handle
		s.methodSpecs[method.Name] = method.methodSpec
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
	data, err := s.orchestrator.SubmitInput(orchestrator.SubmitInputRequestFromParams(params))
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskStart handles agent.task.start.
func (s *Server) handleAgentTaskStart(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.StartTask(orchestrator.StartTaskRequestFromParams(params))
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
	data, err := s.orchestrator.TaskDetailGet(orchestrator.TaskDetailGetRequestFromParams(params))
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskEventsList handles agent.task.events.list.
func (s *Server) handleAgentTaskEventsList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskEventsList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskToolCallsList handles agent.task.tool_calls.list.
func (s *Server) handleAgentTaskToolCallsList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskToolCallsList(params)
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

// handleAgentSettingsModelValidate handles agent.settings.model.validate.
func (s *Server) handleAgentSettingsModelValidate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SettingsModelValidate(params)
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

// wrapOrchestratorResult maps orchestrator return values into the shared RPC
// success/error envelope.
// It does not correct business logic; it only freezes protocol-facing error
// codes, messages, and trace metadata.
func wrapOrchestratorResult(data any, err error) (any, *rpcError) {
	if err == nil {
		return data, nil
	}

	return nil, rpcErrorFromOrchestratorError(err)
}
