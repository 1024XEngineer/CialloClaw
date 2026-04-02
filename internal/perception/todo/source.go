package todo

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cialloclaw/internal/protocol"
	"cialloclaw/internal/runtime/eventbus"
	"cialloclaw/internal/runtime/id"
	"cialloclaw/internal/state/memory"
)

type Source struct {
	Bus        eventbus.Bus
	SessionID  func() string
	MemoryRepo memory.Repository
	Roots      []string
	Interval   time.Duration
	mu         sync.Mutex
	snapshot   map[string]time.Time
	stopCh     chan struct{}
	stopped    bool
}

func New(bus eventbus.Bus, sessionID func() string, memoryRepo memory.Repository, roots []string, interval time.Duration) *Source {
	return &Source{
		Bus:        bus,
		SessionID:  sessionID,
		MemoryRepo: memoryRepo,
		Roots:      roots,
		Interval:   interval,
		snapshot:   map[string]time.Time{},
		stopCh:     make(chan struct{}),
	}
}

func (s *Source) Name() string {
	return "perception.todo"
}

func (s *Source) Start(ctx context.Context) error {
	go s.loop(ctx)
	return nil
}

func (s *Source) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil
	}
	s.stopped = true
	close(s.stopCh)
	return nil
}

func (s *Source) loop(ctx context.Context) {
	s.scan(ctx)
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *Source) scan(ctx context.Context) {
	sessionID := s.SessionID()
	if sessionID == "" {
		return
	}
	changed, pending, completed := s.collect()
	wm, err := ensureWorkingMemory(ctx, s.MemoryRepo, sessionID)
	if err != nil {
		return
	}
	wm.TodoOverview = &memory.TodoOverview{
		Pending:     pending,
		Completed:   completed,
		LastScanAt:  time.Now(),
		ObservedDir: append([]string{}, s.Roots...),
	}
	wm.UpdatedAt = time.Now()
	if err := s.MemoryRepo.SaveWorkingSnapshot(ctx, wm); err != nil {
		return
	}
	for _, path := range changed {
		_ = s.Bus.Publish(ctx, protocol.NewEvent(protocol.EventTypeFileChanged, "perception.todo", sessionID, protocol.PriorityLow, protocol.FileChangedPayload{Path: path}))
	}
	_ = s.Bus.Publish(ctx, protocol.NewEvent(protocol.EventTypeTodoScanCompleted, "perception.todo", sessionID, protocol.PriorityLow, protocol.TodoScanCompletedPayload{
		PendingCount:   len(pending),
		CompletedCount: len(completed),
	}))
}

func (s *Source) collect() ([]string, []memory.TodoItem, []memory.TodoItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var changed []string
	var pending []memory.TodoItem
	var completed []memory.TodoItem

	nextSnapshot := map[string]time.Time{}
	for _, root := range s.Roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			info, statErr := os.Stat(path)
			if statErr != nil {
				return nil
			}
			nextSnapshot[path] = info.ModTime()
			if last, exists := s.snapshot[path]; !exists || !last.Equal(info.ModTime()) {
				changed = append(changed, path)
			}
			items, parseErr := parseFile(path, info.ModTime())
			if parseErr != nil {
				return nil
			}
			for _, item := range items {
				if item.Done {
					completed = append(completed, item)
				} else {
					pending = append(pending, item)
				}
			}
			return nil
		})
	}
	s.snapshot = nextSnapshot
	return changed, pending, completed
}

func parseFile(path string, modTime time.Time) ([]memory.TodoItem, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var out []memory.TodoItem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "- [ ] "):
			out = append(out, memory.TodoItem{
				ID:        id.New("todo"),
				FilePath:  path,
				Text:      strings.TrimPrefix(line, "- [ ] "),
				Done:      false,
				UpdatedAt: modTime,
			})
		case strings.HasPrefix(line, "- [x] "):
			out = append(out, memory.TodoItem{
				ID:        id.New("todo"),
				FilePath:  path,
				Text:      strings.TrimPrefix(line, "- [x] "),
				Done:      true,
				UpdatedAt: modTime,
			})
		}
	}
	return out, scanner.Err()
}

func ensureWorkingMemory(ctx context.Context, repo memory.Repository, sessionID string) (*memory.WorkingMemory, error) {
	wm, err := repo.GetWorkingSnapshot(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if wm != nil {
		return wm, nil
	}
	return &memory.WorkingMemory{
		ID:        id.New("wm"),
		SessionID: sessionID,
		UpdatedAt: time.Now(),
	}, nil
}
