package orchestrator

import (
	"context"
	"strings"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
)

// resolvedTaskTitle keeps model-backed title generation on the formal task path
// while treating the generated text as the final user-facing title.
func (s *Service) resolvedTaskTitle(snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, fallback string) string {
	intentName := strings.TrimSpace(stringValue(taskIntent, "name", ""))
	subjectFallback := strings.TrimSpace(originalTextFromTaskTitle(fallback))
	if subjectFallback == "" {
		subjectFallback = intent.ComposeTaskTitle(snapshot, intentName, "")
	}
	if s == nil || s.titleGenerator == nil {
		return intent.ComposeTaskTitle(snapshot, intentName, subjectFallback)
	}
	return s.titleGenerator.GenerateTaskSubject(context.Background(), snapshot, intentName, subjectFallback)
}
