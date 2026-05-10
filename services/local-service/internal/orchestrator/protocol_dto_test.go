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

	params := request.paramsMap()
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
	response := newTaskEntryResponse(map[string]any{
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

func TestTaskDetailGetResponseMapNormalizesUnknownFields(t *testing.T) {
	response := newTaskDetailGetResponse(map[string]any{
		"task": map[string]any{
			"task_id":      "task_detail_123",
			"session_id":   "sess_123",
			"title":        "Explain error",
			"source_type":  "error",
			"status":       "completed",
			"intent":       nil,
			"current_step": "generate_output",
			"risk_level":   "green",
			"started_at":   "2026-05-10T00:00:00Z",
			"updated_at":   "2026-05-10T00:00:01Z",
			"finished_at":  "2026-05-10T00:00:02Z",
			"unknown":      "drop-me",
		},
		"timeline": []map[string]any{},
		"delivery_result": map[string]any{
			"type":         "bubble",
			"title":        "Explanation",
			"preview_text": "Summary",
			"payload": map[string]any{
				"path":    nil,
				"url":     nil,
				"task_id": "task_detail_123",
				"unknown": "drop-me",
			},
			"unknown": "drop-me",
		},
		"artifacts":            []map[string]any{},
		"citations":            []map[string]any{},
		"mirror_references":    []map[string]any{},
		"approval_request":     nil,
		"authorization_record": nil,
		"audit_record":         nil,
		"security_summary":     map[string]any{"pending_authorizations": 0, "latest_restore_point": nil, "unknown": "drop-me"},
		"runtime_summary":      map[string]any{"loop_stop_reason": nil, "events_count": 0, "latest_event_type": nil, "active_steering_count": 0, "latest_failure_code": nil, "latest_failure_category": nil, "latest_failure_summary": nil, "observation_signals": []string{}, "unknown": "drop-me"},
		"unknown":              "drop-me",
	})

	mapped := response.Map()
	if _, ok := mapped["unknown"]; ok {
		t.Fatalf("expected typed detail normalization to drop unknown top-level fields, got %+v", mapped)
	}
	deliveryResult := mapValue(mapped, "delivery_result")
	if _, ok := deliveryResult["unknown"]; ok {
		t.Fatalf("expected typed detail normalization to drop unknown delivery_result fields, got %+v", deliveryResult)
	}
	payload := mapValue(deliveryResult, "payload")
	if _, ok := payload["unknown"]; ok {
		t.Fatalf("expected typed detail normalization to drop unknown delivery payload fields, got %+v", payload)
	}
	securitySummary := mapValue(mapped, "security_summary")
	if _, ok := securitySummary["unknown"]; ok {
		t.Fatalf("expected typed detail normalization to drop unknown security summary fields, got %+v", securitySummary)
	}
	runtimeSummary := mapValue(mapped, "runtime_summary")
	if _, ok := runtimeSummary["unknown"]; ok {
		t.Fatalf("expected typed detail normalization to drop unknown runtime summary fields, got %+v", runtimeSummary)
	}
}
