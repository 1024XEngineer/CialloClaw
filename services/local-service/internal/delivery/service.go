package delivery

import (
	"fmt"
	"regexp"
	"strings"
)

type StorageWritePlan struct {
	TaskID       string
	TargetPath   string
	MimeType     string
	DeliveryType string
	Source       string
}

type ArtifactPersistPlan struct {
	ArtifactID   string
	TaskID       string
	ArtifactType string
	Path         string
	MimeType     string
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) DefaultResultType() string {
	return "workspace_document"
}

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
