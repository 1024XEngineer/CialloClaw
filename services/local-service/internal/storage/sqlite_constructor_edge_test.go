package storage

import "testing"

func TestSQLiteStoreConstructorsAndNilCloseEdgeCases(t *testing.T) {
	if _, err := NewSQLiteSessionStore(""); err == nil {
		t.Fatal("expected sqlite session constructor to reject empty path")
	}
	if _, err := NewSQLiteSettingsStore(""); err == nil {
		t.Fatal("expected sqlite settings constructor to reject empty path")
	}
	if _, err := NewSQLiteToolCallStore(""); err == nil {
		t.Fatal("expected sqlite tool call constructor to reject empty path")
	}
	if _, err := NewSQLiteLoopRuntimeStore(""); err == nil {
		t.Fatal("expected sqlite loop runtime constructor to reject empty path")
	}

	var nilSessionStore SQLiteSessionStore
	if err := nilSessionStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite session close to succeed, got %v", err)
	}
	var nilSettingsStore SQLiteSettingsStore
	if err := nilSettingsStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite settings close to succeed, got %v", err)
	}
	var nilToolCallStore SQLiteToolCallStore
	if err := nilToolCallStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite tool call close to succeed, got %v", err)
	}
	var nilLoopRuntimeStore SQLiteLoopRuntimeStore
	if err := nilLoopRuntimeStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite loop runtime close to succeed, got %v", err)
	}
}
