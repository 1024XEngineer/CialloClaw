package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ToolDef 工具定义
type ToolDef struct {
	Name        string
	Command     string
	Description string
	Args        []ArgDef
	SkillName   string
}

// ArgDef 参数定义
type ArgDef struct {
	Name        string
	Type        string
	Description string
}

// SkillParser 解析 skill 文件
type SkillParser struct {
	skillsDir string
}

// NewSkillParser 创建解析器
func NewSkillParser(dir string) *SkillParser {
	return &SkillParser{skillsDir: dir}
}

// ParseAll 解析所有 skill 文件
func (sp *SkillParser) ParseAll() ([]ToolDef, error) {
	var tools []ToolDef

	if sp.skillsDir == "" {
		return tools, nil
	}

	entries, err := os.ReadDir(sp.skillsDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// 解析 skill.md
			skillFile := filepath.Join(sp.skillsDir, entry.Name(), "skill.md")
			if content, err := os.ReadFile(skillFile); err == nil {
				parsed := sp.parseContent(string(content), filepath.Join(sp.skillsDir, entry.Name()), entry.Name())
				tools = append(tools, parsed...)
			}
		} else if strings.HasSuffix(entry.Name(), ".md") || strings.HasSuffix(entry.Name(), ".skill") {
			skillName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			content, err := os.ReadFile(filepath.Join(sp.skillsDir, entry.Name()))
			if err != nil {
				continue
			}
			parsed := sp.parseContent(string(content), sp.skillsDir, skillName)
			tools = append(tools, parsed...)
		}
	}

	return tools, nil
}

// parseContent 解析单个 skill 文件
func (sp *SkillParser) parseContent(content, skillDir, skillName string) []ToolDef {
	var tools []ToolDef

	// 匹配工具块
	toolPattern := regexp.MustCompile(`(?ms)###\s+(\w+)\s*\n.*?\*\*命令\*\*:\s*(.+?)\n.*?\*\*描述\*\*:\s*(.+?)(?:\n|$)`)
	
	for _, match := range toolPattern.FindAllStringSubmatch(content, -1) {
		if len(match) < 4 {
			continue
		}

		tool := ToolDef{
			Name:        match[1],
			Command:     strings.TrimSpace(match[2]),
			Description: strings.TrimSpace(match[3]),
			SkillName:   skillName,
			Args:        []ArgDef{},
		}

		// 提取参数
		args := extractArgs(content, match[0])
		tool.Args = args

		// 解析命令中的变量
		tool.Command = resolveVariables(tool.Command, skillDir)

		if tool.Command != "" {
			tools = append(tools, tool)
		}
	}

	return tools
}

// extractArgs 从工具块中提取参数定义
func extractArgs(block, toolBlock string) []ArgDef {
	var args []ArgDef
	
	// 匹配 - name (type): description 格式
	argPattern := regexp.MustCompile(`-\s+(\w+)\s*\((\w+)\):\s*(.+?)(?:\n|$)`)
	
	for _, match := range argPattern.FindAllStringSubmatch(block, -1) {
		if len(match) < 4 {
			continue
		}
		args = append(args, ArgDef{
			Name:        match[1],
			Type:        match[2],
			Description: strings.TrimSpace(match[3]),
		})
	}
	
	return args
}

// resolveVariables 解析命令中的变量和路径
func resolveVariables(cmd, skillDir string) string {
	// 处理 {{var}} 变量 - 保留变量名由调用时替换
	// 处理相对路径 -> 相对于 skill 目录
	if !strings.HasPrefix(cmd, "/") && !strings.HasPrefix(cmd, ".") && !strings.HasPrefix(cmd, "http") {
		// 可能是相对路径
		return "./" + cmd
	}
	return cmd
}
