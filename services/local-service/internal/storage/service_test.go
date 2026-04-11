// 该测试文件验证存储层的数据行为。
package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
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

func TestToolCallSinkReturnsWorkingImplementation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool-calls.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	sink := service.ToolCallSink()
	if sink == nil {
		t.Fatal("expected tool call sink to be available")
	}
	err := sink.SaveToolCall(context.Background(), tools.ToolCallRecord{
		ToolCallID: "tool_call_001",
		RunID:      "run_001",
		TaskID:     "task_001",
		StepID:     "step_001",
		ToolName:   "write_file",
		Status:     tools.ToolCallStatusSucceeded,
		Input:      map[string]any{"path": "workspace/result.md"},
		Output:     map[string]any{"bytes_written": 128},
		DurationMS: 12,
	})
	if err != nil {
		t.Fatalf("SaveToolCall returned error: %v", err)
	}

	sqliteSink, ok := service.toolCallStore.(*SQLiteToolCallStore)
	if !ok {
		t.Fatalf("expected sqlite tool call store, got %T", service.toolCallStore)
	}
	assertToolCallCount(t, sqliteSink.db, 1)
}

func TestAuditWriterReturnsWorkingImplementation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	writer := service.AuditWriter()
	if writer == nil {
		t.Fatal("expected audit writer to be available")
	}
	err := writer.WriteAuditRecord(context.Background(), audit.Record{
		AuditID:   "audit_001",
		TaskID:    "task_001",
		Type:      "file",
		Action:    "write_file",
		Summary:   "write result file",
		Target:    "workspace/result.md",
		Result:    "success",
		CreatedAt: "2026-04-08T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("WriteAuditRecord returned error: %v", err)
	}
	sqliteWriter, ok := service.auditStore.(*SQLiteAuditStore)
	if !ok {
		t.Fatalf("expected sqlite audit store, got %T", service.auditStore)
	}
	assertAuditCount(t, sqliteWriter.db, 1)
}

func TestRecoveryPointWriterReturnsWorkingImplementation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	writer := service.RecoveryPointWriter()
	if writer == nil {
		t.Fatal("expected recovery point writer to be available")
	}
	err := writer.WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_001",
		TaskID:          "task_001",
		Summary:         "before overwrite",
		CreatedAt:       "2026-04-08T10:00:00Z",
		Objects:         []string{"workspace/result.md"},
	})
	if err != nil {
		t.Fatalf("WriteRecoveryPoint returned error: %v", err)
	}
	sqliteWriter, ok := service.recoveryPointStore.(*SQLiteRecoveryPointStore)
	if !ok {
		t.Fatalf("expected sqlite recovery point store, got %T", service.recoveryPointStore)
	}
	assertRecoveryPointCount(t, sqliteWriter.db, 1)
}

func TestAuditStoreListsRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit-list.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	writer := service.AuditWriter()
	_ = writer.WriteAuditRecord(context.Background(), audit.Record{AuditID: "audit_001", TaskID: "task_001", Type: "file", Action: "write_file", Summary: "write one", Target: "workspace/one.md", Result: "success", CreatedAt: "2026-04-08T10:00:00Z"})
	_ = writer.WriteAuditRecord(context.Background(), audit.Record{AuditID: "audit_002", TaskID: "task_002", Type: "file", Action: "write_file", Summary: "write two", Target: "workspace/two.md", Result: "success", CreatedAt: "2026-04-08T10:01:00Z"})

	items, total, err := service.AuditStore().ListAuditRecords(context.Background(), "task_001", 20, 0)
	if err != nil {
		t.Fatalf("ListAuditRecords returned error: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].AuditID != "audit_001" {
		t.Fatalf("unexpected audit list result: total=%d items=%+v", total, items)
	}
}

func TestRecoveryPointStoreListsPoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery-list.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	writer := service.RecoveryPointWriter()
	_ = writer.WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{RecoveryPointID: "rp_001", TaskID: "task_001", Summary: "before one", CreatedAt: "2026-04-08T10:00:00Z", Objects: []string{"workspace/one.md"}})
	_ = writer.WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{RecoveryPointID: "rp_002", TaskID: "task_002", Summary: "before two", CreatedAt: "2026-04-08T10:01:00Z", Objects: []string{"workspace/two.md"}})

	items, total, err := service.RecoveryPointStore().ListRecoveryPoints(context.Background(), "task_002", 20, 0)
	if err != nil {
		t.Fatalf("ListRecoveryPoints returned error: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].RecoveryPointID != "rp_002" {
		t.Fatalf("unexpected recovery point list result: total=%d items=%+v", total, items)
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

func assertToolCallCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM tool_calls`).Scan(&count); err != nil {
		t.Fatalf("query tool_calls count failed: %v", err)
	}
	if count != expected {
		t.Fatalf("expected tool call count %d, got %d", expected, count)
	}
}

func assertAuditCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM audit_records`).Scan(&count); err != nil {
		t.Fatalf("query audit_records count failed: %v", err)
	}
	if count != expected {
		t.Fatalf("expected audit count %d, got %d", expected, count)
	}
}

func assertRecoveryPointCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM recovery_points`).Scan(&count); err != nil {
		t.Fatalf("query recovery_points count failed: %v", err)
	}
	if count != expected {
		t.Fatalf("expected recovery point count %d, got %d", expected, count)
	}
}
