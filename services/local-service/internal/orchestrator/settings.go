package orchestrator

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

type modelSecretRollback struct {
	provider string
	record   storage.SecretRecord
	existed  bool
}

// SettingsGet returns the requested settings snapshot without mutating storage
// or runtime model state. Secret availability is reported as metadata rather
// than exposing secret values.
func (s *Service) SettingsGet(params map[string]any) (map[string]any, error) {
	settings := normalizeSettingsSnapshot(s.runEngine.Settings())
	scope := normalizeSettingsScope(stringValue(params, "scope", "all"))
	if scope == "all" || scope == "models" {
		settingsWithSecrets, err := s.attachSensitiveSettingAvailability(settings)
		if err != nil {
			return nil, err
		}
		settings = settingsWithSecrets
	}
	if scope == "all" {
		return map[string]any{"settings": settings}, nil
	}

	section, ok := settings[scope].(map[string]any)
	if !ok {
		return map[string]any{"settings": map[string]any{}}, nil
	}

	return map[string]any{"settings": map[string]any{scope: cloneMap(section)}}, nil
}

// SettingsUpdate validates and commits a settings patch, then reloads runtime
// model state when needed. Secret writes are transactional with rollback so a
// failed settings save does not leave stale credentials behind.
func (s *Service) SettingsUpdate(params map[string]any) (map[string]any, error) {
	normalizedParams := normalizeSettingsUpdateParams(params)
	previewSettings, previewUpdatedKeys, _, _, err := s.previewSettingsUpdate(normalizedParams)
	if err != nil {
		return nil, err
	}
	modelSettingsChanged := modelSettingsTouched(previewUpdatedKeys)
	modelSecretTouched := false
	secretUpdatedKeys := make([]string, 0, 2)
	rollbacks := make([]modelSecretRollback, 0, 2)
	previousModel := s.currentModel()
	if models := cloneMap(mapValue(normalizedParams, "models")); len(models) > 0 {
		if deleteAPIKey := boolValue(models, "delete_api_key", false); deleteAPIKey {
			provider := s.providerForSettingsUpdate(models)
			rollback, rollbackErr := s.captureModelSecretRollback(provider)
			if rollbackErr != nil {
				return nil, rollbackErr
			}
			if err := s.deleteModelSecret(provider); err != nil {
				return nil, err
			}
			rollbacks = append(rollbacks, rollback)
			delete(models, "delete_api_key")
			normalizedParams["models"] = models
			modelSecretTouched = true
			secretUpdatedKeys = append(secretUpdatedKeys, "models.delete_api_key")
		}
		if apiKey := stringValue(models, "api_key", ""); apiKey != "" {
			provider := s.providerForSettingsUpdate(models)
			rollback, rollbackErr := s.captureModelSecretRollback(provider)
			if rollbackErr != nil {
				return nil, rollbackErr
			}
			if err := s.persistModelSecret(provider, apiKey); err != nil {
				return nil, err
			}
			rollbacks = append(rollbacks, rollback)
			delete(models, "api_key")
			normalizedParams["models"] = models
			modelSecretTouched = true
			secretUpdatedKeys = append(secretUpdatedKeys, "models.api_key")
		}
	}
	if modelSettingsChanged {
		if err := s.reloadRuntimeModelForSettings(previewSettings); err != nil {
			s.rollbackModelSecretMutations(rollbacks)
			return nil, err
		}
	}
	effectiveSettings, updatedKeys, applyMode, needRestart, err := s.runEngine.UpdateSettings(normalizedParams)
	if err != nil {
		s.ReplaceModel(previousModel)
		s.rollbackModelSecretMutations(rollbacks)
		return nil, err
	}
	if modelSettingsChanged {
		applyMode = "next_task_effective"
		needRestart = false
	}
	if modelSecretTouched {
		if _, ok := effectiveSettings["models"]; !ok {
			effectiveSettings["models"] = map[string]any{}
		}
	}
	if _, ok := effectiveSettings["models"]; ok {
		effectiveSettings = s.attachSensitiveSettingAvailabilityForCommittedUpdate(effectiveSettings, secretUpdatedKeys)
	}
	effectiveSettings = outwardSettingsUpdatePatch(effectiveSettings)
	updatedKeys = outwardSettingsUpdateKeys(updatedKeys, secretUpdatedKeys)
	return map[string]any{
		"updated_keys":       updatedKeys,
		"effective_settings": effectiveSettings,
		"apply_mode":         applyMode,
		"need_restart":       needRestart,
	}, nil
}

// attachSensitiveSettingAvailabilityForCommittedUpdate decorates the response
// payload after a settings update has already committed. At this point the
// runtime model, secrets, and settings snapshot may already be live, so a
// follow-up Stronghold read must not turn the committed save back into an RPC
// error. When the readonly secret probe fails, the response degrades to stable
// metadata derived from the just-applied mutation hints and current Stronghold
// descriptor instead of reopening a partial-apply path.
func (s *Service) attachSensitiveSettingAvailabilityForCommittedUpdate(settings map[string]any, secretUpdatedKeys []string) map[string]any {
	decorated, err := s.attachSensitiveSettingAvailability(settings)
	if err == nil {
		return decorated
	}
	return attachSensitiveSettingAvailabilityFallback(settings, strongholdStatusFromStorage(s.storage), settingsUpdateSecretAvailabilityHint(secretUpdatedKeys))
}

// previewSettingsUpdate computes the future settings snapshot without mutating
// runengine state so SettingsUpdate can validate runtime model reloads before
// persisting a next-task-effective model route.
func (s *Service) previewSettingsUpdate(values map[string]any) (map[string]any, []string, string, bool, error) {
	if s == nil || s.runEngine == nil {
		return nil, nil, "", false, nil
	}
	currentSettings := s.runEngine.Settings()
	nextSettings := cloneMap(currentSettings)
	if nextSettings == nil {
		nextSettings = map[string]any{}
	}
	previewPatch := cloneMap(values)
	mergeSettingsPreview(nextSettings, previewPatch)
	updatedKeys := settingsPatchPathsFromPreview(previewPatch)
	applyMode := previewApplyMode(currentSettings, previewPatch, updatedKeys)
	needRestart := previewNeedsRestart(currentSettings, previewPatch)
	return normalizeSettingsSnapshot(nextSettings), updatedKeys, applyMode, needRestart, nil
}

func (s *Service) attachSensitiveSettingAvailability(settings map[string]any) (map[string]any, error) {
	cloned := normalizeSettingsSnapshot(cloneMap(settings))
	if cloned == nil {
		cloned = map[string]any{}
	}
	models := cloneMap(mapValue(cloned, "models"))
	if models == nil {
		models = map[string]any{}
	}
	credentials := cloneMap(mapValue(models, "credentials"))
	if credentials == nil {
		credentials = map[string]any{}
	}
	provider, configured, err := s.modelSecretConfigured(providerFromSettings(models, s.defaultSettingsProvider()))
	if err != nil {
		return nil, err
	}
	if stringValue(models, "provider", "") == "" && provider != "" {
		models["provider"] = provider
	}
	credentials["provider_api_key_configured"] = configured
	if stronghold := strongholdStatusFromStorage(s.storage); len(stronghold) > 0 {
		credentials["stronghold"] = stronghold
	}
	models["credentials"] = credentials
	cloned["models"] = models
	return cloned, nil
}

func (s *Service) modelSecretConfigured(provider string) (string, bool, error) {
	resolvedProvider := model.CanonicalProviderName(firstNonEmptyString(strings.TrimSpace(provider), s.defaultSettingsProvider()))
	if s.storage == nil || s.storage.SecretStore() == nil || resolvedProvider == "" {
		return resolvedProvider, false, nil
	}
	_, err := s.storage.SecretStore().GetSecret(context.Background(), "model", resolvedProvider+"_api_key")
	if err == nil {
		return resolvedProvider, true, nil
	}
	if errors.Is(err, storage.ErrSecretNotFound) {
		return resolvedProvider, false, nil
	}
	if errors.Is(err, storage.ErrSecretStoreAccessFailed) {
		return resolvedProvider, false, ErrStrongholdAccessFailed
	}
	if errors.Is(err, storage.ErrStrongholdUnavailable) {
		return resolvedProvider, false, ErrStrongholdAccessFailed
	}
	return resolvedProvider, false, err
}

func attachSensitiveSettingAvailabilityFallback(settings map[string]any, stronghold map[string]any, providerConfigured *bool) map[string]any {
	cloned := normalizeSettingsSnapshot(cloneMap(settings))
	if cloned == nil {
		cloned = map[string]any{}
	}
	models := cloneMap(mapValue(cloned, "models"))
	if models == nil {
		models = map[string]any{}
	}
	credentials := cloneMap(mapValue(models, "credentials"))
	if credentials == nil {
		credentials = map[string]any{}
	}
	if providerConfigured != nil {
		credentials["provider_api_key_configured"] = *providerConfigured
	} else if _, ok := credentials["provider_api_key_configured"]; !ok {
		credentials["provider_api_key_configured"] = false
	}
	if len(stronghold) > 0 {
		credentials["stronghold"] = cloneMap(stronghold)
	}
	models["credentials"] = credentials
	cloned["models"] = models
	return cloned
}

func settingsUpdateSecretAvailabilityHint(secretUpdatedKeys []string) *bool {
	for _, key := range secretUpdatedKeys {
		switch key {
		case "models.api_key":
			configured := true
			return &configured
		case "models.delete_api_key":
			configured := false
			return &configured
		}
	}
	return nil
}

func (s *Service) persistModelSecret(provider, apiKey string) error {
	resolvedProvider := model.CanonicalProviderName(firstNonEmptyString(strings.TrimSpace(provider), s.defaultSettingsProvider()))
	if s.storage == nil || s.storage.SecretStore() == nil || resolvedProvider == "" {
		return ErrStrongholdAccessFailed
	}
	if err := s.storage.SecretStore().PutSecret(context.Background(), storage.SecretRecord{
		Namespace: "model",
		Key:       resolvedProvider + "_api_key",
		Value:     strings.TrimSpace(apiKey),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		normalizedErr := storage.NormalizeSecretStoreError(err)
		if errors.Is(normalizedErr, storage.ErrStrongholdAccessFailed) || errors.Is(normalizedErr, storage.ErrStrongholdUnavailable) || errors.Is(normalizedErr, storage.ErrSecretStoreAccessFailed) {
			return ErrStrongholdAccessFailed
		}
		return normalizedErr
	}
	return nil
}

func (s *Service) deleteModelSecret(provider string) error {
	resolvedProvider := model.CanonicalProviderName(firstNonEmptyString(strings.TrimSpace(provider), s.defaultSettingsProvider()))
	if s.storage == nil || s.storage.SecretStore() == nil || resolvedProvider == "" {
		return ErrStrongholdAccessFailed
	}
	if err := s.storage.SecretStore().DeleteSecret(context.Background(), "model", resolvedProvider+"_api_key"); err != nil {
		normalizedErr := storage.NormalizeSecretStoreError(err)
		if errors.Is(normalizedErr, storage.ErrStrongholdAccessFailed) || errors.Is(normalizedErr, storage.ErrStrongholdUnavailable) || errors.Is(normalizedErr, storage.ErrSecretStoreAccessFailed) {
			return ErrStrongholdAccessFailed
		}
		return normalizedErr
	}
	return nil
}

func (s *Service) captureModelSecretRollback(provider string) (modelSecretRollback, error) {
	resolvedProvider := model.CanonicalProviderName(firstNonEmptyString(strings.TrimSpace(provider), s.defaultSettingsProvider()))
	rollback := modelSecretRollback{provider: resolvedProvider}
	if s.storage == nil || s.storage.SecretStore() == nil || resolvedProvider == "" {
		return rollback, nil
	}
	record, err := s.storage.SecretStore().GetSecret(context.Background(), "model", resolvedProvider+"_api_key")
	if err == nil {
		rollback.record = record
		rollback.existed = true
		return rollback, nil
	}
	normalizedErr := storage.NormalizeSecretStoreError(err)
	if errors.Is(normalizedErr, storage.ErrSecretNotFound) {
		return rollback, nil
	}
	if errors.Is(normalizedErr, storage.ErrStrongholdAccessFailed) {
		return rollback, ErrStrongholdAccessFailed
	}
	return rollback, normalizedErr
}

func (s *Service) rollbackModelSecretMutations(rollbacks []modelSecretRollback) {
	for index := len(rollbacks) - 1; index >= 0; index-- {
		rollback := rollbacks[index]
		if rollback.provider == "" || s == nil || s.storage == nil || s.storage.SecretStore() == nil {
			continue
		}
		if rollback.existed {
			_ = s.storage.SecretStore().PutSecret(context.Background(), rollback.record)
			continue
		}
		_ = s.storage.SecretStore().DeleteSecret(context.Background(), "model", rollback.provider+"_api_key")
	}
}

func (s *Service) reloadRuntimeModelForSettings(settings map[string]any) error {
	if s == nil || s.runEngine == nil {
		return nil
	}
	resolvedConfig := model.RuntimeConfigFromSettings(s.currentModelConfig(), settings)
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

func settingsPatchPathsFromPreview(patch map[string]any) []string {
	if len(patch) == 0 {
		return nil
	}
	keys := make([]string, 0, len(patch))
	for key := range patch {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	paths := make([]string, 0, len(keys))
	for _, key := range keys {
		nextPrefix := key
		if nested, ok := patch[key].(map[string]any); ok && len(nested) > 0 {
			for _, child := range settingsPatchPathsFromPreview(nested) {
				paths = append(paths, nextPrefix+"."+child)
			}
			continue
		}
		paths = append(paths, nextPrefix)
	}
	return paths
}

func mergeSettingsPreview(target map[string]any, patch map[string]any) {
	for key, value := range patch {
		patchMap, ok := value.(map[string]any)
		if ok {
			currentMap, currentOK := target[key].(map[string]any)
			if !currentOK {
				currentMap = map[string]any{}
			}
			mergeSettingsPreview(currentMap, patchMap)
			target[key] = currentMap
			continue
		}
		target[key] = value
	}
}

func previewNeedsRestart(currentSettings, patch map[string]any) bool {
	generalPatch := cloneMap(mapValue(patch, "general"))
	if len(generalPatch) > 0 {
		nextLanguage, ok := generalPatch["language"]
		if ok {
			currentGeneral := cloneMap(mapValue(currentSettings, "general"))
			currentLanguage, hasCurrentLanguage := currentGeneral["language"]
			if !hasCurrentLanguage || !reflect.DeepEqual(currentLanguage, nextLanguage) {
				return true
			}
		}
		downloadPatch := cloneMap(mapValue(generalPatch, "download"))
		if len(downloadPatch) > 0 {
			nextWorkspacePath, ok := downloadPatch["workspace_path"]
			if ok {
				currentDownload := cloneMap(mapValue(mapValue(currentSettings, "general"), "download"))
				currentWorkspacePath, hasCurrentWorkspacePath := currentDownload["workspace_path"]
				if !hasCurrentWorkspacePath || !reflect.DeepEqual(currentWorkspacePath, nextWorkspacePath) {
					return true
				}
			}
		}
	}
	return false
}

func previewApplyMode(currentSettings, patch map[string]any, updatedKeys []string) string {
	if previewNeedsRestart(currentSettings, patch) {
		return "restart_required"
	}
	if modelSettingsTouched(updatedKeys) {
		return "next_task_effective"
	}
	return "immediate"
}

func strongholdStatusFromStorage(store *storage.Service) map[string]any {
	if store == nil || store.Stronghold() == nil {
		return map[string]any{
			"backend":      "none",
			"available":    false,
			"fallback":     false,
			"initialized":  false,
			"formal_store": false,
		}
	}
	descriptor := store.Stronghold().Descriptor()
	return map[string]any{
		"backend":      descriptor.Backend,
		"available":    descriptor.Available,
		"fallback":     descriptor.Fallback,
		"initialized":  descriptor.Initialized,
		"formal_store": descriptor.Available && !descriptor.Fallback,
	}
}

func normalizeSettingsScope(scope string) string {
	switch strings.TrimSpace(scope) {
	case "", "all":
		return "all"
	case "data_log":
		return "models"
	default:
		return strings.TrimSpace(scope)
	}
}

func normalizeSettingsSnapshot(settings map[string]any) map[string]any {
	cloned := cloneMap(settings)
	if cloned == nil {
		return map[string]any{}
	}
	models := cloneMap(mapValue(cloned, "models"))
	if models == nil {
		models = map[string]any{}
	}
	if legacy := cloneMap(mapValue(cloned, "data_log")); len(legacy) > 0 {
		for key, value := range legacy {
			if key == "provider" {
				models[key] = value
				continue
			}
			credentials := cloneMap(mapValue(models, "credentials"))
			if credentials == nil {
				credentials = map[string]any{}
			}
			credentials[key] = value
			models["credentials"] = credentials
		}
		delete(cloned, "data_log")
	}
	models = normalizeModelSettingsSection(models)
	if len(models) > 0 {
		cloned["models"] = models
	}
	return cloned
}

func normalizeSettingsUpdateParams(params map[string]any) map[string]any {
	cloned := cloneMap(params)
	if cloned == nil {
		return map[string]any{}
	}
	models := cloneMap(mapValue(cloned, "models"))
	if models == nil {
		models = map[string]any{}
	}
	if legacy := cloneMap(mapValue(cloned, "data_log")); len(legacy) > 0 {
		for key, value := range legacy {
			if key == "provider_api_key_configured" || key == "stronghold" {
				continue
			}
			models[key] = value
		}
		delete(cloned, "data_log")
	}
	if credentials := cloneMap(mapValue(models, "credentials")); len(credentials) > 0 {
		for key, value := range credentials {
			if key == "provider_api_key_configured" || key == "stronghold" {
				continue
			}
			models[key] = value
		}
		delete(models, "credentials")
	}
	if len(models) > 0 {
		cloned["models"] = models
	}
	return cloned
}

func normalizeModelSettingsSection(models map[string]any) map[string]any {
	cloned := cloneMap(models)
	if cloned == nil {
		cloned = map[string]any{}
	}
	credentials := cloneMap(mapValue(cloned, "credentials"))
	if credentials == nil {
		credentials = map[string]any{}
	}
	for _, key := range []string{"budget_auto_downgrade", "base_url", "model", "budget_policy"} {
		if value, ok := cloned[key]; ok {
			credentials[key] = value
			delete(cloned, key)
		}
	}
	if len(credentials) > 0 {
		cloned["credentials"] = credentials
	}
	return cloned
}

func modelSettingsSection(settings map[string]any) map[string]any {
	return cloneMap(mapValue(normalizeSettingsSnapshot(settings), "models"))
}

func modelCredentialSettings(settings map[string]any) map[string]any {
	return cloneMap(mapValue(modelSettingsSection(settings), "credentials"))
}

func outwardSettingsUpdatePatch(settings map[string]any) map[string]any {
	cloned := normalizeSettingsSnapshot(settings)
	models := cloneMap(mapValue(cloned, "models"))
	if len(models) == 0 {
		return cloned
	}
	credentials := cloneMap(mapValue(models, "credentials"))
	delete(models, "credentials")
	for _, key := range []string{"budget_auto_downgrade", "provider_api_key_configured", "base_url", "model", "stronghold"} {
		if value, ok := credentials[key]; ok {
			models[key] = value
		}
	}
	cloned["models"] = models
	return cloned
}

func outwardSettingsUpdateKeys(internalKeys, secretUpdatedKeys []string) []string {
	seen := make(map[string]struct{}, len(internalKeys)+len(secretUpdatedKeys))
	result := make([]string, 0, len(internalKeys)+len(secretUpdatedKeys))
	for _, key := range internalKeys {
		mapped := key
		if strings.HasPrefix(mapped, "models.credentials.") {
			mapped = "models." + strings.TrimPrefix(mapped, "models.credentials.")
		}
		if _, ok := seen[mapped]; ok {
			continue
		}
		seen[mapped] = struct{}{}
		result = append(result, mapped)
	}
	for _, key := range secretUpdatedKeys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func (s *Service) providerForSettingsUpdate(models map[string]any) string {
	merged := modelSettingsSection(s.runEngine.Settings())
	if merged == nil {
		merged = map[string]any{}
	}
	for key, value := range normalizeModelSettingsSection(models) {
		merged[key] = value
	}
	return providerFromSettings(merged, s.defaultSettingsProvider())
}

func (s *Service) defaultSettingsProvider() string {
	if s.currentModel() == nil {
		return ""
	}
	return strings.TrimSpace(s.currentModel().Provider())
}

func providerFromSettings(models map[string]any, fallback string) string {
	provider := firstNonEmptyString(stringValue(models, "provider", ""), fallback)
	return model.CanonicalProviderName(provider)
}
