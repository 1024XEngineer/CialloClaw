// 该文件负责插件管理层的最小骨架。
package plugin

import "strings"

type SidecarSpec struct {
	Name      string
	Transport string
}

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

// HasSidecar 判断当前声明中是否包含指定 sidecar。
func (s *Service) HasSidecar(name string) bool {
	needle := strings.TrimSpace(name)
	if needle == "" {
		return false
	}
	for _, sidecar := range s.sidecars {
		if sidecar == needle {
			return true
		}
	}
	return false
}

// PrimarySidecar 返回当前最重要的 sidecar 名称。
func (s *Service) PrimarySidecar() string {
	if len(s.sidecars) == 0 {
		return ""
	}
	return s.sidecars[0]
}

// SidecarSpec 返回指定 sidecar 的最小运行时规格。
func (s *Service) SidecarSpec(name string) (SidecarSpec, bool) {
	if !s.HasSidecar(name) {
		return SidecarSpec{}, false
	}
	return SidecarSpec{
		Name:      name,
		Transport: "named_pipe",
	}, true
}
