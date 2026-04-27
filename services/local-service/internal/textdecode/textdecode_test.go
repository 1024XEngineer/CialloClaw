package textdecode

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestDecodeSupportsUTF8AndGB18030(t *testing.T) {
	utf8Result, err := Decode([]byte("修复乱码"))
	if err != nil {
		t.Fatalf("Decode UTF-8 returned error: %v", err)
	}
	if utf8Result.Text != "修复乱码" || utf8Result.Encoding != EncodingUTF8 {
		t.Fatalf("unexpected UTF-8 result: %+v", utf8Result)
	}

	gb18030Bytes, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte("修复乱码"))
	if err != nil {
		t.Fatalf("GB18030 encode failed: %v", err)
	}
	gb18030Result, err := Decode(gb18030Bytes)
	if err != nil {
		t.Fatalf("Decode GB18030 returned error: %v", err)
	}
	if gb18030Result.Text != "修复乱码" || gb18030Result.Encoding != EncodingGB18030 {
		t.Fatalf("unexpected GB18030 result: %+v", gb18030Result)
	}
}

func TestDecodeRejectsAmbiguousShiftJIS(t *testing.T) {
	shiftJISBytes, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte("\u3053\u3093\u306b\u3061\u306f"))
	if err != nil {
		t.Fatalf("Shift-JIS encode failed: %v", err)
	}
	_, err = Decode(shiftJISBytes)
	if !errors.Is(err, ErrUnsupportedEncoding) {
		t.Fatalf("expected ErrUnsupportedEncoding for ambiguous Shift-JIS bytes, got %v", err)
	}
}

func TestDecodeSupportsUTF16BOM(t *testing.T) {
	data := utf16LEWithBOM("修复乱码")
	result, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode UTF-16LE returned error: %v", err)
	}
	if result.Text != "修复乱码" || result.Encoding != EncodingUTF16LE {
		t.Fatalf("unexpected UTF-16LE result: %+v", result)
	}
}

func TestDecodeRejectsUnsafeText(t *testing.T) {
	_, err := Decode([]byte{0x00, 0x01, 0x02, 0xFF})
	if !errors.Is(err, ErrUnsupportedEncoding) {
		t.Fatalf("expected ErrUnsupportedEncoding, got %v", err)
	}

	gb18030Bytes, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte("ok"))
	if err != nil {
		t.Fatalf("GB18030 encode failed: %v", err)
	}
	result, err := Decode(gb18030Bytes)
	if err != nil || strings.ContainsRune(result.Text, '\uFFFD') {
		t.Fatalf("expected safe decoded text, got result=%+v err=%v", result, err)
	}
}

func utf16LEWithBOM(value string) []byte {
	units := utf16.Encode([]rune(value))
	result := []byte{0xFF, 0xFE}
	for _, unit := range units {
		result = append(result, byte(unit), byte(unit>>8))
	}
	return result
}
