package delivery

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) DefaultResultType() string {
	return "workspace_document"
}
