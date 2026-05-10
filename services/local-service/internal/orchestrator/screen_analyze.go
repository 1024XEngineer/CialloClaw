package orchestrator

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func (s *Service) handleScreenAnalyzeStart(params map[string]any, snapshot taskcontext.TaskContextSnapshot, explicitIntent map[string]any) (map[string]any, bool, error) {
	if stringValue(explicitIntent, "name", "") != "screen_analyze" || s.executor == nil || !s.executor.ScreenCapabilitySnapshot().Available {
		return nil, false, nil
	}
	resolvedIntent := s.resolveScreenAnalyzeIntent(snapshot, explicitIntent)
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(params, "session_id", ""),
		RequestSource:     stringValue(params, "source", ""),
		RequestTrigger:    stringValue(params, "trigger", ""),
		Title:             firstNonEmptyString(stringValue(resolvedIntent, "title", ""), inferredScreenTaskTitle(snapshot)),
		SourceType:        "screen_capture",
		Status:            "waiting_auth",
		Intent:            cloneMap(resolvedIntent),
		PreferredDelivery: "bubble",
		FallbackDelivery:  "bubble",
		CurrentStep:       "waiting_authorization",
		RiskLevel:         "yellow",
		Timeline:          initialTimeline("waiting_auth", "waiting_authorization"),
		Snapshot:          snapshot,
	})
	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
		return nil, false, queueErr
	} else if queued {
		return map[string]any{
			"task":            taskMap(queuedTask),
			"bubble_message":  queueBubble,
			"delivery_result": nil,
		}, true, nil
	}
	approvalRequest, pendingExecution, bubble, err := s.buildScreenAnalysisApprovalState(task)
	if err != nil {
		return nil, false, err
	}
	updatedTask, ok := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !ok {
		return nil, false, ErrTaskNotFound
	}
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, mapValue(pendingExecution, "impact_scope")); err != nil {
		return nil, false, err
	}
	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}, true, nil
}

func (s *Service) handleScreenAnalyzeSuggestion(params map[string]any, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) (map[string]any, bool, error) {
	if stringValue(suggestion.Intent, "name", "") != "screen_analyze" || suggestion.RequiresConfirm {
		return nil, false, nil
	}
	return s.handleScreenAnalyzeStart(params, snapshot, suggestion.Intent)
}

func (s *Service) normalizeSuggestedIntentForAvailability(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, confirmRequired bool) intent.Suggestion {
	if stringValue(suggestion.Intent, "name", "") != "screen_analyze" {
		return suggestion
	}
	if s.executor != nil && s.executor.ScreenCapabilitySnapshot().Available {
		return suggestion
	}
	fallback := suggestion
	fallback.Intent = map[string]any{
		"name":      "agent_loop",
		"arguments": map[string]any{},
	}
	fallback.IntentConfirmed = true
	// Preserve the caller's confirmation gate when screen-specific handling is
	// unavailable so the downgrade does not auto-execute a generic task.
	fallback.RequiresConfirm = confirmRequired
	fallback.TaskSourceType = "hover_input"
	fallback.TaskTitle = inferredScreenFallbackSubject(snapshot)
	fallback.DirectDeliveryType = "bubble"
	fallback.ResultTitle = presentation.Text(presentation.MessageResultTitleGeneric, nil)
	fallback.ResultPreview = presentation.Text(presentation.MessagePreviewBubble, nil)
	fallback.ResultBubbleText = presentation.Text(presentation.MessageBubbleScreenDowngrade, nil)
	return fallback
}

func inferredScreenFallbackSubject(snapshot taskcontext.TaskContextSnapshot) string {
	return truncateText(firstNonEmptyString(strings.TrimSpace(snapshot.Text), screenSubjectFromSnapshot(snapshot)), subjectPreviewMaxLength)
}

// buildScreenAnalysisApprovalState reconstructs the controlled approval plan
// from the task intent so queued resumes can re-enter the same authorization
// path instead of falling through to the generic executor.
func (s *Service) buildScreenAnalysisApprovalState(task runengine.TaskRecord) (map[string]any, map[string]any, map[string]any, error) {
	arguments := mapValue(task.Intent, "arguments")
	sourcePath := stringValue(arguments, "path", "")
	captureMode := screenCaptureModeForIntent(arguments)
	source := firstNonEmptyString(stringValue(arguments, "source", ""), "screen_capture")
	targetObject := screenTargetObject(arguments)
	approvalRequest := buildApprovalRequest(task.TaskID, task.Intent, execution.GovernanceAssessment{
		OperationName: "screen_capture",
		TargetObject:  targetObject,
		RiskLevel:     "yellow",
		Reason:        "screen_capture_requires_authorization",
	})
	pendingExecution := map[string]any{
		"kind":           "screen_analysis",
		"operation_name": "screen_capture",
		"source_path":    sourcePath,
		"capture_mode":   string(captureMode),
		"source":         source,
		"target_object":  targetObject,
		"language":       firstNonEmptyString(stringValue(arguments, "language", ""), "eng"),
		"evidence_role":  firstNonEmptyString(stringValue(arguments, "evidence_role", ""), "error_evidence"),
		"delivery_type":  "bubble",
		"result_title":   presentation.Text(presentation.MessageResultTitleScreen, nil),
		"preview_text":   screenAnalysisPreviewText(captureMode),
		"impact_scope": map[string]any{
			"files":                    impactFilesForScreenTarget(sourcePath),
			"webpages":                 []string{},
			"apps":                     []string{},
			"out_of_workspace":         false,
			"overwrite_or_delete_risk": false,
		},
	}
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleScreenApproval, nil), task.UpdatedAt.Format(dateTimeLayout))
	return approvalRequest, pendingExecution, bubble, nil
}

func (s *Service) resolveScreenAnalyzeIntent(snapshot taskcontext.TaskContextSnapshot, current map[string]any) map[string]any {
	updatedIntent := cloneMap(current)
	arguments := cloneMap(mapValue(updatedIntent, "arguments"))
	if arguments == nil {
		arguments = map[string]any{}
	}
	if strings.TrimSpace(stringValue(arguments, "language", "")) == "" {
		arguments["language"] = "eng"
	}
	if strings.TrimSpace(stringValue(arguments, "capture_mode", "")) == "" {
		arguments["capture_mode"] = string(screenCaptureModeForIntent(arguments))
	}
	if strings.TrimSpace(stringValue(arguments, "evidence_role", "")) == "" {
		arguments["evidence_role"] = inferredScreenEvidenceRole(snapshot, arguments)
	}
	if strings.TrimSpace(stringValue(arguments, "page_title", "")) == "" && strings.TrimSpace(snapshot.PageTitle) != "" {
		arguments["page_title"] = snapshot.PageTitle
	}
	if strings.TrimSpace(stringValue(arguments, "window_title", "")) == "" && strings.TrimSpace(snapshot.WindowTitle) != "" {
		arguments["window_title"] = snapshot.WindowTitle
	}
	if strings.TrimSpace(stringValue(arguments, "visible_text", "")) == "" && strings.TrimSpace(snapshot.VisibleText) != "" {
		arguments["visible_text"] = snapshot.VisibleText
	}
	if strings.TrimSpace(stringValue(arguments, "screen_summary", "")) == "" && strings.TrimSpace(snapshot.ScreenSummary) != "" {
		arguments["screen_summary"] = snapshot.ScreenSummary
	}
	updatedIntent["arguments"] = arguments
	if strings.TrimSpace(stringValue(updatedIntent, "title", "")) == "" {
		updatedIntent["title"] = inferredScreenTaskTitle(snapshot)
	}
	return updatedIntent
}

func screenCaptureModeForIntent(arguments map[string]any) tools.ScreenCaptureMode {
	switch strings.ToLower(strings.TrimSpace(stringValue(arguments, "capture_mode", ""))) {
	case string(tools.ScreenCaptureModeClip):
		return tools.ScreenCaptureModeClip
	case string(tools.ScreenCaptureModeKeyframe):
		return tools.ScreenCaptureModeKeyframe
	case string(tools.ScreenCaptureModeScreenshot):
		return tools.ScreenCaptureModeScreenshot
	}
	if isClipScreenSourcePath(stringValue(arguments, "path", "")) {
		return tools.ScreenCaptureModeClip
	}
	return tools.ScreenCaptureModeScreenshot
}

func isClipScreenSourcePath(pathValue string) bool {
	trimmedPath := strings.ToLower(strings.TrimSpace(pathValue))
	switch path.Ext(trimmedPath) {
	case ".mp4", ".webm", ".mov", ".mkv", ".avi":
		return true
	default:
		return false
	}
}

func inferredScreenTaskTitle(snapshot taskcontext.TaskContextSnapshot) string {
	target := screenSubjectFromSnapshot(snapshot)
	if screenSnapshotHasErrorIntent(snapshot) {
		return fmt.Sprintf("%s报错", truncateText(target, subjectPreviewMaxLength))
	}
	return truncateText(target, subjectPreviewMaxLength)
}

func screenSubjectFromSnapshot(snapshot taskcontext.TaskContextSnapshot) string {
	return firstNonEmptyString(
		snapshot.PageTitle,
		firstNonEmptyString(
			snapshot.WindowTitle,
			firstNonEmptyString(snapshot.ScreenSummary, firstNonEmptyString(snapshot.VisibleText, presentation.Text(presentation.MessageTaskSubjectCurrentScreen, nil))),
		),
	)
}

func screenTargetObject(arguments map[string]any) string {
	if sourcePath := stringValue(arguments, "path", ""); strings.TrimSpace(sourcePath) != "" {
		return sourcePath
	}
	for _, value := range []string{
		stringValue(arguments, "page_title", ""),
		stringValue(arguments, "window_title", ""),
		stringValue(arguments, "screen_summary", ""),
		stringValue(arguments, "visible_text", ""),
	} {
		if strings.TrimSpace(value) != "" {
			return truncateText(value, 64)
		}
	}
	return "current_screen"
}

func screenCaptureModeFromArguments(arguments map[string]any) tools.ScreenCaptureMode {
	mode := tools.ScreenCaptureMode(strings.TrimSpace(stringValue(arguments, "capture_mode", string(tools.ScreenCaptureModeScreenshot))))
	switch mode {
	case tools.ScreenCaptureModeScreenshot, tools.ScreenCaptureModeKeyframe, tools.ScreenCaptureModeClip:
		return mode
	default:
		return tools.ScreenCaptureModeScreenshot
	}
}

func screenAnalysisPreviewText(captureMode tools.ScreenCaptureMode) string {
	return presentation.ScreenPreviewText(string(captureMode))
}

func screenSnapshotHasErrorIntent(snapshot taskcontext.TaskContextSnapshot) bool {
	combined := strings.ToLower(strings.Join([]string{snapshot.Text, snapshot.ErrorText}, " "))
	return strings.TrimSpace(snapshot.ErrorText) != "" ||
		strings.Contains(combined, "错误") ||
		strings.Contains(combined, "报错") ||
		strings.Contains(combined, "error")
}

func impactFilesForScreenTarget(sourcePath string) []string {
	if strings.TrimSpace(sourcePath) == "" {
		return []string{}
	}
	return []string{sourcePath}
}

func inferredScreenEvidenceRole(snapshot taskcontext.TaskContextSnapshot, arguments map[string]any) string {
	if role := stringValue(arguments, "evidence_role", ""); strings.TrimSpace(role) != "" {
		return role
	}
	combined := strings.ToLower(strings.Join([]string{snapshot.Text, snapshot.ErrorText, snapshot.VisibleText, snapshot.ScreenSummary}, " "))
	if strings.Contains(combined, "error") || strings.Contains(combined, "warning") || strings.Contains(combined, "报错") || strings.Contains(combined, "错误") || strings.Contains(combined, "异常") {
		return "error_evidence"
	}
	return "page_context"
}

func (s *Service) executeScreenAnalysisAfterApproval(task runengine.TaskRecord, pendingExecution map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, error) {
	if s.executor == nil || s.executor.ScreenClient() == nil {
		failedTask, failureBubble := s.failExecutionTask(task, map[string]any{"name": "screen_analyze"}, execution.Result{}, tools.ErrScreenCaptureNotSupported)
		return failedTask, failureBubble, nil, nil
	}
	screenClient := s.executor.ScreenClient()
	cleanupExpiredScreenTemps(screenClient, "expired_session_scan", time.Now().UTC())
	captureMode := screenCaptureModeFromArguments(pendingExecution)
	source := firstNonEmptyString(stringValue(pendingExecution, "source", ""), "screen_capture")
	screenSession, err := screenClient.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   task.SessionID,
		TaskID:      task.TaskID,
		RunID:       task.RunID,
		Source:      source,
		CaptureMode: captureMode,
	})
	if err != nil {
		failedTask, failureBubble := s.failExecutionTask(task, map[string]any{"name": "screen_analyze"}, execution.Result{}, err)
		return failedTask, failureBubble, nil, nil
	}
	candidate, err := captureScreenCandidateAfterApproval(screenClient, screenSession.ScreenSessionID, task, pendingExecution, captureMode)
	if err != nil {
		expireAndCleanupScreenSession(screenClient, screenSession.ScreenSessionID, "capture_failed")
		failedTask, failureBubble := s.failExecutionTask(task, map[string]any{"name": "screen_analyze"}, execution.Result{}, err)
		return failedTask, failureBubble, nil, nil
	}
	execIntent := map[string]any{
		"name": "screen_analyze_candidate",
		"arguments": map[string]any{
			"task_id":           task.TaskID,
			"run_id":            task.RunID,
			"screen_session_id": screenSession.ScreenSessionID,
			"frame_id":          candidate.FrameID,
			"path":              candidate.Path,
			"capture_mode":      string(candidate.CaptureMode),
			"source":            candidate.Source,
			"captured_at":       candidate.CapturedAt.UTC().Format(time.RFC3339),
			"retention_policy":  string(candidate.RetentionPolicy),
			"language":          stringValue(pendingExecution, "language", "eng"),
			"evidence_role":     stringValue(pendingExecution, "evidence_role", "error_evidence"),
			"target_object":     stringValue(pendingExecution, "target_object", "current_screen"),
		},
	}
	updatedTask, bubble, deliveryResult, _, err := s.executeTask(task, snapshotFromTask(task), execIntent)
	if err != nil {
		expireAndCleanupScreenSession(screenClient, screenSession.ScreenSessionID, "analysis_failed")
		return runengine.TaskRecord{}, nil, nil, err
	}
	// Successful analyses stop the session so stale authorizations do not linger.
	// Failed terminal attempts still expire and clean temp session outputs because
	// no durable artifact handoff completed for that branch.
	if updatedTask.Status == "completed" {
		stopScreenSession(screenClient, screenSession.ScreenSessionID, "analysis_completed")
		cleanupSuccessfulScreenSession(screenClient, screenSession.ScreenSessionID, candidate.Path)
	} else if taskIsTerminal(updatedTask.Status) {
		expireAndCleanupScreenSession(screenClient, screenSession.ScreenSessionID, "analysis_failed")
	}
	return updatedTask, bubble, deliveryResult, nil
}

// captureScreenCandidateAfterApproval keeps the controlled screen entry on one
// orchestrator path while still selecting the owner-5 capture primitive that
// matches the approved screen analysis mode.
func captureScreenCandidateAfterApproval(screenClient tools.ScreenCaptureClient, screenSessionID string, task runengine.TaskRecord, pendingExecution map[string]any, captureMode tools.ScreenCaptureMode) (tools.ScreenFrameCandidate, error) {
	input := tools.ScreenCaptureInput{
		ScreenSessionID: screenSessionID,
		TaskID:          task.TaskID,
		RunID:           task.RunID,
		CaptureMode:     captureMode,
		Source:          firstNonEmptyString(stringValue(pendingExecution, "source", ""), "screen_capture"),
		SourcePath:      stringValue(pendingExecution, "source_path", ""),
	}
	switch captureMode {
	case tools.ScreenCaptureModeKeyframe:
		result, err := screenClient.CaptureKeyframe(context.Background(), input)
		if err != nil {
			return tools.ScreenFrameCandidate{}, err
		}
		return result.Candidate, nil
	default:
		return screenClient.CaptureScreenshot(context.Background(), input)
	}
}

func stopScreenSession(screenClient tools.ScreenCaptureClient, screenSessionID, reason string) {
	if screenClient == nil || strings.TrimSpace(screenSessionID) == "" {
		return
	}
	_, _ = screenClient.StopSession(context.Background(), screenSessionID, reason)
}

// cleanupSuccessfulScreenSession only clears the tracked capture file that the
// screen client still owns after execution has already promoted durable
// artifacts. Deferred execution cleanup plans keep managing any extra temp clip
// derivatives, so this path must not recursively wipe the whole session dir.
func cleanupSuccessfulScreenSession(screenClient tools.ScreenCaptureClient, screenSessionID, capturePath string) {
	if screenClient == nil || strings.TrimSpace(screenSessionID) == "" || strings.TrimSpace(capturePath) == "" {
		return
	}
	_, _ = screenClient.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{
		ScreenSessionID: screenSessionID,
		Reason:          "analysis_completed",
		Paths:           []string{capturePath},
	})
}

// expireAndCleanupScreenSession keeps failed screen-analysis attempts from
// leaving temporary session state behind when no durable artifact is produced.
func expireAndCleanupScreenSession(screenClient tools.ScreenCaptureClient, screenSessionID, reason string) {
	if screenClient == nil || strings.TrimSpace(screenSessionID) == "" {
		return
	}
	_, _ = screenClient.ExpireSession(context.Background(), screenSessionID, reason)
	_, _ = screenClient.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{
		ScreenSessionID: screenSessionID,
		Reason:          reason,
	})
}

// cleanupExpiredScreenTemps keeps new screen-analysis executions from piling up
// abandoned temp outputs left behind by older expired sessions.
func cleanupExpiredScreenTemps(screenClient tools.ScreenCaptureClient, reason string, expiredBefore time.Time) {
	if screenClient == nil {
		return
	}
	if expiredBefore.IsZero() {
		expiredBefore = time.Now().UTC()
	}
	_, _ = screenClient.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{
		Reason:        reason,
		ExpiredBefore: expiredBefore.UTC(),
	})
}
