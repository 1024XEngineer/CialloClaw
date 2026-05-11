package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindCommentViolationsRejectsAddedChineseComments(t *testing.T) {
	root := writeTestFile(t, "package demo\n// Load loads 配置.\nfunc Load() {}\nvar _ = 1 /* 中文 block */\n")
	diff := `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -1,0 +2,4 @@
+package demo
+// Load loads 配置.
+func Load() {}
+var _ = 1 /* 中文 block */
`

	violations, err := findCommentViolations(root, []diffChunk{{
		diff:     diff,
		readFile: workingTreeFileReader,
	}})
	if err != nil {
		t.Fatalf("findCommentViolations returned error: %v", err)
	}
	if len(violations) != 2 {
		t.Fatalf("expected two violations, got %d: %#v", len(violations), violations)
	}
	if violations[0].line != 2 {
		t.Fatalf("expected first violation on line 2, got %d", violations[0].line)
	}
	if violations[1].line != 4 {
		t.Fatalf("expected second violation on line 4, got %d", violations[1].line)
	}
}

func TestFindAllCommentViolationsRejectsExistingChineseComments(t *testing.T) {
	root := writeTestFile(t, "package demo\n// existing 中文 comment.\nconst message = \"中文 string is allowed\"\n")

	violations, err := findAllCommentViolations(root)
	if err != nil {
		t.Fatalf("findAllCommentViolations returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one violation, got %d: %#v", len(violations), violations)
	}
	if violations[0].line != 2 {
		t.Fatalf("expected violation on line 2, got %d", violations[0].line)
	}
}

func TestFindAllCommentViolationsIgnoresChineseStringLiterals(t *testing.T) {
	root := writeTestFile(t, "package demo\nconst message = \"中文 // not a comment\"\nconst raw = `中文 /* not a comment */`\n")

	violations, err := findAllCommentViolations(root)
	if err != nil {
		t.Fatalf("findAllCommentViolations returned error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %#v", violations)
	}
}

func TestFindCommentViolationsIgnoresChineseStringLiterals(t *testing.T) {
	root := writeTestFile(t, "package demo\nconst message = \"中文 // not a comment\"\nconst raw = `中文 /* not a comment */`\n")
	diff := `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -1,0 +2,3 @@
+package demo
+const message = "中文 // not a comment"
+const raw = ` + "`" + `中文 /* not a comment */` + "`" + `
`

	violations, err := findCommentViolations(root, []diffChunk{{
		diff:     diff,
		readFile: workingTreeFileReader,
	}})
	if err != nil {
		t.Fatalf("findCommentViolations returned error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %#v", violations)
	}
}

func TestFindCommentViolationsTracksAddedBlockComments(t *testing.T) {
	root := writeTestFile(t, "package demo\n/*\nEnglish line.\n中文 line.\n*/\n")
	diff := `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -1,0 +2,5 @@
+package demo
+/*
+English line.
+中文 line.
+*/
`

	violations, err := findCommentViolations(root, []diffChunk{{
		diff:     diff,
		readFile: workingTreeFileReader,
	}})
	if err != nil {
		t.Fatalf("findCommentViolations returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one violation, got %d: %#v", len(violations), violations)
	}
	if violations[0].line != 4 {
		t.Fatalf("expected violation on line 4, got %d", violations[0].line)
	}
}

func TestFindCommentViolationsTracksAddedLinesInsideExistingBlockComments(t *testing.T) {
	root := writeTestFile(t, "package demo\n/*\nEnglish line.\n中文 line.\n*/\nfunc Load() {}\n")
	diff := `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -3,0 +4 @@
+中文 line.
`

	violations, err := findCommentViolations(root, []diffChunk{{
		diff:     diff,
		readFile: workingTreeFileReader,
	}})
	if err != nil {
		t.Fatalf("findCommentViolations returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one violation, got %d: %#v", len(violations), violations)
	}
	if violations[0].line != 4 {
		t.Fatalf("expected violation on line 4, got %d", violations[0].line)
	}
}

func TestFilterReportedPathsIgnoresDownloadNoise(t *testing.T) {
	output := "go: downloading golang.org/x/tools v0.44.0\nservices/local-service/internal/demo/demo.go\n"
	filtered := filterReportedPaths(output, []string{
		"services/local-service/internal/demo/demo.go",
		"scripts/ci/local-service-style/main.go",
	})
	if len(filtered) != 1 {
		t.Fatalf("expected one reported path, got %d: %#v", len(filtered), filtered)
	}
	if filtered[0] != "services/local-service/internal/demo/demo.go" {
		t.Fatalf("unexpected reported path %q", filtered[0])
	}
}

func TestFormatGoimportsCommandTargetsReportedFiles(t *testing.T) {
	command := formatGoimportsCommand([]string{
		"services/local-service/internal/orchestrator/task_confirm.go",
		"scripts/ci/local-service-style/main.go",
	})
	want := "go run golang.org/x/tools/cmd/goimports -w \"services/local-service/internal/orchestrator/task_confirm.go\" \"scripts/ci/local-service-style/main.go\""
	if command != want {
		t.Fatalf("unexpected goimports command: %q", command)
	}
}

func TestFindCommentViolationsSeparatesIndexAndWorkingTreeSnapshots(t *testing.T) {
	const path = "services/local-service/internal/demo/demo.go"

	indexSource := []byte("package demo\n// 中文 staged comment.\nfunc Load() {}\n")
	workingTreeSource := []byte("package demo\n\n// English working tree comment.\nfunc Load() {}\n// 中文 working tree comment.\n")

	diffChunks := []diffChunk{
		{
			diff: `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -1,0 +2,2 @@
+// 中文 staged comment.
+func Load() {}
`,
			readFile: staticFileReader(path, indexSource),
		},
		{
			diff: `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 2222222..3333333 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -4,0 +5 @@
+// 中文 working tree comment.
`,
			readFile: staticFileReader(path, workingTreeSource),
		},
	}

	violations, err := findCommentViolations(t.TempDir(), diffChunks)
	if err != nil {
		t.Fatalf("findCommentViolations returned error: %v", err)
	}
	if len(violations) != 2 {
		t.Fatalf("expected two violations, got %d: %#v", len(violations), violations)
	}
	if violations[0].line != 2 || violations[0].text != "// 中文 staged comment." {
		t.Fatalf("unexpected staged violation: %#v", violations[0])
	}
	if violations[1].line != 5 || violations[1].text != "// 中文 working tree comment." {
		t.Fatalf("unexpected working tree violation: %#v", violations[1])
	}
}

func TestFindFileSizeViolationsRejectsLargeImplementationFiles(t *testing.T) {
	root := writeTestFile(t, "package demo\n")
	const path = "services/local-service/internal/demo/large.go"
	largePath := filepath.Join(root, localServicePath, "internal", "demo", "large.go")
	if err := os.WriteFile(largePath, []byte(strings.Repeat("package_line\n", maxGoFileLines+1)), 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	violations, err := findFileSizeViolations(root, []diffChunk{{
		diff:         diffForFiles(path, "services/local-service/internal/demo/large_test.go"),
		readFile:     workingTreeFileReader,
		previousFile: staticFileReader(path, []byte("package demo\n")),
	}})
	if err != nil {
		t.Fatalf("findFileSizeViolations returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one size violation, got %d: %#v", len(violations), violations)
	}
	if violations[0].file != path {
		t.Fatalf("unexpected violation file: %#v", violations[0])
	}
}

func TestFindFileSizeViolationsReadsDiffSnapshot(t *testing.T) {
	const path = "services/local-service/internal/demo/large.go"
	root := writeTestFile(t, "package demo\n")
	stagedSource := []byte(strings.Repeat("package_line\n", maxGoFileLines+1))

	violations, err := findFileSizeViolations(root, []diffChunk{{
		diff:         diffForFiles(path),
		readFile:     staticFileReader(path, stagedSource),
		previousFile: staticFileReader(path, []byte("package demo\n")),
	}})
	if err != nil {
		t.Fatalf("findFileSizeViolations returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one size violation from diff snapshot, got %d: %#v", len(violations), violations)
	}
	if violations[0].file != path {
		t.Fatalf("unexpected violation file: %#v", violations[0])
	}
}

func TestFindFileSizeViolationsAllowsPreexistingOversizeFilesWithoutGrowth(t *testing.T) {
	const path = "services/local-service/internal/demo/large.go"
	root := writeTestFile(t, "package demo\n")
	source := []byte(strings.Repeat("package_line\n", maxGoFileLines+1))

	violations, err := findFileSizeViolations(root, []diffChunk{{
		diff:         diffForFiles(path),
		readFile:     staticFileReader(path, source),
		previousFile: staticFileReader(path, source),
	}})
	if err != nil {
		t.Fatalf("findFileSizeViolations returned error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violation for unchanged oversize file, got %#v", violations)
	}
}

func TestFindFileSizeViolationsRejectsOversizeGrowth(t *testing.T) {
	const path = "services/local-service/internal/demo/large.go"
	root := writeTestFile(t, "package demo\n")
	before := []byte(strings.Repeat("package_line\n", maxGoFileLines+1))
	after := []byte(strings.Repeat("package_line\n", maxGoFileLines+2))

	violations, err := findFileSizeViolations(root, []diffChunk{{
		diff:         diffForFiles(path),
		readFile:     staticFileReader(path, after),
		previousFile: staticFileReader(path, before),
	}})
	if err != nil {
		t.Fatalf("findFileSizeViolations returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one violation for oversize growth, got %d: %#v", len(violations), violations)
	}
	if violations[0].lines != maxGoFileLines+2 {
		t.Fatalf("expected grown line count, got %#v", violations[0])
	}
}

func TestFindFileSizeViolationsAllowsOversizeRenameWithoutGrowth(t *testing.T) {
	const (
		oldPath = "services/local-service/internal/demo/old_large.go"
		newPath = "services/local-service/internal/demo/new_large.go"
	)
	root := writeTestFile(t, "package demo\n")
	source := []byte(strings.Repeat("package_line\n", maxGoFileLines+1))

	violations, err := findFileSizeViolations(root, []diffChunk{{
		diff:         diffForRenamedFile(oldPath, newPath),
		readFile:     staticFileReader(newPath, source),
		previousFile: staticFileReader(oldPath, source),
	}})
	if err != nil {
		t.Fatalf("findFileSizeViolations returned error: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violation for unchanged oversize rename, got %#v", violations)
	}
}

func TestFindFileSizeViolationsRejectsOversizeRenameGrowth(t *testing.T) {
	const (
		oldPath = "services/local-service/internal/demo/old_large.go"
		newPath = "services/local-service/internal/demo/new_large.go"
	)
	root := writeTestFile(t, "package demo\n")
	before := []byte(strings.Repeat("package_line\n", maxGoFileLines+1))
	after := []byte(strings.Repeat("package_line\n", maxGoFileLines+2))

	violations, err := findFileSizeViolations(root, []diffChunk{{
		diff:         diffForRenamedFile(oldPath, newPath),
		readFile:     staticFileReader(newPath, after),
		previousFile: staticFileReader(oldPath, before),
	}})
	if err != nil {
		t.Fatalf("findFileSizeViolations returned error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one violation for oversize rename growth, got %d: %#v", len(violations), violations)
	}
	if violations[0].file != newPath || violations[0].lines != maxGoFileLines+2 {
		t.Fatalf("expected renamed growth violation to track new path and grown line count, got %#v", violations[0])
	}
}

func TestCountLinesHandlesEmptyAndTrailingNewline(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
		want int
	}{
		{name: "empty", text: "", want: 0},
		{name: "one unterminated", text: "a", want: 1},
		{name: "one terminated", text: "a\n", want: 1},
		{name: "two terminated", text: "a\nb\n", want: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := countLines([]byte(tt.text)); got != tt.want {
				t.Fatalf("countLines(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func writeTestFile(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()
	path := filepath.Join(root, localServicePath, "internal", "demo", "demo.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return root
}

func diffForFiles(paths ...string) string {
	var builder strings.Builder
	for _, path := range paths {
		builder.WriteString("diff --git a/")
		builder.WriteString(path)
		builder.WriteString(" b/")
		builder.WriteString(path)
		builder.WriteString("\nindex 1111111..2222222 100644\n--- a/")
		builder.WriteString(path)
		builder.WriteString("\n+++ b/")
		builder.WriteString(path)
		builder.WriteString("\n@@ -1,0 +1 @@\n+package demo\n")
	}
	return builder.String()
}

func diffForRenamedFile(oldPath, newPath string) string {
	return fmt.Sprintf(`diff --git a/%s b/%s
similarity index 100%%
rename from %s
rename to %s
--- a/%s
+++ b/%s
@@ -1,0 +1 @@
+package demo
`, oldPath, newPath, oldPath, newPath, oldPath, newPath)
}

func staticFileReader(expectedPath string, source []byte) fileReader {
	return func(_ string, relativePath string) ([]byte, error) {
		if relativePath != expectedPath {
			return nil, fmt.Errorf("unexpected path %q", relativePath)
		}
		return source, nil
	}
}
