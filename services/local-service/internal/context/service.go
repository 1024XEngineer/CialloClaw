package context

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Snapshot() map[string]string {
	return map[string]string{"source": "desktop"}
}
