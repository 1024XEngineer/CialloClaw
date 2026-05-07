package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	intentsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

const implicitSessionReuseWindow = 15 * time.Minute

var taskContinuationModelTimeout = 3 * time.Second

type taskContinuationDecision struct {
	Decision string `json:"decision"`
	TaskID   string `json:"task_id"`
	Reason   string `json:"reason"`
}

type taskContinuationContext struct {
	SessionID   string
	Candidates  []runengine.TaskRecord
	SessionMode string
}

type taskContinuationOptions struct {
	// ConfirmRequired is the effective pre-execution gate after default task-start
	// policy has been applied.
	ConfirmRequired bool
	// ForceConfirmRequired records the caller's explicit option; inferred
	// confirmation is not enough to prove implicit plain-text ownership or block
	// already-confirmed pending evidence from resuming.
	ForceConfirmRequired bool
}

func (s *Service) maybeContinueExistingTask(params map[string]any, snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, options taskContinuationOptions) (map[string]any, bool, string, error) {
	explicitSessionID := strings.TrimSpace(stringValue(params, "session_id", ""))
	continuationContext := s.resolveTaskContinuationContext(explicitSessionID)
	decision := s.classifyTaskContinuation(snapshot, explicitIntent, continuationContext, options)
	if decision.Decision == "continue" && strings.TrimSpace(decision.TaskID) != "" {
		task, ok := s.loadTaskForContinuation(decision.TaskID)
		if !ok {
			return nil, false, explicitSessionID, nil
		}
		response, err := s.continueTask(task, snapshot, explicitIntent, decision, options)
		if err != nil {
			return nil, false, explicitSessionID, err
		}
		return response, true, task.SessionID, nil
	}

	// An implicit active session only scopes the continuation classifier. If the
	// decision is "new_task", the backend should open a fresh hidden session so
	// unrelated work does not get serialized behind the old task queue.
	if strings.TrimSpace(continuationContext.SessionID) != "" && (strings.TrimSpace(explicitSessionID) != "" || continuationContext.SessionMode == "implicit_idle") {
		return nil, false, continuationContext.SessionID, nil
	}
	return nil, false, "", nil
}

func (s *Service) continuationCandidates(sessionID string) []runengine.TaskRecord {
	queryViews := newTaskQueryViews(s)
	tasks := queryViews.tasks("unfinished", "updated_at", "desc")
	result := make([]runengine.TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(sessionID) == "" || task.SessionID != strings.TrimSpace(sessionID) {
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

func (s *Service) resolveTaskContinuationContext(explicitSessionID string) taskContinuationContext {
	if strings.TrimSpace(explicitSessionID) != "" {
		candidates := s.continuationCandidates(explicitSessionID)
		if len(candidates) > 0 {
			return taskContinuationContext{
				SessionID:   explicitSessionID,
				Candidates:  candidates,
				SessionMode: "explicit_active",
			}
		}
		if s.sessionIsFresh(explicitSessionID) {
			return taskContinuationContext{
				SessionID:   explicitSessionID,
				Candidates:  nil,
				SessionMode: "explicit_idle",
			}
		}
		return taskContinuationContext{}
	}

	queryViews := newTaskQueryViews(s)
	tasks := queryViews.tasks("unfinished", "updated_at", "desc")
	sessionCandidates := map[string][]runengine.TaskRecord{}
	for _, task := range tasks {
		if !canContinueTask(task) || strings.TrimSpace(task.SessionID) == "" {
			continue
		}
		sessionCandidates[task.SessionID] = append(sessionCandidates[task.SessionID], task)
	}
	if len(sessionCandidates) == 1 {
		for sessionID, candidates := range sessionCandidates {
			if s.sessionIsFresh(sessionID) {
				return taskContinuationContext{
					SessionID:   sessionID,
					Candidates:  candidates,
					SessionMode: "implicit_active",
				}
			}
		}
	}

	if sessionID := s.resolveImplicitSessionID(nil); strings.TrimSpace(sessionID) != "" {
		return taskContinuationContext{
			SessionID:   sessionID,
			Candidates:  nil,
			SessionMode: "implicit_idle",
		}
	}

	return taskContinuationContext{}
}

// canContinueTask keeps continuation scope limited to unfinished tasks that can
// still absorb follow-up input without invalidating an approval boundary,
// stashing guidance behind a paused resume-only transition, or trapping the
// user inside a blocked state.
func canContinueTask(task runengine.TaskRecord) bool {
	switch task.Status {
	case "confirming_intent", "waiting_input":
		return true
	case "processing":
		return taskCanConsumeActiveSteering(task)
	default:
		return false
	}
}

func taskCanConsumeActiveSteering(task runengine.TaskRecord) bool {
	// CurrentStep carries the execution mode that was actually started. The
	// intent can remain agent_loop even when runtime capabilities force prompt
	// fallback, and prompt fallback cannot poll active steering.
	return strings.TrimSpace(stringValue(task.Intent, "name", "")) == "agent_loop" &&
		strings.TrimSpace(task.CurrentStep) == "agent_loop"
}

func (s *Service) classifyTaskContinuation(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, continuationContext taskContinuationContext, options taskContinuationOptions) taskContinuationDecision {
	if len(continuationContext.Candidates) == 0 {
		return taskContinuationDecision{Decision: "new_task", Reason: "no unfinished candidate task"}
	}
	if decision, ok := deterministicTaskContinuationDecision(snapshot, explicitIntent, continuationContext, options); ok {
		return decision
	}
	if decision, ok := uniqueTaskSpecificContinuationDecision(snapshot, explicitIntent, continuationContext, options); ok {
		return decision
	}
	if options.ConfirmRequired {
		return taskContinuationDecision{Decision: "new_task", Reason: "confirmation gate requires a new task without unambiguous pending-task evidence"}
	}
	if decision, ok := s.modelTaskContinuationDecision(snapshot, explicitIntent, continuationContext, options); ok {
		return decision
	}
	return heuristicTaskContinuationDecision(snapshot, explicitIntent, continuationContext)
}

func (s *Service) modelTaskContinuationDecision(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, continuationContext taskContinuationContext, options taskContinuationOptions) (taskContinuationDecision, bool) {
	modelService := s.currentModel()
	if s == nil || modelService == nil {
		return taskContinuationDecision{}, false
	}
	// Continuation classification is a best-effort refinement on top of the local
	// deterministic heuristics. It must stay bounded so shell-ball submits do not
	// wait indefinitely for a remote model decision.
	ctx, cancel := context.WithTimeout(context.Background(), taskContinuationModelTimeout)
	defer cancel()
	response, err := modelService.GenerateText(ctx, model.GenerateTextRequest{
		TaskID: "task_continuation_classifier",
		RunID:  "run_continuation_classifier",
		Input:  buildTaskContinuationPrompt(snapshot, explicitIntent, continuationContext, options),
	})
	if err != nil {
		return taskContinuationDecision{}, false
	}
	decision, ok := parseTaskContinuationDecision(response.OutputText, continuationContext.Candidates)
	return decision, ok
}

// buildTaskContinuationPrompt intentionally sends only coarse task/session
// signals to the model so remote classification does not leak raw text, file
// names, or other cross-task payload details.
func buildTaskContinuationPrompt(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, continuationContext taskContinuationContext, options taskContinuationOptions) string {
	lines := []string{
		"You decide whether one new desktop input should continue an existing task or start a new task.",
		"Return JSON only.",
		`Schema: {"decision":"continue"|"new_task","task_id":"task_xxx or empty","reason":"short reason"}`,
		"Choose continue only when the new input is clearly refining, correcting, narrowing, or attaching evidence for the same ongoing task.",
		"Choose new_task when the input starts another goal, another deliverable, or another analysis target.",
		"Only decide among the candidate tasks from the current hidden desktop session. Do not infer anything outside the provided candidates.",
		"",
		"New input signals:",
		taskContinuationInputSummary(snapshot, explicitIntent, options),
		"",
		fmt.Sprintf("Candidate unfinished tasks in session (%s):", continuationContext.SessionMode),
	}
	for _, candidate := range continuationContext.Candidates {
		lines = append(lines, taskContinuationCandidateSummary(candidate))
	}
	return strings.Join(lines, "\n")
}

func taskContinuationInputSummary(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, options taskContinuationOptions) string {
	suggestion := intentsvc.NewService().Suggest(snapshot, explicitIntent, options.ConfirmRequired)
	resolvedIntentName := stringValue(suggestion.Intent, "name", "")
	resolvedDeliveryType := deliveryTypeFromIntent(suggestion.Intent)
	parts := []string{
		fmt.Sprintf("trigger=%s", snapshot.Trigger),
		fmt.Sprintf("input_type=%s", snapshot.InputType),
		fmt.Sprintf("input_shape=%s", taskContinuationInputShape(snapshot)),
		fmt.Sprintf("resolved_intent_name=%s", firstNonEmptyString(resolvedIntentName, "none")),
		fmt.Sprintf("resolved_delivery_type=%s", resolvedDeliveryType),
		fmt.Sprintf("has_text=%t", strings.TrimSpace(snapshot.Text) != ""),
		fmt.Sprintf("has_selection=%t", strings.TrimSpace(snapshot.SelectionText) != ""),
		fmt.Sprintf("has_error=%t", strings.TrimSpace(snapshot.ErrorText) != ""),
		fmt.Sprintf("file_count=%d", len(snapshot.Files)),
		fmt.Sprintf("has_page_url=%t", strings.TrimSpace(snapshot.PageURL) != ""),
		fmt.Sprintf("has_window_title=%t", strings.TrimSpace(snapshot.WindowTitle) != ""),
		fmt.Sprintf("has_app_name=%t", strings.TrimSpace(snapshot.AppName) != ""),
		fmt.Sprintf("has_hover_target=%t", strings.TrimSpace(snapshot.HoverTarget) != ""),
		fmt.Sprintf("has_screen_context=%t", strings.TrimSpace(snapshot.ScreenSummary) != "" || strings.TrimSpace(snapshot.VisibleText) != ""),
	}
	parts = append(parts,
		fmt.Sprintf("explicit_intent_present=%t", strings.TrimSpace(stringValue(explicitIntent, "name", "")) != ""),
		fmt.Sprintf("requires_confirmation=%t", suggestion.RequiresConfirm),
	)
	return strings.Join(parts, " | ")
}

func taskContinuationCandidateSummary(task runengine.TaskRecord) string {
	intentName := strings.TrimSpace(stringValue(task.Intent, "name", ""))
	parts := []string{
		fmt.Sprintf("- task_id=%s", task.TaskID),
		fmt.Sprintf("status=%s", task.Status),
		fmt.Sprintf("current_step=%s", task.CurrentStep),
		fmt.Sprintf("source_type=%s", task.SourceType),
		fmt.Sprintf("age_seconds=%d", int(time.Since(task.UpdatedAt).Seconds())),
		fmt.Sprintf("intent_name=%s", firstNonEmptyString(intentName, "none")),
		fmt.Sprintf("delivery_type=%s", resolveTaskDeliveryType(task, task.Intent)),
		fmt.Sprintf("awaits_follow_up=%t", task.Status == "waiting_input" || task.Status == "confirming_intent"),
		fmt.Sprintf("has_selection=%t", strings.TrimSpace(task.Snapshot.SelectionText) != ""),
		fmt.Sprintf("has_error=%t", strings.TrimSpace(task.Snapshot.ErrorText) != ""),
		fmt.Sprintf("has_files=%t", len(task.Snapshot.Files) > 0),
		fmt.Sprintf("has_page_url=%t", strings.TrimSpace(task.Snapshot.PageURL) != ""),
		fmt.Sprintf("has_window_title=%t", strings.TrimSpace(task.Snapshot.WindowTitle) != ""),
		fmt.Sprintf("has_app_name=%t", strings.TrimSpace(task.Snapshot.AppName) != ""),
		fmt.Sprintf("has_hover_target=%t", strings.TrimSpace(task.Snapshot.HoverTarget) != ""),
		fmt.Sprintf("has_screen_context=%t", strings.TrimSpace(task.Snapshot.ScreenSummary) != "" || strings.TrimSpace(task.Snapshot.VisibleText) != ""),
	}
	parts = append(parts, fmt.Sprintf("has_intent=%t", intentName != ""))
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

// deterministicTaskContinuationDecision handles the safe local decisions that
// do not need model inference. The goal is to prefer formal waiting states and
// strong context anchors over brittle free-text cue matching while preventing
// agent.task.start explicit intents from being silently grafted onto another
// task unless there is concrete continuation evidence.
func deterministicTaskContinuationDecision(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, continuationContext taskContinuationContext, options taskContinuationOptions) (taskContinuationDecision, bool) {
	if len(continuationContext.Candidates) != 1 {
		return taskContinuationDecision{}, false
	}
	candidate := continuationContext.Candidates[0]
	evidence := buildTaskContinuationEvidence(snapshot, snapshotFromTask(candidate))
	explicitIntentName := strings.TrimSpace(stringValue(explicitIntent, "name", ""))
	if evidence.HasConflictingAnchor {
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "input context conflicts with the unfinished task anchors",
		}, true
	}
	if explicitIntentRequiresFreshTask(explicitIntentName, candidate, evidence, continuationContext) {
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "explicit start intent lacks continuation anchors for the unfinished task",
		}, true
	}
	if candidate.Status == "processing" && evidence.StructuredSupplement {
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "structured input cannot attach to an active execution task",
		}, true
	}
	if options.ConfirmRequired {
		if evidence.StructuredSupplement {
			return confirmationRequiredStructuredContinuationDecision(candidate, evidence)
		}
		// Confirmation gates execution, not ownership of a plain text follow-up
		// for a task that is already waiting on the user.
		if decision, ok := pendingTaskContinuationDecision(candidate, evidence, continuationContext, explicitIntentName, options); ok {
			return decision, true
		}
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "confirmation-required input lacks pending-task continuation evidence",
		}, true
	}
	if decision, ok := pendingTaskContinuationDecision(candidate, evidence, continuationContext, explicitIntentName, options); ok {
		return decision, true
	}

	return taskContinuationDecision{}, false
}

// pendingTaskContinuationDecision keeps waiting tasks open for plain textual
// follow-up in the active session while requiring structured objects to prove
// task-specific lineage or context anchors before they can attach to the
// pending task.
func pendingTaskContinuationDecision(candidate runengine.TaskRecord, evidence taskContinuationEvidence, continuationContext taskContinuationContext, explicitIntentName string, options taskContinuationOptions) (taskContinuationDecision, bool) {
	if candidate.Status != "waiting_input" && candidate.Status != "confirming_intent" {
		return taskContinuationDecision{}, false
	}
	if evidence.StructuredSupplement {
		if !hasTaskSpecificContinuationEvidence(evidence) {
			return taskContinuationDecision{
				Decision: "new_task",
				Reason:   "structured input lacks task-specific continuation evidence for the pending task",
			}, true
		}
		return taskContinuationDecision{
			Decision: "continue",
			TaskID:   candidate.TaskID,
			Reason:   "structured follow-up evidence belongs to the pending task",
		}, true
	}
	if evidence.HasStrongAnchor {
		return taskContinuationDecision{
			Decision: "continue",
			TaskID:   candidate.TaskID,
			Reason:   "unfinished task is explicitly waiting for follow-up input",
		}, true
	}
	if plainTextCanUseActivePendingSession(continuationContext, explicitIntentName, options) {
		return taskContinuationDecision{
			Decision: "continue",
			TaskID:   candidate.TaskID,
			Reason:   "active session task is already waiting for plain-text follow-up input",
		}, true
	}
	if !evidence.CurrentHasContextAnchor && !evidence.PreviousHasContextAnchor && explicitIntentName == "" && plainTextCanUseAnchorlessPendingSession(continuationContext, options) {
		return taskContinuationDecision{
			Decision: "continue",
			TaskID:   candidate.TaskID,
			Reason:   "unfinished task is explicitly waiting for follow-up input",
		}, true
	}
	if continuationContext.SessionMode == "implicit_active" && !options.ForceConfirmRequired && explicitIntentName == "" {
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "implicit pending plain-text input lacks explicit session, confirmation gate, or shared anchors",
		}, true
	}
	return taskContinuationDecision{}, false
}

func plainTextCanUseActivePendingSession(continuationContext taskContinuationContext, explicitIntentName string, options taskContinuationOptions) bool {
	if explicitIntentName != "" || len(continuationContext.Candidates) != 1 {
		return false
	}
	if continuationContext.SessionMode == "explicit_active" {
		return true
	}
	return continuationContext.SessionMode == "implicit_active" && options.ForceConfirmRequired
}

func plainTextCanUseAnchorlessPendingSession(continuationContext taskContinuationContext, options taskContinuationOptions) bool {
	return continuationContext.SessionMode == "explicit_active" || options.ForceConfirmRequired
}

// uniqueTaskSpecificContinuationDecision preserves structured follow-up routing
// when a multi-candidate session still has exactly one task-specific match.
// Inputs without that match are claimed as new tasks here so model fallback
// cannot guess an owner for file, selection, or error evidence.
// Confirmation only gates execution after that ownership decision is made.
func uniqueTaskSpecificContinuationDecision(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, continuationContext taskContinuationContext, options taskContinuationOptions) (taskContinuationDecision, bool) {
	if len(continuationContext.Candidates) < 2 || !isStructuredSupplementInput(snapshot) {
		return taskContinuationDecision{}, false
	}
	explicitIntentName := strings.TrimSpace(stringValue(explicitIntent, "name", ""))
	matches := make([]taskContinuationDecision, 0, 1)
	for _, candidate := range continuationContext.Candidates {
		evidence := buildTaskContinuationEvidence(snapshot, snapshotFromTask(candidate))
		if evidence.HasConflictingAnchor || !hasTaskSpecificContinuationEvidence(evidence) {
			continue
		}
		if explicitIntentRequiresFreshTask(explicitIntentName, candidate, evidence, continuationContext) {
			continue
		}
		if decision, ok := taskSpecificContinuationDecision(candidate, evidence, continuationContext, explicitIntentName, options); ok && decision.Decision == "continue" {
			matches = append(matches, decision)
		}
	}
	switch len(matches) {
	case 0:
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "structured input has no unique task-specific continuation match",
		}, true
	case 1:
		return matches[0], true
	default:
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "structured input matches multiple candidate tasks",
		}, true
	}
}

func taskSpecificContinuationDecision(candidate runengine.TaskRecord, evidence taskContinuationEvidence, continuationContext taskContinuationContext, explicitIntentName string, options taskContinuationOptions) (taskContinuationDecision, bool) {
	if options.ConfirmRequired {
		if evidence.StructuredSupplement {
			return confirmationRequiredStructuredContinuationDecision(candidate, evidence)
		}
		if decision, ok := pendingTaskContinuationDecision(candidate, evidence, continuationContext, explicitIntentName, options); ok {
			return decision, true
		}
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "confirmation-required input lacks pending-task continuation evidence",
		}, true
	}
	if decision, ok := pendingTaskContinuationDecision(candidate, evidence, continuationContext, explicitIntentName, options); ok {
		return decision, ok
	}
	return taskContinuationDecision{}, false
}

func confirmationRequiredStructuredContinuationDecision(candidate runengine.TaskRecord, evidence taskContinuationEvidence) (taskContinuationDecision, bool) {
	if candidate.Status != "waiting_input" && candidate.Status != "confirming_intent" {
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "confirmation-required input cannot attach to an active execution task",
		}, true
	}
	if !hasTaskSpecificContinuationEvidence(evidence) {
		return taskContinuationDecision{
			Decision: "new_task",
			Reason:   "confirmation-required structured input lacks task-specific continuation evidence",
		}, true
	}
	return taskContinuationDecision{
		Decision: "continue",
		TaskID:   candidate.TaskID,
		Reason:   "structured follow-up evidence belongs to the pending task",
	}, true
}

// hasTaskSpecificContinuationEvidence keeps structured inputs from merging into
// the lone pending task solely because they are structured. The input must
// still prove it belongs to that task through lineage or a compatible
// page/window/object anchor.
func hasTaskSpecificContinuationEvidence(evidence taskContinuationEvidence) bool {
	return evidence.HasLineageMatch || evidence.HasStrongAnchor
}

// explicitIntentRequiresFreshTask treats agent.task.start explicit intents as a
// fresh top-level request unless the backend can prove they belong to the same
// task through lineage, structured evidence, or explicit-session anchors.
func explicitIntentRequiresFreshTask(explicitIntentName string, candidate runengine.TaskRecord, evidence taskContinuationEvidence, continuationContext taskContinuationContext) bool {
	if explicitIntentName == "" {
		return false
	}
	// Controlled screen analysis must always establish its own task and approval
	// boundary even when the caller is still focused on the same page/window.
	if explicitIntentName == "screen_analyze" {
		return true
	}
	if evidence.HasLineageMatch || evidence.StructuredSupplement {
		return false
	}
	if continuationContext.SessionMode == "explicit_active" && evidence.HasStrongAnchor {
		return false
	}
	return candidate.Status == "waiting_input" || candidate.Status == "confirming_intent" || candidate.Status == "processing"
}

func heuristicTaskContinuationDecision(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, continuationContext taskContinuationContext) taskContinuationDecision {
	if len(continuationContext.Candidates) != 1 {
		return taskContinuationDecision{Decision: "new_task", Reason: "multiple unfinished candidates"}
	}
	if decision, ok := deterministicTaskContinuationDecision(snapshot, explicitIntent, continuationContext, taskContinuationOptions{}); ok {
		return decision
	}
	return taskContinuationDecision{
		Decision: "new_task",
		Reason:   "fallback heuristic opened a fresh task because structured continuation evidence was insufficient",
	}
}

type taskContinuationEvidence struct {
	HasStrongAnchor          bool
	HasLineageMatch          bool
	HasConflictingAnchor     bool
	StructuredSupplement     bool
	CurrentHasContextAnchor  bool
	PreviousHasContextAnchor bool
}

func buildTaskContinuationEvidence(current, previous contextsvc.TaskContextSnapshot) taskContinuationEvidence {
	structuredSupplement := isStructuredSupplementInput(current)
	currentAnchor := taskSpecificAnchorSnapshot(current)
	previousAnchor := taskSpecificAnchorSnapshot(previous)
	samePageURL := sameNonEmpty(currentAnchor.PageURL, previousAnchor.PageURL)
	sameHoverTarget := sameNonEmpty(currentAnchor.HoverTarget, previousAnchor.HoverTarget)
	sameSelectionText := sameNonEmpty(current.SelectionText, previous.SelectionText)
	sameErrorText := sameNonEmpty(current.ErrorText, previous.ErrorText)
	sharedFiles := sharedContinuationFiles(current.Files, previous.Files)
	sameWindowAnchor := sameNonEmpty(currentAnchor.WindowTitle, previousAnchor.WindowTitle) && sameNonEmpty(currentAnchor.AppName, previousAnchor.AppName)
	samePageAnchor := sameNonEmpty(currentAnchor.PageTitle, previousAnchor.PageTitle) && sameNonEmpty(currentAnchor.AppName, previousAnchor.AppName)

	return taskContinuationEvidence{
		HasStrongAnchor:          samePageURL || sameHoverTarget || sameWindowAnchor || samePageAnchor || sameSelectionText || sameErrorText || sharedFiles,
		HasLineageMatch:          sameSelectionText || sameErrorText || sharedFiles,
		HasConflictingAnchor:     hasConflictingContextAnchor(currentAnchor, previousAnchor),
		StructuredSupplement:     structuredSupplement,
		CurrentHasContextAnchor:  hasSnapshotContextAnchor(currentAnchor),
		PreviousHasContextAnchor: hasSnapshotContextAnchor(previousAnchor),
	}
}

func taskContinuationInputShape(snapshot contextsvc.TaskContextSnapshot) string {
	switch {
	case isStructuredSupplementInput(snapshot) && strings.TrimSpace(snapshot.Text) != "" && snapshot.InputType == "text":
		return "mixed"
	case len(snapshot.Files) > 0 && strings.TrimSpace(snapshot.SelectionText) == "" && strings.TrimSpace(snapshot.ErrorText) == "":
		return "attachment_only"
	case strings.TrimSpace(snapshot.ErrorText) != "" && len(snapshot.Files) == 0 && strings.TrimSpace(snapshot.SelectionText) == "":
		return "error_only"
	case strings.TrimSpace(snapshot.SelectionText) != "" && len(snapshot.Files) == 0 && strings.TrimSpace(snapshot.ErrorText) == "":
		return "selection_only"
	case strings.TrimSpace(snapshot.Text) != "":
		return "plain_text"
	default:
		return "empty"
	}
}

func isStructuredSupplementInput(snapshot contextsvc.TaskContextSnapshot) bool {
	return snapshot.InputType == "file" ||
		snapshot.InputType == "text_selection" ||
		snapshot.InputType == "error" ||
		len(snapshot.Files) > 0 ||
		strings.TrimSpace(snapshot.SelectionText) != "" ||
		strings.TrimSpace(snapshot.ErrorText) != ""
}

func hasSnapshotContextAnchor(snapshot contextsvc.TaskContextSnapshot) bool {
	return strings.TrimSpace(snapshot.PageURL) != "" ||
		strings.TrimSpace(snapshot.WindowTitle) != "" ||
		strings.TrimSpace(snapshot.AppName) != "" ||
		strings.TrimSpace(snapshot.PageTitle) != "" ||
		strings.TrimSpace(snapshot.HoverTarget) != ""
}

func hasConflictingContextAnchor(current, previous contextsvc.TaskContextSnapshot) bool {
	if nonEmptyAndDifferent(current.PageURL, previous.PageURL) {
		return true
	}
	if nonEmptyAndDifferent(current.AppName, previous.AppName) {
		return true
	}
	if strings.TrimSpace(current.PageURL) == "" && strings.TrimSpace(previous.PageURL) == "" &&
		sameNonEmpty(current.AppName, previous.AppName) &&
		nonEmptyAndDifferent(current.WindowTitle, previous.WindowTitle) {
		return true
	}
	return false
}

// taskSpecificAnchorSnapshot removes intake-only shell-ball context before
// comparing continuation anchors. The shell-ball page identifies where input
// entered the system, not which user task the input belongs to.
func taskSpecificAnchorSnapshot(snapshot contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	if isShellBallIntakeAnchor(snapshot) {
		return withoutShellBallIntakeAnchor(snapshot)
	}
	return snapshot
}

// isShellBallIntakeAnchor identifies the frontend's default shell-ball wrapper
// context. It is an intake surface marker, not evidence that the user's active
// work context moved away from the pending task.
func isShellBallIntakeAnchor(snapshot contextsvc.TaskContextSnapshot) bool {
	return strings.TrimSpace(snapshot.PageURL) == "local://shell-ball" &&
		strings.EqualFold(strings.TrimSpace(snapshot.AppName), "desktop")
}

func withoutShellBallIntakeAnchor(snapshot contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	snapshot.PageTitle = ""
	snapshot.PageURL = ""
	snapshot.AppName = ""
	snapshot.WindowTitle = ""
	return snapshot
}

func sharedContinuationFiles(current, previous []string) bool {
	if len(current) == 0 || len(previous) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(previous))
	for _, value := range previous {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}
	for _, value := range current {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			return true
		}
	}
	return false
}

func sameNonEmpty(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && left == right
}

func nonEmptyAndDifferent(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && left != right
}

func (s *Service) continueTask(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, decision taskContinuationDecision, options taskContinuationOptions) (map[string]any, error) {
	if task.Status == "waiting_input" || task.Status == "confirming_intent" {
		return s.continuePendingTask(task, snapshot, explicitIntent, options)
	}

	continuationSnapshot := sanitizeContinuationUpdateSnapshot(snapshotFromTask(task), snapshot)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", buildTaskContinuationBubbleText(continuationSnapshot, decision), time.Now().Format(dateTimeLayout))
	updatedTask, changed := s.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
		Snapshot:        continuationSnapshot,
		BubbleMessage:   bubble,
		SteeringMessage: buildTaskContinuationInstruction(continuationSnapshot, explicitIntent),
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

func (s *Service) continuePendingTask(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, options taskContinuationOptions) (map[string]any, error) {
	baseSnapshot := snapshotFromTask(task)
	continuationSnapshot := sanitizeContinuationUpdateSnapshot(baseSnapshot, snapshot)
	mergedSnapshot := mergeContinuationSnapshots(baseSnapshot, continuationSnapshot)
	if s.intent.AnalyzeSnapshot(mergedSnapshot) == "waiting_input" {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已把补充内容挂回当前任务，请继续补充剩余信息。", time.Now().Format(dateTimeLayout))
		updatedTask, changed := s.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
			Snapshot:      continuationSnapshot,
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

	confirmRequired := pendingContinuationRequiresConfirm(task, continuationSnapshot, options)
	suggestion := s.intent.Suggest(mergedSnapshot, pendingContinuationIntent(task, explicitIntent), confirmRequired)
	suggestion = s.normalizeSuggestedIntentForAvailability(mergedSnapshot, suggestion, confirmRequired)
	if confirmRequired {
		// A pending confirmation gate belongs to the task lifecycle. Follow-up
		// input may enrich the task snapshot, but it must not unlock intent
		// shortcuts that normally skip confirmation for fresh starts.
		suggestion.RequiresConfirm = true
	}
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForInput(suggestion), time.Now().Format(dateTimeLayout))
	updatedTask, changed := s.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
		Snapshot:      continuationSnapshot,
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

func pendingContinuationRequiresConfirm(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, options taskContinuationOptions) bool {
	if options.ForceConfirmRequired {
		return true
	}
	if task.Status == "confirming_intent" {
		return true
	}
	if !isStructuredSupplementInput(snapshot) {
		return false
	}
	// Structured evidence may resume a waiting task only after intent ownership
	// is already known; otherwise attaching evidence is still separate from
	// permission to execute a newly inferred task.
	return !taskHasConfirmedIntent(task)
}

func pendingContinuationIntent(task runengine.TaskRecord, explicitIntent map[string]any) map[string]any {
	if len(explicitIntent) > 0 {
		return explicitIntent
	}
	if task.Status == "confirming_intent" && len(task.Intent) > 0 {
		return cloneMap(task.Intent)
	}
	return nil
}

func taskHasConfirmedIntent(task runengine.TaskRecord) bool {
	return strings.TrimSpace(stringValue(task.Intent, "name", "")) != ""
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
	return subject
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

func (s *Service) sessionIsFresh(sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	if s != nil && s.storage != nil && s.storage.SessionStore() != nil {
		if session, err := s.storage.SessionStore().GetSession(context.Background(), sessionID); err == nil {
			if updatedAt, ok := parseContinuationTime(session.UpdatedAt); ok {
				return time.Since(updatedAt) <= implicitSessionReuseWindow
			}
		}
	}
	if s.sessionHasRecentRuntimeTask(sessionID, "unfinished") {
		return true
	}
	return s.sessionHasRecentRuntimeTask(sessionID, "finished")
}

func (s *Service) sessionHasRecentRuntimeTask(sessionID, group string) bool {
	if s == nil || s.runEngine == nil {
		return false
	}
	tasks, _ := s.runEngine.ListTasks(group, "updated_at", "desc", 50, 0)
	for _, task := range tasks {
		if task.SessionID == sessionID && time.Since(task.UpdatedAt) <= implicitSessionReuseWindow {
			return true
		}
	}
	return false
}

func sanitizeContinuationUpdateSnapshot(base, update contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	if hasSnapshotContextAnchor(base) && isStructuredSupplementInput(update) && isShellBallIntakeAnchor(update) {
		// Shell-ball default anchors describe how supplemental evidence entered
		// the system; they must not replace the real page/app anchors of the
		// pending task that asked for the evidence.
		return withoutShellBallIntakeAnchor(update)
	}
	return update
}

func mergeContinuationSnapshots(base, update contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	update = sanitizeContinuationUpdateSnapshot(base, update)
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
