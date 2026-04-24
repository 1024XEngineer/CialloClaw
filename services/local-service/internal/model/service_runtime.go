package model

import serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"

// RuntimeConfig returns the immutable runtime model configuration currently
// carried by this service so orchestrator/bootstrap can rebuild future-task
// clients without guessing bootstrap defaults.
func (s *Service) RuntimeConfig() serviceconfig.ModelConfig {
	if s == nil {
		return serviceconfig.ModelConfig{}
	}
	return serviceconfig.ModelConfig{
		Provider:             s.provider,
		ModelID:              s.modelID,
		Endpoint:             s.endpoint,
		MaxToolIterations:    s.maxToolIterations,
		PlannerRetryBudget:   s.plannerRetryBudget,
		ToolRetryBudget:      s.toolRetryBudget,
		ContextCompressChars: s.contextCompressChars,
		ContextKeepRecent:    s.contextKeepRecent,
	}
}
