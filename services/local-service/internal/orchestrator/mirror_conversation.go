package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

const (
	mirrorConversationStatusSubmitted = "submitted"
	mirrorConversationStatusResponded = "responded"
	mirrorConversationStatusFailed    = "failed"
)

// MirrorConversationList handles agent.mirror.conversation.list and exposes the
// backend-owned history view that mirror pages can read without depending on a
// renderer-local cache.
func (s *Service) MirrorConversationList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := strings.TrimSpace(stringValue(params, "task_id", ""))
	source := strings.TrimSpace(stringValue(params, "source", ""))
	status := strings.TrimSpace(stringValue(params, "status", ""))
	if s.storage == nil || s.storage.MirrorConversationStore() == nil {
		return map[string]any{"items": []map[string]any{}, "page": pageMap(limit, offset, 0)}, nil
	}
	records, total, err := s.storage.MirrorConversationStore().ListMirrorConversations(context.Background(), taskID, source, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, mirrorConversationRecordMap(record))
	}
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

func (s *Service) beginMirrorConversationRecord(params map[string]any, snapshot contextsvc.TaskContextSnapshot) *storage.MirrorConversationRecord {
	if s == nil || s.storage == nil || s.storage.MirrorConversationStore() == nil {
		return nil
	}
	rawTraceID := strings.TrimSpace(requestTraceID(params))
	if rawTraceID == "" && strings.TrimSpace(snapshot.Text) == "" {
		return nil
	}
	traceID := ensureMirrorConversationTraceID(rawTraceID)
	now := time.Now().Format(dateTimeLayout)
	record := &storage.MirrorConversationRecord{
		RecordID:  mirrorConversationRecordID(traceID),
		TraceID:   traceID,
		CreatedAt: now,
		UpdatedAt: now,
		Source:    stringValue(params, "source", ""),
		Trigger:   stringValue(params, "trigger", ""),
		InputMode: snapshot.InputMode,
		SessionID: strings.TrimSpace(stringValue(params, "session_id", "")),
		UserText:  snapshot.Text,
		Status:    mirrorConversationStatusSubmitted,
	}
	s.persistMirrorConversationRecord(*record)
	return record
}

// finishMirrorConversationRecord keeps mirror history persistence best-effort
// because input submission may already have created or resumed real tasks by the
// time the auxiliary mirror read model is updated.
func (s *Service) finishMirrorConversationRecord(record *storage.MirrorConversationRecord, response map[string]any, submitErr error) {
	if s == nil || record == nil {
		return
	}
	finalRecord := *record
	finalRecord.UpdatedAt = time.Now().Format(dateTimeLayout)
	if sessionID := responseTaskSessionID(response); sessionID != "" {
		finalRecord.SessionID = sessionID
	}
	if taskID := responseTaskID(response); taskID != "" {
		finalRecord.TaskID = taskID
	}
	if submitErr != nil {
		finalRecord.Status = mirrorConversationStatusFailed
		finalRecord.ErrorMessage = submitErr.Error()
		s.persistMirrorConversationRecord(finalRecord)
		return
	}
	bubble := responseBubbleMessage(response)
	if responseTaskStatus(response) == "failed" {
		finalRecord.Status = mirrorConversationStatusFailed
		finalRecord.AgentText = stringValue(bubble, "text", "")
		finalRecord.AgentBubbleType = stringValue(bubble, "type", "")
		finalRecord.ErrorMessage = firstNonEmptyString(finalRecord.AgentText, "task execution failed")
		if createdAt := strings.TrimSpace(stringValue(bubble, "created_at", "")); createdAt != "" {
			finalRecord.UpdatedAt = createdAt
		}
		s.persistMirrorConversationRecord(finalRecord)
		return
	}
	if len(bubble) == 0 {
		finalRecord.Status = mirrorConversationStatusSubmitted
		s.persistMirrorConversationRecord(finalRecord)
		return
	}
	finalRecord.Status = mirrorConversationStatusResponded
	finalRecord.AgentText = stringValue(bubble, "text", "")
	finalRecord.AgentBubbleType = stringValue(bubble, "type", "")
	if createdAt := strings.TrimSpace(stringValue(bubble, "created_at", "")); createdAt != "" {
		finalRecord.UpdatedAt = createdAt
	}
	finalRecord.ErrorMessage = ""
	s.persistMirrorConversationRecord(finalRecord)
}

func (s *Service) persistMirrorConversationRecord(record storage.MirrorConversationRecord) {
	if s == nil || s.storage == nil || s.storage.MirrorConversationStore() == nil {
		return
	}
	_ = s.storage.MirrorConversationStore().SaveMirrorConversation(context.Background(), record)
}

func mirrorConversationRecordID(traceID string) string {
	trimmed := strings.TrimSpace(traceID)
	if trimmed == "" {
		return fmt.Sprintf("mirror_conversation_%d", time.Now().UnixNano())
	}
	// Trace IDs are request-correlation metadata, not a business uniqueness key,
	// so persisted mirror history must append even when callers reuse a trace.
	return fmt.Sprintf("mirror_conversation_%s_%d", trimmed, time.Now().UnixNano())
}

// ensureMirrorConversationTraceID keeps the persisted mirror history contract
// valid even when callers omit request_meta.trace_id at the loose map boundary.
func ensureMirrorConversationTraceID(traceID string) string {
	trimmed := strings.TrimSpace(traceID)
	if trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("trace_mirror_%d", time.Now().UnixNano())
}

func mirrorConversationRecordMap(record storage.MirrorConversationRecord) map[string]any {
	return map[string]any{
		"record_id":         record.RecordID,
		"trace_id":          record.TraceID,
		"created_at":        record.CreatedAt,
		"updated_at":        record.UpdatedAt,
		"source":            record.Source,
		"trigger":           record.Trigger,
		"input_mode":        record.InputMode,
		"session_id":        nullableStringValue(record.SessionID),
		"task_id":           nullableStringValue(record.TaskID),
		"user_text":         record.UserText,
		"agent_text":        nullableStringValue(record.AgentText),
		"agent_bubble_type": nullableStringValue(record.AgentBubbleType),
		"status":            record.Status,
		"error_message":     nullableStringValue(record.ErrorMessage),
	}
}

func responseTaskID(response map[string]any) string {
	return stringValue(mapValue(response, "task"), "task_id", "")
}

func responseTaskSessionID(response map[string]any) string {
	task := mapValue(response, "task")
	if raw, ok := task["session_id"]; ok {
		if value, ok := raw.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func responseTaskStatus(response map[string]any) string {
	return strings.TrimSpace(stringValue(mapValue(response, "task"), "status", ""))
}

func responseBubbleMessage(response map[string]any) map[string]any {
	raw, ok := response["bubble_message"]
	if !ok || raw == nil {
		return nil
	}
	bubble, _ := raw.(map[string]any)
	return bubble
}

func nullableStringValue(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
