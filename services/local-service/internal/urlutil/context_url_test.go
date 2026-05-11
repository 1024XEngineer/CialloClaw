package urlutil

import "testing"

func TestSanitizeContextURLStripsCredentialsAndVolatileFragments(t *testing.T) {
	got := SanitizeContextURL(" https://user:pass@example.com/docs?id=42#intro ")
	if got != "https://example.com/docs" {
		t.Fatalf("expected sanitized https url, got %q", got)
	}
}

func TestSanitizeContextURLKeepsLocalSchemesStable(t *testing.T) {
	got := SanitizeContextURL("local://shell-ball?source=floating_ball#focus")
	if got != "local://shell-ball" {
		t.Fatalf("expected local scheme to drop query and fragment, got %q", got)
	}
}

func TestSanitizeContextURLSanitizesNestedOpaqueURLs(t *testing.T) {
	got := SanitizeContextURL(" view-source:https://user:pass@example.com/docs?id=42#intro ")
	if got != "view-source:https://example.com/docs" {
		t.Fatalf("expected nested opaque url to be sanitized, got %q", got)
	}
}

func TestSanitizeContextURLRecursivelySanitizesNestedOpaqueWrappers(t *testing.T) {
	got := SanitizeContextURL("view-source:jar:https://user:pass@example.com/app.jar!/BOOT-INF/classes?token=1#frag")
	if got != "view-source:jar:https://example.com/app.jar!/BOOT-INF/classes" {
		t.Fatalf("expected recursive opaque sanitization, got %q", got)
	}
}

func TestSanitizeContextURLLeavesNonURLOpaquePayloadsUntouched(t *testing.T) {
	got := SanitizeContextURL("mailto:user:pass@example.com?subject=secret")
	if got != "mailto:user:pass@example.com" {
		t.Fatalf("expected non-url opaque payload to keep stable shape, got %q", got)
	}
}

func TestSanitizeContextURLDropsMalformedInputsInsteadOfPersistingThemVerbatim(t *testing.T) {
	got := SanitizeContextURL(" https://user:pass@example.com/%zz?token=secret ")
	if got != "" {
		t.Fatalf("expected malformed url to be dropped, got %q", got)
	}
}
