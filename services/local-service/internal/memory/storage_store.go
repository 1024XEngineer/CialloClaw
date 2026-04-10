package memory

import (
	"context"

	storagesvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

type storageStore struct {
	store storagesvc.MemoryStore
}

func newStorageStore(store storagesvc.MemoryStore) Store {
	if store == nil {
		return nil
	}

	return &storageStore{store: store}
}

func (s *storageStore) SaveSummary(ctx context.Context, summary MemorySummary) error {
	return s.store.SaveSummary(ctx, storagesvc.MemorySummaryRecord{
		MemorySummaryID: summary.MemorySummaryID,
		TaskID:          summary.TaskID,
		RunID:           summary.RunID,
		Summary:         summary.Summary,
		CreatedAt:       summary.CreatedAt,
	})
}

func (s *storageStore) SaveRetrievalHits(ctx context.Context, hits []RetrievalHit) error {
	records := make([]storagesvc.MemoryRetrievalRecord, 0, len(hits))
	for _, hit := range hits {
		records = append(records, storagesvc.MemoryRetrievalRecord{
			RetrievalHitID: hit.RetrievalHitID,
			TaskID:         hit.TaskID,
			RunID:          hit.RunID,
			MemoryID:       hit.MemoryID,
			Score:          hit.Score,
			Source:         hit.Source,
			Summary:        hit.Summary,
			CreatedAt:      hit.CreatedAt,
		})
	}

	return s.store.SaveRetrievalHits(ctx, records)
}

func (s *storageStore) Search(ctx context.Context, query RetrievalQuery) ([]RetrievalHit, error) {
	records, err := s.store.SearchSummaries(ctx, query.TaskID, query.RunID, query.Query, query.Limit)
	if err != nil {
		return nil, err
	}

	hits := make([]RetrievalHit, 0, len(records))
	for _, record := range records {
		hits = append(hits, RetrievalHit{
			RetrievalHitID: record.RetrievalHitID,
			TaskID:         record.TaskID,
			RunID:          record.RunID,
			MemoryID:       record.MemoryID,
			Score:          record.Score,
			Source:         record.Source,
			Summary:        record.Summary,
			CreatedAt:      record.CreatedAt,
		})
	}

	return hits, nil
}

func (s *storageStore) ListSummaries(ctx context.Context, limit int) ([]MemorySummary, error) {
	records, err := s.store.ListRecentSummaries(ctx, limit)
	if err != nil {
		return nil, err
	}

	summaries := make([]MemorySummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, MemorySummary{
			MemorySummaryID: record.MemorySummaryID,
			TaskID:          record.TaskID,
			RunID:           record.RunID,
			Summary:         record.Summary,
			CreatedAt:       record.CreatedAt,
		})
	}

	return summaries, nil
}
