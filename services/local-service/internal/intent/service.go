package intent

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Analyze(input string) string {
	if input == "" {
		return "awaiting_input"
	}

	return "confirm_intent"
}
