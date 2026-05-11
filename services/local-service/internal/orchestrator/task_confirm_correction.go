package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

const taskConfirmCorrectionModelTimeout = 3 * time.Second

var errTaskConfirmCorrectionPayloadInvalid = errors.New("task confirm correction payload invalid")

type taskIntentCorrection struct {
	Intent map[string]any `json:"intent"`
	Reason string         `json:"reason"`
}

func validateTaskConfirmCorrectionPayload(confirmed bool, correctedIntent map[string]any, correctionText string) error {
	hasCorrectedIntent := len(correctedIntent) > 0
	hasCorrectionText := strings.TrimSpace(correctionText) != ""
	if confirmed && (hasCorrectedIntent || hasCorrectionText) {
		return fmt.Errorf("%w: confirmed=true cannot include corrected_intent or correction_text", errTaskConfirmCorrectionPayloadInvalid)
	}
	if hasCorrectedIntent && hasCorrectionText {
		return fmt.Errorf("%w: corrected_intent and correction_text are mutually exclusive", errTaskConfirmCorrectionPayloadInvalid)
	}
	return nil
}

// reinferTaskIntentFromCorrection preserves the same task and original context
// snapshot while letting a natural-language correction replace the next
// executing intent directly.
func (s *Service) reinferTaskIntentFromCorrection(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, correctionText string) intent.Suggestion {
	intentValue := s.inferTaskIntentFromCorrection(task, snapshot, correctionText)
	return s.normalizedTaskConfirmSuggestion(snapshot, intentValue, false)
}

func (s *Service) inferTaskIntentFromCorrection(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, correctionText string) map[string]any {
	if modelService := s.currentModel(); modelService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), taskConfirmCorrectionModelTimeout)
		defer cancel()
		response, err := modelService.GenerateText(ctx, model.GenerateTextRequest{
			TaskID: task.TaskID,
			RunID:  task.RunID,
			Input:  buildTaskConfirmCorrectionPrompt(task, snapshot, correctionText),
		})
		if err == nil {
			if intentValue, ok := parseTaskConfirmCorrectionIntent(response.OutputText); ok {
				return intentValue
			}
		}
	}
	return fallbackTaskConfirmCorrectionIntent(correctionText)
}

func buildTaskConfirmCorrectionPrompt(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, correctionText string) string {
	lines := []string{
		"You infer a corrected CialloClaw IntentPayload for an existing task confirmation.",
		"Return JSON only.",
		`Schema: {"intent":{"name":"<formal intent name>","arguments":{}},"reason":"short reason"}`,
		"Keep the same task and infer the next execution intent from the latest correction.",
		"Use the correction text as the user's latest instruction while preserving the original task context.",
		"Pick the most appropriate formal intent for the user's latest goal when the task is specific enough.",
		"Use agent_loop only when the goal remains open-ended, mixed, or underspecified after considering the full task context.",
		"",
		"Original task:",
		fmt.Sprintf("task_id=%s", task.TaskID),
		fmt.Sprintf("status=%s", task.Status),
		fmt.Sprintf("source_type=%s", task.SourceType),
		fmt.Sprintf("current_intent=%s", firstNonEmptyString(stringValue(task.Intent, "name", ""), "none")),
		fmt.Sprintf("delivery_type=%s", resolveTaskDeliveryType(task, task.Intent)),
		"",
		"Original context:",
		taskConfirmCorrectionContextSummary(snapshot),
		"",
		"User correction text:",
		strings.TrimSpace(correctionText),
	}
	return strings.Join(lines, "\n")
}

func taskConfirmCorrectionContextSummary(snapshot taskcontext.TaskContextSnapshot) string {
	parts := []string{
		fmt.Sprintf("source=%s", snapshot.Source),
		fmt.Sprintf("trigger=%s", snapshot.Trigger),
		fmt.Sprintf("input_type=%s", snapshot.InputType),
		fmt.Sprintf("text=%s", truncatePromptField(snapshot.Text)),
		fmt.Sprintf("selection_text=%s", truncatePromptField(snapshot.SelectionText)),
		fmt.Sprintf("error_text=%s", truncatePromptField(snapshot.ErrorText)),
		fmt.Sprintf("files=%s", strings.Join(snapshot.Files, ",")),
		fmt.Sprintf("page_title=%s", truncatePromptField(snapshot.PageTitle)),
		fmt.Sprintf("page_url=%s", truncatePromptField(snapshot.PageURL)),
		fmt.Sprintf("app_name=%s", truncatePromptField(snapshot.AppName)),
		fmt.Sprintf("window_title=%s", truncatePromptField(snapshot.WindowTitle)),
		fmt.Sprintf("visible_text=%s", truncatePromptField(snapshot.VisibleText)),
		fmt.Sprintf("screen_summary=%s", truncatePromptField(snapshot.ScreenSummary)),
	}
	return strings.Join(parts, "\n")
}

func truncatePromptField(value string) string {
	const limit = 240
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return textutil.TruncateGraphemes(trimmed, limit)
}

func parseTaskConfirmCorrectionIntent(raw string) (map[string]any, bool) {
	payload := extractJSONObject(raw)
	if payload == "" {
		return nil, false
	}
	var correction taskIntentCorrection
	if err := json.Unmarshal([]byte(payload), &correction); err == nil && len(correction.Intent) > 0 {
		return normalizeTaskConfirmIntent(correction.Intent)
	}
	var intentValue map[string]any
	if err := json.Unmarshal([]byte(payload), &intentValue); err != nil {
		return nil, false
	}
	return normalizeTaskConfirmIntent(intentValue)
}

// normalizeTaskConfirmIntent only validates the formal payload shape. Confirm
// corrections must keep the full orchestrator intent surface instead of
// reintroducing a confirmation-only allowlist that hides supported paths.
func normalizeTaskConfirmIntent(intentValue map[string]any) (map[string]any, bool) {
	name := strings.TrimSpace(stringValue(intentValue, "name", ""))
	if name == "" {
		return nil, false
	}
	arguments := cloneMap(mapValue(intentValue, "arguments"))
	if arguments == nil {
		arguments = map[string]any{}
	}
	return map[string]any{
		"name":      name,
		"arguments": arguments,
	}, true
}

func fallbackTaskConfirmCorrectionIntent(correctionText string) map[string]any {
	return map[string]any{
		"name": "agent_loop",
		"arguments": map[string]any{
			"goal":            strings.TrimSpace(correctionText),
			"correction_text": strings.TrimSpace(correctionText),
		},
	}
}

// normalizedTaskConfirmSuggestion reuses the same intent normalization and
// capability downgrade path as task creation so confirmation edits cannot
// persist unsupported intent names or unavailable screen-only intents.
func (s *Service) normalizedTaskConfirmSuggestion(snapshot taskcontext.TaskContextSnapshot, intentValue map[string]any, confirmRequired bool) intent.Suggestion {
	suggestion := s.intent.Suggest(snapshot, intentValue, confirmRequired)
	suggestion = s.normalizeSuggestedIntentForAvailability(snapshot, suggestion, confirmRequired)
	suggestion.RequiresConfirm = confirmRequired
	return suggestion
}
