package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSQLiteMemoryStoreInitializesWALMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	store, err := NewSQLiteMemoryStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteMemoryStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	mode, err := store.journalMode(context.Background())
	if err != nil {
		t.Fatalf("journalMode returned error: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected wal journal mode, got %q", mode)
	}
}

func TestSQLiteMemoryStoreSaveSearchAndListRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	store, err := NewSQLiteMemoryStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteMemoryStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	seed := []MemorySummaryRecord{
		{MemorySummaryID: "mem_001", TaskID: "task_old_001", RunID: "run_old_001", Summary: "user prefers markdown summary", CreatedAt: time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)},
		{MemorySummaryID: "mem_002", TaskID: "task_old_002", RunID: "run_old_002", Summary: "user likes concise bullets", CreatedAt: time.Date(2026, 4, 8, 10, 1, 0, 0, time.UTC).Format(time.RFC3339)},
		{MemorySummaryID: "mem_003", TaskID: "task_001", RunID: "run_001", Summary: "current task markdown summary", CreatedAt: time.Date(2026, 4, 8, 10, 2, 0, 0, time.UTC).Format(time.RFC3339)},
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
	if len(hits) != 1 || hits[0].MemoryID != "mem_001" {
		t.Fatalf("unexpected search hits: %+v", hits)
	}
	if hits[0].Source != sqliteMemorySource {
		t.Fatalf("unexpected search source: %+v", hits[0])
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

func TestNewServicePrefersSQLiteMemoryStoreWhenConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.db")
	service := NewService(stubAdapter{databasePath: path})

	store, ok := service.MemoryStore().(*SQLiteMemoryStore)
	if !ok {
		t.Fatalf("expected SQLiteMemoryStore, got %T", service.MemoryStore())
	}
	defer func() { _ = store.Close() }()

	mode, err := store.journalMode(context.Background())
	if err != nil {
		t.Fatalf("journalMode returned error: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected wal journal mode, got %q", mode)
	}
}

func TestSQLiteMemoryStoreRejectsInvalidSummaryRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.db")
	store, err := NewSQLiteMemoryStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteMemoryStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	err = store.SaveSummary(context.Background(), MemorySummaryRecord{
		MemorySummaryID: "mem_001",
		TaskID:          "task_001",
		RunID:           "run_001",
		Summary:         "summary",
		CreatedAt:       "not-rfc3339",
	})
	if !errors.Is(err, ErrMemoryCreatedAtInvalid) {
		t.Fatalf("expected ErrMemoryCreatedAtInvalid, got %v", err)
	}
}
