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
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
