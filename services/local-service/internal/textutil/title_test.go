package textutil

import "testing"

func TestCompactSubjectRemovesRequestWrappers(t *testing.T) {
	t.Parallel()

	got := CompactSubject([]string{"帮我整理今天的会议纪要"}, "当前内容", 24)
	if got != "今天的会议纪要" {
		t.Fatalf("expected wrapped request text to compact into a clean subject, got %q", got)
	}
}

func TestCompactSubjectCombinesFollowUpDetail(t *testing.T) {
	t.Parallel()

	got := CompactSubject([]string{
		"请帮我整理这次发布复盘",
		"重点补齐风险项和后续跟进安排",
	}, "当前内容", 24)
	if got != "这次发布复盘 · 重点补齐风险项和后续跟进..." {
		t.Fatalf("expected follow-up detail to influence compacted subject, got %q", got)
	}
}

func TestCompactLabelPrefersFullNoteContext(t *testing.T) {
	t.Parallel()

	got := CompactLabel([]string{
		"把这周会议里的共识、待确认事项和风险点整理成一页结构化纪要，方便后续同步给项目组。",
		"会议纪要",
	}, "会议纪要", 24)
	if got != "这周会议里的共识 · 待确认事项和风险点整..." {
		t.Fatalf("expected note label to come from full note context, got %q", got)
	}
}
