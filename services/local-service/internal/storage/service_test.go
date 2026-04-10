// 该测试文件验证存储层的数据行为。
package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// stubAdapter 定义当前模块的数据结构。
type stubAdapter struct {
	databasePath string
}

// DatabasePath 处理当前模块的相关逻辑。
func (s stubAdapter) DatabasePath() string {
	return s.databasePath
}

// TestBackendReturnsSQLiteWAL 验证BackendReturnsSQLiteWAL。
func TestBackendReturnsSQLiteWAL(t *testing.T) {
	service := NewService(nil)

	if service.Backend() != "sqlite_wal" {
		t.Fatalf("backend mismatch: got %q", service.Backend())
	}
}

// TestDatabasePathReturnsEmptyWhenAdapterMissing 验证DatabasePathReturnsEmptyWhenAdapterMissing。
func TestDatabasePathReturnsEmptyWhenAdapterMissing(t *testing.T) {
	service := NewService(nil)

	if service.DatabasePath() != "" {
		t.Fatalf("expected empty database path, got %q", service.DatabasePath())
	}
}

// TestDatabasePathTrimsWhitespace 验证DatabasePathTrimsWhitespace。
func TestDatabasePathTrimsWhitespace(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "  D:/CialloClaw/data.db  "})

	if service.DatabasePath() != "D:/CialloClaw/data.db" {
		t.Fatalf("database path mismatch: got %q", service.DatabasePath())
	}
}

// TestConfiguredReturnsFalseWhenAdapterMissing 验证ConfiguredReturnsFalseWhenAdapterMissing。
func TestConfiguredReturnsFalseWhenAdapterMissing(t *testing.T) {
	service := NewService(nil)

	if service.Configured() {
		t.Fatal("expected service to be unconfigured")
	}
}

// TestConfiguredReturnsFalseWhenPathEmpty 验证ConfiguredReturnsFalseWhenPathEmpty。
func TestConfiguredReturnsFalseWhenPathEmpty(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "   "})

	if service.Configured() {
		t.Fatal("expected service to be unconfigured")
	}
}

// TestConfiguredReturnsTrueWhenAdapterAndPathPresent 验证ConfiguredReturnsTrueWhenAdapterAndPathPresent。
func TestConfiguredReturnsTrueWhenAdapterAndPathPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configured.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	if !service.Configured() {
		t.Fatal("expected service to be configured")
	}
}

// TestValidateReturnsErrorWhenAdapterMissing 验证ValidateReturnsErrorWhenAdapterMissing。
func TestValidateReturnsErrorWhenAdapterMissing(t *testing.T) {
	service := NewService(nil)

	err := service.Validate()
	if !errors.Is(err, ErrAdapterNotConfigured) {
		t.Fatalf("expected ErrAdapterNotConfigured, got %v", err)
	}
}

// TestValidateReturnsErrorWhenPathMissing 验证ValidateReturnsErrorWhenPathMissing。
func TestValidateReturnsErrorWhenPathMissing(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "   "})

	err := service.Validate()
	if !errors.Is(err, ErrDatabasePathRequired) {
		t.Fatalf("expected ErrDatabasePathRequired, got %v", err)
	}
}

// TestValidatePassesWhenAdapterConfigured 验证ValidatePassesWhenAdapterConfigured。
func TestValidatePassesWhenAdapterConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "validate.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	if err := service.Validate(); err != nil {
		t.Fatalf("expected valid storage service, got %v", err)
	}
}

// TestDescriptorReturnsTypedSnapshot 验证DescriptorReturnsTypedSnapshot。
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

// TestCapabilitiesReturnsConfiguredStructuredStorageOnly 验证CapabilitiesReturnsConfiguredStructuredStorageOnly。
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
	if !capabilities.SupportsRetrievalHits || !capabilities.SupportsFTS5 || !capabilities.SupportsSQLiteVecStub {
		t.Fatalf("expected retrieval and search skeleton capabilities to be enabled: %+v", capabilities)
	}
	if capabilities.MemoryStoreBackend != memoryStoreBackendSQLite || capabilities.FallbackActive {
		t.Fatalf("unexpected backend state: %+v", capabilities)
	}
	if capabilities.MemoryRetrievalBackend != memoryRetrievalBackendSQLite {
		t.Fatalf("unexpected retrieval backend: %+v", capabilities)
	}
}

// TestCapabilitiesReturnsUnconfiguredSnapshotWhenPathMissing 验证CapabilitiesReturnsUnconfiguredSnapshotWhenPathMissing。
func TestCapabilitiesReturnsUnconfiguredSnapshotWhenPathMissing(t *testing.T) {
	service := NewService(stubAdapter{databasePath: "   "})

	capabilities := service.Capabilities()
	if capabilities.Configured || capabilities.SupportsStructuredData {
		t.Fatalf("unexpected unconfigured capabilities: %+v", capabilities)
	}
}

// TestMemoryStoreReturnsWorkingImplementation 验证MemoryStoreReturnsWorkingImplementation。
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

	err = store.SaveRetrievalHits(context.Background(), []MemoryRetrievalRecord{{
		RetrievalHitID: "hit_001",
		TaskID:         "task_001",
		RunID:          "run_001",
		MemoryID:       "mem_001",
		Score:          0.9,
		Source:         memoryRetrievalBackendSQLite,
		Summary:        "summary",
		CreatedAt:      "2026-04-08T10:01:00Z",
	}})
	if err != nil {
		t.Fatalf("SaveRetrievalHits returned error: %v", err)
	}
}

// TestCloseIsSafeWithoutConfiguredStore 验证CloseIsSafeWithoutConfiguredStore。
func TestCloseIsSafeWithoutConfiguredStore(t *testing.T) {
	service := NewService(nil)
	if err := service.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestTaskRunStoreReturnsWorkingImplementation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-runs.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	store := service.TaskRunStore()
	taskID, err := store.AllocateIdentifier(context.Background(), "task")
	if err != nil {
		t.Fatalf("AllocateIdentifier returned error: %v", err)
	}
	if taskID != "task_001" {
		t.Fatalf("expected sqlite-backed task identifier, got %q", taskID)
	}

	record := sampleTaskRunRecord()
	record.TaskID = taskID
	if err := store.SaveTaskRun(context.Background(), record); err != nil {
		t.Fatalf("SaveTaskRun returned error: %v", err)
	}

	records, err := store.LoadTaskRuns(context.Background())
	if err != nil {
		t.Fatalf("LoadTaskRuns returned error: %v", err)
	}
	if len(records) != 1 || records[0].TaskID != taskID {
		t.Fatalf("unexpected persisted task runs: %+v", records)
	}
}
