// Package tools provides the process-local tool registry.
//
// ToolRegistry is the shared registry for:
//   - registering tools
//   - looking up tools by name
//   - listing tool metadata
//   - filtering tools by source
//
// It intentionally excludes plugin marketplace lookup, dynamic reloads, and
// remote discovery. The registry only owns the minimal in-process registration
// and lookup boundary.
package tools

import (
	"fmt"
	"sort"
	"sync"
)

// ToolRegistry stores process-local tool registrations.
//
// It maintains a name-to-tool map and guarantees that:
//   - tool names are globally unique within the process
//   - duplicate registration is rejected
//   - each tool passes ToolMetadata.Validate before registration
//   - missing lookups return the shared not-found error
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// Registry is the existing public alias for ToolRegistry.
// Keeping it avoids expanding call-site churn while ToolRegistry remains the
// implementation type.
type Registry = ToolRegistry

// NewRegistry returns an empty registry and optionally registers initial tools.
//
// Initial registration panics on invalid tools so bootstrap can fail fast on
// impossible static configuration.
func NewRegistry(initialTools ...Tool) *Registry {
	registry := &ToolRegistry{
		tools: make(map[string]Tool),
	}
	for _, tool := range initialTools {
		registry.MustRegister(tool)
	}
	return registry
}

// Register validates and stores one tool.
//
// The tool must be non-nil, its metadata must be valid, and its name must be
// unique in the registry.
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

// MustRegister registers one static bootstrap tool and panics on failure.
func (r *ToolRegistry) MustRegister(tool Tool) {
	if err := r.Register(tool); err != nil {
		panic(err)
	}
}

// Get returns the tool registered under name or ErrToolNotFound.
func (r *ToolRegistry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	return tool, nil
}

// List returns registered tool metadata sorted by name for stable callers.
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

// ListBySource returns registered tool metadata for one source sorted by name.
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

// Names returns registered tool names for existing orchestrator call sites.
func (r *ToolRegistry) Names() []string {
	items := r.List()
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools)
}
