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

var englishEvidenceWords = map[string]struct{}{
	"a":         {},
	"add":       {},
	"answer":    {},
	"browser":   {},
	"build":     {},
	"can":       {},
	"checklist": {},
	"copy":      {},
	"could":     {},
	"create":    {},
	"cleanup":   {},
	"continue":  {},
	"delete":    {},
	"deploy":    {},
	"doc":       {},
	"docs":      {},
	"edit":      {},
	"english":   {},
	"error":     {},
	"explain":   {},
	"file":      {},
	"files":     {},
	"fix":       {},
	"folder":    {},
	"good":      {},
	"hello":     {},
	"help":      {},
	"hey":       {},
	"hi":        {},
	"how":       {},
	"i":         {},
	"inspect":   {},
	"install":   {},
	"issue":     {},
	"lgtm":      {},
	"list":      {},
	"logs":      {},
	"make":      {},
	"morning":   {},
	"move":      {},
	"need":      {},
	"note":      {},
	"notes":     {},
	"open":      {},
	"page":      {},
	"please":    {},
	"proceed":   {},
	"project":   {},
	"read":      {},
	"readme":    {},
	"remove":    {},
	"rename":    {},
	"release":   {},
	"review":    {},
	"rewrite":   {},
	"rollout":   {},
	"run":       {},
	"search":    {},
	"summarise": {},
	"summarize": {},
	"task":      {},
	"that":      {},
	"thank":     {},
	"thanks":    {},
	"the":       {},
	"thx":       {},
	"this":      {},
	"translate": {},
	"update":    {},
	"utils":     {},
	"what":      {},
	"why":       {},
	"workspace": {},
	"would":     {},
	"write":     {},
}

var englishSignalPhrases = map[string]struct{}{
	"done":        {},
	"go ahead":    {},
	"i am ready":  {},
	"i m ready":   {},
	"im ready":    {},
	"looks good":  {},
	"no":          {},
	"ok":          {},
	"okay":        {},
	"ready":       {},
	"ship it":     {},
	"sounds good": {},
	"sure":        {},
	"yes":         {},
}

var englishSignalTextNormalizer = strings.NewReplacer(
	"\u2018", "'",
	"\u2019", "'",
	"\u201c", `"`,
	"\u201d", `"`,
	"\u2013", "-",
	"\u2014", "-",
)

var englishWordSuffixes = []string{
	"able",
	"al",
	"ed",
	"er",
	"ful",
	"ible",
	"ing",
	"ise",
	"ize",
	"less",
	"ly",
	"ment",
	"ness",
	"ous",
	"sion",
	"tion",
}

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
// request. The heuristic stays intentionally conservative: it still rejects Han
// text and non-ASCII letters, but it now accepts broader plain-English command
// phrases instead of relying on one fixed phrase whitelist.
func IsEnglishOnlyText(text string) bool {
	trimmed := strings.TrimSpace(normalizeEnglishSignalText(text))
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
		case isASCIIEnglishPunctuation(r):
			continue
		default:
			if r > unicode.MaxASCII {
				return false
			}
		}
	}

	if latinLetters < 2 {
		return false
	}

	tokens := asciiWordTokens(trimmed)
	if containsEnglishSignalPhrase(strings.Join(tokens, " ")) {
		return true
	}
	return hasEnglishWordEvidence(tokens)
}

func containsEnglishSignalPhrase(phrase string) bool {
	if phrase == "" {
		return false
	}
	_, ok := englishSignalPhrases[phrase]
	return ok
}

func hasEnglishWordEvidence(tokens []string) bool {
	for _, token := range tokens {
		if isEnglishEvidenceToken(token) {
			return true
		}
	}
	return false
}

func isEnglishEvidenceToken(token string) bool {
	if token == "" {
		return false
	}
	if _, ok := englishEvidenceWords[token]; ok {
		return true
	}
	if len(token) <= 3 {
		return false
	}
	for _, suffix := range englishWordSuffixes {
		if strings.HasSuffix(token, suffix) && len(token) > len(suffix)+1 {
			return true
		}
	}
	return false
}

func asciiWordTokens(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !isASCIIAlpha(r)
	})
	if len(fields) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.ToLower(strings.TrimSpace(field))
		if trimmed != "" {
			tokens = append(tokens, trimmed)
		}
	}
	return tokens
}

func isASCIIAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIEnglishPunctuation(r rune) bool {
	return r <= unicode.MaxASCII && strings.ContainsRune(`.,!?;:'"()[]{}<>/@#$%^&*_+-=|\\~`, r)
}

func normalizeEnglishSignalText(text string) string {
	if text == "" {
		return ""
	}

	normalized := englishSignalTextNormalizer.Replace(text)
	var builder strings.Builder
	builder.Grow(len(normalized))
	for _, r := range normalized {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsSpace(r):
			builder.WriteRune(r)
		case isASCIIEnglishPunctuation(r):
			builder.WriteRune(r)
		case r > unicode.MaxASCII && !unicode.IsLetter(r) && !unicode.IsDigit(r):
			// Strip emoji and other non-ASCII symbols so short English signals like
			// "thanks 👍" still keep their language intent.
			builder.WriteRune(' ')
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
