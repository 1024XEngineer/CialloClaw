// 该文件负责恢复点层的最小骨架。
package checkpoint

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Status() string {
	return "ready"
}
