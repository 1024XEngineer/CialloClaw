package approval

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
type Risk string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusExpired  Status = "expired"
	StatusCanceled Status = "canceled"
)

const (
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

type Approval struct {
	ID            string         `json:"id"`
	SessionID     string         `json:"session_id"`
	TaskID        *string        `json:"task_id,omitempty"`
	TaskStepID    *string        `json:"task_step_id,omitempty"`
	LoopID        *string        `json:"loop_id,omitempty"`
	Status        Status         `json:"status"`
	Risk          Risk           `json:"risk"`
	ActionType    string         `json:"action_type"`
	ActionSummary string         `json:"action_summary"`
	ProposedArgs  map[string]any `json:"proposed_args,omitempty"`
	ResourceRefs  []string       `json:"resource_refs,omitempty"`
	RequestedBy   string         `json:"requested_by"`
	RequestedAt   time.Time      `json:"requested_at"`
	DecidedBy     *string        `json:"decided_by,omitempty"`
	DecidedAt     *time.Time     `json:"decided_at,omitempty"`
	TimeoutAt     *time.Time     `json:"timeout_at,omitempty"`
	Reason        *string        `json:"reason,omitempty"`
	ResumeToken   *string        `json:"resume_token,omitempty"`
	TraceID       string         `json:"trace_id"`
}

type Repository interface {
	Create(ctx context.Context, a *Approval) error
	GetByID(ctx context.Context, id string) (*Approval, error)
	ListPendingBySession(ctx context.Context, sessionID string) ([]*Approval, error)
	ListAll(ctx context.Context) ([]*Approval, error)
	Update(ctx context.Context, a *Approval) error
}

type fileState struct {
	Items map[string]Approval `json:"items"`
}

type FileRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileRepository(dataDir string) *FileRepository {
	return &FileRepository{path: filepath.Join(dataDir, "runtime", "approvals.json")}
}

func (r *FileRepository) Create(_ context.Context, a *Approval) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}
	if _, exists := state.Items[a.ID]; exists {
		return fmt.Errorf("approval %s already exists", a.ID)
	}
	state.Items[a.ID] = *a
	return r.save(state)
}

func (r *FileRepository) GetByID(_ context.Context, id string) (*Approval, error) {
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

func (r *FileRepository) ListPendingBySession(_ context.Context, sessionID string) ([]*Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}
	var out []*Approval
	for _, item := range state.Items {
		if item.SessionID != sessionID || item.Status != StatusPending {
			continue
		}
		cp := item
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RequestedAt.After(out[j].RequestedAt)
	})
	return out, nil
}

func (r *FileRepository) ListAll(_ context.Context) ([]*Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}
	var out []*Approval
	for _, item := range state.Items {
		cp := item
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RequestedAt.After(out[j].RequestedAt)
	})
	return out, nil
}

func (r *FileRepository) Update(_ context.Context, a *Approval) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}
	state.Items[a.ID] = *a
	return r.save(state)
}

func (r *FileRepository) load() (fileState, error) {
	return filedb.ReadJSON(r.path, fileState{Items: map[string]Approval{}})
}

func (r *FileRepository) save(state fileState) error {
	return filedb.WriteJSONAtomic(r.path, state)
}
