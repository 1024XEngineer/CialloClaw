// 该文件负责风险评估层的最小骨架。
package risk

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) DefaultLevel() string {
	return "green"
}
