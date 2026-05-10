package orchestrator

func startTaskForTest(s *Service, params map[string]any) (map[string]any, error) {
	response, err := s.StartTask(StartTaskRequestFromParams(params))
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func submitInputForTest(s *Service, params map[string]any) (map[string]any, error) {
	response, err := s.SubmitInput(SubmitInputRequestFromParams(params))
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func taskDetailGetForTest(s *Service, params map[string]any) (map[string]any, error) {
	response, err := s.TaskDetailGet(TaskDetailGetRequestFromParams(params))
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}
