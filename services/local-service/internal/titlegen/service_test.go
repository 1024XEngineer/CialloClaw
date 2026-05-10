package titlegen

import (
	"context"
	"errors"
	"testing"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

type stubModelClient struct {
	output string
	err    error
}

func (s stubModelClient) GenerateText(_ context.Context, _ model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	if s.err != nil {
		return model.GenerateTextResponse{}, s.err
	}
	return model.GenerateTextResponse{OutputText: s.output}, nil
}

func TestGenerateTaskSubjectUsesModelSummary(t *testing.T) {
	service := NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"发布复盘风险跟进"}`,
	}))

	title := service.GenerateTaskSubject(context.Background(), contextsvc.TaskContextSnapshot{
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

	title := service.GenerateTaskSubject(context.Background(), contextsvc.TaskContextSnapshot{
		InputType: "text",
		Text:      "请帮我整理这次发布复盘，重点补齐风险项和后续跟进安排",
	}, "agent_loop", "这次发布复盘")

	if title != "这次发布复盘" {
		t.Fatalf("expected fallback title, got %q", title)
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
