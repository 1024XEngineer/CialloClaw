// 该测试文件验证存储层的数据行为。
package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type stubAdapter struct {
	databasePath string
}

func (s stubAdapter) DatabasePath() string {
	return s.databasePath
}

func TestBackendReturnsSQLiteWAL(t *testing.T) {
	service := NewService(nil)

	if service.Backend() != "sqlite_wal" {
		t.Fatalf("backend mismatch: got %q", service.Backend())
	}
}

func TestDatabasePathReturnsEmptyWhenAdapterMissing(t *testing.T) {
	service := NewService(nil)

	if service.DatabasePath() != "" {
		t.Fatalf("expected empty database path, got %q", service.DatabasePath())
	}
}

func TestDatabasePathTrimsWhitespace(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "  D:/CialloClaw/data.db  "})

	if service.DatabasePath() != "D:/CialloClaw/data.db" {
		t.Fatalf("database path mismatch: got %q", service.DatabasePath())
	}
}

func TestConfiguredReturnsFalseWhenAdapterMissing(t *testing.T) {
	service := NewService(nil)

	if service.Configured() {
		t.Fatal("expected service to be unconfigured")
	}
}

func TestConfiguredReturnsFalseWhenPathEmpty(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "   "})

	if service.Configured() {
		t.Fatal("expected service to be unconfigured")
	}
}

func TestConfiguredReturnsTrueWhenAdapterAndPathPresent(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "D:/CialloClaw/data.db"})

	if !service.Configured() {
		t.Fatal("expected service to be configured")
	}
}

func TestValidateReturnsErrorWhenAdapterMissing(t *testing.T) {
	service := NewService(nil)

	err := service.Validate()
	if !errors.Is(err, ErrAdapterNotConfigured) {
		t.Fatalf("expected ErrAdapterNotConfigured, got %v", err)
	}
}

func TestValidateReturnsErrorWhenPathMissing(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "   "})

	err := service.Validate()
	if !errors.Is(err, ErrDatabasePathRequired) {
		t.Fatalf("expected ErrDatabasePathRequired, got %v", err)
	}
}

func TestValidatePassesWhenAdapterConfigured(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "D:/CialloClaw/data.db"})

	if err := service.Validate(); err != nil {
		t.Fatalf("expected valid storage service, got %v", err)
	}
}

func TestDescriptorReturnsTypedSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "descriptor.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	descriptor := service.Descriptor()
	if descriptor.Backend != "sqlite_wal" {
		t.Fatalf("backend mismatch: got %q", descriptor.Backend)
	}
	if descriptor.DatabasePath != path {
		t.Fatalf("database path mismatch: got %q", descriptor.DatabasePath)
	}
	if !descriptor.Configured {
		t.Fatal("expected descriptor to report configured service")
	}
	if !descriptor.StoreReady {
		t.Fatal("expected descriptor to report ready store")
	}
}

func TestCapabilitiesReturnsConfiguredStructuredStorageOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capabilities.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	capabilities := service.Capabilities()
	if capabilities.Backend != "sqlite_wal" {
		t.Fatalf("backend mismatch: got %q", capabilities.Backend)
	}
	if !capabilities.Configured || !capabilities.SupportsStructuredData {
		t.Fatalf("unexpected configured capabilities: %+v", capabilities)
	}
	if !capabilities.SupportsMemoryStore || capabilities.SupportsArtifactStore || capabilities.SupportsSecretStore {
		t.Fatalf("unexpected unsupported capabilities enabled: %+v", capabilities)
	}
	if capabilities.MemoryStoreBackend != memoryStoreBackendSQLite || capabilities.FallbackActive {
		t.Fatalf("unexpected backend state: %+v", capabilities)
	}
}

func TestCapabilitiesReturnsUnconfiguredSnapshotWhenPathMissing(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "   "})

	capabilities := service.Capabilities()
	if capabilities.Configured || capabilities.SupportsStructuredData {
		t.Fatalf("unexpected unconfigured capabilities: %+v", capabilities)
	}
}

func TestMemoryStoreReturnsWorkingImplementation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()
	store := service.MemoryStore()

	err := store.SaveSummary(context.Background(), MemorySummaryRecord{MemorySummaryID: "mem_001", TaskID: "task_old_001", RunID: "run_old_001", Summary: "summary", CreatedAt: "2026-04-08T10:00:00Z"})
	if err != nil {
		t.Fatalf("SaveSummary returned error: %v", err)
	}

	hits, err := store.SearchSummaries(context.Background(), "task_001", "run_001", "summary", 5)
	if err != nil {
		t.Fatalf("SearchSummaries returned error: %v", err)
	}
	if len(hits) != 1 || hits[0].MemoryID != "mem_001" {
		t.Fatalf("unexpected hits: %+v", hits)
	}

	recent, err := store.ListRecentSummaries(context.Background(), 5)
	if err != nil {
		t.Fatalf("ListRecentSummaries returned error: %v", err)
	}
	if len(recent) != 1 || recent[0].MemorySummaryID != "mem_001" {
		t.Fatalf("unexpected recent summaries: %+v", recent)
	}
}

func TestCloseIsSafeWithoutConfiguredStore(t *testing.T) {
	service := NewService(nil)
	if err := service.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}
