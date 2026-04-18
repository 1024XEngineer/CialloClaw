// 该测试文件验证存储层的数据行为。
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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

// SecretStorePath 处理当前模块的相关逻辑。
func (s stubAdapter) SecretStorePath() string {
	if s.databasePath == "" {
		return ""
	}
	return s.databasePath + ".stronghold"
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
	if !capabilities.SupportsMemoryStore || !capabilities.SupportsArtifactStore || !capabilities.SupportsSecretStore {
		t.Fatalf("unexpected unsupported capabilities enabled: %+v", capabilities)
	}
	if service.TraceStore() == nil || service.EvalStore() == nil {
		t.Fatalf("expected trace and eval stores to be wired: %+v", capabilities)
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
	if capabilities.SecretStoreBackend != memoryStoreBackendSQLite {
		t.Fatalf("unexpected secret backend: %+v", capabilities)
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

func TestArtifactStoreReturnsWorkingImplementation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifacts.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	store := service.ArtifactStore()
	if store == nil {
		t.Fatal("expected artifact store to be available")
	}
	err := store.SaveArtifacts(context.Background(), []ArtifactRecord{{
		ArtifactID:          "art_001",
		TaskID:              "task_001",
		ArtifactType:        "generated_doc",
		Title:               "result.md",
		Path:                "workspace/result.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/result.md","task_id":"task_001"}`,
		CreatedAt:           "2026-04-14T10:00:00Z",
	}})
	if err != nil {
		t.Fatalf("SaveArtifacts returned error: %v", err)
	}
	items, total, err := store.ListArtifacts(context.Background(), "task_001", 20, 0)
	if err != nil {
		t.Fatalf("ListArtifacts returned error: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ArtifactID != "art_001" {
		t.Fatalf("unexpected artifact records: total=%d items=%+v", total, items)
	}
}

func TestSecretStoreReturnsWorkingImplementation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	store := service.SecretStore()
	if store == nil {
		t.Fatal("expected secret store to be available")
	}
	record := SecretRecord{
		Namespace: "model",
		Key:       "openai_responses_api_key",
		Value:     "secret-key",
		UpdatedAt: "2026-04-15T10:00:00Z",
	}
	if err := store.PutSecret(context.Background(), record); err != nil {
		t.Fatalf("PutSecret returned error: %v", err)
	}
	resolved, err := store.GetSecret(context.Background(), record.Namespace, record.Key)
	if err != nil {
		t.Fatalf("GetSecret returned error: %v", err)
	}
	if resolved.Value != "secret-key" {
		t.Fatalf("unexpected secret value: %+v", resolved)
	}
	value, err := service.ResolveModelAPIKey("openai_responses")
	if err != nil {
		t.Fatalf("ResolveModelAPIKey returned error: %v", err)
	}
	if value != "secret-key" {
		t.Fatalf("unexpected resolved key: %q", value)
	}
}

func TestResolveModelAPIKeyReturnsNotFoundWhenSecretMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-secret.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()
	_, err := service.ResolveModelAPIKey("openai_responses")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestResolveModelAPIKeyReturnsAccessFailureWhenStoreClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "closed-secret.db")
	service := NewService(stubAdapter{databasePath: path})
	record := SecretRecord{
		Namespace: "model",
		Key:       "openai_responses_api_key",
		Value:     "secret-key",
		UpdatedAt: "2026-04-15T10:00:00Z",
	}
	if err := service.SecretStore().PutSecret(context.Background(), record); err != nil {
		t.Fatalf("PutSecret returned error: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	_, err := service.ResolveModelAPIKey("openai_responses")
	if !errors.Is(err, ErrSecretStoreAccessFailed) {
		t.Fatalf("expected ErrSecretStoreAccessFailed, got %v", err)
	}
}

func TestNewServiceFallsBackTraceAndEvalTogetherWhenEvalInitFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace-eval-fallback.db")
	originalTraceFactory := newSQLiteTraceStoreForService
	originalEvalFactory := newSQLiteEvalStoreForService
	defer func() {
		newSQLiteTraceStoreForService = originalTraceFactory
		newSQLiteEvalStoreForService = originalEvalFactory
	}()

	traceClosed := false
	traceStore := &stubTraceStoreWithClose{closeFn: func() error {
		traceClosed = true
		return nil
	}}
	newSQLiteTraceStoreForService = func(databasePath string) (TraceStore, error) {
		return traceStore, nil
	}
	newSQLiteEvalStoreForService = func(databasePath string) (EvalStore, error) {
		return nil, fmt.Errorf("eval init failed")
	}

	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	if !traceClosed {
		t.Fatal("expected trace store to close when eval init fails")
	}
	if _, ok := service.TraceStore().(*stubTraceStoreWithClose); ok {
		t.Fatal("expected trace store to fall back with eval store instead of keeping sqlite trace only")
	}
	if _, ok := service.EvalStore().(*inMemoryEvalStore); !ok {
		t.Fatalf("expected eval store to fall back to in-memory, got %T", service.EvalStore())
	}
	if err := service.Validate(); err == nil || !strings.Contains(err.Error(), "initialize sqlite trace/eval stores") {
		t.Fatalf("expected joined trace/eval init error, got %v", err)
	}
	if !service.Capabilities().FallbackActive {
		t.Fatal("expected fallback flag when trace/eval pair downgrades together")
	}
}

func TestInitializeSQLiteTraceEvalStoresReturnsPairOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace-eval-success.db")
	originalTraceFactory := newSQLiteTraceStoreForService
	originalEvalFactory := newSQLiteEvalStoreForService
	defer func() {
		newSQLiteTraceStoreForService = originalTraceFactory
		newSQLiteEvalStoreForService = originalEvalFactory
	}()

	traceStore := newInMemoryTraceStore()
	evalStore := newInMemoryEvalStore()
	newSQLiteTraceStoreForService = func(databasePath string) (TraceStore, error) {
		return traceStore, nil
	}
	newSQLiteEvalStoreForService = func(databasePath string) (EvalStore, error) {
		return evalStore, nil
	}

	gotTraceStore, gotEvalStore, err := initializeSQLiteTraceEvalStores(path)
	if err != nil {
		t.Fatalf("initializeSQLiteTraceEvalStores returned error: %v", err)
	}
	if gotTraceStore != traceStore || gotEvalStore != evalStore {
		t.Fatalf("expected paired stores to be returned, got trace=%T eval=%T", gotTraceStore, gotEvalStore)
	}
}

func TestInitializeSQLiteTraceEvalStoresReturnsTraceError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace-eval-trace-error.db")
	originalTraceFactory := newSQLiteTraceStoreForService
	originalEvalFactory := newSQLiteEvalStoreForService
	defer func() {
		newSQLiteTraceStoreForService = originalTraceFactory
		newSQLiteEvalStoreForService = originalEvalFactory
	}()

	newSQLiteTraceStoreForService = func(databasePath string) (TraceStore, error) {
		return nil, fmt.Errorf("trace init failed")
	}
	newSQLiteEvalStoreForService = func(databasePath string) (EvalStore, error) {
		return newInMemoryEvalStore(), nil
	}

	gotTraceStore, gotEvalStore, err := initializeSQLiteTraceEvalStores(path)
	if err == nil || !strings.Contains(err.Error(), "trace store") {
		t.Fatalf("expected trace init error, got trace=%v eval=%v err=%v", gotTraceStore, gotEvalStore, err)
	}
}

type stubTraceStoreWithClose struct {
	closeFn func() error
}

func (s *stubTraceStoreWithClose) WriteTraceRecord(context.Context, TraceRecord) error { return nil }
func (s *stubTraceStoreWithClose) DeleteTraceRecord(context.Context, string) error     { return nil }
func (s *stubTraceStoreWithClose) ListTraceRecords(context.Context, string, int, int) ([]TraceRecord, int, error) {
	return nil, 0, nil
}
func (s *stubTraceStoreWithClose) Close() error {
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
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
