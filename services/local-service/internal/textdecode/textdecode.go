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

var commonHanRunes = buildRuneSet(`的一是在不了有人和这中大为上个国我以要他时来用们生到作地于出就分对成会可主发年动同工也能下过子说产种面而方后多定行学法所民得经十三之进着等部度家电力里如水化高自二理起小现实加量都两体制机当使点从业本去把性好应开它合还因由其些然前外天政四日那社义事平形相全表间样与关各重新线内数正心反你明看原又么利比或但质气第向道命此变条只没结解问意建月公无系军很情者最立代想已通并提直题党程展五果料象员革位入常文总次品式活设及管特件长求老头基资边流路级少图山统接知较将组见计别她手角期根论运农指几九区强放决西被干做必战先回则任取据处理世车价远步改领
修复乱码执行输入输出文件内容任务来源检查说明标题备注状态同步失败成功问题错误日志预览摘要工作空间中文文本文档目录计划配置设置更新读取写入打开关闭转换安全识别`)

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
	if !roundTripsGB18030(data, text) {
		return false
	}
	return hasReadableLegacyTextSignal(text)
}

func roundTripsGB18030(data []byte, text string) bool {
	encoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(text))
	return err == nil && bytes.Equal(encoded, data)
}

func hasReadableLegacyTextSignal(text string) bool {
	for _, value := range text {
		switch {
		case value <= unicode.MaxASCII:
			if unicode.IsLetter(value) || unicode.IsDigit(value) || unicode.IsPunct(value) || unicode.IsSpace(value) || unicode.IsSymbol(value) {
				return true
			}
		case isCommonHanRune(value):
			return true
		case unicode.In(value, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Latin, unicode.Greek, unicode.Cyrillic):
			return true
		case isCommonCJKPunctuation(value):
			return true
		}
	}
	return false
}

func isCommonHanRune(value rune) bool {
	_, ok := commonHanRunes[value]
	return ok
}

func isCommonCJKPunctuation(value rune) bool {
	switch value {
	case '。', '，', '、', '；', '：', '！', '？', '（', '）', '【', '】', '《', '》', '“', '”', '‘', '’':
		return true
	default:
		return false
	}
}

func buildRuneSet(values string) map[rune]struct{} {
	result := make(map[rune]struct{}, len(values))
	for _, value := range values {
		if value == '\n' || value == '\r' || value == '\t' || value == ' ' {
			continue
		}
		result[value] = struct{}{}
	}
	return result
}
