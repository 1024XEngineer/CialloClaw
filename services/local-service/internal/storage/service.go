package storage

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"

type Service struct {
	adapter platform.StorageAdapter
}

func NewService(adapter platform.StorageAdapter) *Service {
	return &Service{adapter: adapter}
}

func (s *Service) DatabasePath() string {
	return s.adapter.DatabasePath()
}
