package orchestrator

func startTaskForTest(s *Service, request StartTaskRequest) (map[string]any, error) {
	response, err := s.StartTask(request)
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func submitInputForTest(s *Service, request SubmitInputRequest) (map[string]any, error) {
	response, err := s.SubmitInput(request)
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func taskDetailGetForTest(s *Service, request TaskDetailGetRequest) (map[string]any, error) {
	response, err := s.TaskDetailGet(request)
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}
