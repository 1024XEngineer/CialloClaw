package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	inspectionTaskIDPrefix            = "insp_"
	inspectorTitleGenerationIntent    = "task_inspector.generate_note_title"
	inspectorTitleGenerationTaskMatch = inspectionTaskIDPrefix + "%"
)

// InspectorTitleGenerationUsageSummary captures day-scoped token and cost
// totals for manual inspector note-title generation that runs outside formal
// task execution.
type InspectorTitleGenerationUsageSummary struct {
	TotalTokens int
	TotalCost   float64
}

// InspectorTitleGenerationUsage returns today's manual inspector title
// generation usage so dashboard/security summaries can read a bounded
// aggregate instead of scanning the full trace/eval history on every poll.
func (s *Service) InspectorTitleGenerationUsage(ctx context.Context, now time.Time) (InspectorTitleGenerationUsageSummary, error) {
	if s == nil || s.traceStore == nil || s.evalStore == nil {
		return InspectorTitleGenerationUsageSummary{}, nil
	}

	dayStart := now.UTC().Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)

	if traceStore, ok := s.traceStore.(*SQLiteTraceStore); ok {
		return traceStore.summarizeInspectorTitleGenerationUsage(ctx, dayStart, dayEnd)
	}

	return s.fallbackInspectorTitleGenerationUsage(ctx, dayStart, dayEnd)
}

func (s *Service) fallbackInspectorTitleGenerationUsage(ctx context.Context, dayStart, dayEnd time.Time) (InspectorTitleGenerationUsageSummary, error) {
	traceRecords, _, err := s.traceStore.ListTraceRecords(ctx, "", 0, 0)
	if err != nil {
		return InspectorTitleGenerationUsageSummary{}, err
	}

	relevantTraceIDs := make(map[string]struct{}, len(traceRecords))
	summary := InspectorTitleGenerationUsageSummary{}
	for _, record := range traceRecords {
		createdAt := parseGovernanceTime(record.CreatedAt).UTC()
		if createdAt.Before(dayStart) || !createdAt.Before(dayEnd) {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(record.TaskID), inspectionTaskIDPrefix) {
			continue
		}
		if strings.TrimSpace(record.LLMInputSummary) != inspectorTitleGenerationIntent {
			continue
		}
		relevantTraceIDs[record.TraceID] = struct{}{}
		summary.TotalCost += record.Cost
	}
	if len(relevantTraceIDs) == 0 {
		return summary, nil
	}

	evalSnapshots, _, err := s.evalStore.ListEvalSnapshots(ctx, "", 0, 0)
	if err != nil {
		return InspectorTitleGenerationUsageSummary{}, err
	}
	for _, snapshot := range evalSnapshots {
		createdAt := parseGovernanceTime(snapshot.CreatedAt).UTC()
		if createdAt.Before(dayStart) || !createdAt.Before(dayEnd) {
			continue
		}
		if _, ok := relevantTraceIDs[snapshot.TraceID]; !ok {
			continue
		}
		summary.TotalTokens += totalTokensFromJSON(snapshot.MetricsJSON)
	}
	return summary, nil
}

func (s *SQLiteTraceStore) summarizeInspectorTitleGenerationUsage(ctx context.Context, dayStart, dayEnd time.Time) (InspectorTitleGenerationUsageSummary, error) {
	row := s.db.QueryRowContext(ctx, `
		WITH filtered_traces AS (
			SELECT trace_id, cost
			FROM trace_records
			WHERE created_at >= ?
			  AND created_at < ?
			  AND task_id LIKE ?
			  AND llm_input_summary = ?
		)
		SELECT
			COALESCE((
				SELECT SUM(COALESCE(CAST(json_extract(eval_snapshots.metrics_json, '$.total_tokens') AS INTEGER), 0))
				FROM eval_snapshots
				JOIN filtered_traces ON filtered_traces.trace_id = eval_snapshots.trace_id
			), 0),
			COALESCE((SELECT SUM(cost) FROM filtered_traces), 0)
	`,
		dayStart.Format(time.RFC3339),
		dayEnd.Format(time.RFC3339),
		inspectorTitleGenerationTaskMatch,
		inspectorTitleGenerationIntent,
	)

	var (
		totalTokens sql.NullInt64
		totalCost   sql.NullFloat64
	)
	if err := row.Scan(&totalTokens, &totalCost); err != nil {
		return InspectorTitleGenerationUsageSummary{}, fmt.Errorf("summarize inspector title generation usage: %w", err)
	}

	return InspectorTitleGenerationUsageSummary{
		TotalTokens: int(totalTokens.Int64),
		TotalCost:   totalCost.Float64,
	}, nil
}

func totalTokensFromJSON(metricsJSON string) int {
	metricsJSON = strings.TrimSpace(metricsJSON)
	if metricsJSON == "" {
		return 0
	}

	metrics := map[string]any{}
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		return 0
	}
	return intValue(metrics["total_tokens"])
}

func intValue(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
