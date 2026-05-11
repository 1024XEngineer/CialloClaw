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

func TestSanitizeContextURLDropsMalformedInputsInsteadOfPersistingThemVerbatim(t *testing.T) {
	got := SanitizeContextURL(" https://user:pass@example.com/%zz?token=secret ")
	if got != "" {
		t.Fatalf("expected malformed url to be dropped, got %q", got)
	}
}
