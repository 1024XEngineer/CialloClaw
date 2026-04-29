package execution

import (
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// agentLoopCapabilityCatalog is the single source of truth for the current
// bounded agent loop tool surface. The planner prompt and runtime allowlist both
// derive from this catalog so the model cannot be shown one capability set while
// the executor silently accepts another.
var agentLoopCapabilityCatalog = []agentLoopCapabilitySpec{
	{
		Name: "read_file",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Workspace-relative path to a file."},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	},
	{
		Name: "list_dir",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":  map[string]any{"type": "string", "description": "Workspace-relative path to a directory."},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50},
			},
			"required":             []string{"path"},
			"additionalProperties": false,
		},
	},
	{
		Name: "page_read",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "Absolute URL to read."},
			},
			"required":             []string{"url"},
			"additionalProperties": false,
		},
	},
	{
		Name: "page_search",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":   map[string]any{"type": "string", "description": "Absolute URL to search."},
				"query": map[string]any{"type": "string", "description": "Query to search within the page."},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 20},
			},
			"required":             []string{"url", "query"},
			"additionalProperties": false,
		},
	},
}

type agentLoopCapabilitySpec struct {
	Name        string
	InputSchema map[string]any
}

// agentLoopToolDefinitions resolves the runtime-visible planner tools from the
// shared catalog and the live registry. Missing registry entries are skipped so
// partially wired environments never advertise tools that cannot execute.
func (s *Service) agentLoopToolDefinitions() []model.ToolDefinition {
	if s == nil || s.tools == nil {
		return nil
	}

	definitions := make([]model.ToolDefinition, 0, len(agentLoopCapabilityCatalog))
	for _, capability := range agentLoopCapabilityCatalog {
		metadata, ok := s.agentLoopToolMetadata(capability.Name)
		if !ok {
			continue
		}
		definitions = append(definitions, capability.toolDefinition(metadata))
	}
	return definitions
}

// isAllowedAgentLoopTool keeps the execution guard aligned with the planner
// catalog and the live registry. This prevents hallucinated or unregistered tool
// names from slipping past the allowlist even when they resemble supported tools.
func (s *Service) isAllowedAgentLoopTool(name string) bool {
	if s == nil || s.tools == nil {
		return false
	}

	capability, ok := agentLoopCapabilityByName(name)
	if !ok {
		return false
	}

	_, ok = s.agentLoopToolMetadata(capability.Name)
	return ok
}

func (s *Service) agentLoopToolMetadata(name string) (tools.ToolMetadata, bool) {
	tool, err := s.tools.Get(strings.TrimSpace(name))
	if err != nil {
		return tools.ToolMetadata{}, false
	}
	return tool.Metadata(), true
}

func agentLoopCapabilityByName(name string) (agentLoopCapabilitySpec, bool) {
	trimmed := strings.TrimSpace(name)
	for _, capability := range agentLoopCapabilityCatalog {
		if capability.Name == trimmed {
			return capability, true
		}
	}
	return agentLoopCapabilitySpec{}, false
}

func (c agentLoopCapabilitySpec) toolDefinition(metadata tools.ToolMetadata) model.ToolDefinition {
	return model.ToolDefinition{
		Name:        metadata.Name,
		Description: metadata.Description,
		InputSchema: cloneMap(c.InputSchema),
	}
}
