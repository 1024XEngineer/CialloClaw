package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestInMemorySecretStoreRoundTrip(t *testing.T) {
	store := newInMemorySecretStore()
	record := SecretRecord{
		Namespace: "model",
		Key:       "openai_responses_api_key",
		Value:     "secret-key",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := store.PutSecret(context.Background(), record); err != nil {
		t.Fatalf("PutSecret returned error: %v", err)
	}
	resolved, err := store.GetSecret(context.Background(), record.Namespace, record.Key)
	if err != nil {
		t.Fatalf("GetSecret returned error: %v", err)
	}
	if resolved.Value != record.Value {
		t.Fatalf("unexpected secret value: %+v", resolved)
	}
	if err := store.DeleteSecret(context.Background(), record.Namespace, record.Key); err != nil {
		t.Fatalf("DeleteSecret returned error: %v", err)
	}
	if _, err := store.GetSecret(context.Background(), record.Namespace, record.Key); err != ErrSecretNotFound {
		t.Fatalf("expected ErrSecretNotFound after delete, got %v", err)
	}
}

func TestSQLiteSecretStoreRoundTrip(t *testing.T) {
	store, err := NewSQLiteSecretStore(filepath.Join(t.TempDir(), "stronghold.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSecretStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()
	record := SecretRecord{
		Namespace: "model",
		Key:       "openai_responses_api_key",
		Value:     "secret-key",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := store.PutSecret(context.Background(), record); err != nil {
		t.Fatalf("PutSecret returned error: %v", err)
	}
	resolved, err := store.GetSecret(context.Background(), record.Namespace, record.Key)
	if err != nil {
		t.Fatalf("GetSecret returned error: %v", err)
	}
	if resolved.Value != record.Value {
		t.Fatalf("unexpected sqlite secret value: %+v", resolved)
	}
	record.Value = "rotated-key"
	record.UpdatedAt = time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	if err := store.PutSecret(context.Background(), record); err != nil {
		t.Fatalf("PutSecret replacement returned error: %v", err)
	}
	resolved, err = store.GetSecret(context.Background(), record.Namespace, record.Key)
	if err != nil {
		t.Fatalf("GetSecret after replace returned error: %v", err)
	}
	if resolved.Value != "rotated-key" {
		t.Fatalf("expected rotated value, got %+v", resolved)
	}
	if err := store.DeleteSecret(context.Background(), record.Namespace, record.Key); err != nil {
		t.Fatalf("DeleteSecret returned error: %v", err)
	}
	if _, err := store.GetSecret(context.Background(), record.Namespace, record.Key); err != ErrSecretNotFound {
		t.Fatalf("expected ErrSecretNotFound after delete, got %v", err)
	}
}

func TestValidateSecretRecordRejectsMissingFields(t *testing.T) {
	valid := SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	tests := []struct {
		name   string
		mutate func(*SecretRecord)
	}{
		{name: "missing namespace", mutate: func(record *SecretRecord) { record.Namespace = "" }},
		{name: "missing key", mutate: func(record *SecretRecord) { record.Key = "" }},
		{name: "missing value", mutate: func(record *SecretRecord) { record.Value = "" }},
		{name: "missing time", mutate: func(record *SecretRecord) { record.UpdatedAt = "" }},
		{name: "invalid time", mutate: func(record *SecretRecord) { record.UpdatedAt = "bad-time" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := valid
			test.mutate(&record)
			if err := validateSecretRecord(record); err == nil {
				t.Fatalf("expected validation error for %s", test.name)
			}
		})
	}
	valid.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := validateSecretRecord(valid); err != nil {
		t.Fatalf("expected RFC3339Nano timestamp to be accepted, got %v", err)
	}
}

func TestStrongholdSQLiteFallbackProviderLifecycle(t *testing.T) {
	provider := NewStrongholdSQLiteFallbackProvider(filepath.Join(t.TempDir(), "stronghold-fallback.db"))
	descriptor := provider.Descriptor()
	if descriptor.Available || descriptor.Initialized || !descriptor.Fallback || descriptor.Backend == "" {
		t.Fatalf("expected unopened provider descriptor to stay unavailable until lifecycle open succeeds, got %+v", descriptor)
	}
	store, err := provider.Open(context.Background())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	descriptor = provider.Descriptor()
	if !descriptor.Available || !descriptor.Initialized {
		t.Fatalf("expected opened provider descriptor to expose live availability, got %+v", descriptor)
	}
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()
	if err := store.PutSecret(context.Background(), SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("fallback provider store put returned error: %v", err)
	}
	if _, err := store.GetSecret(context.Background(), "model", "openai_responses_api_key"); err != nil {
		t.Fatalf("fallback provider store get returned error: %v", err)
	}
	missingProvider := NewStrongholdSQLiteFallbackProvider("   ")
	if _, err := missingProvider.Open(context.Background()); err == nil || !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected missing fallback provider to report ErrStrongholdUnavailable, got %v", err)
	}
	missingDescriptor := missingProvider.Descriptor()
	if missingDescriptor.Available || missingDescriptor.Initialized {
		t.Fatalf("expected failed provider descriptor to report unavailable status, got %+v", missingDescriptor)
	}
}

func TestStrongholdSQLiteProviderLifecycle(t *testing.T) {
	provider := NewStrongholdSQLiteProvider(filepath.Join(t.TempDir(), "stronghold-formal.db"))
	descriptor := provider.Descriptor()
	if descriptor.Available || descriptor.Initialized || descriptor.Fallback || descriptor.Backend != "stronghold" {
		t.Fatalf("expected unopened formal provider descriptor to advertise formal stronghold only, got %+v", descriptor)
	}
	store, err := provider.Open(context.Background())
	if runtime.GOOS != "windows" {
		if err == nil || !errors.Is(err, ErrStrongholdUnavailable) {
			t.Fatalf("expected unsupported platform to report ErrStrongholdUnavailable, got %v", err)
		}
		descriptor = provider.Descriptor()
		if descriptor.Available || descriptor.Initialized {
			t.Fatalf("expected unsupported platform descriptor to remain unavailable, got %+v", descriptor)
		}
		return
	}
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()
	descriptor = provider.Descriptor()
	if !descriptor.Available || !descriptor.Initialized || descriptor.Fallback || descriptor.Backend != "stronghold" {
		t.Fatalf("expected opened provider descriptor to report formal stronghold availability, got %+v", descriptor)
	}
	if err := store.PutSecret(context.Background(), SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("formal provider store put returned error: %v", err)
	}
	if _, err := store.GetSecret(context.Background(), "model", "openai_responses_api_key"); err != nil {
		t.Fatalf("formal provider store get returned error: %v", err)
	}
}

func TestStrongholdSQLiteFallbackProviderHonorsCanceledContext(t *testing.T) {
	provider := NewStrongholdSQLiteFallbackProvider(filepath.Join(t.TempDir(), "stronghold-fallback-canceled.db"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Open(ctx); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected wrapped ErrStrongholdUnavailable on canceled context, got %v", err)
	}
	descriptor := provider.Descriptor()
	if descriptor.Available || descriptor.Initialized {
		t.Fatalf("expected canceled provider descriptor to remain unavailable, got %+v", descriptor)
	}
}

func TestStrongholdSQLiteFallbackProviderRejectsUnopenablePath(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "stronghold-fallback-dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("prepare fallback directory path failed: %v", err)
	}
	provider := NewStrongholdSQLiteFallbackProvider(dirPath)
	if _, err := provider.Open(context.Background()); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected unopenable fallback path to return ErrStrongholdUnavailable, got %v", err)
	}
	descriptor := provider.Descriptor()
	if descriptor.Available || descriptor.Initialized || !descriptor.Fallback {
		t.Fatalf("expected failed fallback provider descriptor to stay unavailable, got %+v", descriptor)
	}
}

func TestStrongholdSQLiteProviderRejectsMissingPath(t *testing.T) {
	provider := NewStrongholdSQLiteProvider("   ")
	if _, err := provider.Open(context.Background()); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected missing formal provider path to return ErrStrongholdUnavailable, got %v", err)
	}
}

func TestStrongholdProvidersDescriptorAndCanceledContextBranches(t *testing.T) {
	var nilFormalProvider *StrongholdSQLiteProvider
	descriptor := nilFormalProvider.Descriptor()
	if descriptor.Backend != "stronghold" || descriptor.Available || descriptor.Initialized || descriptor.Fallback {
		t.Fatalf("unexpected nil formal provider descriptor: %+v", descriptor)
	}
	if _, err := nilFormalProvider.Open(context.Background()); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected nil formal provider open to return ErrStrongholdUnavailable, got %v", err)
	}
	var nilFallbackProvider *StrongholdSQLiteFallbackProvider
	descriptor = nilFallbackProvider.Descriptor()
	if descriptor.Backend != "stronghold_sqlite_fallback" || descriptor.Available || descriptor.Initialized || !descriptor.Fallback {
		t.Fatalf("unexpected nil fallback provider descriptor: %+v", descriptor)
	}
	if _, err := nilFallbackProvider.Open(context.Background()); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected nil fallback provider open to return ErrStrongholdUnavailable, got %v", err)
	}

	formalProvider := NewStrongholdSQLiteProvider(filepath.Join(t.TempDir(), "stronghold-formal-cancel.db"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := formalProvider.Open(ctx); runtime.GOOS == "windows" {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled formal provider open to return context.Canceled on windows, got %v", err)
		}
	} else if !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected unsupported platform to still report ErrStrongholdUnavailable, got %v", err)
	}
	missingFallbackProvider := NewStrongholdSQLiteFallbackProvider("   ")
	if _, err := missingFallbackProvider.Open(context.Background()); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected missing fallback provider path to return ErrStrongholdUnavailable, got %v", err)
	}
}

func TestDPAPISecretStoreRoundTripAndCloseBehavior(t *testing.T) {
	store, err := NewDPAPISecretStore(filepath.Join(t.TempDir(), "stronghold-formal-store.db"))
	if runtime.GOOS != "windows" {
		if err == nil || !errors.Is(err, ErrStrongholdUnavailable) {
			t.Fatalf("expected unsupported platform to reject formal stronghold store, got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("NewDPAPISecretStore returned error: %v", err)
	}
	record := SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := store.PutSecret(context.Background(), record); err != nil {
		t.Fatalf("PutSecret returned error: %v", err)
	}
	resolved, err := store.GetSecret(context.Background(), record.Namespace, record.Key)
	if err != nil || resolved.Value != record.Value {
		t.Fatalf("GetSecret returned record=%+v err=%v", resolved, err)
	}
	if err := store.DeleteSecret(context.Background(), record.Namespace, record.Key); err != nil {
		t.Fatalf("DeleteSecret returned error: %v", err)
	}
	if _, err := store.GetSecret(context.Background(), record.Namespace, record.Key); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected deleted secret to disappear, got %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := store.PutSecret(context.Background(), record); !errors.Is(err, ErrSecretStoreAccessFailed) {
		t.Fatalf("expected closed store PutSecret to fail, got %v", err)
	}
	if _, err := store.GetSecret(context.Background(), record.Namespace, record.Key); !errors.Is(err, ErrSecretStoreAccessFailed) {
		t.Fatalf("expected closed store GetSecret to fail, got %v", err)
	}
	if err := store.DeleteSecret(context.Background(), record.Namespace, record.Key); !errors.Is(err, ErrSecretStoreAccessFailed) {
		t.Fatalf("expected closed store DeleteSecret to fail, got %v", err)
	}
}

func TestDPAPISecretStoreRejectsCorruptPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stronghold-corrupt.db")
	if runtime.GOOS != "windows" {
		if _, err := NewDPAPISecretStore(path); err == nil || !errors.Is(err, ErrStrongholdUnavailable) {
			t.Fatalf("expected unsupported platform to reject formal stronghold store, got %v", err)
		}
		return
	}
	if err := os.WriteFile(path, []byte("not-encrypted-payload"), 0o600); err != nil {
		t.Fatalf("write corrupt payload failed: %v", err)
	}
	store, err := NewDPAPISecretStore(path)
	if err != nil {
		t.Fatalf("NewDPAPISecretStore returned error: %v", err)
	}
	if _, err := store.GetSecret(context.Background(), "model", "openai_responses_api_key"); !errors.Is(err, ErrSecretStoreAccessFailed) {
		t.Fatalf("expected corrupt payload to fail with ErrSecretStoreAccessFailed, got %v", err)
	}
}

func TestDPAPISecretStoreLoadAndSavePayloadHelpers(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "stronghold-helper.db")
	if runtime.GOOS != "windows" {
		if _, err := NewDPAPISecretStore(storePath); err == nil || !errors.Is(err, ErrStrongholdUnavailable) {
			t.Fatalf("expected unsupported platform to reject formal stronghold store, got %v", err)
		}
		return
	}
	store, err := NewDPAPISecretStore(storePath)
	if err != nil {
		t.Fatalf("NewDPAPISecretStore returned error: %v", err)
	}
	payload, err := store.loadPayloadLocked()
	if err != nil || payload.Backend != "stronghold" || len(payload.Records) != 0 {
		t.Fatalf("expected empty payload defaults, payload=%+v err=%v", payload, err)
	}
	if err := store.savePayloadLocked(strongholdFilePayload{}); err != nil {
		t.Fatalf("savePayloadLocked returned error: %v", err)
	}
	payload, err = store.loadPayloadLocked()
	if err != nil || payload.Backend != "stronghold" || payload.Records == nil {
		t.Fatalf("expected saved payload to normalize backend and records, payload=%+v err=%v", payload, err)
	}
	encoded, err := json.Marshal(strongholdFilePayload{Records: map[string]SecretRecord{"model::openai_responses_api_key": {Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}}})
	if err != nil {
		t.Fatalf("marshal helper payload failed: %v", err)
	}
	protected, err := protectStrongholdBytes(encoded)
	if err != nil {
		t.Fatalf("protectStrongholdBytes returned error: %v", err)
	}
	if err := os.WriteFile(storePath, protected, 0o600); err != nil {
		t.Fatalf("write protected payload failed: %v", err)
	}
	payload, err = store.loadPayloadLocked()
	if err != nil || payload.Backend != "stronghold" || payload.Records["model::openai_responses_api_key"].Value != "secret" {
		t.Fatalf("expected helper payload to round-trip, payload=%+v err=%v", payload, err)
	}
}

func TestSQLiteSecretStoreEnsuresStrongholdMetadata(t *testing.T) {
	store, err := NewSQLiteSecretStore(filepath.Join(t.TempDir(), "stronghold-metadata.db"))
	if err != nil {
		t.Fatalf("NewSQLiteSecretStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.EnsureStrongholdMetadata(context.Background(), "stronghold_sqlite_fallback"); err != nil {
		t.Fatalf("EnsureStrongholdMetadata returned error: %v", err)
	}
	var backend string
	if err := store.db.QueryRow(`SELECT backend FROM stronghold_metadata WHERE metadata_key = ?`, "active_backend").Scan(&backend); err != nil {
		t.Fatalf("query stronghold metadata failed: %v", err)
	}
	if backend != "stronghold_sqlite_fallback" {
		t.Fatalf("expected stored backend metadata, got %s", backend)
	}
}

func TestNormalizeSecretStoreErrorMapsStrongholdFailures(t *testing.T) {
	if NormalizeSecretStoreError(nil) != nil {
		t.Fatal("expected nil error to stay nil")
	}
	if !errors.Is(NormalizeSecretStoreError(ErrSecretNotFound), ErrSecretNotFound) {
		t.Fatal("expected ErrSecretNotFound to remain unchanged")
	}
	if !errors.Is(NormalizeSecretStoreError(ErrStrongholdUnavailable), ErrStrongholdAccessFailed) {
		t.Fatal("expected stronghold unavailable to normalize to stronghold access failure")
	}
	if !errors.Is(NormalizeSecretStoreError(ErrSecretStoreAccessFailed), ErrStrongholdAccessFailed) {
		t.Fatal("expected secret store access failure to normalize to stronghold access failure")
	}
	if NormalizeSecretStoreError(context.Canceled) != context.Canceled {
		t.Fatal("expected unrelated errors to stay unchanged")
	}
}

func TestUnavailableSecretStoreRejectsAllOperations(t *testing.T) {
	store := UnavailableSecretStore{}
	if err := store.PutSecret(context.Background(), SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected put to fail with ErrStrongholdUnavailable, got %v", err)
	}
	if _, err := store.GetSecret(context.Background(), "model", "openai_responses_api_key"); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected get to fail with ErrStrongholdUnavailable, got %v", err)
	}
	if err := store.DeleteSecret(context.Background(), "model", "openai_responses_api_key"); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected delete to fail with ErrStrongholdUnavailable, got %v", err)
	}
}
