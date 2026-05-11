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
		return trimmed
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
