//go:build windows

package storage

import "testing"

func TestWindowsStrongholdBlobHelpers(t *testing.T) {
	if blob := newWindowsDataBlob(nil); blob.Data != nil || blob.Size != 0 {
		t.Fatalf("expected empty blob for nil input, got %+v", blob)
	}
	data := []byte("secret")
	blob := newWindowsDataBlob(data)
	if blob.Data == nil || blob.Size != uint32(len(data)) {
		t.Fatalf("expected populated blob, got %+v", blob)
	}
	freeWindowsDataBlob(newWindowsDataBlob(nil))
	if decoded, err := unprotectStrongholdBytes([]byte("not-protected")); err == nil || decoded != nil {
		t.Fatalf("expected invalid ciphertext to fail, decoded=%v err=%v", decoded, err)
	}
}
