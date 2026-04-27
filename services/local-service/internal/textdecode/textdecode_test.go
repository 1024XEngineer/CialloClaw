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

func TestDecodeSupportedTextEncodings(t *testing.T) {
	fixText := "\u4fee\u590d\u4e71\u7801"

	tests := []struct {
		name         string
		data         []byte
		wantText     string
		wantEncoding string
	}{
		{
			name:         "empty_file_defaults_to_utf8",
			data:         nil,
			wantText:     "",
			wantEncoding: EncodingUTF8,
		},
		{
			name:         "utf8_plain_text",
			data:         []byte("plain notes\nnext line"),
			wantText:     "plain notes\nnext line",
			wantEncoding: EncodingUTF8,
		},
		{
			name:         "utf8_with_bom",
			data:         utf8WithBOM("bom notes"),
			wantText:     "bom notes",
			wantEncoding: EncodingUTF8,
		},
		{
			name:         "utf8_literal_replacement_rune",
			data:         []byte("keep \uFFFD as authored"),
			wantText:     "keep \uFFFD as authored",
			wantEncoding: EncodingUTF8,
		},
		{
			name:         "utf16le_with_bom",
			data:         utf16LEWithBOM("plain " + fixText),
			wantText:     "plain " + fixText,
			wantEncoding: EncodingUTF16LE,
		},
		{
			name:         "utf16be_with_bom",
			data:         utf16BEWithBOM("plain " + fixText),
			wantText:     "plain " + fixText,
			wantEncoding: EncodingUTF16BE,
		},
		{
			name:         "gb18030_chinese_text",
			data:         gb18030Encoded(t, fixText),
			wantText:     fixText,
			wantEncoding: EncodingGB18030,
		},
		{
			name:         "gb18030_latin_accents_and_symbol",
			data:         gb18030Encoded(t, "R\u00e9sum\u00e9: caf\u00e9 \u20ac"),
			wantText:     "R\u00e9sum\u00e9: caf\u00e9 \u20ac",
			wantEncoding: EncodingGB18030,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Decode(tc.data)
			if err != nil {
				t.Fatalf("Decode returned error: %v", err)
			}
			if result.Text != tc.wantText || result.Encoding != tc.wantEncoding {
				t.Fatalf("unexpected result: %+v, want text=%q encoding=%s", result, tc.wantText, tc.wantEncoding)
			}
			if !strings.ContainsRune(tc.wantText, '\uFFFD') && strings.ContainsRune(result.Text, '\uFFFD') {
				t.Fatalf("decoded text contains generated replacement rune: %q", result.Text)
			}
		})
	}
}

func TestDecodeRejectsUnsafeText(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "binary_control_bytes",
			data: []byte{0x00, 0x01, 0x02, 0xff},
		},
		{
			name: "utf8_nul_control",
			data: []byte("a\x00b"),
		},
		{
			name: "utf8_nonprinting_control",
			data: []byte("a\x01b"),
		},
		{
			name: "utf16le_decoded_nul_control",
			data: utf16LEWithBOM("a\x00b"),
		},
		{
			name: "utf16be_decoded_nonprinting_control",
			data: utf16BEWithBOM("a\x01b"),
		},
		{
			name: "truncated_utf16le_bom_payload",
			data: []byte{0xff, 0xfe, 0x41},
		},
		{
			name: "malformed_gb18030_replacement_sequence",
			data: []byte{0x81, 0x30, 0x81},
		},
		{
			name: "noncanonical_gb18030_single_byte_euro",
			data: []byte{0x80},
		},
		{
			name: "shift_jis_mojibake_is_not_accepted_as_gb18030",
			data: shiftJISEncoded(t, "こんにちは"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(tc.data)
			if !errors.Is(err, ErrUnsupportedEncoding) {
				t.Fatalf("expected ErrUnsupportedEncoding, got %v", err)
			}
		})
	}
}

func gb18030Encoded(t *testing.T, value string) []byte {
	t.Helper()
	encoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(value))
	if err != nil {
		t.Fatalf("GB18030 encode failed: %v", err)
	}
	return encoded
}

func shiftJISEncoded(t *testing.T, value string) []byte {
	t.Helper()
	encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(value))
	if err != nil {
		t.Fatalf("Shift-JIS encode failed: %v", err)
	}
	return encoded
}

func utf8WithBOM(value string) []byte {
	result := []byte{0xEF, 0xBB, 0xBF}
	return append(result, []byte(value)...)
}

func utf16LEWithBOM(value string) []byte {
	units := utf16.Encode([]rune(value))
	result := []byte{0xFF, 0xFE}
	for _, unit := range units {
		result = append(result, byte(unit), byte(unit>>8))
	}
	return result
}

func utf16BEWithBOM(value string) []byte {
	units := utf16.Encode([]rune(value))
	result := []byte{0xFE, 0xFF}
	for _, unit := range units {
		result = append(result, byte(unit>>8), byte(unit))
	}
	return result
}
