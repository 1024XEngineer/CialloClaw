package textutil

import (
	"strings"

	"github.com/rivo/uniseg"
)

// TruncateGraphemes keeps preview text aligned with user-visible characters so
// delivery and recommendation surfaces do not cut emoji, ZWJ sequences, or
// combining marks in half. maxLength is the final visible-character budget,
// including the trailing ellipsis when truncation happens.
func TruncateGraphemes(value string, maxLength int) string {
	if maxLength <= 0 {
		return value
	}

	graphemes := uniseg.NewGraphemes(value)
	var builder strings.Builder
	count := 0
	for graphemes.Next() {
		if maxLength > 3 {
			if count == maxLength-3 {
				return builder.String() + "..."
			}
		} else if count == maxLength {
			return builder.String()
		}
		builder.WriteString(graphemes.Str())
		count++
	}
	return value
}
