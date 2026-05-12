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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

const taskConfirmQuestionModelTimeout = 3 * time.Second

func (s *Service) bubbleTextForInput(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return presentation.Text(presentation.MessageBubbleInputConfirmUnknown, nil)
		}
		return s.confirmIntentText(snapshot, suggestion)
	}
	return suggestion.ResultBubbleText
}

func (s *Service) bubbleTextForStart(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return presentation.Text(presentation.MessageBubbleStartConfirmUnknown, nil)
		}
		return s.confirmIntentText(snapshot, suggestion)
	}
	return suggestion.ResultBubbleText
}

// confirmIntentText returns the deterministic confirmation question that keeps
// the main entry flow responsive. Model refinement runs asynchronously after
// the task has already entered the confirming_intent gate.
func (s *Service) confirmIntentText(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	return fallbackConfirmIntentText(snapshot, suggestion)
}

func (s *Service) maybeRefineConfirmIntentTextAsync(taskID string, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) {
	if strings.TrimSpace(taskID) == "" || !shouldUseModelBackedConfirmIntentText(suggestion) {
		return
	}
	modelService := s.currentModel()
	if modelService == nil {
		return
	}

	go func() {
		fallback := fallbackConfirmIntentText(snapshot, suggestion)
		question := s.modelBackedConfirmIntentText(modelService, snapshot, suggestion, fallback)
		if question == "" || question == fallback {
			return
		}

		task, ok := s.runEngine.GetTask(taskID)
		if !ok || task.Status != "confirming_intent" || task.CurrentStep != "intent_confirmation" {
			return
		}

		bubble := cloneMap(task.BubbleMessage)
		if len(bubble) == 0 {
			return
		}
		if stringValue(bubble, "text", "") == question {
			return
		}
		bubble["text"] = question
		_, _ = s.runEngine.SetPresentation(taskID, bubble, nil, nil)
	}()
}

func (s *Service) modelBackedConfirmIntentText(modelService *model.Service, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, fallback string) string {
	if modelService == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), taskConfirmQuestionModelTimeout)
	defer cancel()
	response, err := modelService.GenerateText(ctx, model.GenerateTextRequest{
		Input: buildTaskConfirmQuestionPrompt(snapshot, suggestion, fallback),
	})
	if err != nil {
		return ""
	}
	return normalizeTaskConfirmQuestion(response.OutputText, snapshot, suggestion, fallback)
}

func shouldUseModelBackedConfirmIntentText(suggestion intent.Suggestion) bool {
	intentName := strings.TrimSpace(stringValue(suggestion.Intent, "name", ""))
	if intentName == "" || intentName == "agent_loop" || intentName == "screen_analyze" {
		return false
	}
	if strings.TrimSpace(intentConfirmArgumentTarget(suggestion.Intent)) != "" {
		return true
	}
	action, subject := intentConfirmTitleParts(suggestion.TaskTitle)
	return action == "" || action == "处理" || subject == ""
}

// fallbackConfirmIntentText keeps the confirmation gate functional even when a
// runtime model is unavailable. It still derives the question from the formal
// task title and current object context instead of restoring a frontend phrase
// table or a per-intent copy map.
func fallbackConfirmIntentText(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	action, subject := intentConfirmTitleParts(suggestion.TaskTitle)
	if subject == "" {
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

func buildTaskConfirmQuestionPrompt(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, fallback string) string {
	intentPayload, _ := json.Marshal(suggestion.Intent)
	lines := []string{
		"You write one confirmation question for CialloClaw before the task executes.",
		"Return exactly one concise Chinese question and nothing else.",
		"Keep the confirmation loyal to the inferred operation and target object from the formal intent payload.",
		"Only refine the fallback question when you can stay on the same action and target.",
		"Do not mention internal labels such as intent, task title, current intent, JSON, payload, or field names.",
		"Do not explain your reasoning. Do not add bullets, quotes, or multiple options.",
		"If the task changes files, pages, or other state, make that action explicit in the question.",
		"",
		fmt.Sprintf("task_title=%s", strings.TrimSpace(suggestion.TaskTitle)),
		fmt.Sprintf("intent_payload=%s", string(intentPayload)),
		fmt.Sprintf("source_type=%s", strings.TrimSpace(suggestion.TaskSourceType)),
		fmt.Sprintf("delivery_type=%s", strings.TrimSpace(suggestion.DirectDeliveryType)),
		fmt.Sprintf("fallback_question=%s", fallback),
		"",
		"trusted_task_context:",
		taskConfirmQuestionContextSummary(trustedTaskConfirmQuestionSnapshot(snapshot)),
	}
	return strings.Join(lines, "\n")
}

func trustedTaskConfirmQuestionSnapshot(snapshot taskcontext.TaskContextSnapshot) taskcontext.TaskContextSnapshot {
	return taskcontext.TaskContextSnapshot{
		InputType:     snapshot.InputType,
		Text:          snapshot.Text,
		SelectionText: snapshot.SelectionText,
		ErrorText:     snapshot.ErrorText,
		Files:         append([]string(nil), snapshot.Files...),
	}
}

func taskConfirmQuestionContextSummary(snapshot taskcontext.TaskContextSnapshot) string {
	parts := []string{
		fmt.Sprintf("input_type=%s", snapshot.InputType),
		fmt.Sprintf("text=%s", truncateTaskConfirmQuestionField(snapshot.Text)),
		fmt.Sprintf("selection_text=%s", truncateTaskConfirmQuestionField(snapshot.SelectionText)),
		fmt.Sprintf("error_text=%s", truncateTaskConfirmQuestionField(snapshot.ErrorText)),
		fmt.Sprintf("files=%s", strings.Join(snapshot.Files, ",")),
	}
	return strings.Join(parts, "\n")
}

func truncateTaskConfirmQuestionField(value string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return textutil.TruncateGraphemes(trimmed, 240)
}

func normalizeTaskConfirmQuestion(raw string, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, fallback string) string {
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
	if !isSemanticallyTrustedConfirmQuestion(trimmed, snapshot, suggestion, fallback) {
		return ""
	}
	return textutil.TruncateGraphemes(trimmed, 80)
}

func isSemanticallyTrustedConfirmQuestion(question string, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, fallback string) bool {
	if strings.TrimSpace(question) == strings.TrimSpace(fallback) {
		return true
	}

	action, _ := intentConfirmTitleParts(suggestion.TaskTitle)
	if action != "" && action != "处理" && !strings.Contains(question, action) {
		return false
	}

	target := normalizeIntentConfirmSubject(intentConfirmArgumentTarget(suggestion.Intent))
	if target == "" {
		target = intentConfirmSubject(snapshot, suggestion.Intent)
	}
	if target == "" {
		return true
	}
	if strings.Contains(question, target) {
		return true
	}

	baseTarget := normalizeIntentConfirmSubject(filepath.Base(target))
	return baseTarget != "" && strings.Contains(question, baseTarget)
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
