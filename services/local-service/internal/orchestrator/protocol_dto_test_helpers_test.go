package orchestrator

func startTaskForTest(s *Service, params map[string]any) (map[string]any, error) {
	response, err := s.StartTaskFromParams(StartTaskRequestFromParams(params).ProtocolParamsMap())
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func submitInputForTest(s *Service, params map[string]any) (map[string]any, error) {
	response, err := s.SubmitInputFromParams(SubmitInputRequestFromParams(params).ProtocolParamsMap())
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}

func taskDetailGetForTest(s *Service, params map[string]any) (map[string]any, error) {
	response, err := s.TaskDetailGetFromParams(TaskDetailGetRequestFromParams(params).ProtocolParamsMap())
	if err != nil {
		return nil, err
	}
	return response.Map(), nil
}
