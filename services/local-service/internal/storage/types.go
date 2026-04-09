// 该文件负责存储层的数据接口或落盘实现。
package storage

import "context"

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
