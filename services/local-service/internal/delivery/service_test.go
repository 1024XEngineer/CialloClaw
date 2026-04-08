// 该测试文件验证交付与落盘计划组装行为。
package delivery

import "testing"

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

	artifactPlans := service.BuildArtifactPersistPlans("task_001", artifacts)
	if len(artifactPlans) != 1 {
		t.Fatalf("expected one artifact persist plan, got %d", len(artifactPlans))
	}
}
