package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	taskcontext "github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

const (
	inputRouteTaskRequest         = "task_request"
	inputRouteClarificationNeeded = "clarification_needed"
	inputRouteSocialChat          = "social_chat"
)

type inputRouteDecision struct {
	Route string
	Reply string
}

// routeUnanchoredSubmitInput separates lightweight chat from task creation
// before the formal task/run mapping exists. Invalid or unavailable classifier
// output fails open to the task path so executable requests are not dropped.
func (s *Service) routeUnanchoredSubmitInput(ctx context.Context, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, confirmRequired bool) (inputRouteDecision, bool) {
	if !shouldRouteUnanchoredSubmitInput(snapshot, suggestion, confirmRequired) || s == nil || s.model == nil {
		return inputRouteDecision{}, false
	}
	response, err := s.model.GenerateText(ctx, model.GenerateTextRequest{
		Input: buildInputRoutePrompt(snapshot),
	})
	if err != nil {
		return inputRouteDecision{}, false
	}
	decision, ok := parseInputRouteDecision(response.OutputText)
	if !ok {
		return inputRouteDecision{}, false
	}
	return decision, true
}

func shouldRouteUnanchoredSubmitInput(snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, confirmRequired bool) bool {
	if confirmRequired || snapshot.InputType != "text" || strings.TrimSpace(snapshot.Text) == "" {
		return false
	}
	if len(snapshot.Files) > 0 || strings.TrimSpace(snapshot.SelectionText) != "" || strings.TrimSpace(snapshot.ErrorText) != "" {
		return false
	}
	return stringValue(suggestion.Intent, "name", "") == "agent_loop"
}

func buildInputRoutePrompt(snapshot taskcontext.TaskContextSnapshot) string {
	sections := []string{
		"Classify whether this near-field desktop input should create a formal task.",
		"",
		"Return only compact JSON in this exact shape:",
		`{"route":"social_chat|clarification_needed|task_request","reply":"..."}`,
		"",
		"Routes:",
		"- social_chat: greetings, emotions, emoji-only input, casual conversation, or lightweight questions that can be answered without acting on files, pages, tools, or system state.",
		"- clarification_needed: the user probably wants work done, but the target, object, or action is missing.",
		"- task_request: the user gives an executable goal such as summarize, translate, analyze, edit, generate, inspect, open, search, or write.",
		"",
		"Rules:",
		"- For social_chat, reply briefly in the user's language and do not promise task execution.",
		"- For clarification_needed and task_request, use an empty reply.",
		"- Passive foreground context below is not a task anchor by itself.",
		"",
		"User input:",
		strings.TrimSpace(snapshot.Text),
	}
	if contextSummary := passiveInputRouteContext(snapshot); contextSummary != "" {
		sections = append(sections, "", "Passive foreground context:", contextSummary)
	}
	return strings.Join(sections, "\n")
}

func passiveInputRouteContext(snapshot taskcontext.TaskContextSnapshot) string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(snapshot.AppName) != "" {
		parts = append(parts, "app="+snapshot.AppName)
	}
	if strings.TrimSpace(snapshot.PageTitle) != "" {
		parts = append(parts, "page_title="+snapshot.PageTitle)
	}
	if strings.TrimSpace(snapshot.WindowTitle) != "" {
		parts = append(parts, "window_title="+snapshot.WindowTitle)
	}
	if strings.TrimSpace(snapshot.HoverTarget) != "" {
		parts = append(parts, "hover_target="+snapshot.HoverTarget)
	}
	return strings.Join(parts, "\n")
}

func parseInputRouteDecision(raw string) (inputRouteDecision, bool) {
	payload := extractJSONObject(raw)
	if payload == "" {
		return inputRouteDecision{}, false
	}
	var decoded struct {
		Route string `json:"route"`
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return inputRouteDecision{}, false
	}
	decision := inputRouteDecision{
		Route: strings.TrimSpace(decoded.Route),
		Reply: strings.TrimSpace(decoded.Reply),
	}
	switch decision.Route {
	case inputRouteSocialChat:
		if decision.Reply == "" {
			decision.Reply = "我在。"
		}
		return decision, true
	case inputRouteClarificationNeeded, inputRouteTaskRequest:
		return decision, true
	default:
		return inputRouteDecision{}, false
	}
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return ""
	}
	return trimmed[start : end+1]
}

func (s *Service) socialChatInputResponse(decision inputRouteDecision) map[string]any {
	createdAt := time.Now().Format(dateTimeLayout)
	bubble := s.delivery.BuildBubbleMessage("", "result", decision.Reply, createdAt)
	return map[string]any{
		"task":            nil,
		"bubble_message":  bubble,
		"delivery_result": nil,
	}
}

func applyInputRouteDecision(suggestion intent.Suggestion, decision inputRouteDecision) intent.Suggestion {
	if decision.Route != inputRouteClarificationNeeded {
		return suggestion
	}
	updated := suggestion
	updated.RequiresConfirm = true
	return updated
}
