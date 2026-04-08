// 该文件负责交付结果、气泡消息与 artifact 计划的组装。
package delivery

import (
	"fmt"
	"regexp"
	"strings"
)

// StorageWritePlan 定义当前模块的数据结构。
type StorageWritePlan struct {
	TaskID       string
	TargetPath   string
	MimeType     string
	DeliveryType string
	Source       string
}

// ArtifactPersistPlan 定义当前模块的数据结构。
type ArtifactPersistPlan struct {
	ArtifactID   string
	TaskID       string
	ArtifactType string
	Path         string
	MimeType     string
}

// ApprovalExecutionPlan 描述授权通过后继续执行所需的最小交付计划。
type ApprovalExecutionPlan struct {
	TaskID           string
	DeliveryType     string
	ResultTitle      string
	PreviewText      string
	ResultBubbleText string
}

// Service 提供当前模块的服务能力。
type Service struct{}

// NewService 创建并返回Service。
func NewService() *Service {
	return &Service{}
}

// DefaultResultType 处理当前模块的相关逻辑。
func (s *Service) DefaultResultType() string {
	return "workspace_document"
}

// BuildBubbleMessage 构建BubbleMessage。
func (s *Service) BuildBubbleMessage(taskID, bubbleType, text, createdAt string) map[string]any {
	return map[string]any{
		"bubble_id":  fmt.Sprintf("bubble_%s", taskID),
		"task_id":    taskID,
		"type":       bubbleType,
		"text":       text,
		"pinned":     false,
		"hidden":     false,
		"created_at": createdAt,
	}
}

// BuildDeliveryResult 构建DeliveryResult。
func (s *Service) BuildDeliveryResult(taskID, deliveryType, title, previewText string) map[string]any {
	payload := map[string]any{
		"path":    nil,
		"url":     nil,
		"task_id": taskID,
	}

	if deliveryType == "workspace_document" {
		payload["path"] = fmt.Sprintf("D:/CialloClawWorkspace/%s.md", slugify(title, taskID))
	}

	return map[string]any{
		"type":         deliveryType,
		"title":        title,
		"payload":      payload,
		"preview_text": previewText,
	}
}

// BuildArtifact 构建Artifact。
func (s *Service) BuildArtifact(taskID, title string, deliveryResult map[string]any) []map[string]any {
	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		return nil
	}

	path, _ := payload["path"].(string)
	if path == "" {
		return nil
	}

	return []map[string]any{
		{
			"artifact_id":   fmt.Sprintf("art_%s", taskID),
			"task_id":       taskID,
			"artifact_type": "generated_doc",
			"title":         fmt.Sprintf("%s.md", title),
			"path":          path,
			"mime_type":     "text/markdown",
		},
	}
}

// BuildStorageWritePlan 构建StorageWritePlan。
func (s *Service) BuildStorageWritePlan(taskID string, deliveryResult map[string]any) map[string]any {
	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		return nil
	}

	path, _ := payload["path"].(string)
	if path == "" {
		return nil
	}

	return map[string]any{
		"task_id":       taskID,
		"target_path":   path,
		"mime_type":     "text/markdown",
		"delivery_type": deliveryResult["type"],
		"source":        "delivery_result",
	}
}

// BuildArtifactPersistPlans 构建ArtifactPersistPlans。
func (s *Service) BuildArtifactPersistPlans(taskID string, artifacts []map[string]any) []map[string]any {
	if len(artifacts) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		result = append(result, map[string]any{
			"artifact_id":   artifact["artifact_id"],
			"task_id":       taskID,
			"artifact_type": artifact["artifact_type"],
			"path":          artifact["path"],
			"mime_type":     artifact["mime_type"],
		})
	}

	return result
}

// BuildApprovalExecutionPlan 构建授权通过后的继续执行计划。
func (s *Service) BuildApprovalExecutionPlan(taskID string, intent map[string]any) map[string]any {
	intentName, _ := intent["name"].(string)
	plan := map[string]any{
		"task_id":            taskID,
		"delivery_type":      "workspace_document",
		"result_title":       "处理结果",
		"preview_text":       "已为你写入文档并打开",
		"result_bubble_text": "结果已经生成，可直接查看。",
	}

	switch intentName {
	case "rewrite":
		plan["result_title"] = "改写结果"
		plan["result_bubble_text"] = "内容已经按要求改写完成，可直接查看。"
	case "translate":
		plan["delivery_type"] = "bubble"
		plan["result_title"] = "翻译结果"
		plan["preview_text"] = "结果已通过气泡返回"
		plan["result_bubble_text"] = "翻译结果已经生成，可直接查看。"
	case "explain":
		plan["delivery_type"] = "bubble"
		plan["result_title"] = "解释结果"
		plan["preview_text"] = "结果已通过气泡返回"
		plan["result_bubble_text"] = "这段内容的意思已经整理好了。"
	case "write_file":
		plan["result_title"] = "文件写入结果"
		plan["result_bubble_text"] = "文件已经生成，可直接查看。"
	case "summarize":
		plan["result_title"] = "总结结果"
		plan["result_bubble_text"] = "总结结果已经生成，可直接查看。"
	}

	return plan
}

// slugify 处理当前模块的相关逻辑。
func slugify(title, fallback string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return fallback
	}

	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	cleaner := regexp.MustCompile(`[^\p{Han}A-Za-z0-9_-]+`)
	trimmed = cleaner.ReplaceAllString(trimmed, "")
	trimmed = strings.Trim(trimmed, "-")
	if trimmed == "" {
		return fallback
	}

	return trimmed
}
