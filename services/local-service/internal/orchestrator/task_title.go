package orchestrator

import (
	"context"
	"strings"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
)

// resolvedTaskTitle keeps model-backed title generation on the formal task path
// while preserving the stable intent-specific prefix contract.
func (s *Service) resolvedTaskTitle(snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, fallback string) string {
	intentName := strings.TrimSpace(stringValue(taskIntent, "name", ""))
	if intentName == "screen_analyze" {
		return firstNonEmptyString(strings.TrimSpace(fallback), "查看屏幕：当前内容")
	}
	subjectFallback := strings.TrimSpace(originalTextFromTaskTitle(fallback))
	if subjectFallback == "" {
		subjectFallback = "当前内容"
	}
	if s == nil || s.titleGenerator == nil {
		return intent.ComposeTaskTitle(snapshot, intentName, subjectFallback)
	}
	subject := s.titleGenerator.GenerateTaskSubject(context.Background(), snapshot, intentName, subjectFallback)
	return intent.ComposeTaskTitle(snapshot, intentName, subject)
}
