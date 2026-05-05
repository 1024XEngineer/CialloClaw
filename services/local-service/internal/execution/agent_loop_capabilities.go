package execution

import (
	"strings"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// agentLoopCapabilityCatalog is the single source of truth for the bounded
// agent-loop tool surface. The planner prompt and runtime allowlist both derive
// from this catalog so the model cannot see one capability set while the
// executor silently accepts another.
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
		Name:                   "browser_attach_current",
		RequiresCurrentBrowser: true,
		UseWhen:                "需要附着当前真实浏览器标签页，并确认当前页面 URL 或标题",
		AvoidWhen:              "当前任务并不依赖用户真实浏览器，或上下文里没有受支持浏览器线索",
		Constraints: []string{
			"仅附着本地已开启调试端口的 Chrome/Edge",
			"执行层会自动注入附着线索",
			"不会隐式导航或交互页面",
		},
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
	},
	{
		Name:                   "browser_snapshot",
		RequiresCurrentBrowser: true,
		UseWhen:                "需要读取当前真实浏览器页的可见文本、标题和结构化摘要",
		AvoidWhen:              "用户已经提供明确 URL，并且只需要离线读取页面而不关心真实浏览器状态",
		Constraints: []string{
			"仅适用于当前真实浏览器标签页",
			"执行层会自动注入附着线索",
			"不会主动导航页面",
		},
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
	},
	{
		Name:                   "browser_tabs_list",
		RequiresCurrentBrowser: true,
		UseWhen:                "需要查看当前真实浏览器里有哪些标签页可供后续切换",
		AvoidWhen:              "用户已经明确指定当前页就足够，不需要切换其它标签页",
		Constraints: []string{
			"仅返回标签页摘要",
			"执行层会自动注入附着线索",
			"不会切换或关闭标签页",
		},
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
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
	{
		Name:      "structured_dom",
		UseWhen:   "需要快速了解页面的标题层级、链接、按钮和输入框结构",
		AvoidWhen: "用户只需要通读正文，或只需要确认某个关键词是否出现",
		Constraints: []string{
			"页面结构读取可能触发审批",
			"一次只提取一个绝对 URL 的结构摘要",
			"不会执行页面交互",
		},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "Absolute URL to inspect."},
			},
			"required":             []string{"url"},
			"additionalProperties": false,
		},
	},
}

type agentLoopCapabilitySpec struct {
	Name                   string
	RequiresCurrentBrowser bool
	UseWhen                string
	AvoidWhen              string
	Constraints            []string
	InputSchema            map[string]any
}

// agentLoopToolDefinitions resolves the runtime-visible planner tools from the
// shared catalog and the live registry. Missing registry entries are skipped so
// partially wired environments never advertise tools that cannot execute.
func (s *Service) agentLoopToolDefinitions() []model.ToolDefinition {
	return s.agentLoopToolDefinitionsForSnapshot(contextsvc.TaskContextSnapshot{})
}

func (s *Service) agentLoopToolDefinitionsForSnapshot(snapshot contextsvc.TaskContextSnapshot) []model.ToolDefinition {
	if s == nil || s.tools == nil {
		return nil
	}

	definitions := make([]model.ToolDefinition, 0, len(agentLoopCapabilityCatalog))
	for _, capability := range agentLoopCapabilityCatalog {
		if !capability.allowedForSnapshot(snapshot) {
			continue
		}
		metadata, ok := s.agentLoopToolMetadata(capability.Name)
		if !ok {
			continue
		}
		definitions = append(definitions, capability.toolDefinition(metadata))
	}
	return definitions
}

// isAllowedAgentLoopTool keeps the execution guard aligned with the planner
// catalog and the live registry. This prevents hallucinated or unregistered
// tool names from slipping past the allowlist.
func (s *Service) isAllowedAgentLoopTool(name string) bool {
	return s.isAllowedAgentLoopToolForSnapshot(name, contextsvc.TaskContextSnapshot{})
}

func (s *Service) isAllowedAgentLoopToolForSnapshot(name string, snapshot contextsvc.TaskContextSnapshot) bool {
	if s == nil || s.tools == nil {
		return false
	}
	capability, ok := agentLoopCapabilityByName(name)
	if !ok || !capability.allowedForSnapshot(snapshot) {
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

func resolveAgentLoopToolInput(toolName string, arguments map[string]any, snapshot contextsvc.TaskContextSnapshot) (map[string]any, bool) {
	trimmedName := strings.TrimSpace(toolName)
	if browserInput, ok := resolveBrowserToolInput(trimmedName, arguments, snapshot); ok {
		return browserInput, true
	}
	return resolveDirectToolInput(trimmedName, arguments, snapshot)
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

func (c agentLoopCapabilitySpec) allowedForSnapshot(snapshot contextsvc.TaskContextSnapshot) bool {
	if !c.RequiresCurrentBrowser {
		return true
	}
	browserKind := strings.ToLower(strings.TrimSpace(snapshot.BrowserKind))
	if browserKind != "chrome" && browserKind != "edge" {
		return false
	}
	if c.Name == "browser_attach_current" || c.Name == "browser_tabs_list" {
		return true
	}
	if strings.TrimSpace(snapshot.PageURL) != "" {
		return true
	}
	if strings.TrimSpace(snapshot.PageTitle) != "" {
		return true
	}
	return strings.TrimSpace(snapshot.WindowTitle) != ""
}
