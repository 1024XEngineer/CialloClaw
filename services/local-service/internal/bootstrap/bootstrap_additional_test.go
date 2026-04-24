package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

type failingPluginManifestStore struct{ err error }

func (s failingPluginManifestStore) WritePluginManifest(context.Context, storage.PluginManifestRecord) error {
	return s.err
}

func (s failingPluginManifestStore) GetPluginManifest(context.Context, string) (storage.PluginManifestRecord, error) {
	return storage.PluginManifestRecord{}, s.err
}

func (s failingPluginManifestStore) ListPluginManifests(context.Context, int, int) ([]storage.PluginManifestRecord, int, error) {
	return nil, 0, s.err
}

// replacePluginManifestStore swaps the composed storage dependency so bootstrap
// tests can exercise persistPluginManifests failure handling without changing
// the production assembly path.
func replacePluginManifestStore(t *testing.T, service *storage.Service, store storage.PluginManifestStore) {
	t.Helper()
	serviceValue := reflect.ValueOf(service).Elem()
	field := serviceValue.FieldByName("pluginManifestStore")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(store))
}

func appRuntimeModelService(t *testing.T, app *App) *model.Service {
	t.Helper()
	if app == nil || app.server == nil {
		t.Fatal("expected app server to be wired")
	}
	serverValue := reflect.ValueOf(app.server).Elem()
	orchestratorField := serverValue.FieldByName("orchestrator")
	orchestratorValue := reflect.NewAt(orchestratorField.Type(), unsafe.Pointer(orchestratorField.UnsafeAddr())).Elem()
	if orchestratorValue.IsNil() {
		t.Fatal("expected bootstrap orchestrator to be wired")
	}
	serviceValue := orchestratorValue.Elem()
	modelField := serviceValue.FieldByName("model")
	modelValue := reflect.NewAt(modelField.Type(), unsafe.Pointer(modelField.UnsafeAddr())).Elem()
	if modelValue.IsNil() {
		return nil
	}
	service, _ := modelValue.Interface().(*model.Service)
	return service
}

func TestPersistPluginManifestsHandlesNilAndSuccessPaths(t *testing.T) {
	if err := persistPluginManifests(context.Background(), nil, nil); err != nil {
		t.Fatalf("expected nil dependencies to be ignored, got %v", err)
	}
	service := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "plugin-manifests.db")))
	defer func() { _ = service.Close() }()
	if err := persistPluginManifests(context.Background(), service, plugin.NewService()); err != nil {
		t.Fatalf("persistPluginManifests returned error: %v", err)
	}
	items, total, err := service.PluginManifestStore().ListPluginManifests(context.Background(), 10, 0)
	if err != nil || total == 0 || len(items) == 0 {
		t.Fatalf("expected plugin manifests to be persisted, total=%d len=%d err=%v", total, len(items), err)
	}
	var runtimeNames []string
	if err := json.Unmarshal([]byte(items[0].RuntimeNamesJSON), &runtimeNames); err != nil || len(runtimeNames) == 0 {
		t.Fatalf("expected persisted plugin manifests to include runtime names, names=%+v err=%v", runtimeNames, err)
	}
	if items[0].Summary == "" {
		t.Fatalf("expected persisted plugin manifests to keep manifest summaries, item=%+v", items[0])
	}
	if err := persistPluginManifests(context.Background(), service, &plugin.Service{}); err != nil {
		t.Fatalf("expected empty plugin registry to be ignored, got %v", err)
	}
}

func TestAppStartAndCloseHandleLifecycle(t *testing.T) {
	baseDir := t.TempDir()
	app, err := New(config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-bootstrap-start-test`,
			DebugHTTPAddress: ":0",
		},
		WorkspaceRoot: filepath.Join(baseDir, "workspace"),
		DatabasePath:  filepath.Join(baseDir, "data", "local.db"),
		Model: config.ModelConfig{
			Provider:             "openai_responses",
			ModelID:              "gpt-5.4",
			Endpoint:             "https://api.openai.com/v1/responses",
			SingleTaskLimit:      10.0,
			DailyLimit:           50.0,
			BudgetAutoDowngrade:  true,
			MaxToolIterations:    4,
			ContextCompressChars: 2400,
			ContextKeepRecent:    4,
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("Start returned error for canceled context: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := (&App{}).Close(); err != nil {
		t.Fatalf("Close on empty app returned error: %v", err)
	}
	// Give the debug HTTP server shutdown path a brief chance to settle before the
	// next test reuses the process resources.
	time.Sleep(10 * time.Millisecond)
}

func TestNewFailsWhenWorkspaceRootIsInvalidAdditional(t *testing.T) {
	_, err := New(config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-bootstrap-invalid-workspace`,
			DebugHTTPAddress: ":0",
		},
		WorkspaceRoot: string([]byte{'b', 'a', 'd', 0, 'r', 'o', 'o', 't'}),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model: config.ModelConfig{
			Provider:            "openai_responses",
			ModelID:             "gpt-5.4",
			Endpoint:            "https://api.openai.com/v1/responses",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
	})
	if err == nil {
		t.Fatal("expected invalid workspace root to fail bootstrap")
	}
}

func TestPersistPluginManifestsPropagatesStoreWriteFailures(t *testing.T) {
	service := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "plugin-manifests-fail.db")))
	defer func() { _ = service.Close() }()
	writeErr := errors.New("plugin manifest write failed")
	originalStore := service.PluginManifestStore()
	defer func() {
		if closer, ok := originalStore.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()
	replacePluginManifestStore(t, service, failingPluginManifestStore{err: writeErr})

	err := persistPluginManifests(context.Background(), service, plugin.NewService())
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected persistPluginManifests to return wrapped store error, got %v", err)
	}
}

func TestNewModelServiceFallbackAndFailureBranches(t *testing.T) {
	baseConfig := func(baseDir string) config.Config {
		return config.Config{
			RPC: config.RPCConfig{
				Transport:        "named_pipe",
				NamedPipeName:    `\\.\pipe\cialloclaw-bootstrap-model-branches`,
				DebugHTTPAddress: ":0",
			},
			WorkspaceRoot: filepath.Join(baseDir, "workspace"),
			DatabasePath:  filepath.Join(baseDir, "data", "local.db"),
			Model: config.ModelConfig{
				Provider:            "openai_responses",
				ModelID:             "gpt-5.4",
				Endpoint:            "https://api.openai.com/v1/responses",
				SingleTaskLimit:     10.0,
				DailyLimit:          50.0,
				BudgetAutoDowngrade: true,
			},
		}
	}
	originalNewModelService := newModelServiceFromConfigForBootstrap
	defer func() { newModelServiceFromConfigForBootstrap = originalNewModelService }()

	fallbackErrors := []error{
		model.ErrSecretNotFound,
		storage.ErrSecretNotFound,
		storage.ErrSecretStoreAccessFailed,
		storage.ErrStrongholdUnavailable,
		storage.ErrStrongholdAccessFailed,
	}
	for index, missingSecretErr := range fallbackErrors {
		newModelServiceFromConfigForBootstrap = func(model.ServiceConfig) (*model.Service, error) {
			return nil, fmt.Errorf("%w: %w", model.ErrSecretSourceFailed, missingSecretErr)
		}
		app, err := New(baseConfig(filepath.Join(t.TempDir(), fmt.Sprintf("fallback-%d", index))))
		if err != nil {
			t.Fatalf("expected bootstrap fallback to succeed for %v, got %v", missingSecretErr, err)
		}
		if err := app.Close(); err != nil {
			t.Fatalf("close app after secret fallback failed: %v", err)
		}
	}

	genericErr := errors.New("model bootstrap failed")
	newModelServiceFromConfigForBootstrap = func(model.ServiceConfig) (*model.Service, error) {
		return nil, genericErr
	}
	if _, err := New(baseConfig(filepath.Join(t.TempDir(), "generic-error"))); !errors.Is(err, genericErr) {
		t.Fatalf("expected generic model bootstrap error to propagate, got %v", err)
	}

	secretSourceErr := errors.New("secret backend unavailable")
	newModelServiceFromConfigForBootstrap = func(model.ServiceConfig) (*model.Service, error) {
		return nil, fmt.Errorf("%w: %w", model.ErrSecretSourceFailed, secretSourceErr)
	}
	if _, err := New(baseConfig(filepath.Join(t.TempDir(), "secret-source-error"))); !errors.Is(err, secretSourceErr) {
		t.Fatalf("expected non-missing secret source failure to propagate, got %v", err)
	}
}

func TestNewUsesPersistedModelSettingsForRuntimeModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_bootstrap","output_text":"persisted model ok","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	baseDir := t.TempDir()
	cfg := config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-bootstrap-persisted-model`,
			DebugHTTPAddress: ":0",
		},
		WorkspaceRoot: filepath.Join(baseDir, "workspace"),
		DatabasePath:  filepath.Join(baseDir, "data", "local.db"),
		Model: config.ModelConfig{
			Provider:             model.OpenAIResponsesProvider,
			ModelID:              "gpt-bootstrap-default",
			Endpoint:             "https://invalid.example/v1/responses",
			SingleTaskLimit:      10.0,
			DailyLimit:           50.0,
			BudgetAutoDowngrade:  true,
			MaxToolIterations:    4,
			PlannerRetryBudget:   1,
			ToolRetryBudget:      1,
			ContextCompressChars: 2400,
			ContextKeepRecent:    4,
		},
	}
	seed := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	if err := seed.SettingsStore().SaveSettingsSnapshot(context.Background(), map[string]any{
		"models": map[string]any{
			"provider": "openai",
			"credentials": map[string]any{
				"base_url": server.URL,
				"model":    "gpt-persisted",
			},
		},
	}); err != nil {
		t.Fatalf("seed settings snapshot failed: %v", err)
	}
	if err := seed.SecretStore().PutSecret(context.Background(), storage.SecretRecord{
		Namespace: "model",
		Key:       "openai_responses_api_key",
		Value:     "persisted-secret-key",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed secret store failed: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed storage failed: %v", err)
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("bootstrap returned error: %v", err)
	}
	defer func() { _ = app.Close() }()

	runtimeModel := appRuntimeModelService(t, app)
	if runtimeModel == nil {
		t.Fatal("expected bootstrap runtime model to be wired")
	}
	configSnapshot := runtimeModel.RuntimeConfig()
	if configSnapshot.Provider != model.OpenAIResponsesProvider || configSnapshot.Endpoint != server.URL || configSnapshot.ModelID != "gpt-persisted" {
		t.Fatalf("expected bootstrap runtime model to honor persisted settings, got %+v", configSnapshot)
	}
	response, err := runtimeModel.GenerateText(context.Background(), model.GenerateTextRequest{Input: "hello"})
	if err != nil {
		t.Fatalf("expected persisted runtime model to build a working client, got %v", err)
	}
	if response.OutputText != "persisted model ok" {
		t.Fatalf("expected persisted runtime model to use stored endpoint, got %+v", response)
	}
}

func TestNewFallsBackToPlaceholderWhenPersistedProviderIsUnsupported(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-bootstrap-unsupported-model`,
			DebugHTTPAddress: ":0",
		},
		WorkspaceRoot: filepath.Join(baseDir, "workspace"),
		DatabasePath:  filepath.Join(baseDir, "data", "local.db"),
		Model: config.ModelConfig{
			Provider:             model.OpenAIResponsesProvider,
			ModelID:              "gpt-bootstrap-default",
			Endpoint:             "https://api.openai.com/v1/responses",
			SingleTaskLimit:      10.0,
			DailyLimit:           50.0,
			BudgetAutoDowngrade:  true,
			MaxToolIterations:    4,
			PlannerRetryBudget:   1,
			ToolRetryBudget:      1,
			ContextCompressChars: 2400,
			ContextKeepRecent:    4,
		},
	}
	seed := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	if err := seed.SettingsStore().SaveSettingsSnapshot(context.Background(), map[string]any{
		"models": map[string]any{
			"provider": "anthropic",
			"credentials": map[string]any{
				"base_url": "https://example.invalid/v1/messages",
				"model":    "claude-3-7-sonnet",
			},
		},
	}); err != nil {
		t.Fatalf("seed settings snapshot failed: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed storage failed: %v", err)
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("expected bootstrap to fall back for persisted unsupported provider, got %v", err)
	}
	defer func() { _ = app.Close() }()

	runtimeModel := appRuntimeModelService(t, app)
	if runtimeModel == nil {
		t.Fatal("expected placeholder runtime model to be wired")
	}
	configSnapshot := runtimeModel.RuntimeConfig()
	if configSnapshot.Provider != "anthropic" || configSnapshot.Endpoint != "https://example.invalid/v1/messages" || configSnapshot.ModelID != "claude-3-7-sonnet" {
		t.Fatalf("expected placeholder runtime model to preserve persisted unsupported provider config, got %+v", configSnapshot)
	}
	if _, err := runtimeModel.GenerateText(context.Background(), model.GenerateTextRequest{Input: "hello"}); !errors.Is(err, model.ErrClientNotConfigured) {
		t.Fatalf("expected placeholder runtime model to remain clientless, got %v", err)
	}
}
