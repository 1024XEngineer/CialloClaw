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

func TestTaskDetailGetResponseMapNormalizesUnknownFields(t *testing.T) {
	response, err := newTaskDetailGetResponse(map[string]any{
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
		"security_summary":     map[string]any{"security_status": "normal", "risk_level": "green", "pending_authorizations": 0, "latest_restore_point": nil, "unknown": "drop-me"},
		"runtime_summary":      map[string]any{"loop_stop_reason": nil, "events_count": 0, "latest_event_type": nil, "active_steering_count": 0, "latest_failure_code": nil, "latest_failure_category": nil, "latest_failure_summary": nil, "observation_signals": []string{}, "unknown": "drop-me"},
		"unknown":              "drop-me",
	})
	if err != nil {
		t.Fatalf("build task detail response failed: %v", err)
	}

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

func TestTaskDetailGetResponseMapDropsAuthorizationRecordRunID(t *testing.T) {
	response, err := newTaskDetailGetResponse(map[string]any{
		"task": map[string]any{
			"task_id":      "task_detail_auth",
			"session_id":   "sess_auth",
			"title":        "Inspect approval history",
			"source_type":  "screen_capture",
			"status":       "completed",
			"current_step": "generate_output",
			"risk_level":   "yellow",
			"started_at":   "2026-05-10T00:00:00Z",
			"updated_at":   "2026-05-10T00:00:01Z",
		},
		"timeline":          []map[string]any{},
		"delivery_result":   nil,
		"artifacts":         []map[string]any{},
		"citations":         []map[string]any{},
		"mirror_references": []map[string]any{},
		"approval_request":  nil,
		"authorization_record": map[string]any{
			"authorization_record_id": "auth_001",
			"task_id":                 "task_detail_auth",
			"run_id":                  "run_hidden",
			"approval_id":             "appr_001",
			"decision":                "allow_once",
			"remember_rule":           false,
			"operator":                "user",
			"created_at":              "2026-05-10T00:00:02Z",
		},
		"audit_record":     nil,
		"security_summary": map[string]any{"security_status": "normal", "risk_level": "yellow", "pending_authorizations": 0, "latest_restore_point": nil},
		"runtime_summary":  map[string]any{"events_count": 0, "active_steering_count": 0, "observation_signals": []string{}},
	})
	if err != nil {
		t.Fatalf("build task detail response failed: %v", err)
	}

	mapped := response.Map()
	authorizationRecord := mapValue(mapped, "authorization_record")
	if _, ok := authorizationRecord["run_id"]; ok {
		t.Fatalf("expected authorization_record run_id to stay out of the stable dto, got %+v", authorizationRecord)
	}
	if stringValue(authorizationRecord, "authorization_record_id", "") != "auth_001" {
		t.Fatalf("expected typed detail normalization to preserve declared authorization fields, got %+v", authorizationRecord)
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

func TestTaskDetailGetResponseRejectsMalformedDeclaredObjects(t *testing.T) {
	_, err := newTaskDetailGetResponse(map[string]any{"task": "not-an-object"})
	if err == nil {
		t.Fatal("expected malformed declared task object to fail response dto construction")
	}
}

func TestTaskDetailGetResponseRejectsMissingRequiredSummaryObjects(t *testing.T) {
	basePayload := map[string]any{
		"task": map[string]any{
			"task_id":      "task_detail_required_objects",
			"title":        "Required task detail objects",
			"source_type":  "floating_ball",
			"status":       "completed",
			"current_step": "deliver_result",
			"risk_level":   "green",
			"updated_at":   "2026-05-10T00:00:01Z",
		},
		"timeline":             []map[string]any{},
		"delivery_result":      nil,
		"artifacts":            []map[string]any{},
		"citations":            []map[string]any{},
		"mirror_references":    []map[string]any{},
		"approval_request":     nil,
		"authorization_record": nil,
		"audit_record":         nil,
		"security_summary": map[string]any{
			"security_status":        "safe",
			"risk_level":             "green",
			"pending_authorizations": 0,
			"latest_restore_point":   nil,
		},
		"runtime_summary": map[string]any{
			"events_count":            0,
			"active_steering_count":   0,
			"observation_signals":     []string{},
			"latest_event_type":       nil,
			"latest_failure_code":     nil,
			"latest_failure_category": nil,
			"latest_failure_summary":  nil,
			"loop_stop_reason":        nil,
		},
	}

	testCases := []struct {
		name    string
		missing string
	}{
		{name: "missing security_summary", missing: "security_summary"},
		{name: "missing runtime_summary", missing: "runtime_summary"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload := cloneMap(basePayload)
			delete(payload, testCase.missing)
			if _, err := newTaskDetailGetResponse(payload); err == nil {
				t.Fatalf("expected missing %s to fail task detail dto construction", testCase.missing)
			}
		})
	}
}

func TestTaskDetailGetResponseRejectsMissingRequiredSummaryFields(t *testing.T) {
	testCases := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "security_summary pending_authorizations",
			payload: map[string]any{
				"task": map[string]any{
					"task_id":      "task_detail_missing_security_field",
					"title":        "Missing security field",
					"source_type":  "floating_ball",
					"status":       "completed",
					"current_step": "deliver_result",
					"risk_level":   "green",
					"updated_at":   "2026-05-10T00:00:01Z",
				},
				"timeline":             []map[string]any{},
				"delivery_result":      nil,
				"artifacts":            []map[string]any{},
				"citations":            []map[string]any{},
				"mirror_references":    []map[string]any{},
				"approval_request":     nil,
				"authorization_record": nil,
				"audit_record":         nil,
				"security_summary": map[string]any{
					"security_status":      "safe",
					"risk_level":           "green",
					"latest_restore_point": nil,
				},
				"runtime_summary": map[string]any{
					"events_count":          0,
					"active_steering_count": 0,
					"observation_signals":   []string{},
				},
			},
		},
		{
			name: "runtime_summary observation_signals",
			payload: map[string]any{
				"task": map[string]any{
					"task_id":      "task_detail_missing_runtime_field",
					"title":        "Missing runtime field",
					"source_type":  "floating_ball",
					"status":       "completed",
					"current_step": "deliver_result",
					"risk_level":   "green",
					"updated_at":   "2026-05-10T00:00:01Z",
				},
				"timeline":             []map[string]any{},
				"delivery_result":      nil,
				"artifacts":            []map[string]any{},
				"citations":            []map[string]any{},
				"mirror_references":    []map[string]any{},
				"approval_request":     nil,
				"authorization_record": nil,
				"audit_record":         nil,
				"security_summary": map[string]any{
					"security_status":        "safe",
					"risk_level":             "green",
					"pending_authorizations": 0,
					"latest_restore_point":   nil,
				},
				"runtime_summary": map[string]any{
					"events_count":          0,
					"active_steering_count": 0,
				},
			},
		},
		{
			name: "delivery_result payload",
			payload: map[string]any{
				"task": map[string]any{
					"task_id":      "task_detail_missing_delivery_payload",
					"title":        "Missing delivery payload",
					"source_type":  "floating_ball",
					"status":       "completed",
					"current_step": "deliver_result",
					"risk_level":   "green",
					"updated_at":   "2026-05-10T00:00:01Z",
				},
				"timeline": []map[string]any{},
				"delivery_result": map[string]any{
					"type":         "bubble",
					"title":        "Result",
					"preview_text": "Preview",
				},
				"artifacts":            []map[string]any{},
				"citations":            []map[string]any{},
				"mirror_references":    []map[string]any{},
				"approval_request":     nil,
				"authorization_record": nil,
				"audit_record":         nil,
				"security_summary": map[string]any{
					"security_status":        "safe",
					"risk_level":             "green",
					"pending_authorizations": 0,
					"latest_restore_point":   nil,
				},
				"runtime_summary": map[string]any{
					"events_count":          0,
					"active_steering_count": 0,
					"observation_signals":   []string{},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := newTaskDetailGetResponse(testCase.payload); err == nil {
				t.Fatalf("expected missing required declared field to fail for %s", testCase.name)
			}
		})
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

func TestTaskDetailGetResponseRejectsMissingRequiredDeclaredFields(t *testing.T) {
	testCases := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "task.task_id",
			payload: map[string]any{
				"task": map[string]any{
					"title":        "Missing task id",
					"source_type":  "floating_ball",
					"status":       "completed",
					"current_step": "deliver_result",
					"risk_level":   "green",
					"updated_at":   "2026-05-10T00:00:01Z",
				},
				"timeline":             []map[string]any{},
				"delivery_result":      nil,
				"artifacts":            []map[string]any{},
				"citations":            []map[string]any{},
				"mirror_references":    []map[string]any{},
				"approval_request":     nil,
				"authorization_record": nil,
				"audit_record":         nil,
				"security_summary": map[string]any{
					"security_status":        "safe",
					"risk_level":             "green",
					"pending_authorizations": 0,
					"latest_restore_point":   nil,
				},
				"runtime_summary": map[string]any{
					"events_count":          0,
					"active_steering_count": 0,
					"observation_signals":   []string{},
				},
			},
		},
		{
			name: "timeline array",
			payload: map[string]any{
				"task": map[string]any{
					"task_id":      "task_missing_timeline",
					"title":        "Missing timeline",
					"source_type":  "floating_ball",
					"status":       "completed",
					"current_step": "deliver_result",
					"risk_level":   "green",
					"updated_at":   "2026-05-10T00:00:01Z",
				},
				"delivery_result":      nil,
				"artifacts":            []map[string]any{},
				"citations":            []map[string]any{},
				"mirror_references":    []map[string]any{},
				"approval_request":     nil,
				"authorization_record": nil,
				"audit_record":         nil,
				"security_summary": map[string]any{
					"security_status":        "safe",
					"risk_level":             "green",
					"pending_authorizations": 0,
					"latest_restore_point":   nil,
				},
				"runtime_summary": map[string]any{
					"events_count":          0,
					"active_steering_count": 0,
					"observation_signals":   []string{},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := newTaskDetailGetResponse(testCase.payload); err == nil {
				t.Fatalf("expected missing required declared field to fail for %s", testCase.name)
			}
		})
	}
}
