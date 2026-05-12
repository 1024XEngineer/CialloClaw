package orchestrator

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
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
// user's goal into a notepad-specific intent bucket first. Title-only notes
// must stay anchored on the original title instead of the synthetic note_text
// fallback that the dashboard uses for display-only notepad cards.
func notepadSnapshot(item map[string]any, workspaceRoot string) taskcontext.TaskContextSnapshot {
	text := notepadSnapshotText(item)

	return taskcontext.TaskContextSnapshot{
		Source:    "dashboard",
		Trigger:   "recommendation_click",
		InputType: "text",
		InputMode: "text",
		Text:      text,
		Files:     notepadResourcePaths(item, workspaceRoot),
		PageTitle: "notepad",
		AppName:   "dashboard",
	}
}

func notepadSnapshotText(item map[string]any) string {
	title := strings.TrimSpace(stringValue(item, "title", ""))
	noteText := strings.TrimSpace(stringValue(item, "note_text", ""))
	if noteText == "" {
		return title
	}
	if title == "" {
		return noteText
	}

	// Title-only notes receive a synthetic note_text for dashboard display, but
	// task routing must only collapse back to title when the note body was
	// synthesized by runtime normalization instead of authored by the user.
	if strings.TrimSpace(stringValue(item, "note_text_origin", "")) == "derived_default" {
		return title
	}
	return noteText
}

func notepadTaskTitle(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	if strings.TrimSpace(snapshot.Text) == "" {
		return suggestion.TaskTitle
	}

	titleSnapshot := snapshot
	titleSnapshot.Files = nil
	return intent.NewService().Suggest(titleSnapshot, suggestion.Intent, suggestion.RequiresConfirm).TaskTitle
}

func notepadResourcePaths(item map[string]any, workspaceRoot string) []string {
	resources := relatedResourceMaps(item["related_resources"])
	if len(resources) == 0 {
		return nil
	}

	paths := make([]string, 0, len(resources))
	seen := make(map[string]struct{}, len(resources))
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

		switch notepadResourceTargetKind(resource) {
		case "file":
			path, ok := normalizeNotepadSnapshotPath(path, workspaceRoot)
			if !ok {
				continue
			}
			if _, duplicated := seen[path]; duplicated {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}

	if len(paths) == 0 {
		return nil
	}
	return paths
}

// normalizeNotepadSnapshotPath only promotes explicit note resources that stay
// inside the current workspace root. Snapshot file inputs must keep the same
// workspace-formal shape as other desktop task entry points so execution does
// not accidentally treat arbitrary host paths as model-readable context.
func normalizeNotepadSnapshotPath(resourcePath, workspaceRoot string) (string, bool) {
	trimmedPath := strings.TrimSpace(resourcePath)
	trimmedRoot := strings.TrimSpace(workspaceRoot)
	if trimmedPath == "" || trimmedRoot == "" {
		return "", false
	}
	if !filepath.IsAbs(trimmedPath) && !hasWindowsDriveLetterPrefix(trimmedPath) {
		normalized := strings.Trim(strings.ReplaceAll(trimmedPath, "\\", "/"), "/")
		if normalized == "workspace" {
			trimmedPath = "."
		} else if strings.HasPrefix(normalized, "workspace/") {
			trimmedPath = strings.TrimPrefix(normalized, "workspace/")
		}
	}

	pathPolicy, err := platform.NewLocalPathPolicy(trimmedRoot)
	if err != nil {
		return "", false
	}
	safePath, err := pathPolicy.EnsureWithinWorkspace(trimmedPath)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(filepath.Clean(safePath))
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}

	relative, ok := relativizePathWithinRoot(filepath.Clean(safePath), filepath.Clean(trimmedRoot))
	if !ok {
		return "", false
	}
	if relative == "" {
		return "workspace", true
	}
	return filepath.ToSlash(path.Join("workspace", filepath.ToSlash(relative))), true
}

func notepadResourceTargetKind(resource map[string]any) string {
	if targetKind := strings.TrimSpace(stringValue(resource, "target_kind", "")); targetKind != "" {
		return targetKind
	}
	switch strings.TrimSpace(stringValue(resource, "open_action", "")) {
	case "open_file":
		return "file"
	case "reveal_in_folder":
		return "folder"
	}
	switch strings.TrimSpace(firstNonEmptyString(
		stringValue(resource, "resource_type", ""),
		stringValue(resource, "type", ""),
	)) {
	case "file":
		return "file"
	case "folder", "directory":
		return "folder"
	default:
		return ""
	}
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
	return subject
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
