package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryStore provides conversation memory with context management.
type MemoryStore struct {
	mu          sync.RWMutex
	messages    []string
	summary     string
	longTerm    string // Long-term memory
	workspace   string
	maxMessages int
}

// NewMemoryStore creates a new memory store.
func NewMemoryStore(workspace string) *MemoryStore {
	return &MemoryStore{
		workspace:   workspace,
		maxMessages: 100, // Keep last 100 messages in context
	}
}

// AddMessage adds a message to memory.
func (m *MemoryStore) AddMessage(role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := fmt.Sprintf("[%s] %s", role, content)
	m.messages = append(m.messages, msg)

	// Trim old messages if exceeding limit
	if len(m.messages) > m.maxMessages {
		m.messages = m.messages[len(m.messages)-m.maxMessages:]
	}
}

// AddToolResult adds a tool execution result to memory.
func (m *MemoryStore) AddToolResult(toolName, result string, isError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := "success"
	if isError {
		status = "error"
	}
	msg := fmt.Sprintf("[tool:%s:%s] %s", toolName, status, result)
	m.messages = append(m.messages, msg)
}

// GetContext returns the current conversation context.
func (m *MemoryStore) GetContext() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ctx strings.Builder

	// Add summary if available
	if m.summary != "" {
		ctx.WriteString("## Summary\n")
		ctx.WriteString(m.summary)
		ctx.WriteString("\n\n")
	}

	// Add recent messages
	if len(m.messages) > 0 {
		ctx.WriteString("## Recent Conversation\n")
		for _, msg := range m.messages {
			ctx.WriteString(msg)
			ctx.WriteString("\n")
		}
	}

	// Add long-term memory if available
	if m.longTerm != "" {
		ctx.WriteString("\n## Long-term Memory\n")
		ctx.WriteString(m.longTerm)
	}

	return ctx.String()
}

// GetMessages returns all messages as a string slice.
func (m *MemoryStore) GetMessages() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.messages...)
}

// WriteLongTerm writes to long-term memory.
func (m *MemoryStore) WriteLongTerm(content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.longTerm += "\n[" + time.Now().Format("2006-01-02 15:04") + "]\n" + content
	return nil
}

// ReadLongTerm returns the long-term memory.
func (m *MemoryStore) ReadLongTerm() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.longTerm
}

// GetMemoryContext returns formatted memory for prompts.
func (m *MemoryStore) GetMemoryContext() string {
	ctx := m.GetContext()
	if ctx == "" {
		return ""
	}
	return "\n\n--- Conversation Context ---\n" + ctx + "\n--- End Context ---\n"
}

// SetSummary sets the conversation summary.
func (m *MemoryStore) SetSummary(summary string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summary = summary
}

// Clear clears all memory.
func (m *MemoryStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
	m.summary = ""
}
