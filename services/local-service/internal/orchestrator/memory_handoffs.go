package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/languagepolicy"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

func (s *Service) refreshMirrorReferences(taskID string) {
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return
	}
	_, _ = s.runEngine.SetMirrorReferences(taskID, buildTaskMirrorReferences(task))
}

func (s *Service) syncTaskReadMirrorReferences(taskID string, references []map[string]any, err error) {
	if err == nil {
		_, _ = s.runEngine.SetMirrorReferences(taskID, cloneMapSlice(references))
		return
	}
	if errors.Is(err, memory.ErrStoreNotConfigured) {
		s.refreshMirrorReferences(taskID)
	}
}

func (s *Service) syncTaskWriteMirrorReferences(taskID string, references []map[string]any, err error) {
	if err == nil {
		_, _ = s.runEngine.SetMirrorReferences(taskID, mergeMirrorReferences(currentTaskMirrorReferences(s.runEngine, taskID), references))
		return
	}
	if errors.Is(err, memory.ErrStoreNotConfigured) {
		s.refreshMirrorReferences(taskID)
	}
}

func buildTaskMirrorReferences(task runengine.TaskRecord) []map[string]any {
	references := make([]map[string]any, 0, len(task.MemoryReadPlans)+len(task.MemoryWritePlans))
	for index, plan := range task.MemoryReadPlans {
		query := firstNonEmptyString(
			stringValue(plan, "query", ""),
			stringValue(plan, "selection_text", ""),
		)
		query = firstNonEmptyString(query, stringValue(plan, "input_text", ""))
		query = firstNonEmptyString(query, task.Title)
		references = append(references, map[string]any{
			"memory_id": fmt.Sprintf("mem_read_%s_%d", task.TaskID, index+1),
			"reason":    firstNonEmptyString(stringValue(plan, "reason", ""), "任务开始前准备记忆召回"),
			"summary":   fmt.Sprintf("召回查询：%s", truncateText(query, 48)),
		})
	}
	for index, plan := range task.MemoryWritePlans {
		summary := firstNonEmptyString(stringValue(plan, "summary", ""), task.Title)
		references = append(references, map[string]any{
			"memory_id": fmt.Sprintf("mem_write_%s_%d", task.TaskID, index+1),
			"reason":    firstNonEmptyString(stringValue(plan, "reason", ""), "任务完成后准备写入记忆摘要"),
			"summary":   truncateText(summary, 64),
		})
	}
	return references
}

func currentTaskMirrorReferences(engine *runengine.Engine, taskID string) []map[string]any {
	if engine == nil {
		return nil
	}
	task, ok := engine.GetTask(taskID)
	if !ok {
		return nil
	}
	return cloneMapSlice(task.MirrorReferences)
}

func mergeMirrorReferences(referenceGroups ...[]map[string]any) []map[string]any {
	merged := make([]map[string]any, 0)
	seen := make(map[string]struct{})
	for _, references := range referenceGroups {
		for _, reference := range references {
			memoryID := stringValue(reference, "memory_id", "")
			if memoryID == "" {
				continue
			}
			if _, ok := seen[memoryID]; ok {
				continue
			}
			seen[memoryID] = struct{}{}
			merged = append(merged, cloneMap(reference))
		}
	}
	return merged
}

func (s *Service) materializeMemoryReadReferences(taskID, runID string, snapshot taskcontext.TaskContextSnapshot) ([]map[string]any, []memory.RetrievalHit, error) {
	if s.memory == nil {
		return nil, nil, memory.ErrStoreNotConfigured
	}
	hits, err := s.memory.Search(context.Background(), memory.RetrievalQuery{
		TaskID: taskID,
		RunID:  runID,
		Query:  memoryQueryFromSnapshot(snapshot),
		Limit:  memory.DefaultSearchLimit,
	})
	if err != nil {
		return nil, nil, err
	}
	persistedHits := cloneRetrievalHitsForTask(taskID, runID, hits)
	if err := s.memory.WriteRetrievalHits(context.Background(), persistedHits); err != nil {
		return nil, nil, err
	}
	return mirrorReferencesFromRetrievalHits(persistedHits), persistedHits, nil
}

func (s *Service) materializeMemoryWriteReferences(taskID, runID string, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, deliveryResult map[string]any) ([]map[string]any, error) {
	if s.memory == nil {
		return nil, memory.ErrStoreNotConfigured
	}
	summary := memory.MemorySummary{
		MemorySummaryID: fmt.Sprintf("memsum_%s_%s", taskID, runID),
		TaskID:          taskID,
		RunID:           runID,
		Summary:         buildMemorySummary(snapshot, taskIntent, deliveryResult),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.memory.WriteSummary(context.Background(), summary); err != nil {
		return nil, err
	}
	return []map[string]any{mirrorReferenceFromSummary(summary)}, nil
}

func mirrorReferencesFromRetrievalHits(hits []memory.RetrievalHit) []map[string]any {
	if len(hits) == 0 {
		return nil
	}
	references := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		reason := "当前任务命中了历史记忆"
		if strings.TrimSpace(hit.Source) != "" {
			reason = fmt.Sprintf("当前任务命中了来源为 %s 的历史记忆", hit.Source)
		}
		references = append(references, map[string]any{
			"memory_id": hit.MemoryID,
			"reason":    reason,
			"summary":   truncateText(hit.Summary, 64),
		})
	}
	return references
}

func cloneRetrievalHitsForTask(taskID, runID string, hits []memory.RetrievalHit) []memory.RetrievalHit {
	if len(hits) == 0 {
		return nil
	}
	cloned := make([]memory.RetrievalHit, 0, len(hits))
	for _, hit := range hits {
		hit.TaskID = taskID
		hit.RunID = runID
		hit.RetrievalHitID = ""
		cloned = append(cloned, hit)
	}
	return cloned
}

func mirrorReferenceFromSummary(summary memory.MemorySummary) map[string]any {
	return map[string]any{
		"memory_id": summary.MemorySummaryID,
		"reason":    "任务完成后写入真实记忆摘要",
		"summary":   truncateText(summary.Summary, 64),
	}
}

// attachMemoryReadPlans registers the retrieval plans attached at task start or
// confirmation time. Read plans are persisted before execution so later mirror,
// debug, or storage-backed views can explain what memory lookup the task was
// supposed to perform even if execution changes or the process restarts. When
// clarification already materialized retrieval hits for the same task, reuse the
// same evidence so the confirmation bubble and execution stay aligned.
func (s *Service) attachMemoryReadPlans(taskID, runID string, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any) {
	if hits := currentTaskReadPlanHits(s.runEngine, taskID); len(hits) > 0 {
		reusedHits := cloneRetrievalHitsForTask(taskID, runID, hits)
		_, _ = s.runEngine.SetMemoryPlans(taskID, buildMemoryReadPlans(s.memory, taskID, runID, snapshot, taskIntent, reusedHits), nil)
		s.syncTaskReadMirrorReferences(taskID, mirrorReferencesFromRetrievalHits(reusedHits), nil)
		return
	}

	readPlans := buildMemoryReadPlans(s.memory, taskID, runID, snapshot, taskIntent, nil)
	_, _ = s.runEngine.SetMemoryPlans(taskID, readPlans, nil)
	references, hits, err := s.materializeMemoryReadReferences(taskID, runID, snapshot)
	if err == nil {
		_, _ = s.runEngine.SetMemoryPlans(taskID, buildMemoryReadPlans(s.memory, taskID, runID, snapshot, taskIntent, hits), nil)
	}
	s.syncTaskReadMirrorReferences(taskID, references, err)
}

// previewMemoryContext performs the same storage-backed retrieval as execution
// planning, but it only returns the matched summaries so clarification bubbles
// can acknowledge recent context without changing task ownership decisions.
func (s *Service) previewMemoryContext(taskID, runID string, snapshot taskcontext.TaskContextSnapshot) []memory.RetrievalHit {
	if s == nil || s.memory == nil {
		return nil
	}

	hits, err := s.memory.Search(context.Background(), memory.RetrievalQuery{
		TaskID: taskID,
		RunID:  runID,
		Query:  memoryQueryFromSnapshot(snapshot),
		Limit:  memory.DefaultSearchLimit,
	})
	if err != nil {
		return nil
	}

	return hits
}

// clarificationPreviewHits prefers the retrieval_context already materialized on
// the task so clarification bubbles and later execution reuse the same memory
// evidence even after storage round-trips or follow-up confirmation RPCs.
func (s *Service) clarificationPreviewHits(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot) []memory.RetrievalHit {
	if s == nil {
		return nil
	}

	currentTask := task
	if s.runEngine != nil && strings.TrimSpace(task.TaskID) != "" {
		if persistedTask, ok := s.runEngine.GetTask(task.TaskID); ok {
			currentTask = persistedTask
		}
	}

	if hits := taskReadPlanRetrievalHits(currentTask); len(hits) > 0 {
		return hits
	}

	return s.previewMemoryContext(currentTask.TaskID, currentTask.RunID, snapshot)
}

func currentTaskReadPlanHits(engine *runengine.Engine, taskID string) []memory.RetrievalHit {
	if engine == nil || strings.TrimSpace(taskID) == "" {
		return nil
	}
	task, ok := engine.GetTask(taskID)
	if !ok {
		return nil
	}
	return taskReadPlanRetrievalHits(task)
}

func taskReadPlanRetrievalHits(task runengine.TaskRecord) []memory.RetrievalHit {
	if len(task.MemoryReadPlans) == 0 {
		return nil
	}

	hits := make([]memory.RetrievalHit, 0, len(task.MemoryReadPlans))
	for _, plan := range task.MemoryReadPlans {
		for _, item := range readPlanRetrievalContextItems(plan) {
			summary := strings.TrimSpace(stringValue(item, "summary", ""))
			if summary == "" {
				continue
			}
			hits = append(hits, memory.RetrievalHit{
				TaskID:   task.TaskID,
				RunID:    task.RunID,
				MemoryID: strings.TrimSpace(stringValue(item, "memory_id", "")),
				Source:   strings.TrimSpace(stringValue(item, "source", "")),
				Summary:  summary,
				Score:    floatValueFromAny(item["score"]),
			})
		}
	}
	if len(hits) == 0 {
		return nil
	}
	return hits
}

func readPlanRetrievalContextItems(plan map[string]any) []map[string]any {
	rawValue, ok := plan["retrieval_context"]
	if !ok {
		return nil
	}
	switch value := rawValue.(type) {
	case []map[string]any:
		return cloneMapSlice(value)
	case []any:
		items := make([]map[string]any, 0, len(value))
		for _, entry := range value {
			item, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			items = append(items, cloneMap(item))
		}
		if len(items) == 0 {
			return nil
		}
		return items
	default:
		return nil
	}
}

func clarificationBubbleTextForLanguage(suggestionIntent map[string]any, _ []memory.RetrievalHit, replyLanguage string) string {
	return clarificationBaseTextForLanguage(suggestionIntent, replyLanguage)
}

func clarificationBaseTextForLanguage(suggestionIntent map[string]any, replyLanguage string) string {
	if replyLanguage == languagepolicy.ReplyLanguageEnglish {
		return clarificationFollowUpPrompt(suggestionIntent, true)
	}
	return clarificationFollowUpPrompt(suggestionIntent, false)
}

func initialClarificationPromptForLanguage(snapshot taskcontext.TaskContextSnapshot, startFlow bool, replyLanguage string) string {
	if strings.TrimSpace(replyLanguage) == "" {
		preferredInput := clarificationLanguageEvidence(snapshot)
		replyLanguage = languagepolicy.PreferredReplyLanguage(preferredInput)
	}
	if replyLanguage == languagepolicy.ReplyLanguageEnglish {
		if startFlow {
			return "I am not sure how you want me to handle this yet. Please confirm the goal first."
		}
		return "I am not sure how you want me to handle this content yet. Please confirm the goal first."
	}
	if startFlow {
		return "我还不确定你想如何处理当前对象，请先确认。"
	}
	return "我还不确定你想如何处理这段内容，请确认目标。"
}

func clarificationFollowUpPrompt(taskIntent map[string]any, english bool) string {
	switch stringValue(taskIntent, "name", "") {
	case "translate":
		if english {
			return "do you want me to translate it?"
		}
		return "你是想让我翻译它吗？"
	case "rewrite":
		if english {
			return "do you want me to rewrite it?"
		}
		return "你是想让我改写它吗？"
	case "explain":
		if english {
			return "do you want me to explain it?"
		}
		return "你是想让我解释它吗？"
	case "summarize":
		if english {
			return "do you want me to summarize it?"
		}
		return "你是想让我总结它吗？"
	default:
		if english {
			return "what would you like me to do next?"
		}
		return "你现在希望我具体怎么处理？"
	}
}

func buildMemoryReadPlans(memoryService *memory.Service, taskID, runID string, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, hits []memory.RetrievalHit) []map[string]any {
	readPlan := map[string]any{
		"kind":           "retrieval",
		"task_id":        taskID,
		"run_id":         runID,
		"query":          memoryQueryFromSnapshot(snapshot),
		"reason":         "任务开始前准备记忆召回",
		"intent_name":    stringValue(taskIntent, "name", "summarize"),
		"selection_text": snapshot.SelectionText,
		"input_text":     snapshot.Text,
		"source_type":    snapshot.Trigger,
	}
	if memoryService != nil {
		readPlan["backend"] = memoryService.RetrievalBackend()
	}
	if contextItems := retrievalContextItems(hits); len(contextItems) > 0 {
		readPlan["retrieval_context"] = contextItems
	}

	return []map[string]any{readPlan}
}

func retrievalContextItems(hits []memory.RetrievalHit) []map[string]any {
	if len(hits) == 0 {
		return nil
	}

	items := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		summary := strings.TrimSpace(hit.Summary)
		if summary == "" {
			continue
		}
		items = append(items, map[string]any{
			"memory_id": hit.MemoryID,
			"source":    hit.Source,
			"summary":   summary,
			"score":     hit.Score,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

// attachPostDeliveryHandoffs registers memory-write and delivery persistence
// handoffs after a task finishes. Keeping these side effects in one post-
// delivery step prevents runtime execution from mixing formal delivery with
// memory persistence details while still leaving a durable handoff trail.
func (s *Service) attachPostDeliveryHandoffs(taskID, runID string, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, deliveryResult map[string]any, artifacts []map[string]any) {
	writePlans := []map[string]any{
		{
			"kind":        "summary_write",
			"backend":     s.memory.RetrievalBackend(),
			"task_id":     taskID,
			"run_id":      runID,
			"summary":     buildMemorySummary(snapshot, taskIntent, deliveryResult),
			"reason":      "任务完成后准备写入阶段摘要",
			"source_type": snapshot.Trigger,
		},
	}
	_, _ = s.runEngine.SetMemoryPlans(taskID, nil, writePlans)
	references, err := s.materializeMemoryWriteReferences(taskID, runID, snapshot, taskIntent, deliveryResult)
	s.syncTaskWriteMirrorReferences(taskID, references, err)

	storageWritePlan := s.delivery.BuildStorageWritePlan(taskID, deliveryResult)
	artifacts = delivery.EnsureArtifactIdentifiers(taskID, attachDeliveryResultToArtifacts(deliveryResult, artifacts))
	artifactPlans := s.delivery.BuildArtifactPersistPlans(taskID, artifacts)
	_, _ = s.runEngine.SetDeliveryPlans(taskID, storageWritePlan, artifactPlans)
	s.persistArtifacts(taskID, artifactPlans)
}

// memoryQueryFromSnapshot selects the most representative retrieval query from
// the current context snapshot. The fallback order intentionally prefers direct
// user focus, then file context, then broader perception signals so memory
// lookup stays anchored to what most likely triggered the task.
func memoryQueryFromSnapshot(snapshot taskcontext.TaskContextSnapshot) string {
	for _, value := range []string{snapshot.SelectionText, snapshot.Text, snapshot.ErrorText} {
		if value != "" {
			return truncateText(value, 64)
		}
	}

	if len(snapshot.Files) > 0 {
		return snapshot.Files[0]
	}

	for _, value := range []string{snapshot.VisibleText, snapshot.ScreenSummary, snapshot.PageTitle, snapshot.WindowTitle, snapshot.ClipboardText} {
		if value != "" {
			return truncateText(value, 64)
		}
	}

	return "task_context"
}

// buildMemorySummary creates the short post-task memory summary written after
// delivery completes. It keeps the output compact on purpose because this text
// is later used as durable memory material rather than a full-fidelity trace.
func buildMemorySummary(snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, deliveryResult map[string]any) string {
	intentName := stringValue(taskIntent, "name", "summarize")
	title := stringValue(deliveryResult, "title", "任务结果")
	query := memoryQueryFromSnapshot(snapshot)
	preview := stringValue(deliveryResult, "preview_text", "")
	if preview == "" {
		preview = title
	}
	perceptionSummary := []string{}
	if snapshot.CopyCount > 0 || strings.EqualFold(snapshot.LastAction, "copy") {
		perceptionSummary = append(perceptionSummary, "copy")
	}
	if snapshot.DwellMillis > 0 {
		perceptionSummary = append(perceptionSummary, fmt.Sprintf("dwell=%dms", snapshot.DwellMillis))
	}
	if snapshot.WindowSwitches > 0 || snapshot.PageSwitches > 0 {
		perceptionSummary = append(perceptionSummary, fmt.Sprintf("switch=%d/%d", snapshot.WindowSwitches, snapshot.PageSwitches))
	}
	if snapshot.PageTitle != "" {
		perceptionSummary = append(perceptionSummary, "page="+truncateText(snapshot.PageTitle, 24))
	}
	if len(perceptionSummary) == 0 {
		return fmt.Sprintf("任务完成，意图=%s，输入=%s，交付=%s，结果摘要=%s", intentName, truncateText(query, 48), title, truncateText(preview, resultPreviewMaxLength))
	}
	return fmt.Sprintf("任务完成，意图=%s，输入=%s，感知=%s，交付=%s，结果摘要=%s", intentName, truncateText(query, 48), strings.Join(perceptionSummary, ", "), title, truncateText(preview, resultPreviewMaxLength))
}
