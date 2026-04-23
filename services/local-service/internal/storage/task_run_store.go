package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// InMemoryTaskRunStore provides an in-memory fallback for task/run persistence.
type InMemoryTaskRunStore struct {
	mu        sync.RWMutex
	records   map[string]TaskRunRecord
	sequences map[string]uint64
	taskStore TaskStore
	stepStore TaskStepStore
}

// NewInMemoryTaskRunStore builds a fresh in-memory task/run store.
func NewInMemoryTaskRunStore() *InMemoryTaskRunStore {
	return &InMemoryTaskRunStore{
		records:   make(map[string]TaskRunRecord),
		sequences: make(map[string]uint64),
		taskStore: newInMemoryTaskStore(),
		stepStore: newInMemoryTaskStepStore(),
	}
}

// WithStructuredStores attaches first-class task/task_step writers for dual-write persistence.
func (s *InMemoryTaskRunStore) WithStructuredStores(taskStore TaskStore, stepStore TaskStepStore) *InMemoryTaskRunStore {
	if taskStore != nil {
		s.taskStore = taskStore
	}
	if stepStore != nil {
		s.stepStore = stepStore
	}
	return s
}

// AllocateIdentifier reserves the next stable identifier for the given prefix.
func (s *InMemoryTaskRunStore) AllocateIdentifier(_ context.Context, prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", ErrTaskRunIdentifierPrefixRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sequences[prefix]++
	return fmt.Sprintf("%s_%03d", prefix, s.sequences[prefix]), nil
}

// DeleteTaskRun removes one persisted task/run snapshot from the in-memory store.
func (s *InMemoryTaskRunStore) DeleteTaskRun(_ context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrTaskRunTaskIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, taskID)
	if s.taskStore != nil {
		if err := s.taskStore.DeleteTask(context.Background(), taskID); err != nil {
			return err
		}
	}
	if s.stepStore != nil {
		if err := s.stepStore.ReplaceTaskSteps(context.Background(), taskID, nil); err != nil {
			return err
		}
	}
	return nil
}

// SaveTaskRun saves or overwrites one task/run snapshot.
func (s *InMemoryTaskRunStore) SaveTaskRun(_ context.Context, record TaskRunRecord) error {
	if err := validateTaskRunRecord(record); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := writeStructuredTaskState(context.Background(), s.taskStore, s.stepStore, record); err != nil {
		return err
	}
	s.records[record.TaskID] = cloneTaskRunRecord(record)
	return nil
}

// LoadTaskRuns returns all currently persisted task/run snapshots.
func (s *InMemoryTaskRunStore) LoadTaskRuns(_ context.Context) ([]TaskRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]TaskRunRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, cloneTaskRunRecord(record))
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].StartedAt.Equal(records[j].StartedAt) {
			return records[i].TaskID > records[j].TaskID
		}
		return records[i].StartedAt.After(records[j].StartedAt)
	})

	return records, nil
}

// GetTaskRun loads one persisted task/run snapshot by task_id.
func (s *InMemoryTaskRunStore) GetTaskRun(_ context.Context, taskID string) (TaskRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[strings.TrimSpace(taskID)]
	if !ok {
		return TaskRunRecord{}, sql.ErrNoRows
	}
	return cloneTaskRunRecord(record), nil
}

// LoadLegacyTaskRuns returns task_run compatibility snapshots whose task_id is
// still missing from the structured tasks table.
func (s *InMemoryTaskRunStore) LoadLegacyTaskRuns(_ context.Context, structuredTaskIDs []string) ([]TaskRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	excluded := make(map[string]struct{}, len(structuredTaskIDs))
	for _, taskID := range structuredTaskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		excluded[taskID] = struct{}{}
	}

	records := make([]TaskRunRecord, 0, len(s.records))
	for taskID, record := range s.records {
		if _, skip := excluded[taskID]; skip {
			continue
		}
		records = append(records, cloneTaskRunRecord(record))
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].StartedAt.Equal(records[j].StartedAt) {
			return records[i].TaskID > records[j].TaskID
		}
		return records[i].StartedAt.After(records[j].StartedAt)
	})

	return records, nil
}

// writeStructuredTaskState keeps the new product-facing tasks/task_steps tables
// in sync with the legacy task_runs snapshot so the migration can stay dual-write
// until all read paths fully leave the compatibility record_json layer.
func writeStructuredTaskState(ctx context.Context, taskStore TaskStore, stepStore TaskStepStore, record TaskRunRecord) error {
	if taskStore != nil {
		taskRecord, err := taskRecordFromSnapshot(record)
		if err != nil {
			return err
		}
		if err := taskStore.WriteTask(ctx, taskRecord); err != nil {
			return err
		}
	}
	if stepStore != nil {
		if err := stepStore.ReplaceTaskSteps(ctx, record.TaskID, taskStepRecordsFromSnapshot(record)); err != nil {
			return err
		}
	}
	return nil
}

// taskRecordFromSnapshot projects one compatibility snapshot into the formal
// tasks row while still preserving the full snapshot payload for gradual reads.
func taskRecordFromSnapshot(record TaskRunRecord) (TaskRecord, error) {
	intentArgumentsJSON := "{}"
	if arguments, ok := record.Intent["arguments"]; ok {
		payload, err := json.Marshal(arguments)
		if err != nil {
			return TaskRecord{}, fmt.Errorf("marshal task intent arguments: %w", err)
		}
		intentArgumentsJSON = string(payload)
	}
	snapshotJSON, err := marshalTaskRunRecord(record)
	if err != nil {
		return TaskRecord{}, err
	}
	finishedAt := ""
	if record.FinishedAt != nil {
		finishedAt = record.FinishedAt.Format(time.RFC3339Nano)
	}
	requestSource := strings.TrimSpace(record.RequestSource)
	if requestSource == "" {
		requestSource = strings.TrimSpace(record.Snapshot.Source)
	}
	requestTrigger := strings.TrimSpace(record.RequestTrigger)
	if requestTrigger == "" {
		requestTrigger = strings.TrimSpace(record.Snapshot.Trigger)
	}
	return TaskRecord{
		TaskID:              record.TaskID,
		SessionID:           record.SessionID,
		RunID:               record.RunID,
		PrimaryRunID:        record.RunID,
		Title:               record.Title,
		SourceType:          record.SourceType,
		Status:              record.Status,
		IntentName:          stringValueFromMap(record.Intent, "name"),
		IntentArgumentsJSON: intentArgumentsJSON,
		PreferredDelivery:   record.PreferredDelivery,
		FallbackDelivery:    record.FallbackDelivery,
		CurrentStep:         record.CurrentStep,
		CurrentStepStatus:   record.CurrentStepStatus,
		RiskLevel:           record.RiskLevel,
		RequestSource:       requestSource,
		RequestTrigger:      requestTrigger,
		StartedAt:           record.StartedAt.Format(time.RFC3339Nano),
		UpdatedAt:           record.UpdatedAt.Format(time.RFC3339Nano),
		FinishedAt:          finishedAt,
		SnapshotJSON:        snapshotJSON,
	}, nil
}

func taskStepRecordsFromSnapshot(record TaskRunRecord) []TaskStepRecord {
	items := make([]TaskStepRecord, 0, len(record.Timeline))
	createdAt := record.StartedAt.Format(time.RFC3339Nano)
	updatedAt := record.UpdatedAt.Format(time.RFC3339Nano)
	for _, step := range record.Timeline {
		items = append(items, TaskStepRecord{
			StepID:        step.StepID,
			TaskID:        record.TaskID,
			Name:          step.Name,
			Status:        step.Status,
			OrderIndex:    step.OrderIndex,
			InputSummary:  step.InputSummary,
			OutputSummary: step.OutputSummary,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		})
	}
	return items
}

func stringValueFromMap(value map[string]any, key string) string {
	raw, ok := value[key]
	if !ok {
		return ""
	}
	text, _ := raw.(string)
	return text
}
