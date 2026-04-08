// 该文件负责把稳定 RPC 方法路由到 Harness 主链路服务。
package rpc

import (
	"errors"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

func (s *Server) registerHandlers() {
	s.handlers = map[string]methodHandler{
		"agent.input.submit":                   s.handleAgentInputSubmit,
		"agent.task.start":                     s.handleAgentTaskStart,
		"agent.task.confirm":                   s.handleAgentTaskConfirm,
		"agent.recommendation.get":             s.handleAgentRecommendationGet,
		"agent.recommendation.feedback.submit": s.handleAgentRecommendationFeedbackSubmit,
		"agent.task.list":                      s.handleAgentTaskList,
		"agent.task.detail.get":                s.handleAgentTaskDetailGet,
		"agent.task.control":                   s.handleAgentTaskControl,
		"agent.task_inspector.config.get":      s.handleAgentTaskInspectorConfigGet,
		"agent.task_inspector.config.update":   s.handleAgentTaskInspectorConfigUpdate,
		"agent.task_inspector.run":             s.handleAgentTaskInspectorRun,
		"agent.notepad.list":                   s.handleAgentNotepadList,
		"agent.notepad.convert_to_task":        s.handleAgentNotepadConvertToTask,
		"agent.dashboard.overview.get":         s.handleAgentDashboardOverviewGet,
		"agent.dashboard.module.get":           s.handleAgentDashboardModuleGet,
		"agent.mirror.overview.get":            s.handleAgentMirrorOverviewGet,
		"agent.security.summary.get":           s.handleAgentSecuritySummaryGet,
		"agent.security.pending.list":          s.handleAgentSecurityPendingList,
		"agent.security.respond":               s.handleAgentSecurityRespond,
		"agent.settings.get":                   s.handleAgentSettingsGet,
		"agent.settings.update":                s.handleAgentSettingsUpdate,
	}
}

func (s *Server) handleAgentInputSubmit(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SubmitInput(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskStart(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.StartTask(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskConfirm(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.ConfirmTask(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentRecommendationGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.RecommendationGet(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentRecommendationFeedbackSubmit(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.RecommendationFeedbackSubmit(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskList(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskDetailGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskDetailGet(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskControl(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskControl(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskInspectorConfigGet(params map[string]any) (any, *rpcError) {
	_ = params
	data, err := s.orchestrator.TaskInspectorConfigGet()
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskInspectorConfigUpdate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskInspectorConfigUpdate(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentTaskInspectorRun(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskInspectorRun(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentNotepadList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.NotepadList(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentNotepadConvertToTask(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.NotepadConvertToTask(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentDashboardOverviewGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.DashboardOverviewGet(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentDashboardModuleGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.DashboardModuleGet(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentMirrorOverviewGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.MirrorOverviewGet(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentSecuritySummaryGet(params map[string]any) (any, *rpcError) {
	_ = params
	data, err := s.orchestrator.SecuritySummaryGet()
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentSecurityPendingList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityPendingList(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentSecurityRespond(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityRespond(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentSettingsGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SettingsGet(params)
	return wrapOrchestratorResult(data, err)
}

func (s *Server) handleAgentSettingsUpdate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SettingsUpdate(params)
	return wrapOrchestratorResult(data, err)
}

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

	return nil, &rpcError{
		Code:    errInvalidParams,
		Message: "INVALID_PARAMS",
		Detail:  err.Error(),
		TraceID: "trace_orchestrator_error",
	}
}
