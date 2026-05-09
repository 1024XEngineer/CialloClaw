// Package storage contains process-local and SQLite-backed persistence stores.
package storage

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// defaultMemoryListLimit is the fallback result count for memory reads.
const defaultMemoryListLimit = 5

// InMemoryMemoryStore is the process-local memory persistence fallback.
type InMemoryMemoryStore struct {
	mu            sync.RWMutex
	summaries     []MemorySummaryRecord
	retrievalHits []MemoryRetrievalRecord
}

// NewInMemoryMemoryStore returns an empty in-memory store.
func NewInMemoryMemoryStore() *InMemoryMemoryStore {
	return &InMemoryMemoryStore{
		summaries:     make([]MemorySummaryRecord, 0),
		retrievalHits: make([]MemoryRetrievalRecord, 0),
	}
}

// SaveSummary appends one validated summary to the in-memory store.
func (s *InMemoryMemoryStore) SaveSummary(_ context.Context, summary MemorySummaryRecord) error {
	if err := validateMemorySummaryRecord(summary); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.summaries = append(s.summaries, summary)
	return nil
}

// SaveRetrievalHits appends validated retrieval-hit records.
func (s *InMemoryMemoryStore) SaveRetrievalHits(_ context.Context, hits []MemoryRetrievalRecord) error {
	if len(hits) == 0 {
		return nil
	}

	for _, hit := range hits {
		if err := validateMemoryRetrievalRecord(hit); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.retrievalHits = append(s.retrievalHits, hits...)
	return nil
}

// SearchSummaries returns ranked summaries from other task/run pairs.
func (s *InMemoryMemoryStore) SearchSummaries(_ context.Context, taskID, runID, query string, limit int) ([]MemoryRetrievalRecord, error) {
	limit = normalizeMemoryLimit(limit)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []MemoryRetrievalRecord{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	hits := make([]MemoryRetrievalRecord, 0, len(s.summaries))
	for _, summary := range s.summaries {
		if summary.TaskID == taskID && summary.RunID == runID {
			continue
		}

		score := matchMemorySummary(summary.Summary, query)
		if score <= 0 {
			continue
		}

		hits = append(hits, MemoryRetrievalRecord{
			RetrievalHitID: summary.MemorySummaryID,
			TaskID:         taskID,
			RunID:          runID,
			MemoryID:       summary.MemorySummaryID,
			Score:          score,
			Source:         "storage_in_memory",
			Summary:        summary.Summary,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].RetrievalHitID < hits[j].RetrievalHitID
		}

		return hits[i].Score > hits[j].Score
	})

	if len(hits) > limit {
		return hits[:limit], nil
	}

	return hits, nil
}

// ListRecentSummaries returns newest summaries first.
func (s *InMemoryMemoryStore) ListRecentSummaries(_ context.Context, limit int) ([]MemorySummaryRecord, error) {
	limit = normalizeMemoryLimit(limit)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.summaries) == 0 {
		return []MemorySummaryRecord{}, nil
	}

	result := make([]MemorySummaryRecord, 0, limit)
	for index := len(s.summaries) - 1; index >= 0 && len(result) < limit; index-- {
		result = append(result, s.summaries[index])
	}

	return result, nil
}

// normalizeMemoryLimit applies the default list limit for non-positive requests.
func normalizeMemoryLimit(limit int) int {
	if limit <= 0 {
		return defaultMemoryListLimit
	}

	return limit
}

// matchMemorySummary scores query-term overlap against one summary.
func matchMemorySummary(summary, query string) float64 {
	summary = strings.ToLower(strings.TrimSpace(summary))
	if summary == "" || query == "" {
		return 0
	}

	terms := strings.Fields(query)
	if len(terms) == 0 {
		return 0
	}

	matches := 0
	for _, term := range terms {
		if strings.Contains(summary, term) {
			matches++
		}
	}

	if matches == 0 {
		return 0
	}

	return float64(matches) / float64(len(terms))
}
