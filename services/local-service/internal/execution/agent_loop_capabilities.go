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
		Name:      "read_file",
		UseWhen:   "you need exact text from a known workspace file path.",
		AvoidWhen: "the user only needs a directory overview or the target path is still unknown.",
		Constraints: []string{
			"workspace files only",
			"cannot infer missing paths",
			"prefer list_dir first when the path is uncertain",
		},
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
		Name:      "list_dir",
		UseWhen:   "you need to inspect which files or folders exist under a known workspace directory.",
		AvoidWhen: "the user already gave an exact file path and needs file content instead of a listing.",
		Constraints: []string{
			"workspace directories only",
			"returns a bounded entry list",
			"use read_file after locating the target file",
		},
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
		Name:      "page_read",
		UseWhen:   "you need the title or visible text from a specific webpage.",
		AvoidWhen: "the user only needs keyword hits from a page without reading the full content.",
		Constraints: []string{
			"webpage read access may require approval",
			"reads a single absolute URL",
			"does not interact with the page",
		},
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
		Name:      "page_search",
		UseWhen:   "you need to confirm whether a keyword or phrase appears on a specific webpage.",
		AvoidWhen: "the user needs the full page content or broader page navigation.",
		Constraints: []string{
			"webpage read access may require approval",
			"searches a single absolute URL",
			"returns bounded keyword matches",
		},
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
	UseWhen     string
	AvoidWhen   string
	Constraints []string
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
		Description: c.plannerDescription(metadata.Description),
		InputSchema: cloneMap(c.InputSchema),
	}
}

func (c agentLoopCapabilitySpec) plannerDescription(baseDescription string) string {
	parts := make([]string, 0, 4)
	if description := strings.TrimSpace(baseDescription); description != "" {
		parts = append(parts, description)
	}
	if useWhen := strings.TrimSpace(c.UseWhen); useWhen != "" {
		parts = append(parts, "Use when: "+useWhen)
	}
	if avoidWhen := strings.TrimSpace(c.AvoidWhen); avoidWhen != "" {
		parts = append(parts, "Avoid when: "+avoidWhen)
	}
	if constraints := joinCapabilityConstraints(c.Constraints); constraints != "" {
		parts = append(parts, "Constraints: "+constraints)
	}
	return strings.Join(parts, " ")
}

func joinCapabilityConstraints(values []string) string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, ", ")
}
