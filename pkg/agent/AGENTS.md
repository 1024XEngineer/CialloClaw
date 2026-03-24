# pkg/agent/ — Core Agent Logic

## OVERVIEW

Agent loop with tool execution, conversation memory, and prompt context building.

## STRUCTURE

```
pkg/agent/
├── agent.go     # AgentLoop: main loop, tool execution, message processing
├── context.go   # ContextBuilder: prompt assembly, skills loading
└── memory.go    # MemoryStore: conversation history, long-term memory
```

## KEY TYPES

| Type | Responsibility |
|------|----------------|
| `AgentLoop` | Orchestrates LLM calls + tool execution (maxIter=10) |
| `ContextBuilder` | Builds system prompt with skills + memory context |
| `MemoryStore` | Thread-safe message storage with summary + long-term |

## TOOL EXECUTION FLOW

```
User input → memory.AddMessage() → build messages → LLM call
    ↓
If tool_calls: execute each → add results → loop
If content only: return response
```

## CONVENTIONS

- Message format: `[role] content` for storage
- Tool result format: `[tool:name:success|error] content`
- Memory limit: 100 messages (trims oldest)
- 60-second timeout per tool execution
- Skills loaded from `workspace/skills/*.md` at startup

## VARIABLES

- `defaultSystemPrompt`: Hardcoded in `context.go` (line 34)
- `maxIter`: 10 iterations max per message
