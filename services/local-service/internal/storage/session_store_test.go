package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestSessionStoresSupportCRUDAndPaging(t *testing.T) {
	inMemory := newInMemorySessionStore()
	if err := inMemory.WriteSession(context.Background(), SessionRecord{SessionID: "sess_mem_001", Title: "Session A", Status: "active", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:01:00Z"}); err != nil {
		t.Fatalf("in-memory WriteSession returned error: %v", err)
	}
	if err := inMemory.WriteSession(context.Background(), SessionRecord{SessionID: "sess_mem_002", Title: "Session B", Status: "idle", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:02:00Z"}); err != nil {
		t.Fatalf("in-memory WriteSession returned error: %v", err)
	}
	items, total, err := inMemory.ListSessions(context.Background(), 1, 0)
	if err != nil || total != 2 || len(items) != 1 || items[0].SessionID != "sess_mem_002" {
		t.Fatalf("in-memory ListSessions returned total=%d items=%+v err=%v", total, items, err)
	}
	if _, err := inMemory.GetSession(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing in-memory session to return sql.ErrNoRows, got %v", err)
	}
	if err := inMemory.DeleteSession(context.Background(), "sess_mem_001"); err != nil {
		t.Fatalf("in-memory DeleteSession returned error: %v", err)
	}

	sqliteStore, err := NewSQLiteSessionStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	if err := sqliteStore.WriteSession(context.Background(), SessionRecord{SessionID: "sess_sql_001", Title: "Session SQL", Status: "active", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:03:00Z"}); err != nil {
		t.Fatalf("sqlite WriteSession returned error: %v", err)
	}
	items, total, err = sqliteStore.ListSessions(context.Background(), 10, 0)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("sqlite ListSessions returned total=%d items=%+v err=%v", total, items, err)
	}
	record, err := sqliteStore.GetSession(context.Background(), "sess_sql_001")
	if err != nil || record.Title != "Session SQL" {
		t.Fatalf("sqlite GetSession returned record=%+v err=%v", record, err)
	}
	if err := sqliteStore.DeleteSession(context.Background(), "sess_sql_001"); err != nil {
		t.Fatalf("sqlite DeleteSession returned error: %v", err)
	}
	items, total, err = sqliteStore.ListSessions(context.Background(), 10, 0)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("sqlite ListSessions after delete returned total=%d items=%+v err=%v", total, items, err)
	}
	if paged := pageSessions([]SessionRecord{{SessionID: "one"}, {SessionID: "two"}}, 1, 1); len(paged) != 1 || paged[0].SessionID != "two" {
		t.Fatalf("pageSessions returned %+v", paged)
	}
	if paged := pageSessions([]SessionRecord{{SessionID: "one"}}, 10, 9); len(paged) != 0 {
		t.Fatalf("expected pageSessions overflow to return empty slice, got %+v", paged)
	}
}

func TestSessionStoreAdditionalPagingAndErrorBranches(t *testing.T) {
	if _, err := NewSQLiteSessionStore(""); err == nil {
		t.Fatal("expected sqlite session constructor to reject empty path")
	}

	inMemory := newInMemorySessionStore()
	if err := inMemory.WriteSession(context.Background(), SessionRecord{SessionID: "sess_mem_a", Title: "Session A", Status: "active", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:01:00Z"}); err != nil {
		t.Fatalf("in-memory WriteSession returned error: %v", err)
	}
	if err := inMemory.WriteSession(context.Background(), SessionRecord{SessionID: "sess_mem_b", Title: "Session B", Status: "idle", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:02:00Z"}); err != nil {
		t.Fatalf("in-memory WriteSession returned error: %v", err)
	}
	items, total, err := inMemory.ListSessions(context.Background(), 0, 1)
	if err != nil || total != 2 || len(items) != 1 || items[0].SessionID != "sess_mem_a" {
		t.Fatalf("expected unlimited in-memory paging from offset, total=%d items=%+v err=%v", total, items, err)
	}
	if paged := pageSessions([]SessionRecord{{SessionID: "one"}, {SessionID: "two"}}, 0, 1); len(paged) != 1 || paged[0].SessionID != "two" {
		t.Fatalf("expected unlimited pageSessions branch, got %+v", paged)
	}

	sqliteStore, err := NewSQLiteSessionStore(filepath.Join(t.TempDir(), "sessions-additional.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore returned error: %v", err)
	}
	if err := sqliteStore.WriteSession(context.Background(), SessionRecord{SessionID: "sess_sql_a", Title: "Session SQL A", Status: "active", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:03:00Z"}); err != nil {
		t.Fatalf("sqlite WriteSession returned error: %v", err)
	}
	if err := sqliteStore.WriteSession(context.Background(), SessionRecord{SessionID: "sess_sql_b", Title: "Session SQL B", Status: "idle", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:04:00Z"}); err != nil {
		t.Fatalf("sqlite WriteSession returned error: %v", err)
	}
	items, total, err = sqliteStore.ListSessions(context.Background(), 0, 0)
	if err != nil || total != 2 || len(items) != 2 || items[0].SessionID != "sess_sql_b" {
		t.Fatalf("expected sqlite ListSessions without limit to return all rows, total=%d items=%+v err=%v", total, items, err)
	}
	if _, err := sqliteStore.GetSession(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing sqlite session to return sql.ErrNoRows, got %v", err)
	}
	if err := sqliteStore.initialize(context.Background()); err != nil {
		t.Fatalf("expected repeated sqlite session initialize to succeed, got %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := sqliteStore.WriteSession(context.Background(), SessionRecord{SessionID: "sess_sql_c", Title: "Session SQL C", Status: "active", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:05:00Z"}); err == nil {
		t.Fatal("expected WriteSession on closed sqlite store to fail")
	}
	if err := sqliteStore.DeleteSession(context.Background(), "sess_sql_a"); err == nil {
		t.Fatal("expected DeleteSession on closed sqlite store to fail")
	}
	if _, err := sqliteStore.GetSession(context.Background(), "sess_sql_a"); err == nil {
		t.Fatal("expected GetSession on closed sqlite store to fail")
	}
	if _, _, err := sqliteStore.ListSessions(context.Background(), 10, 0); err == nil {
		t.Fatal("expected ListSessions on closed sqlite store to fail")
	}
	if err := sqliteStore.initialize(context.Background()); err == nil {
		t.Fatal("expected initialize on closed sqlite store to fail")
	}

	var nilStore SQLiteSessionStore
	if err := nilStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite session close to succeed, got %v", err)
	}
}
