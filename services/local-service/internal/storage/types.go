// 该文件负责存储层的数据接口或落盘实现。
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// CapabilitySnapshot 定义当前模块的数据结构。
type CapabilitySnapshot struct {
	Backend                string
	Configured             bool
	SupportsStructuredData bool
	SupportsMemoryStore    bool
	SupportsToolCallSink   bool
	SupportsRetrievalHits  bool
	SupportsFTS5           bool
	SupportsSQLiteVecStub  bool
	SupportsArtifactStore  bool
	SupportsSecretStore    bool
	MemoryStoreBackend     string
	ToolCallStoreBackend   string
	ArtifactStoreBackend   string
	SecretStoreBackend     string
	MemoryRetrievalBackend string
	FallbackActive         bool
}

var (
	// ErrSecretNotFound reports that the requested secret key does not exist.
	ErrSecretNotFound = errors.New("secret not found")
	// ErrSecretStoreAccessFailed reports that the stronghold-compatible secret store could not be accessed.
	ErrSecretStoreAccessFailed = errors.New("secret store access failed")
)

// MemorySummaryRecord 描述当前模块记录。
type MemorySummaryRecord struct {
	MemorySummaryID string
	TaskID          string
	RunID           string
	Summary         string
	CreatedAt       string
}

// MemoryRetrievalRecord 描述当前模块记录。
type MemoryRetrievalRecord struct {
	RetrievalHitID string
	TaskID         string
	RunID          string
	MemoryID       string
	Score          float64
	Source         string
	Summary        string
	CreatedAt      string
}

// MemoryStore 定义当前模块的接口约束。
type MemoryStore interface {
	SaveSummary(ctx context.Context, summary MemorySummaryRecord) error
	SaveRetrievalHits(ctx context.Context, hits []MemoryRetrievalRecord) error
	SearchSummaries(ctx context.Context, taskID, runID, query string, limit int) ([]MemoryRetrievalRecord, error)
	ListRecentSummaries(ctx context.Context, limit int) ([]MemorySummaryRecord, error)
}

// ArtifactRecord describes one persisted artifact snapshot.
type ArtifactRecord struct {
	ArtifactID          string
	TaskID              string
	ArtifactType        string
	Title               string
	Path                string
	MimeType            string
	DeliveryType        string
	DeliveryPayloadJSON string
	CreatedAt           string
}

// ArtifactStore defines artifact persistence and lookup behavior.
type ArtifactStore interface {
	SaveArtifacts(ctx context.Context, records []ArtifactRecord) error
	ListArtifacts(ctx context.Context, taskID string, limit, offset int) ([]ArtifactRecord, int, error)
}

// TodoItemRecord describes one persisted notes/todo snapshot.
type TodoItemRecord struct {
	ItemID               string
	Title                string
	Bucket               string
	Status               string
	SourcePath           string
	SourceLine           int
	DueAt                string
	TagsJSON             string
	AgentSuggestion      string
	NoteText             string
	Prerequisite         string
	PlannedAt            string
	EndedAt              string
	RelatedResourcesJSON string
	LinkedTaskID         string
	CreatedAt            string
	UpdatedAt            string
}

// RecurringRuleRecord describes one persisted recurring-rule snapshot.
type RecurringRuleRecord struct {
	RuleID               string
	ItemID               string
	RuleType             string
	CronExpr             string
	IntervalValue        int
	IntervalUnit         string
	ReminderStrategy     string
	Enabled              bool
	RepeatRuleText       string
	NextOccurrenceAt     string
	RecentInstanceStatus string
	EffectiveScope       string
	CreatedAt            string
	UpdatedAt            string
}

// TodoStore defines persistence for notes/todo items and recurring rules.
type TodoStore interface {
	ReplaceTodoState(ctx context.Context, items []TodoItemRecord, rules []RecurringRuleRecord) error
	LoadTodoState(ctx context.Context) ([]TodoItemRecord, []RecurringRuleRecord, error)
}

// SecretRecord captures one secret value persisted outside the normal settings path.
type SecretRecord struct {
	Namespace string
	Key       string
	Value     string
	UpdatedAt string
}

// SecretStore defines Stronghold-compatible secret storage behavior.
type SecretStore interface {
	PutSecret(ctx context.Context, record SecretRecord) error
	GetSecret(ctx context.Context, namespace, key string) (SecretRecord, error)
	DeleteSecret(ctx context.Context, namespace, key string) error
}

// TaskStepSnapshot 描述 task timeline 在存储层的快照格式。
type TaskStepSnapshot struct {
	StepID        string
	TaskID        string
	Name          string
	Status        string
	OrderIndex    int
	InputSummary  string
	OutputSummary string
}

// NotificationSnapshot 描述待发送通知在存储层的快照格式。
type NotificationSnapshot struct {
	Method    string
	Params    map[string]any
	CreatedAt time.Time
}

// TaskRunRecord 描述 task/run 主状态在存储层的完整快照。
type TaskRunRecord struct {
	TaskID            string
	SessionID         string
	RunID             string
	Title             string
	SourceType        string
	Status            string
	Intent            map[string]any
	PreferredDelivery string
	FallbackDelivery  string
	CurrentStep       string
	RiskLevel         string
	StartedAt         time.Time
	UpdatedAt         time.Time
	FinishedAt        *time.Time
	Timeline          []TaskStepSnapshot
	BubbleMessage     map[string]any
	DeliveryResult    map[string]any
	Artifacts         []map[string]any
	AuditRecords      []map[string]any
	MirrorReferences  []map[string]any
	Snapshot          contextsvc.TaskContextSnapshot
	SecuritySummary   map[string]any
	ApprovalRequest   map[string]any
	PendingExecution  map[string]any
	Authorization     map[string]any
	ImpactScope       map[string]any
	TokenUsage        map[string]any
	MemoryReadPlans   []map[string]any
	MemoryWritePlans  []map[string]any
	StorageWritePlan  map[string]any
	ArtifactPlans     []map[string]any
	Notifications     []NotificationSnapshot
	LatestEvent       map[string]any
	LatestToolCall    map[string]any
	CurrentStepStatus string
}

// TaskRunStore 定义 task/run 主状态的持久化契约。
type TaskRunStore interface {
	AllocateIdentifier(ctx context.Context, prefix string) (string, error)
	DeleteTaskRun(ctx context.Context, taskID string) error
	SaveTaskRun(ctx context.Context, record TaskRunRecord) error
	LoadTaskRuns(ctx context.Context) ([]TaskRunRecord, error)
}

// ToolCallStore 定义 tool_call 持久化契约。
type ToolCallStore interface {
	SaveToolCall(ctx context.Context, record tools.ToolCallRecord) error
}

// AuditStore 定义 audit 记录持久化契约。
type AuditStore interface {
	WriteAuditRecord(ctx context.Context, record audit.Record) error
	ListAuditRecords(ctx context.Context, taskID string, limit, offset int) ([]audit.Record, int, error)
}

// RecoveryPointStore 定义恢复点持久化契约。
type RecoveryPointStore interface {
	WriteRecoveryPoint(ctx context.Context, point checkpoint.RecoveryPoint) error
	ListRecoveryPoints(ctx context.Context, taskID string, limit, offset int) ([]checkpoint.RecoveryPoint, int, error)
	GetRecoveryPoint(ctx context.Context, recoveryPointID string) (checkpoint.RecoveryPoint, error)
}
