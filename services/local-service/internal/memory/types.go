package memory

import "context"

const DefaultSearchLimit = 5
const MaxSearchLimit = 20

type MirrorReference struct {
	MemoryID string
	Reason   string
	Summary  string
}

func (r MirrorReference) Map() map[string]any {
	return map[string]any{
		"memory_id": r.MemoryID,
		"reason":    r.Reason,
		"summary":   r.Summary,
	}
}

type MemorySummary struct {
	MemorySummaryID string
	TaskID          string
	RunID           string
	Summary         string
	CreatedAt       string
}

type MemoryCandidate struct {
	MemoryCandidateID string
	TaskID            string
	RunID             string
	Summary           string
	Source            string
}

type RetrievalHit struct {
	RetrievalHitID string
	TaskID         string
	RunID          string
	MemoryID       string
	Score          float64
	Source         string
	Summary        string
	CreatedAt      string
}

type RetrievalQuery struct {
	TaskID string
	RunID  string
	Query  string
	Limit  int
}

func (q RetrievalQuery) Normalized() RetrievalQuery {
	normalized := q
	if normalized.Limit <= 0 {
		normalized.Limit = DefaultSearchLimit
	}
	if normalized.Limit > MaxSearchLimit {
		normalized.Limit = MaxSearchLimit
	}

	return normalized
}

type Store interface {
	SaveSummary(ctx context.Context, summary MemorySummary) error
	SaveRetrievalHits(ctx context.Context, hits []RetrievalHit) error
	Search(ctx context.Context, query RetrievalQuery) ([]RetrievalHit, error)
	ListSummaries(ctx context.Context, limit int) ([]MemorySummary, error)
}
