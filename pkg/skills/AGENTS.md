# pkg/skills/ — Skill System

## OVERVIEW

Parses skill definitions from markdown files, executes skill scripts as tools.

## STRUCTURE

```
pkg/skills/
├── parser.go       # SkillParser: reads SKILL.md, extracts tool definitions
└── script_tool.go  # ScriptTool: executes skill commands with variable substitution
```

## KEY TYPES

| Type | Responsibility |
|------|----------------|
| `SkillParser` | Regex-parses markdown for `### tool-name` blocks |
| `ScriptTool` | Runs shell commands with `{{var}}` substitution |
| `ToolDef` | Parsed tool: name, command, description, args |
| `ArgDef` | Parameter: name, type, description |

## SKILL MARKDOWN FORMAT

```markdown
### tool-name
**命令**: ./scripts/run.sh {{arg1}}
**描述**: What this tool does

**参数**:
- arg1 (string): Description of arg1
```

## CONVENTIONS

- Regex pattern: `###\s+(\w+)\s*\n.*?\*\*命令\*\*:\s*(.+?)\n.*?\*\*描述\*\*:\s*(.+?)`
- Args pattern: `-\s+(\w+)\s*\((\w+)\):\s*(.+?)`
- Relative paths prefixed with `./` if not starting with `/`, `.`, or `http`
- Working directory set to skill's folder during execution
- Windows: uses `cmd /C`, Unix: uses `sh -c`
