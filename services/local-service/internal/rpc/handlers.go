// 该文件负责把稳定 RPC 方法路由到 Harness 主链路服务。
package rpc

import (
	"errors"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

// registerHandlers 处理当前模块的相关逻辑。

// registerHandlers 把稳定的 agent.* JSON-RPC 方法注册到主链路编排服务。
// 这里是 RPC 方法名到 orchestrator 入口函数的唯一收口点。
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

// handleAgentInputSubmit 处理当前模块的相关逻辑。

// handleAgentInputSubmit 处理 agent.input.submit。
func (s *Server) handleAgentInputSubmit(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SubmitInput(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskStart 处理当前模块的相关逻辑。

// handleAgentTaskStart 处理 agent.task.start。
func (s *Server) handleAgentTaskStart(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.StartTask(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskConfirm 处理当前模块的相关逻辑。

// handleAgentTaskConfirm 处理 agent.task.confirm。
func (s *Server) handleAgentTaskConfirm(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.ConfirmTask(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentRecommendationGet 处理当前模块的相关逻辑。

// handleAgentRecommendationGet 处理 agent.recommendation.get。
func (s *Server) handleAgentRecommendationGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.RecommendationGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentRecommendationFeedbackSubmit 处理当前模块的相关逻辑。

// handleAgentRecommendationFeedbackSubmit 处理 agent.recommendation.feedback.submit。
func (s *Server) handleAgentRecommendationFeedbackSubmit(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.RecommendationFeedbackSubmit(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskList 处理当前模块的相关逻辑。

// handleAgentTaskList 处理 agent.task.list。
func (s *Server) handleAgentTaskList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskDetailGet 处理当前模块的相关逻辑。

// handleAgentTaskDetailGet 处理 agent.task.detail.get。
func (s *Server) handleAgentTaskDetailGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskDetailGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskControl 处理当前模块的相关逻辑。

// handleAgentTaskControl 处理 agent.task.control。
func (s *Server) handleAgentTaskControl(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskControl(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskInspectorConfigGet 处理当前模块的相关逻辑。

// handleAgentTaskInspectorConfigGet 处理 agent.task_inspector.config.get。
func (s *Server) handleAgentTaskInspectorConfigGet(params map[string]any) (any, *rpcError) {
	_ = params
	data, err := s.orchestrator.TaskInspectorConfigGet()
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskInspectorConfigUpdate 处理当前模块的相关逻辑。

// handleAgentTaskInspectorConfigUpdate 处理 agent.task_inspector.config.update。
func (s *Server) handleAgentTaskInspectorConfigUpdate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskInspectorConfigUpdate(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentTaskInspectorRun 处理当前模块的相关逻辑。

// handleAgentTaskInspectorRun 处理 agent.task_inspector.run。
func (s *Server) handleAgentTaskInspectorRun(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.TaskInspectorRun(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentNotepadList 处理当前模块的相关逻辑。

// handleAgentNotepadList 处理 agent.notepad.list。
func (s *Server) handleAgentNotepadList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.NotepadList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentNotepadConvertToTask 处理当前模块的相关逻辑。

// handleAgentNotepadConvertToTask 处理 agent.notepad.convert_to_task。
func (s *Server) handleAgentNotepadConvertToTask(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.NotepadConvertToTask(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentDashboardOverviewGet 处理当前模块的相关逻辑。

// handleAgentDashboardOverviewGet 处理 agent.dashboard.overview.get。
func (s *Server) handleAgentDashboardOverviewGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.DashboardOverviewGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentDashboardModuleGet 处理当前模块的相关逻辑。

// handleAgentDashboardModuleGet 处理 agent.dashboard.module.get。
func (s *Server) handleAgentDashboardModuleGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.DashboardModuleGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentMirrorOverviewGet 处理当前模块的相关逻辑。

// handleAgentMirrorOverviewGet 处理 agent.mirror.overview.get。
func (s *Server) handleAgentMirrorOverviewGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.MirrorOverviewGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecuritySummaryGet 处理当前模块的相关逻辑。

// handleAgentSecuritySummaryGet 处理 agent.security.summary.get。
func (s *Server) handleAgentSecuritySummaryGet(params map[string]any) (any, *rpcError) {
	_ = params
	data, err := s.orchestrator.SecuritySummaryGet()
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecurityPendingList 处理当前模块的相关逻辑。

// handleAgentSecurityPendingList 处理 agent.security.pending.list。
func (s *Server) handleAgentSecurityPendingList(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityPendingList(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSecurityRespond 处理当前模块的相关逻辑。

// handleAgentSecurityRespond 处理 agent.security.respond。
func (s *Server) handleAgentSecurityRespond(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SecurityRespond(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSettingsGet 处理当前模块的相关逻辑。

// handleAgentSettingsGet 处理 agent.settings.get。
func (s *Server) handleAgentSettingsGet(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SettingsGet(params)
	return wrapOrchestratorResult(data, err)
}

// handleAgentSettingsUpdate 处理当前模块的相关逻辑。

// handleAgentSettingsUpdate 处理 agent.settings.update。
func (s *Server) handleAgentSettingsUpdate(params map[string]any) (any, *rpcError) {
	data, err := s.orchestrator.SettingsUpdate(params)
	return wrapOrchestratorResult(data, err)
}

// wrapOrchestratorResult 处理当前模块的相关逻辑。

// wrapOrchestratorResult 负责把 orchestrator 返回值映射成 RPC 层统一错误结构。
// 这里不做业务纠正，只负责错误码和 trace 信息的协议收口。
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
