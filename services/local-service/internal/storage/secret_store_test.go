package storage

import (
	"context"
	"path/filepath"
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
