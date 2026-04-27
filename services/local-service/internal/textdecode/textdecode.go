// Package textdecode centralizes workspace byte-to-text decoding.
package textdecode

import (
	"bytes"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
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
		return Result{Text: string(data), Encoding: EncodingUTF8}, nil
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
	return Result{Text: string(data), Encoding: EncodingUTF8}, nil
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
	if !isLikelyGB18030Text(data, result.Text) {
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

func isLikelyGB18030Text(data []byte, text string) bool {
	// GB18030 overlaps with other legacy encodings at the byte level. Keep this
	// fallback conservative so unsupported text is surfaced instead of silently
	// becoming plausible but wrong workspace content.
	if looksLikeShiftJISBytes(data) {
		return false
	}
	return containsChineseTextSignal(text)
}

func containsChineseTextSignal(text string) bool {
	for _, value := range text {
		if unicode.In(value, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return false
		}
		if unicode.In(value, unicode.Han) || isCJKPunctuation(value) {
			return true
		}
	}
	return false
}

func isCJKPunctuation(value rune) bool {
	return (value >= 0x3000 && value <= 0x303f) || (value >= 0xff00 && value <= 0xffef)
}

func looksLikeShiftJISBytes(data []byte) bool {
	totalLegacySequences := 0
	shiftJISPairs := 0
	kanaPairs := 0

	for index := 0; index < len(data); {
		current := data[index]
		if current < 0x80 {
			index++
			continue
		}

		totalLegacySequences++
		if index+1 < len(data) {
			next := data[index+1]
			if isShiftJISPair(current, next) {
				shiftJISPairs++
				if isShiftJISKanaPair(current, next) {
					kanaPairs++
				}
			}
			if isGB18030FourByteSequence(data, index) {
				index += 4
				continue
			}
			if isGB18030TwoByteSequence(current, next) {
				index += 2
				continue
			}
		}
		index++
	}

	if kanaPairs >= 2 {
		return true
	}
	return totalLegacySequences >= 2 && shiftJISPairs == totalLegacySequences
}

func isShiftJISPair(first byte, second byte) bool {
	return ((first >= 0x81 && first <= 0x9f) || (first >= 0xe0 && first <= 0xfc)) &&
		((second >= 0x40 && second <= 0x7e) || (second >= 0x80 && second <= 0xfc))
}

func isShiftJISKanaPair(first byte, second byte) bool {
	return (first == 0x82 && second >= 0x9f && second <= 0xf1) ||
		(first == 0x83 && second >= 0x40 && second <= 0x96)
}

func isGB18030FourByteSequence(data []byte, index int) bool {
	if index+3 >= len(data) {
		return false
	}
	return data[index] >= 0x81 && data[index] <= 0xfe &&
		data[index+1] >= 0x30 && data[index+1] <= 0x39 &&
		data[index+2] >= 0x81 && data[index+2] <= 0xfe &&
		data[index+3] >= 0x30 && data[index+3] <= 0x39
}

func isGB18030TwoByteSequence(first byte, second byte) bool {
	return first >= 0x81 && first <= 0xfe &&
		((second >= 0x40 && second <= 0x7e) || (second >= 0x80 && second <= 0xfe))
}
