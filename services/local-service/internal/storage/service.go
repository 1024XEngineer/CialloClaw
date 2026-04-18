// 该文件负责存储层的数据接口或落盘实现。
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// backendName 定义当前模块的基础变量。
const backendName = "sqlite_wal"

// ErrAdapterNotConfigured 定义当前模块的基础变量。
var ErrAdapterNotConfigured = errors.New("storage adapter not configured")

// ErrDatabasePathRequired 定义当前模块的基础变量。
var ErrDatabasePathRequired = errors.New("storage database path is required")

// ErrStructuredStoreUnavailable 定义当前模块的基础变量。
var ErrStructuredStoreUnavailable = errors.New("storage structured store unavailable")

// memoryStoreBackendInMemory 定义当前模块的基础变量。
const memoryStoreBackendInMemory = "in_memory"

// memoryStoreBackendSQLite 定义当前模块的基础变量。
const memoryStoreBackendSQLite = "sqlite_wal"

const memoryRetrievalBackendInMemory = "in_memory"
const memoryRetrievalBackendSQLite = "sqlite_fts5+sqlite_vec"

var newSQLiteTraceStoreForService = func(databasePath string) (TraceStore, error) {
	return NewSQLiteTraceStore(databasePath)
}

var newSQLiteEvalStoreForService = func(databasePath string) (EvalStore, error) {
	return NewSQLiteEvalStore(databasePath)
}

// Descriptor 定义当前模块的数据结构。
type Descriptor struct {
	Backend      string
	DatabasePath string
	Configured   bool
	StoreReady   bool
}

// Service 提供当前模块的服务能力。
type Service struct {
	adapter                  platform.StorageAdapter
	memoryStore              MemoryStore
	taskRunStore             TaskRunStore
	toolCallStore            ToolCallStore
	loopRuntimeStore         LoopRuntimeStore
	artifactStore            ArtifactStore
	todoStore                TodoStore
	traceStore               TraceStore
	evalStore                EvalStore
	secretStore              SecretStore
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

// NewService 创建并返回Service。
func NewService(adapter platform.StorageAdapter) *Service {
	memoryStore := MemoryStore(NewInMemoryMemoryStore())
	taskRunStore := TaskRunStore(NewInMemoryTaskRunStore())
	toolCallStore := ToolCallStore(newInMemoryToolCallStore())
	loopRuntimeStore := LoopRuntimeStore(newInMemoryLoopRuntimeStore())
	artifactStore := ArtifactStore(newInMemoryArtifactStore())
	todoStore := TodoStore(NewInMemoryTodoStore())
	traceStore := TraceStore(newInMemoryTraceStore())
	evalStore := EvalStore(newInMemoryEvalStore())
	secretStore := SecretStore(newInMemorySecretStore())
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

			sqliteTraceStore, sqliteEvalStore, err := initializeSQLiteTraceEvalStores(databasePath)
			if err == nil {
				traceStore = sqliteTraceStore
				evalStore = sqliteEvalStore
			}
			if err != nil {
				storeInitErrors = append(storeInitErrors, err)
				fallbackActive = true
			}

			if secretPath := strings.TrimSpace(adapter.SecretStorePath()); secretPath != "" {
				sqliteSecretStore, err := NewSQLiteSecretStore(secretPath)
				if err == nil {
					secretStore = sqliteSecretStore
					secretStoreName = memoryStoreBackendSQLite
				}
				if err != nil {
					storeInitErrors = append(storeInitErrors, fmt.Errorf("initialize stronghold secret store: %w", err))
					fallbackActive = true
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
		artifactStore:            artifactStore,
		todoStore:                todoStore,
		traceStore:               traceStore,
		evalStore:                evalStore,
		secretStore:              secretStore,
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

// Backend 处理当前模块的相关逻辑。
func (s *Service) Backend() string {
	return backendName
}

// DatabasePath 处理当前模块的相关逻辑。
func (s *Service) DatabasePath() string {
	if s.adapter == nil {
		return ""
	}

	return strings.TrimSpace(s.adapter.DatabasePath())
}

// Configured 处理当前模块的相关逻辑。
func (s *Service) Configured() bool {
	return s.adapter != nil && s.DatabasePath() != ""
}

// Validate 处理当前模块的相关逻辑。
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

// Descriptor 处理当前模块的相关逻辑。
func (s *Service) Descriptor() Descriptor {
	return Descriptor{
		Backend:      s.Backend(),
		DatabasePath: s.DatabasePath(),
		Configured:   s.Configured(),
		StoreReady:   s.storeInitErr == nil,
	}
}

// Capabilities 处理当前模块的相关逻辑。
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

// MemoryStore 处理当前模块的相关逻辑。
func (s *Service) MemoryStore() MemoryStore {
	return s.memoryStore
}

func (s *Service) TaskRunStore() TaskRunStore {
	return s.taskRunStore
}

func (s *Service) ToolCallSink() tools.ToolCallSink {
	return s.toolCallStore
}

// LoopRuntimeStore returns the normalized loop runtime persistence store.
func (s *Service) LoopRuntimeStore() LoopRuntimeStore {
	return s.loopRuntimeStore
}

// ArtifactStore returns the configured artifact store.
func (s *Service) ArtifactStore() ArtifactStore {
	return s.artifactStore
}

// TodoStore returns the configured notes/todo persistence store.
func (s *Service) TodoStore() TodoStore {
	return s.todoStore
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
	record, err := s.secretStore.GetSecret(context.Background(), "model", strings.TrimSpace(provider)+"_api_key")
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

// Close 处理当前模块的相关逻辑。
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
	if closer, ok := s.artifactStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.todoStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.traceStore.(interface{ Close() error }); ok {
		errs = append(errs, closer.Close())
	}
	if closer, ok := s.evalStore.(interface{ Close() error }); ok {
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
