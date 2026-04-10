package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
)

type InMemoryStore struct {
	mu            sync.RWMutex
	summaries     []MemorySummary
	retrievalHits []RetrievalHit
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		summaries:     make([]MemorySummary, 0),
		retrievalHits: make([]RetrievalHit, 0),
	}
}

func (s *InMemoryStore) SaveSummary(_ context.Context, summary MemorySummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.summaries = append(s.summaries, summary)
	return nil
}

func (s *InMemoryStore) SaveRetrievalHits(_ context.Context, hits []RetrievalHit) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.retrievalHits = append(s.retrievalHits, hits...)
	return nil
}

func (s *InMemoryStore) Search(_ context.Context, query RetrievalQuery) ([]RetrievalHit, error) {
	query = query.Normalized()
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryText := strings.ToLower(strings.TrimSpace(query.Query))
	hits := make([]RetrievalHit, 0, len(s.summaries))

	for _, summary := range s.summaries {
		if summary.TaskID == query.TaskID && summary.RunID == query.RunID {
			continue
		}

		score := matchScore(summary.Summary, queryText)
		if score <= 0 {
			continue
		}

		hits = append(hits, RetrievalHit{
			RetrievalHitID: summary.MemorySummaryID,
			TaskID:         query.TaskID,
			RunID:          query.RunID,
			MemoryID:       summary.MemorySummaryID,
			Score:          score,
			Source:         retrievalBackend,
			Summary:        summary.Summary,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].RetrievalHitID < hits[j].RetrievalHitID
		}

		return hits[i].Score > hits[j].Score
	})

	if len(hits) > query.Limit {
		return hits[:query.Limit], nil
	}

	return hits, nil
}

func (s *InMemoryStore) ListSummaries(_ context.Context, limit int) ([]MemorySummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	if len(s.summaries) == 0 {
		return []MemorySummary{}, nil
	}

	result := make([]MemorySummary, 0, limit)
	for index := len(s.summaries) - 1; index >= 0 && len(result) < limit; index-- {
		result = append(result, s.summaries[index])
	}

	return result, nil
}

func matchScore(summary, query string) float64 {
	summaryLower := strings.ToLower(strings.TrimSpace(summary))
	if summaryLower == "" || query == "" {
		return 0
	}

	terms := strings.Fields(query)
	if len(terms) == 0 {
		return 0
	}

	matches := 0
	for _, term := range terms {
		if strings.Contains(summaryLower, term) {
			matches++
		}
	}

	if matches == 0 {
		return 0
	}

	return float64(matches) / float64(len(terms))
}
