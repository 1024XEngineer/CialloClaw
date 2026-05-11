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
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	localServicePath = "services/local-service"
	styleToolPath    = "scripts/ci/local-service-style"
	goimportsTool    = "golang.org/x/tools/cmd/goimports"
	goimportsRunCmd  = "go run " + goimportsTool
	maxGoFileLines   = 4000
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

type fileSizeViolation struct {
	file     string
	lines    int
	maxLines int
}

type changedGoFile struct {
	file         string
	previousFile string
}

type diffChunk struct {
	diff         string
	readFile     fileReader
	previousFile fileReader
}

type fileReader func(root, relativePath string) ([]byte, error)

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
		if err := checkAllComments(root); err != nil {
			return err
		}
	}
	if err := checkChangedFileSizes(root, baseRef); err != nil {
		return err
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
	diffChunks, err := collectDiffChunks(root, strings.TrimSpace(baseRef))
	if err != nil {
		return err
	}

	violations, err := findCommentViolations(root, diffChunks)
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

func checkAllComments(root string) error {
	violations, err := findAllCommentViolations(root)
	if err != nil {
		return err
	}
	if len(violations) == 0 {
		return nil
	}

	var builder strings.Builder
	builder.WriteString("Chinese characters are not allowed in local-service Go comments:\n")
	for _, item := range violations {
		fmt.Fprintf(&builder, "%s:%d: %s\n", item.file, item.line, strings.TrimSpace(item.text))
	}
	return errors.New(strings.TrimRight(builder.String(), "\n"))
}

func checkChangedFileSizes(root, baseRef string) error {
	diffChunks, err := collectDiffChunks(root, strings.TrimSpace(baseRef))
	if err != nil {
		return err
	}

	violations, err := findFileSizeViolations(root, diffChunks)
	if err != nil {
		return err
	}
	if len(violations) == 0 {
		return nil
	}

	var builder strings.Builder
	builder.WriteString("changed local-service Go files are too large:\n")
	for _, item := range violations {
		fmt.Fprintf(&builder, "%s: %d lines exceeds %d\n", item.file, item.lines, item.maxLines)
	}
	return errors.New(strings.TrimRight(builder.String(), "\n"))
}

func collectChangedGoFiles(diff string) []changedGoFile {
	seen := make(map[string]bool)
	files := make([]changedGoFile, 0)
	currentPreviousFile := ""
	for _, rawLine := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(rawLine, "diff --git "):
			currentPreviousFile = parsePreviousFile(rawLine)
		case strings.HasPrefix(rawLine, "+++ "):
			file := parseNewFile(rawLine)
			if file == "" || seen[file] {
				continue
			}
			seen[file] = true
			files = append(files, changedGoFile{
				file:         file,
				previousFile: currentPreviousFile,
			})
		}
	}
	return files
}

func collectDiffChunks(root, baseRef string) ([]diffChunk, error) {
	if baseRef != "" && !isZeroRevision(baseRef) {
		baseRevision, err := resolveBaseRevision(root, baseRef)
		if err != nil {
			return nil, err
		}
		if diff, err := gitDiff(root, baseRef+"...HEAD"); err == nil {
			return []diffChunk{{
				diff:         diff,
				readFile:     gitRevisionFileReader("HEAD:", "HEAD"),
				previousFile: gitRevisionFileReader(baseRevision+":", baseRevision),
			}}, nil
		}
		diff, err := gitDiff(root, baseRef, "HEAD")
		if err != nil {
			return nil, err
		}
		return []diffChunk{{
			diff:         diff,
			readFile:     gitRevisionFileReader("HEAD:", "HEAD"),
			previousFile: gitRevisionFileReader(baseRef+":", baseRef),
		}}, nil
	}

	type localDiffConfig struct {
		args     []string
		readFile fileReader
		previous fileReader
	}
	configs := []localDiffConfig{
		{
			args:     []string{"diff", "--cached", "--unified=0", "--", localServicePath},
			readFile: gitRevisionFileReader(":", "index"),
			previous: gitRevisionFileReader("HEAD:", "HEAD"),
		},
		{
			args:     []string{"diff", "--unified=0", "--", localServicePath},
			readFile: workingTreeFileReader,
			previous: gitRevisionFileReader(":", "index"),
		},
	}

	chunks := make([]diffChunk, 0, len(configs))
	for _, config := range configs {
		output, err := commandOutput(root, "git", config.args...)
		if err != nil {
			return nil, fmt.Errorf("collect local diff: %w\n%s", err, strings.TrimSpace(output))
		}
		chunks = append(chunks, diffChunk{
			diff:         output,
			readFile:     config.readFile,
			previousFile: config.previous,
		})
	}
	return chunks, nil
}

func resolveBaseRevision(root, baseRef string) (string, error) {
	output, err := commandOutput(root, "git", "merge-base", baseRef, "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve merge-base for %s: %w\n%s", baseRef, err, strings.TrimSpace(output))
	}
	return strings.TrimSpace(output), nil
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

func findCommentViolations(root string, diffChunks []diffChunk) ([]violation, error) {
	var violations []violation
	for _, chunk := range diffChunks {
		addedLines := collectAddedLines(chunk.diff)
		if len(addedLines) == 0 {
			continue
		}

		commentLinesByFile := make(map[string]map[int][]string)
		for _, item := range addedLines {
			commentLines := commentLinesByFile[item.file]
			if commentLines == nil {
				var err error
				commentLines, err = scanCommentLines(root, item.file, chunk.readFile)
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
	}

	return violations, nil
}

func findAllCommentViolations(root string) ([]violation, error) {
	basePath := filepath.Join(root, localServicePath)
	var violations []violation

	if err := filepath.WalkDir(basePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)

		commentLines, err := scanCommentLines(root, relativePath, workingTreeFileReader)
		if err != nil {
			return err
		}
		for line, comments := range commentLines {
			for _, comment := range comments {
				if containsHan(comment) {
					violations = append(violations, violation{file: relativePath, line: line, text: comment})
				}
			}
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan local-service comments: %w", err)
	}

	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].file == violations[j].file {
			return violations[i].line < violations[j].line
		}
		return violations[i].file < violations[j].file
	})

	return violations, nil
}

func findFileSizeViolations(root string, diffChunks []diffChunk) ([]fileSizeViolation, error) {
	violationsByFile := make(map[string]fileSizeViolation)
	for _, chunk := range diffChunks {
		for _, file := range collectChangedGoFiles(chunk.diff) {
			if !shouldCheckGoFileSize(file.file) {
				continue
			}

			source, err := chunk.readFile(root, file.file)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("read %s for size check: %w", file.file, err)
			}

			lineCount := countLines(source)
			if lineCount <= maxGoFileLines {
				continue
			}
			previousLineCount, err := countPreviousLines(root, file, chunk.previousFile)
			if err != nil {
				return nil, err
			}
			if previousLineCount >= lineCount {
				continue
			}
			violation := fileSizeViolation{
				file:     file.file,
				lines:    lineCount,
				maxLines: maxGoFileLines,
			}
			if previous, ok := violationsByFile[file.file]; !ok || lineCount > previous.lines {
				violationsByFile[file.file] = violation
			}
		}
	}

	files := make([]string, 0, len(violationsByFile))
	for file := range violationsByFile {
		files = append(files, file)
	}
	sort.Strings(files)

	violations := make([]fileSizeViolation, 0, len(files))
	for _, file := range files {
		violations = append(violations, violationsByFile[file])
	}
	return violations, nil
}

func countPreviousLines(root string, file changedGoFile, readFile fileReader) (int, error) {
	if readFile == nil {
		return 0, nil
	}

	previousFile := file.previousFile
	if previousFile == "" {
		previousFile = file.file
	}
	source, err := readFile(root, previousFile)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read %s for previous size check: %w", previousFile, err)
	}
	return countLines(source), nil
}

func shouldCheckGoFileSize(file string) bool {
	return strings.HasPrefix(file, localServicePath+"/") &&
		strings.HasSuffix(file, ".go") &&
		!strings.HasSuffix(file, "_test.go")
}

func countLines(source []byte) int {
	if len(source) == 0 {
		return 0
	}

	lines := bytes.Count(source, []byte{'\n'})
	if source[len(source)-1] != '\n' {
		lines++
	}
	return lines
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

func parsePreviousFile(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return ""
	}
	path := strings.TrimPrefix(fields[2], "a/")
	if !strings.HasSuffix(path, ".go") {
		return ""
	}
	if !strings.HasPrefix(path, localServicePath+"/") && path != localServicePath {
		return ""
	}
	return path
}

func scanCommentLines(root, relativePath string, readFile fileReader) (map[int][]string, error) {
	source, err := readFile(root, relativePath)
	if err != nil {
		return nil, err
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

func workingTreeFileReader(root, relativePath string) ([]byte, error) {
	source, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		return nil, fmt.Errorf("read %s from working tree for comment scan: %w", relativePath, err)
	}
	return source, nil
}

func gitRevisionFileReader(prefix, label string) fileReader {
	return func(root, relativePath string) ([]byte, error) {
		output, err := commandOutput(root, "git", "show", prefix+relativePath)
		if err != nil {
			if isMissingGitRevisionPath(output) {
				return nil, os.ErrNotExist
			}
			return nil, fmt.Errorf("read %s from %s for comment scan: %w\n%s", relativePath, label, err, strings.TrimSpace(output))
		}
		return []byte(output), nil
	}
}

func isMissingGitRevisionPath(output string) bool {
	return strings.Contains(output, "does not exist in") ||
		strings.Contains(output, "exists on disk, but not in")
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
