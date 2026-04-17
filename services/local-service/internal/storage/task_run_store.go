package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// InMemoryTaskRunStore provides an in-memory fallback for task/run persistence.
type InMemoryTaskRunStore struct {
	mu        sync.RWMutex
	records   map[string]TaskRunRecord
	sequences map[string]uint64
}

// NewInMemoryTaskRunStore builds a fresh in-memory task/run store.
func NewInMemoryTaskRunStore() *InMemoryTaskRunStore {
	return &InMemoryTaskRunStore{
		records:   make(map[string]TaskRunRecord),
		sequences: make(map[string]uint64),
	}
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
	return nil
}

// SaveTaskRun saves or overwrites one task/run snapshot.
func (s *InMemoryTaskRunStore) SaveTaskRun(_ context.Context, record TaskRunRecord) error {
	if err := validateTaskRunRecord(record); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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
