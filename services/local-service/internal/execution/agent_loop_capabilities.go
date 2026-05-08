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
		UseWhen:   "需要读取工作区内某个已知文件的精确内容",
		AvoidWhen: "用户只需要目录概览，或还不知道具体文件路径",
		Constraints: []string{
			"仅限工作区文件",
			"不会推断缺失路径",
			"路径不确定时先用 list_dir",
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
		UseWhen:   "需要查看某个已知工作区目录下有哪些文件或子目录",
		AvoidWhen: "用户已经给出明确文件路径，并且真正需要的是文件内容而不是目录列表",
		Constraints: []string{
			"仅限工作区目录",
			"返回受限数量的目录项",
			"定位到目标文件后再用 read_file",
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
		UseWhen:   "需要读取某个网页的标题或主要可见文本",
		AvoidWhen: "用户只需要确认关键词是否出现，而不需要通读页面内容",
		Constraints: []string{
			"网页读取可能触发审批",
			"一次只读取一个绝对 URL",
			"不会执行页面交互",
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
		UseWhen:   "需要确认某个网页里是否出现某个关键词或短语",
		AvoidWhen: "用户需要完整页面内容，或需要进一步浏览页面结构",
		Constraints: []string{
			"网页读取可能触发审批",
			"一次只搜索一个绝对 URL",
			"返回受限数量的关键词命中",
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
		parts = append(parts, "适用场景："+useWhen)
	}
	if avoidWhen := strings.TrimSpace(c.AvoidWhen); avoidWhen != "" {
		parts = append(parts, "不适用场景："+avoidWhen)
	}
	if constraints := joinCapabilityConstraints(c.Constraints); constraints != "" {
		parts = append(parts, "约束："+constraints)
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
