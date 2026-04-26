package storage

import (
	"context"
	"fmt"
	"strings"
)

// ErrStrongholdUnavailable indicates that the formal Stronghold backend cannot
// be opened and callers should decide whether fallback is acceptable.
var ErrStrongholdUnavailable = fmt.Errorf("%w: stronghold backend unavailable", ErrSecretStoreAccessFailed)

// StrongholdSQLiteProvider exposes the formal Stronghold lifecycle boundary on
// top of the dedicated secret-store path. It keeps fallback handling explicit in
// bootstrap/storage wiring instead of advertising the fallback as the primary
// backend.
type StrongholdSQLiteProvider struct {
	databasePath string
	available    bool
	initialized  bool
	lastOpenErr  error
}

// NewStrongholdSQLiteProvider creates the formal Stronghold provider backed by
// the dedicated secret-store file path.
func NewStrongholdSQLiteProvider(databasePath string) *StrongholdSQLiteProvider {
	return &StrongholdSQLiteProvider{
		databasePath: strings.TrimSpace(databasePath),
		available:    strings.TrimSpace(databasePath) != "",
	}
}

func (p *StrongholdSQLiteProvider) Open(ctx context.Context) (SecretStore, error) {
	if p == nil || !p.available || strings.TrimSpace(p.databasePath) == "" {
		if p != nil {
			p.initialized = false
			p.lastOpenErr = ErrStrongholdUnavailable
		}
		return nil, ErrStrongholdUnavailable
	}
	store, err := NewDPAPISecretStore(p.databasePath)
	if err != nil {
		p.initialized = false
		p.lastOpenErr = err
		return nil, fmt.Errorf("%w: %v", ErrStrongholdUnavailable, err)
	}
	select {
	case <-ctx.Done():
		p.initialized = false
		p.lastOpenErr = ctx.Err()
		return nil, ctx.Err()
	default:
	}
	p.initialized = true
	p.lastOpenErr = nil
	return store, nil
}

func (p *StrongholdSQLiteProvider) Descriptor() StrongholdDescriptor {
	available := p != nil && p.available && p.initialized && p.lastOpenErr == nil
	return StrongholdDescriptor{
		Backend:     "stronghold",
		Available:   available,
		Fallback:    false,
		Initialized: p != nil && p.initialized,
	}
}

// StrongholdSQLiteFallbackProvider uses the current SQLite-backed secret store
// as a fallback implementation while preserving a formal Stronghold lifecycle
// boundary for future production Stronghold wiring.
type StrongholdSQLiteFallbackProvider struct {
	databasePath string
	available    bool
	initialized  bool
	lastOpenErr  error
}

// NewStrongholdSQLiteFallbackProvider creates a provider that advertises the
// Stronghold lifecycle contract but falls back to SQLite in development.
func NewStrongholdSQLiteFallbackProvider(databasePath string) *StrongholdSQLiteFallbackProvider {
	return &StrongholdSQLiteFallbackProvider{
		databasePath: strings.TrimSpace(databasePath),
		available:    strings.TrimSpace(databasePath) != "",
	}
}

func (p *StrongholdSQLiteFallbackProvider) Open(ctx context.Context) (SecretStore, error) {
	if p == nil || !p.available || strings.TrimSpace(p.databasePath) == "" {
		if p != nil {
			p.initialized = false
			p.lastOpenErr = ErrStrongholdUnavailable
		}
		return nil, ErrStrongholdUnavailable
	}
	store, err := NewSQLiteSecretStore(p.databasePath)
	if err != nil {
		p.initialized = false
		p.lastOpenErr = err
		return nil, fmt.Errorf("%w: %v", ErrStrongholdUnavailable, err)
	}
	if err := store.EnsureStrongholdMetadata(ctx, "stronghold_sqlite_fallback"); err != nil {
		_ = store.Close()
		p.initialized = false
		p.lastOpenErr = err
		return nil, fmt.Errorf("%w: %v", ErrStrongholdUnavailable, err)
	}
	select {
	case <-ctx.Done():
		_ = store.Close()
		p.initialized = false
		p.lastOpenErr = ctx.Err()
		return nil, ctx.Err()
	default:
	}
	p.initialized = true
	p.lastOpenErr = nil
	return store, nil
}

func (p *StrongholdSQLiteFallbackProvider) Descriptor() StrongholdDescriptor {
	available := p != nil && p.available && p.initialized && p.lastOpenErr == nil
	return StrongholdDescriptor{
		Backend:     "stronghold_sqlite_fallback",
		Available:   available,
		Fallback:    true,
		Initialized: p != nil && p.initialized,
	}
}
