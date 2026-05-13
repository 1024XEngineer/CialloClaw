package builtin

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type stubReadFilePlatform struct {
	workspaceRoot string
	files         map[string][]byte
	outOfScope    map[string]bool
	readErr       error
}

func newStubReadFilePlatform(workspaceRoot string) *stubReadFilePlatform {
	return &stubReadFilePlatform{
		workspaceRoot: workspaceRoot,
		files:         make(map[string][]byte),
		outOfScope:    make(map[string]bool),
	}
}

func (s *stubReadFilePlatform) Join(elem ...string) string { return filepath.Join(elem...) }
func (s *stubReadFilePlatform) Abs(path string) (string, error) {
	if isStubAbsolutePath(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(s.workspaceRoot, path), nil
}
func (s *stubReadFilePlatform) EnsureWithinWorkspace(path string) (string, error) {
	clean := filepath.Clean(path)
	if s.outOfScope[clean] {
		return "", errors.New("outside workspace")
	}
	if isStubAbsolutePath(clean) {
		return clean, nil
	}
	return filepath.Join(s.workspaceRoot, clean), nil
}
func (s *stubReadFilePlatform) ReadDir(path string) ([]fs.DirEntry, error) { return nil, nil }
func (s *stubReadFilePlatform) ReadFile(path string) ([]byte, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	content, ok := s.files[filepath.Clean(path)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), content...), nil
}
func (s *stubReadFilePlatform) WriteFile(path string, content []byte) error { return nil }
func (s *stubReadFilePlatform) Stat(path string) (fs.FileInfo, error) {
	content, ok := s.files[filepath.Clean(path)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return stubReadFileInfo{name: filepath.Base(path), size: int64(len(content))}, nil
}

type stubReadFileInfo struct {
	name string
	size int64
}

type stubReadFileOCR struct {
	result      tools.OCRTextResult
	err         error
	calledPaths []string
}

func (s stubReadFileInfo) Name() string       { return s.name }
func (s stubReadFileInfo) Size() int64        { return s.size }
func (s stubReadFileInfo) Mode() fs.FileMode  { return 0o644 }
func (s stubReadFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubReadFileInfo) IsDir() bool        { return false }
func (s stubReadFileInfo) Sys() any           { return nil }

func (s *stubReadFileOCR) ExtractText(_ context.Context, path string) (tools.OCRTextResult, error) {
	s.calledPaths = append(s.calledPaths, path)
	if s.err != nil {
		return tools.OCRTextResult{}, s.err
	}
	return s.result, nil
}

func (s *stubReadFileOCR) OCRImage(_ context.Context, _, _ string) (tools.OCRTextResult, error) {
	if s.err != nil {
		return tools.OCRTextResult{}, s.err
	}
	return s.result, nil
}

func (s *stubReadFileOCR) OCRPDF(_ context.Context, _, _ string) (tools.OCRTextResult, error) {
	if s.err != nil {
		return tools.OCRTextResult{}, s.err
	}
	return s.result, nil
}

func TestReadFileToolExecuteSuccess(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "demo.txt")
	platform.files[target] = []byte("hello world")
	tool := NewReadFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": target})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["path"] != target || result.RawOutput["mime_type"] != "text/plain" {
		t.Fatalf("unexpected raw output: %+v", result.RawOutput)
	}
	if result.SummaryOutput["content_preview"] != "hello world" {
		t.Fatalf("unexpected summary output: %+v", result.SummaryOutput)
	}
}

func TestReadFileToolDetectsMarkdownMimeType(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "demo.md")
	platform.files[target] = []byte("# title\nhello")
	tool := NewReadFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": target})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["mime_type"] != "text/markdown" {
		t.Fatalf("expected markdown mime type, got %+v", result.RawOutput)
	}
}

func TestReadFileToolDecodesWorkspaceTextEncodings(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	gb18030Path := filepath.Join(workspace, "notes", "legacy.txt")
	gb18030Content, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte("修复乱码"))
	if err != nil {
		t.Fatalf("GB18030 encode failed: %v", err)
	}
	platform.files[gb18030Path] = gb18030Content
	utf16Path := filepath.Join(workspace, "notes", "utf16.txt")
	platform.files[utf16Path] = utf16LEWithBOM("统一提示")
	tool := NewReadFileTool()

	gbResult, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": gb18030Path})
	if err != nil {
		t.Fatalf("Execute returned error for GB18030 file: %v", err)
	}
	if gbResult.RawOutput["content"] != "修复乱码" {
		t.Fatalf("expected decoded GB18030 output, got %+v", gbResult.RawOutput)
	}
	if _, ok := gbResult.RawOutput["text_encoding"]; ok {
		t.Fatalf("read_file raw output must not expose undocumented text_encoding: %+v", gbResult.RawOutput)
	}

	utf16Result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": utf16Path})
	if err != nil {
		t.Fatalf("Execute returned error for UTF-16 file: %v", err)
	}
	if utf16Result.RawOutput["content"] != "统一提示" {
		t.Fatalf("expected decoded UTF-16 output, got %+v", utf16Result.RawOutput)
	}
	if _, ok := utf16Result.SummaryOutput["text_encoding"]; ok {
		t.Fatalf("read_file summary output must not expose undocumented text_encoding: %+v", utf16Result.SummaryOutput)
	}
}

func TestReadFileToolFallsBackToOCRForDocumentAttachments(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "report.docx")
	platform.files[target] = []byte{0x50, 0x4b, 0x03, 0x04, 0xff}
	ocr := &stubReadFileOCR{result: tools.OCRTextResult{Path: target, Text: "Document body text", Language: "docx_text", PageCount: 1, Source: "ocr_worker_docx"}}
	tool := NewReadFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform, OCR: ocr}, map[string]any{"path": target})
	if err != nil {
		t.Fatalf("Execute returned error for docx file: %v", err)
	}
	if len(ocr.calledPaths) != 1 || ocr.calledPaths[0] != target {
		t.Fatalf("expected read_file to invoke OCR for the document path, got %+v", ocr.calledPaths)
	}
	if result.Error != nil {
		t.Fatalf("expected OCR-backed read_file result to succeed, got %+v", result.Error)
	}
	if result.RawOutput["content"] != "Document body text" {
		t.Fatalf("expected OCR-extracted content, got %+v", result.RawOutput)
	}
	if result.SummaryOutput["content_preview"] != "Document body text" {
		t.Fatalf("expected OCR preview content, got %+v", result.SummaryOutput)
	}
	if result.RawOutput["text_type"] == "document_extracted" {
		t.Fatalf("read_file OCR fallback must not expose undocumented text_type values: %+v", result.RawOutput)
	}
	if _, ok := result.RawOutput["extracted"]; ok {
		t.Fatalf("read_file OCR fallback must not expose undocumented extracted flag: %+v", result.RawOutput)
	}
}

func TestReadFileToolReturnsDocumentExtractionFailureForUnreadableDocumentAttachments(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "report.docx")
	platform.files[target] = []byte{0x50, 0x4b, 0x03, 0x04, 0xff}
	ocr := &stubReadFileOCR{err: errors.New("ocr failed")}
	tool := NewReadFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform, OCR: ocr}, map[string]any{"path": target})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected OCR failure to keep tool execution failure semantics, got %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected OCR-backed document failure to return a tool result error, got %+v", result)
	}
	if result.Error.Message != readFileDocumentExtractFailedUserMessage {
		t.Fatalf("expected document extraction failure message, got %+v", result.Error)
	}
	preview, _ := result.SummaryOutput["content_preview"].(string)
	if preview != readFileDocumentExtractFailedUserMessage {
		t.Fatalf("expected document extraction failure preview, got %+v", result.SummaryOutput)
	}
	if strings.Contains(preview, "UTF-8") {
		t.Fatalf("expected document failure to avoid text-encoding guidance, got %q", preview)
	}
}

func TestReadFileToolFailsForUnsafeText(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "unsafe.txt")
	platform.files[target] = []byte{0x00, 0x01, 0x02, 0xFF}
	tool := NewReadFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": target})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected unsafe text to fail the tool call, got %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected tool result with machine-readable error, got %+v", result)
	}
	if !strings.Contains(result.Error.Message, "UTF-8") {
		t.Fatalf("expected decode warning in tool error, got %+v", result.Error)
	}
	if _, ok := result.RawOutput["content"]; ok {
		t.Fatalf("failed read_file raw output must not look like an empty file: %+v", result.RawOutput)
	}
	if _, ok := result.RawOutput["decode_warning"]; ok {
		t.Fatalf("read_file raw output must not expose undocumented decode_warning: %+v", result.RawOutput)
	}
	preview, _ := result.SummaryOutput["content_preview"].(string)
	if !strings.Contains(preview, "UTF-8") || strings.ContainsRune(preview, '\uFFFD') {
		t.Fatalf("expected explicit decode warning without replacement characters, got %q", preview)
	}
}

func TestReadFileToolRejectsOutsideWorkspace(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	outside := filepath.Clean("D:/outside/demo.txt")
	platform.outOfScope[outside] = true
	tool := NewReadFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": outside})
	if !errors.Is(err, tools.ErrWorkspaceBoundaryDenied) {
		t.Fatalf("expected ErrWorkspaceBoundaryDenied, got %v", err)
	}
}

func TestReadFileToolReturnsExecutionErrorWhenReadFails(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	platform.readErr = errors.New("read failed")
	tool := NewReadFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": "notes/demo.txt"})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected ErrToolExecutionFailed, got %v", err)
	}
}

func TestReadFileToolRequiresPlatform(t *testing.T) {
	tool := NewReadFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{}, map[string]any{"path": "notes/demo.txt"})
	if !errors.Is(err, tools.ErrCapabilityDenied) {
		t.Fatalf("expected ErrCapabilityDenied, got %v", err)
	}
}

func TestReadFileToolRejectsOversizedFile(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "large.txt")
	platform.files[target] = make([]byte, readFileMaxBytes+1)
	tool := NewReadFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": target})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected ErrToolExecutionFailed for oversized file, got %v", err)
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
