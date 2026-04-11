// 该文件负责插件管理层的最小骨架。
package plugin

// Service 提供当前模块的服务能力。
type Service struct {
	workers  []string
	sidecars []string
}

// NewService 创建并返回Service。
func NewService() *Service {
	return &Service{
		workers:  []string{"playwright_worker", "ocr_worker", "media_worker"},
		sidecars: []string{"playwright_sidecar"},
	}
}

// Workers 处理当前模块的相关逻辑。
func (s *Service) Workers() []string {
	return append([]string(nil), s.workers...)
}

// Sidecars 返回当前已知 sidecar 名称列表。
func (s *Service) Sidecars() []string {
	return append([]string(nil), s.sidecars...)
}
