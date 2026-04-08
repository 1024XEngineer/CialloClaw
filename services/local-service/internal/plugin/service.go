// 该文件负责插件管理层的最小骨架。
package plugin

type Service struct {
	workers []string
}

func NewService() *Service {
	return &Service{workers: []string{"playwright_worker", "ocr_worker", "media_worker"}}
}

func (s *Service) Workers() []string {
	return append([]string(nil), s.workers...)
}
