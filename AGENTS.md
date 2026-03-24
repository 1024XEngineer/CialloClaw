# PROJECT KNOWLEDGE BASE

**Generated:** 2026-03-24
**Language:** Go 1.21
**Module:** `minimal-agent`

## OVERVIEW

Minimal AI agent implementing PicoClaw core pattern. OpenAI-compatible LLM provider with tool execution loop, skill system, and conversation memory.

## STRUCTURE

```
minimal-agent/
├── cmd/main.go           # CLI entry point, config loading, agent bootstrap
├── config/config.go      # YAML config parser with env var overrides
├── pkg/
│   ├── agent/            # Core: AgentLoop, ContextBuilder, MemoryStore
│   ├── providers/        # OpenAI-compatible LLM HTTP client
│   ├── skills/           # Skill parser + script executor
│   └── tools/            # Tool interface + ShellTool implementation
└── workspace/skills/     # Skill definitions (markdown + scripts)
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Entry point / CLI | `cmd/main.go` | main(), config loading, echo mode |
| Agent loop | `pkg/agent/agent.go` | Tool execution iteration (maxIter=10) |
| LLM calls | `pkg/providers/llm.go` | OpenAI-compatible chat API |
| Skill definitions | `workspace/skills/*/SKILL.md` | Markdown with tool blocks |
| Config format | `config/config.yaml` | Provider key, model, workspace |

## CONVENTIONS

- **Package naming**: lowercase single words (`agent`, `providers`, `tools`, `skills`)
- **File naming**: lowercase with underscores for multi-word (`script_tool.go`)
- **Error handling**: Wrap with `fmt.Errorf("context: %w", err)`
- **Chinese comments**: Used in `pkg/skills/` (e.g., `// 解析 skill 文件`)
- **Provider names**: Auto-detected from API base URL (DeepSeek, Groq, Qwen, etc.)

## ANTI-PATTERNS (THIS PROJECT)

- ❌ **No tests exist** — zero test files in entire codebase
- ❌ **No CI/CD** — no `.github/workflows`, no Makefile
- ❌ **No linting** — no `.golangci.yml`, no formatter config
- ❌ **Binaries committed** — `main.exe` and `cmd/main.exe` in repo

## UNIQUE STYLES

- **Tool iteration loop**: `processMessage()` loops up to `maxIter` times, collecting tool calls until LLM returns content without tool calls
- **Memory format**: `[role] content` and `[tool:name:status] result` prefixes
- **Skills system**: Markdown files with `### tool-name` blocks, regex-parsed for `**命令**` and `**描述**`

## COMMANDS

```bash
# Build
go build -o main.exe ./cmd

# Run (with config)
./main.exe

# Run (with env vars)
API_KEY=sk-xxx MODEL=qwen-plus ./main.exe
```

## NOTES

- Default model: `gpt-3.5-turbo`
- Default API base: `https://api.openai.com/v1`
- Config search paths: `config/config.yaml`, `../config/config.yaml`, alongside executable
- Single dependency: `gopkg.in/yaml.v3`
- Windows-compatible: Uses `cmd /C` on Windows, `sh -c` on Unix
