package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

// TaskArtifactList returns protocol-ready artifacts for one task. It merges
// runtime and persisted artifacts behind the orchestrator boundary so the RPC
// layer can expose one stable collection shape.
func (s *Service) TaskArtifactList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := stringValue(params, "task_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	items, total, err := s.listArtifactsPage(taskID, limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items": protocolArtifactList(items),
		"page":  pageMap(limit, offset, total),
	}, nil
}

// TaskArtifactOpen resolves one task artifact into an open action while keeping
// the formal Artifact payload available for audit and task-detail callers.
func (s *Service) TaskArtifactOpen(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	artifactID := stringValue(params, "artifact_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	if strings.TrimSpace(artifactID) == "" {
		return nil, errors.New("artifact_id is required")
	}
	artifact, err := s.findArtifactForTask(taskID, artifactID)
	if err != nil {
		return nil, err
	}
	openResult := buildDeliveryOpenResult(cloneMap(artifact), nil, taskID)
	openResult["artifact"] = protocolArtifactMap(artifact)
	return openResult, nil
}

// DeliveryOpen resolves the final open action from a delivery_result or its
// artifact. It does not bypass the formal delivery/artifact boundary.
func (s *Service) DeliveryOpen(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	artifactID := stringValue(params, "artifact_id", "")
	if strings.TrimSpace(artifactID) != "" {
		artifact, err := s.findArtifactForTask(taskID, artifactID)
		if err != nil {
			return nil, err
		}
		result := buildDeliveryOpenResult(cloneMap(artifact), nil, taskID)
		result["artifact"] = protocolArtifactMap(artifact)
		return result, nil
	}
	task, ok := s.taskDetailFromStorage(taskID)
	if runtimeTask, runtimeOK := s.runEngine.TaskDetail(taskID); runtimeOK {
		if ok {
			task = mergeRuntimeTaskDetail(task, runtimeTask)
		} else {
			task = runtimeTask
			ok = true
		}
	}
	if !ok {
		return nil, ErrTaskNotFound
	}
	deliveryResult := s.latestAttemptDeliveryResultFromStorage(task)
	if len(deliveryResult) == 0 {
		deliveryResult = cloneMap(task.DeliveryResult)
	}
	return buildDeliveryOpenResult(nil, deliveryResult, taskID), nil
}

func inferArtifactDeliveryType(artifact map[string]any) string {
	if deliveryType := stringValue(artifact, "delivery_type", ""); deliveryType != "" {
		return deliveryType
	}
	if path := stringValue(artifact, "path", ""); path != "" {
		return "open_file"
	}
	return "task_detail"
}

// protocolTaskStepList guarantees that task detail timeline stays an array.
func protocolTaskStepList(steps []map[string]any) []map[string]any {
	if len(steps) == 0 {
		return []map[string]any{}
	}
	return cloneMapSlice(steps)
}

// protocolArtifactList trims artifact items to the declared protocol fields and
// keeps the collection non-null for RPC consumers.
func protocolArtifactList(artifacts []map[string]any) []map[string]any {
	if len(artifacts) == 0 {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		normalized := protocolArtifactMap(artifact)
		if normalized == nil {
			continue
		}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return []map[string]any{}
	}
	return result
}

func protocolCitationList(citations []map[string]any) []map[string]any {
	if len(citations) == 0 {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(citations))
	for _, citation := range citations {
		result = append(result, protocolCitationMap(citation))
	}
	return result
}

func protocolCitationMap(citation map[string]any) map[string]any {
	result := map[string]any{
		"citation_id": stringValue(citation, "citation_id", ""),
		"task_id":     stringValue(citation, "task_id", ""),
		"run_id":      stringValue(citation, "run_id", ""),
		"source_type": stringValue(citation, "source_type", "context"),
		"source_ref":  stringValue(citation, "source_ref", ""),
		"label":       stringValue(citation, "label", ""),
	}
	if artifactID := strings.TrimSpace(stringValue(citation, "artifact_id", "")); artifactID != "" {
		result["artifact_id"] = artifactID
	}
	if artifactType := strings.TrimSpace(stringValue(citation, "artifact_type", "")); artifactType != "" {
		result["artifact_type"] = artifactType
	}
	if evidenceRole := strings.TrimSpace(stringValue(citation, "evidence_role", "")); evidenceRole != "" {
		result["evidence_role"] = evidenceRole
	}
	if excerptText := strings.TrimSpace(stringValue(citation, "excerpt_text", "")); excerptText != "" {
		result["excerpt_text"] = excerptText
	}
	if screenSessionID := strings.TrimSpace(stringValue(citation, "screen_session_id", "")); screenSessionID != "" {
		result["screen_session_id"] = screenSessionID
	}
	return result
}

// protocolArtifactMap trims one artifact to the formal Artifact contract.
func protocolArtifactMap(artifact map[string]any) map[string]any {
	if len(artifact) == 0 {
		return nil
	}
	return map[string]any{
		"artifact_id":   stringValue(artifact, "artifact_id", ""),
		"task_id":       stringValue(artifact, "task_id", ""),
		"artifact_type": stringValue(artifact, "artifact_type", ""),
		"title":         stringValue(artifact, "title", ""),
		"path":          stringValue(artifact, "path", ""),
		"mime_type":     stringValue(artifact, "mime_type", ""),
	}
}

// protocolMirrorReferenceList trims mirror references to the declared protocol
// fields and keeps the collection non-null for RPC consumers.
func protocolMirrorReferenceList(references []map[string]any) []map[string]any {
	if len(references) == 0 {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(references))
	for _, reference := range references {
		if len(reference) == 0 {
			continue
		}
		result = append(result, map[string]any{
			"memory_id": stringValue(reference, "memory_id", ""),
			"reason":    stringValue(reference, "reason", ""),
			"summary":   stringValue(reference, "summary", ""),
		})
	}
	if len(result) == 0 {
		return []map[string]any{}
	}
	return result
}

func buildDeliveryOpenResult(artifact map[string]any, deliveryResult map[string]any, taskID string) map[string]any {
	resolvedDelivery := normalizeDeliveryOpenResult(artifact, deliveryResult, taskID)
	return map[string]any{
		"delivery_result":  resolvedDelivery,
		"open_action":      stringValue(resolvedDelivery, "type", "task_detail"),
		"resolved_payload": cloneMap(mapValue(resolvedDelivery, "payload")),
	}
}

func normalizeDeliveryOpenResult(artifact map[string]any, deliveryResult map[string]any, taskID string) map[string]any {
	if len(deliveryResult) == 0 {
		payload := cloneMap(mapValue(artifact, "delivery_payload"))
		if payload == nil {
			payload = map[string]any{}
		}
		pathValue := firstNonEmptyString(stringValue(artifact, "path", ""), stringValue(payload, "path", ""))
		if pathValue != "" {
			payload["path"] = pathValue
		}
		if payload["task_id"] == nil {
			payload["task_id"] = taskID
		}
		return map[string]any{
			"type":         firstNonEmptyString(stringValue(artifact, "delivery_type", ""), inferArtifactDeliveryType(artifact)),
			"title":        stringValue(artifact, "title", ""),
			"payload":      normalizeFormalDeliveryPayload(payload, taskID),
			"preview_text": stringValue(artifact, "title", ""),
		}
	}
	resolved := cloneMap(deliveryResult)
	payload := cloneMap(mapValue(resolved, "payload"))
	if payload == nil {
		payload = map[string]any{}
	}
	resolved["payload"] = normalizeFormalDeliveryPayload(payload, taskID)
	if stringValue(resolved, "type", "") == "" {
		resolved["type"] = "task_detail"
	}
	if stringValue(resolved, "title", "") == "" {
		resolved["title"] = "任务交付结果"
	}
	if stringValue(resolved, "preview_text", "") == "" {
		resolved["preview_text"] = stringValue(resolved, "title", "")
	}
	return resolved
}

// normalizeFormalDeliveryPayload keeps formal delivery payload keys stable for
// protocol consumers even when historical storage records omitted sparse fields.
func normalizeFormalDeliveryPayload(payload map[string]any, taskID string) map[string]any {
	normalized := cloneMap(payload)
	if normalized == nil {
		normalized = map[string]any{}
	}
	if normalized["path"] == nil {
		normalized["path"] = nil
	}
	if normalized["url"] == nil {
		normalized["url"] = nil
	}
	if normalized["task_id"] == nil {
		if strings.TrimSpace(taskID) == "" {
			normalized["task_id"] = nil
		} else {
			normalized["task_id"] = taskID
		}
	}
	return normalized
}

// normalizeTaskDetailDeliveryResult keeps task detail aligned with the formal
// delivery contract without forcing the dashboard to infer missing payload fields.
func normalizeTaskDetailDeliveryResult(taskID string, deliveryResult map[string]any) map[string]any {
	if len(deliveryResult) == 0 {
		return nil
	}
	return normalizeDeliveryOpenResult(nil, cloneMap(deliveryResult), taskID)
}

// resultSpecFromIntent returns the default result title, preview text, and
// completion bubble text for an intent.
func resultSpecFromIntent(taskIntent map[string]any) (string, string, string) {
	switch stringValue(taskIntent, "name", "summarize") {
	case "agent_loop":
		return "处理结果", "结果已通过气泡返回", "结果已经生成，可直接查看。"
	case "rewrite":
		return "改写结果", "已为你写入文档并打开", "内容已经按要求改写完成，可直接查看。"
	case "translate":
		return "翻译结果", "结果已通过气泡返回", "翻译结果已经生成，可直接查看。"
	case "explain":
		return "解释结果", "结果已通过气泡返回", "这段内容的意思已经整理好了。"
	case "page_read":
		return "网页读取结果", "结果已通过气泡返回", "网页主要内容已经整理完成，可直接查看。"
	case "page_search":
		return "网页搜索结果", "结果已通过气泡返回", "网页搜索结果已经返回，可直接查看。"
	case "write_file":
		return "文件写入结果", "已为你写入文档并打开", "文件已经生成，可直接查看。"
	default:
		return "处理结果", "已为你写入文档并打开", "结果已经生成，可直接查看。"
	}
}

// deliveryTypeFromIntent returns the default delivery type for an intent.
func deliveryTypeFromIntent(taskIntent map[string]any) string {
	switch stringValue(taskIntent, "name", "summarize") {
	case "agent_loop", "translate", "explain", "page_read", "page_search":
		return "bubble"
	default:
		return "workspace_document"
	}
}

func attachDeliveryResultToArtifacts(deliveryResult map[string]any, artifacts []map[string]any) []map[string]any {
	if len(artifacts) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		cloned := cloneMap(artifact)
		if cloned == nil {
			continue
		}
		if stringValue(cloned, "delivery_type", "") == "" {
			cloned["delivery_type"] = stringValue(deliveryResult, "type", "")
		}
		if len(mapValue(cloned, "delivery_payload")) == 0 {
			cloned["delivery_payload"] = cloneMap(mapValue(deliveryResult, "payload"))
		}
		if stringValue(cloned, "created_at", "") == "" {
			cloned["created_at"] = time.Now().UTC().Format(time.RFC3339)
		}
		result = append(result, cloned)
	}
	return result
}

func (s *Service) persistArtifacts(taskID string, artifactPlans []map[string]any) {
	if s.storage == nil || s.storage.ArtifactStore() == nil || len(artifactPlans) == 0 {
		return
	}
	runID := ""
	if task, ok := s.runEngine.GetTask(taskID); ok {
		runID = task.RunID
	}
	records := make([]storage.ArtifactRecord, 0, len(artifactPlans))
	for _, plan := range artifactPlans {
		records = append(records, storage.ArtifactRecord{
			ArtifactID:          stringValue(plan, "artifact_id", ""),
			TaskID:              firstNonEmptyString(stringValue(plan, "task_id", ""), taskID),
			RunID:               firstNonEmptyString(stringValue(plan, "run_id", ""), runID),
			ArtifactType:        stringValue(plan, "artifact_type", ""),
			Title:               stringValue(plan, "title", ""),
			Path:                stringValue(plan, "path", ""),
			MimeType:            stringValue(plan, "mime_type", ""),
			DeliveryType:        stringValue(plan, "delivery_type", ""),
			DeliveryPayloadJSON: stringValue(plan, "delivery_payload_json", "{}"),
			CreatedAt:           firstNonEmptyString(stringValue(plan, "created_at", ""), time.Now().UTC().Format(time.RFC3339)),
		})
	}
	_ = s.storage.ArtifactStore().SaveArtifacts(context.Background(), records)
	if task, ok := s.runEngine.GetTask(taskID); ok {
		merged := mergeArtifactsWithStored(task.Artifacts, s.loadAttemptArtifactsFromStorage(task, 0, 0))
		_, _ = s.runEngine.SetPresentation(taskID, task.BubbleMessage, task.DeliveryResult, merged)
	}
}

func (s *Service) artifactsForTask(task runengine.TaskRecord, runtimeArtifacts []map[string]any) []map[string]any {
	return mergeArtifactsWithStored(delivery.EnsureArtifactIdentifiers(task.TaskID, runtimeArtifacts), s.loadAttemptArtifactsFromStorage(task, 0, 0))
}

func (s *Service) citationsForTask(task runengine.TaskRecord, runtimeCitations []map[string]any) []map[string]any {
	return mergeCitationsWithStored(s.loadAttemptTaskCitationsFromStorage(task), runtimeCitations)
}

func (s *Service) loadAttemptArtifactsFromStorage(task runengine.TaskRecord, limit, offset int) []map[string]any {
	if s.storage == nil || s.storage.ArtifactStore() == nil || strings.TrimSpace(task.TaskID) == "" {
		return nil
	}
	records, _, err := s.storage.ArtifactStore().ListArtifacts(context.Background(), task.TaskID, taskAttemptRunIDFilter(task), limit, offset)
	if err != nil {
		return nil
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, artifactMapFromStorage(record))
	}
	return items
}

func (s *Service) listArtifactsPage(taskID string, limit, offset int) ([]map[string]any, int, error) {
	task, taskFound := formalReadTask(taskID, s.runEngine, s.taskDetailFromStorage)
	runIDFilter := ""
	if taskFound {
		runIDFilter = taskAttemptRunIDFilter(task)
	}
	if s.storage != nil && s.storage.ArtifactStore() != nil {
		records, total, err := s.storage.ArtifactStore().ListArtifacts(context.Background(), taskID, runIDFilter, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		if total > 0 {
			items := make([]map[string]any, 0, len(records))
			for _, record := range records {
				items = append(items, artifactMapFromStorage(record))
			}
			return items, total, nil
		}
	}
	items := delivery.EnsureArtifactIdentifiers(taskID, currentTaskArtifacts(s.runEngine, taskID))
	if taskFound {
		items = s.artifactsForTask(task, task.Artifacts)
	}
	total := len(items)
	if offset >= total {
		return []map[string]any{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return cloneMapSlice(items[offset:end]), total, nil
}

func currentTaskArtifacts(engine *runengine.Engine, taskID string) []map[string]any {
	if engine == nil || strings.TrimSpace(taskID) == "" {
		return nil
	}
	task, ok := engine.GetTask(taskID)
	if !ok {
		return nil
	}
	return cloneMapSlice(task.Artifacts)
}

func (s *Service) findArtifactForTask(taskID, artifactID string) (map[string]any, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, ErrTaskNotFound
	}
	task, taskFound := formalReadTask(taskID, s.runEngine, s.taskDetailFromStorage)
	exists := taskFound
	runIDFilter := ""
	if taskFound {
		runIDFilter = taskAttemptRunIDFilter(task)
	}
	if s.storage != nil && s.storage.ArtifactStore() != nil {
		records, _, err := s.storage.ArtifactStore().ListArtifacts(context.Background(), taskID, runIDFilter, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		if len(records) > 0 {
			exists = true
		}
		for _, record := range records {
			if record.ArtifactID == artifactID {
				return artifactMapFromStorage(record), nil
			}
		}
	}
	if taskFound {
		for _, artifact := range delivery.EnsureArtifactIdentifiers(taskID, task.Artifacts) {
			if stringValue(artifact, "artifact_id", "") == artifactID {
				return cloneMap(artifact), nil
			}
		}
	}
	if !exists {
		return nil, ErrTaskNotFound
	}
	return nil, ErrArtifactNotFound
}

func mergeArtifactsWithStored(runtimeArtifacts, storedArtifacts []map[string]any) []map[string]any {
	if len(runtimeArtifacts) == 0 && len(storedArtifacts) == 0 {
		return nil
	}
	merged := make([]map[string]any, 0, len(runtimeArtifacts)+len(storedArtifacts))
	seen := make(map[string]struct{})
	for _, group := range [][]map[string]any{storedArtifacts, runtimeArtifacts} {
		for _, artifact := range group {
			artifactID := stringValue(artifact, "artifact_id", "")
			if artifactID == "" {
				continue
			}
			if _, ok := seen[artifactID]; ok {
				continue
			}
			seen[artifactID] = struct{}{}
			merged = append(merged, cloneMap(artifact))
		}
	}
	return merged
}

func mergeCitationsWithStored(storedCitations, runtimeCitations []map[string]any) []map[string]any {
	if len(storedCitations) == 0 && len(runtimeCitations) == 0 {
		return nil
	}
	merged := make([]map[string]any, 0, len(storedCitations)+len(runtimeCitations))
	seen := make(map[string]struct{})
	for _, group := range [][]map[string]any{storedCitations, runtimeCitations} {
		for index, citation := range group {
			mergeKey := citationMergeKey(citation, index)
			if _, ok := seen[mergeKey]; ok {
				continue
			}
			seen[mergeKey] = struct{}{}
			merged = append(merged, cloneMap(citation))
		}
	}
	return merged
}

func citationMergeKey(citation map[string]any, index int) string {
	if citationID := strings.TrimSpace(stringValue(citation, "citation_id", "")); citationID != "" {
		return citationID
	}
	parts := []string{
		strings.TrimSpace(stringValue(citation, "task_id", "")),
		strings.TrimSpace(stringValue(citation, "source_ref", "")),
		strings.TrimSpace(stringValue(citation, "artifact_id", "")),
		strings.TrimSpace(stringValue(citation, "label", "")),
		strings.TrimSpace(stringValue(citation, "excerpt_text", "")),
	}
	key := strings.Join(parts, "|")
	if strings.Trim(key, "|") != "" {
		return key
	}
	return fmt.Sprintf("citation_%d", index)
}

func artifactMapFromStorage(record storage.ArtifactRecord) map[string]any {
	payload := map[string]any{}
	if strings.TrimSpace(record.DeliveryPayloadJSON) != "" {
		_ = json.Unmarshal([]byte(record.DeliveryPayloadJSON), &payload)
	}
	return map[string]any{
		"artifact_id":      record.ArtifactID,
		"task_id":          record.TaskID,
		"artifact_type":    record.ArtifactType,
		"title":            record.Title,
		"path":             record.Path,
		"mime_type":        record.MimeType,
		"delivery_type":    record.DeliveryType,
		"delivery_payload": payload,
		"created_at":       record.CreatedAt,
	}
}

// applyResolvedDeliveryToPlan folds the resolved task-level delivery preference
// back into a pending execution plan.
func (s *Service) applyResolvedDeliveryToPlan(task runengine.TaskRecord, plan map[string]any, taskIntent map[string]any) map[string]any {
	if len(plan) == 0 {
		return nil
	}

	updatedPlan := cloneMap(plan)
	deliveryType := resolveTaskDeliveryType(task, taskIntent)
	updatedPlan["delivery_type"] = deliveryType
	updatedPlan["preview_text"] = previewTextForDeliveryType(deliveryType)
	return updatedPlan
}

// resolveTaskDeliveryType computes the effective delivery type for a task.
func resolveTaskDeliveryType(task runengine.TaskRecord, taskIntent map[string]any) string {
	return resolveDeliveryType(task.PreferredDelivery, task.FallbackDelivery, deliveryTypeFromIntent(taskIntent))
}

// resolveDeliveryType resolves the final delivery type in priority order:
// task preference, fallback, then default.
func resolveDeliveryType(preferred, fallback, defaultType string) string {
	if normalized := normalizeDeliveryType(preferred); normalized != "" {
		return normalized
	}
	if strings.TrimSpace(preferred) != "" {
		if normalized := normalizeDeliveryType(fallback); normalized != "" {
			return normalized
		}
	}
	if normalized := normalizeDeliveryType(defaultType); normalized != "" {
		return normalized
	}
	if normalized := normalizeDeliveryType(fallback); normalized != "" {
		return normalized
	}
	return "workspace_document"
}

func normalizeDeliveryType(deliveryType string) string {
	switch deliveryType {
	case "bubble", "workspace_document":
		return deliveryType
	default:
		return ""
	}
}

// previewTextForDeliveryType returns the preview copy for each delivery type.
func previewTextForDeliveryType(deliveryType string) string {
	if deliveryType == "bubble" {
		return "\u7ed3\u679c\u5df2\u901a\u8fc7\u6c14\u6ce1\u8fd4\u56de"
	}
	return "\u5df2\u4e3a\u4f60\u5199\u5165\u6587\u6863\u5e76\u6253\u5f00"
}
