package orchestrator

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

func storageTaskRunRecordFromSnapshotJSON(payload string) (storage.TaskRunRecord, error) {
	var record storage.TaskRunRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return storage.TaskRunRecord{}, err
	}
	return record, nil
}

func timelineFromStorage(timeline []storage.TaskStepSnapshot) []runengine.TaskStepRecord {
	if len(timeline) == 0 {
		return nil
	}
	result := make([]runengine.TaskStepRecord, len(timeline))
	for index, step := range timeline {
		result[index] = runengine.TaskStepRecord{
			StepID:        step.StepID,
			TaskID:        step.TaskID,
			Name:          step.Name,
			Status:        step.Status,
			OrderIndex:    step.OrderIndex,
			InputSummary:  step.InputSummary,
			OutputSummary: step.OutputSummary,
		}
	}
	return result
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// notepadSnapshot rebuilds a free-text task snapshot from one notepad item so
// convert_to_task can reuse the normal task pipeline without narrowing the
// user's goal into a notepad-specific intent bucket first.
func notepadSnapshot(item map[string]any) taskcontext.TaskContextSnapshot {
	text := firstNonEmptyString(
		stringValue(item, "note_text", ""),
		stringValue(item, "title", ""),
	)

	return taskcontext.TaskContextSnapshot{
		Source:    "dashboard",
		Trigger:   "recommendation_click",
		InputType: "text",
		InputMode: "text",
		Text:      text,
		Files:     notepadResourcePaths(item),
		PageTitle: "notepad",
		AppName:   "dashboard",
	}
}

func notepadResourcePaths(item map[string]any) []string {
	resources := relatedResourceMaps(item["related_resources"])
	if len(resources) == 0 {
		return nil
	}

	paths := make([]string, 0, len(resources))
	for _, resource := range resources {
		// Derived defaults keep notepad cards actionable in the dashboard, but
		// they must not widen the formal task snapshot as if the user attached
		// those paths explicitly.
		if strings.TrimSpace(stringValue(resource, "resource_origin", "")) == "derived_default" {
			continue
		}
		path := strings.TrimSpace(stringValue(resource, "path", ""))
		if path == "" {
			continue
		}

		resourceType := firstNonEmptyString(
			stringValue(resource, "resource_type", ""),
			stringValue(resource, "type", ""),
		)
		switch resourceType {
		case "file", "folder", "directory":
			paths = append(paths, path)
		case "":
			switch strings.TrimSpace(stringValue(resource, "target_kind", "")) {
			case "file", "folder":
				paths = append(paths, path)
			}
		default:
			if strings.TrimSpace(stringValue(resource, "target_kind", "")) == "folder" {
				paths = append(paths, path)
			}
		}
	}

	if len(paths) == 0 {
		return nil
	}
	return paths
}

func relatedResourceMaps(rawValue any) []map[string]any {
	if resources, ok := rawValue.([]map[string]any); ok {
		return cloneMapSlice(resources)
	}
	anyResources, ok := rawValue.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(anyResources))
	for _, rawResource := range anyResources {
		resource, ok := rawResource.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, cloneMap(resource))
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func latestOutputPathFromTasks(tasks []runengine.TaskRecord) string {
	for _, task := range tasks {
		for _, artifact := range task.Artifacts {
			if outputPath := stringValue(artifact, "path", ""); outputPath != "" {
				return outputPath
			}
		}
		if outputPath := pathFromDeliveryResult(task.DeliveryResult); outputPath != "" {
			return outputPath
		}
		if outputPath := stringValue(task.StorageWritePlan, "target_path", ""); outputPath != "" {
			return outputPath
		}
	}
	return ""
}

// snapshotFromTask rebuilds the minimum context snapshot needed for resume and
// other post-creation flows.
func snapshotFromTask(task runengine.TaskRecord) taskcontext.TaskContextSnapshot {
	if !isEmptySnapshot(task.Snapshot) {
		return cloneTaskSnapshot(task.Snapshot)
	}
	return taskcontext.TaskContextSnapshot{
		Trigger:   task.SourceType,
		InputType: "text",
		Text:      originalTextFromTaskTitle(task.Title),
	}
}

func cloneTaskSnapshot(snapshot taskcontext.TaskContextSnapshot) taskcontext.TaskContextSnapshot {
	cloned := snapshot
	if len(snapshot.Files) > 0 {
		cloned.Files = append([]string(nil), snapshot.Files...)
	}
	return cloned
}

func isEmptySnapshot(snapshot taskcontext.TaskContextSnapshot) bool {
	return strings.TrimSpace(snapshot.Source) == "" &&
		strings.TrimSpace(snapshot.Trigger) == "" &&
		strings.TrimSpace(snapshot.InputType) == "" &&
		strings.TrimSpace(snapshot.InputMode) == "" &&
		strings.TrimSpace(snapshot.Text) == "" &&
		strings.TrimSpace(snapshot.SelectionText) == "" &&
		strings.TrimSpace(snapshot.ErrorText) == "" &&
		len(snapshot.Files) == 0 &&
		strings.TrimSpace(snapshot.PageTitle) == "" &&
		strings.TrimSpace(snapshot.PageURL) == "" &&
		strings.TrimSpace(snapshot.AppName) == "" &&
		strings.TrimSpace(snapshot.BrowserKind) == "" &&
		strings.TrimSpace(snapshot.ProcessPath) == "" &&
		snapshot.ProcessID == 0 &&
		strings.TrimSpace(snapshot.WindowTitle) == "" &&
		strings.TrimSpace(snapshot.VisibleText) == "" &&
		strings.TrimSpace(snapshot.ScreenSummary) == "" &&
		strings.TrimSpace(snapshot.ClipboardText) == "" &&
		strings.TrimSpace(snapshot.HoverTarget) == "" &&
		strings.TrimSpace(snapshot.LastAction) == "" &&
		snapshot.DwellMillis == 0 &&
		snapshot.CopyCount == 0 &&
		snapshot.WindowSwitches == 0 &&
		snapshot.PageSwitches == 0
}

func originalTextFromTaskTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	for _, prefix := range presentation.TaskTitlePrefixes() {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return trimmed
}

func confirmationTitleFromTask(task runengine.TaskRecord) string {
	subject := strings.TrimSpace(originalTextFromTaskTitle(task.Title))
	if subject == "" {
		subject = presentation.Text(presentation.MessageTaskTitleCurrentTask, nil)
	}
	return presentation.TaskTitle("", presentation.TaskTitleOptions{Subject: subject})
}

// mergeSuggestedDeliveryPreference preserves explicit caller preferences and only
// falls back to the intent layer's suggested delivery when the caller left the
// preferred delivery unset.
func mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, suggestedDelivery string) (string, string) {
	if strings.TrimSpace(preferredDelivery) == "" && strings.TrimSpace(suggestedDelivery) != "" {
		preferredDelivery = suggestedDelivery
	}
	return preferredDelivery, fallbackDelivery
}

// buildPendingExecution creates the minimum delivery plan required to resume a
// task after authorization. The stored plan must be deterministic and task-
// centric because waiting_auth can outlive the original request and later needs
// to restart execution without recomputing delivery intent from transport-only
// inputs.
func (s *Service) buildPendingExecution(task runengine.TaskRecord, taskIntent map[string]any) map[string]any {
	plan := s.delivery.BuildApprovalExecutionPlan(task.TaskID, taskIntent)
	return s.applyResolvedDeliveryToPlan(task, plan, taskIntent)
}
