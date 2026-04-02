package session

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"cialloclaw/internal/storage/filedb"
)

type Status string

const (
	StatusActive  Status = "active"
	StatusPaused  Status = "paused"
	StatusClosing Status = "closing"
	StatusClosed  Status = "closed"
	StatusCrashed Status = "crashed"
)

type Session struct {
	ID                 string            `json:"id"`
	UserID             string            `json:"user_id"`
	DeviceID           string            `json:"device_id"`
	Status             Status            `json:"status"`
	StartedAt          time.Time         `json:"started_at"`
	LastActiveAt       time.Time         `json:"last_active_at"`
	EndedAt            *time.Time        `json:"ended_at,omitempty"`
	CurrentTaskID      *string           `json:"current_task_id,omitempty"`
	ActiveTaskIDs      []string          `json:"active_task_ids,omitempty"`
	PendingApprovalIDs []string          `json:"pending_approval_ids,omitempty"`
	WorkingMemoryID    string            `json:"working_memory_id"`
	ContextSnapshotID  *string           `json:"context_snapshot_id,omitempty"`
	TraceRootID        string            `json:"trace_root_id"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, s *Session) error
	GetByID(ctx context.Context, id string) (*Session, error)
	Update(ctx context.Context, s *Session) error
	ListActive(ctx context.Context) ([]*Session, error)
	Latest(ctx context.Context) (*Session, error)
}

type fileState struct {
	Items map[string]Session `json:"items"`
}

type FileRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileRepository(dataDir string) *FileRepository {
	return &FileRepository{path: filepath.Join(dataDir, "runtime", "sessions.json")}
}

func (r *FileRepository) Create(_ context.Context, s *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}
	if _, exists := state.Items[s.ID]; exists {
		return fmt.Errorf("session %s already exists", s.ID)
	}
	state.Items[s.ID] = *s
	return r.save(state)
}

func (r *FileRepository) GetByID(_ context.Context, id string) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}
	item, exists := state.Items[id]
	if !exists {
		return nil, nil
	}
	cp := item
	return &cp, nil
}

func (r *FileRepository) Update(_ context.Context, s *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}
	state.Items[s.ID] = *s
	return r.save(state)
}

func (r *FileRepository) ListActive(_ context.Context) ([]*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}
	var out []*Session
	for _, item := range state.Items {
		if item.Status == StatusActive || item.Status == StatusPaused {
			cp := item
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out, nil
}

func (r *FileRepository) Latest(_ context.Context) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}
	var latest *Session
	for _, item := range state.Items {
		cp := item
		if latest == nil || cp.StartedAt.After(latest.StartedAt) {
			latest = &cp
		}
	}
	return latest, nil
}

func (r *FileRepository) load() (fileState, error) {
	return filedb.ReadJSON(r.path, fileState{Items: map[string]Session{}})
}

func (r *FileRepository) save(state fileState) error {
	return filedb.WriteJSONAtomic(r.path, state)
}
