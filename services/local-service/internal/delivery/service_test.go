// 该测试文件验证交付与落盘计划组装行为。
package delivery

import "testing"

// TestBuildStorageAndArtifactPlans 验证BuildStorageAndArtifactPlans。
func TestBuildStorageAndArtifactPlans(t *testing.T) {
	service := NewService()
	deliveryResult := service.BuildDeliveryResult("task_001", "workspace_document", "测试摘要", "已为你写入文档并打开")
	artifacts := service.BuildArtifact("task_001", "测试摘要", deliveryResult)

	storagePlan := service.BuildStorageWritePlan("task_001", deliveryResult)
	if storagePlan == nil {
		t.Fatal("expected storage write plan to be generated for workspace document delivery")
	}
	if storagePlan["target_path"] == nil {
		t.Fatal("expected storage write plan to carry a target path")
	}
	if storagePlan["target_path"] != "workspace/测试摘要.md" {
		t.Fatalf("expected storage write plan to use workspace-relative target path, got %v", storagePlan["target_path"])
	}

	artifactPlans := service.BuildArtifactPersistPlans("task_001", artifacts)
	if len(artifactPlans) != 1 {
		t.Fatalf("expected one artifact persist plan, got %d", len(artifactPlans))
	}
}

// TestBuildDeliveryResultWithTargetPath 验证显式输出路径会进入 delivery_result 和 artifact。
func TestBuildDeliveryResultWithTargetPath(t *testing.T) {
	service := NewService()
	deliveryResult := service.BuildDeliveryResultWithTargetPath(
		"task_001",
		"workspace_document",
		"文件写入结果",
		"已为你写入文档并打开",
		"notes/output.md",
	)

	payload := deliveryResult["payload"].(map[string]any)
	if payload["path"] != "workspace/notes/output.md" {
		t.Fatalf("expected explicit target path to be normalized into workspace, got %v", payload["path"])
	}

	artifacts := service.BuildArtifact("task_001", "文件写入结果", deliveryResult)
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %d", len(artifacts))
	}
	if artifacts[0]["title"] != "output.md" {
		t.Fatalf("expected artifact title to follow target path base name, got %v", artifacts[0]["title"])
	}
}

func TestBuildArtifactPersistPlansBackfillsDeliveryPayloadAndCreatedAt(t *testing.T) {
	service := NewService()
	plans := service.BuildArtifactPersistPlans("task_001", []map[string]any{{
		"artifact_id":      "art_001",
		"artifact_type":    "generated_doc",
		"title":            "result.md",
		"path":             "workspace/result.md",
		"mime_type":        "text/markdown",
		"delivery_type":    "open_file",
		"delivery_payload": nil,
	}})
	if len(plans) != 1 {
		t.Fatalf("expected one artifact persist plan, got %d", len(plans))
	}
	if plans[0]["delivery_payload_json"] != "{}" {
		t.Fatalf("expected empty delivery payload json fallback, got %+v", plans[0])
	}
	if plans[0]["created_at"] == "" {
		t.Fatalf("expected created_at fallback, got %+v", plans[0])
	}
}

func TestBuildArtifactPersistPlansAssignsIdentifiersWhenMissing(t *testing.T) {
	service := NewService()
	plans := service.BuildArtifactPersistPlans("task_001", []map[string]any{{
		"artifact_type": "generated_file",
		"title":         "result.txt",
		"path":          "workspace/result.txt",
		"mime_type":     "text/plain",
		"delivery_type": "open_file",
	}})
	if len(plans) != 1 {
		t.Fatalf("expected one artifact persist plan, got %d", len(plans))
	}
	if plans[0]["artifact_id"] == "" {
		t.Fatalf("expected missing artifact_id to be backfilled, got %+v", plans[0])
	}
	if plans[0]["task_id"] != "task_001" {
		t.Fatalf("expected task_id to be preserved during identifier backfill, got %+v", plans[0])
	}
}

func TestBuildArtifactReturnsNilWithoutUsablePayload(t *testing.T) {
	service := NewService()
	if artifacts := service.BuildArtifact("task_001", "title", map[string]any{"payload": "invalid"}); artifacts != nil {
		t.Fatalf("expected invalid payload to skip artifacts, got %+v", artifacts)
	}
	if artifacts := service.BuildArtifact("task_001", "title", map[string]any{"payload": map[string]any{"path": ""}}); artifacts != nil {
		t.Fatalf("expected empty path to skip artifacts, got %+v", artifacts)
	}
}

func TestBuildApprovalExecutionPlanRoutesByIntent(t *testing.T) {
	service := NewService()
	translatePlan := service.BuildApprovalExecutionPlan("task_001", map[string]any{"name": "translate"})
	if translatePlan["delivery_type"] != "bubble" {
		t.Fatalf("expected translate plan to use bubble delivery, got %+v", translatePlan)
	}
	writePlan := service.BuildApprovalExecutionPlan("task_001", map[string]any{"name": "write_file"})
	if writePlan["result_title"] != "文件写入结果" {
		t.Fatalf("expected write_file title override, got %+v", writePlan)
	}
}

func TestDeliveryHelpersExposeDefaultAndBubbleContract(t *testing.T) {
	service := NewService()
	if service.DefaultResultType() != "workspace_document" {
		t.Fatalf("expected workspace_document default result type, got %q", service.DefaultResultType())
	}
	bubble := service.BuildBubbleMessage("task_001", "status", "done", "2026-04-14T10:00:00Z")
	if bubble["bubble_id"] != "bubble_task_001" || bubble["task_id"] != "task_001" || bubble["type"] != "status" {
		t.Fatalf("unexpected bubble message payload: %+v", bubble)
	}
}
