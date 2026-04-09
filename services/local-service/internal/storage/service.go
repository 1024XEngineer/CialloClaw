// 该文件负责存储层的数据接口或落盘实现。
package storage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
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

// Descriptor 定义当前模块的数据结构。
type Descriptor struct {
	Backend      string
	DatabasePath string
	Configured   bool
	StoreReady   bool
}

// Service 提供当前模块的服务能力。
type Service struct {
	adapter          platform.StorageAdapter
	memoryStore      MemoryStore
	memoryStoreName  string
	retrievalBackend string
	storeInitErr     error
	fallbackActive   bool
}

// NewService 创建并返回Service。
func NewService(adapter platform.StorageAdapter) *Service {
	memoryStore := MemoryStore(NewInMemoryMemoryStore())
	memoryStoreName := memoryStoreBackendInMemory
	retrievalBackend := memoryRetrievalBackendInMemory
	var storeInitErr error
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
				storeInitErr = fmt.Errorf("initialize sqlite memory store: %w", err)
				fallbackActive = true
			}
		}
	}

	return &Service{
		adapter:          adapter,
		memoryStore:      memoryStore,
		memoryStoreName:  memoryStoreName,
		retrievalBackend: retrievalBackend,
		storeInitErr:     storeInitErr,
		fallbackActive:   fallbackActive,
	}
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
	structuredReady := configured && s.storeInitErr == nil && s.memoryStoreName == memoryStoreBackendSQLite

	return CapabilitySnapshot{
		Backend:                s.Backend(),
		Configured:             configured,
		SupportsStructuredData: structuredReady,
		SupportsMemoryStore:    s.memoryStore != nil,
		SupportsRetrievalHits:  s.memoryStore != nil,
		SupportsFTS5:           structuredReady,
		SupportsSQLiteVecStub:  structuredReady,
		SupportsArtifactStore:  false,
		SupportsSecretStore:    false,
		MemoryStoreBackend:     s.memoryStoreName,
		MemoryRetrievalBackend: s.retrievalBackend,
		FallbackActive:         s.fallbackActive,
	}
}

// MemoryStore 处理当前模块的相关逻辑。
func (s *Service) MemoryStore() MemoryStore {
	return s.memoryStore
}

// Close 处理当前模块的相关逻辑。
func (s *Service) Close() error {
	if closer, ok := s.memoryStore.(interface{ Close() error }); ok {
		return closer.Close()
	}

	return nil
}
