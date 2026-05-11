package orchestrator

import "testing"

func TestStartTaskRequestFromParamsNormalizesUnknownFields(t *testing.T) {
	request := StartTaskRequestFromParams(map[string]any{
		"request_meta": map[string]any{
			"trace_id":    "trace_task_start_normalize",
			"client_time": "2026-05-10T00:00:00Z",
		},
		"session_id": "sess_task_start_normalize",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type":          "text_selection",
			"text":          "selected content",
			"unknown_field": "drop-me",
			"page_context": map[string]any{
				"title":         "Editor",
				"unknown_field": "drop-me",
			},
		},
		"context": map[string]any{
			"selection": map[string]any{
				"text":          "selected content",
				"unknown_field": "drop-me",
			},
		},
		"unknown_field": "drop-me",
	})

	params := request.ProtocolParamsMap()
	if _, ok := params["unknown_field"]; ok {
		t.Fatalf("expected typed request normalization to drop unknown top-level fields, got %+v", params)
	}
	input := mapValue(params, "input")
	if _, ok := input["unknown_field"]; ok {
		t.Fatalf("expected typed request normalization to drop unknown input fields, got %+v", input)
	}
	pageContext := mapValue(input, "page_context")
	if _, ok := pageContext["unknown_field"]; ok {
		t.Fatalf("expected typed request normalization to drop unknown page_context fields, got %+v", pageContext)
	}
	selection := mapValue(mapValue(params, "context"), "selection")
	if _, ok := selection["unknown_field"]; ok {
		t.Fatalf("expected typed request normalization to drop unknown selection fields, got %+v", selection)
	}
	if stringValue(input, "text", "") != "selected content" {
		t.Fatalf("expected typed request normalization to preserve declared fields, got %+v", input)
	}
}

func TestTaskEntryResponseMapNormalizesUnknownFields(t *testing.T) {
	response, err := newTaskEntryResponse(map[string]any{
		"task": map[string]any{
			"task_id":      "task_123",
			"session_id":   "sess_123",
			"title":        "Summarize selection",
			"source_type":  "selection",
			"status":       "processing",
			"intent":       nil,
			"current_step": "generate_output",
			"risk_level":   "green",
			"started_at":   "2026-05-10T00:00:00Z",
			"updated_at":   "2026-05-10T00:00:00Z",
			"finished_at":  nil,
			"unknown":      "drop-me",
		},
		"bubble_message": map[string]any{
			"bubble_id":  "bubble_123",
			"task_id":    "task_123",
			"type":       "result",
			"text":       "Done",
			"pinned":     false,
			"hidden":     false,
			"created_at": "2026-05-10T00:00:00Z",
			"unknown":    "drop-me",
		},
		"delivery_result": nil,
		"unknown":         "drop-me",
	})
	if err != nil {
		t.Fatalf("build task entry response failed: %v", err)
	}

	mapped := response.Map()
	if _, ok := mapped["unknown"]; ok {
		t.Fatalf("expected typed response normalization to drop unknown top-level fields, got %+v", mapped)
	}
	task := mapValue(mapped, "task")
	if _, ok := task["unknown"]; ok {
		t.Fatalf("expected typed response normalization to drop unknown task fields, got %+v", task)
	}
	bubble := mapValue(mapped, "bubble_message")
	if _, ok := bubble["unknown"]; ok {
		t.Fatalf("expected typed response normalization to drop unknown bubble fields, got %+v", bubble)
	}
	if stringValue(task, "task_id", "") != "task_123" {
		t.Fatalf("expected typed response normalization to preserve declared task fields, got %+v", task)
	}
}

func TestTaskEntryResponseIgnoresUnknownNonJSONFields(t *testing.T) {
	response, err := newTaskEntryResponse(map[string]any{
		"task": map[string]any{
			"task_id":      "task_non_json_unknown",
			"title":        "Ignore unknown function field",
			"source_type":  "floating_ball",
			"status":       "completed",
			"current_step": "deliver_result",
			"risk_level":   "green",
			"started_at":   "2026-05-10T00:00:00Z",
			"updated_at":   "2026-05-10T00:00:01Z",
		},
		"unknown": func() {},
	})
	if err != nil {
		t.Fatalf("expected unknown non-json field to stay outside response dto, got %v", err)
	}

	task := mapValue(response.Map(), "task")
	if stringValue(task, "task_id", "") != "task_non_json_unknown" {
		t.Fatalf("expected direct response mapping to preserve declared fields, got %+v", task)
	}
}

func TestTaskEntryResponseRejectsMissingRequiredDeclaredFields(t *testing.T) {
	testCases := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "task.status",
			payload: map[string]any{
				"task": map[string]any{
					"task_id":      "task_missing_status",
					"title":        "Missing status",
					"source_type":  "floating_ball",
					"current_step": "deliver_result",
					"risk_level":   "green",
					"updated_at":   "2026-05-10T00:00:01Z",
				},
			},
		},
		{
			name: "bubble_message.pinned",
			payload: map[string]any{
				"task": map[string]any{
					"task_id":      "task_with_bubble_missing_bool",
					"title":        "Missing bubble bool",
					"source_type":  "floating_ball",
					"status":       "completed",
					"current_step": "deliver_result",
					"risk_level":   "green",
					"updated_at":   "2026-05-10T00:00:01Z",
				},
				"bubble_message": map[string]any{
					"bubble_id":  "bubble_missing_pinned",
					"task_id":    "task_with_bubble_missing_bool",
					"type":       "result",
					"text":       "Done",
					"hidden":     false,
					"created_at": "2026-05-10T00:00:01Z",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := newTaskEntryResponse(testCase.payload); err == nil {
				t.Fatalf("expected missing required declared field to fail for %s", testCase.name)
			}
		})
	}
}

func TestTaskEntryResponseNormalizesIntentArgumentsToStableObject(t *testing.T) {
	response, err := newTaskEntryResponse(map[string]any{
		"task": map[string]any{
			"task_id":      "task_intent_arguments",
			"session_id":   "sess_intent_arguments",
			"title":        "Preserve empty intent arguments",
			"source_type":  "floating_ball",
			"status":       "processing",
			"intent":       map[string]any{"name": "agent_loop"},
			"current_step": "awaiting_execution",
			"risk_level":   "green",
			"started_at":   "2026-05-10T00:00:00Z",
			"updated_at":   "2026-05-10T00:00:01Z",
		},
	})
	if err != nil {
		t.Fatalf("build task entry response failed: %v", err)
	}

	intent := mapValue(mapValue(response.Map(), "task"), "intent")
	if _, ok := intent["arguments"]; !ok {
		t.Fatalf("expected stable intent payload to include arguments, got %+v", intent)
	}
	arguments := mapValue(intent, "arguments")
	if len(arguments) != 0 {
		t.Fatalf("expected empty intent arguments object, got %+v", arguments)
	}
}
