package rpc

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestTaskIDsFromResponseCollectsNestedIDs(t *testing.T) {
	response := newSuccessEnvelope(json.RawMessage(`"req-task-ids"`), map[string]any{
		"task": map[string]any{
			"task_id": "task_root",
		},
		"items": []any{
			map[string]any{"parent_task_id": "task_parent"},
			map[string]any{"task_id": "task_root"},
		},
	}, "2026-04-08T10:00:00Z")

	taskIDs := taskIDsFromResponse(response)
	sort.Strings(taskIDs)

	expected := []string{"task_parent", "task_root"}
	if !reflect.DeepEqual(taskIDs, expected) {
		t.Fatalf("expected nested task ids %v, got %v", expected, taskIDs)
	}
}

func TestOwnedTaskIDsForReplayIgnoresAggregateReadResponseTaskIDs(t *testing.T) {
	response := newSuccessEnvelope(json.RawMessage(`"req-task-list"`), map[string]any{
		"items": []any{
			map[string]any{"task_id": "task_list_a"},
			map[string]any{"task_id": "task_list_b"},
		},
	}, "2026-04-08T10:00:00Z")

	taskIDs := ownedTaskIDsForReplay("agent.task.list", nil, response)
	if taskIDs != nil {
		t.Fatalf("expected aggregate read response not to claim task ownership, got %v", taskIDs)
	}
}

func TestOwnedTaskIDsForReplayClaimsResponseTaskIDsForTaskStart(t *testing.T) {
	response := newSuccessEnvelope(json.RawMessage(`"req-task-start"`), map[string]any{
		"task": map[string]any{
			"task_id": "task_started",
		},
	}, "2026-04-08T10:00:00Z")

	taskIDs := ownedTaskIDsForReplay("agent.task.start", nil, response)
	expected := []string{"task_started"}
	if !reflect.DeepEqual(taskIDs, expected) {
		t.Fatalf("expected task.start to claim response task ids %v, got %v", expected, taskIDs)
	}
}

func TestOwnedTaskIDsForReplayKeepsTrackedTaskIDsOnErrorResponse(t *testing.T) {
	response := newErrorEnvelope(json.RawMessage(`"req-task-control"`), &rpcError{
		Code:    errMethodNotFound,
		Message: "METHOD_NOT_FOUND",
		Detail:  "task updated before downstream failure",
		TraceID: "trace_task_control_failed",
	})

	taskIDs := ownedTaskIDsForReplay("agent.task.control", map[string]bool{
		"task_tracked": true,
	}, response)
	expected := []string{"task_tracked"}
	if !reflect.DeepEqual(taskIDs, expected) {
		t.Fatalf("expected error response to keep tracked task ids %v, got %v", expected, taskIDs)
	}
}

func TestRequestRoutingHintsExtractsTaskSessionAndTrace(t *testing.T) {
	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-routing-hints"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"task_id":    "task_primary",
			"session_id": "sess_routing",
			"request_meta": map[string]any{
				"trace_id": "trace_routing",
			},
			"related": []any{
				map[string]any{"child_task_id": "task_child"},
			},
		}),
	}

	taskIDs, sessionID, traceID := requestRoutingHints(request)
	if sessionID != "sess_routing" || traceID != "trace_routing" {
		t.Fatalf("expected session and trace hints, got session=%q trace=%q", sessionID, traceID)
	}
	if len(taskIDs) != 2 || !taskIDs["task_primary"] || !taskIDs["task_child"] {
		t.Fatalf("expected routing task ids to include primary and child ids, got %+v", taskIDs)
	}
}

func TestNotificationKeyNormalizesLiveRuntimePayload(t *testing.T) {
	withEventPayload := notificationKey("loop.round.completed", "", map[string]any{
		"task_id": " task_live ",
		"event": map[string]any{
			"payload": map[string]any{
				"round": float64(1),
			},
		},
	})
	withDirectPayload := notificationKey("loop.round.completed", "task_live", map[string]any{
		"round": float64(1),
	})

	if withEventPayload != withDirectPayload {
		t.Fatalf("expected equivalent live runtime notifications to share a key, got %q and %q", withEventPayload, withDirectPayload)
	}
}
