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
		registered(methodAgentTaskConfirm, decodeParamsRequiringRequestMeta, s.handleAgentTaskConfirm),
		registered(methodAgentRecommendationGet, decodeParamsRequiringRequestMeta, s.handleAgentRecommendationGet),
		registered(methodAgentRecommendationFeedbackSubmit, decodeParamsRequiringRequestMeta, s.handleAgentRecommendationFeedbackSubmit),
		registered(methodAgentTaskList, decodeAgentTaskListParams, s.handleAgentTaskList),
		registered(methodAgentTaskDetailGet, decodeAgentTaskDetailGetParams, s.handleAgentTaskDetailGet),
		registered(methodAgentTaskEventsList, decodeParamsRequiringRequestMeta, s.handleAgentTaskEventsList),
		registered(methodAgentTaskToolCallsList, decodeParamsRequiringRequestMeta, s.handleAgentTaskToolCallsList),
		registered(methodAgentTaskSteer, decodeParamsRequiringRequestMeta, s.handleAgentTaskSteer),
		registered(methodAgentTaskArtifactList, decodeParamsRequiringRequestMeta, s.handleAgentTaskArtifactList),
		registered(methodAgentTaskArtifactOpen, decodeParamsRequiringRequestMeta, s.handleAgentTaskArtifactOpen),
		registered(methodAgentTaskControl, decodeParamsRequiringRequestMeta, s.handleAgentTaskControl),
		registered(methodAgentTaskInspectorConfigGet, decodeParamsRequiringRequestMeta, s.handleAgentTaskInspectorConfigGet),
		registered(methodAgentTaskInspectorConfigUpdate, decodeParamsRequiringRequestMeta, s.handleAgentTaskInspectorConfigUpdate),
		registered(methodAgentTaskInspectorRun, decodeParamsRequiringRequestMeta, s.handleAgentTaskInspectorRun),
		registered(methodAgentNotepadList, decodeParamsRequiringRequestMeta, s.handleAgentNotepadList),
		registered(methodAgentNotepadConvertToTask, decodeParamsRequiringRequestMeta, s.handleAgentNotepadConvertToTask),
		registered(methodAgentNotepadUpdate, decodeParamsRequiringRequestMeta, s.handleAgentNotepadUpdate),
		registered(methodAgentDashboardOverviewGet, decodeParamsRequiringRequestMeta, s.handleAgentDashboardOverviewGet),
		registered(methodAgentDashboardModuleGet, decodeParamsRequiringRequestMeta, s.handleAgentDashboardModuleGet),
		registered(methodAgentMirrorOverviewGet, decodeParamsRequiringRequestMeta, s.handleAgentMirrorOverviewGet),
		registered(methodAgentSecuritySummaryGet, decodeParamsRequiringRequestMeta, s.handleAgentSecuritySummaryGet),
		registered(methodAgentSecurityAuditList, decodeParamsRequiringRequestMeta, s.handleAgentSecurityAuditList),
		registered(methodAgentSecurityRestorePointsList, decodeParamsRequiringRequestMeta, s.handleAgentSecurityRestorePointsList),
		registered(methodAgentSecurityRestoreApply, decodeParamsRequiringRequestMeta, s.handleAgentSecurityRestoreApply),
		registered(methodAgentSecurityPendingList, decodeParamsRequiringRequestMeta, s.handleAgentSecurityPendingList),
		registered(methodAgentSecurityRespond, decodeParamsRequiringRequestMeta, s.handleAgentSecurityRespond),
		registered(methodAgentDeliveryOpen, decodeParamsRequiringRequestMeta, s.handleAgentDeliveryOpen),
		registered(methodAgentSettingsGet, decodeParamsRequiringRequestMeta, s.handleAgentSettingsGet),
		registered(methodAgentSettingsUpdate, decodeParamsRequiringRequestMeta, s.handleAgentSettingsUpdate),
		registered(methodAgentSettingsModelValidate, decodeParamsRequiringRequestMeta, s.handleAgentSettingsModelValidate),
		registered(methodAgentPluginRuntimeList, decodeParamsRequiringRequestMeta, s.handleAgentPluginRuntimeList),
		registered(methodAgentPluginList, decodeParamsRequiringRequestMeta, s.handleAgentPluginList),
		registered(methodAgentPluginDetailGet, decodeParamsRequiringRequestMeta, s.handleAgentPluginDetailGet),
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
