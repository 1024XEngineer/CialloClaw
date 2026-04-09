// 该文件负责交付结果、气泡消息与 artifact 计划的组装。
package delivery

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

const defaultWorkspaceRoot = "workspace"

// StorageWritePlan 定义当前模块的数据结构。

// StorageWritePlan 描述正式交付物需要写入 workspace 时的最小落盘计划。
// runengine 只保存这份计划，不直接执行真实文件写入。
type StorageWritePlan struct {
	TaskID       string
	TargetPath   string
	MimeType     string
	DeliveryType string
	Source       string
}

// ArtifactPersistPlan 定义当前模块的数据结构。

// ArtifactPersistPlan 描述 artifact 元数据后续需要如何持久化。
// 它和 StorageWritePlan 分层存在，避免把大对象落盘和正式交付写入混成一件事。
type ArtifactPersistPlan struct {
	ArtifactID   string
	TaskID       string
	ArtifactType string
	Path         string
	MimeType     string
}

// ApprovalExecutionPlan 描述授权通过后继续执行所需的最小交付计划。

// ApprovalExecutionPlan 描述命中风险并通过授权后，主链路需要恢复的最小交付计划。
// 这里保存的是“恢复执行需要知道什么”，而不是完整运行态快照。
type ApprovalExecutionPlan struct {
	TaskID           string
	DeliveryType     string
	ResultTitle      string
	PreviewText      string
	ResultBubbleText string
}

// Service 提供当前模块的服务能力。

// Service 负责把 runengine 的执行结果组装成对外 task-centric 语义。
// 包括 bubble_message、delivery_result、artifact 计划，以及授权恢复后的默认交付配置。
type Service struct{}

// NewService 创建并返回Service。

// NewService 创建交付服务。
func NewService() *Service {
	return &Service{}
}

// DefaultResultType 处理当前模块的相关逻辑。

// DefaultResultType 返回当前主链路的默认正式交付类型。
func (s *Service) DefaultResultType() string {
	return "workspace_document"
}

// BuildBubbleMessage 构建BubbleMessage。

// BuildBubbleMessage 生成统一的气泡消息结构。
// orchestrator 在确认、授权、完成等节点都通过它构造对前端可直接消费的 bubble_message。
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

// BuildDeliveryResult 生成对外返回的 delivery_result。
// 当交付类型是 workspace_document 时，这里还会同步约定 workspace 内的相对输出路径。
func (s *Service) BuildDeliveryResult(taskID, deliveryType, title, previewText string) map[string]any {
	return s.BuildDeliveryResultWithTargetPath(taskID, deliveryType, title, previewText, "")
}

// BuildDeliveryResultWithTargetPath 生成带显式输出路径的 delivery_result。
func (s *Service) BuildDeliveryResultWithTargetPath(taskID, deliveryType, title, previewText, targetPath string) map[string]any {
	payload := map[string]any{
		"path":    nil,
		"url":     nil,
		"task_id": taskID,
	}

	if deliveryType == "workspace_document" {
		payload["path"] = resolveWorkspaceTargetPath(targetPath, fmt.Sprintf("%s.md", slugify(title, taskID)))
	}

	return map[string]any{
		"type":         deliveryType,
		"title":        title,
		"payload":      payload,
		"preview_text": previewText,
	}
}

// BuildArtifact 构建Artifact。

// BuildArtifact 从正式交付结果反推 artifact 列表。
// 当前主链路只在生成 workspace 文档时附带一个 generated_doc artifact。
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
			"title":         artifactTitle(path, title),
			"path":          path,
			"mime_type":     "text/markdown",
		},
	}
}

// BuildStorageWritePlan 构建StorageWritePlan。

// BuildStorageWritePlan 把 delivery_result 转成 runengine 保存的 workspace 写入计划。
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

// BuildArtifactPersistPlans 把 artifact 列表转换成后续持久化计划。
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

// BuildApprovalExecutionPlan 为等待授权的任务构造恢复执行计划。
// 不同 intent 会在这里得到不同的默认交付类型、结果标题和气泡文案。
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

// slugify 把结果标题转换成适合 workspace 文件名的片段。
// 如果标题清洗后为空，则退回 task_id，避免生成不可用路径。
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

func resolveWorkspaceTargetPath(targetPath, fallbackName string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(targetPath, "\\", "/"))
	if normalized == "" {
		return path.Join(defaultWorkspaceRoot, fallbackName)
	}

	cleaned := path.Clean(normalized)
	switch cleaned {
	case ".", "/", "":
		return path.Join(defaultWorkspaceRoot, fallbackName)
	}

	if strings.HasPrefix(cleaned, "../") {
		return path.Join(defaultWorkspaceRoot, fallbackName)
	}
	if cleaned == defaultWorkspaceRoot {
		return path.Join(defaultWorkspaceRoot, fallbackName)
	}
	if strings.HasPrefix(cleaned, defaultWorkspaceRoot+"/") {
		return cleaned
	}

	return path.Join(defaultWorkspaceRoot, cleaned)
}

func artifactTitle(targetPath, fallbackTitle string) string {
	if trimmed := strings.TrimSpace(targetPath); trimmed != "" {
		return path.Base(trimmed)
	}

	return fmt.Sprintf("%s.md", fallbackTitle)
}
