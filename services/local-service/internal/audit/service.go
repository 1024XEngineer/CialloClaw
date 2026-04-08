// 该文件负责审计层的最小骨架。
package audit

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Status() string {
	return "ready"
}
