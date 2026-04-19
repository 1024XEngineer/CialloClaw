package sidecarclient

import (
	"errors"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestSidecarRuntimeHelpers(t *testing.T) {
	response, err := decodeSidecarResponse([]byte(` {"ok":true,"result":{"status":"ok"}} `))
	if err != nil {
		t.Fatalf("decodeSidecarResponse returned error: %v", err)
	}
	if !response.OK || stringValue(response.Result, "status") != "ok" {
		t.Fatalf("unexpected decoded response: %+v", response)
	}

	if _, err := decodeSidecarResponse(nil); err == nil {
		t.Fatal("expected empty payload to fail decoding")
	}

	errMap := responseErrorMap(&sidecarErrorBody{Code: "bad_request", Message: "broken"})
	if errMap["code"] != "bad_request" || errMap["message"] != "broken" {
		t.Fatalf("unexpected error map: %+v", errMap)
	}
	if responseErrorMap(nil) != nil {
		t.Fatal("expected nil error body to produce nil map")
	}

	cmdErr := commandWorkerError(errors.New("exit status 1"), " worker crashed ")
	if cmdErr.Error() != "worker command failed: worker crashed" {
		t.Fatalf("unexpected command worker error: %v", cmdErr)
	}
	baseErr := errors.New("exit status 2")
	if commandWorkerError(baseErr, " ") != baseErr {
		t.Fatal("expected empty stderr to preserve original error")
	}

	if !shouldMarkRuntimeFailure(sidecarTransportError{err: errors.New("boom")}) {
		t.Fatal("expected transport errors to mark runtime failure")
	}
	if shouldMarkRuntimeFailure(sidecarRequestError{code: "bad_request", message: "broken"}) {
		t.Fatal("expected request errors not to mark runtime failure")
	}

	if got := sidecarPipeName("ocr_worker"); got != "cialloclaw-ocr_worker" {
		t.Fatalf("unexpected pipe name: %q", got)
	}
	if got := stringValue(nil, "missing"); got != "" {
		t.Fatalf("expected blank string value, got %q", got)
	}
	if got := intValue(map[string]any{"count": 3.0}, "count"); got != 3 {
		t.Fatalf("expected float64 int value conversion, got %d", got)
	}
	if got := stringSliceValue(map[string]any{"items": []any{"a", " ", "b"}}, "items"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected string slice value: %+v", got)
	}
}

func TestMediaAndOCRHelpers(t *testing.T) {
	if err := validateMediaIO(map[string]any{"path": "in.mov", "output_path": "out.mp4"}, true); err != nil {
		t.Fatalf("expected validateMediaIO to pass, got %v", err)
	}
	if err := validateMediaIO(map[string]any{"path": "in.mov"}, true); err == nil {
		t.Fatal("expected missing output path to fail validation")
	}
	if err := validateMediaIO(map[string]any{"output_path": "out.mp4"}, false); err == nil {
		t.Fatal("expected missing input path to fail validation")
	}

	if err := validateMediaOutputDir(map[string]any{"path": "in.mov", "output_dir": "frames"}); err != nil {
		t.Fatalf("expected validateMediaOutputDir to pass, got %v", err)
	}
	if err := validateMediaOutputDir(map[string]any{"path": "in.mov"}); err == nil {
		t.Fatal("expected missing output dir to fail validation")
	}

	if got := guessMediaMimeType("clip.mp4"); got != "video/mp4" {
		t.Fatalf("unexpected mp4 mime type: %q", got)
	}
	if got := guessMediaMimeType("clip.mp3"); got != "audio/mpeg" {
		t.Fatalf("unexpected mp3 mime type: %q", got)
	}
	if got := guessMediaMimeType("clip.wav"); got != "audio/wav" {
		t.Fatalf("unexpected wav mime type: %q", got)
	}
	if got := guessMediaMimeType("clip.bin"); got != "application/octet-stream" {
		t.Fatalf("unexpected fallback mime type: %q", got)
	}

	toolResult := buildOCRToolResult("extract_text", OCRTextResult())
	if toolResult.ToolName != "extract_text" {
		t.Fatalf("unexpected OCR tool result: %+v", toolResult)
	}
	if toolResult.SummaryOutput["source"] != "ocr_worker" {
		t.Fatalf("expected default OCR source, got %+v", toolResult.SummaryOutput)
	}

	mediaResult := buildMediaTranscodeResult("transcode_media", MediaTranscodeResult())
	if mediaResult.ToolName != "transcode_media" || len(mediaResult.Artifacts) != 1 {
		t.Fatalf("unexpected media tool result: %+v", mediaResult)
	}
	if mediaResult.Artifacts[0].MimeType != "application/octet-stream" {
		t.Fatalf("expected fallback media mime type, got %+v", mediaResult.Artifacts[0])
	}
}

func TestPlaywrightHelpers(t *testing.T) {
	if got := mapSliceValue(map[string]any{"actions": []any{map[string]any{"type": "click"}, "skip"}}, "actions"); len(got) != 1 || got[0]["type"] != "click" {
		t.Fatalf("unexpected action slice from []any: %+v", got)
	}
	if got := mapSliceValue(map[string]any{"actions": []map[string]any{{"type": "fill"}}}, "actions"); len(got) != 1 || got[0]["type"] != "fill" {
		t.Fatalf("unexpected action slice from []map: %+v", got)
	}
	if got := mapSliceValue(map[string]any{"actions": "skip"}, "actions"); got != nil {
		t.Fatalf("expected nil actions for invalid type, got %+v", got)
	}

	if got := cloneActionMap(nil); got != nil {
		t.Fatalf("expected nil clone for empty action map, got %+v", got)
	}
	clonedMap := cloneActionMap(map[string]any{"type": "click"})
	clonedMap["type"] = "changed"
	if clonedMap["type"] != "changed" {
		t.Fatalf("expected cloned map mutation to succeed, got %+v", clonedMap)
	}

	if got := cloneActionSlice(nil); got != nil {
		t.Fatalf("expected nil clone for empty action slice, got %+v", got)
	}
	original := []map[string]any{{"type": "click"}}
	clonedSlice := cloneActionSlice(original)
	clonedSlice[0]["type"] = "fill"
	if original[0]["type"] != "click" {
		t.Fatalf("expected deep-cloned action slice, got original=%+v cloned=%+v", original, clonedSlice)
	}

	if got := stringValueMap(map[string]any{"url": " https://example.com "}, "url"); got != "https://example.com" {
		t.Fatalf("unexpected trimmed string map value: %q", got)
	}
	if got := previewPageText("  hello world  "); got != "hello world" {
		t.Fatalf("unexpected short preview text: %q", got)
	}
	if got := previewPageText(strings.Repeat("a", pageTextPreviewLimit+10)); len(got) != pageTextPreviewLimit {
		t.Fatalf("expected truncated preview length %d, got %d", pageTextPreviewLimit, len(got))
	}
	if got := firstNonEmptyString(" ", " alpha ", "beta"); got != "alpha" {
		t.Fatalf("unexpected first non-empty string: %q", got)
	}
}

func OCRTextResult() tools.OCRTextResult {
	return tools.OCRTextResult{Path: "workspace/demo.txt", Text: "hello world", Language: "plain_text", PageCount: 1}
}

func MediaTranscodeResult() tools.MediaTranscodeResult {
	return tools.MediaTranscodeResult{InputPath: "workspace/demo.mov", OutputPath: "workspace/demo.bin", Format: "bin"}
}
