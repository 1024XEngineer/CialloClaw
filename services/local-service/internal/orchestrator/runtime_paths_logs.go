package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type executionLogRecord struct {
	LogID     string
	TaskID    string
	RunID     string
	Source    string
	Kind      string
	Level     string
	Summary   string
	Detail    string
	Status    string
	ErrorCode string
	CreatedAt string
}

type errorEventLister interface {
	ListErrorEvents(ctx context.Context, taskID, runID string, limit, offset int) ([]storage.EventRecord, int, error)
}

type errorToolCallLister interface {
	ListErrorToolCalls(ctx context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error)
}

type errorAuditLister interface {
	ListErrorAuditRecords(ctx context.Context, taskID, runID string, limit, offset int) ([]audit.Record, int, error)
}

// SettingsRuntimePathsGet handles agent.settings.runtime_paths.get.
func (s *Service) SettingsRuntimePathsGet(params map[string]any) (map[string]any, error) {
	_ = params
	return map[string]any{
		"runtime_root":      currentRuntimeRoot(s.storage),
		"database_path":     currentDatabasePath(s.storage),
		"secret_store_path": currentSecretStorePath(s.storage),
		"workspace_path":    currentRuntimeWorkspaceRoot(s.executor),
	}, nil
}

// LogExecutionList handles agent.log.execution.list by aggregating the current
// formal event, tool-call, and audit read models into one control-panel query.
func (s *Service) LogExecutionList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := strings.TrimSpace(stringValue(params, "task_id", ""))
	source := strings.TrimSpace(stringValue(params, "source", ""))
	items, total, err := s.collectExecutionLogRecordsPage(taskID, source, limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items": executionLogRecordMaps(items),
		"page":  pageMap(limit, offset, total),
	}, nil
}

// LogErrorList handles agent.log.error.list by filtering the aggregated control-
// panel execution log stream down to failure-oriented records only.
func (s *Service) LogErrorList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := strings.TrimSpace(stringValue(params, "task_id", ""))
	source := strings.TrimSpace(stringValue(params, "source", ""))
	items, total, err := s.collectErrorExecutionLogRecordsPage(taskID, source, limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items": executionLogRecordMaps(items),
		"page":  pageMap(limit, offset, total),
	}, nil
}

func (s *Service) collectExecutionLogRecordsPage(taskID, source string, limit, offset int) ([]executionLogRecord, int, error) {
	if s == nil || s.storage == nil {
		return []executionLogRecord{}, 0, nil
	}
	ctx := context.Background()
	fetchLimit := executionLogCandidateLimit(limit, offset)
	items := make([]executionLogRecord, 0, fetchLimit*3)
	total := 0
	if source == "" || source == "event" {
		events, count, err := s.storage.LoopRuntimeStore().ListEvents(ctx, taskID, "", "", "", "", fetchLimit, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		total += count
		items = append(items, executionLogRecordsFromEvents(events)...)
	}
	if source == "" || source == "tool_call" {
		toolCalls, count, err := s.storage.ToolCallStore().ListToolCalls(ctx, taskID, "", fetchLimit, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		total += count
		items = append(items, executionLogRecordsFromToolCalls(toolCalls)...)
	}
	if source == "" || source == "audit" {
		audits, count, err := s.storage.AuditStore().ListAuditRecords(ctx, taskID, "", fetchLimit, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		total += count
		items = append(items, executionLogRecordsFromAudits(audits)...)
	}
	sortExecutionLogRecords(items)
	return paginateExecutionLogRecords(items, limit, offset), total, nil
}

func (s *Service) collectErrorExecutionLogRecordsPage(taskID, source string, limit, offset int) ([]executionLogRecord, int, error) {
	if s == nil || s.storage == nil {
		return []executionLogRecord{}, 0, nil
	}
	ctx := context.Background()
	fetchLimit := executionLogCandidateLimit(limit, offset)
	items := make([]executionLogRecord, 0, fetchLimit*3)
	total := 0
	if source == "" || source == "event" {
		events, count, err := listErrorEventRecords(ctx, s.storage.LoopRuntimeStore(), taskID, fetchLimit, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		total += count
		items = append(items, executionLogRecordsFromEvents(events)...)
	}
	if source == "" || source == "tool_call" {
		toolCalls, count, err := listErrorToolCallRecords(ctx, s.storage.ToolCallStore(), taskID, fetchLimit, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		total += count
		items = append(items, executionLogRecordsFromToolCalls(toolCalls)...)
	}
	if source == "" || source == "audit" {
		audits, count, err := listErrorAuditRecords(ctx, s.storage.AuditStore(), taskID, fetchLimit, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		total += count
		items = append(items, executionLogRecordsFromAudits(audits)...)
	}
	sortExecutionLogRecords(items)
	return paginateExecutionLogRecords(items, limit, offset), total, nil
}

// executionLogCandidateLimit asks each source for only the first offset+limit
// rows. Any record ranked below that window inside its own source cannot appear
// in the merged page window because that source already has enough newer rows to
// keep it out of the union's first offset+limit positions.
func executionLogCandidateLimit(limit, offset int) int {
	if limit <= 0 {
		return 0
	}
	return limit + offset
}

func listErrorEventRecords(ctx context.Context, store storage.LoopRuntimeStore, taskID string, limit, offset int) ([]storage.EventRecord, int, error) {
	if store == nil {
		return []storage.EventRecord{}, 0, nil
	}
	if lister, ok := store.(errorEventLister); ok {
		return lister.ListErrorEvents(ctx, taskID, "", limit, offset)
	}
	items, _, err := store.ListEvents(ctx, taskID, "", "", "", "", 0, 0)
	if err != nil {
		return nil, 0, err
	}
	filtered := filterErrorEventRecords(items)
	return paginateEventRecords(filtered, limit, offset), len(filtered), nil
}

func listErrorToolCallRecords(ctx context.Context, store storage.ToolCallStore, taskID string, limit, offset int) ([]tools.ToolCallRecord, int, error) {
	if store == nil {
		return []tools.ToolCallRecord{}, 0, nil
	}
	if lister, ok := store.(errorToolCallLister); ok {
		return lister.ListErrorToolCalls(ctx, taskID, "", limit, offset)
	}
	items, _, err := store.ListToolCalls(ctx, taskID, "", 0, 0)
	if err != nil {
		return nil, 0, err
	}
	filtered := filterErrorToolCalls(items)
	return paginateToolCallRecords(filtered, limit, offset), len(filtered), nil
}

func listErrorAuditRecords(ctx context.Context, store storage.AuditStore, taskID string, limit, offset int) ([]audit.Record, int, error) {
	if store == nil {
		return []audit.Record{}, 0, nil
	}
	if lister, ok := store.(errorAuditLister); ok {
		return lister.ListErrorAuditRecords(ctx, taskID, "", limit, offset)
	}
	items, _, err := store.ListAuditRecords(ctx, taskID, "", 0, 0)
	if err != nil {
		return nil, 0, err
	}
	filtered := filterErrorAuditRecords(items)
	return paginateAuditRecords(filtered, limit, offset), len(filtered), nil
}

func filterErrorEventRecords(records []storage.EventRecord) []storage.EventRecord {
	items := make([]storage.EventRecord, 0, len(records))
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Level), "error") || strings.Contains(strings.ToLower(strings.TrimSpace(record.Type)), "failed") {
			items = append(items, record)
		}
	}
	return items
}

func filterErrorToolCalls(records []tools.ToolCallRecord) []tools.ToolCallRecord {
	items := make([]tools.ToolCallRecord, 0, len(records))
	for _, record := range records {
		if record.ErrorCode != nil || record.Status == tools.ToolCallStatusFailed || record.Status == tools.ToolCallStatusTimeout {
			items = append(items, record)
		}
	}
	return items
}

func filterErrorAuditRecords(records []audit.Record) []audit.Record {
	items := make([]audit.Record, 0, len(records))
	for _, record := range records {
		result := strings.ToLower(strings.TrimSpace(record.Result))
		if strings.Contains(result, "fail") || strings.Contains(result, "error") || result == "denied" || result == "blocked" {
			items = append(items, record)
		}
	}
	return items
}

func paginateEventRecords(records []storage.EventRecord, limit, offset int) []storage.EventRecord {
	if offset >= len(records) {
		return []storage.EventRecord{}
	}
	end := offset + limit
	if limit <= 0 || end > len(records) {
		end = len(records)
	}
	return append([]storage.EventRecord(nil), records[offset:end]...)
}

func paginateToolCallRecords(records []tools.ToolCallRecord, limit, offset int) []tools.ToolCallRecord {
	if offset >= len(records) {
		return []tools.ToolCallRecord{}
	}
	end := offset + limit
	if limit <= 0 || end > len(records) {
		end = len(records)
	}
	return append([]tools.ToolCallRecord(nil), records[offset:end]...)
}

func paginateAuditRecords(records []audit.Record, limit, offset int) []audit.Record {
	if offset >= len(records) {
		return []audit.Record{}
	}
	end := offset + limit
	if limit <= 0 || end > len(records) {
		end = len(records)
	}
	return append([]audit.Record(nil), records[offset:end]...)
}

func executionLogRecordsFromEvents(records []storage.EventRecord) []executionLogRecord {
	items := make([]executionLogRecord, 0, len(records))
	for _, record := range records {
		items = append(items, executionLogRecord{
			LogID:     "event:" + record.EventID,
			TaskID:    record.TaskID,
			RunID:     record.RunID,
			Source:    "event",
			Kind:      record.Type,
			Level:     firstNonEmptyString(strings.TrimSpace(record.Level), "info"),
			Summary:   record.Type,
			Detail:    record.PayloadJSON,
			CreatedAt: record.CreatedAt,
		})
	}
	return items
}

func executionLogRecordsFromToolCalls(records []tools.ToolCallRecord) []executionLogRecord {
	items := make([]executionLogRecord, 0, len(records))
	for _, record := range records {
		items = append(items, executionLogRecord{
			LogID:     "tool_call:" + record.ToolCallID,
			TaskID:    record.TaskID,
			RunID:     record.RunID,
			Source:    "tool_call",
			Kind:      record.ToolName,
			Level:     toolCallLogLevel(record),
			Summary:   record.ToolName,
			Detail:    fmt.Sprintf("status=%s duration_ms=%d", record.Status, record.DurationMS),
			Status:    string(record.Status),
			ErrorCode: toolCallErrorCodeString(record.ErrorCode),
			CreatedAt: record.CreatedAt,
		})
	}
	return items
}

func executionLogRecordsFromAudits(records []audit.Record) []executionLogRecord {
	items := make([]executionLogRecord, 0, len(records))
	for _, record := range records {
		items = append(items, executionLogRecord{
			LogID:     "audit:" + record.AuditID,
			TaskID:    record.TaskID,
			Source:    "audit",
			Kind:      record.Action,
			Level:     auditLogLevel(record.Result),
			Summary:   firstNonEmptyString(record.Summary, record.Action),
			Detail:    fmt.Sprintf("target=%s result=%s", record.Target, record.Result),
			Status:    record.Result,
			CreatedAt: record.CreatedAt,
		})
	}
	return items
}

func executionLogRecordMaps(records []executionLogRecord) []map[string]any {
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"log_id":     record.LogID,
			"task_id":    nullableResponseString(record.TaskID),
			"run_id":     nullableResponseString(record.RunID),
			"source":     record.Source,
			"kind":       record.Kind,
			"level":      record.Level,
			"summary":    record.Summary,
			"detail":     record.Detail,
			"status":     nullableResponseString(record.Status),
			"error_code": nullableResponseString(record.ErrorCode),
			"created_at": record.CreatedAt,
		})
	}
	return items
}

func filterErrorExecutionLogRecords(records []executionLogRecord) []executionLogRecord {
	items := make([]executionLogRecord, 0, len(records))
	for _, record := range records {
		if isErrorExecutionLogRecord(record) {
			items = append(items, record)
		}
	}
	return items
}

func isErrorExecutionLogRecord(record executionLogRecord) bool {
	status := strings.ToLower(strings.TrimSpace(record.Status))
	level := strings.ToLower(strings.TrimSpace(record.Level))
	kind := strings.ToLower(strings.TrimSpace(record.Kind))
	switch record.Source {
	case "tool_call":
		return status == "failed" || strings.TrimSpace(record.ErrorCode) != ""
	case "event":
		return level == "error" || strings.Contains(kind, "failed")
	case "audit":
		return strings.Contains(status, "fail") || strings.Contains(status, "error")
	default:
		return false
	}
}

func sortExecutionLogRecords(records []executionLogRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].CreatedAt != records[j].CreatedAt {
			return records[i].CreatedAt > records[j].CreatedAt
		}
		return records[i].LogID > records[j].LogID
	})
}

func paginateExecutionLogRecords(records []executionLogRecord, limit, offset int) []executionLogRecord {
	if offset >= len(records) {
		return []executionLogRecord{}
	}
	end := offset + limit
	if limit <= 0 || end > len(records) {
		end = len(records)
	}
	return append([]executionLogRecord(nil), records[offset:end]...)
}

func toolCallLogLevel(record tools.ToolCallRecord) string {
	if record.ErrorCode != nil || record.Status == tools.ToolCallStatusFailed || record.Status == tools.ToolCallStatusTimeout {
		return "error"
	}
	if record.Status == tools.ToolCallStatusStarted {
		return "info"
	}
	return "info"
}

func toolCallErrorCodeString(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func auditLogLevel(result string) string {
	normalized := strings.ToLower(strings.TrimSpace(result))
	if strings.Contains(normalized, "fail") || strings.Contains(normalized, "error") {
		return "error"
	}
	if normalized == "denied" || normalized == "blocked" {
		return "warning"
	}
	return "info"
}

func currentRuntimeRoot(storageService *storage.Service) string {
	if databasePath := currentDatabasePath(storageService); databasePath != "" {
		databaseDir := filepath.Dir(databasePath)
		if strings.EqualFold(filepath.Base(databaseDir), "data") {
			return filepath.ToSlash(filepath.Clean(filepath.Dir(databaseDir)))
		}
		return filepath.ToSlash(filepath.Clean(databaseDir))
	}
	return filepath.ToSlash(filepath.Clean(serviceconfig.DefaultRuntimeRoot()))
}

func currentDatabasePath(storageService *storage.Service) string {
	if storageService == nil {
		return filepath.ToSlash(filepath.Clean(serviceconfig.DefaultDatabasePath()))
	}
	if value := strings.TrimSpace(storageService.DatabasePath()); value != "" {
		return filepath.ToSlash(filepath.Clean(value))
	}
	return filepath.ToSlash(filepath.Clean(serviceconfig.DefaultDatabasePath()))
}

func currentSecretStorePath(storageService *storage.Service) string {
	if storageService == nil {
		return ""
	}
	value := strings.TrimSpace(storageService.SecretStorePath())
	if value == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(value))
}

func nullableResponseString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
