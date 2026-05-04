package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMirrorConversationStoresSaveListAndFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		newStore func(t *testing.T) MirrorConversationStore
	}{
		{
			name: "in-memory",
			newStore: func(t *testing.T) MirrorConversationStore {
				t.Helper()
				return newInMemoryMirrorConversationStore()
			},
		},
		{
			name: "sqlite",
			newStore: func(t *testing.T) MirrorConversationStore {
				t.Helper()
				store, err := NewSQLiteMirrorConversationStore(filepath.Join(t.TempDir(), "mirror-conversations.db"))
				if err != nil {
					t.Fatalf("NewSQLiteMirrorConversationStore returned error: %v", err)
				}
				t.Cleanup(func() { _ = store.Close() })
				return store
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := test.newStore(t)
			ctx := context.Background()
			seed := []MirrorConversationRecord{
				{
					RecordID:        "mirror_001",
					TraceID:         "trace_001",
					CreatedAt:       "2026-04-18T10:00:00Z",
					UpdatedAt:       "2026-04-18T10:00:05Z",
					Source:          "dashboard",
					Trigger:         "voice_commit",
					InputMode:       "voice",
					SessionID:       "session_001",
					TaskID:          "task_001",
					UserText:        "summarize the error",
					AgentText:       "I summarized the error.",
					AgentBubbleType: "result",
					Status:          "responded",
				},
				{
					RecordID:     "mirror_002",
					TraceID:      "trace_002",
					CreatedAt:    "2026-04-18T10:00:10Z",
					UpdatedAt:    "2026-04-18T10:00:12Z",
					Source:       "floating_ball",
					Trigger:      "hover_text_input",
					InputMode:    "text",
					UserText:     "check this note",
					Status:       "failed",
					ErrorMessage: "submit input failed",
				},
			}
			for _, record := range seed {
				if err := store.SaveMirrorConversation(ctx, record); err != nil {
					t.Fatalf("SaveMirrorConversation returned error: %v", err)
				}
			}

			items, total, err := store.ListMirrorConversations(ctx, "", "", "", 20, 0)
			if err != nil {
				t.Fatalf("ListMirrorConversations returned error: %v", err)
			}
			if total != 2 || len(items) != 2 {
				t.Fatalf("expected two mirror conversations, got total=%d items=%+v", total, items)
			}
			if items[0].RecordID != "mirror_002" || items[1].RecordID != "mirror_001" {
				t.Fatalf("expected newest-first ordering, got %+v", items)
			}

			filtered, total, err := store.ListMirrorConversations(ctx, "task_001", "dashboard", "responded", 20, 0)
			if err != nil {
				t.Fatalf("ListMirrorConversations filtered returned error: %v", err)
			}
			if total != 1 || len(filtered) != 1 || filtered[0].RecordID != "mirror_001" {
				t.Fatalf("expected task/source/status filter to keep mirror_001, got total=%d items=%+v", total, filtered)
			}

			paged, total, err := store.ListMirrorConversations(ctx, "", "", "", 1, 1)
			if err != nil {
				t.Fatalf("ListMirrorConversations paged returned error: %v", err)
			}
			if total != 2 || len(paged) != 1 || paged[0].RecordID != "mirror_001" {
				t.Fatalf("expected pagination to return older record, got total=%d items=%+v", total, paged)
			}

			updated := seed[0]
			updated.UpdatedAt = "2026-04-18T10:00:20Z"
			updated.AgentText = "Updated summary"
			if err := store.SaveMirrorConversation(ctx, updated); err != nil {
				t.Fatalf("SaveMirrorConversation update returned error: %v", err)
			}

			items, total, err = store.ListMirrorConversations(ctx, "", "", "", 20, 0)
			if err != nil {
				t.Fatalf("ListMirrorConversations after update returned error: %v", err)
			}
			if total != 2 || items[0].RecordID != "mirror_001" || items[0].AgentText != "Updated summary" {
				t.Fatalf("expected updated record to move to the top, got total=%d items=%+v", total, items)
			}
		})
	}
}

func TestMirrorConversationStoreNormalizesMissingTimestamps(t *testing.T) {
	store := newInMemoryMirrorConversationStore()
	if err := store.SaveMirrorConversation(context.Background(), MirrorConversationRecord{
		RecordID:  "mirror_missing_time",
		TraceID:   "trace_missing_time",
		Source:    "dashboard",
		Trigger:   "voice_commit",
		InputMode: "voice",
		UserText:  "hello",
		Status:    "submitted",
	}); err != nil {
		t.Fatalf("SaveMirrorConversation returned error: %v", err)
	}
	items, total, err := store.ListMirrorConversations(context.Background(), "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListMirrorConversations returned error: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one stored record, got total=%d items=%+v", total, items)
	}
	if items[0].CreatedAt == "" || items[0].UpdatedAt == "" {
		t.Fatalf("expected timestamps to be normalized, got %+v", items[0])
	}
}
