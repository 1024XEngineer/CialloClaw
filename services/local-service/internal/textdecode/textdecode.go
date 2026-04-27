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
	if !roundTripsGB18030(data, result.Text) {
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

func roundTripsGB18030(data []byte, text string) bool {
	// The fallback accepts GB18030 because it is part of the documented safe text
	// set, not because it can guess every legacy charset. The round-trip check
	// keeps malformed or non-canonical byte sequences from being accepted while
	// avoiding format-specific heuristics that reject valid GB18030 content.
	encoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(text))
	return err == nil && bytes.Equal(encoded, data)
}
