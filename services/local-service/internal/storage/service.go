// Package storage wires the formal persistence backends used by local-service.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// backendName identifies the formal storage backend.
const backendName = "sqlite_wal"

// ErrAdapterNotConfigured reports that no storage adapter was wired.
var ErrAdapterNotConfigured = errors.New("storage adapter not configured")

// ErrDatabasePathRequired reports that the storage backend needs a database path.
var ErrDatabasePathRequired = errors.New("storage database path is required")

// ErrStructuredStoreUnavailable reports that structured persistence could not be initialized.
var ErrStructuredStoreUnavailable = errors.New("storage structured store unavailable")

// memoryStoreBackendInMemory identifies the in-memory fallback backend.
const memoryStoreBackendInMemory = "in_memory"

// memoryStoreBackendSQLite identifies the SQLite-backed formal store.
const memoryStoreBackendSQLite = "sqlite_wal"

const memoryRetrievalBackendInMemory = "in_memory"
const memoryRetrievalBackendSQLite = "sqlite_fts5+sqlite_vec"

var newSQLiteTraceStoreForService = func(databasePath string) (TraceStore, error) {
	return NewSQLiteTraceStore(databasePath)
}

var newSQLiteEvalStoreForService = func(databasePath string) (EvalStore, error) {
	return NewSQLiteEvalStore(databasePath)
}

// strongholdBootstrapConfig keeps Stronghold provider injection scoped to one
// service bootstrap so tests can validate formal/fallback semantics without
// mutating shared package state.
type strongholdBootstrapConfig struct {
	formalProvider   func(databasePath string) StrongholdProvider
	fallbackProvider func(databasePath string) StrongholdProvider
}

func defaultStrongholdBootstrapConfig() strongholdBootstrapConfig {
	return strongholdBootstrapConfig{
		formalProvider: func(databasePath string) StrongholdProvider {
			return NewStrongholdSQLiteProvider(databasePath)
		},
		fallbackProvider: func(databasePath string) StrongholdProvider {
			return NewStrongholdSQLiteFallbackProvider(databasePath)
		},
	}
}

// Descriptor captures the configured storage backend state.
type Descriptor struct {
	Backend      string
	DatabasePath string
	Configured   bool
	StoreReady   bool
}

// Service exposes the composed storage dependencies for local-service.
type Service struct {
	adapter                  platform.StorageAdapter
	memoryStore              MemoryStore
	taskRunStore             TaskRunStore
	toolCallStore            ToolCallStore
	loopRuntimeStore         LoopRuntimeStore
	sessionStore             SessionStore
	taskStore                TaskStore
	taskStepStore            TaskStepStore
	artifactStore            ArtifactStore
	todoStore                TodoStore
	settingsStore            SettingsStore
	traceStore               TraceStore
	evalStore                EvalStore
	skillManifestStore       SkillManifestStore
	blueprintDefinitionStore BlueprintDefinitionStore
	promptTemplateStore      PromptTemplateVersionStore
	pluginManifestStore      PluginManifestStore
	secretStore              SecretStore
	stronghold               StrongholdProvider
	auditStore               AuditStore
	recoveryPointStore       RecoveryPointStore
	approvalRequestStore     ApprovalRequestStore
	authorizationRecordStore AuthorizationRecordStore
	memoryStoreName          string
	taskRunStoreName         string
	toolCallStoreName        string
	artifactStoreName        string
	secretStoreName          string
	retrievalBackend         string
	storeInitErr             error
	fallbackActive           bool
}

func NewService(adapter platform.StorageAdapter) *Service {
	return newServiceWithStrongholdBootstrap(adapter, defaultStrongholdBootstrapConfig())
}

func newServiceWithStrongholdBootstrap(adapter platform.StorageAdapter, bootstrap strongholdBootstrapConfig) *Service {
	if bootstrap.formalProvider == nil || bootstrap.fallbackProvider == nil {
		defaults := defaultStrongholdBootstrapConfig()
		if bootstrap.formalProvider == nil {
			bootstrap.formalProvider = defaults.formalProvider
		}
		if bootstrap.fallbackProvider == nil {
			bootstrap.fallbackProvider = defaults.fallbackProvider
		}
	}

	memoryStore := MemoryStore(NewInMemoryMemoryStore())
	toolCallStore := ToolCallStore(newInMemoryToolCallStore())
	loopRuntimeStore := LoopRuntimeStore(newInMemoryLoopRuntimeStore())
	sessionStore := SessionStore(newInMemorySessionStore())
	taskStore := TaskStore(newInMemoryTaskStore())
	taskStepStore := TaskStepStore(newInMemoryTaskStepStore())
	taskRunStore := TaskRunStore(NewInMemoryTaskRunStore().WithStructuredStores(taskStore, taskStepStore))
	artifactStore := ArtifactStore(newInMemoryArtifactStore())
	todoStore := TodoStore(NewInMemoryTodoStore())
	settingsStore := SettingsStore(newInMemorySettingsStore())
	traceStore := TraceStore(newInMemoryTraceStore())
	evalStore := EvalStore(newInMemoryEvalStore())
	skillManifestStore := SkillManifestStore(newInMemorySkillManifestStore())
	blueprintDefinitionStore := BlueprintDefinitionStore(newInMemoryBlueprintDefinitionStore())
	promptTemplateStore := PromptTemplateVersionStore(newInMemoryPromptTemplateVersionStore())
	pluginManifestStore := PluginManifestStore(newInMemoryPluginManifestStore())
	secretStore := SecretStore(newInMemorySecretStore())
	strongholdProvider := StrongholdProvider(nil)
	auditStore := AuditStore(newInMemoryAuditStore())
	recoveryPointStore := RecoveryPointStore(newInMemoryRecoveryPointStore())
	governanceState := &inMemoryGovernanceState{
		approvalRequests:     make([]ApprovalRequestRecord, 0),
		authorizationRecords: make([]AuthorizationRecordRecord, 0),
	}
	approvalRequestStore := ApprovalRequestStore(newInMemoryApprovalRequestStoreWithState(governanceState))
	authorizationRecordStore := AuthorizationRecordStore(newInMemoryAuthorizationRecordStoreWithState(governanceState))
	memoryStoreName := memoryStoreBackendInMemory
	taskRunStoreName := memoryStoreBackendInMemory
	toolCallStoreName := memoryStoreBackendInMemory
	artifactStoreName := memoryStoreBackendInMemory
	secretStoreName := memoryStoreBackendInMemory
	retrievalBackend := memoryRetrievalBackendInMemory
	storeInitErrors := make([]error, 0, 2)
	fallbackActive := false

	if adapter != nil {
		if databasePath := strings.TrimSpace(adapter.DatabasePath()); databasePath != "" {
			sqliteStore, err := NewSQLiteMemoryStore(databasePath)
			if err == nil {
				memoryStore = sqliteStore
				memoryStoreName = memoryStoreBackendSQLite
				retrievalBackend = memoryRetrievalBackendSQLite
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite memory store: %w", err))
				fallbackActive = true
			}

			sqliteTaskRunStore, err := NewSQLiteTaskRunStore(databasePath)
			if err == nil {
				taskRunStore = sqliteTaskRunStore
				taskRunStoreName = memoryStoreBackendSQLite
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite task_run store: %w", err))
				fallbackActive = true
			}

			sqliteToolCallStore, err := NewSQLiteToolCallStore(databasePath)
			if err == nil {
				toolCallStore = sqliteToolCallStore
				toolCallStoreName = memoryStoreBackendSQLite
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite tool_call store: %w", err))
				fallbackActive = true
			}

			sqliteLoopRuntimeStore, err := NewSQLiteLoopRuntimeStore(databasePath)
			if err == nil {
				loopRuntimeStore = sqliteLoopRuntimeStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite loop runtime store: %w", err))
				fallbackActive = true
			}

			sqliteSessionStore, err := NewSQLiteSessionStore(databasePath)
			if err == nil {
				sessionStore = sqliteSessionStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite session store: %w", err))
				fallbackActive = true
			}

			sqliteTaskStore, err := NewSQLiteTaskStore(databasePath)
			if err == nil {
				taskStore = sqliteTaskStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite task store: %w", err))
				fallbackActive = true
			}

			sqliteTaskStepStore, err := NewSQLiteTaskStepStore(databasePath)
			if err == nil {
				taskStepStore = sqliteTaskStepStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite task_step store: %w", err))
				fallbackActive = true
			}

			sqliteArtifactStore, err := NewSQLiteArtifactStore(databasePath)
			if err == nil {
				artifactStore = sqliteArtifactStore
				artifactStoreName = memoryStoreBackendSQLite
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite artifact store: %w", err))
				fallbackActive = true
			}

			sqliteTodoStore, err := NewSQLiteTodoStore(databasePath)
			if err == nil {
				todoStore = sqliteTodoStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite todo store: %w", err))
				fallbackActive = true
			}

			sqliteSettingsStore, err := NewSQLiteSettingsStore(databasePath)
			if err == nil {
				settingsStore = sqliteSettingsStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite settings store: %w", err))
				fallbackActive = true
			}

			sqliteTraceStore, sqliteEvalStore, err := initializeSQLiteTraceEvalStores(databasePath)
			if err == nil {
				traceStore = sqliteTraceStore
				evalStore = sqliteEvalStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, err)
				fallbackActive = true
			}

			sqliteSkillManifestStore, err := NewSQLiteSkillManifestStore(databasePath)
			if err == nil {
				skillManifestStore = sqliteSkillManifestStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite skill manifest store: %w", err))
				fallbackActive = true
			}

			sqliteBlueprintDefinitionStore, err := NewSQLiteBlueprintDefinitionStore(databasePath)
			if err == nil {
				blueprintDefinitionStore = sqliteBlueprintDefinitionStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite blueprint definition store: %w", err))
				fallbackActive = true
			}

			sqlitePromptTemplateStore, err := NewSQLitePromptTemplateVersionStore(databasePath)
			if err == nil {
				promptTemplateStore = sqlitePromptTemplateStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite prompt template store: %w", err))
				fallbackActive = true
			}

			sqlitePluginManifestStore, err := NewSQLitePluginManifestStore(databasePath)
			if err == nil {
				pluginManifestStore = sqlitePluginManifestStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite plugin manifest store: %w", err))
				fallbackActive = true
			}

			if secretPath := strings.TrimSpace(adapter.SecretStorePath()); secretPath != "" {
				formalStrongholdProvider := bootstrap.formalProvider(secretPath)
				strongholdProvider = formalStrongholdProvider
				strongholdStore, err := formalStrongholdProvider.Open(context.Background())
				if err == nil {
					secretStore = strongholdStore
					secretStoreName = formalStrongholdProvider.Descriptor().Backend
				}
				if err != nil {
					formalStrongholdErr := fmt.Errorf("initialize formal stronghold secret store: %w", err)
					fallbackProvider := bootstrap.fallbackProvider(secretPath)
					fallbackStore, fallbackErr := fallbackProvider.Open(context.Background())
					if fallbackErr == nil {
						// A working fallback keeps the overall storage service healthy in
						// CI and unsupported environments. The formal backend failure is
						// still exposed through fallbackActive/descriptor state instead
						// of poisoning Validate() for the entire structured store.
						strongholdProvider = fallbackProvider
						secretStore = fallbackStore
						secretStoreName = fallbackProvider.Descriptor().Backend
						fallbackActive = true
					} else {
						secretStore = SecretStore(UnavailableSecretStore{})
						secretStoreName = formalStrongholdProvider.Descriptor().Backend
						storeInitErrors = append(storeInitErrors, formalStrongholdErr)
						storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize fallback stronghold secret store: %w", fallbackErr))
						fallbackActive = true
					}
				}
			}

			sqliteAuditStore, err := NewSQLiteAuditStore(databasePath)
			if err == nil {
				auditStore = sqliteAuditStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite audit store: %w", err))
				fallbackActive = true
			}

			sqliteRecoveryPointStore, err := NewSQLiteRecoveryPointStore(databasePath)
			if err == nil {
				recoveryPointStore = sqliteRecoveryPointStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite recovery point store: %w", err))
				fallbackActive = true
			}

			sqliteApprovalRequestStore, err := NewSQLiteApprovalRequestStore(databasePath)
			if err == nil {
				approvalRequestStore = sqliteApprovalRequestStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite approval request store: %w", err))
				fallbackActive = true
			}

			sqliteAuthorizationRecordStore, err := NewSQLiteAuthorizationRecordStore(databasePath)
			if err == nil {
				authorizationRecordStore = sqliteAuthorizationRecordStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize sqlite authorization record store: %w", err))
				fallbackActive = true
			}
		}
	}

	storeInitErr := errors.Join(storeInitErrors...)

	return &Service{
		adapter:                  adapter,
		memoryStore:              memoryStore,
		taskRunStore:             taskRunStore,
		toolCallStore:            toolCallStore,
		loopRuntimeStore:         loopRuntimeStore,
		sessionStore:             sessionStore,
		taskStore:                taskStore,
		taskStepStore:            taskStepStore,
		artifactStore:            artifactStore,
		todoStore:                todoStore,
		settingsStore:            settingsStore,
		traceStore:               traceStore,
		evalStore:                evalStore,
		skillManifestStore:       skillManifestStore,
		blueprintDefinitionStore: blueprintDefinitionStore,
		promptTemplateStore:      promptTemplateStore,
		pluginManifestStore:      pluginManifestStore,
		secretStore:              secretStore,
		stronghold:               strongholdProvider,
		auditStore:               auditStore,
		recoveryPointStore:       recoveryPointStore,
		approvalRequestStore:     approvalRequestStore,
		authorizationRecordStore: authorizationRecordStore,
		memoryStoreName:          memoryStoreName,
		taskRunStoreName:         taskRunStoreName,
		toolCallStoreName:        toolCallStoreName,
		artifactStoreName:        artifactStoreName,
		secretStoreName:          secretStoreName,
		retrievalBackend:         retrievalBackend,
		storeInitErr:             storeInitErr,
		fallbackActive:           fallbackActive,
	}
}

// Stronghold returns the configured formal secret backend lifecycle provider.
func (s *Service) Stronghold() StrongholdProvider {
	return s.stronghold
}

func initializeSQLiteTraceEvalStores(databasePath string) (TraceStore, EvalStore, error) {
	traceStore, err := newSQLiteTraceStoreForService(databasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize sqlite trace/eval stores: trace store: %w", err)
	}
	evalStore, err := newSQLiteEvalStoreForService(databasePath)
	if err != nil {
		if closer, ok := traceStore.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return nil, nil, fmt.Errorf("initialize sqlite trace/eval stores: eval store: %w", err)
	}
	return traceStore, evalStore, nil
}

// TraceStore returns the configured trace persistence store.
func (s *Service) TraceStore() TraceStore {
	return s.traceStore
}

// EvalStore returns the configured eval snapshot persistence store.
func (s *Service) EvalStore() EvalStore {
	return s.evalStore
}

// SkillManifestStore returns the configured skill manifest asset store.
func (s *Service) SkillManifestStore() SkillManifestStore {
	return s.skillManifestStore
}

// BlueprintDefinitionStore returns the configured blueprint definition asset store.
func (s *Service) BlueprintDefinitionStore() BlueprintDefinitionStore {
	return s.blueprintDefinitionStore
}

// PromptTemplateVersionStore returns the configured prompt template asset store.
func (s *Service) PromptTemplateVersionStore() PromptTemplateVersionStore {
	return s.promptTemplateStore
}

// PluginManifestStore returns the configured plugin manifest asset store.
func (s *Service) PluginManifestStore() PluginManifestStore {
	return s.pluginManifestStore
}

// Backend returns the configured storage backend name.
func (s *Service) Backend() string {
	return backendName
}

// DatabasePath returns the backing database path.
func (s *Service) DatabasePath() string {
	if s.adapter == nil {
		return ""
	}

	return strings.TrimSpace(s.adapter.DatabasePath())
}

// Configured reports whether a concrete storage backend is configured.
func (s *Service) Configured() bool {
	return s.adapter != nil && s.DatabasePath() != ""
}

// Validate checks whether the configured storage backend is ready for use.
func (s *Service) Validate() error {
	if s.adapter == nil {
		return ErrAdapterNotConfigured
	}

	if s.DatabasePath() == "" {
		return ErrDatabasePathRequired
	}

	if s.storeInitErr != nil {
		return fmt.Errorf("%w: %v", ErrStructuredStoreUnavailable, s.storeInitErr)
	}

	return nil
}

// Descriptor returns a snapshot of the storage backend state.
func (s *Service) Descriptor() Descriptor {
	return Descriptor{
		Backend:      s.Backend(),
		DatabasePath: s.DatabasePath(),
		Configured:   s.Configured(),
		StoreReady:   s.storeInitErr == nil,
	}
}

// Capabilities returns the storage capability snapshot.
func (s *Service) Capabilities() CapabilitySnapshot {
	configured := s.Configured()
	structuredReady := configured && s.storeInitErr == nil && s.memoryStoreName == memoryStoreBackendSQLite && s.taskRunStoreName == memoryStoreBackendSQLite

	return CapabilitySnapshot{
		Backend:                s.Backend(),
		Configured:             configured,
		SupportsStructuredData: structuredReady,
		SupportsMemoryStore:    s.memoryStore != nil,
		SupportsToolCallSink:   s.toolCallStore != nil,
		SupportsRetrievalHits:  s.memoryStore != nil,
		SupportsFTS5:           structuredReady,
		SupportsSQLiteVecStub:  structuredReady,
		SupportsArtifactStore:  s.artifactStore != nil,
		SupportsLoopRuntime:    s.loopRuntimeStore != nil,
		SupportsSecretStore:    s.secretStore != nil,
		MemoryStoreBackend:     s.memoryStoreName,
		ToolCallStoreBackend:   s.toolCallStoreName,
		ArtifactStoreBackend:   s.artifactStoreName,
		SecretStoreBackend:     s.secretStoreName,
		MemoryRetrievalBackend: s.retrievalBackend,
		FallbackActive:         s.fallbackActive,
	}
}

// MemoryStore returns the configured memory persistence store.
func (s *Service) MemoryStore() MemoryStore {
	return s.memoryStore
}

func (s *Service) TaskRunStore() TaskRunStore {
	return s.taskRunStore
}

func (s *Service) ToolCallSink() tools.ToolCallSink {
	return s.toolCallStore
}

// ToolCallStore returns the configured tool_call persistence store.
func (s *Service) ToolCallStore() ToolCallStore {
	return s.toolCallStore
}

// LoopRuntimeStore returns the normalized loop runtime persistence store.
func (s *Service) LoopRuntimeStore() LoopRuntimeStore {
	return s.loopRuntimeStore
}

// SessionStore returns the configured first-class sessions store.
func (s *Service) SessionStore() SessionStore {
	return s.sessionStore
}

// TaskStore returns the configured first-class tasks store.
func (s *Service) TaskStore() TaskStore {
	return s.taskStore
}

// TaskStepStore returns the configured first-class task_steps store.
func (s *Service) TaskStepStore() TaskStepStore {
	return s.taskStepStore
}

// ArtifactStore returns the configured artifact store.
func (s *Service) ArtifactStore() ArtifactStore {
	return s.artifactStore
}

// TodoStore returns the configured notes/todo persistence store.
func (s *Service) TodoStore() TodoStore {
	return s.todoStore
}

// SettingsStore returns the configured ordinary settings persistence store.
func (s *Service) SettingsStore() SettingsStore {
	return s.settingsStore
}

// SecretStore returns the configured secret store.
func (s *Service) SecretStore() SecretStore {
	return s.secretStore
}

// ResolveModelAPIKey returns one model provider API key from the dedicated secret store.
func (s *Service) ResolveModelAPIKey(provider string) (string, error) {
	if s.secretStore == nil {
		return "", ErrSecretStoreAccessFailed
	}
	canonicalProvider := model.CanonicalProviderName(provider)
	record, err := s.secretStore.GetSecret(context.Background(), "model", strings.TrimSpace(canonicalProvider)+"_api_key")
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return "", err
		}
		return "", err
	}
	return secretRecordValue(record), nil
}

func (s *Service) AuditWriter() audit.Writer {
	return s.auditStore
}

func (s *Service) AuditStore() AuditStore {
	return s.auditStore
}

func (s *Service) RecoveryPointWriter() checkpoint.Writer {
	return s.recoveryPointStore
}

func (s *Service) RecoveryPointStore() RecoveryPointStore {
	return s.recoveryPointStore
}

// ApprovalRequestStore returns the configured approval_request persistence store.
func (s *Service) ApprovalRequestStore() ApprovalRequestStore {
	return s.approvalRequestStore
}

// AuthorizationRecordStore returns the configured authorization_record persistence store.
func (s *Service) AuthorizationRecordStore() AuthorizationRecordStore {
	return s.authorizationRecordStore
}

// Close releases all configured storage handles.
func (s *Service) Close() error {
	errs := make([]error, 0, 2)
	if closer, ok := s.memoryStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.taskRunStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.toolCallStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.loopRuntimeStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.sessionStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.taskStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.taskStepStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.artifactStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.todoStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.settingsStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.traceStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.evalStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.skillManifestStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.blueprintDefinitionStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.promptTemplateStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.pluginManifestStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.secretStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.auditStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.recoveryPointStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.approvalRequestStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.authorizationRecordStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}

	return errors.Join(errs...)
}
