// 该文件负责 task/run 主状态在存储层的本地回退实现。
package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// InMemoryTaskRunStore 提供 task/run 主状态的内存回退存储。
type InMemoryTaskRunStore struct {
	mu        sync.RWMutex
	records   map[string]TaskRunRecord
	sequences map[string]uint64
}

// NewInMemoryTaskRunStore 创建并返回 InMemoryTaskRunStore。
func NewInMemoryTaskRunStore() *InMemoryTaskRunStore {
	return &InMemoryTaskRunStore{
		records:   make(map[string]TaskRunRecord),
		sequences: make(map[string]uint64),
	}
}

// AllocateIdentifier 为给定前缀分配一个稳定递增的标识符。
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

// SaveTaskRun 保存或覆盖一条 task/run 主状态快照。
func (s *InMemoryTaskRunStore) SaveTaskRun(_ context.Context, record TaskRunRecord) error {
	if err := validateTaskRunRecord(record); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[record.TaskID] = cloneTaskRunRecord(record)
	return nil
}

// LoadTaskRuns 返回当前所有 task/run 主状态快照。
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
