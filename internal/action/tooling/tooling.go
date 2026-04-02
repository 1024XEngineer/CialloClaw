package tooling

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

type RiskLevel string

const (
	RiskNone     RiskLevel = "none"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Metadata struct {
	Name             string
	Description      string
	Version          string
	Risk             RiskLevel
	Idempotent       bool
	RequiresApproval bool
	Timeout          time.Duration
	Tags             []string
}

type ToolCall struct {
	SessionID string
	TaskID    *string
	StepID    *string
	Args      map[string]any
}

type ToolResult struct {
	Success bool
	Data    map[string]any
	Error   *string
}

type Tool interface {
	Metadata() Metadata
	Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}

type TextGenerator interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	Configured() bool
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Metadata().Name
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}
	r.tools[name] = tool
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) List() []Metadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Metadata
	for _, tool := range r.tools {
		out = append(out, tool.Metadata())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

type ContentTool struct {
	client *http.Client
	llm    TextGenerator
}

func NewContentTool(llm TextGenerator) *ContentTool {
	return &ContentTool{
		client: &http.Client{Timeout: 8 * time.Second},
		llm:    llm,
	}
}

func (t *ContentTool) Metadata() Metadata {
	return Metadata{
		Name:             "content",
		Description:      "内容理解工具，支持总结、翻译、解释、下一步建议和视频链接预总结。",
		Version:          "1.1.0",
		Risk:             RiskLow,
		Idempotent:       true,
		RequiresApproval: false,
		Timeout:          45 * time.Second,
		Tags:             []string{"内容", "文本", "视频", "大模型"},
	}
}

func (t *ContentTool) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	action := stringArg(call.Args, "action")
	switch action {
	case "summary", "translate", "explain", "next_steps", "video_summary":
		return t.handleContentAction(ctx, call, action)
	default:
		text := stringArg(call.Args, "text")
		if text == "" {
			text = "当前没有可处理的内容，请先输入文本或复制内容。"
		}
		return t.handleContentAction(ctx, ToolCall{
			SessionID: call.SessionID,
			TaskID:    call.TaskID,
			StepID:    call.StepID,
			Args: map[string]any{
				"action": "explain",
				"text":   text,
			},
		}, "explain")
	}
}

func (t *ContentTool) handleContentAction(ctx context.Context, call ToolCall, action string) (ToolResult, error) {
	if t.llm != nil && t.llm.Configured() {
		content, title, err := t.generateByLLM(ctx, call.Args, action)
		if err == nil {
			return successResult(map[string]any{
				"action":  action,
				"title":   title,
				"content": content,
				"mode":    "llm",
			}), nil
		}
		fallback, fallbackTitle := t.generateFallback(ctx, call.Args, action)
		return successResult(map[string]any{
			"action":  action,
			"title":   fallbackTitle,
			"content": "大模型调用失败，已自动切换到本地降级结果。\n\n原因：" + err.Error() + "\n\n" + fallback,
			"mode":    "fallback_after_error",
		}), nil
	}

	content, title := t.generateFallback(ctx, call.Args, action)
	return successResult(map[string]any{
		"action":  action,
		"title":   title,
		"content": "当前未配置大模型，以下是本地降级结果。请在“模型设置”里填写 Base URL、API Key、Model。\n\n" + content,
		"mode":    "local_fallback",
	}), nil
}

func (t *ContentTool) generateByLLM(ctx context.Context, args map[string]any, action string) (string, string, error) {
	systemPrompt, userPrompt, title := buildPrompt(action, args)
	content, err := t.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(content), title, nil
}

func (t *ContentTool) generateFallback(ctx context.Context, args map[string]any, action string) (string, string) {
	switch action {
	case "summary":
		return summarizeText(stringArg(args, "text")), "内容总结"
	case "translate":
		target := stringArg(args, "target_language")
		if target == "" {
			target = autoTargetLanguage(stringArg(args, "text"))
		}
		text, _ := translateText(stringArg(args, "text"), target)
		return text, "翻译结果"
	case "explain":
		return explainText(stringArg(args, "text")), "内容解释"
	case "next_steps":
		return nextSteps(stringArg(args, "text")), "下一步建议"
	case "video_summary":
		url := stringArg(args, "url")
		if url == "" {
			return "没有检测到视频链接。", "视频总结"
		}
		content, title := t.videoSummary(ctx, url)
		return content, title
	default:
		return explainText(stringArg(args, "text")), "助手回复"
	}
}

func buildPrompt(action string, args map[string]any) (string, string, string) {
	baseSystem := "你是 CialloClaw 的中文桌面 Agent。除翻译任务外，一律使用简体中文回答。输出直接面向用户，不要解释你的身份，不要输出多余前言。"
	text := strings.TrimSpace(stringArg(args, "text"))
	url := strings.TrimSpace(stringArg(args, "url"))
	target := strings.TrimSpace(stringArg(args, "target_language"))

	switch action {
	case "summary":
		return baseSystem, "请总结下面的内容，输出格式：1. 摘要一句话；2. 3-5 条关键点。\n\n内容：\n" + text, "内容总结"
	case "translate":
		if target == "" {
			target = autoTargetLanguage(text)
		}
		language := "中文"
		if target == "en" {
			language = "英文"
		}
		return "你是高质量翻译助手。只输出译文，不要解释。", "请把下面内容翻译成" + language + "：\n\n" + text, "翻译结果"
	case "explain":
		return baseSystem, "请用通俗但准确的方式解释下面内容，输出格式：1. 这是什么；2. 为什么重要；3. 如果用户要继续处理，建议看什么。\n\n内容：\n" + text, "内容解释"
	case "next_steps":
		return baseSystem, "请基于下面内容给出 3-5 条可执行的下一步建议，要求动作明确、短句、按优先级排序。\n\n内容：\n" + text, "下一步建议"
	case "video_summary":
		return baseSystem, "请基于下面的视频链接元数据输出一个“预总结”。必须明确说明这是基于链接和标题的结构化预判，不是完整转写总结。输出格式：1. 视频可能主题；2. 建议关注的关键点；3. 适合继续补充的信息。\n\n链接：\n" + url, "视频总结"
	default:
		return baseSystem, "请解释下面内容：\n\n" + text, "助手回复"
	}
}

func (t *ContentTool) videoSummary(ctx context.Context, url string) (string, string) {
	title := titleFromURL(url)
	if fetched, err := t.fetchTitle(ctx, url); err == nil && fetched != "" {
		title = fetched
	}
	host := hostLabel(url)
	sections := []string{
		fmt.Sprintf("视频预总结：检测到来自 %s 的视频链接，标题可能是《%s》。当前结果基于链接元数据生成，不是完整转写总结。", host, title),
		"建议关注：\n- 先看视频前 20% 获取背景\n- 重点观察中段的核心观点或演示\n- 如果需要精确总结，下一步应补充字幕或转写",
		"可继续补充的信息：\n1. 视频字幕\n2. 视频时长\n3. 你最关心的问题",
		fmt.Sprintf("来源链接：%s", url),
	}
	return strings.Join(sections, "\n\n"), "视频总结"
}

func (t *ContentTool) fetchTitle(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "CialloClaw/1.0")
	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	if n == 0 {
		return "", nil
	}
	matches := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(string(buf[:n]))
	if len(matches) < 2 {
		return "", nil
	}
	return strings.TrimSpace(htmlCompact(matches[1])), nil
}

type TodoScanTool struct{}

func NewTodoScanTool() *TodoScanTool {
	return &TodoScanTool{}
}

func (t *TodoScanTool) Metadata() Metadata {
	return Metadata{
		Name:             "todo_scan",
		Description:      "扫描 Markdown 待办文件，只识别 - [ ] 和 - [x]。",
		Version:          "1.0.0",
		Risk:             RiskNone,
		Idempotent:       true,
		RequiresApproval: false,
		Timeout:          8 * time.Second,
		Tags:             []string{"待办", "Markdown"},
	}
}

func (t *TodoScanTool) Execute(_ context.Context, call ToolCall) (ToolResult, error) {
	rawRoots, ok := call.Args["roots"].([]string)
	if !ok {
		if generic, exists := call.Args["roots"].([]any); exists {
			rawRoots = make([]string, 0, len(generic))
			for _, item := range generic {
				if text, ok := item.(string); ok {
					rawRoots = append(rawRoots, text)
				}
			}
		}
	}

	pending, completed, err := scanTodoRoots(rawRoots)
	if err != nil {
		return failureResult(err.Error()), err
	}
	return successResult(map[string]any{
		"pending":         pending,
		"completed":       completed,
		"pending_count":   len(pending),
		"completed_count": len(completed),
		"content":         renderTodoSummary(pending, completed),
	}), nil
}

func scanTodoRoots(roots []string) ([]map[string]any, []map[string]any, error) {
	var pending []map[string]any
	var completed []map[string]any
	for _, root := range roots {
		if root == "" {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			items, err := parseTodoFile(path)
			if err != nil {
				return nil
			}
			for _, item := range items {
				if item["done"] == true {
					completed = append(completed, item)
				} else {
					pending = append(pending, item)
				}
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		return strings.Compare(fmt.Sprint(pending[i]["text"]), fmt.Sprint(pending[j]["text"])) < 0
	})
	sort.Slice(completed, func(i, j int) bool {
		return strings.Compare(fmt.Sprint(completed[i]["text"]), fmt.Sprint(completed[j]["text"])) < 0
	})
	return pending, completed, nil
}

func parseTodoFile(path string) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var items []map[string]any
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- [ ] ") {
			items = append(items, map[string]any{"file": path, "text": strings.TrimPrefix(line, "- [ ] "), "done": false})
		}
		if strings.HasPrefix(line, "- [x] ") {
			items = append(items, map[string]any{"file": path, "text": strings.TrimPrefix(line, "- [x] "), "done": true})
		}
	}
	return items, scanner.Err()
}

func renderTodoSummary(pending []map[string]any, completed []map[string]any) string {
	lines := []string{
		fmt.Sprintf("待办总数：%d 未完成，%d 已完成。", len(pending), len(completed)),
	}
	if len(pending) > 0 {
		lines = append(lines, "建议优先处理：")
		for i, item := range pending {
			if i == 3 {
				break
			}
			lines = append(lines, "- "+fmt.Sprint(item["text"]))
		}
	}
	return strings.Join(lines, "\n")
}

func successResult(data map[string]any) ToolResult {
	return ToolResult{Success: true, Data: data}
}

func failureResult(message string) ToolResult {
	return ToolResult{Success: false, Error: &message}
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if text, ok := args[key].(string); ok {
		return text
	}
	return ""
}

func summarizeText(text string) string {
	text = normalizeText(text)
	if text == "" {
		return "当前没有可总结的内容。"
	}
	parts := splitSentences(text)
	if len(parts) == 0 {
		return text
	}
	lines := []string{"摘要："}
	for i, part := range parts {
		if i == 3 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s", part))
	}
	return strings.Join(lines, "\n")
}

func explainText(text string) string {
	text = normalizeText(text)
	if text == "" {
		return "当前没有可解释的内容。"
	}
	keywords := topKeywords(text, 4)
	lines := []string{
		"解释：这段内容主要围绕以下信息展开。",
	}
	if len(keywords) > 0 {
		lines = append(lines, "关键词："+strings.Join(keywords, "、"))
	}
	lines = append(lines, "简化说明："+firstClause(text))
	lines = append(lines, "如果要继续处理，建议再点“总结”或“下一步建议”。")
	return strings.Join(lines, "\n")
}

func nextSteps(text string) string {
	text = normalizeText(text)
	if text == "" {
		return "当前没有上下文，建议先复制内容或直接输入任务。"
	}
	lines := []string{
		"建议下一步：",
		"1. 先明确这是理解型任务、翻译型任务还是执行型任务。",
		"2. 如果涉及任务，请把动作拆成可执行待办。",
		"3. 如果涉及决策，请补齐背景、约束和期望输出。",
	}
	if looksLikeCode(text) {
		lines = append(lines, "4. 这是代码或报错片段，建议补充运行环境和报错上下文。")
	}
	return strings.Join(lines, "\n")
}

func translateText(text, target string) (string, string) {
	text = normalizeText(text)
	if text == "" {
		return "当前没有可翻译的内容。", "fallback"
	}

	dictionary := map[string]map[string]string{
		"zh": {
			"error":     "错误",
			"warning":   "警告",
			"success":   "成功",
			"failed":    "失败",
			"task":      "任务",
			"session":   "会话",
			"clipboard": "剪贴板",
			"file":      "文件",
			"folder":    "文件夹",
			"summary":   "总结",
			"translate": "翻译",
			"next":      "下一步",
			"memory":    "记忆",
			"dashboard": "控制面板",
		},
		"en": {
			"错误":   "error",
			"警告":   "warning",
			"成功":   "success",
			"失败":   "failed",
			"任务":   "task",
			"会话":   "session",
			"剪贴板":  "clipboard",
			"文件":   "file",
			"文件夹":  "folder",
			"总结":   "summary",
			"翻译":   "translate",
			"下一步":  "next step",
			"记忆":   "memory",
			"控制面板": "dashboard",
		},
	}

	translated := text
	for source, dest := range dictionary[target] {
		translated = replaceWord(translated, source, dest)
	}
	if translated == text {
		if target == "zh" {
			return "离线翻译模式：当前词典未命中足够内容，建议配置大模型获取更完整的翻译。\n\n原文：\n" + text, "fallback"
		}
		return "Offline translation fallback: the local dictionary did not match enough terms.\n\nOriginal:\n" + text, "fallback"
	}
	return translated, "dictionary"
}

func autoTargetLanguage(text string) string {
	if containsCJK(text) {
		return "en"
	}
	return "zh"
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	return regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
}

func splitSentences(text string) []string {
	text = strings.ReplaceAll(text, "。", ".")
	text = strings.ReplaceAll(text, "！", ".")
	text = strings.ReplaceAll(text, "？", ".")
	chunks := strings.Split(text, ".")
	var out []string
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if len(chunk) >= 8 {
			out = append(out, chunk)
		}
	}
	if len(out) == 0 && text != "" {
		out = append(out, text)
	}
	return out
}

func topKeywords(text string, limit int) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_'
	})
	stopWords := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "that": {}, "this": {},
		"is": {}, "are": {}, "was": {}, "were": {}, "have": {}, "from": {},
	}
	counts := map[string]int{}
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		if _, blocked := stopWords[word]; blocked {
			continue
		}
		counts[word]++
	}
	type pair struct {
		word  string
		count int
	}
	var pairs []pair
	for word, count := range counts {
		pairs = append(pairs, pair{word: word, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].word < pairs[j].word
		}
		return pairs[i].count > pairs[j].count
	})
	var out []string
	for i, item := range pairs {
		if i == limit {
			break
		}
		out = append(out, item.word)
	}
	return out
}

func firstClause(text string) string {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return text
	}
	return sentences[0]
}

func looksLikeCode(text string) bool {
	return strings.Contains(text, "{") || strings.Contains(text, "}") || strings.Contains(text, "func ") || strings.Contains(text, "panic")
}

func containsCJK(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func replaceWord(text, from, to string) string {
	if containsCJK(from) {
		return strings.ReplaceAll(text, from, to)
	}
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(from) + `\b`)
	return re.ReplaceAllString(text, to)
}

func titleFromURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, "/")
	return url
}

func hostLabel(url string) string {
	text := titleFromURL(url)
	parts := strings.Split(text, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return text
}

func htmlCompact(text string) string {
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(strings.ReplaceAll(text, "&amp;", "&"))
}
