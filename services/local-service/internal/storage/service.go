package storage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
)

const backendName = "sqlite_wal"

var ErrAdapterNotConfigured = errors.New("storage adapter not configured")
var ErrDatabasePathRequired = errors.New("storage database path is required")
var ErrStructuredStoreUnavailable = errors.New("storage structured store unavailable")

const memoryStoreBackendInMemory = "in_memory"
const memoryStoreBackendSQLite = "sqlite_wal"

type Descriptor struct {
	Backend      string
	DatabasePath string
	Configured   bool
	StoreReady   bool
}

type Service struct {
	adapter         platform.StorageAdapter
	memoryStore     MemoryStore
	memoryStoreName string
	storeInitErr    error
	fallbackActive  bool
}

func NewService(adapter platform.StorageAdapter) *Service {
	memoryStore := MemoryStore(NewInMemoryMemoryStore())
	memoryStoreName := memoryStoreBackendInMemory
	var storeInitErr error
	fallbackActive := false

	if adapter != nil {
		if databasePath := strings.TrimSpace(adapter.DatabasePath()); databasePath != "" {
			sqliteStore, err := NewSQLiteMemoryStore(databasePath)
			if err == nil {
				memoryStore = sqliteStore
				memoryStoreName = memoryStoreBackendSQLite
			}
			if err != nil {
				storeInitErr = fmt.Errorf("initialize sqlite memory store: %w", err)
				fallbackActive = true
			}
		}
	}

	return &Service{
		adapter:         adapter,
		memoryStore:     memoryStore,
		memoryStoreName: memoryStoreName,
		storeInitErr:    storeInitErr,
		fallbackActive:  fallbackActive,
	}
}

func (s *Service) Backend() string {
	return backendName
}

func (s *Service) DatabasePath() string {
	if s.adapter == nil {
		return ""
	}

	return strings.TrimSpace(s.adapter.DatabasePath())
}

func (s *Service) Configured() bool {
	return s.adapter != nil && s.DatabasePath() != ""
}

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

func (s *Service) Descriptor() Descriptor {
	return Descriptor{
		Backend:      s.Backend(),
		DatabasePath: s.DatabasePath(),
		Configured:   s.Configured(),
		StoreReady:   s.storeInitErr == nil,
	}
}

func (s *Service) Capabilities() CapabilitySnapshot {
	configured := s.Configured()
	structuredReady := configured && s.storeInitErr == nil && s.memoryStoreName == memoryStoreBackendSQLite

	return CapabilitySnapshot{
		Backend:                s.Backend(),
		Configured:             configured,
		SupportsStructuredData: structuredReady,
		SupportsMemoryStore:    s.memoryStore != nil,
		SupportsArtifactStore:  false,
		SupportsSecretStore:    false,
		MemoryStoreBackend:     s.memoryStoreName,
		FallbackActive:         s.fallbackActive,
	}
}

func (s *Service) MemoryStore() MemoryStore {
	return s.memoryStore
}

func (s *Service) Close() error {
	if closer, ok := s.memoryStore.(interface{ Close() error }); ok {
		return closer.Close()
	}

	return nil
}
