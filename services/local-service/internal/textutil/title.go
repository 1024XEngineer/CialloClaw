package textutil

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

const defaultCompactTitleLength = 24

var compactTitleSeparators = map[rune]struct{}{
	'\n': {}, '\r': {}, '\t': {},
	'。': {}, '！': {}, '？': {}, '；': {}, '，': {}, '、': {},
	'.': {}, '!': {}, '?': {}, ';': {}, ',': {},
}

var compactTitleNoiseValues = map[string]struct{}{
	"":         {},
	"当前内容":     {},
	"当前任务":     {},
	"待办事项":     {},
	"new note": {},
}

var compactTitleRequestPrefixes = []string{
	"请帮我把", "请帮我", "帮我把", "帮我", "麻烦你把", "麻烦你", "我想把", "我想", "我需要把", "我需要",
	"需要把", "需要", "想把", "想", "帮忙把", "帮忙", "可以把", "可以", "能不能把", "能不能",
	"先把", "先", "把", "看下", "看看",
}

var compactTitleActionPrefixes = []string{
	"总结一下", "总结", "整理一下", "整理", "分析一下", "分析", "检查一下", "检查", "解释一下", "解释",
	"翻译一下", "翻译", "改写一下", "改写", "处理一下", "处理", "review", "summarize", "rewrite",
	"translate", "explain", "check", "analyze",
}

var compactTitleSuffixes = []string{
	"一下", "一下子", "吗", "吧", "呀", "呢", "谢谢", "感谢",
}

type compactTitleFragment struct {
	text      string
	partIndex int
}

// CompactSubject reduces full user input into a short task subject. It strips
// request wrappers aggressively because the task title already carries the
// intent prefix such as "处理：" or "总结：".
func CompactSubject(parts []string, fallback string, maxLength int) string {
	return compactTitle(parts, fallback, maxLength, true)
}

// CompactLabel reduces longer note content into a stable user-facing label.
func CompactLabel(parts []string, fallback string, maxLength int) string {
	return compactTitle(parts, fallback, maxLength, false)
}

func compactTitle(parts []string, fallback string, maxLength int, stripActions bool) string {
	if maxLength <= 0 {
		maxLength = defaultCompactTitleLength
	}

	fragments := collectCompactTitleFragments(parts, stripActions)
	if len(fragments) == 0 {
		return TruncateGraphemes(strings.TrimSpace(fallback), maxLength)
	}

	primary := bestCompactTitleFragment(fragments, maxLength)
	title := primary.text
	if secondary := secondaryCompactTitleFragment(fragments, primary); secondary.text != "" {
		if combined := combineCompactTitleFragments(primary.text, secondary.text, maxLength); combined != "" {
			title = combined
		}
	}
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(fallback)
	}
	if strings.TrimSpace(title) == "" && len(fragments) > 0 {
		title = fragments[0].text
	}
	return TruncateGraphemes(title, maxLength)
}

func collectCompactTitleFragments(parts []string, stripActions bool) []compactTitleFragment {
	result := make([]compactTitleFragment, 0)
	seen := map[string]struct{}{}

	for partIndex, part := range parts {
		normalizedPart := normalizeCompactTitleText(part)
		if normalizedPart == "" {
			continue
		}
		for _, piece := range splitCompactTitleText(normalizedPart) {
			normalizedPiece := normalizeCompactTitleFragment(piece, stripActions)
			if normalizedPiece == "" {
				continue
			}
			key := strings.ToLower(normalizedPiece)
			if _, ok := compactTitleNoiseValues[key]; ok {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, compactTitleFragment{
				text:      normalizedPiece,
				partIndex: partIndex,
			})
		}
	}

	return result
}

func splitCompactTitleText(value string) []string {
	rawParts := strings.FieldsFunc(value, func(r rune) bool {
		_, ok := compactTitleSeparators[r]
		return ok
	})
	if len(rawParts) == 0 {
		return []string{value}
	}
	return rawParts
}

func normalizeCompactTitleText(value string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(value), " "))
}

func normalizeCompactTitleFragment(value string, stripActions bool) string {
	normalized := normalizeCompactTitleText(value)
	if normalized == "" {
		return ""
	}
	normalized = trimCompactTitleMetadataPrefix(normalized)
	normalized = trimCompactTitlePrefixes(normalized, compactTitleRequestPrefixes)
	if stripActions {
		normalized = trimCompactTitlePrefixes(normalized, compactTitleActionPrefixes)
	}
	normalized = trimCompactTitleSuffixes(normalized)
	normalized = strings.Trim(normalized, " -_:：,.，;；/\\|()[]{}<>\"'`")
	normalized = normalizeCompactTitleText(normalized)
	if isCompactTitleMostlyNoise(normalized) {
		return ""
	}
	return normalized
}

func trimCompactTitleMetadataPrefix(value string) string {
	lower := strings.ToLower(value)
	for _, prefix := range []string{"note:", "notes:", "agent:", "suggest:", "todo:", "title:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return value
}

func trimCompactTitlePrefixes(value string, prefixes []string) string {
	trimmed := strings.TrimSpace(value)
	for {
		updated := trimmed
		lower := strings.ToLower(trimmed)
		for _, prefix := range prefixes {
			prefixLower := strings.ToLower(prefix)
			if strings.HasPrefix(lower, prefixLower) && len(trimmed) > len(prefix) {
				updated = strings.TrimSpace(trimmed[len(prefix):])
				break
			}
		}
		if updated == trimmed {
			return trimmed
		}
		trimmed = updated
	}
}

func trimCompactTitleSuffixes(value string) string {
	trimmed := strings.TrimSpace(value)
	for {
		updated := trimmed
		for _, suffix := range compactTitleSuffixes {
			if strings.HasSuffix(updated, suffix) && len(updated) > len(suffix) {
				updated = strings.TrimSpace(strings.TrimSuffix(updated, suffix))
				break
			}
		}
		if updated == trimmed {
			return trimmed
		}
		trimmed = updated
	}
}

func isCompactTitleMostlyNoise(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	meaningful := 0
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.In(r, unicode.Han) {
			meaningful++
		}
	}
	return meaningful == 0
}

func bestCompactTitleFragment(fragments []compactTitleFragment, maxLength int) compactTitleFragment {
	best := fragments[0]
	bestScore := compactTitleFragmentScore(best, maxLength)
	for _, fragment := range fragments[1:] {
		score := compactTitleFragmentScore(fragment, maxLength)
		if score > bestScore {
			best = fragment
			bestScore = score
		}
	}
	return best
}

func secondaryCompactTitleFragment(fragments []compactTitleFragment, primary compactTitleFragment) compactTitleFragment {
	best := compactTitleFragment{}
	bestScore := -1 << 30
	for _, fragment := range fragments {
		if fragment.text == primary.text {
			continue
		}
		if compactTitleFragmentsOverlap(primary.text, fragment.text) {
			continue
		}
		score := compactTitleFragmentScore(fragment, defaultCompactTitleLength)
		if score > bestScore {
			best = fragment
			bestScore = score
		}
	}
	return best
}

func compactTitleFragmentScore(fragment compactTitleFragment, maxLength int) int {
	length := countCompactTitleGraphemes(fragment.text)
	score := 0
	switch {
	case length >= 7 && length <= maxLength:
		score += 12
	case length >= 4 && length <= 6:
		score += 10
	case length > maxLength && length <= maxLength+16:
		score += 8
	case length > maxLength+16:
		score += 6
	default:
		score += length
	}
	if strings.Contains(fragment.text, " ") || countHanRunes(fragment.text) >= 4 {
		score += 3
	}
	if strings.ContainsAny(fragment.text, "0123456789") {
		score += 1
	}
	score -= fragment.partIndex * 2
	return score
}

func combineCompactTitleFragments(primary string, secondary string, maxLength int) string {
	if primary == "" {
		return secondary
	}
	if secondary == "" {
		return primary
	}
	if countCompactTitleGraphemes(primary) >= maxLength-4 {
		return ""
	}
	combined := primary + " · " + secondary
	if countCompactTitleGraphemes(combined) <= maxLength {
		return combined
	}
	shorterCombined := primary + " " + secondary
	if countCompactTitleGraphemes(shorterCombined) <= maxLength {
		return shorterCombined
	}
	if countCompactTitleGraphemes(primary) <= maxLength/2 {
		return primary + " · " + TruncateGraphemes(secondary, maxLength-countCompactTitleGraphemes(primary)-3)
	}
	return ""
}

func compactTitleFragmentsOverlap(left string, right string) bool {
	leftKey := strings.ToLower(strings.TrimSpace(left))
	rightKey := strings.ToLower(strings.TrimSpace(right))
	if leftKey == "" || rightKey == "" {
		return false
	}
	return strings.Contains(leftKey, rightKey) || strings.Contains(rightKey, leftKey)
}

func countCompactTitleGraphemes(value string) int {
	graphemes := uniseg.NewGraphemes(value)
	count := 0
	for graphemes.Next() {
		count++
	}
	return count
}

func countHanRunes(value string) int {
	count := 0
	for _, r := range value {
		if unicode.In(r, unicode.Han) {
			count++
		}
	}
	return count
}
