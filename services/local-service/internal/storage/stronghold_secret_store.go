//go:build windows

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DPAPISecretStore is the formal Stronghold-backed secret store. It persists a
// single encrypted payload outside the normal SQLite settings/runtime path so
// the formal Stronghold lifecycle no longer shares the fallback implementation.
type DPAPISecretStore struct {
	mu     sync.Mutex
	path   string
	closed bool
}

// NewDPAPISecretStore creates the formal file-backed secret store used by the
// Stronghold provider on supported platforms.
func NewDPAPISecretStore(databasePath string) (*DPAPISecretStore, error) {
	trimmed := strings.TrimSpace(databasePath)
	if trimmed == "" {
		return nil, ErrStrongholdUnavailable
	}
	if err := ensureStrongholdPlatformSupport(); err != nil {
		return nil, err
	}
	cleaned := filepath.Clean(trimmed)
	if err := os.MkdirAll(filepath.Dir(cleaned), 0o755); err != nil {
		return nil, fmt.Errorf("prepare stronghold directory: %w", err)
	}
	return &DPAPISecretStore{path: cleaned}, nil
}

func (s *DPAPISecretStore) PutSecret(_ context.Context, record SecretRecord) error {
	if err := validateSecretRecord(record); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrSecretStoreAccessFailed
	}
	payload, err := s.loadPayloadLocked()
	if err != nil {
		return err
	}
	payload.Records[secretStoreKey(record.Namespace, record.Key)] = record
	return s.savePayloadLocked(payload)
}

func (s *DPAPISecretStore) GetSecret(_ context.Context, namespace, key string) (SecretRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return SecretRecord{}, ErrSecretStoreAccessFailed
	}
	payload, err := s.loadPayloadLocked()
	if err != nil {
		return SecretRecord{}, err
	}
	record, ok := payload.Records[secretStoreKey(namespace, key)]
	if !ok {
		return SecretRecord{}, ErrSecretNotFound
	}
	return record, nil
}

func (s *DPAPISecretStore) DeleteSecret(_ context.Context, namespace, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrSecretStoreAccessFailed
	}
	payload, err := s.loadPayloadLocked()
	if err != nil {
		return err
	}
	delete(payload.Records, secretStoreKey(namespace, key))
	return s.savePayloadLocked(payload)
}

// Close marks the formal Stronghold store unavailable for further access.
func (s *DPAPISecretStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *DPAPISecretStore) loadPayloadLocked() (strongholdFilePayload, error) {
	payload := strongholdFilePayload{Backend: "stronghold", Records: map[string]SecretRecord{}}
	rawBytes, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return payload, nil
		}
		return strongholdFilePayload{}, fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	if len(rawBytes) == 0 {
		return payload, nil
	}
	decoded, err := unprotectStrongholdBytes(rawBytes)
	if err != nil {
		return strongholdFilePayload{}, fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return strongholdFilePayload{}, fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	if payload.Records == nil {
		payload.Records = map[string]SecretRecord{}
	}
	if strings.TrimSpace(payload.Backend) == "" {
		payload.Backend = "stronghold"
	}
	return payload, nil
}

func (s *DPAPISecretStore) savePayloadLocked(payload strongholdFilePayload) error {
	if payload.Records == nil {
		payload.Records = map[string]SecretRecord{}
	}
	payload.Backend = "stronghold"
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	protected, err := protectStrongholdBytes(encoded)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	if err := os.WriteFile(s.path, protected, 0o600); err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	return nil
}
