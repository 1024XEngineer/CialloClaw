package memory

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"cialloclaw/internal/storage/filedb"
)

type Suggestion struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Action      string            `json:"action"`
	Payload     map[string]string `json:"payload,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type TodoItem struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TodoOverview struct {
	Pending     []TodoItem `json:"pending,omitempty"`
	Completed   []TodoItem `json:"completed,omitempty"`
	LastScanAt  time.Time  `json:"last_scan_at"`
	ObservedDir []string   `json:"observed_dir,omitempty"`
}

type WorkingMemory struct {
	ID                 string        `json:"id"`
	SessionID          string        `json:"session_id"`
	CurrentIntent      *string       `json:"current_intent,omitempty"`
	CurrentFocus       *string       `json:"current_focus,omitempty"`
	SelectedContent    *string       `json:"selected_content,omitempty"`
	RecentEventIDs     []string      `json:"recent_event_ids,omitempty"`
	ActiveTaskIDs      []string      `json:"active_task_ids,omitempty"`
	PendingApprovalIDs []string      `json:"pending_approval_ids,omitempty"`
	ContextSummary     string        `json:"context_summary"`
	RecentClipboard    *string       `json:"recent_clipboard,omitempty"`
	ClipboardKind      string        `json:"clipboard_kind,omitempty"`
	Suggestions        []Suggestion  `json:"suggestions,omitempty"`
	TodoOverview       *TodoOverview `json:"todo_overview,omitempty"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

type EpisodicMemory struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	TaskID       *string   `json:"task_id,omitempty"`
	Summary      string    `json:"summary"`
	EventRefs    []string  `json:"event_refs,omitempty"`
	ArtifactRefs []string  `json:"artifact_refs,omitempty"`
	Importance   float64   `json:"importance"`
	Tags         []string  `json:"tags,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Profile struct {
	ID          string    `json:"id"`
	Preferences []string  `json:"preferences,omitempty"`
	Facts       []string  `json:"facts,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Repository interface {
	SaveWorkingSnapshot(ctx context.Context, wm *WorkingMemory) error
	GetWorkingSnapshot(ctx context.Context, sessionID string) (*WorkingMemory, error)
	StoreEpisode(ctx context.Context, m *EpisodicMemory) error
	ListEpisodes(ctx context.Context) ([]*EpisodicMemory, error)
	SaveProfile(ctx context.Context, profile *Profile) error
	GetProfile(ctx context.Context) (*Profile, error)
}

type workingState struct {
	Items map[string]WorkingMemory `json:"items"`
}

type episodicState struct {
	Items map[string]EpisodicMemory `json:"items"`
}

type FileRepository struct {
	workingPath string
	episodePath string
	profilePath string
	mu          sync.Mutex
}

func NewFileRepository(dataDir string) *FileRepository {
	base := filepath.Join(dataDir, "runtime")
	return &FileRepository{
		workingPath: filepath.Join(base, "working_memory.json"),
		episodePath: filepath.Join(base, "episodic_memory.json"),
		profilePath: filepath.Join(base, "profile.json"),
	}
}

func (r *FileRepository) SaveWorkingSnapshot(_ context.Context, wm *WorkingMemory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := filedb.ReadJSON(r.workingPath, workingState{Items: map[string]WorkingMemory{}})
	if err != nil {
		return err
	}
	state.Items[wm.SessionID] = *wm
	return filedb.WriteJSONAtomic(r.workingPath, state)
}

func (r *FileRepository) GetWorkingSnapshot(_ context.Context, sessionID string) (*WorkingMemory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := filedb.ReadJSON(r.workingPath, workingState{Items: map[string]WorkingMemory{}})
	if err != nil {
		return nil, err
	}
	item, exists := state.Items[sessionID]
	if !exists {
		return nil, nil
	}
	cp := item
	return &cp, nil
}

func (r *FileRepository) StoreEpisode(_ context.Context, m *EpisodicMemory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := filedb.ReadJSON(r.episodePath, episodicState{Items: map[string]EpisodicMemory{}})
	if err != nil {
		return err
	}
	state.Items[m.ID] = *m
	return filedb.WriteJSONAtomic(r.episodePath, state)
}

func (r *FileRepository) ListEpisodes(_ context.Context) ([]*EpisodicMemory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := filedb.ReadJSON(r.episodePath, episodicState{Items: map[string]EpisodicMemory{}})
	if err != nil {
		return nil, err
	}
	var out []*EpisodicMemory
	for _, item := range state.Items {
		cp := item
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (r *FileRepository) SaveProfile(_ context.Context, profile *Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return filedb.WriteJSONAtomic(r.profilePath, profile)
}

func (r *FileRepository) GetProfile(_ context.Context) (*Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	profile, err := filedb.ReadJSON(r.profilePath, Profile{})
	if err != nil {
		return nil, err
	}
	if profile.ID == "" {
		return nil, nil
	}
	return &profile, nil
}
