// Package tools 提供工具注册中心。
//
// ToolRegistry 是 tools 模块的统一注册中心，用于：
//   - 注册工具
//   - 按名称查找工具
//   - 列出全部工具元数据
//   - 按来源筛选工具
//
// 该注册中心不负责插件市场、动态热加载或远程发现，
// 只负责进程内最小可用的注册与查找能力。
package tools

import (
	"fmt"
	"sort"
	"sync"
)

// ToolRegistry 是 P0 阶段的统一工具注册中心。
//
// 它维护 name -> Tool 的内存映射，保证：
//   - tool name 全局唯一
//   - 重复注册被拒绝
//   - 所有注册项都通过 ToolMetadata.Validate 校验
//   - 查找不存在工具时返回统一错误
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// Registry 是 ToolRegistry 的兼容别名。
//
// 仓库内已有模块使用 `*tools.Registry`，这里保留别名，
// 避免在引入 ToolRegistry 的同时扩大跨模块改动范围。
type Registry = ToolRegistry

// NewRegistry 创建并返回一个空的工具注册中心。
//
// 可选传入若干工具，创建时会立即注册；若注册失败则 panic，
// 这样可以在 bootstrap 阶段尽早暴露非法工具定义。
func NewRegistry(initialTools ...Tool) *Registry {
	registry := &ToolRegistry{
		tools: make(map[string]Tool),
	}
	for _, tool := range initialTools {
		registry.MustRegister(tool)
	}
	return registry
}

// Register 将一个工具注册到注册中心。
//
// 约束：
//   - tool 不能为空
//   - tool.Metadata() 必须合法
//   - name 必须唯一
func (r *ToolRegistry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("%w: nil tool", ErrToolValidationFailed)
	}

	metadata := tool.Metadata()
	if err := metadata.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[metadata.Name]; exists {
		return fmt.Errorf("%w: %s", ErrToolDuplicateName, metadata.Name)
	}

	r.tools[metadata.Name] = tool
	return nil
}

// MustRegister 注册工具；若失败则直接 panic。
//
// 适用于启动期静态注册，避免上层反复处理不可能恢复的配置错误。
func (r *ToolRegistry) MustRegister(tool Tool) {
	if err := r.Register(tool); err != nil {
		panic(err)
	}
}

// Get 按名称查找工具。
//
// 如果工具不存在，返回 ErrToolNotFound。
func (r *ToolRegistry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	return tool, nil
}

// List 返回当前已注册工具的元数据列表。
//
// 返回结果按 name 升序排序，便于测试与上层稳定消费。
func (r *ToolRegistry) List() []ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]ToolMetadata, 0, len(r.tools))
	for _, tool := range r.tools {
		items = append(items, tool.Metadata())
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items
}

// ListBySource 返回指定来源的工具元数据列表。
//
// 返回结果按 name 升序排序。
func (r *ToolRegistry) ListBySource(source ToolSource) []ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]ToolMetadata, 0)
	for _, tool := range r.tools {
		metadata := tool.Metadata()
		if metadata.Source == source {
			items = append(items, metadata)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items
}

// Names 返回当前已注册工具名称列表。
//
// 这是对现有 orchestrator 使用方式的兼容辅助方法。
func (r *ToolRegistry) Names() []string {
	items := r.List()
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// Count 返回当前已注册工具数量。
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}
