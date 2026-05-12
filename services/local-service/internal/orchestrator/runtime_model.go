package orchestrator

import (
	"errors"
	"strings"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

// ReplaceModel swaps the runtime model dependency used to prepare future-task
// executions. Existing tasks keep the model snapshot already captured by the
// execution layer, so settings changes surface through `next_task_effective`.
func (s *Service) ReplaceModel(modelService *model.Service) {
	if s == nil {
		return
	}
	s.modelMu.Lock()
	s.model = modelService
	s.modelMu.Unlock()
	if s.titleGenerator != nil {
		s.titleGenerator.ReplaceModel(modelService)
	}
	if s.executor != nil {
		s.executor.ReplaceModel(modelService)
	}
}

func (s *Service) currentModel() *model.Service {
	if s == nil {
		return nil
	}
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()
	return s.model
}

func (s *Service) currentModelConfig() serviceconfig.ModelConfig {
	modelService := s.currentModel()
	if modelService == nil {
		return serviceconfig.ModelConfig{}
	}
	return modelService.RuntimeConfig()
}

func (s *Service) currentModelDescriptor() string {
	modelService := s.currentModel()
	if modelService == nil {
		return ""
	}
	return modelService.Descriptor()
}

func (s *Service) reloadRuntimeModelFromSettings() error {
	if s == nil || s.runEngine == nil {
		return nil
	}
	resolvedConfig := model.RuntimeConfigFromSettings(s.currentModelConfig(), s.runEngine.Settings())
	modelService, err := model.NewServiceFromConfig(model.ServiceConfig{
		ModelConfig:  resolvedConfig,
		SecretSource: model.NewStaticSecretSource(s.storage),
	})
	if err != nil {
		if shouldFallbackRuntimeModelReload(err) {
			modelService = model.NewService(resolvedConfig)
		} else {
			return err
		}
	}
	s.ReplaceModel(modelService)
	return nil
}

func shouldFallbackRuntimeModelReload(err error) bool {
	if errors.Is(err, model.ErrModelProviderUnsupported) {
		return true
	}
	if errors.Is(err, model.ErrSecretSourceFailed) {
		return true
	}
	return errors.Is(err, storage.ErrSecretNotFound) ||
		errors.Is(err, storage.ErrSecretStoreAccessFailed) ||
		errors.Is(err, storage.ErrStrongholdUnavailable) ||
		errors.Is(err, storage.ErrStrongholdAccessFailed)
}

func modelSettingsTouched(updatedKeys []string) bool {
	for _, key := range updatedKeys {
		switch strings.TrimSpace(key) {
		case "models.provider",
			"models.base_url",
			"models.model",
			"models.api_key",
			"models.delete_api_key",
			"models.credentials.provider",
			"models.credentials.base_url",
			"models.credentials.model":
			return true
		}
	}
	return false
}
