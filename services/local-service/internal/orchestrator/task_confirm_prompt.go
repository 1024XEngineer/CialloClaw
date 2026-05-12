package orchestrator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

func (s *Service) bubbleTextForInput(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return presentation.Text(presentation.MessageBubbleInputConfirmUnknown, nil)
		}
		return confirmIntentText(snapshot, suggestion)
	}
	return suggestion.ResultBubbleText
}

func (s *Service) bubbleTextForStart(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return presentation.Text(presentation.MessageBubbleStartConfirmUnknown, nil)
		}
		return confirmIntentText(snapshot, suggestion)
	}
	return suggestion.ResultBubbleText
}

// confirmIntentText derives one backend-owned confirmation question from the
// inferred task title and current object context so shell-ball does not need a
// frontend phrase table or a duplicated intent label.
func confirmIntentText(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion) string {
	action, subject := intentConfirmTitleParts(suggestion.TaskTitle)
	if subject == "" {
		subject = intentConfirmSubject(snapshot)
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
	if subject == "" || subject == presentation.Text(presentation.MessageTaskTitleCurrentTask, nil) {
		return action, ""
	}
	return action, subject
}

func intentConfirmSubject(snapshot taskcontext.TaskContextSnapshot) string {
	switch {
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

func normalizeIntentConfirmSubject(raw string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	trimmed = strings.Trim(trimmed, "\"'`“”")
	if trimmed == "" {
		return ""
	}
	return textutil.TruncateGraphemes(trimmed, 32)
}
