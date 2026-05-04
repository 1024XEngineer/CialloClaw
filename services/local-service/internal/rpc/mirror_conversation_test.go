package rpc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestDispatchReturnsMirrorConversationList(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "mirror-history.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	seed := []storage.MirrorConversationRecord{
		{
			RecordID:        "mirror_002",
			TraceID:         "trace_002",
			CreatedAt:       "2026-04-08T10:02:00Z",
			UpdatedAt:       "2026-04-08T10:02:05Z",
			Source:          "dashboard",
			Trigger:         "voice_commit",
			InputMode:       "voice",
			TaskID:          "task_002",
			UserText:        "second",
			AgentText:       "second response",
			AgentBubbleType: "result",
			Status:          "responded",
		},
		{
			RecordID:  "mirror_001",
			TraceID:   "trace_001",
			CreatedAt: "2026-04-08T10:01:00Z",
			UpdatedAt: "2026-04-08T10:01:05Z",
			Source:    "floating_ball",
			Trigger:   "hover_text_input",
			InputMode: "text",
			TaskID:    "task_001",
			UserText:  "first",
			Status:    "submitted",
		},
	}
	for _, record := range seed {
		if err := storageService.MirrorConversationStore().SaveMirrorConversation(context.Background(), record); err != nil {
			t.Fatalf("SaveMirrorConversation returned error: %v", err)
		}
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-mirror-conversation-list"`),
		Method:  "agent.mirror.conversation.list",
		Params: mustMarshal(t, map[string]any{
			"limit":  20,
			"offset": 0,
			"source": "dashboard",
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one filtered mirror conversation item, got %d", len(items))
	}
	if items[0]["record_id"] != "mirror_002" || items[0]["task_id"] != "task_002" {
		t.Fatalf("expected mirror_002 payload, got %+v", items[0])
	}
}
