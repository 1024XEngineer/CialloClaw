package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

const implicitSessionReuseWindow = 15 * time.Minute

type taskContinuationDecision struct {
	Decision string `json:"decision"`
	TaskID   string `json:"task_id"`
	Reason   string `json:"reason"`
}

func (s *Service) maybeContinueExistingTask(params map[string]any, snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any) (map[string]any, bool, string, error) {
	explicitSessionID := strings.TrimSpace(stringValue(params, "session_id", ""))
	candidates := s.continuationCandidates(explicitSessionID)
	decision := s.classifyTaskContinuation(snapshot, explicitIntent, candidates)
	if decision.Decision == "continue" && strings.TrimSpace(decision.TaskID) != "" {
		task, ok := s.loadTaskForContinuation(decision.TaskID)
		if !ok {
			return nil, false, explicitSessionID, nil
		}
		response, err := s.continueTask(task, snapshot, explicitIntent, decision)
		if err != nil {
			return nil, false, explicitSessionID, err
		}
		return response, true, task.SessionID, nil
	}

	if explicitSessionID != "" {
		return nil, false, explicitSessionID, nil
	}
	return nil, false, s.resolveImplicitSessionID(candidates), nil
}

func (s *Service) continuationCandidates(sessionID string) []runengine.TaskRecord {
	queryViews := newTaskQueryViews(s)
	tasks := queryViews.tasks("unfinished", "updated_at", "desc")
	result := make([]runengine.TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(sessionID) != "" && task.SessionID != strings.TrimSpace(sessionID) {
			continue
		}
		if !canContinueTask(task) {
			continue
		}
		result = append(result, task)
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func canContinueTask(task runengine.TaskRecord) bool {
	switch task.Status {
	case "confirming_intent", "processing", "waiting_auth", "waiting_input", "paused", "blocked":
		return true
	default:
		return false
	}
}

func (s *Service) classifyTaskContinuation(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, candidates []runengine.TaskRecord) taskContinuationDecision {
	if len(candidates) == 0 {
		return taskContinuationDecision{Decision: "new_task", Reason: "no unfinished candidate task"}
	}
	if decision, ok := s.modelTaskContinuationDecision(snapshot, explicitIntent, candidates); ok {
		return decision
	}
	return heuristicTaskContinuationDecision(snapshot, explicitIntent, candidates)
}

func (s *Service) modelTaskContinuationDecision(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, candidates []runengine.TaskRecord) (taskContinuationDecision, bool) {
	if s == nil || s.model == nil {
		return taskContinuationDecision{}, false
	}
	response, err := s.model.GenerateText(context.Background(), model.GenerateTextRequest{
		TaskID: "task_continuation_classifier",
		RunID:  "run_continuation_classifier",
		Input:  buildTaskContinuationPrompt(snapshot, explicitIntent, candidates),
	})
	if err != nil {
		return taskContinuationDecision{}, false
	}
	decision, ok := parseTaskContinuationDecision(response.OutputText, candidates)
	return decision, ok
}

func buildTaskContinuationPrompt(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, candidates []runengine.TaskRecord) string {
	lines := []string{
		"You decide whether one new desktop input should continue an existing task or start a new task.",
		"Return JSON only.",
		`Schema: {"decision":"continue"|"new_task","task_id":"task_xxx or empty","reason":"short reason"}`,
		"Choose continue only when the new input is clearly refining, correcting, narrowing, or attaching evidence for the same ongoing task.",
		"Choose new_task when the input starts another goal, another deliverable, or another analysis target.",
		"",
		"New input:",
		taskContinuationInputSummary(snapshot, explicitIntent),
		"",
		"Candidate unfinished tasks:",
	}
	for _, candidate := range candidates {
		lines = append(lines, taskContinuationCandidateSummary(candidate))
	}
	return strings.Join(lines, "\n")
}

func taskContinuationInputSummary(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any) string {
	parts := []string{
		fmt.Sprintf("trigger=%s", snapshot.Trigger),
		fmt.Sprintf("input_type=%s", snapshot.InputType),
	}
	if text := strings.TrimSpace(snapshot.Text); text != "" {
		parts = append(parts, "text="+truncateText(text, 280))
	}
	if selectionText := strings.TrimSpace(snapshot.SelectionText); selectionText != "" && selectionText != strings.TrimSpace(snapshot.Text) {
		parts = append(parts, "selection_text="+truncateText(selectionText, 280))
	}
	if errorText := strings.TrimSpace(snapshot.ErrorText); errorText != "" {
		parts = append(parts, "error_text="+truncateText(errorText, 280))
	}
	if len(snapshot.Files) > 0 {
		parts = append(parts, "files="+strings.Join(snapshot.Files, ", "))
	}
	if pageTitle := strings.TrimSpace(snapshot.PageTitle); pageTitle != "" {
		parts = append(parts, "page_title="+pageTitle)
	}
	if appName := strings.TrimSpace(snapshot.AppName); appName != "" {
		parts = append(parts, "app_name="+appName)
	}
	if len(explicitIntent) > 0 {
		if payload, err := json.Marshal(explicitIntent); err == nil {
			parts = append(parts, "explicit_intent="+string(payload))
		}
	}
	return strings.Join(parts, " | ")
}

func taskContinuationCandidateSummary(task runengine.TaskRecord) string {
	parts := []string{
		fmt.Sprintf("- task_id=%s", task.TaskID),
		fmt.Sprintf("session_id=%s", task.SessionID),
		fmt.Sprintf("status=%s", task.Status),
		fmt.Sprintf("current_step=%s", task.CurrentStep),
		fmt.Sprintf("title=%s", task.Title),
	}
	if intentName := strings.TrimSpace(stringValue(task.Intent, "name", "")); intentName != "" {
		parts = append(parts, "intent="+intentName)
	}
	if snapshotText := strings.TrimSpace(snapshotFromTask(task).Text); snapshotText != "" {
		parts = append(parts, "summary="+truncateText(snapshotText, 200))
	}
	if len(task.Snapshot.Files) > 0 {
		parts = append(parts, "files="+strings.Join(task.Snapshot.Files, ", "))
	}
	return strings.Join(parts, " | ")
}

func parseTaskContinuationDecision(raw string, candidates []runengine.TaskRecord) (taskContinuationDecision, bool) {
	source := strings.TrimSpace(raw)
	start := strings.Index(source, "{")
	end := strings.LastIndex(source, "}")
	if start < 0 || end <= start {
		return taskContinuationDecision{}, false
	}
	var decision taskContinuationDecision
	if err := json.Unmarshal([]byte(source[start:end+1]), &decision); err != nil {
		return taskContinuationDecision{}, false
	}
	switch decision.Decision {
	case "new_task":
		return decision, true
	case "continue":
		for _, candidate := range candidates {
			if candidate.TaskID == strings.TrimSpace(decision.TaskID) {
				decision.TaskID = candidate.TaskID
				return decision, true
			}
		}
	}
	return taskContinuationDecision{}, false
}

func heuristicTaskContinuationDecision(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, candidates []runengine.TaskRecord) taskContinuationDecision {
	if len(candidates) != 1 {
		return taskContinuationDecision{Decision: "new_task", Reason: "multiple unfinished candidates"}
	}
	candidate := candidates[0]
	if shouldHeuristicallyContinueTask(snapshot, explicitIntent, candidate) {
		return taskContinuationDecision{
			Decision: "continue",
			TaskID:   candidate.TaskID,
			Reason:   "fallback follow-up heuristic matched the latest unfinished task",
		}
	}
	return taskContinuationDecision{Decision: "new_task", Reason: "fallback heuristic treated the input as a new top-level request"}
}

func shouldHeuristicallyContinueTask(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, candidate runengine.TaskRecord) bool {
	if len(snapshot.Files) > 0 {
		return true
	}
	if intentName := strings.TrimSpace(stringValue(explicitIntent, "name", "")); intentName != "" && intentName == strings.TrimSpace(stringValue(candidate.Intent, "name", "")) {
		return true
	}
	combined := strings.ToLower(strings.Join([]string{snapshot.Text, snapshot.SelectionText, snapshot.ErrorText}, " "))
	if continuationContainsAny(combined, "补充", "继续", "另外", "再", "重点", "不要", "改成", "顺便", "follow-up", "continue", "also", "instead", "focus", "title", "output") {
		return sameContinuationContext(snapshot, snapshotFromTask(candidate))
	}
	return false
}

func sameContinuationContext(current, previous contextsvc.TaskContextSnapshot) bool {
	if strings.TrimSpace(current.PageURL) != "" && current.PageURL == previous.PageURL {
		return true
	}
	if strings.TrimSpace(current.PageTitle) != "" && current.PageTitle == previous.PageTitle {
		return true
	}
	if strings.TrimSpace(current.WindowTitle) != "" && current.WindowTitle == previous.WindowTitle {
		return true
	}
	if strings.TrimSpace(current.AppName) != "" && current.AppName == previous.AppName {
		return true
	}
	return previous.PageTitle == "" && previous.WindowTitle == "" && previous.AppName == ""
}

func (s *Service) continueTask(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, decision taskContinuationDecision) (map[string]any, error) {
	if task.Status == "waiting_input" || task.Status == "confirming_intent" {
		return s.continuePendingTask(task, snapshot, explicitIntent)
	}

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", buildTaskContinuationBubbleText(snapshot, decision), time.Now().Format(dateTimeLayout))
	updatedTask, changed := s.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
		Snapshot:        snapshot,
		BubbleMessage:   bubble,
		SteeringMessage: buildTaskContinuationInstruction(snapshot, explicitIntent),
	})
	if !changed {
		return nil, ErrTaskNotFound
	}
	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}, nil
}

func (s *Service) continuePendingTask(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any) (map[string]any, error) {
	mergedSnapshot := mergeContinuationSnapshots(snapshotFromTask(task), snapshot)
	if s.intent.AnalyzeSnapshot(mergedSnapshot) == "waiting_input" {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已把补充内容挂回当前任务，请继续补充剩余信息。", time.Now().Format(dateTimeLayout))
		updatedTask, changed := s.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
			Snapshot:      snapshot,
			Status:        "waiting_input",
			CurrentStep:   firstNonEmptyString(task.CurrentStep, "collect_input"),
			BubbleMessage: bubble,
		})
		if !changed {
			return nil, ErrTaskNotFound
		}
		return map[string]any{
			"task":            taskMap(updatedTask),
			"bubble_message":  bubble,
			"delivery_result": nil,
		}, nil
	}

	suggestion := s.intent.Suggest(mergedSnapshot, explicitIntent, false)
	suggestion = s.normalizeSuggestedIntentForAvailability(mergedSnapshot, suggestion, false)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForInput(suggestion), time.Now().Format(dateTimeLayout))
	updatedTask, changed := s.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
		Snapshot:      snapshot,
		Title:         suggestion.TaskTitle,
		Intent:        suggestion.Intent,
		Status:        taskStatusForSuggestion(suggestion.RequiresConfirm),
		CurrentStep:   currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent),
		BubbleMessage: bubble,
	})
	if !changed {
		return nil, ErrTaskNotFound
	}
	if suggestion.RequiresConfirm {
		return map[string]any{
			"task":            taskMap(updatedTask),
			"bubble_message":  bubble,
			"delivery_result": nil,
		}, nil
	}

	governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(updatedTask, suggestion.Intent)
	if governanceErr != nil {
		return nil, governanceErr
	}
	if handled {
		return governedResponse, nil
	}
	executedTask, resultBubble, deliveryResult, _, execErr := s.executeTask(governedTask, mergedSnapshot, suggestion.Intent)
	if execErr != nil {
		return nil, execErr
	}
	return map[string]any{
		"task":            taskMap(executedTask),
		"bubble_message":  resultBubble,
		"delivery_result": deliveryResult,
	}, nil
}

func (s *Service) loadTaskForContinuation(taskID string) (runengine.TaskRecord, bool) {
	if task, ok := s.runEngine.GetTask(taskID); ok {
		return task, true
	}
	task, ok := s.taskDetailFromStorage(taskID)
	if !ok {
		return runengine.TaskRecord{}, false
	}
	return s.runEngine.HydrateTaskFromStorage(task), true
}

func (s *Service) resolveImplicitSessionID(unfinishedCandidates []runengine.TaskRecord) string {
	if len(unfinishedCandidates) > 0 {
		return ""
	}
	if s != nil && s.storage != nil && s.storage.SessionStore() != nil {
		sessions, _, err := s.storage.SessionStore().ListSessions(context.Background(), 1, 0)
		if err == nil && len(sessions) > 0 && strings.TrimSpace(sessions[0].Status) == "idle" {
			if updatedAt, ok := parseContinuationTime(sessions[0].UpdatedAt); ok && time.Since(updatedAt) <= implicitSessionReuseWindow {
				return sessions[0].SessionID
			}
		}
	}
	if s != nil && s.runEngine != nil {
		finishedTasks, _ := s.runEngine.ListTasks("finished", "updated_at", "desc", 20, 0)
		for _, task := range finishedTasks {
			if strings.TrimSpace(task.SessionID) == "" {
				continue
			}
			if time.Since(task.UpdatedAt) <= implicitSessionReuseWindow {
				return task.SessionID
			}
		}
	}
	return ""
}

func parseContinuationTime(raw string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, dateTimeLayout} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(raw)); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func withResolvedSessionID(params map[string]any, sessionID string) map[string]any {
	if strings.TrimSpace(sessionID) == "" {
		return params
	}
	cloned := make(map[string]any, len(params)+1)
	for key, value := range params {
		cloned[key] = value
	}
	cloned["session_id"] = strings.TrimSpace(sessionID)
	return cloned
}

func buildTaskContinuationBubbleText(snapshot contextsvc.TaskContextSnapshot, decision taskContinuationDecision) string {
	subject := continuationSubject(snapshot)
	if strings.TrimSpace(subject) == "" {
		subject = "已把补充内容挂回当前任务。"
	}
	if strings.TrimSpace(decision.Reason) == "" {
		return subject
	}
	return subject + " " + truncateText(decision.Reason, 80)
}

func continuationSubject(snapshot contextsvc.TaskContextSnapshot) string {
	if len(snapshot.Files) > 0 {
		return fmt.Sprintf("已把 %d 个补充文件挂回当前任务。", len(snapshot.Files))
	}
	if strings.TrimSpace(snapshot.SelectionText) != "" {
		return "已把补充选中文本挂回当前任务。"
	}
	if strings.TrimSpace(snapshot.ErrorText) != "" {
		return "已把补充报错信息挂回当前任务。"
	}
	return "已把补充说明挂回当前任务。"
}

func buildTaskContinuationInstruction(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any) string {
	parts := make([]string, 0, 5)
	if text := strings.TrimSpace(snapshot.Text); text != "" {
		parts = append(parts, "Additional user text:\n"+text)
	}
	if selectionText := strings.TrimSpace(snapshot.SelectionText); selectionText != "" && selectionText != strings.TrimSpace(snapshot.Text) {
		parts = append(parts, "Selected text to include:\n"+selectionText)
	}
	if errorText := strings.TrimSpace(snapshot.ErrorText); errorText != "" {
		parts = append(parts, "Error details to include:\n"+errorText)
	}
	if len(snapshot.Files) > 0 {
		parts = append(parts, "Attached files:\n- "+strings.Join(snapshot.Files, "\n- "))
	}
	if len(explicitIntent) > 0 {
		if payload, err := json.Marshal(explicitIntent); err == nil {
			parts = append(parts, "Explicit intent override:\n"+string(payload))
		}
	}
	return strings.Join(parts, "\n\n")
}

func mergeContinuationSnapshots(base, update contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	merged := base
	merged.Source = pickContinuationValue(base.Source, update.Source)
	merged.Trigger = pickContinuationValue(base.Trigger, update.Trigger)
	merged.InputType = pickContinuationValue(base.InputType, update.InputType)
	merged.InputMode = pickContinuationValue(base.InputMode, update.InputMode)
	merged.Text = mergeContinuationText(base.Text, update.Text)
	merged.SelectionText = mergeContinuationText(base.SelectionText, update.SelectionText)
	merged.ErrorText = mergeContinuationText(base.ErrorText, update.ErrorText)
	merged.Files = dedupeContinuationFiles(base.Files, update.Files)
	merged.PageTitle = pickContinuationValue(base.PageTitle, update.PageTitle)
	merged.PageURL = pickContinuationValue(base.PageURL, update.PageURL)
	merged.AppName = pickContinuationValue(base.AppName, update.AppName)
	merged.WindowTitle = pickContinuationValue(base.WindowTitle, update.WindowTitle)
	merged.VisibleText = mergeContinuationText(base.VisibleText, update.VisibleText)
	merged.ScreenSummary = mergeContinuationText(base.ScreenSummary, update.ScreenSummary)
	merged.ClipboardText = mergeContinuationText(base.ClipboardText, update.ClipboardText)
	merged.HoverTarget = pickContinuationValue(base.HoverTarget, update.HoverTarget)
	merged.LastAction = pickContinuationValue(base.LastAction, update.LastAction)
	if update.DwellMillis > 0 {
		merged.DwellMillis = update.DwellMillis
	}
	if update.CopyCount > 0 {
		merged.CopyCount = update.CopyCount
	}
	if update.WindowSwitches > 0 {
		merged.WindowSwitches = update.WindowSwitches
	}
	if update.PageSwitches > 0 {
		merged.PageSwitches = update.PageSwitches
	}
	return merged
}

func pickContinuationValue(base, update string) string {
	if strings.TrimSpace(update) != "" {
		return strings.TrimSpace(update)
	}
	return strings.TrimSpace(base)
}

func mergeContinuationText(base, update string) string {
	base = strings.TrimSpace(base)
	update = strings.TrimSpace(update)
	switch {
	case update == "":
		return base
	case base == "":
		return update
	case base == update:
		return base
	default:
		return base + "\n\n" + update
	}
}

func dedupeContinuationFiles(base, update []string) []string {
	if len(base) == 0 && len(update) == 0 {
		return nil
	}
	result := make([]string, 0, len(base)+len(update))
	seen := make(map[string]struct{}, len(base)+len(update))
	for _, value := range append(append([]string{}, base...), update...) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func continuationContainsAny(text string, markers ...string) bool {
	for _, marker := range markers {
		if marker != "" && strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
