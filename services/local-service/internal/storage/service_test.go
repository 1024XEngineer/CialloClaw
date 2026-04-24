// 该测试文件验证存储层的数据行为。
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
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

type stubStrongholdProvider struct {
	backend     string
	fallback    bool
	store       SecretStore
	openErr     error
	initialized bool
	openCalls   int
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

func (s *stubStrongholdProvider) Open(context.Context) (SecretStore, error) {
	s.openCalls++
	if s.openErr != nil {
		s.initialized = false
		return nil, s.openErr
	}
	s.initialized = true
	if s.store == nil {
		s.store = newInMemorySecretStore()
	}
	return s.store, nil
}

func (s *stubStrongholdProvider) Descriptor() StrongholdDescriptor {
	return StrongholdDescriptor{
		Backend:     s.backend,
		Available:   s.initialized && s.openErr == nil,
		Fallback:    s.fallback,
		Initialized: s.initialized,
	}
}

func TestDefaultStrongholdBootstrapConfigReturnsProviderFactories(t *testing.T) {
	config := defaultStrongholdBootstrapConfig()
	if config.formalProvider == nil || config.fallbackProvider == nil {
		t.Fatalf("expected default Stronghold bootstrap config to provide both factories, got %+v", config)
	}
	formal := config.formalProvider(filepath.Join(t.TempDir(), "formal.stronghold"))
	if formal == nil || formal.Descriptor().Backend != "stronghold" || formal.Descriptor().Fallback {
		t.Fatalf("expected default formal provider descriptor, got %+v", formal)
	}
	fallback := config.fallbackProvider(filepath.Join(t.TempDir(), "fallback.stronghold"))
	if fallback == nil || fallback.Descriptor().Backend != "stronghold_sqlite_fallback" || !fallback.Descriptor().Fallback {
		t.Fatalf("expected default fallback provider descriptor, got %+v", fallback)
	}
}

func TestNewServiceWithStrongholdBootstrapFillsMissingFactories(t *testing.T) {
	t.Run("fills missing formal provider", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bootstrap-missing-formal.db")
		fallbackCalls := 0
		fallbackProvider := &stubStrongholdProvider{backend: "fallback_only", fallback: true, store: newInMemorySecretStore()}
		service := newServiceWithStrongholdBootstrap(stubAdapter{databasePath: path}, strongholdBootstrapConfig{
			fallbackProvider: func(string) StrongholdProvider {
				fallbackCalls++
				return fallbackProvider
			},
		})
		defer func() { _ = service.Close() }()
		if service == nil {
			t.Fatal("expected missing formal provider bootstrap to create a storage service")
		}
		if runtime.GOOS == "windows" {
			if service.Stronghold() == nil || service.Stronghold().Descriptor().Backend != "stronghold" {
				t.Fatalf("expected default formal provider to back Stronghold on supported platforms, got %+v", service.Stronghold())
			}
			if fallbackCalls != 0 {
				t.Fatalf("expected fallback provider to remain unused when default formal Stronghold succeeds, got %d calls", fallbackCalls)
			}
			return
		}
		if service.Stronghold() == nil || service.Stronghold().Descriptor().Backend != "fallback_only" || fallbackCalls != 1 {
			t.Fatalf("expected default formal provider failure to activate injected fallback on unsupported platforms, provider=%+v fallbackCalls=%d", service.Stronghold(), fallbackCalls)
		}
	})

	t.Run("fills missing fallback provider", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bootstrap-missing-fallback.db")
		formalCalls := 0
		formalProvider := &stubStrongholdProvider{backend: "formal_only", openErr: ErrStrongholdUnavailable}
		service := newServiceWithStrongholdBootstrap(stubAdapter{databasePath: path}, strongholdBootstrapConfig{
			formalProvider: func(string) StrongholdProvider {
				formalCalls++
				return formalProvider
			},
		})
		defer func() { _ = service.Close() }()
		if service == nil {
			t.Fatal("expected missing fallback provider bootstrap to create a storage service")
		}
		if formalCalls != 1 {
			t.Fatalf("expected injected formal provider to be attempted once, got %d calls", formalCalls)
		}
		if service.Stronghold() == nil || service.Stronghold().Descriptor().Backend != "stronghold_sqlite_fallback" || !service.Stronghold().Descriptor().Fallback {
			t.Fatalf("expected missing fallback provider to use default fallback Stronghold, got %+v", service.Stronghold())
		}
	})

	t.Run("fills both providers from defaults", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bootstrap-defaults.db")
		service := newServiceWithStrongholdBootstrap(stubAdapter{databasePath: path}, strongholdBootstrapConfig{})
		defer func() { _ = service.Close() }()
		if service == nil {
			t.Fatal("expected default bootstrap to create a storage service")
		}
		if service.Stronghold() == nil {
			t.Fatal("expected default bootstrap to wire a Stronghold provider")
		}
		descriptor := service.Stronghold().Descriptor()
		if runtime.GOOS == "windows" {
			if descriptor.Backend != "stronghold" || descriptor.Fallback {
				t.Fatalf("expected supported platforms to use formal Stronghold defaults, got %+v", descriptor)
			}
			return
		}
		if descriptor.Backend != "stronghold_sqlite_fallback" || !descriptor.Fallback {
			t.Fatalf("expected unsupported platforms to fall back with default providers, got %+v", descriptor)
		}
	})
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
	if service.ToolCallStore() == nil {
		t.Fatalf("expected typed tool call store accessor to be wired: %+v", capabilities)
	}
	if !capabilities.SupportsRetrievalHits || !capabilities.SupportsFTS5 || !capabilities.SupportsSQLiteVecStub {
		t.Fatalf("expected retrieval and search skeleton capabilities to be enabled: %+v", capabilities)
	}
	if capabilities.MemoryStoreBackend != memoryStoreBackendSQLite || capabilities.FallbackActive {
		if runtime.GOOS == "windows" {
			t.Fatalf("unexpected backend state: %+v", capabilities)
		}
		if !capabilities.FallbackActive {
			t.Fatalf("expected unsupported platforms to report fallback activity, got %+v", capabilities)
		}
	}
	if capabilities.MemoryRetrievalBackend != memoryRetrievalBackendSQLite {
		t.Fatalf("unexpected retrieval backend: %+v", capabilities)
	}
	if runtime.GOOS == "windows" {
		if capabilities.SecretStoreBackend != "stronghold" {
			t.Fatalf("unexpected secret backend: %+v", capabilities)
		}
	} else {
		if capabilities.SecretStoreBackend != "stronghold_sqlite_fallback" {
			t.Fatalf("expected fallback secret backend on unsupported platforms, got %+v", capabilities)
		}
	}
	if service.SkillManifestStore() == nil || service.BlueprintDefinitionStore() == nil || service.PromptTemplateVersionStore() == nil || service.PluginManifestStore() == nil {
		t.Fatalf("expected config asset stores to be wired: %+v", capabilities)
	}
	if service.SessionStore() == nil || service.SettingsStore() == nil {
		t.Fatalf("expected session/settings stores to be wired: %+v", capabilities)
	}
}

func TestServiceConfigAssetStoresPersistRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-assets-service.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()
	if err := service.SkillManifestStore().WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_001", Name: "skill", Version: "v1", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write skill manifest failed: %v", err)
	}
	if err := service.BlueprintDefinitionStore().WriteBlueprintDefinition(context.Background(), BlueprintDefinitionRecord{BlueprintDefinitionID: "blueprint_001", Name: "blueprint", Version: "v1", Source: "builtin", Summary: "summary", DefinitionJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write blueprint definition failed: %v", err)
	}
	if err := service.PromptTemplateVersionStore().WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_001", TemplateName: "default", Version: "v1", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write prompt template version failed: %v", err)
	}
	if err := service.PluginManifestStore().WritePluginManifest(context.Background(), PluginManifestRecord{PluginID: "plugin_001", Name: "ocr", Version: "v1", Entry: "builtin://plugin/ocr", Source: "builtin", Summary: "summary", CapabilitiesJSON: `["ocr_image"]`, PermissionsJSON: `["artifact_read"]`, RuntimeNamesJSON: `["ocr_worker"]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write plugin manifest failed: %v", err)
	}
	if _, err := service.SkillManifestStore().GetSkillManifest(context.Background(), "skill_001"); err != nil {
		t.Fatalf("get skill manifest failed: %v", err)
	}
	if _, err := service.BlueprintDefinitionStore().GetBlueprintDefinition(context.Background(), "blueprint_001"); err != nil {
		t.Fatalf("get blueprint definition failed: %v", err)
	}
	if _, err := service.PromptTemplateVersionStore().GetPromptTemplateVersion(context.Background(), "prompt_001"); err != nil {
		t.Fatalf("get prompt template version failed: %v", err)
	}
	if _, err := service.PluginManifestStore().GetPluginManifest(context.Background(), "plugin_001"); err != nil {
		t.Fatalf("get plugin manifest failed: %v", err)
	}
}

func TestServiceSessionAndSettingsStoresPersistRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-settings-service.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()
	if err := service.SessionStore().WriteSession(context.Background(), SessionRecord{SessionID: "sess_001", Title: "Session 001", Status: "active", CreatedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:00:00Z"}); err != nil {
		t.Fatalf("write session failed: %v", err)
	}
	if err := service.SettingsStore().SaveSettingsSnapshot(context.Background(), map[string]any{
		"general": map[string]any{"language": "zh-CN"},
		"models":  map[string]any{"provider": "openai", "credentials": map[string]any{"budget_auto_downgrade": true}},
	}); err != nil {
		t.Fatalf("save settings snapshot failed: %v", err)
	}
	session, err := service.SessionStore().GetSession(context.Background(), "sess_001")
	if err != nil || session.Title != "Session 001" {
		t.Fatalf("get session failed: session=%+v err=%v", session, err)
	}
	snapshot, err := service.SettingsStore().LoadSettingsSnapshot(context.Background())
	if err != nil {
		t.Fatalf("load settings snapshot failed: %v", err)
	}
	if snapshot["general"].(map[string]any)["language"] != "zh-CN" {
		t.Fatalf("unexpected settings snapshot: %+v", snapshot)
	}
}

func TestResolveModelAPIKeyFailsWhenSecretStoreMissing(t *testing.T) {
	service := NewService(nil)
	service.secretStore = nil
	if _, err := service.ResolveModelAPIKey("openai_responses"); !errors.Is(err, ErrSecretStoreAccessFailed) {
		t.Fatalf("expected ErrSecretStoreAccessFailed when secret store is missing, got %v", err)
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

func TestApprovalAndAuthorizationStoresPersistStructuredGovernanceRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "approval-auth.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	err := service.ApprovalRequestStore().WriteApprovalRequest(context.Background(), ApprovalRequestRecord{
		ApprovalID:      "appr_001",
		TaskID:          "task_approval_001",
		OperationName:   "screen_capture",
		RiskLevel:       "yellow",
		TargetObject:    "inputs/screen.png",
		Reason:          "screen_capture_requires_authorization",
		Status:          "pending",
		ImpactScopeJSON: `{"files":["inputs/screen.png"]}`,
		CreatedAt:       "2026-04-18T10:00:00Z",
		UpdatedAt:       "2026-04-18T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("write approval request failed: %v", err)
	}
	err = service.AuthorizationRecordStore().WriteAuthorizationRecord(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_001",
		TaskID:                "task_approval_001",
		ApprovalID:            "appr_001",
		Decision:              "allow_once",
		Operator:              "user",
		RememberRule:          true,
		CreatedAt:             "2026-04-18T10:01:00Z",
	})
	if err != nil {
		t.Fatalf("write authorization record failed: %v", err)
	}
	approvalItems, approvalTotal, err := service.ApprovalRequestStore().ListApprovalRequests(context.Background(), "task_approval_001", 10, 0)
	if err != nil || approvalTotal != 1 || len(approvalItems) != 1 {
		t.Fatalf("unexpected approval records total=%d len=%d err=%v", approvalTotal, len(approvalItems), err)
	}
	if approvalItems[0].OperationName != "screen_capture" || approvalItems[0].Status != "pending" {
		t.Fatalf("unexpected approval record: %+v", approvalItems[0])
	}
	authorizationItems, authorizationTotal, err := service.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), "task_approval_001", 10, 0)
	if err != nil || authorizationTotal != 1 || len(authorizationItems) != 1 {
		t.Fatalf("unexpected authorization records total=%d len=%d err=%v", authorizationTotal, len(authorizationItems), err)
	}
	if authorizationItems[0].Decision != "allow_once" || !authorizationItems[0].RememberRule {
		t.Fatalf("unexpected authorization record: %+v", authorizationItems[0])
	}
	if err := service.ApprovalRequestStore().UpdateApprovalRequestStatus(context.Background(), "appr_001", "approved", "2026-04-18T10:02:00Z"); err != nil {
		t.Fatalf("update approval request status failed: %v", err)
	}
	updatedApprovalItems, updatedApprovalTotal, err := service.ApprovalRequestStore().ListApprovalRequests(context.Background(), "task_approval_001", 10, 0)
	if err != nil || updatedApprovalTotal != 1 || len(updatedApprovalItems) != 1 {
		t.Fatalf("unexpected updated approval records total=%d len=%d err=%v", updatedApprovalTotal, len(updatedApprovalItems), err)
	}
	if updatedApprovalItems[0].Status != "approved" || updatedApprovalItems[0].UpdatedAt != "2026-04-18T10:02:00Z" {
		t.Fatalf("expected updated approval record status, got %+v", updatedApprovalItems[0])
	}
	pendingApprovalItems, pendingApprovalTotal, err := service.ApprovalRequestStore().ListPendingApprovalRequests(context.Background(), 10, 0)
	if err != nil || pendingApprovalTotal != 0 || len(pendingApprovalItems) != 0 {
		t.Fatalf("expected no pending approvals after approval update, got total=%d len=%d err=%v items=%+v", pendingApprovalTotal, len(pendingApprovalItems), err, pendingApprovalItems)
	}
	err = service.AuthorizationRecordStore().WriteAuthorizationRecord(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_002",
		TaskID:                "task_approval_001",
		ApprovalID:            "appr_001",
		Decision:              "deny_once",
		Operator:              "user",
		RememberRule:          false,
		CreatedAt:             "2026-04-18T10:03:00Z",
	})
	if err != nil {
		t.Fatalf("write second authorization record failed: %v", err)
	}
	updatedAuthorizationItems, updatedAuthorizationTotal, err := service.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), "task_approval_001", 10, 0)
	if err != nil || updatedAuthorizationTotal != 2 || len(updatedAuthorizationItems) != 2 {
		t.Fatalf("expected full authorization history total=%d len=%d err=%v items=%+v", updatedAuthorizationTotal, len(updatedAuthorizationItems), err, updatedAuthorizationItems)
	}
	if updatedAuthorizationItems[0].AuthorizationRecordID != "auth_002" || updatedAuthorizationItems[1].AuthorizationRecordID != "auth_001" {
		t.Fatalf("expected authorization history ordering to preserve both records, got %+v", updatedAuthorizationItems)
	}
}

func TestTaskStoresCloseAndErrorHelpers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-store-close.db")
	taskStore, err := NewSQLiteTaskStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskStore returned error: %v", err)
	}
	if err := taskStore.WriteTask(context.Background(), TaskRecord{
		TaskID:              "task_close_001",
		SessionID:           "sess_close_001",
		RunID:               "run_close_001",
		Title:               "close helper task",
		SourceType:          "hover_input",
		Status:              "completed",
		IntentName:          "summarize",
		IntentArgumentsJSON: `{}`,
		PreferredDelivery:   "workspace_document",
		FallbackDelivery:    "bubble",
		CurrentStep:         "return_result",
		CurrentStepStatus:   "completed",
		RiskLevel:           "green",
		StartedAt:           "2026-04-18T10:00:00Z",
		UpdatedAt:           "2026-04-18T10:01:00Z",
		SnapshotJSON:        `{}`,
	}); err != nil {
		t.Fatalf("WriteTask returned error: %v", err)
	}
	if err := taskStore.DeleteTask(context.Background(), "task_close_001"); err != nil {
		t.Fatalf("DeleteTask returned error: %v", err)
	}
	if _, err := taskStore.GetTask(context.Background(), "task_close_001"); !IsTaskRecordNotFound(err) {
		t.Fatalf("expected IsTaskRecordNotFound to detect deleted row, got %v", err)
	}
	if err := taskStore.Close(); err != nil {
		t.Fatalf("expected SQLiteTaskStore close to succeed, got %v", err)
	}
	var nilTaskStore SQLiteTaskStore
	if err := nilTaskStore.Close(); err != nil {
		t.Fatalf("expected nil SQLiteTaskStore close to succeed, got %v", err)
	}

	stepStore, err := NewSQLiteTaskStepStore(filepath.Join(t.TempDir(), "task-step-close.db"))
	if err != nil {
		t.Fatalf("NewSQLiteTaskStepStore returned error: %v", err)
	}
	if err := stepStore.ReplaceTaskSteps(context.Background(), "task_close_001", []TaskStepRecord{{
		StepID:        "step_close_001",
		TaskID:        "task_close_001",
		Name:          "return_result",
		Status:        "completed",
		OrderIndex:    1,
		InputSummary:  "input",
		OutputSummary: "output",
		CreatedAt:     "2026-04-18T10:00:00Z",
		UpdatedAt:     "2026-04-18T10:01:00Z",
	}}); err != nil {
		t.Fatalf("ReplaceTaskSteps returned error: %v", err)
	}
	if err := stepStore.Close(); err != nil {
		t.Fatalf("expected SQLiteTaskStepStore close to succeed, got %v", err)
	}
	var nilStepStore SQLiteTaskStepStore
	if err := nilStepStore.Close(); err != nil {
		t.Fatalf("expected nil SQLiteTaskStepStore close to succeed, got %v", err)
	}

	if nullableText("") != nil || nullableText("value") != "value" {
		t.Fatalf("expected nullableText helper to preserve empty/non-empty semantics")
	}
	if IsTaskRecordNotFound(nil) || IsTaskRecordNotFound(sql.ErrConnDone) {
		t.Fatalf("expected IsTaskRecordNotFound to reject nil and unrelated errors")
	}
}

func TestAuthorizationDecisionWriteIsAtomicInSQLiteStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "approval-auth-atomic.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	if err := service.ApprovalRequestStore().WriteApprovalRequest(context.Background(), ApprovalRequestRecord{
		ApprovalID:      "appr_atomic_001",
		TaskID:          "task_atomic_001",
		OperationName:   "write_file",
		RiskLevel:       "yellow",
		TargetObject:    "workspace/result.md",
		Reason:          "atomic authorization persistence",
		Status:          "pending",
		ImpactScopeJSON: `{"files":["workspace/result.md"]}`,
		CreatedAt:       "2026-04-18T10:00:00Z",
		UpdatedAt:       "2026-04-18T10:00:00Z",
	}); err != nil {
		t.Fatalf("write approval request failed: %v", err)
	}
	if err := service.AuthorizationRecordStore().WriteAuthorizationDecision(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_atomic_001",
		TaskID:                "task_atomic_001",
		ApprovalID:            "appr_atomic_001",
		Decision:              "allow_once",
		Operator:              "user",
		RememberRule:          true,
		CreatedAt:             "2026-04-18T10:01:00Z",
	}, "approved", "2026-04-18T10:01:00Z"); err != nil {
		t.Fatalf("write atomic authorization decision failed: %v", err)
	}

	approvalItems, approvalTotal, err := service.ApprovalRequestStore().ListApprovalRequests(context.Background(), "task_atomic_001", 10, 0)
	if err != nil || approvalTotal != 1 || len(approvalItems) != 1 {
		t.Fatalf("unexpected approval records after atomic authorization write total=%d len=%d err=%v", approvalTotal, len(approvalItems), err)
	}
	if approvalItems[0].Status != "approved" {
		t.Fatalf("expected atomic authorization write to update approval status, got %+v", approvalItems[0])
	}

	err = service.AuthorizationRecordStore().WriteAuthorizationDecision(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_atomic_missing",
		TaskID:                "task_atomic_missing",
		ApprovalID:            "appr_missing",
		Decision:              "deny_once",
		Operator:              "user",
		CreatedAt:             "2026-04-18T10:02:00Z",
	}, "denied", "2026-04-18T10:02:00Z")
	if !errors.Is(err, ErrApprovalRequestNotFound) {
		t.Fatalf("expected missing approval to fail atomic authorization write, got %v", err)
	}

	authorizationItems, authorizationTotal, err := service.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), "", 10, 0)
	if err != nil || authorizationTotal != 1 || len(authorizationItems) != 1 {
		t.Fatalf("expected failed atomic authorization write to leave history unchanged total=%d len=%d err=%v items=%+v", authorizationTotal, len(authorizationItems), err, authorizationItems)
	}
	if authorizationItems[0].AuthorizationRecordID != "auth_atomic_001" {
		t.Fatalf("expected only committed authorization decision to remain, got %+v", authorizationItems)
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

func TestLoopRuntimeStorePersistsNormalizedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop-runtime.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	store := service.LoopRuntimeStore()
	if store == nil {
		t.Fatal("expected loop runtime store to be available")
	}
	if err := store.SaveRun(context.Background(), RunRecord{
		RunID:      "run_loop_001",
		TaskID:     "task_loop_001",
		SessionID:  "sess_loop_001",
		SourceType: "hover_input",
		Status:     "completed",
		IntentName: "agent_loop",
		StartedAt:  "2026-04-17T10:00:00Z",
		UpdatedAt:  "2026-04-17T10:00:05Z",
		FinishedAt: "2026-04-17T10:00:06Z",
		StopReason: "completed",
	}); err != nil {
		t.Fatalf("SaveRun returned error: %v", err)
	}
	if err := store.SaveSteps(context.Background(), []StepRecord{{
		StepID:        "step_loop_001",
		RunID:         "run_loop_001",
		TaskID:        "task_loop_001",
		OrderIndex:    1,
		AttemptIndex:  1,
		SegmentKind:   "initial",
		LoopRound:     1,
		Name:          "agent_loop_round",
		Status:        "completed",
		InputSummary:  "planner input",
		OutputSummary: "planner output",
		StopReason:    "completed",
		StartedAt:     "2026-04-17T10:00:00Z",
		CompletedAt:   "2026-04-17T10:00:01Z",
		PlannerInput:  "read file",
		PlannerOutput: "call read_file",
		Observation:   "file contents loaded",
		ToolName:      "read_file",
		ToolCallID:    "tool_call_001",
	}}); err != nil {
		t.Fatalf("SaveSteps returned error: %v", err)
	}
	if err := store.SaveEvents(context.Background(), []EventRecord{{
		EventID:     "evt_loop_001",
		RunID:       "run_loop_001",
		TaskID:      "task_loop_001",
		StepID:      "step_loop_001",
		Type:        "loop.completed",
		Level:       "info",
		PayloadJSON: `{"stop_reason":"completed"}`,
		CreatedAt:   "2026-04-17T10:00:06Z",
	}}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}
	if err := store.SaveDeliveryResult(context.Background(), DeliveryResultRecord{
		DeliveryResultID: "delivery_result_001",
		TaskID:           "task_loop_001",
		Type:             "bubble",
		Title:            "Loop result",
		PayloadJSON:      `{"task_id":"task_loop_001"}`,
		PreviewText:      "loop preview",
		CreatedAt:        "2026-04-17T10:00:06Z",
	}); err != nil {
		t.Fatalf("SaveDeliveryResult returned error: %v", err)
	}
	if err := store.ReplaceTaskCitations(context.Background(), "task_loop_001", []CitationRecord{{
		CitationID:      "cit_loop_001",
		TaskID:          "task_loop_001",
		RunID:           "run_loop_001",
		SourceType:      "file",
		SourceRef:       "art_loop_001",
		Label:           "error_evidence | screen_capture | missing release asset",
		ArtifactID:      "art_loop_001",
		ArtifactType:    "screen_capture",
		EvidenceRole:    "error_evidence",
		ExcerptText:     "missing release asset",
		ScreenSessionID: "screen_sess_001",
		OrderIndex:      0,
	}}); err != nil {
		t.Fatalf("ReplaceTaskCitations returned error: %v", err)
	}

	sqliteStore, ok := service.loopRuntimeStore.(*SQLiteLoopRuntimeStore)
	if !ok {
		t.Fatalf("expected sqlite loop runtime store, got %T", service.loopRuntimeStore)
	}
	assertTableCount(t, sqliteStore.db, "runs", 1)
	assertTableCount(t, sqliteStore.db, "steps", 1)
	var runSourceType string
	if err := sqliteStore.db.QueryRow(`SELECT source_type FROM runs WHERE run_id = ?`, "run_loop_001").Scan(&runSourceType); err != nil {
		t.Fatalf("query run source_type failed: %v", err)
	}
	if runSourceType != "hover_input" {
		t.Fatalf("expected run source_type to persist, got %s", runSourceType)
	}
	var attemptIndex int
	var segmentKind string
	if err := sqliteStore.db.QueryRow(`SELECT attempt_index, segment_kind FROM steps WHERE step_id = ?`, "step_loop_001").Scan(&attemptIndex, &segmentKind); err != nil {
		t.Fatalf("query step attempt/segment failed: %v", err)
	}
	if attemptIndex != 1 || segmentKind != "initial" {
		t.Fatalf("expected step attempt/segment to persist, got attempt=%d segment=%s", attemptIndex, segmentKind)
	}
	if err := service.loopRuntimeStore.SaveSteps(context.Background(), []StepRecord{{
		StepID:        "task_loop_001_attempt_01_resume_step_loop_001",
		RunID:         "run_loop_001",
		TaskID:        "task_loop_001",
		OrderIndex:    1,
		AttemptIndex:  1,
		SegmentKind:   "resume",
		LoopRound:     1,
		Name:          "agent_loop_round",
		Status:        "completed",
		InputSummary:  "resume planner input",
		OutputSummary: "resume planner output",
		StopReason:    "completed",
		StartedAt:     "2026-04-17T10:00:02Z",
		CompletedAt:   "2026-04-17T10:00:03Z",
	}}); err != nil {
		t.Fatalf("SaveSteps for resume segment returned error: %v", err)
	}
	assertTableCount(t, sqliteStore.db, "steps", 2)
	assertTableCount(t, sqliteStore.db, "events", 1)
	assertTableCount(t, sqliteStore.db, "delivery_results", 1)
	assertTableCount(t, sqliteStore.db, "task_citations", 1)

	events, total, err := store.ListEvents(context.Background(), "task_loop_001", "", "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if total != 1 || len(events) != 1 || events[0].Type != "loop.completed" {
		t.Fatalf("unexpected loop events: total=%d items=%+v", total, events)
	}
	deliveryResult, ok, err := store.GetLatestDeliveryResult(context.Background(), "task_loop_001")
	if err != nil {
		t.Fatalf("GetLatestDeliveryResult returned error: %v", err)
	}
	if !ok || deliveryResult.DeliveryResultID != "delivery_result_001" || deliveryResult.PreviewText != "loop preview" {
		t.Fatalf("unexpected latest delivery_result: ok=%v record=%+v", ok, deliveryResult)
	}
	citations, err := store.ListTaskCitations(context.Background(), "task_loop_001")
	if err != nil {
		t.Fatalf("ListTaskCitations returned error: %v", err)
	}
	if len(citations) != 1 || citations[0].CitationID != "cit_loop_001" || citations[0].ExcerptText != "missing release asset" {
		t.Fatalf("unexpected stored citations: %+v", citations)
	}
}

func TestServiceTaskStoresTrackStructuredTaskRecordsFromTaskRunSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-structured.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()

	record := sampleTaskRunRecord()
	if err := service.TaskRunStore().SaveTaskRun(context.Background(), record); err != nil {
		t.Fatalf("SaveTaskRun returned error: %v", err)
	}

	taskItems, taskTotal, err := service.TaskStore().ListTasks(context.Background(), 10, 0)
	if err != nil || taskTotal != 1 || len(taskItems) != 1 {
		t.Fatalf("expected one first-class task record, got total=%d items=%+v err=%v", taskTotal, taskItems, err)
	}
	if taskItems[0].TaskID != record.TaskID || taskItems[0].IntentName != "summarize" {
		t.Fatalf("unexpected first-class task record: %+v", taskItems[0])
	}
	stepItems, stepTotal, err := service.TaskStepStore().ListTaskSteps(context.Background(), record.TaskID, 10, 0)
	if err != nil || stepTotal != 1 || len(stepItems) != 1 {
		t.Fatalf("expected one first-class task_step record, got total=%d items=%+v err=%v", stepTotal, stepItems, err)
	}
	if stepItems[0].StepID != record.Timeline[0].StepID || stepItems[0].Name != "return_result" {
		t.Fatalf("unexpected first-class task_step record: %+v", stepItems[0])
	}
	if err := service.TaskRunStore().DeleteTaskRun(context.Background(), record.TaskID); err != nil {
		t.Fatalf("DeleteTaskRun returned error: %v", err)
	}
	taskItems, taskTotal, err = service.TaskStore().ListTasks(context.Background(), 10, 0)
	if err != nil || taskTotal != 0 || len(taskItems) != 0 {
		t.Fatalf("expected first-class task record to be deleted, got total=%d items=%+v err=%v", taskTotal, taskItems, err)
	}
}

func TestLoopRuntimeStoreKeepsAppendOnlyEventsAcrossRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop-runtime-append.db")
	service := NewService(stubAdapter{databasePath: path})
	defer func() { _ = service.Close() }()
	store := service.LoopRuntimeStore()
	if err := store.SaveEvents(context.Background(), []EventRecord{{
		EventID:     "evt_loop_run_001_001",
		RunID:       "run_001",
		TaskID:      "task_001",
		StepID:      "run_001_step_loop_01",
		Type:        "loop.round.completed",
		Level:       "info",
		PayloadJSON: `{"stop_reason":"completed"}`,
		CreatedAt:   "2026-04-17T10:00:00Z",
	}}); err != nil {
		t.Fatalf("save first event failed: %v", err)
	}
	if err := store.SaveEvents(context.Background(), []EventRecord{{
		EventID:     "evt_loop_run_002_001",
		RunID:       "run_002",
		TaskID:      "task_001",
		StepID:      "run_002_step_loop_01",
		Type:        "loop.round.completed",
		Level:       "info",
		PayloadJSON: `{"stop_reason":"completed"}`,
		CreatedAt:   "2026-04-17T10:01:00Z",
	}}); err != nil {
		t.Fatalf("save second event failed: %v", err)
	}
	events, total, err := store.ListEvents(context.Background(), "task_001", "", "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("list append-only events failed: %v", err)
	}
	if total != 2 || len(events) != 2 {
		t.Fatalf("expected append-only events from multiple runs, got total=%d items=%+v", total, events)
	}

	filteredByRun, totalByRun, err := store.ListEvents(context.Background(), "task_001", "run_002", "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("list events by run failed: %v", err)
	}
	if totalByRun != 1 || len(filteredByRun) != 1 || filteredByRun[0].RunID != "run_002" {
		t.Fatalf("expected one run-scoped event, got total=%d items=%+v", totalByRun, filteredByRun)
	}

	filteredByType, totalByType, err := store.ListEvents(context.Background(), "task_001", "", "loop.round.completed", "", "", 20, 0)
	if err != nil {
		t.Fatalf("list events by type failed: %v", err)
	}
	if totalByType != 2 || len(filteredByType) != 2 {
		t.Fatalf("expected two type-scoped events, got total=%d items=%+v", totalByType, filteredByType)
	}

	filteredByTime, totalByTime, err := store.ListEvents(context.Background(), "task_001", "", "", "2026-04-17T10:00:30Z", "2026-04-17T10:01:30Z", 20, 0)
	if err != nil {
		t.Fatalf("list events by time failed: %v", err)
	}
	if totalByTime != 1 || len(filteredByTime) != 1 || filteredByTime[0].RunID != "run_002" {
		t.Fatalf("expected one time-scoped event, got total=%d items=%+v", totalByTime, filteredByTime)
	}
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

func TestServiceUsesUnavailableSecretStoreWhenStrongholdCannotOpen(t *testing.T) {
	service := NewService(stubAdapter{databasePath: ""})
	defer func() { _ = service.Close() }()
	if service.Stronghold() != nil {
		t.Fatalf("expected no Stronghold provider when secret path missing, got %+v", service.Stronghold().Descriptor())
	}
	if err := service.SecretStore().PutSecret(context.Background(), SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: "2026-04-15T10:00:00Z"}); err != nil {
		t.Fatalf("expected in-memory fallback store when Stronghold path missing, got %v", err)
	}

	store := SecretStore(UnavailableSecretStore{})
	if err := store.PutSecret(context.Background(), SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: "2026-04-15T10:00:00Z"}); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected unavailable formal store to reject writes, got %v", err)
	}
}

func TestServiceUsesFormalStrongholdProviderWhenAvailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stronghold-formal-service.db")

	formalProvider := &stubStrongholdProvider{
		backend:  "stronghold",
		fallback: false,
		store:    newInMemorySecretStore(),
	}
	service := newServiceWithStrongholdBootstrap(stubAdapter{databasePath: path}, strongholdBootstrapConfig{
		formalProvider: func(string) StrongholdProvider { return formalProvider },
		fallbackProvider: func(string) StrongholdProvider {
			t.Fatal("fallback provider should not open when formal Stronghold succeeds")
			return nil
		},
	})
	defer func() { _ = service.Close() }()

	if service.Stronghold() == nil {
		t.Fatal("expected stronghold provider to be wired")
	}
	descriptor := service.Stronghold().Descriptor()
	if descriptor.Backend != "stronghold" || !descriptor.Available || descriptor.Fallback || !descriptor.Initialized {
		t.Fatalf("expected formal stronghold descriptor after successful bootstrap, got %+v", descriptor)
	}
	if formalProvider.openCalls != 1 {
		t.Fatalf("expected formal stronghold provider to open exactly once, got %d", formalProvider.openCalls)
	}
	if service.Capabilities().SecretStoreBackend != "stronghold" || service.Capabilities().FallbackActive {
		t.Fatalf("expected formal secret backend capability, got %+v", service.Capabilities())
	}
	if err := service.SecretStore().PutSecret(context.Background(), SecretRecord{Namespace: "model", Key: "openai_api_key", Value: "secret", UpdatedAt: "2026-04-15T10:00:00Z"}); err != nil {
		t.Fatalf("expected formal secret store write to succeed, got %v", err)
	}
}

func TestServiceFallsBackToDevelopmentStrongholdProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stronghold-fallback-service.db")

	formalProvider := &stubStrongholdProvider{
		backend:  "stronghold",
		fallback: false,
		openErr:  ErrStrongholdUnavailable,
	}
	fallbackProvider := &stubStrongholdProvider{
		backend:  "stronghold_sqlite_fallback",
		fallback: true,
		store:    newInMemorySecretStore(),
	}
	service := newServiceWithStrongholdBootstrap(stubAdapter{databasePath: path}, strongholdBootstrapConfig{
		formalProvider:   func(string) StrongholdProvider { return formalProvider },
		fallbackProvider: func(string) StrongholdProvider { return fallbackProvider },
	})
	defer func() { _ = service.Close() }()

	descriptor := service.Stronghold().Descriptor()
	if descriptor.Backend != "stronghold_sqlite_fallback" || !descriptor.Available || !descriptor.Fallback || !descriptor.Initialized {
		t.Fatalf("expected fallback descriptor after formal bootstrap failure, got %+v", descriptor)
	}
	if !service.Capabilities().FallbackActive || service.Capabilities().SecretStoreBackend != "stronghold_sqlite_fallback" {
		t.Fatalf("expected fallback secret backend capability, got %+v", service.Capabilities())
	}
	if err := service.Validate(); err != nil {
		t.Fatalf("expected fallback-backed storage to remain valid, got %v", err)
	}
	if formalProvider.openCalls != 1 || fallbackProvider.openCalls != 1 {
		t.Fatalf("expected exactly one formal and fallback open attempt, got formal=%d fallback=%d", formalProvider.openCalls, fallbackProvider.openCalls)
	}
	if err := service.SecretStore().PutSecret(context.Background(), SecretRecord{Namespace: "model", Key: "anthropic_api_key", Value: "fallback-secret", UpdatedAt: "2026-04-15T10:00:00Z"}); err != nil {
		t.Fatalf("expected fallback secret store write to succeed, got %v", err)
	}
}

func TestServiceReportsStrongholdBootstrapFailureWhenFallbackUnavailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stronghold-unavailable-service.db")
	service := newServiceWithStrongholdBootstrap(stubAdapter{databasePath: path}, strongholdBootstrapConfig{
		formalProvider: func(string) StrongholdProvider {
			return &stubStrongholdProvider{backend: "stronghold", openErr: ErrStrongholdUnavailable}
		},
		fallbackProvider: func(string) StrongholdProvider {
			return &stubStrongholdProvider{backend: "stronghold_sqlite_fallback", fallback: true, openErr: ErrStrongholdUnavailable}
		},
	})
	defer func() { _ = service.Close() }()

	if err := service.Validate(); err == nil || !strings.Contains(err.Error(), "initialize formal stronghold secret store") || !strings.Contains(err.Error(), "initialize fallback stronghold secret store") {
		t.Fatalf("expected validation to surface both stronghold bootstrap errors, got %v", err)
	}
	if _, err := service.ResolveModelAPIKey("openai"); !errors.Is(err, ErrSecretStoreAccessFailed) {
		t.Fatalf("expected unavailable stronghold bootstrap to surface secret store access failure, got %v", err)
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

func assertTableCount(t *testing.T, db *sql.DB, table string, expected int) {
	t.Helper()
	var count int
	query := "SELECT COUNT(1) FROM " + table
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("query %s count failed: %v", table, err)
	}
	if count != expected {
		t.Fatalf("expected %s count %d, got %d", table, expected, count)
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
