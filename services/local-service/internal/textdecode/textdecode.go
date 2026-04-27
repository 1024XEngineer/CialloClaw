// Package textdecode centralizes workspace byte-to-text decoding.
package textdecode

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	xunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	EncodingUTF8    = "utf-8"
	EncodingUTF16BE = "utf-16be"
	EncodingUTF16LE = "utf-16le"
	EncodingGB18030 = "gb18030"
)

// UnsupportedEncodingUserMessage is the stable user-facing warning shown when
// workspace bytes cannot be decoded safely.
const UnsupportedEncodingUserMessage = "文件编码无法安全识别，请转换为 UTF-8、UTF-16 BOM 或 GB18030 后重试。"

// ErrUnsupportedEncoding marks input that cannot be decoded without risking
// replacement characters or binary data leaking into user-facing text.
var ErrUnsupportedEncoding = errors.New("unsupported or unsafe text encoding")

// Result carries decoded text plus the encoding that was accepted.
type Result struct {
	Text     string
	Encoding string
}

type legacyCodec struct {
	decoder *encoding.Decoder
	encoder *encoding.Encoder
}

var competingLegacyCodecs = []legacyCodec{
	{
		decoder: japanese.ShiftJIS.NewDecoder(),
		encoder: japanese.ShiftJIS.NewEncoder(),
	},
	{
		decoder: japanese.EUCJP.NewDecoder(),
		encoder: japanese.EUCJP.NewEncoder(),
	},
	{
		decoder: korean.EUCKR.NewDecoder(),
		encoder: korean.EUCKR.NewEncoder(),
	},
}

// Decode normalizes workspace file bytes before they enter tool output,
// execution prompts, or task inspection. Unsupported data returns a typed
// error so callers can show an explicit warning instead of propagating mojibake.
func Decode(data []byte) (Result, error) {
	if len(data) == 0 {
		return Result{Encoding: EncodingUTF8}, nil
	}
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return decodeUTF8(data[3:])
	}
	if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		return decodeTransform(data, xunicode.UTF16(xunicode.BigEndian, xunicode.ExpectBOM).NewDecoder(), EncodingUTF16BE)
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		return decodeTransform(data, xunicode.UTF16(xunicode.LittleEndian, xunicode.ExpectBOM).NewDecoder(), EncodingUTF16LE)
	}
	if utf8.Valid(data) {
		return decodeUTF8(data)
	}
	if hasBinaryControls(data) {
		return Result{}, ErrUnsupportedEncoding
	}
	return decodeGB18030(data)
}

func decodeUTF8(data []byte) (Result, error) {
	if !utf8.Valid(data) {
		return Result{}, ErrUnsupportedEncoding
	}
	text := string(data)
	if !hasSafeControls(text) {
		return Result{}, ErrUnsupportedEncoding
	}
	return Result{Text: text, Encoding: EncodingUTF8}, nil
}

func decodeTransform(data []byte, decoder *encoding.Decoder, encodingName string) (Result, error) {
	decoded, _, err := transform.Bytes(decoder, data)
	if err != nil {
		return Result{}, errors.Join(ErrUnsupportedEncoding, err)
	}
	text := string(decoded)
	if !isSafeDecodedText(text) {
		return Result{}, ErrUnsupportedEncoding
	}
	return Result{Text: text, Encoding: encodingName}, nil
}

func decodeGB18030(data []byte) (Result, error) {
	result, err := decodeTransform(data, simplifiedchinese.GB18030.NewDecoder(), EncodingGB18030)
	if err != nil {
		return Result{}, err
	}
	if !isSupportedGB18030Text(data, result.Text) {
		return Result{}, ErrUnsupportedEncoding
	}
	return result, nil
}

func hasBinaryControls(data []byte) bool {
	const maxSample = 4096
	sample := data
	if len(sample) > maxSample {
		sample = sample[:maxSample]
	}
	controlCount := 0
	for _, value := range sample {
		switch value {
		case '\n', '\r', '\t':
			continue
		case 0:
			return true
		}
		if value < 0x20 {
			controlCount++
		}
	}
	return controlCount > len(sample)/16
}

func isSafeDecodedText(text string) bool {
	if strings.ContainsRune(text, utf8.RuneError) {
		return false
	}
	return hasSafeControls(text)
}

func hasSafeControls(text string) bool {
	for _, value := range text {
		switch value {
		case '\n', '\r', '\t':
			continue
		}
		if value == 0 || unicode.IsControl(value) {
			return false
		}
	}
	return true
}

func isSupportedGB18030Text(data []byte, text string) bool {
	// GB18030 remains a documented safe input, but the decoder must not become a
	// generic fallback for every legacy byte stream. Keep the byte-level
	// round-trip invariant and require decoded text to contain a readable script
	// signal instead of blindly accepting any reversible mojibake.
	if !roundTripsWithEncoding(data, text, simplifiedchinese.GB18030.NewEncoder()) {
		return false
	}
	score := legacyTextReadabilityScore(text)
	if score == 0 {
		return false
	}
	return !hasStrongerCompetingLegacyDecode(data, score)
}

func roundTripsWithEncoding(data []byte, text string, encoder *encoding.Encoder) bool {
	encoded, _, err := transform.Bytes(encoder, []byte(text))
	return err == nil && bytes.Equal(encoded, data)
}

func legacyTextReadabilityScore(text string) int {
	score := 0
	for _, value := range text {
		switch {
		case value <= unicode.MaxASCII:
			switch {
			case unicode.IsLetter(value) || unicode.IsDigit(value):
				score += 2
			case unicode.IsPunct(value) || unicode.IsSpace(value) || unicode.IsSymbol(value):
				score++
			}
		case unicode.In(value, unicode.Han):
			score += 2
		case unicode.In(value, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Latin, unicode.Greek, unicode.Cyrillic):
			score += 3
		case isCommonCJKPunctuation(value):
			score++
		}
	}
	return score
}

func hasStrongerCompetingLegacyDecode(data []byte, acceptedScore int) bool {
	for _, codec := range competingLegacyCodecs {
		decoded, _, err := transform.Bytes(codec.decoder, data)
		if err != nil {
			continue
		}
		text := string(decoded)
		if !isSafeDecodedText(text) || !roundTripsWithEncoding(data, text, codec.encoder) {
			continue
		}
		if score := legacyTextReadabilityScore(text); score > acceptedScore && hasPureDistinctiveLegacyScript(text) {
			return true
		}
	}
	return false
}

func hasPureDistinctiveLegacyScript(text string) bool {
	distinctiveCount := 0
	for _, value := range text {
		switch {
		case unicode.In(value, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
			distinctiveCount++
		case value <= unicode.MaxASCII:
			if unicode.IsPunct(value) || unicode.IsSpace(value) || unicode.IsSymbol(value) || unicode.IsDigit(value) {
				continue
			}
			return false
		case isCommonCJKPunctuation(value):
			continue
		default:
			return false
		}
	}
	return distinctiveCount >= 2
}

func isCommonCJKPunctuation(value rune) bool {
	switch value {
	case '。', '，', '、', '；', '：', '！', '？', '（', '）', '【', '】', '《', '》', '“', '”', '‘', '’':
		return true
	default:
		return false
	}
}
