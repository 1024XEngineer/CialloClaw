package checkpoint

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Status() string {
	return "ready"
}
