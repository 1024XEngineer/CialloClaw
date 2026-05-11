package orchestrator

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/titlegen"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

// TaskInspectorConfigGet returns the inspector configuration projected from the
// formal task_automation settings snapshot.
func (s *Service) TaskInspectorConfigGet() (map[string]any, error) {
	return inspectorConfigFromSettings(s.runEngine.Settings()), nil
}

// TaskInspectorConfigUpdate stores inspector source changes through settings so
// the inspector does not maintain a parallel configuration truth.
func (s *Service) TaskInspectorConfigUpdate(params map[string]any) (map[string]any, error) {
	settingsPatch := taskAutomationSettingsPatchFromInspectorConfig(params)
	if _, _, _, _, err := s.runEngine.UpdateSettings(settingsPatch); err != nil {
		return nil, err
	}
	effective := inspectorConfigFromSettings(s.runEngine.Settings())
	return map[string]any{
		"updated":          true,
		"effective_config": effective,
	}, nil
}

// TaskInspectorRun runs the configured inspector and returns summary data plus
// suggestions without creating or mutating formal tasks.
func (s *Service) TaskInspectorRun(params map[string]any) (map[string]any, error) {
	config := inspectorConfigFromSettings(s.runEngine.Settings())
	targetSources := stringSliceValue(params["target_sources"])
	notepadItems, _ := s.runEngine.NotepadItems("", 0, 0)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)
	reason := stringValue(params, "reason", "")
	inspectionID := fmt.Sprintf("insp_%d", time.Now().UTC().UnixNano())

	result, err := s.inspector.Run(taskinspector.RunInput{
		Reason:       reason,
		InspectionID: inspectionID,
		// `agent.task_inspector.run` is a public RPC surface. Its caller-supplied
		// reason string is descriptive only and cannot serve as trusted desktop UI
		// provenance for exporting workspace note content to a remote title model.
		// Keep this RPC on the deterministic local fallback path until the backend
		// has a server-owned authorization/provenance signal for manual runs.
		AllowGeneratedTitles: false,
		TitleGenerationOwner: titlegen.GenerationOwner{TaskID: inspectionID, RunID: inspectionID},
		TargetSources:        targetSources,
		Config:               config,
		UnfinishedTasks:      unfinishedTasks,
		FinishedTasks:        finishedTasks,
		NotepadItems:         notepadItems,
	})
	if err != nil {
		return nil, err
	}
	// Manual title generation can spend model quota before note syncing writes the
	// derived notepad state. Persist the audit/trace owner first so later sync
	// failures do not erase already-consumed model usage from governance views.
	s.recordInspectorTitleGeneration(result.InspectionID, reason, result.TitleGenerationAuditData)
	if result.SourceSynced {
		if err := s.runEngine.SyncNotepadItems(result.NotepadItems); err != nil {
			return nil, fmt.Errorf("sync notepad items for inspection %s: %w", result.InspectionID, err)
		}
	}

	return map[string]any{
		"inspection_id": result.InspectionID,
		"summary":       result.Summary,
		"suggestions":   append([]string(nil), result.Suggestions...),
	}, nil
}

// recordInspectorTitleGeneration projects manual note-title generation into one
// durable audit and metering owner keyed by inspection_id, because inspector
// runs do not create formal tasks yet still spend model quota on workspace
// content.
func (s *Service) recordInspectorTitleGeneration(inspectionID string, reason string, invocations []taskinspector.TitleGenerationAuditRecord) {
	inspectionID = strings.TrimSpace(inspectionID)
	if s == nil || inspectionID == "" || len(invocations) == 0 {
		return
	}

	for _, invocation := range invocations {
		summary := "generate compact note title during manual inspector run"
		result := "success"
		outputText := "manual note title generated"
		if !invocation.Generated {
			summary = "note title model call fell back to local manual inspector title"
			result = "fallback"
			outputText = "manual note title fallback kept"
		}
		_, _ = s.audit.Write(context.Background(), audit.RecordInput{
			TaskID:  inspectionID,
			RunID:   inspectionID,
			Type:    "model",
			Action:  "note_title.generate",
			Summary: summary,
			Target:  firstNonEmptyString(strings.TrimSpace(reason), "task_inspector.run"),
			Result:  result,
		})
		if s.traceEval == nil {
			continue
		}
		traceResult, err := s.traceEval.Capture(traceeval.CaptureInput{
			TaskID:          inspectionID,
			RunID:           inspectionID,
			IntentName:      "task_inspector.generate_note_title",
			OutputText:      outputText,
			ModelInvocation: invocation.Invocation.Map(),
			TokenUsage:      modelInvocationTokenUsage(invocation.Invocation),
			DurationMS:      invocation.Invocation.LatencyMS,
		})
		if err == nil {
			_ = s.traceEval.Record(context.Background(), traceResult)
		}
	}
}

func modelInvocationTokenUsage(invocation model.InvocationRecord) map[string]any {
	return auxiliaryTokenUsageSummary{
		InputTokens:   invocation.Usage.InputTokens,
		OutputTokens:  invocation.Usage.OutputTokens,
		TotalTokens:   invocation.Usage.TotalTokens,
		EstimatedCost: 0.0,
	}.Map()
}

func inspectorConfigFromSettings(settings map[string]any) map[string]any {
	taskAutomation := cloneMap(mapValue(normalizeSettingsSnapshot(settings), "task_automation"))
	if taskAutomation == nil {
		taskAutomation = map[string]any{}
	}
	return map[string]any{
		"task_sources":           inspectorTaskSourcesFromSettings(taskAutomation["task_sources"]),
		"inspection_interval":    cloneMap(mapValue(taskAutomation, "inspection_interval")),
		"inspect_on_file_change": boolValue(taskAutomation, "inspect_on_file_change", true),
		"inspect_on_startup":     boolValue(taskAutomation, "inspect_on_startup", true),
		"remind_before_deadline": boolValue(taskAutomation, "remind_before_deadline", true),
		"remind_when_stale":      boolValue(taskAutomation, "remind_when_stale", false),
	}
}

// inspectorTaskSourcesFromSettings keeps compatibility RPCs aligned with the
// formal task_automation snapshot shape while preserving workspace-relative
// sources instead of eagerly migrating them to runtime absolute paths.
func inspectorTaskSourcesFromSettings(rawValue any) []string {
	sources, recognized := optionalStringSliceValue(rawValue)
	if recognized {
		result := make([]string, 0, len(sources))
		for _, source := range sources {
			result = append(result, presentInspectorTaskSource(source))
		}
		return result
	}
	return stringSliceValue(rawValue)
}

// presentInspectorTaskSource maps persisted runtime-absolute task sources back to
// the compatibility RPC shape expected by desktop inspector settings so the UI
// continues to reason about workspace-formal paths instead of host-specific
// runtime locations.
func presentInspectorTaskSource(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return ""
	}
	if !filepath.IsAbs(trimmed) {
		return trimmed
	}
	cleanSource := filepath.Clean(trimmed)
	workspaceRoot := filepath.Clean(serviceconfig.DefaultWorkspaceRoot())
	if relative, ok := relativizePathWithinRoot(cleanSource, workspaceRoot); ok {
		if relative == "" {
			return "workspace"
		}
		return filepath.ToSlash(path.Join("workspace", filepath.ToSlash(relative)))
	}
	runtimeRoot := filepath.Clean(serviceconfig.DefaultRuntimeRoot())
	if relative, ok := relativizePathWithinRoot(cleanSource, runtimeRoot); ok {
		if relative == "" {
			return "."
		}
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(cleanSource)
}

func relativizePathWithinRoot(candidate, root string) (string, bool) {
	if root == "" {
		return "", false
	}
	if candidate == root {
		return "", true
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", false
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return relative, true
}

func taskAutomationSettingsPatchFromInspectorConfig(params map[string]any) map[string]any {
	patch := map[string]any{}
	if rawSources, ok := params["task_sources"]; ok {
		if sources, recognized := optionalStringSliceValue(rawSources); recognized {
			patch["task_sources"] = sources
		}
	}
	if interval := cloneMap(mapValue(params, "inspection_interval")); len(interval) > 0 {
		patch["inspection_interval"] = interval
	}
	for _, key := range []string{"inspect_on_file_change", "inspect_on_startup", "remind_before_deadline", "remind_when_stale"} {
		if value, ok := params[key].(bool); ok {
			patch[key] = value
		}
	}
	if len(patch) == 0 {
		return map[string]any{}
	}
	return map[string]any{"task_automation": patch}
}

// optionalStringSliceValue preserves the difference between an omitted field and
// an explicitly empty list so compatibility RPCs can clear task sources without
// leaving stale workspace scan roots behind.
func optionalStringSliceValue(rawValue any) ([]string, bool) {
	switch values := rawValue.(type) {
	case []string:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			result = append(result, value)
		}
		return result, true
	case []any:
		result := make([]string, 0, len(values))
		for _, rawItem := range values {
			item, ok := rawItem.(string)
			if !ok || strings.TrimSpace(item) == "" {
				continue
			}
			result = append(result, item)
		}
		return result, true
	default:
		return nil, false
	}
}
