package execution

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/model"

const (
	defaultAgentLoopMaxToolIterations    = 4
	defaultAgentLoopPlannerRetryBudget   = 1
	defaultAgentLoopToolRetryBudget      = 1
	defaultAgentLoopContextCompressChars = 2400
	defaultAgentLoopContextKeepRecent    = 4
)

// ReplaceModel swaps the runtime model dependency used for future task
// executions. In-flight requests keep the model snapshot captured earlier in
// their execution path.
func (s *Service) ReplaceModel(modelService *model.Service) *Service {
	if s == nil {
		return nil
	}
	s.modelMu.Lock()
	s.model = modelService
	s.modelMu.Unlock()
	return s
}

// CurrentModel exposes the runtime model dependency so orchestrator and tests
// can verify which provider future tasks will use.
func (s *Service) CurrentModel() *model.Service {
	if s == nil {
		return nil
	}
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()
	return s.model
}

func (s *Service) currentModel() *model.Service {
	return s.CurrentModel()
}
