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

func (s stubReadFileInfo) Name() string       { return s.name }
func (s stubReadFileInfo) Size() int64        { return s.size }
func (s stubReadFileInfo) Mode() fs.FileMode  { return 0o644 }
func (s stubReadFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubReadFileInfo) IsDir() bool        { return false }
func (s stubReadFileInfo) Sys() any           { return nil }

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

func TestReadFileToolReturnsExplicitMessageForUnsafeText(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "unsafe.txt")
	platform.files[target] = []byte{0x00, 0x01, 0x02, 0xFF}
	tool := NewReadFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": target})
	if err != nil {
		t.Fatalf("Execute should return an explicit tool result instead of an execution error: %v", err)
	}
	if content, _ := result.RawOutput["content"].(string); content != "" {
		t.Fatalf("expected unsafe content to stay empty, got %q", content)
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
