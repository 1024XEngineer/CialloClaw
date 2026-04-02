package agents

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"cialloclaw/internal/state/memory"
	"cialloclaw/internal/state/task"
)

type Result struct {
	Summary  string
	ToolName string
	ToolArgs map[string]any
}

type Agent interface {
	Name() string
	Capabilities() []string
	CanHandle(t task.Task) bool
	Execute(ctx context.Context, t task.Task, wm memory.WorkingMemory) (Result, error)
}

type Registry struct {
	mu     sync.RWMutex
	agents map[string]Agent
}

func NewRegistry() *Registry {
	return &Registry{agents: map[string]Agent{}}
}

func (r *Registry) Register(agent Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.agents[agent.Name()]; exists {
		return fmt.Errorf("agent %s already registered", agent.Name())
	}
	r.agents[agent.Name()] = agent
	return nil
}

func (r *Registry) Get(name string) (Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[name]
	return agent, ok
}

func (r *Registry) Match(t task.Task) []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Agent
	for _, agent := range r.agents {
		if agent.CanHandle(t) {
			out = append(out, agent)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out
}

type Assistant struct{}

func NewAssistant() *Assistant {
	return &Assistant{}
}

func (a *Assistant) Name() string {
	return "assistant"
}

func (a *Assistant) Capabilities() []string {
	return []string{"summary", "explain", "translate", "next_steps", "video_summary", "todo_scan"}
}

func (a *Assistant) CanHandle(_ task.Task) bool {
	return true
}

func (a *Assistant) Execute(_ context.Context, t task.Task, wm memory.WorkingMemory) (Result, error) {
	action, _ := t.Input["action"].(string)
	text, _ := t.Input["text"].(string)
	url, _ := t.Input["url"].(string)
	target, _ := t.Input["target_language"].(string)
	if text == "" && wm.RecentClipboard != nil {
		text = *wm.RecentClipboard
	}
	args := map[string]any{
		"action":          action,
		"text":            text,
		"url":             url,
		"target_language": target,
	}
	if action == "todo_scan" {
		args = map[string]any{
			"roots": t.Input["roots"],
		}
	}
	if action == "" {
		args["action"] = "explain"
	}
	return Result{
		Summary:  "assistant prepared tool call",
		ToolName: chooseTool(action),
		ToolArgs: args,
	}, nil
}

func chooseTool(action string) string {
	if action == "todo_scan" {
		return "todo_scan"
	}
	return "content"
}
