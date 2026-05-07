package rpc

import (
	"encoding/json"
	"testing"
)

func TestStreamRequestTrackerAssociatesStartedTaskByTrace(t *testing.T) {
	tracker := newStreamRequestTracker(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-tracker-start"`),
		Method:  "agent.input.submit",
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_tracker",
			"request_meta": map[string]any{
				"trace_id": "trace_tracker",
			},
		}),
	})

	if !tracker.shouldSubscribeRuntime() || !tracker.shouldSubscribeTaskStart() {
		t.Fatal("expected input.submit tracker to subscribe before the task id is known")
	}
	if !tracker.matchesTaskStart("other_session", "trace_tracker") {
		t.Fatal("expected trace id to associate the started task with this request")
	}
	if tracker.matchesTaskStart("sess_tracker", "other_trace") {
		t.Fatal("expected trace id to take precedence over session id")
	}

	tracker.addTaskID(" task_from_start ")
	if !tracker.hasTaskID("task_from_start") {
		t.Fatal("expected tracker to remember the started task id")
	}
}

func TestStreamRequestTrackerConsumesLiveRuntimeReplayOnce(t *testing.T) {
	tracker := newStreamRequestTracker(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-tracker-replay"`),
		Method:  "agent.task.detail.get",
		Params:  mustMarshal(t, map[string]any{"task_id": "task_tracker"}),
	})
	if !tracker.shouldSubscribeRuntime() || tracker.shouldSubscribeTaskStart() {
		t.Fatal("expected existing task requests to subscribe only to runtime notifications")
	}

	params := map[string]any{"task_id": "task_tracker", "round": float64(1)}
	tracker.recordStreamedRuntime("loop.round.completed", "task_tracker", params)
	if !tracker.shouldSkipBufferedRuntime("loop.round.completed", "task_tracker", params) {
		t.Fatal("expected buffered replay to skip the already streamed runtime notification")
	}
	if tracker.shouldSkipBufferedRuntime("loop.round.completed", "task_tracker", params) {
		t.Fatal("expected streamed runtime count to be consumed once")
	}
	if tracker.shouldSkipBufferedRuntime("task.updated", "task_tracker", params) {
		t.Fatal("expected non-runtime notifications to remain eligible for replay")
	}
}
