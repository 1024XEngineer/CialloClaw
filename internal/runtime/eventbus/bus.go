package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cialloclaw/internal/protocol"
)

type Bus interface {
	Publish(ctx context.Context, event protocol.Event) error
	Subscribe(sub Subscriber) error
	Unsubscribe(id string) error
	Use(mw Middleware)
}

type Subscriber interface {
	ID() string
	SubscribeTypes() []string
	Handle(ctx context.Context, event protocol.Event) error
}

type Middleware interface {
	ID() string
	Handle(ctx context.Context, event protocol.Event, next func(context.Context, protocol.Event) error) error
}

type SyncBus struct {
	mu          sync.RWMutex
	subscribers map[string]Subscriber
	order       []string
	middleware  []Middleware
}

func New() *SyncBus {
	return &SyncBus{
		subscribers: map[string]Subscriber{},
		order:       []string{},
		middleware:  []Middleware{},
	}
}

func (b *SyncBus) Publish(ctx context.Context, event protocol.Event) error {
	if event.ID == "" || event.Type == "" || event.SessionID == "" || event.Source == "" || event.TraceID == "" || event.SpanID == "" {
		return errors.New("event missing required fields")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	var firstErr error
	for _, subID := range b.order {
		sub := b.subscribers[subID]
		if !matches(sub.SubscribeTypes(), event.Type) {
			continue
		}
		handler := func(inner context.Context, innerEvent protocol.Event) error {
			defer func() {
				if r := recover(); r != nil {
					firstErr = fmt.Errorf("subscriber %s panicked: %v", sub.ID(), r)
				}
			}()
			return sub.Handle(inner, innerEvent)
		}
		for i := len(b.middleware) - 1; i >= 0; i-- {
			current := b.middleware[i]
			next := handler
			handler = func(inner context.Context, innerEvent protocol.Event) error {
				return current.Handle(inner, innerEvent, next)
			}
		}
		if err := handler(ctx, event); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("subscriber %s: %w", sub.ID(), err)
		}
	}
	return firstErr
}

func (b *SyncBus) Subscribe(sub Subscriber) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.subscribers[sub.ID()]; exists {
		return fmt.Errorf("subscriber %s already registered", sub.ID())
	}
	b.subscribers[sub.ID()] = sub
	b.order = append(b.order, sub.ID())
	return nil
}

func (b *SyncBus) Unsubscribe(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.subscribers[id]; !exists {
		return fmt.Errorf("subscriber %s not found", id)
	}
	delete(b.subscribers, id)
	for index, item := range b.order {
		if item == id {
			b.order = append(b.order[:index], b.order[index+1:]...)
			break
		}
	}
	return nil
}

func (b *SyncBus) Use(mw Middleware) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.middleware = append(b.middleware, mw)
}

func matches(types []string, eventType string) bool {
	for _, candidate := range types {
		if candidate == eventType {
			return true
		}
	}
	return false
}
