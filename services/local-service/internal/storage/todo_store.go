package storage

import (
	"context"
	"sort"
	"sync"
)

// InMemoryTodoStore provides a process-local fallback for notes/todo state.
type InMemoryTodoStore struct {
	mu    sync.RWMutex
	items map[string]TodoItemRecord
	rules map[string]RecurringRuleRecord
}

// NewInMemoryTodoStore creates and returns an in-memory todo store.
func NewInMemoryTodoStore() *InMemoryTodoStore {
	return &InMemoryTodoStore{
		items: make(map[string]TodoItemRecord),
		rules: make(map[string]RecurringRuleRecord),
	}
}

// ReplaceTodoState atomically replaces the persisted todo and recurring state.
func (s *InMemoryTodoStore) ReplaceTodoState(_ context.Context, items []TodoItemRecord, rules []RecurringRuleRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = make(map[string]TodoItemRecord, len(items))
	for _, item := range items {
		s.items[item.ItemID] = item
	}
	s.rules = make(map[string]RecurringRuleRecord, len(rules))
	for _, rule := range rules {
		s.rules[rule.RuleID] = rule
	}
	return nil
}

// LoadTodoState returns the currently persisted todo and recurring snapshots.
func (s *InMemoryTodoStore) LoadTodoState(_ context.Context) ([]TodoItemRecord, []RecurringRuleRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]TodoItemRecord, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt == items[j].UpdatedAt {
			return items[i].ItemID > items[j].ItemID
		}
		return items[i].UpdatedAt > items[j].UpdatedAt
	})

	rules := make([]RecurringRuleRecord, 0, len(s.rules))
	for _, rule := range s.rules {
		rules = append(rules, rule)
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].UpdatedAt == rules[j].UpdatedAt {
			return rules[i].RuleID > rules[j].RuleID
		}
		return rules[i].UpdatedAt > rules[j].UpdatedAt
	})

	return items, rules, nil
}
