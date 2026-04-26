//go:build !windows

package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDPAPISecretStoreStubAlwaysReportsUnavailable(t *testing.T) {
	if store, err := NewDPAPISecretStore("stronghold.db"); store != nil || !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected non-Windows DPAPI constructor to return ErrStrongholdUnavailable, store=%+v err=%v", store, err)
	}
	stub := &DPAPISecretStore{}
	record := SecretRecord{Namespace: "model", Key: "openai_responses_api_key", Value: "secret", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := stub.PutSecret(context.Background(), record); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected stub PutSecret to return ErrStrongholdUnavailable, got %v", err)
	}
	if _, err := stub.GetSecret(context.Background(), record.Namespace, record.Key); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected stub GetSecret to return ErrStrongholdUnavailable, got %v", err)
	}
	if err := stub.DeleteSecret(context.Background(), record.Namespace, record.Key); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected stub DeleteSecret to return ErrStrongholdUnavailable, got %v", err)
	}
	if _, err := stub.loadPayloadLocked(); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected stub loadPayloadLocked to return ErrStrongholdUnavailable, got %v", err)
	}
	if err := stub.savePayloadLocked(strongholdFilePayload{}); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected stub savePayloadLocked to return ErrStrongholdUnavailable, got %v", err)
	}
	if err := stub.Close(); err != nil {
		t.Fatalf("expected stub Close to succeed, got %v", err)
	}
	if _, err := protectStrongholdBytes([]byte("secret")); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected stub protectStrongholdBytes to return ErrStrongholdUnavailable, got %v", err)
	}
	if _, err := unprotectStrongholdBytes([]byte("secret")); !errors.Is(err, ErrStrongholdUnavailable) {
		t.Fatalf("expected stub unprotectStrongholdBytes to return ErrStrongholdUnavailable, got %v", err)
	}
}
