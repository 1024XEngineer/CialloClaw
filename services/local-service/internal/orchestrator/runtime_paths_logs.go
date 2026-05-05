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
	items, err := s.collectExecutionLogRecords(taskID, source)
	if err != nil {
		return nil, err
	}
	total := len(items)
	items = paginateExecutionLogRecords(items, limit, offset)
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
	items, err := s.collectExecutionLogRecords(taskID, source)
	if err != nil {
		return nil, err
	}
	items = filterErrorExecutionLogRecords(items)
	total := len(items)
	items = paginateExecutionLogRecords(items, limit, offset)
	return map[string]any{
		"items": executionLogRecordMaps(items),
		"page":  pageMap(limit, offset, total),
	}, nil
}

func (s *Service) collectExecutionLogRecords(taskID, source string) ([]executionLogRecord, error) {
	if s == nil || s.storage == nil {
		return []executionLogRecord{}, nil
	}
	ctx := context.Background()
	items := make([]executionLogRecord, 0)
	if source == "" || source == "event" {
		events, _, err := s.storage.LoopRuntimeStore().ListEvents(ctx, taskID, "", "", "", "", 0, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		items = append(items, executionLogRecordsFromEvents(events)...)
	}
	if source == "" || source == "tool_call" {
		toolCalls, _, err := s.storage.ToolCallStore().ListToolCalls(ctx, taskID, "", 0, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		items = append(items, executionLogRecordsFromToolCalls(toolCalls)...)
	}
	if source == "" || source == "audit" {
		audits, _, err := s.storage.AuditStore().ListAuditRecords(ctx, taskID, "", 0, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		items = append(items, executionLogRecordsFromAudits(audits)...)
	}
	sortExecutionLogRecords(items)
	return items, nil
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
