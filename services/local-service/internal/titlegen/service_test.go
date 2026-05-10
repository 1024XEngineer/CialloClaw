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
	return model.GenerateTextResponse{OutputText: s.output}, nil
}

func TestGenerateTaskSubjectUsesModelSummary(t *testing.T) {
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布复盘风险跟进"}`,
	}))

	title := service.GenerateTaskSubject(context.Background(), taskcontext.TaskContextSnapshot{
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

	title := service.GenerateTaskSubject(context.Background(), taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "请帮我整理这次发布复盘，重点补齐风险项和后续跟进安排",
	}, "agent_loop", "这次发布复盘")

	if title != "这次发布复盘" {
		t.Fatalf("expected fallback title, got %q", title)
	}
}

func TestCompactTaskFallbackTrimsConversationLeadIn(t *testing.T) {
	title := CompactTaskFallback("请帮我整理这次发布复盘\n重点补齐风险项和后续跟进安排")

	if title != "整理这次发布复盘 重点补齐风险项和后续跟进..." {
		t.Fatalf("expected local fallback compaction to drop the request wrapper, got %q", title)
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

	title := service.GenerateNoteTitle(context.Background(), map[string]any{
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

	first := service.GenerateNoteTitle(context.Background(), map[string]any{
		"title":     "Weekly retro",
		"note_text": "Weekly retro\n补齐风险项和责任人",
	}, "Weekly retro")
	second := service.GenerateNoteTitle(context.Background(), map[string]any{
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

	_ = service.GenerateTaskSubject(context.Background(), taskcontext.TaskContextSnapshot{
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

	_ = service.GenerateTaskSubject(context.Background(), taskcontext.TaskContextSnapshot{
		InputType:     "text_selection",
		SelectionText: longText,
		VisibleText:   longText,
	}, "agent_loop", "发布说明")

	if strings.Contains(prompt, "TAIL") {
		t.Fatalf("expected prompt budgeting to truncate oversized task context, got %q", prompt)
	}
}

func TestGenerateNoteTitlePromptBudgetsLargeNoteFields(t *testing.T) {
	var prompt string
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘阻塞项"}`,
		last:   &prompt,
	}))
	longNote := strings.Repeat("B", notePromptPrimaryLimit+40) + "TAIL"

	_ = service.GenerateNoteTitle(context.Background(), map[string]any{
		"title":     "Weekly retro",
		"note_text": longNote,
	}, "Weekly retro")

	if strings.Contains(prompt, "TAIL") {
		t.Fatalf("expected prompt budgeting to truncate oversized note context, got %q", prompt)
	}
}
