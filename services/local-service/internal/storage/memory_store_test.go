package storage

import (
	"context"
	"testing"
)

func TestInMemoryMemoryStoreSearchReturnsRankedMatches(t *testing.T) {
	store := NewInMemoryMemoryStore()
	seed := []MemorySummaryRecord{
		{MemorySummaryID: "mem_001", TaskID: "task_old_001", RunID: "run_old_001", Summary: "user prefers markdown summary", CreatedAt: "2026-04-08T10:00:00Z"},
		{MemorySummaryID: "mem_002", TaskID: "task_old_002", RunID: "run_old_002", Summary: "user likes markdown", CreatedAt: "2026-04-08T10:01:00Z"},
		{MemorySummaryID: "mem_003", TaskID: "task_001", RunID: "run_001", Summary: "current task markdown summary", CreatedAt: "2026-04-08T10:02:00Z"},
	}

	for _, summary := range seed {
		if err := store.SaveSummary(context.Background(), summary); err != nil {
			t.Fatalf("SaveSummary returned error: %v", err)
		}
	}

	hits, err := store.SearchSummaries(context.Background(), "task_001", "run_001", "markdown summary", 5)
	if err != nil {
		t.Fatalf("SearchSummaries returned error: %v", err)
	}

	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d (%+v)", len(hits), hits)
	}
	if hits[0].MemoryID != "mem_001" || hits[1].MemoryID != "mem_002" {
		t.Fatalf("unexpected hit order: %+v", hits)
	}
}

func TestInMemoryMemoryStoreListRecentSummariesReturnsLatestFirst(t *testing.T) {
	store := NewInMemoryMemoryStore()
	seed := []MemorySummaryRecord{
		{MemorySummaryID: "mem_001", TaskID: "task_001", RunID: "run_001", Summary: "first", CreatedAt: "2026-04-08T10:00:00Z"},
		{MemorySummaryID: "mem_002", TaskID: "task_002", RunID: "run_002", Summary: "second", CreatedAt: "2026-04-08T10:01:00Z"},
		{MemorySummaryID: "mem_003", TaskID: "task_003", RunID: "run_003", Summary: "third", CreatedAt: "2026-04-08T10:02:00Z"},
	}

	for _, summary := range seed {
		if err := store.SaveSummary(context.Background(), summary); err != nil {
			t.Fatalf("SaveSummary returned error: %v", err)
		}
	}

	recent, err := store.ListRecentSummaries(context.Background(), 2)
	if err != nil {
		t.Fatalf("ListRecentSummaries returned error: %v", err)
	}

	if len(recent) != 2 {
		t.Fatalf("expected 2 recent summaries, got %d", len(recent))
	}
	if recent[0].MemorySummaryID != "mem_003" || recent[1].MemorySummaryID != "mem_002" {
		t.Fatalf("unexpected recent summaries: %+v", recent)
	}
}

func TestInMemoryMemoryStoreUsesDefaultLimitWhenNonPositive(t *testing.T) {
	store := NewInMemoryMemoryStore()
	for i := 0; i < 6; i++ {
		if err := store.SaveSummary(context.Background(), MemorySummaryRecord{
			MemorySummaryID: string(rune('a' + i)),
			TaskID:          "task_default",
			RunID:           "run_default",
			Summary:         "summary",
			CreatedAt:       "2026-04-08T10:00:00Z",
		}); err != nil {
			t.Fatalf("SaveSummary returned error: %v", err)
		}
	}

	recent, err := store.ListRecentSummaries(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListRecentSummaries returned error: %v", err)
	}

	if len(recent) != defaultMemoryListLimit {
		t.Fatalf("expected default limit %d, got %d", defaultMemoryListLimit, len(recent))
	}
}

func TestInMemoryMemoryStoreSaveRetrievalHitsStoresRecords(t *testing.T) {
	store := NewInMemoryMemoryStore()

	err := store.SaveRetrievalHits(context.Background(), []MemoryRetrievalRecord{{
		RetrievalHitID: "hit_001",
		TaskID:         "task_001",
		RunID:          "run_001",
		MemoryID:       "mem_001",
		Score:          0.9,
		Source:         memoryRetrievalBackendInMemory,
		Summary:        "memory hit",
		CreatedAt:      "2026-04-08T10:00:00Z",
	}})
	if err != nil {
		t.Fatalf("SaveRetrievalHits returned error: %v", err)
	}

	if len(store.retrievalHits) != 1 || store.retrievalHits[0].RetrievalHitID != "hit_001" {
		t.Fatalf("unexpected retrieval hits: %+v", store.retrievalHits)
	}
}
