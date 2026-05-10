// Package languagepolicy centralizes lightweight request-scoped language
// inference so clarification, planner prompts, and fallback responses do not
// silently drift between English and Chinese defaults.
package languagepolicy

import (
	"strings"
	"unicode"
)

const (
	ReplyLanguageChinese = "zh-CN"
	ReplyLanguageEnglish = "en-US"
)

// PreferredReplyLanguage infers the reply language from the current user-facing
// text only. The heuristic intentionally stays conservative: it upgrades to
// English only for English-only inputs so Chinese-first behavior remains stable
// for mixed-language and ambiguous requests.
func PreferredReplyLanguage(text string) string {
	if IsEnglishOnlyText(text) {
		return ReplyLanguageEnglish
	}

	return ReplyLanguageChinese
}

// IsEnglishOnlyText reports whether one input is composed like an English-only
// request. The check rejects Han characters and requires enough Latin letters
// so short punctuation-only inputs do not accidentally flip the reply policy.
func IsEnglishOnlyText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}

	latinLetters := 0
	for _, r := range trimmed {
		switch {
		case unicode.Is(unicode.Han, r):
			return false
		case unicode.IsLetter(r):
			if r > unicode.MaxASCII {
				return false
			}
			latinLetters++
		case unicode.IsDigit(r), unicode.IsSpace(r):
			continue
		case strings.ContainsRune(`.,!?;:'"()[]{}<>/@#$%^&*_+-=|\\~`, r):
			continue
		default:
			if r > unicode.MaxASCII {
				return false
			}
		}
	}

	return latinLetters >= 2
}
