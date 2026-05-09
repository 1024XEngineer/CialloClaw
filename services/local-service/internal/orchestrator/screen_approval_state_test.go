package orchestrator

import (
	"reflect"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func TestBuildScreenAnalysisApprovalStateTypedGolden(t *testing.T) {
	service := newTestService()
	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_screen_state",
		Title:       "Analyze current screen",
		SourceType:  "screen_capture",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		Intent: map[string]any{
			"name": "screen_analyze",
			"arguments": map[string]any{
				"path":          "inputs/screen.png",
				"language":      "eng",
				"evidence_role": "error_evidence",
			},
		},
	})

	state, err := service.buildScreenAnalysisApprovalState(task)
	if err != nil {
		t.Fatalf("build screen approval state failed: %v", err)
	}

	got := map[string]any{
		"approval_request":  state.approvalRequestMap(),
		"pending_execution": state.pendingExecutionMap(),
		"bubble_message":    state.bubbleMessageMap(),
	}
	got["approval_request"].(map[string]any)["approval_id"] = "<approval_id>"
	got["approval_request"].(map[string]any)["created_at"] = "<created_at>"
	got["bubble_message"].(map[string]any)["created_at"] = "<created_at>"

	want := map[string]any{
		"approval_request": map[string]any{
			"approval_id":    "<approval_id>",
			"task_id":        task.TaskID,
			"operation_name": "screen_capture",
			"risk_level":     "yellow",
			"target_object":  "inputs/screen.png",
			"reason":         "screen_capture_requires_authorization",
			"status":         "pending",
			"created_at":     "<created_at>",
		},
		"pending_execution": map[string]any{
			"kind":           "screen_analysis",
			"operation_name": "screen_capture",
			"source_path":    "inputs/screen.png",
			"capture_mode":   "screenshot",
			"source":         "screen_capture",
			"target_object":  "inputs/screen.png",
			"language":       "eng",
			"evidence_role":  "error_evidence",
			"delivery_type":  "bubble",
			"result_title":   "屏幕分析结果",
			"preview_text":   "已准备分析屏幕截图",
			"impact_scope": map[string]any{
				"files":                    []string{"inputs/screen.png"},
				"webpages":                 []string{},
				"apps":                     []string{},
				"out_of_workspace":         false,
				"overwrite_or_delete_risk": false,
			},
		},
		"bubble_message": map[string]any{
			"bubble_id":  "bubble_" + task.TaskID,
			"task_id":    task.TaskID,
			"type":       "status",
			"text":       "屏幕截图分析属于敏感能力，请先确认授权。",
			"pinned":     false,
			"hidden":     false,
			"created_at": "<created_at>",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected screen approval state:\nwant: %#v\n got: %#v", want, got)
	}
}
