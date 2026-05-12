package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

const taskConfirmQuestionModelTimeout = 3 * time.Second

func (s *Service) bubbleTextForInput(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	return s.bubbleTextForConfirmation(task, snapshot, suggestion, false)
}

func (s *Service) bubbleTextForStart(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	return s.bubbleTextForConfirmation(task, snapshot, suggestion, true)
}

func (s *Service) bubbleTextForConfirmation(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, startFlow bool) string {
	if !suggestion.RequiresConfirm {
		return suggestion.ResultBubbleText
	}
	if shouldUseClarificationBubble(suggestion) {
		if !suggestion.IntentConfirmed {
			return initialClarificationPromptForLanguage(snapshot, startFlow, snapshot.SessionReplyLanguage)
		}
		return clarificationBubbleTextForLanguage(suggestion.Intent, s.clarificationPreviewHits(task, snapshot), clarificationReplyLanguage(snapshot))
	}
	if !suggestion.IntentConfirmed {
		if startFlow {
			return presentation.Text(presentation.MessageBubbleStartConfirmUnknown, nil)
		}
		return presentation.Text(presentation.MessageBubbleInputConfirmUnknown, nil)
	}
	return s.confirmIntentText(snapshot, suggestion)
}

func shouldUseClarificationBubble(suggestion intent.Suggestion) bool {
	if !suggestion.IntentConfirmed {
		return true
	}
	return strings.TrimSpace(stringValue(suggestion.Intent, "name", "")) == "agent_loop"
}

// confirmIntentText prefers a model-authored confirmation question that can use
// the inferred intent payload plus the current task object context. When the
// runtime model is unavailable or returns unusable text, the backend falls back
// to a deterministic question so the confirmation gate remains stable.
func (s *Service) confirmIntentText(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	if question := s.modelBackedConfirmIntentText(snapshot, suggestion); question != "" {
		return question
	}
	return fallbackConfirmIntentText(snapshot, suggestion)
}

func (s *Service) modelBackedConfirmIntentText(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	modelService := s.currentModel()
	if modelService == nil || !shouldUseModelBackedConfirmIntentText(suggestion) {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), taskConfirmQuestionModelTimeout)
	defer cancel()
	response, err := modelService.GenerateText(ctx, model.GenerateTextRequest{
		Input: buildTaskConfirmQuestionPrompt(snapshot, suggestion),
	})
	if err != nil {
		return ""
	}
	return normalizeTaskConfirmQuestion(response.OutputText)
}

func shouldUseModelBackedConfirmIntentText(suggestion intent.Suggestion) bool {
	intentName := strings.TrimSpace(stringValue(suggestion.Intent, "name", ""))
	if intentName == "" || intentName == "agent_loop" || intentName == "screen_analyze" {
		return false
	}
	// Confirmation copy should stay local for free-text or pre-confirm follow-ups
	// so the backend does not spend extra model calls before the user has approved
	// the task. A structured intent target is the minimum signal that justifies a
	// model-authored question.
	return strings.TrimSpace(intentConfirmArgumentTarget(suggestion.Intent)) != ""
}

// fallbackConfirmIntentText keeps the confirmation gate functional even when a
// runtime model is unavailable. It still derives the question from the formal
// task title and current object context instead of restoring a frontend phrase
// table or a per-intent copy map.
func fallbackConfirmIntentText(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	action, subject := intentConfirmTitleParts(suggestion.TaskTitle)
	if subject == "" {
		action = intentConfirmAction(snapshot, suggestion.Intent)
		subject = intentConfirmSubject(snapshot, suggestion.Intent)
	}

	switch {
	case action != "" && subject != "":
		return fmt.Sprintf("你现在是希望我%s「%s」吗？", action, subject)
	case action != "":
		return fmt.Sprintf("你现在是希望我%s吗？", action)
	default:
		return presentation.Text(presentation.MessageBubbleConfirmDefault, nil)
	}
}

func intentConfirmAction(snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any) string {
	intentName := strings.TrimSpace(stringValue(taskIntent, "name", ""))
	if intentName == "" {
		return ""
	}
	const subjectMarker = "__confirm_subject__"
	title := presentation.TaskTitle(intentName, presentation.TaskTitleOptions{
		Subject:  subjectMarker,
		HasError: strings.TrimSpace(snapshot.ErrorText) != "",
		IsFile:   len(snapshot.Files) > 0,
	})
	prefix, _, ok := strings.Cut(title, subjectMarker)
	if !ok {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(prefix, "：:"))
}

func buildTaskConfirmQuestionPrompt(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	intentPayload, _ := json.Marshal(suggestion.Intent)
	lines := []string{
		"You write one confirmation question for CialloClaw before the task executes.",
		"Return exactly one concise Chinese question and nothing else.",
		"Use the inferred operation and the most specific target object you can justify from the intent payload and task context.",
		"Do not mention internal labels such as intent, task title, current intent, JSON, payload, or field names.",
		"Do not explain your reasoning. Do not add bullets, quotes, or multiple options.",
		"If the task changes files, pages, or other state, make that action explicit in the question.",
		"",
		fmt.Sprintf("task_title=%s", strings.TrimSpace(suggestion.TaskTitle)),
		fmt.Sprintf("intent_payload=%s", string(intentPayload)),
		fmt.Sprintf("source_type=%s", strings.TrimSpace(suggestion.TaskSourceType)),
		fmt.Sprintf("delivery_type=%s", strings.TrimSpace(suggestion.DirectDeliveryType)),
		"",
		"task_context:",
		taskConfirmQuestionContextSummary(snapshot),
	}
	return strings.Join(lines, "\n")
}

func taskConfirmQuestionContextSummary(snapshot taskcontext.TaskContextSnapshot) string {
	parts := []string{
		fmt.Sprintf("input_type=%s", snapshot.InputType),
		fmt.Sprintf("text=%s", truncateTaskConfirmQuestionField(snapshot.Text)),
		fmt.Sprintf("selection_text=%s", truncateTaskConfirmQuestionField(snapshot.SelectionText)),
		fmt.Sprintf("error_text=%s", truncateTaskConfirmQuestionField(snapshot.ErrorText)),
		fmt.Sprintf("files=%s", strings.Join(snapshot.Files, ",")),
		fmt.Sprintf("page_title=%s", truncateTaskConfirmQuestionField(snapshot.PageTitle)),
		fmt.Sprintf("page_url=%s", truncateTaskConfirmQuestionField(snapshot.PageURL)),
		fmt.Sprintf("window_title=%s", truncateTaskConfirmQuestionField(snapshot.WindowTitle)),
		fmt.Sprintf("visible_text=%s", truncateTaskConfirmQuestionField(snapshot.VisibleText)),
		fmt.Sprintf("screen_summary=%s", truncateTaskConfirmQuestionField(snapshot.ScreenSummary)),
		fmt.Sprintf("hover_target=%s", truncateTaskConfirmQuestionField(snapshot.HoverTarget)),
	}
	return strings.Join(parts, "\n")
}

func truncateTaskConfirmQuestionField(value string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return textutil.TruncateGraphemes(trimmed, 240)
}

func normalizeTaskConfirmQuestion(raw string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	trimmed = strings.Trim(trimmed, "\"'`“”")
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(trimmed, "当前意图") ||
		strings.Contains(lower, "intent") ||
		strings.Contains(lower, "json") ||
		strings.Contains(lower, "payload") {
		return ""
	}
	if !strings.HasSuffix(trimmed, "？") && !strings.HasSuffix(trimmed, "?") {
		return ""
	}
	return textutil.TruncateGraphemes(trimmed, 80)
}

func intentConfirmTitleParts(taskTitle string) (string, string) {
	trimmed := strings.TrimSpace(taskTitle)
	if trimmed == "" {
		return "", ""
	}

	parts := strings.SplitN(trimmed, "：", 2)
	action := strings.TrimSpace(parts[0])
	if action == "" {
		return "", ""
	}
	if action == "确认处理方式" {
		action = "处理"
	}

	if len(parts) == 1 {
		return action, ""
	}

	subject := normalizeIntentConfirmSubject(parts[1])
	if isGenericIntentConfirmSubject(subject) {
		return action, ""
	}
	return action, subject
}

func intentConfirmSubject(snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any) string {
	switch {
	case normalizeIntentConfirmSubject(intentConfirmArgumentTarget(taskIntent)) != "":
		return normalizeIntentConfirmSubject(intentConfirmArgumentTarget(taskIntent))
	case len(snapshot.Files) > 0:
		return normalizeIntentConfirmSubject(filepath.Base(snapshot.Files[0]))
	case strings.TrimSpace(snapshot.SelectionText) != "":
		return normalizeIntentConfirmSubject(snapshot.SelectionText)
	case strings.TrimSpace(snapshot.Text) != "":
		return normalizeIntentConfirmSubject(snapshot.Text)
	case strings.TrimSpace(snapshot.ErrorText) != "":
		return normalizeIntentConfirmSubject(snapshot.ErrorText)
	case strings.TrimSpace(snapshot.HoverTarget) != "":
		return normalizeIntentConfirmSubject(snapshot.HoverTarget)
	case strings.TrimSpace(snapshot.PageTitle) != "":
		return normalizeIntentConfirmSubject(snapshot.PageTitle)
	default:
		return ""
	}
}

func intentConfirmArgumentTarget(taskIntent map[string]any) string {
	arguments := mapValue(taskIntent, "arguments")
	candidates := []string{
		stringValue(arguments, "target_path", ""),
		stringValue(arguments, "path", ""),
		stringValue(arguments, "target_file", ""),
		stringValue(arguments, "target_object", ""),
		stringValue(arguments, "url", ""),
		stringValue(arguments, "page_title", ""),
		stringValue(arguments, "goal", ""),
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

func isGenericIntentConfirmSubject(subject string) bool {
	return subject == "" ||
		subject == presentation.Text(presentation.MessageTaskTitleCurrentTask, nil) ||
		subject == "当前内容"
}

func normalizeIntentConfirmSubject(raw string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	trimmed = strings.Trim(trimmed, "\"'`“”")
	if trimmed == "" {
		return ""
	}
	return textutil.TruncateGraphemes(trimmed, 32)
}
