package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestServiceSubmitInputPersistsMirrorConversationResponse(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Mirror reply body.")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	result, err := service.SubmitInput(map[string]any{
		"request_meta": map[string]any{"trace_id": "trace_mirror_success_001"},
		"session_id":   "session_mirror_success_001",
		"source":       "dashboard",
		"trigger":      "voice_commit",
		"input": map[string]any{
			"type":       "text",
			"text":       "Please summarize this report.",
			"input_mode": "voice",
		},
	})
	if err != nil {
		t.Fatalf("SubmitInput returned error: %v", err)
	}

	history, err := service.MirrorConversationList(map[string]any{"limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("MirrorConversationList returned error: %v", err)
	}
	items := history["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one mirror conversation item, got %+v", items)
	}
	item := items[0]
	if item["trace_id"] != "trace_mirror_success_001" || item["source"] != "dashboard" || item["trigger"] != "voice_commit" {
		t.Fatalf("expected stored request metadata, got %+v", item)
	}
	if item["input_mode"] != "voice" || item["status"] != mirrorConversationStatusResponded {
		t.Fatalf("expected responded voice record, got %+v", item)
	}
	if item["task_id"] != result["task"].(map[string]any)["task_id"] {
		t.Fatalf("expected stored task linkage, got %+v", item)
	}
	if item["agent_text"] == nil || item["agent_bubble_type"] == nil {
		t.Fatalf("expected stored bubble payload, got %+v", item)
	}
}

func TestServiceSubmitInputPersistsMirrorConversationFailure(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Mirror helper failure.")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	snapshot := service.context.Capture(map[string]any{
		"request_meta": map[string]any{"trace_id": "trace_mirror_failed_001"},
		"source":       "floating_ball",
		"trigger":      "hover_text_input",
		"input": map[string]any{
			"type":       "text",
			"text":       "Check why this failed.",
			"input_mode": "text",
		},
	})
	record := service.beginMirrorConversationRecord(map[string]any{
		"request_meta": map[string]any{"trace_id": "trace_mirror_failed_001"},
		"source":       "floating_ball",
		"trigger":      "hover_text_input",
		"input": map[string]any{
			"type":       "text",
			"text":       "Check why this failed.",
			"input_mode": "text",
		},
	}, snapshot)
	service.finishMirrorConversationRecord(record, map[string]any{
		"task": map[string]any{
			"task_id": "task_failed_001",
			"status":  "failed",
		},
		"bubble_message": map[string]any{
			"type":       "status",
			"text":       "执行失败：model unavailable",
			"created_at": "2026-04-18T10:03:00Z",
		},
	}, nil)

	history, listErr := service.MirrorConversationList(map[string]any{"limit": 20, "offset": 0})
	if listErr != nil {
		t.Fatalf("MirrorConversationList returned error: %v", listErr)
	}
	items := history["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one mirror conversation item, got %+v", items)
	}
	item := items[0]
	if item["status"] != mirrorConversationStatusFailed {
		t.Fatalf("expected failed mirror record, got %+v", item)
	}
	if item["error_message"] != "执行失败：model unavailable" {
		t.Fatalf("expected stored error message, got %+v", item)
	}
	if item["task_id"] != "task_failed_001" {
		t.Fatalf("expected failed record to keep task linkage, got %+v", item)
	}
}

func TestServiceSubmitInputSynthesizesMirrorConversationTraceIDWhenMissing(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Mirror reply without trace id.")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	_, err := service.SubmitInput(map[string]any{
		"session_id": "session_mirror_generated_trace_001",
		"source":     "dashboard",
		"trigger":    "voice_commit",
		"input": map[string]any{
			"type":       "text",
			"text":       "Please summarize this report.",
			"input_mode": "voice",
		},
	})
	if err != nil {
		t.Fatalf("SubmitInput returned error: %v", err)
	}

	history, err := service.MirrorConversationList(map[string]any{"limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("MirrorConversationList returned error: %v", err)
	}
	items := history["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one mirror conversation item, got %+v", items)
	}
	traceID, ok := items[0]["trace_id"].(string)
	if !ok || strings.TrimSpace(traceID) == "" {
		t.Fatalf("expected synthesized non-empty trace id, got %+v", items[0])
	}
	if !strings.HasPrefix(traceID, "trace_mirror_") {
		t.Fatalf("expected synthesized mirror trace prefix, got %+v", items[0])
	}
}

func TestServiceSubmitInputKeepsDistinctMirrorHistoryRowsWhenTraceIDRepeats(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Mirror reply with repeated trace id.")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	requests := []map[string]any{
		{
			"request_meta": map[string]any{"trace_id": "trace_mirror_reused_001"},
			"session_id":   "session_mirror_reused_001",
			"source":       "dashboard",
			"trigger":      "voice_commit",
			"input": map[string]any{
				"type":       "text",
				"text":       "first mirror request",
				"input_mode": "voice",
			},
		},
		{
			"request_meta": map[string]any{"trace_id": "trace_mirror_reused_001"},
			"session_id":   "session_mirror_reused_002",
			"source":       "dashboard",
			"trigger":      "voice_commit",
			"input": map[string]any{
				"type":       "text",
				"text":       "second mirror request",
				"input_mode": "voice",
			},
		},
	}

	for _, params := range requests {
		if _, err := service.SubmitInput(params); err != nil {
			t.Fatalf("SubmitInput returned error: %v", err)
		}
	}

	history, err := service.MirrorConversationList(map[string]any{"limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("MirrorConversationList returned error: %v", err)
	}
	items := history["items"].([]map[string]any)
	if len(items) != 2 {
		t.Fatalf("expected two persisted mirror rows for reused trace id, got %+v", items)
	}
	if items[0]["record_id"] == items[1]["record_id"] {
		t.Fatalf("expected reused trace id to keep distinct record ids, got %+v", items)
	}
	texts := map[string]bool{}
	for _, item := range items {
		if item["trace_id"] != "trace_mirror_reused_001" {
			t.Fatalf("expected reused trace id to round-trip, got %+v", item)
		}
		text, _ := item["user_text"].(string)
		texts[text] = true
	}
	if !texts["first mirror request"] || !texts["second mirror request"] {
		t.Fatalf("expected both mirror requests to remain in history, got %+v", items)
	}
}

func TestServiceMirrorConversationListSupportsFilters(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "Mirror list filters.")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	seed := []storage.MirrorConversationRecord{
		{
			RecordID:  "mirror_filter_002",
			TraceID:   "trace_filter_002",
			CreatedAt: "2026-04-18T10:02:00Z",
			UpdatedAt: "2026-04-18T10:02:05Z",
			Source:    "dashboard",
			Trigger:   "voice_commit",
			InputMode: "voice",
			TaskID:    "task_filter_002",
			UserText:  "second",
			Status:    mirrorConversationStatusFailed,
		},
		{
			RecordID:        "mirror_filter_001",
			TraceID:         "trace_filter_001",
			CreatedAt:       "2026-04-18T10:01:00Z",
			UpdatedAt:       "2026-04-18T10:01:05Z",
			Source:          "dashboard",
			Trigger:         "voice_commit",
			InputMode:       "voice",
			TaskID:          "task_filter_001",
			UserText:        "first",
			AgentText:       "done",
			AgentBubbleType: "result",
			Status:          mirrorConversationStatusResponded,
		},
	}
	for _, record := range seed {
		if err := service.storage.MirrorConversationStore().SaveMirrorConversation(context.Background(), record); err != nil {
			t.Fatalf("SaveMirrorConversation returned error: %v", err)
		}
	}

	history, err := service.MirrorConversationList(map[string]any{
		"task_id": "task_filter_001",
		"source":  "dashboard",
		"status":  mirrorConversationStatusResponded,
		"limit":   20,
		"offset":  0,
	})
	if err != nil {
		t.Fatalf("MirrorConversationList returned error: %v", err)
	}
	items := history["items"].([]map[string]any)
	if len(items) != 1 || items[0]["record_id"] != "mirror_filter_001" {
		t.Fatalf("expected filtered mirror history to keep mirror_filter_001, got %+v", items)
	}
}
