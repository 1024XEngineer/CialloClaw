package risk

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) DefaultLevel() string {
	return "green"
}
