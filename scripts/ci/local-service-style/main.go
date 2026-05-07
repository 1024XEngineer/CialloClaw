package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	localServicePath = "services/local-service"
	styleToolPath    = "scripts/ci/local-service-style"
)

var hunkHeaderPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

type violation struct {
	file string
	line int
	text string
}

type commentState struct {
	inBlock bool
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

	args := []string{"run", "golang.org/x/tools/cmd/goimports@latest", "-l"}
	args = append(args, files...)
	output, err := commandOutput(root, "go", args...)
	if err != nil {
		return fmt.Errorf("run goimports check: %w\n%s", err, strings.TrimSpace(output))
	}
	if strings.TrimSpace(output) == "" {
		return nil
	}

	return fmt.Errorf(
		"goimports is required for:\n%s\nrun: go run golang.org/x/tools/cmd/goimports@latest -w %s %s",
		strings.TrimSpace(output),
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

	violations := findCommentViolations(diff)
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

func findCommentViolations(diff string) []violation {
	var violations []violation
	states := make(map[string]*commentState)

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
			if file != "" && states[file] == nil {
				states[file] = &commentState{}
			}
		case strings.HasPrefix(rawLine, "@@ "):
			newLine = parseNewLine(rawLine)
		case file == "" || newLine == 0:
			continue
		case strings.HasPrefix(rawLine, "+") && !strings.HasPrefix(rawLine, "+++"):
			added := strings.TrimPrefix(rawLine, "+")
			comment, ok := addedComment(added, states[file])
			if ok && containsHan(comment) {
				violations = append(violations, violation{file: file, line: newLine, text: comment})
			}
			newLine++
		case strings.HasPrefix(rawLine, "-") && !strings.HasPrefix(rawLine, "---"):
			continue
		case strings.HasPrefix(rawLine, `\`):
			continue
		default:
			newLine++
		}
	}

	return violations
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

func addedComment(line string, state *commentState) (string, bool) {
	if state == nil {
		state = &commentState{}
	}

	if state.inBlock {
		if strings.Contains(line, "*/") {
			state.inBlock = false
		}
		return line, true
	}

	index, block := commentStart(line)
	if index < 0 {
		return "", false
	}

	comment := line[index:]
	if block && !strings.Contains(comment, "*/") {
		state.inBlock = true
	}
	return comment, true
}

func commentStart(line string) (int, bool) {
	var quote byte
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if quote != 0 {
			switch quote {
			case '`':
				if ch == '`' {
					quote = 0
				}
			default:
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == quote {
					quote = 0
				}
			}
			continue
		}

		switch ch {
		case '"', '\'', '`':
			quote = ch
		case '/':
			if i+1 >= len(line) {
				continue
			}
			if line[i+1] == '/' {
				return i, false
			}
			if line[i+1] == '*' {
				return i, true
			}
		}
	}

	return -1, false
}

func containsHan(text string) bool {
	for _, value := range text {
		if unicode.In(value, unicode.Han) {
			return true
		}
	}
	return false
}
