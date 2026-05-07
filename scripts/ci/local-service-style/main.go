package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	localServicePath = "services/local-service"
	styleToolPath    = "scripts/ci/local-service-style"
	goimportsTool    = "golang.org/x/tools/cmd/goimports"
	goimportsRunCmd  = "go run " + goimportsTool
)

var hunkHeaderPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

type violation struct {
	file string
	line int
	text string
}

type addedLine struct {
	file string
	line int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		baseRef      string
		skipComments bool
		skipImports  bool
	)
	flag.StringVar(&baseRef, "base", "", "git revision used as the diff base for added-comment checks")
	flag.BoolVar(&skipComments, "skip-comments", false, "skip added-comment checks")
	flag.BoolVar(&skipImports, "skip-goimports", false, "skip goimports formatting checks")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		return err
	}

	if !skipImports {
		if err := checkGoimports(root, baseRef); err != nil {
			return err
		}
	}
	if !skipComments {
		if err := checkAddedComments(root, baseRef); err != nil {
			return err
		}
	}
	return nil
}

func repoRoot() (string, error) {
	output, err := commandOutput("", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w\n%s", err, strings.TrimSpace(output))
	}
	return strings.TrimSpace(output), nil
}

func checkGoimports(root, baseRef string) error {
	files, err := changedGoFiles(root, strings.TrimSpace(baseRef))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	args := []string{"run", goimportsTool, "-l"}
	args = append(args, files...)
	output, err := commandOutput(root, "go", args...)
	if err != nil {
		return fmt.Errorf("run goimports check: %w\n%s", err, strings.TrimSpace(output))
	}

	unformattedFiles := filterReportedPaths(output, files)
	if len(unformattedFiles) == 0 {
		return nil
	}

	return fmt.Errorf(
		"goimports is required for:\n%s\nrun: %s -w %s %s",
		strings.Join(unformattedFiles, "\n"),
		goimportsRunCmd,
		localServicePath,
		styleToolPath,
	)
}

func changedGoFiles(root, baseRef string) ([]string, error) {
	output, err := collectNameOnlyDiff(root, baseRef, localServicePath, styleToolPath)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var files []string
	for _, line := range strings.Split(output, "\n") {
		file := strings.TrimSpace(line)
		if file == "" || !strings.HasSuffix(file, ".go") || seen[file] {
			continue
		}
		seen[file] = true
		files = append(files, file)
	}
	return files, nil
}

func checkAddedComments(root, baseRef string) error {
	diff, err := collectDiff(root, strings.TrimSpace(baseRef))
	if err != nil {
		return err
	}

	violations, err := findCommentViolations(root, diff)
	if err != nil {
		return err
	}
	if len(violations) == 0 {
		return nil
	}

	var builder strings.Builder
	builder.WriteString("Chinese characters are not allowed in added Go comments:\n")
	for _, item := range violations {
		fmt.Fprintf(&builder, "%s:%d: %s\n", item.file, item.line, strings.TrimSpace(item.text))
	}
	return errors.New(strings.TrimRight(builder.String(), "\n"))
}

func collectDiff(root, baseRef string) (string, error) {
	if baseRef != "" && !isZeroRevision(baseRef) {
		if diff, err := gitDiff(root, baseRef+"...HEAD"); err == nil {
			return diff, nil
		}
		return gitDiff(root, baseRef, "HEAD")
	}

	var combined strings.Builder
	for _, args := range [][]string{
		{"diff", "--cached", "--unified=0", "--", localServicePath},
		{"diff", "--unified=0", "--", localServicePath},
	} {
		output, err := commandOutput(root, "git", args...)
		if err != nil {
			return "", fmt.Errorf("collect local diff: %w\n%s", err, strings.TrimSpace(output))
		}
		combined.WriteString(output)
	}
	return combined.String(), nil
}

func collectNameOnlyDiff(root, baseRef string, paths ...string) (string, error) {
	if baseRef != "" && !isZeroRevision(baseRef) {
		if output, err := gitNameOnlyDiff(root, []string{baseRef + "...HEAD"}, paths); err == nil {
			return output, nil
		}
		return gitNameOnlyDiff(root, []string{baseRef, "HEAD"}, paths)
	}

	var combined strings.Builder
	for _, args := range [][]string{
		{"diff", "--cached", "--name-only", "--diff-filter=ACMRT", "--"},
		{"diff", "--name-only", "--diff-filter=ACMRT", "--"},
	} {
		fullArgs := append(args, paths...)
		output, err := commandOutput(root, "git", fullArgs...)
		if err != nil {
			return "", fmt.Errorf("collect changed file list: %w\n%s", err, strings.TrimSpace(output))
		}
		combined.WriteString(output)
	}
	return combined.String(), nil
}

func gitDiff(root string, revisions ...string) (string, error) {
	args := []string{"diff", "--unified=0"}
	args = append(args, revisions...)
	args = append(args, "--", localServicePath)
	output, err := commandOutput(root, "git", args...)
	if err != nil {
		return "", fmt.Errorf("collect diff: %w\n%s", err, strings.TrimSpace(output))
	}
	return output, nil
}

func gitNameOnlyDiff(root string, revisions, paths []string) (string, error) {
	args := []string{"diff", "--name-only", "--diff-filter=ACMRT"}
	args = append(args, revisions...)
	args = append(args, "--")
	args = append(args, paths...)
	output, err := commandOutput(root, "git", args...)
	if err != nil {
		return "", fmt.Errorf("collect changed file list: %w\n%s", err, strings.TrimSpace(output))
	}
	return output, nil
}

func commandOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.String(), err
}

func isZeroRevision(revision string) bool {
	trimmed := strings.Trim(revision, "0")
	return trimmed == ""
}

func findCommentViolations(root, diff string) ([]violation, error) {
	addedLines := collectAddedLines(diff)
	if len(addedLines) == 0 {
		return nil, nil
	}

	commentLinesByFile := make(map[string]map[int][]string)
	var violations []violation
	for _, item := range addedLines {
		commentLines := commentLinesByFile[item.file]
		if commentLines == nil {
			var err error
			commentLines, err = scanCommentLines(root, item.file)
			if err != nil {
				return nil, err
			}
			commentLinesByFile[item.file] = commentLines
		}
		for _, comment := range commentLines[item.line] {
			if containsHan(comment) {
				violations = append(violations, violation{file: item.file, line: item.line, text: comment})
			}
		}
	}

	return violations, nil
}

func collectAddedLines(diff string) []addedLine {
	var addedLines []addedLine
	var (
		file    string
		newLine int
	)

	for _, rawLine := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(rawLine, "diff --git "):
			file = ""
			newLine = 0
		case strings.HasPrefix(rawLine, "+++ "):
			file = parseNewFile(rawLine)
			newLine = 0
		case strings.HasPrefix(rawLine, "@@ "):
			newLine = parseNewLine(rawLine)
		case file == "" || newLine == 0:
			continue
		case strings.HasPrefix(rawLine, "+") && !strings.HasPrefix(rawLine, "+++"):
			addedLines = append(addedLines, addedLine{file: file, line: newLine})
			newLine++
		case strings.HasPrefix(rawLine, "-") && !strings.HasPrefix(rawLine, "---"):
			continue
		case strings.HasPrefix(rawLine, `\`):
			continue
		default:
			newLine++
		}
	}

	return addedLines
}

func parseNewFile(line string) string {
	path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
	if path == "/dev/null" {
		return ""
	}
	path = strings.TrimPrefix(path, "b/")
	if !strings.HasSuffix(path, ".go") {
		return ""
	}
	if !strings.HasPrefix(path, localServicePath+"/") && path != localServicePath {
		return ""
	}
	return path
}

func parseNewLine(line string) int {
	matches := hunkHeaderPattern.FindStringSubmatch(line)
	if len(matches) != 2 {
		return 0
	}
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return value
}

func scanCommentLines(root, relativePath string) (map[int][]string, error) {
	source, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		return nil, fmt.Errorf("read %s for comment scan: %w", relativePath, err)
	}

	fset := token.NewFileSet()
	file := fset.AddFile(relativePath, -1, len(source))
	var scan scanner.Scanner
	scan.Init(file, source, nil, scanner.ScanComments)

	commentLines := make(map[int][]string)
	for {
		pos, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.COMMENT {
			continue
		}
		startLine := fset.Position(pos).Line
		for offset, line := range strings.Split(lit, "\n") {
			commentLines[startLine+offset] = append(commentLines[startLine+offset], line)
		}
	}

	return commentLines, nil
}

func filterReportedPaths(output string, expected []string) []string {
	allowed := make(map[string]struct{}, len(expected))
	for _, file := range expected {
		allowed[file] = struct{}{}
	}

	var filtered []string
	for _, line := range strings.Split(output, "\n") {
		file := strings.TrimSpace(line)
		if file == "" {
			continue
		}
		if _, ok := allowed[file]; ok {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func containsHan(text string) bool {
	for _, value := range text {
		if unicode.In(value, unicode.Han) {
			return true
		}
	}
	return false
}
