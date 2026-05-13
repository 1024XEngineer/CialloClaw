package orchestrator

import (
	"context"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

// PluginRuntimeList exposes the smallest backend query surface for runtime
// plugin visibility so dashboard work can consume health and metric snapshots
// without depending on static worker declarations only.
func (s *Service) PluginRuntimeList(params map[string]any) (map[string]any, error) {
	_ = params
	snapshots := pluginCatalogSnapshots(s.plugin)
	if len(snapshots) == 0 {
		return map[string]any{"items": []map[string]any{}, "metrics": []map[string]any{}, "events": []map[string]any{}}, nil
	}
	runtimes := pluginSnapshotRuntimes(snapshots)
	metrics := pluginSnapshotMetrics(snapshots)
	events := pluginSnapshotEvents(snapshots)
	return map[string]any{
		"items":   pluginRuntimeItems(runtimes),
		"metrics": pluginMetricItems(metrics),
		"events":  pluginEventItems(events),
	}, nil
}

// SecuritySummaryGet returns the dashboard security summary by merging task,
// governance, audit, recovery, and budget signals into one read-only payload.
func (s *Service) SecuritySummaryGet() (map[string]any, error) {
	_, runtimePendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	queryViews := newTaskQueryViews(s)
	unfinishedTasks := queryViews.tasks("unfinished", "updated_at", "desc")
	finishedTasks := queryViews.tasks("finished", "finished_at", "desc")
	pendingTotal := mergedPendingApprovalTotal(unfinishedTasks, runtimePendingTotal)
	allTasks := append(append([]runengine.TaskRecord{}, unfinishedTasks...), finishedTasks...)
	modelCredentials := modelCredentialSettings(s.runEngine.Settings())
	latestRestorePoint := latestRestorePointFromTasks(allTasks)
	if latestRestorePoint == nil {
		latestRestorePoint = s.latestRestorePointFromStorage("")
	}
	return map[string]any{
		"summary": map[string]any{
			"security_status":        aggregateSecurityStatus(allTasks, pendingTotal),
			"pending_authorizations": pendingTotal,
			"latest_restore_point":   latestRestorePoint,
			"token_cost_summary":     aggregateTokenCostSummary(unfinishedTasks, finishedTasks, boolValue(modelCredentials, "budget_auto_downgrade", true), s.inspectorTitleGenerationUsage(context.Background())),
		},
	}, nil
}

func (s *Service) pluginRuntimeSummary() map[string]any {
	snapshots := pluginCatalogSnapshots(s.plugin)
	if len(snapshots) == 0 {
		return map[string]any{
			"total":       0,
			"healthy":     0,
			"failed":      0,
			"unavailable": 0,
		}
	}
	runtimes := pluginSnapshotRuntimes(snapshots)
	summary := map[string]any{
		"total":       len(runtimes),
		"healthy":     0,
		"failed":      0,
		"unavailable": 0,
	}
	for _, runtime := range runtimes {
		switch runtime.Health {
		case plugin.RuntimeHealthHealthy:
			summary["healthy"] = intValue(summary, "healthy", 0) + 1
		case plugin.RuntimeHealthFailed:
			summary["failed"] = intValue(summary, "failed", 0) + 1
		case plugin.RuntimeHealthUnavailable:
			summary["unavailable"] = intValue(summary, "unavailable", 0) + 1
		}
	}
	return summary
}

func pluginRuntimeItems(items []plugin.RuntimeState) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry := map[string]any{
			"name":         item.Name,
			"kind":         item.Kind,
			"status":       item.Status,
			"transport":    item.Transport,
			"health":       item.Health,
			"last_seen_at": item.LastSeenAt,
			"last_error":   item.LastError,
			"capabilities": append([]string(nil), item.Capabilities...),
		}
		if item.Manifest != nil {
			entry["manifest"] = map[string]any{
				"plugin_id":    item.Manifest.PluginID,
				"name":         item.Manifest.Name,
				"version":      item.Manifest.Version,
				"entry":        item.Manifest.Entry,
				"source":       item.Manifest.Source,
				"capabilities": append([]string(nil), item.Manifest.Capabilities...),
				"permissions":  append([]string(nil), item.Manifest.Permissions...),
			}
		}
		result = append(result, entry)
	}
	return result
}

func pluginMetricItems(items []plugin.MetricSnapshot) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"name":            item.Name,
			"kind":            item.Kind,
			"start_count":     item.StartCount,
			"success_count":   item.SuccessCount,
			"failure_count":   item.FailureCount,
			"last_started_at": item.LastStartedAt,
			"last_failed_at":  item.LastFailedAt,
			"last_seen_at":    item.LastSeenAt,
		})
	}
	return result
}

func pluginEventItems(items []plugin.RuntimeEvent) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"name":       item.Name,
			"kind":       item.Kind,
			"event_type": item.EventType,
			"payload":    cloneMap(item.Payload),
			"created_at": item.CreatedAt,
		})
	}
	return result
}
