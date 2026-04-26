//go:build !windows

package storage

import "context"

// DPAPISecretStore is the non-Windows stub for the formal Stronghold store.
// It intentionally reports Stronghold unavailability on unsupported platforms.
type DPAPISecretStore struct{}

// NewDPAPISecretStore reports Stronghold unavailability on non-Windows hosts.
func NewDPAPISecretStore(string) (*DPAPISecretStore, error) {
	return nil, ErrStrongholdUnavailable
}

func (s *DPAPISecretStore) PutSecret(context.Context, SecretRecord) error {
	_ = s
	return ErrStrongholdUnavailable
}

func (s *DPAPISecretStore) GetSecret(context.Context, string, string) (SecretRecord, error) {
	_ = s
	return SecretRecord{}, ErrStrongholdUnavailable
}

func (s *DPAPISecretStore) DeleteSecret(context.Context, string, string) error {
	_ = s
	return ErrStrongholdUnavailable
}

func (s *DPAPISecretStore) Close() error {
	_ = s
	return nil
}

func (s *DPAPISecretStore) loadPayloadLocked() (strongholdFilePayload, error) {
	_ = s
	return strongholdFilePayload{}, ErrStrongholdUnavailable
}

func (s *DPAPISecretStore) savePayloadLocked(strongholdFilePayload) error {
	_ = s
	return ErrStrongholdUnavailable
}

func protectStrongholdBytes([]byte) ([]byte, error) {
	return nil, ErrStrongholdUnavailable
}

func unprotectStrongholdBytes([]byte) ([]byte, error) {
	return nil, ErrStrongholdUnavailable
}
