package rpc

import "encoding/json"

const (
	methodAgentInputSubmit                  = "agent.input.submit"
	methodAgentTaskStart                    = "agent.task.start"
	methodAgentTaskConfirm                  = "agent.task.confirm"
	methodAgentRecommendationGet            = "agent.recommendation.get"
	methodAgentRecommendationFeedbackSubmit = "agent.recommendation.feedback.submit"
	methodAgentTaskList                     = "agent.task.list"
	methodAgentTaskDetailGet                = "agent.task.detail.get"
	methodAgentTaskEventsList               = "agent.task.events.list"
	methodAgentTaskToolCallsList            = "agent.task.tool_calls.list"
	methodAgentTaskSteer                    = "agent.task.steer"
	methodAgentTaskArtifactList             = "agent.task.artifact.list"
	methodAgentTaskArtifactOpen             = "agent.task.artifact.open"
	methodAgentTaskControl                  = "agent.task.control"
	methodAgentTaskInspectorConfigGet       = "agent.task_inspector.config.get"
	methodAgentTaskInspectorConfigUpdate    = "agent.task_inspector.config.update"
	methodAgentTaskInspectorRun             = "agent.task_inspector.run"
	methodAgentNotepadList                  = "agent.notepad.list"
	methodAgentNotepadConvertToTask         = "agent.notepad.convert_to_task"
	methodAgentNotepadUpdate                = "agent.notepad.update"
	methodAgentDashboardOverviewGet         = "agent.dashboard.overview.get"
	methodAgentDashboardModuleGet           = "agent.dashboard.module.get"
	methodAgentMirrorOverviewGet            = "agent.mirror.overview.get"
	methodAgentSecuritySummaryGet           = "agent.security.summary.get"
	methodAgentSecurityAuditList            = "agent.security.audit.list"
	methodAgentSecurityRestorePointsList    = "agent.security.restore_points.list"
	methodAgentSecurityRestoreApply         = "agent.security.restore.apply"
	methodAgentSecurityPendingList          = "agent.security.pending.list"
	methodAgentSecurityRespond              = "agent.security.respond"
	methodAgentDeliveryOpen                 = "agent.delivery.open"
	methodAgentSettingsGet                  = "agent.settings.get"
	methodAgentSettingsUpdate               = "agent.settings.update"
	methodAgentSettingsModelValidate        = "agent.settings.model.validate"
	methodAgentPluginRuntimeList            = "agent.plugin.runtime.list"
	methodAgentPluginList                   = "agent.plugin.list"
	methodAgentPluginDetailGet              = "agent.plugin.detail.get"
)

type methodSpec struct {
	Name   string
	Decode func(json.RawMessage) (map[string]any, *rpcError)
}

type registeredMethod struct {
	methodSpec
	Handle methodHandler
}

// stableMethodRegistry is the Go-side mirror of packages/protocol/rpc/methods.ts.
// The RPC layer owns method decoding so orchestrator code receives one
// normalized entry payload instead of raw transport envelopes.
func (s *Server) stableMethodRegistry() []registeredMethod {
	return []registeredMethod{
		registered(methodAgentInputSubmit, decodeAgentInputSubmitParams, s.handleAgentInputSubmit),
		registered(methodAgentTaskStart, decodeAgentTaskStartParams, s.handleAgentTaskStart),
		registered(methodAgentTaskConfirm, decodeAgentTaskConfirmParams, s.handleAgentTaskConfirm),
		registered(methodAgentRecommendationGet, decodeParams, s.handleAgentRecommendationGet),
		registered(methodAgentRecommendationFeedbackSubmit, decodeParams, s.handleAgentRecommendationFeedbackSubmit),
		registered(methodAgentTaskList, decodeParams, s.handleAgentTaskList),
		registered(methodAgentTaskDetailGet, decodeParams, s.handleAgentTaskDetailGet),
		registered(methodAgentTaskEventsList, decodeParams, s.handleAgentTaskEventsList),
		registered(methodAgentTaskToolCallsList, decodeParams, s.handleAgentTaskToolCallsList),
		registered(methodAgentTaskSteer, decodeParams, s.handleAgentTaskSteer),
		registered(methodAgentTaskArtifactList, decodeParams, s.handleAgentTaskArtifactList),
		registered(methodAgentTaskArtifactOpen, decodeParams, s.handleAgentTaskArtifactOpen),
		registered(methodAgentTaskControl, decodeParams, s.handleAgentTaskControl),
		registered(methodAgentTaskInspectorConfigGet, decodeParams, s.handleAgentTaskInspectorConfigGet),
		registered(methodAgentTaskInspectorConfigUpdate, decodeParams, s.handleAgentTaskInspectorConfigUpdate),
		registered(methodAgentTaskInspectorRun, decodeParams, s.handleAgentTaskInspectorRun),
		registered(methodAgentNotepadList, decodeParams, s.handleAgentNotepadList),
		registered(methodAgentNotepadConvertToTask, decodeParams, s.handleAgentNotepadConvertToTask),
		registered(methodAgentNotepadUpdate, decodeParams, s.handleAgentNotepadUpdate),
		registered(methodAgentDashboardOverviewGet, decodeParams, s.handleAgentDashboardOverviewGet),
		registered(methodAgentDashboardModuleGet, decodeParams, s.handleAgentDashboardModuleGet),
		registered(methodAgentMirrorOverviewGet, decodeParams, s.handleAgentMirrorOverviewGet),
		registered(methodAgentSecuritySummaryGet, decodeParams, s.handleAgentSecuritySummaryGet),
		registered(methodAgentSecurityAuditList, decodeParams, s.handleAgentSecurityAuditList),
		registered(methodAgentSecurityRestorePointsList, decodeParams, s.handleAgentSecurityRestorePointsList),
		registered(methodAgentSecurityRestoreApply, decodeParams, s.handleAgentSecurityRestoreApply),
		registered(methodAgentSecurityPendingList, decodeParams, s.handleAgentSecurityPendingList),
		registered(methodAgentSecurityRespond, decodeParams, s.handleAgentSecurityRespond),
		registered(methodAgentDeliveryOpen, decodeParams, s.handleAgentDeliveryOpen),
		registered(methodAgentSettingsGet, decodeParams, s.handleAgentSettingsGet),
		registered(methodAgentSettingsUpdate, decodeParams, s.handleAgentSettingsUpdate),
		registered(methodAgentSettingsModelValidate, decodeParams, s.handleAgentSettingsModelValidate),
		registered(methodAgentPluginRuntimeList, decodeParams, s.handleAgentPluginRuntimeList),
		registered(methodAgentPluginList, decodeParams, s.handleAgentPluginList),
		registered(methodAgentPluginDetailGet, decodeParams, s.handleAgentPluginDetailGet),
	}
}

func registered(name string, decode func(json.RawMessage) (map[string]any, *rpcError), handle methodHandler) registeredMethod {
	return registeredMethod{
		methodSpec: methodSpec{
			Name:   name,
			Decode: decode,
		},
		Handle: handle,
	}
}
