package memory

import (
	"context"
	"errors"
	"testing"
)

type stubStore struct {
	savedSummary MemorySummary
	savedHits    []RetrievalHit
	searchQuery  RetrievalQuery
	hits         []RetrievalHit
	summaries    []MemorySummary
	err          error
	saved        bool
	searched     bool
	listed       bool
}

func (s *stubStore) SaveSummary(_ context.Context, summary MemorySummary) error {
	s.saved = true
	s.savedSummary = summary
	return s.err
}

func (s *stubStore) Search(_ context.Context, query RetrievalQuery) ([]RetrievalHit, error) {
	s.searched = true
	s.searchQuery = query
	if s.err != nil {
		return nil, s.err
	}

	return s.hits, nil
}

func (s *stubStore) SaveRetrievalHits(_ context.Context, hits []RetrievalHit) error {
	s.savedHits = append([]RetrievalHit(nil), hits...)
	return s.err
}

func (s *stubStore) ListSummaries(_ context.Context, limit int) ([]MemorySummary, error) {
	s.listed = true
	if s.err != nil {
		return nil, s.err
	}
	if limit > 0 && len(s.summaries) > limit {
		return s.summaries[:limit], nil
	}

	return s.summaries, nil
}

func TestNewServiceWithoutStoreStillReportsBackend(t *testing.T) {
	service := NewService()

	if service.RetrievalBackend() != "sqlite_fts5+sqlite_vec" {
		t.Fatalf("backend mismatch: got %q", service.RetrievalBackend())
	}
}

func TestNewInMemoryServiceProvidesWorkingStore(t *testing.T) {
	service := NewInMemoryService()

	err := service.WriteSummary(context.Background(), MemorySummary{
		MemorySummaryID: "mem_001",
		TaskID:          "task_old_001",
		RunID:           "run_old_001",
		Summary:         "user prefers markdown summary",
	})
	if err != nil {
		t.Fatalf("WriteSummary returned error: %v", err)
	}

	hits, err := service.Search(context.Background(), RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "markdown summary",
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if len(hits) != 1 || hits[0].MemoryID != "mem_001" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestWriteSummaryRequiresStore(t *testing.T) {
	service := NewService()

	err := service.WriteSummary(context.Background(), MemorySummary{
		TaskID:  "task_001",
		RunID:   "run_001",
		Summary: "summary",
	})
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("expected ErrStoreNotConfigured, got %v", err)
	}
}

func TestWriteSummaryValidatesRequiredFields(t *testing.T) {
	service := NewService(&stubStore{})

	err := service.WriteSummary(context.Background(), MemorySummary{RunID: "run_001", Summary: "summary"})
	if !errors.Is(err, ErrTaskIDRequired) {
		t.Fatalf("expected ErrTaskIDRequired, got %v", err)
	}

	err = service.WriteSummary(context.Background(), MemorySummary{TaskID: "task_001", Summary: "summary"})
	if !errors.Is(err, ErrRunIDRequired) {
		t.Fatalf("expected ErrRunIDRequired, got %v", err)
	}

	err = service.WriteSummary(context.Background(), MemorySummary{TaskID: "task_001", RunID: "run_001"})
	if !errors.Is(err, ErrSummaryRequired) {
		t.Fatalf("expected ErrSummaryRequired, got %v", err)
	}
}

func TestWriteSummaryDelegatesToStore(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)
	summary := MemorySummary{
		MemorySummaryID: "memsum_001",
		TaskID:          "task_001",
		RunID:           "run_001",
		Summary:         "user prefers markdown output",
		CreatedAt:       "2026-04-08T10:00:00Z",
	}

	err := service.WriteSummary(context.Background(), summary)
	if err != nil {
		t.Fatalf("WriteSummary returned error: %v", err)
	}

	if !store.saved {
		t.Fatal("expected store SaveSummary to be called")
	}

	if store.savedSummary != summary {
		t.Fatalf("saved summary mismatch: got %+v want %+v", store.savedSummary, summary)
	}
}

func TestWriteRetrievalHitsRequiresStore(t *testing.T) {
	service := NewService()

	err := service.WriteRetrievalHits(context.Background(), []RetrievalHit{{TaskID: "task_001", RunID: "run_001", MemoryID: "mem_001"}})
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("expected ErrStoreNotConfigured, got %v", err)
	}
}

func TestWriteRetrievalHitsValidatesRequiredFields(t *testing.T) {
	service := NewService(&stubStore{})

	err := service.WriteRetrievalHits(context.Background(), []RetrievalHit{{RunID: "run_001", MemoryID: "mem_001"}})
	if !errors.Is(err, ErrTaskIDRequired) {
		t.Fatalf("expected ErrTaskIDRequired, got %v", err)
	}

	err = service.WriteRetrievalHits(context.Background(), []RetrievalHit{{TaskID: "task_001", MemoryID: "mem_001"}})
	if !errors.Is(err, ErrRunIDRequired) {
		t.Fatalf("expected ErrRunIDRequired, got %v", err)
	}

	err = service.WriteRetrievalHits(context.Background(), []RetrievalHit{{TaskID: "task_001", RunID: "run_001"}})
	if !errors.Is(err, ErrMemoryIDRequired) {
		t.Fatalf("expected ErrMemoryIDRequired, got %v", err)
	}
}

func TestWriteRetrievalHitsAssignsDefaultsAndDelegates(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	err := service.WriteRetrievalHits(context.Background(), []RetrievalHit{{
		TaskID:   "task_001",
		RunID:    "run_001",
		MemoryID: "mem_001",
		Score:    0.8,
		Summary:  "markdown summary",
	}})
	if err != nil {
		t.Fatalf("WriteRetrievalHits returned error: %v", err)
	}

	if len(store.savedHits) != 1 {
		t.Fatalf("expected one retrieval hit, got %+v", store.savedHits)
	}
	if store.savedHits[0].RetrievalHitID == "" {
		t.Fatalf("expected retrieval hit id to be generated, got %+v", store.savedHits[0])
	}
	if store.savedHits[0].Source != retrievalBackend {
		t.Fatalf("expected retrieval source %q, got %+v", retrievalBackend, store.savedHits[0])
	}
	if store.savedHits[0].CreatedAt == "" {
		t.Fatalf("expected retrieval hit created_at to be populated, got %+v", store.savedHits[0])
	}
}

func TestSearchRequiresStore(t *testing.T) {
	service := NewService()

	_, err := service.Search(context.Background(), RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "markdown",
	})
	if !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("expected ErrStoreNotConfigured, got %v", err)
	}
}

func TestSearchValidatesRequiredFields(t *testing.T) {
	service := NewService(&stubStore{})

	_, err := service.Search(context.Background(), RetrievalQuery{RunID: "run_001", Query: "markdown"})
	if !errors.Is(err, ErrTaskIDRequired) {
		t.Fatalf("expected ErrTaskIDRequired, got %v", err)
	}

	_, err = service.Search(context.Background(), RetrievalQuery{TaskID: "task_001", Query: "markdown"})
	if !errors.Is(err, ErrRunIDRequired) {
		t.Fatalf("expected ErrRunIDRequired, got %v", err)
	}

	_, err = service.Search(context.Background(), RetrievalQuery{TaskID: "task_001", RunID: "run_001"})
	if !errors.Is(err, ErrQueryRequired) {
		t.Fatalf("expected ErrQueryRequired, got %v", err)
	}
}

func TestSearchDelegatesToStore(t *testing.T) {
	store := &stubStore{
		hits: []RetrievalHit{
			{
				RetrievalHitID: "hit_001",
				TaskID:         "task_001",
				RunID:          "run_001",
				MemoryID:       "memsum_001",
				Score:          0.91,
				Source:         "rag_index",
				Summary:        "user prefers markdown output",
			},
		},
	}
	service := NewService(store)
	query := RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "markdown summary",
		Limit:  3,
	}

	hits, err := service.Search(context.Background(), query)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if !store.searched {
		t.Fatal("expected store Search to be called")
	}

	if store.searchQuery != query {
		t.Fatalf("search query mismatch: got %+v want %+v", store.searchQuery, query)
	}

	if len(hits) != 1 || hits[0].RetrievalHitID != "hit_001" {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestRetrievalQueryNormalizedAppliesDefaultLimit(t *testing.T) {
	query := RetrievalQuery{TaskID: "task_001", RunID: "run_001", Query: "markdown"}

	normalized := query.Normalized()
	if normalized.Limit != DefaultSearchLimit {
		t.Fatalf("expected default limit %d, got %d", DefaultSearchLimit, normalized.Limit)
	}
}

func TestRetrievalQueryNormalizedCapsLimit(t *testing.T) {
	query := RetrievalQuery{TaskID: "task_001", RunID: "run_001", Query: "markdown", Limit: MaxSearchLimit + 10}

	normalized := query.Normalized()
	if normalized.Limit != MaxSearchLimit {
		t.Fatalf("expected max limit %d, got %d", MaxSearchLimit, normalized.Limit)
	}
}

func TestSearchNormalizesLimitBeforeCallingStore(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	_, err := service.Search(context.Background(), RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "markdown",
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if store.searchQuery.Limit != DefaultSearchLimit {
		t.Fatalf("expected normalized default limit %d, got %d", DefaultSearchLimit, store.searchQuery.Limit)
	}
}

func TestSearchDeduplicatesHitsByMemoryIDAndSortsByScore(t *testing.T) {
	store := &stubStore{
		hits: []RetrievalHit{
			{RetrievalHitID: "hit_003", MemoryID: "mem_002", Score: 0.3, Summary: "low"},
			{RetrievalHitID: "hit_001", MemoryID: "mem_001", Score: 0.4, Summary: "first"},
			{RetrievalHitID: "hit_002", MemoryID: "mem_001", Score: 0.9, Summary: "better duplicate"},
			{RetrievalHitID: "hit_004", MemoryID: "mem_003", Score: 0.8, Summary: "second"},
		},
	}
	service := NewService(store)

	hits, err := service.Search(context.Background(), RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "markdown",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if len(hits) != 3 {
		t.Fatalf("expected 3 deduplicated hits, got %d", len(hits))
	}

	if hits[0].RetrievalHitID != "hit_002" || hits[1].RetrievalHitID != "hit_004" || hits[2].RetrievalHitID != "hit_003" {
		t.Fatalf("unexpected hit order: %+v", hits)
	}
}

func TestInMemoryStoreSearchMatchesTermsAndSkipsSameTaskRun(t *testing.T) {
	store := NewInMemoryStore()
	service := NewService(store)

	seed := []MemorySummary{
		{MemorySummaryID: "mem_001", TaskID: "task_old_001", RunID: "run_old_001", Summary: "user prefers markdown summary"},
		{MemorySummaryID: "mem_002", TaskID: "task_old_002", RunID: "run_old_002", Summary: "user likes concise bullets"},
		{MemorySummaryID: "mem_003", TaskID: "task_001", RunID: "run_001", Summary: "current task markdown summary"},
	}

	for _, summary := range seed {
		if err := service.WriteSummary(context.Background(), summary); err != nil {
			t.Fatalf("WriteSummary returned error: %v", err)
		}
	}

	hits, err := service.Search(context.Background(), RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "markdown summary",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d (%+v)", len(hits), hits)
	}

	if hits[0].MemoryID != "mem_001" {
		t.Fatalf("expected memory mem_001, got %+v", hits[0])
	}
}

func TestWriteSummaryAssignsDefaultIDWhenMissing(t *testing.T) {
	store := &stubStore{}
	service := NewService(store)

	err := service.WriteSummary(context.Background(), MemorySummary{
		TaskID:  "task_001",
		RunID:   "run_001",
		Summary: "generated id",
	})
	if err != nil {
		t.Fatalf("WriteSummary returned error: %v", err)
	}

	if store.savedSummary.MemorySummaryID != "memsum_task_001_run_001" {
		t.Fatalf("unexpected generated summary id: %+v", store.savedSummary)
	}
}

func TestSearchMirrorReferencesMapsHitsForRPCOutput(t *testing.T) {
	store := &stubStore{
		hits: []RetrievalHit{{
			RetrievalHitID: "hit_001",
			MemoryID:       "mem_001",
			Source:         retrievalBackend,
			Summary:        "user prefers markdown summary",
			Score:          0.9,
		}},
	}
	service := NewService(store)

	references, err := service.SearchMirrorReferences(context.Background(), RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "markdown summary",
	})
	if err != nil {
		t.Fatalf("SearchMirrorReferences returned error: %v", err)
	}

	if len(references) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(references))
	}

	if references[0]["memory_id"] != "mem_001" {
		t.Fatalf("unexpected reference payload: %+v", references[0])
	}
}

func TestSearchMirrorReferenceItemsReturnsTypedReferences(t *testing.T) {
	store := &stubStore{
		hits: []RetrievalHit{{
			RetrievalHitID: "hit_001",
			MemoryID:       "mem_001",
			Source:         retrievalBackend,
			Summary:        "typed reference",
			Score:          0.9,
		}},
	}
	service := NewService(store)

	references, err := service.SearchMirrorReferenceItems(context.Background(), RetrievalQuery{
		TaskID: "task_001",
		RunID:  "run_001",
		Query:  "typed reference",
	})
	if err != nil {
		t.Fatalf("SearchMirrorReferenceItems returned error: %v", err)
	}

	if len(references) != 1 || references[0].MemoryID != "mem_001" {
		t.Fatalf("unexpected references: %+v", references)
	}
}

func TestRecentReferencesMapsRecentSummariesForRPCOutput(t *testing.T) {
	store := &stubStore{
		summaries: []MemorySummary{{MemorySummaryID: "mem_001", Summary: "first"}, {MemorySummaryID: "mem_002", Summary: "second"}},
	}
	service := NewService(store)

	references, err := service.RecentReferences(context.Background(), 2)
	if err != nil {
		t.Fatalf("RecentReferences returned error: %v", err)
	}

	if !store.listed {
		t.Fatal("expected store ListSummaries to be called")
	}

	if len(references) != 2 || references[1]["memory_id"] != "mem_002" {
		t.Fatalf("unexpected references: %+v", references)
	}
}

func TestRecentReferenceItemsNormalizesLimitBeforeListing(t *testing.T) {
	store := &stubStore{
		summaries: []MemorySummary{{MemorySummaryID: "mem_001", Summary: "first"}},
	}
	service := NewService(store)

	references, err := service.RecentReferenceItems(context.Background(), 0)
	if err != nil {
		t.Fatalf("RecentReferenceItems returned error: %v", err)
	}

	if !store.listed {
		t.Fatal("expected store ListSummaries to be called")
	}

	if len(references) != 1 || references[0].MemoryID != "mem_001" {
		t.Fatalf("unexpected references: %+v", references)
	}
}

func TestMirrorReferenceMapReturnsProtocolShape(t *testing.T) {
	reference := MirrorReference{
		MemoryID: "mem_001",
		Reason:   "recent memory",
		Summary:  "markdown summary",
	}

	mapped := reference.Map()
	if mapped["memory_id"] != "mem_001" || mapped["reason"] != "recent memory" || mapped["summary"] != "markdown summary" {
		t.Fatalf("unexpected map: %+v", mapped)
	}
}
