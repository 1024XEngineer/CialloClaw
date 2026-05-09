package rpc

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"

func startTaskForTest(s *orchestrator.Service, params map[string]any) (map[string]any, error) {
	response, err := s.StartTask(orchestrator.StartTaskRequestFromParams(params))
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func submitInputForTest(s *orchestrator.Service, params map[string]any) (map[string]any, error) {
	response, err := s.SubmitInput(orchestrator.SubmitInputRequestFromParams(params))
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func taskDetailGetForTest(s *orchestrator.Service, params map[string]any) (map[string]any, error) {
	response, err := s.TaskDetailGet(orchestrator.TaskDetailGetRequestFromParams(params))
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}
