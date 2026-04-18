// Package delivery assembles task-facing delivery results, bubbles, and
// persistence plans.
package delivery

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"path"
	"regexp"
	"strings"
	"time"
)

const defaultWorkspaceRoot = "workspace"

// StorageWritePlan describes the minimum workspace write plan required for a
// formal delivery result.
type StorageWritePlan struct {
	TaskID       string
	TargetPath   string
	MimeType     string
	DeliveryType string
	Source       string
}

// ArtifactPersistPlan describes how artifact metadata should later be persisted
// without mixing metadata persistence with workspace content writes.
type ArtifactPersistPlan struct {
	ArtifactID   string
	TaskID       string
	ArtifactType string
	Path         string
	MimeType     string
}

// ApprovalExecutionPlan captures the minimum delivery settings required to
// resume execution after a risky action is approved.
type ApprovalExecutionPlan struct {
	TaskID           string
	DeliveryType     string
	ResultTitle      string
	PreviewText      string
	ResultBubbleText string
}

// Service assembles task-centric delivery outputs from runtime execution data.
type Service struct{}

// NewService constructs a delivery service.
func NewService() *Service {
	return &Service{}
}

// DefaultResultType returns the default formal delivery type for the main
// pipeline.
func (s *Service) DefaultResultType() string {
	return "workspace_document"
}

// BuildBubbleMessage creates the stable bubble_message payload returned to the
// frontend across confirmation, authorization, and completion states.
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

// BuildDeliveryResult creates the outward-facing delivery_result payload.
func (s *Service) BuildDeliveryResult(taskID, deliveryType, title, previewText string) map[string]any {
	return s.BuildDeliveryResultWithTargetPath(taskID, deliveryType, title, previewText, "")
}

// BuildDeliveryResultWithTargetPath creates a delivery_result with an explicit
// target path when a caller already resolved one.
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

// BuildArtifact derives artifact metadata from a formal delivery result.
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
			"delivery_type": deliveryResult["type"],
			"created_at":    time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// EnsureArtifactIdentifiers backfills stable runtime artifact identifiers when
// callers only provide the artifact body. It also backfills task_id so later
// list/open/persist flows all resolve against the same formal artifact shape.
func EnsureArtifactIdentifiers(taskID string, artifacts []map[string]any) []map[string]any {
	if len(artifacts) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		if len(artifact) == 0 {
			continue
		}
		cloned := cloneArtifactMap(artifact)
		resolvedTaskID := firstNonEmptyString(taskID, artifactStringValue(cloned, "task_id"))
		if resolvedTaskID != "" {
			cloned["task_id"] = resolvedTaskID
		}
		if artifactStringValue(cloned, "artifact_id") == "" {
			cloned["artifact_id"] = runtimeArtifactID(resolvedTaskID, cloned)
		}
		result = append(result, cloned)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// BuildStorageWritePlan translates a delivery_result into the workspace write
// plan that runengine persists for later execution.
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

// BuildArtifactPersistPlans converts runtime artifacts into metadata persistence
// plans.
func (s *Service) BuildArtifactPersistPlans(taskID string, artifacts []map[string]any) []map[string]any {
	artifacts = EnsureArtifactIdentifiers(taskID, artifacts)
	if len(artifacts) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		deliveryPayloadJSON := "{}"
		if payloadJSON, err := json.Marshal(mapValueOrEmpty(artifact, "delivery_payload")); err == nil {
			deliveryPayloadJSON = string(payloadJSON)
		}
		result = append(result, map[string]any{
			"artifact_id":           artifact["artifact_id"],
			"task_id":               taskID,
			"artifact_type":         artifact["artifact_type"],
			"path":                  artifact["path"],
			"mime_type":             artifact["mime_type"],
			"title":                 artifact["title"],
			"delivery_type":         artifact["delivery_type"],
			"delivery_payload_json": deliveryPayloadJSON,
			"created_at":            firstNonEmptyString(artifactStringValue(artifact, "created_at"), time.Now().UTC().Format(time.RFC3339)),
		})
	}

	return result
}

func mapValueOrEmpty(values map[string]any, key string) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	if nested, ok := values[key].(map[string]any); ok && nested != nil {
		return nested
	}
	return map[string]any{}
}

func artifactStringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func cloneArtifactMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		if nested, ok := value.(map[string]any); ok {
			result[key] = cloneArtifactMap(nested)
			continue
		}
		result[key] = value
	}
	return result
}

func runtimeArtifactID(taskID string, artifact map[string]any) string {
	resolvedTaskID := firstNonEmptyString(taskID, artifactStringValue(artifact, "task_id"), "runtime")
	hasher := fnv.New32a()
	artifactPath := strings.TrimSpace(strings.ReplaceAll(artifactStringValue(artifact, "path"), "\\", "/"))
	if artifactPath != "" {
		artifactPath = path.Clean(artifactPath)
	}
	_, _ = hasher.Write([]byte(resolvedTaskID))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(artifactStringValue(artifact, "artifact_type")))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(artifactStringValue(artifact, "title")))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(artifactPath))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(artifactStringValue(artifact, "mime_type")))
	return fmt.Sprintf("art_%s_%08x", resolvedTaskID, hasher.Sum32())
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// BuildApprovalExecutionPlan builds the minimum delivery plan needed when a
// task resumes after approval.
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

// slugify converts a delivery title into a safe workspace filename segment.
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
