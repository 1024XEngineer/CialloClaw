package orchestrator

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/perception"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/recommendation"
)

// RecommendationGet handles agent.recommendation.get and returns lightweight
// recommendation actions derived from current context signals.
func (s *Service) RecommendationGet(params map[string]any) (map[string]any, error) {
	contextValue := mapValue(params, "context")
	signals := perception.CaptureContextSignals(stringValue(params, "source", "floating_ball"), stringValue(params, "scene", "hover"), contextValue)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 20, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 20, 0)
	notepadItems, _ := s.runEngine.NotepadItems("", 20, 0)
	result := s.recommendation.Get(recommendation.GenerateInput{
		Source:          stringValue(params, "source", "floating_ball"),
		Scene:           stringValue(params, "scene", "hover"),
		PageTitle:       signals.PageTitle,
		PageURL:         signals.PageURL,
		AppName:         signals.AppName,
		WindowTitle:     signals.WindowTitle,
		VisibleText:     signals.VisibleText,
		ScreenSummary:   signals.ScreenSummary,
		SelectionText:   signals.SelectionText,
		ClipboardText:   signals.ClipboardText,
		ClipboardMime:   signals.ClipboardMimeType,
		HoverTarget:     signals.HoverTarget,
		LastAction:      signals.LastAction,
		ErrorText:       signals.ErrorText,
		DwellMillis:     signals.DwellMillis,
		WindowSwitches:  signals.WindowSwitchCount,
		PageSwitches:    signals.PageSwitchCount,
		CopyCount:       signals.CopyCount,
		Observations:    s.recommendationObservations(signals),
		Signals:         signals,
		UnfinishedTasks: unfinishedTasks,
		FinishedTasks:   finishedTasks,
		NotepadItems:    notepadItems,
	})
	return map[string]any{
		"cooldown_hit": result.CooldownHit,
		"items":        result.Items,
	}, nil
}

func (s *Service) recommendationObservations(signals perception.SignalSnapshot) []string {
	observations := perception.BehaviorSignals(signals)
	if hasErrorOpportunity := strings.TrimSpace(signals.ErrorText) != "" || strings.Contains(strings.ToLower(strings.Join([]string{signals.VisibleText, signals.ScreenSummary}, " ")), "error") || strings.Contains(strings.ToLower(strings.Join([]string{signals.VisibleText, signals.ScreenSummary}, " ")), "报错"); hasErrorOpportunity {
		observations = append(observations, "当前上下文包含可解释的视觉错误信号。")
	}
	if strings.TrimSpace(signals.ScreenSummary) != "" {
		observations = append(observations, fmt.Sprintf("screen:%s", truncateText(signals.ScreenSummary, 48)))
	}
	if strings.TrimSpace(signals.VisibleText) != "" {
		observations = append(observations, fmt.Sprintf("visible:%s", truncateText(signals.VisibleText, 48)))
	}
	return uniqueTrimmedStrings(observations)
}

func uniqueTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
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

// RecommendationFeedbackSubmit records user feedback for a recommendation
// candidate without changing task state or creating delivery results.
func (s *Service) RecommendationFeedbackSubmit(params map[string]any) (map[string]any, error) {
	return map[string]any{
		"applied": s.recommendation.SubmitFeedback(
			stringValue(params, "recommendation_id", ""),
			stringValue(params, "feedback", ""),
		),
	}, nil
}
