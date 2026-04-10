// 该文件负责存储层的数据接口或落盘实现。
package storage

import (
	"context"
	"time"
)

// CapabilitySnapshot 定义当前模块的数据结构。
type CapabilitySnapshot struct {
	Backend                string
	Configured             bool
	SupportsStructuredData bool
	SupportsMemoryStore    bool
	SupportsRetrievalHits  bool
	SupportsFTS5           bool
	SupportsSQLiteVecStub  bool
	SupportsArtifactStore  bool
	SupportsSecretStore    bool
	MemoryStoreBackend     string
	MemoryRetrievalBackend string
	FallbackActive         bool
}

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
	MirrorReferences  []map[string]any
	SecuritySummary   map[string]any
	ApprovalRequest   map[string]any
	PendingExecution  map[string]any
	Authorization     map[string]any
	ImpactScope       map[string]any
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
	SaveTaskRun(ctx context.Context, record TaskRunRecord) error
	LoadTaskRuns(ctx context.Context) ([]TaskRunRecord, error)
}
