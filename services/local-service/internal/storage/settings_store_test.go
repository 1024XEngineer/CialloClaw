package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestSettingsStoresPersistSnapshotsAndCloneNestedValues(t *testing.T) {
	snapshot := map[string]any{
		"general": map[string]any{"language": "zh-CN"},
		"models": map[string]any{
			"provider": "openai",
			"credentials": map[string]any{
				"budget_auto_downgrade": true,
				"allowed":               []string{"fast", "safe"},
			},
		},
		"artifacts": []map[string]any{{"artifact_id": "art_001"}},
	}

	inMemory := newInMemorySettingsStore()
	if err := inMemory.SaveSettingsSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("in-memory SaveSettingsSnapshot returned error: %v", err)
	}
	loaded, err := inMemory.LoadSettingsSnapshot(context.Background())
	if err != nil {
		t.Fatalf("in-memory LoadSettingsSnapshot returned error: %v", err)
	}
	loaded["general"].(map[string]any)["language"] = "en-US"
	loadedAgain, err := inMemory.LoadSettingsSnapshot(context.Background())
	if err != nil {
		t.Fatalf("in-memory LoadSettingsSnapshot second read returned error: %v", err)
	}
	if loadedAgain["general"].(map[string]any)["language"] != "zh-CN" {
		t.Fatalf("expected cloneSettingsMap to isolate mutations, got %+v", loadedAgain)
	}

	sqliteStore, err := NewSQLiteSettingsStore(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSettingsStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	if err := sqliteStore.SaveSettingsSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("sqlite SaveSettingsSnapshot returned error: %v", err)
	}
	loaded, err = sqliteStore.LoadSettingsSnapshot(context.Background())
	if err != nil {
		t.Fatalf("sqlite LoadSettingsSnapshot returned error: %v", err)
	}
	if loaded["models"].(map[string]any)["provider"] != "openai" {
		t.Fatalf("unexpected sqlite settings snapshot: %+v", loaded)
	}
	if cloned := cloneSettingsMap(snapshot); len(cloned["artifacts"].([]map[string]any)) != 1 {
		t.Fatalf("cloneSettingsMap returned unexpected clone: %+v", cloned)
	}
}

func TestSQLiteSettingsStoreLoadReturnsNilWhenEmpty(t *testing.T) {
	store, err := NewSQLiteSettingsStore(filepath.Join(t.TempDir(), "settings-empty.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSettingsStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()
	loaded, err := store.LoadSettingsSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSettingsSnapshot returned error: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected empty settings store to return nil snapshot, got %+v", loaded)
	}
	if page := cloneSettingsMap(nil); page != nil {
		t.Fatalf("expected cloneSettingsMap(nil) to stay nil, got %+v", page)
	}
}

func TestSQLiteSettingsStoreWrapsStructuredStoreErrors(t *testing.T) {
	store, err := NewSQLiteSettingsStore(filepath.Join(t.TempDir(), "settings-structured-error.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSettingsStore returned error: %v", err)
	}
	_ = store.Close()
	if err := store.SaveSettingsSnapshot(context.Background(), map[string]any{"general": map[string]any{"language": "zh-CN"}}); !errors.Is(err, ErrStructuredStoreUnavailable) {
		t.Fatalf("expected closed sqlite store write to wrap ErrStructuredStoreUnavailable, got %v", err)
	}
	if _, err := store.LoadSettingsSnapshot(context.Background()); !errors.Is(err, ErrStructuredStoreUnavailable) {
		t.Fatalf("expected closed sqlite store read to wrap ErrStructuredStoreUnavailable, got %v", err)
	}
	closedStore, err := NewSQLiteSettingsStore(filepath.Join(t.TempDir(), "settings-initialize-error.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSettingsStore returned error: %v", err)
	}
	_ = closedStore.Close()
	if err := closedStore.initialize(context.Background()); !errors.Is(err, ErrStructuredStoreUnavailable) {
		t.Fatalf("expected initialize on closed db to wrap ErrStructuredStoreUnavailable, got %v", err)
	}
}
