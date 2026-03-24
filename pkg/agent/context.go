package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"minimal-agent/pkg/providers"
)

// ContextBuilder builds prompts and messages for the LLM.
type ContextBuilder struct {
	workspace   string
	memory     *MemoryStore
	systemPrompt string
	skillsDir  string
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder(workspace string) *ContextBuilder {
	cb := &ContextBuilder{
		workspace:   workspace,
		memory:      NewMemoryStore(workspace),
		systemPrompt: defaultSystemPrompt,
	}
	
	// Set skills directory
	cb.skillsDir = filepath.Join(workspace, "skills")
	
	return cb
}

const defaultSystemPrompt = `You are a helpful AI assistant with access to tools.

You can use tools to help the user complete tasks. When you need to use a tool, respond with a tool call.

Available tools:
- shell: Execute shell commands. Takes a "command" parameter.

Guidelines:
- Be concise and helpful
- If you need to run commands, use the shell tool
- Explain what you're doing before using tools
- After getting tool results, provide a summary to the user`

// SetMemory sets the memory store.
func (cb *ContextBuilder) SetMemory(mem *MemoryStore) {
	cb.memory = mem
}

// SetSystemPrompt sets a custom system prompt.
func (cb *ContextBuilder) SetSystemPrompt(prompt string) {
	cb.systemPrompt = prompt
}

// BuildSystemPrompt returns the system prompt.
func (cb *ContextBuilder) BuildSystemPrompt() string {
	var prompt strings.Builder
	
	prompt.WriteString(cb.systemPrompt)
	prompt.WriteString("\n\n")
	
	// Add skills
	skillsContent := cb.loadSkills()
	if skillsContent != "" {
		prompt.WriteString("\n--- Available Skills ---\n")
		prompt.WriteString(skillsContent)
		prompt.WriteString("\n--- End Skills ---\n")
	}
	
	// Add memory context
	memContext := cb.memory.GetMemoryContext()
	if memContext != "" {
		prompt.WriteString(memContext)
	}
	
	return prompt.String()
}

// BuildMessages builds the message array for the LLM.
func (cb *ContextBuilder) BuildMessages(currentMessage string) []providers.Message {
	messages := []providers.Message{
		{Role: "system", Content: cb.BuildSystemPrompt()},
	}
	
	// Add memory context as a message if there's significant history
	messages = append(messages, providers.Message{
		Role:    "system",
		Content: cb.memory.GetContext(),
	})
	
	// Add current user message
	if currentMessage != "" {
		messages = append(messages, providers.Message{
			Role:    "user",
			Content: currentMessage,
		})
	}
	
	return messages
}

// loadSkills loads skills from the skills directory.
func (cb *ContextBuilder) loadSkills() string {
	var skills strings.Builder
	
	// Check if skills directory exists
	info, err := os.Stat(cb.skillsDir)
	if err != nil || !info.IsDir() {
		return ""
	}
	
	// Read all skill files
	entries, err := os.ReadDir(cb.skillsDir)
	if err != nil {
		return ""
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".skill") {
			continue
		}
		
		content, err := os.ReadFile(filepath.Join(cb.skillsDir, name))
		if err != nil {
			continue
		}
		
		// Add skill content
		skills.WriteString(fmt.Sprintf("\n### Skill: %s\n", strings.TrimSuffix(name, filepath.Ext(name))))
		skills.WriteString(string(content))
		skills.WriteString("\n")
	}
	
	return skills.String()
}

// GetMemory returns the memory store.
func (cb *ContextBuilder) GetMemory() *MemoryStore {
	return cb.memory
}
