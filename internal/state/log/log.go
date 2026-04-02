package log

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"cialloclaw/internal/storage/filedb"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
	LevelAudit Level = "audit"
)

type Entry struct {
	ID           string         `json:"id"`
	Timestamp    time.Time      `json:"timestamp"`
	Level        Level          `json:"level"`
	SessionID    *string        `json:"session_id,omitempty"`
	TaskID       *string        `json:"task_id,omitempty"`
	TaskStepID   *string        `json:"task_step_id,omitempty"`
	LoopID       *string        `json:"loop_id,omitempty"`
	ApprovalID   *string        `json:"approval_id,omitempty"`
	EventID      *string        `json:"event_id,omitempty"`
	TraceID      string         `json:"trace_id"`
	SpanID       string         `json:"span_id"`
	ParentSpanID *string        `json:"parent_span_id,omitempty"`
	Category     string         `json:"category"`
	Message      string         `json:"message"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type Query struct {
	SessionID *string
	TaskID    *string
	Limit     int
}

type Repository interface {
	Append(ctx context.Context, entry *Entry) error
	Query(ctx context.Context, q Query) ([]*Entry, error)
}

type fileState struct {
	Items []Entry `json:"items"`
}

type FileRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileRepository(dataDir string) *FileRepository {
	return &FileRepository{path: filepath.Join(dataDir, "logs", "entries.json")}
}

func (r *FileRepository) Append(_ context.Context, entry *Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := filedb.ReadJSON(r.path, fileState{Items: []Entry{}})
	if err != nil {
		return err
	}
	state.Items = append(state.Items, *entry)
	return filedb.WriteJSONAtomic(r.path, state)
}

func (r *FileRepository) Query(_ context.Context, q Query) ([]*Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := filedb.ReadJSON(r.path, fileState{Items: []Entry{}})
	if err != nil {
		return nil, err
	}
	var out []*Entry
	for _, item := range state.Items {
		if q.SessionID != nil {
			if item.SessionID == nil || *item.SessionID != *q.SessionID {
				continue
			}
		}
		if q.TaskID != nil {
			if item.TaskID == nil || *item.TaskID != *q.TaskID {
				continue
			}
		}
		cp := item
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}
