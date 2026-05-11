package urlutil

import (
	"net/url"
	"strings"
)

// SanitizeContextURL strips credentials and volatile URL fragments before page
// context enters task snapshots, perception signals, or other persisted state.
func SanitizeContextURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		// Parsing failures must not fall back to the original value because malformed
		// URLs can still embed credentials or query text that should never persist.
		return ""
	}
	sanitizeParsedURL(parsed)
	return parsed.String()
}

// sanitizeParsedURL clears volatile components from a parsed URL and recursively
// sanitizes nested URLs that live inside opaque wrapper schemes like view-source.
func sanitizeParsedURL(parsed *url.URL) {
	if parsed == nil {
		return
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if sanitizedOpaque, ok := sanitizeNestedOpaqueURL(parsed.Opaque); ok {
		parsed.Opaque = sanitizedOpaque
	}
}

func sanitizeNestedOpaqueURL(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.Contains(trimmed, "://") {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" {
		return "", false
	}
	sanitizeParsedURL(parsed)
	return parsed.String(), true
}
