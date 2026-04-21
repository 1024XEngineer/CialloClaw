package storage

import (
	"context"
	"errors"
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
