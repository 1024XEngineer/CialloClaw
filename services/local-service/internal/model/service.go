package model

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"

type Service struct {
	provider string
	modelID  string
	endpoint string
}

func NewService(cfg config.ModelConfig) *Service {
	return &Service{
		provider: cfg.Provider,
		modelID:  cfg.ModelID,
		endpoint: cfg.Endpoint,
	}
}

func (s *Service) Descriptor() string {
	return s.provider + ":" + s.modelID
}
