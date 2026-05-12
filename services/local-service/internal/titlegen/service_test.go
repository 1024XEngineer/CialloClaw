package titlegen

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

type stubModelClient struct {
	output string
	err    error
	calls  *atomic.Int32
	last   *string
}

func (s stubModelClient) GenerateText(_ context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	if s.last != nil {
		*s.last = request.Input
	}
	if s.err != nil {
		return model.GenerateTextResponse{}, s.err
	}
	return model.GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  "req_titlegen_test",
		Provider:   "openai_responses",
		ModelID:    "gpt-5.4",
		OutputText: s.output,
	}, nil
}

func TestGenerateTaskSubjectUsesModelSummary(t *testing.T) {
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布复盘风险跟进"}`,
	}))

	title := service.GenerateTaskSubject(context.Background(), GenerationOwner{TaskID: "task_123", RunID: "run_123"}, taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "请帮我整理这次发布复盘，重点补齐风险项和后续跟进安排",
	}, "agent_loop", "这次发布复盘")

	if title != "发布复盘风险跟进" {
		t.Fatalf("expected model generated title, got %q", title)
	}
}

func TestGenerateTaskSubjectFallsBackWhenModelFails(t *testing.T) {
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		err: errors.New("boom"),
	}))

	title := service.GenerateTaskSubject(context.Background(), GenerationOwner{TaskID: "task_123", RunID: "run_123"}, taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "请帮我整理这次发布复盘，重点补齐风险项和后续跟进安排",
	}, "agent_loop", "这次发布复盘")

	if title != "这次发布复盘" {
		t.Fatalf("expected fallback title, got %q", title)
	}
}

func TestCompactTaskFallbackKeepsDeterministicMultiLineContext(t *testing.T) {
	title := CompactTaskFallback("请帮我整理这次发布复盘\n重点补齐风险项和后续跟进安排")

	if title != "请帮我整理这次发布复盘 重点补齐风险项和后..." {
		t.Fatalf("expected local fallback compaction to stay deterministic without semantic rewriting, got %q", title)
	}
}

func TestCompactTaskFallbackDoesNotRewriteLeadingNoun(t *testing.T) {
	title := CompactTaskFallback("请假流程说明\n补充审批范围")

	if title != "请假流程说明 补充审批范围" {
		t.Fatalf("expected local fallback compaction to preserve leading nouns, got %q", title)
	}
}

func TestCompactTaskFallbackSplitsLongSingleSentenceClauses(t *testing.T) {
	title := CompactTaskFallback("请详细介绍这次琪露诺是谁，出自哪部作品，出名的同人作有哪些")

	if title != "请详细介绍这次琪露诺是谁 出自哪部作品" {
		t.Fatalf("expected long single-sentence fallback title to compact by clauses, got %q", title)
	}
}

func TestCompactNoteFallbackKeepsWholeNoteContextBounded(t *testing.T) {
	title := CompactNoteFallback("- [ ] Weekly retro\nreview blockers and next steps")

	if title != "Weekly retro review b..." {
		t.Fatalf("expected note fallback compaction to keep multi-line context bounded, got %q", title)
	}
}

func TestGenerateNoteTitleParsesRawTextFallback(t *testing.T) {
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: "每周复盘待补充事项",
	}))

	title := service.GenerateNoteTitle(context.Background(), GenerationOwner{TaskID: "insp_123", RunID: "insp_123"}, map[string]any{
		"title":            "Weekly retro",
		"note_text":        "Weekly retro\n补齐风险项、责任人和发布时间",
		"agent_suggestion": "先整理未完成项",
	}, "Weekly retro")

	if title != "每周复盘待补充事项" {
		t.Fatalf("expected raw title output to be normalized, got %q", title)
	}
}

func TestGenerateNoteTitleDoesNotCacheFallbackAfterModelFailure(t *testing.T) {
	callCount := &atomic.Int32{}
	client := &stubModelClient{
		err:   errors.New("timeout"),
		calls: callCount,
	}
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, client))

	first := service.GenerateNoteTitle(context.Background(), GenerationOwner{TaskID: "insp_123", RunID: "insp_123"}, map[string]any{
		"title":     "Weekly retro",
		"note_text": "Weekly retro\n补齐风险项和责任人",
	}, "Weekly retro")
	second := service.GenerateNoteTitle(context.Background(), GenerationOwner{TaskID: "insp_123", RunID: "insp_123"}, map[string]any{
		"title":     "Weekly retro",
		"note_text": "Weekly retro\n补齐风险项和责任人",
	}, "Weekly retro")

	if first != "Weekly retro" || second != "Weekly retro" {
		t.Fatalf("expected fallback titles when model fails, got %q and %q", first, second)
	}
	if got := callCount.Load(); got != 2 {
		t.Fatalf("expected transient fallback not to be cached, got %d model calls", got)
	}
}

func TestGenerateTaskSubjectPromptOmitsClipboardText(t *testing.T) {
	var prompt string
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布说明校对"}`,
		last:   &prompt,
	}))

	_ = service.GenerateTaskSubject(context.Background(), GenerationOwner{TaskID: "task_123", RunID: "run_123"}, taskcontext.TaskContextSnapshot{
		InputType:     "text_selection",
		SelectionText: "请检查发布说明",
		ClipboardText: "secret copied token",
	}, "agent_loop", "发布说明")

	if strings.Contains(prompt, "clipboard_text") || strings.Contains(prompt, "secret copied token") {
		t.Fatalf("expected clipboard text to stay outside title generation prompt, got %q", prompt)
	}
}

func TestGenerateTaskSubjectPromptBudgetsLargeSnapshotFields(t *testing.T) {
	var prompt string
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布说明校对"}`,
		last:   &prompt,
	}))
	longText := strings.Repeat("A", taskPromptPrimaryLimit+40) + "TAIL"

	_ = service.GenerateTaskSubject(context.Background(), GenerationOwner{TaskID: "task_123", RunID: "run_123"}, taskcontext.TaskContextSnapshot{
		InputType:     "text_selection",
		SelectionText: longText,
		VisibleText:   longText,
	}, "agent_loop", "发布说明")

	if strings.Contains(prompt, "TAIL") {
		t.Fatalf("expected prompt budgeting to truncate oversized task context, got %q", prompt)
	}
}

func TestGenerateTaskSubjectPromptOmitsAmbientScreenContextForOrdinaryTasks(t *testing.T) {
	var prompt string
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布说明校对"}`,
		last:   &prompt,
	}))

	_ = service.GenerateTaskSubject(context.Background(), GenerationOwner{TaskID: "task_123", RunID: "run_123"}, taskcontext.TaskContextSnapshot{
		InputType:     "text_selection",
		SelectionText: "请检查发布说明",
		VisibleText:   "ambient page text",
		ScreenSummary: "ambient screen summary",
	}, "agent_loop", "发布说明")

	if strings.Contains(prompt, "visible_text") || strings.Contains(prompt, "ambient page text") {
		t.Fatalf("expected ordinary task title prompt to exclude visible_text, got %q", prompt)
	}
	if strings.Contains(prompt, "screen_summary") || strings.Contains(prompt, "ambient screen summary") {
		t.Fatalf("expected ordinary task title prompt to exclude screen_summary, got %q", prompt)
	}
}

func TestGenerateTaskSubjectPromptKeepsScreenContextForScreenAnalyze(t *testing.T) {
	var prompt string
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布说明校对"}`,
		last:   &prompt,
	}))

	_ = service.GenerateTaskSubject(context.Background(), GenerationOwner{TaskID: "task_123", RunID: "run_123"}, taskcontext.TaskContextSnapshot{
		InputType:     "text",
		Text:          "帮我看看这个报错",
		VisibleText:   "runtime stack trace",
		ScreenSummary: "browser page with error dialog",
	}, "screen_analyze", "报错分析")

	if !strings.Contains(prompt, "visible_text: runtime stack trace") {
		t.Fatalf("expected screen analysis prompt to keep visible_text, got %q", prompt)
	}
	if !strings.Contains(prompt, "screen_summary: browser page with error dialog") {
		t.Fatalf("expected screen analysis prompt to keep screen_summary, got %q", prompt)
	}
}

func TestGenerateNoteTitlePromptBudgetsLargeNoteFields(t *testing.T) {
	var prompt string
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘阻塞项"}`,
		last:   &prompt,
	}))
	longNote := strings.Repeat("B", notePromptPrimaryLimit+40) + "TAIL"

	_ = service.GenerateNoteTitle(context.Background(), GenerationOwner{TaskID: "insp_123", RunID: "insp_123"}, map[string]any{
		"title":     "Weekly retro",
		"note_text": longNote,
	}, "Weekly retro")

	if strings.Contains(prompt, "TAIL") {
		t.Fatalf("expected prompt budgeting to truncate oversized note context, got %q", prompt)
	}
}

func TestGenerateTaskSubjectResultPreservesOwnerAttribution(t *testing.T) {
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布复盘风险跟进"}`,
	}))

	result := service.GenerateTaskSubjectResult(context.Background(), GenerationOwner{
		TaskID: "task_title_owner",
		RunID:  "run_title_owner",
	}, taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "请帮我整理这次发布复盘，重点补齐风险项和后续跟进安排",
	}, "agent_loop", "这次发布复盘")

	if result.Invocation == nil {
		t.Fatal("expected task title generation to expose invocation metadata")
	}
	if result.Invocation.TaskID != "task_title_owner" || result.Invocation.RunID != "run_title_owner" {
		t.Fatalf("expected invocation owner attribution to be preserved, got %+v", result.Invocation)
	}
}

func TestGenerateNoteTitleResultPreservesOwnerAttribution(t *testing.T) {
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘待补充事项"}`,
	}))

	result := service.GenerateNoteTitleResult(context.Background(), GenerationOwner{
		TaskID: "insp_note_owner",
		RunID:  "insp_note_owner",
	}, map[string]any{
		"title":            "Weekly retro",
		"note_text":        "Weekly retro\n补齐风险项、责任人和发布时间",
		"agent_suggestion": "先整理未完成项",
	}, "Weekly retro")

	if result.Invocation == nil {
		t.Fatal("expected note title generation to expose invocation metadata")
	}
	if result.Invocation.TaskID != "insp_note_owner" || result.Invocation.RunID != "insp_note_owner" {
		t.Fatalf("expected note title invocation owner attribution to be preserved, got %+v", result.Invocation)
	}
}
