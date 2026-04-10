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
