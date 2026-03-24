package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"minimal-agent/pkg/providers"
	"minimal-agent/pkg/skills"
	"minimal-agent/pkg/tools"
)

// AgentLoop is the main agent loop with tool execution support.
type AgentLoop struct {
	in         chan string
	out        chan string
	context    *ContextBuilder
	memory     *MemoryStore
	tools      map[string]tools.Tool
	maxIter    int
}

// NewAgentLoop creates a new agent loop.
func NewAgentLoop(workspace string, skillDir string) *AgentLoop {
	cb := NewContextBuilder(workspace)
	mem := cb.GetMemory()
	
	al := &AgentLoop{
		in:      make(chan string, 16),
		out:     make(chan string, 16),
		context: cb,
		memory:  mem,
		tools:   make(map[string]tools.Tool),
		maxIter: 10,
	}
	
	// 注册默认 shell 工具
	al.tools["shell"] = &tools.ShellTool{}
	
	// 加载 skill 工具
	if skillDir != "" {
		parser := skills.NewSkillParser(skillDir)
		skillTools, err := parser.ParseAll()
		if err == nil {
			for _, t := range skillTools {
				al.tools[t.Name] = &skills.ScriptTool{
					Name:        t.Name,
					Command:     t.Command,
					Description: t.Description,
					Args:        t.Args,
					SkillDir:    filepath.Join(skillDir, t.SkillName),
				}
				fmt.Printf("[Skill] Loaded tool: %s (from %s)\n", t.Name, t.SkillName)
			}
		}
	}
	
	return al
}

// In returns the input channel.
func (al *AgentLoop) In() chan<- string {
	return al.in
}

// Out returns the output channel.
func (al *AgentLoop) Out() <-chan string {
	return al.out
}

// Run executes the agent loop with a provider.
func (al *AgentLoop) Run(ctx context.Context, provider providers.Provider, sysPrompt string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-al.in:
			response, err := al.processMessage(ctx, provider, msg, sysPrompt)
			if err != nil {
				al.out <- fmt.Sprintf("Error: %v", err)
			} else {
				al.out <- response
			}
		}
	}
}

// processMessage handles a single message with tool execution.
func (al *AgentLoop) processMessage(ctx context.Context, provider providers.Provider, userInput string, sysPrompt string) (string, error) {
	// Add user message to memory
	al.memory.AddMessage("user", userInput)
	
	// Build messages
	messages := []providers.Message{
		{Role: "system", Content: sysPrompt},
	}
	
	// Add memory context
	memContext := al.memory.GetContext()
	if memContext != "" {
		messages = append(messages, providers.Message{
			Role:    "system",
			Content: memContext,
		})
	}
	
	// Add current user message
	messages = append(messages, providers.Message{
		Role:    "user",
		Content: userInput,
	})
	
	// Get tool definitions
	toolDefs := al.getToolDefinitions()
	
	// Tool iteration loop
	iteration := 0
	for iteration < al.maxIter {
		iteration++
		
		// Send to LLM
		fmt.Print("Thinking...")
		responses, err := provider.SendMessage(ctx, messages, toolDefs)
		fmt.Print("\r         \r")
		
		if err != nil {
			return "", fmt.Errorf("LLM error: %w", err)
		}
		
		if len(responses) == 0 {
			return "", fmt.Errorf("no response from LLM")
		}
		
		resp := responses[0]
		
		// Check for tool calls
		if len(resp.ToolCalls) > 0 {
			// Add assistant message with tool calls
			messages = append(messages, providers.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})
			
			// Execute tools
			for _, tc := range resp.ToolCalls {
				fmt.Printf("\n[Using tool: %s]\n", tc.Function.Name)
				
				result := al.executeTool(ctx, tc.Function.Name, tc.Function.Arguments)
				
				// Add tool result to memory
				al.memory.AddToolResult(tc.Function.Name, result.Content, result.IsError)
				
				// Add tool result to messages
				messages = append(messages, providers.Message{
					Role:    "tool",
					Content: result.Content,
					Name:    tc.Function.Name,
				})
				
				// Show result
				if result.IsError {
					fmt.Printf("Tool error: %s\n", truncate(result.Content, 200))
				} else {
					fmt.Printf("Result: %s\n", truncate(result.Content, 200))
				}
			}
			
			// Continue loop to get final response
			continue
		}
		
		// No tool calls - return the response
		if resp.Content != "" {
			al.memory.AddMessage("assistant", resp.Content)
			return resp.Content, nil
		}
	}
	
	return "Max tool iterations reached without a response.", nil
}

// getToolDefinitions returns tool definitions for the LLM.
func (al *AgentLoop) getToolDefinitions() []providers.ToolDefinition {
	var defs []providers.ToolDefinition
	
	for _, tool := range al.tools {
		defs = append(defs, providers.ToolDefinition{
			Type: "function",
			Function: providers.ToolFunctionDefinition{
				Name:        tool.GetName(),
				Description: tool.GetDescription(),
				Parameters:  tool.GetParameters(),
			},
		})
	}
	
	return defs
}

// executeTool executes a tool by name with given arguments.
func (al *AgentLoop) executeTool(ctx context.Context, name string, argsJSON string) *providers.ToolResult {
	tool, ok := al.tools[name]
	if !ok {
		return &providers.ToolResult{
			Name:    name,
			Content: fmt.Sprintf("Tool '%s' not found", name),
			IsError: true,
		}
	}
	
	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		args = map[string]any{"command": argsJSON}
	}
	
	// Execute with timeout
	toolCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	
	return tool.Execute(toolCtx, args)
}

// Stop stops the agent loop.
func (al *AgentLoop) Stop() {}

// GetTools returns all registered tools.
func (al *AgentLoop) GetTools() map[string]tools.Tool {
	return al.tools
}

// truncate truncates a string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
