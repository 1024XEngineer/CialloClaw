package task

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
type Kind string
type StepStatus string

const (
	StatusQueued    Status = "queued"
	StatusPlanned   Status = "planned"
	StatusRunning   Status = "running"
	StatusWaiting   Status = "waiting"
	StatusBlocked   Status = "blocked"
	StatusPaused    Status = "paused"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

const (
	KindQuestionAnswer Kind = "question_answer"
	KindResearch       Kind = "research"
	KindFileOperation  Kind = "file_operation"
	KindWriting        Kind = "writing"
	KindCoding         Kind = "coding"
	KindWorkflow       Kind = "workflow"
)

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepWaiting StepStatus = "waiting"
	StepDone    StepStatus = "done"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
)

type TaskStep struct {
	ID          string         `json:"id"`
	Index       int            `json:"index"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      StepStatus     `json:"status"`
	AgentName   *string        `json:"agent_name,omitempty"`
	ToolName    *string        `json:"tool_name,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       *string        `json:"error,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	EndedAt     *time.Time     `json:"ended_at,omitempty"`
}

type Task struct {
	ID            string            `json:"id"`
	SessionID     string            `json:"session_id"`
	ParentTaskID  *string           `json:"parent_task_id,omitempty"`
	Kind          Kind              `json:"kind"`
	Title         string            `json:"title"`
	Goal          string            `json:"goal"`
	Status        Status            `json:"status"`
	Priority      int               `json:"priority"`
	Planner       string            `json:"planner"`
	AssigneeAgent *string           `json:"assignee_agent,omitempty"`
	Input         map[string]any    `json:"input,omitempty"`
	Output        map[string]any    `json:"output,omitempty"`
	Error         *string           `json:"error,omitempty"`
	PlanID        *string           `json:"plan_id,omitempty"`
	LoopID        *string           `json:"loop_id,omitempty"`
	RetryCount    int               `json:"retry_count"`
	MaxRetry      int               `json:"max_retry"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	TraceID       string            `json:"trace_id"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Steps         []TaskStep        `json:"steps,omitempty"`
	Summary       string            `json:"summary,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, t *Task) error
	GetByID(ctx context.Context, id string) (*Task, error)
	ListBySession(ctx context.Context, sessionID string) ([]*Task, error)
	ListAll(ctx context.Context) ([]*Task, error)
	Update(ctx context.Context, t *Task) error
}

type fileState struct {
	Items map[string]Task `json:"items"`
}

type FileRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileRepository(dataDir string) *FileRepository {
	return &FileRepository{path: filepath.Join(dataDir, "runtime", "tasks.json")}
}

func (r *FileRepository) Create(_ context.Context, t *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}
	if _, exists := state.Items[t.ID]; exists {
		return fmt.Errorf("task %s already exists", t.ID)
	}
	state.Items[t.ID] = *t
	return r.save(state)
}

func (r *FileRepository) GetByID(_ context.Context, id string) (*Task, error) {
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

func (r *FileRepository) ListBySession(_ context.Context, sessionID string) ([]*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}
	var out []*Task
	for _, item := range state.Items {
		if item.SessionID != sessionID {
			continue
		}
		cp := item
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (r *FileRepository) ListAll(_ context.Context) ([]*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return nil, err
	}
	var out []*Task
	for _, item := range state.Items {
		cp := item
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (r *FileRepository) Update(_ context.Context, t *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.load()
	if err != nil {
		return err
	}
	state.Items[t.ID] = *t
	return r.save(state)
}

func (r *FileRepository) load() (fileState, error) {
	return filedb.ReadJSON(r.path, fileState{Items: map[string]Task{}})
}

func (r *FileRepository) save(state fileState) error {
	return filedb.WriteJSONAtomic(r.path, state)
}
