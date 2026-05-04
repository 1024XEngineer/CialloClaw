package textutil

import (
	"strings"

	"github.com/rivo/uniseg"
)

// TruncateGraphemes keeps preview text aligned with user-visible characters so
// delivery and recommendation surfaces do not cut emoji, ZWJ sequences, or
// combining marks in half.
func TruncateGraphemes(value string, maxLength int) string {
	if maxLength <= 0 {
		return value
	}

	graphemes := uniseg.NewGraphemes(value)
	var builder strings.Builder
	builder.Grow(len(value) + 3)
	count := 0
	for graphemes.Next() {
		if count == maxLength {
			return builder.String() + "..."
		}
		builder.WriteString(graphemes.Str())
		count++
	}
	return value
}
