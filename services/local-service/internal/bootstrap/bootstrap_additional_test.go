package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestMigrateLegacyRuntimeDefaultsIfNeededCopiesLegacyWorkspaceAndDatabase(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	legacyRoot := t.TempDir()
	legacyWorkspaceRoot := filepath.Join(legacyRoot, "workspace")
	legacyDatabasePath := filepath.Join(legacyRoot, "data", "cialloclaw.db")
	legacySecretPath := secretStorePathForDatabase(legacyDatabasePath)
	if err := os.MkdirAll(filepath.Join(legacyWorkspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("mkdir legacy workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyDatabasePath), 0o755); err != nil {
		t.Fatalf("mkdir legacy database dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyWorkspaceRoot, "todos", "inbox.md"), []byte("- [ ] migrate me\n"), 0o644); err != nil {
		t.Fatalf("write legacy workspace file: %v", err)
	}
	if err := os.WriteFile(legacyDatabasePath, []byte("legacy-db"), 0o644); err != nil {
		t.Fatalf("write legacy database file: %v", err)
	}
	if err := os.WriteFile(legacySecretPath, []byte("legacy-secret"), 0o644); err != nil {
		t.Fatalf("write legacy secret store file: %v", err)
	}

	cfg := config.Load()
	if err := migrateLegacyRuntimeDefaultsIfNeeded(cfg, []string{legacyRoot}); err != nil {
		t.Fatalf("migrateLegacyRuntimeDefaultsIfNeeded returned error: %v", err)
	}
	if migratedWorkspace, err := os.ReadFile(filepath.Join(cfg.WorkspaceRoot, "todos", "inbox.md")); err != nil || string(migratedWorkspace) != "- [ ] migrate me\n" {
		t.Fatalf("expected migrated workspace file, content=%q err=%v", string(migratedWorkspace), err)
	}
	if migratedDatabase, err := os.ReadFile(cfg.DatabasePath); err != nil || string(migratedDatabase) != "legacy-db" {
		t.Fatalf("expected migrated database file, content=%q err=%v", string(migratedDatabase), err)
	}
	if migratedSecret, err := os.ReadFile(secretStorePathForDatabase(cfg.DatabasePath)); err != nil || string(migratedSecret) != "legacy-secret" {
		t.Fatalf("expected migrated secret store file, content=%q err=%v", string(migratedSecret), err)
	}
	if err := migrateLegacyRuntimeDefaultsIfNeeded(cfg, []string{legacyRoot}); err != nil {
		t.Fatalf("expected repeated migration to stay idempotent, got %v", err)
	}
}

func TestBuildRuntimeMigrationPlanSkipsCustomAndMissingRoots(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	defaultCfg := config.Load()
	if _, ok := buildRuntimeMigrationPlan(defaultCfg, []string{t.TempDir()}); ok {
		t.Fatal("expected missing legacy roots to skip migration plan")
	}
	customCfg := defaultCfg
	customCfg.WorkspaceRoot = filepath.Join(t.TempDir(), "custom-workspace")
	if _, ok := buildRuntimeMigrationPlan(customCfg, []string{t.TempDir()}); ok {
		t.Fatal("expected custom workspace root to skip default-path migration")
	}
	legacyRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(legacyRoot, "workspace"), 0o755); err != nil {
		t.Fatalf("mkdir legacy workspace: %v", err)
	}
	plan, ok := buildRuntimeMigrationPlan(defaultCfg, []string{"", legacyRoot, legacyRoot})
	if !ok {
		t.Fatal("expected legacy runtime plan to be created")
	}
	if plan.legacyWorkspaceRoot != filepath.Join(legacyRoot, "workspace") || plan.targetWorkspaceRoot != defaultCfg.WorkspaceRoot {
		t.Fatalf("unexpected migration plan: %+v", plan)
	}
	if plan.targetSecretPath != secretStorePathForDatabase(defaultCfg.DatabasePath) {
		t.Fatalf("expected secret-store target path to follow database path, got %+v", plan)
	}
}

func TestBuildRuntimeMigrationPlanSkipsRootsMatchingCurrentRuntimeTargets(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	defaultCfg := config.Load()
	currentRuntimeRoot := filepath.Dir(defaultCfg.WorkspaceRoot)
	if err := os.MkdirAll(defaultCfg.WorkspaceRoot, 0o755); err != nil {
		t.Fatalf("mkdir current runtime workspace: %v", err)
	}
	if _, ok := buildRuntimeMigrationPlan(defaultCfg, []string{currentRuntimeRoot}); ok {
		t.Fatal("expected migration plan to skip roots that already match the current runtime paths")
	}
}

func TestCopyDirectoryIfMissingRetriesIntoPartiallyCreatedWorkspace(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "legacy-workspace")
	targetRoot := filepath.Join(t.TempDir(), "runtime-workspace")
	if err := os.MkdirAll(filepath.Join(sourceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("mkdir source workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "todos", "inbox.md"), []byte("- [ ] recover partial migration\n"), 0o644); err != nil {
		t.Fatalf("write source workspace file: %v", err)
	}
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		t.Fatalf("mkdir target workspace: %v", err)
	}

	if err := copyDirectoryIfMissing(sourceRoot, targetRoot); err != nil {
		t.Fatalf("copyDirectoryIfMissing returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(targetRoot, "todos", "inbox.md"))
	if err != nil || string(content) != "- [ ] recover partial migration\n" {
		t.Fatalf("expected retry-safe directory copy to fill missing file, content=%q err=%v", string(content), err)
	}
}

func TestCopyDirectoryIfMissingSkipsMissingSource(t *testing.T) {
	if err := copyDirectoryIfMissing(filepath.Join(t.TempDir(), "missing-source"), filepath.Join(t.TempDir(), "target")); err != nil {
		t.Fatalf("expected missing source directory copy to noop, got %v", err)
	}
}

func TestCopyFileContentsHandlesExistingTargetsAndZeroPermissionMode(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.md")
	targetDir := filepath.Join(t.TempDir(), "target")
	targetPath := filepath.Join(targetDir, "copied.md")
	if err := os.WriteFile(sourcePath, []byte("source-note"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := copyFileContents(sourcePath, targetPath, os.ModeDevice); err != nil {
		t.Fatalf("copyFileContents returned error: %v", err)
	}
	metadata, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat target file: %v", err)
	}
	if metadata.Mode().Perm() == 0 {
		t.Fatalf("expected zero-permission mode to fall back to 0644, got %v", metadata.Mode().Perm())
	}
	if err := copyFileContents(sourcePath, targetPath, 0); err != nil {
		t.Fatalf("expected existing target copy to stay idempotent, got %v", err)
	}
	content, err := os.ReadFile(targetPath)
	if err != nil || string(content) != "source-note" {
		t.Fatalf("expected target content to remain intact, content=%q err=%v", string(content), err)
	}
}

func TestCopyFileIfMissingAndSecretStorePathHelpersCoverCompatibilityBranches(t *testing.T) {
	if secretStorePathForDatabase("") != "" {
		t.Fatal("expected blank database path to keep empty secret-store path")
	}
	if secretStorePathForDatabase("runtime/data/cialloclaw") != "runtime/data/cialloclaw.stronghold.db" {
		t.Fatal("expected extensionless database path to receive stronghold suffix")
	}
	if secretStorePathForDatabase("runtime/data/cialloclaw.db") != "runtime/data/cialloclaw.stronghold.db" {
		t.Fatal("expected database extension to be rewritten with stronghold suffix")
	}

	sourcePath := filepath.Join(t.TempDir(), "source.md")
	targetPath := filepath.Join(t.TempDir(), "target", "copied.md")
	if err := copyFileIfMissing(sourcePath, targetPath); err != nil {
		t.Fatalf("expected missing source copy to noop, got %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("bootstrap"), 0o644); err != nil {
		t.Fatalf("write helper source file: %v", err)
	}
	if err := copyFileIfMissing(sourcePath, targetPath); err != nil {
		t.Fatalf("copyFileIfMissing returned error: %v", err)
	}
	if err := copyFileIfMissing(sourcePath, targetPath); err != nil {
		t.Fatalf("expected repeated copyFileIfMissing call to noop, got %v", err)
	}
	content, err := os.ReadFile(targetPath)
	if err != nil || string(content) != "bootstrap" {
		t.Fatalf("expected helper target file to remain copied, content=%q err=%v", string(content), err)
	}
}

func TestLegacyRuntimeRootsForCompatibilityUsesExecutableDirectoryOnly(t *testing.T) {
	legacyExecutableRoot := filepath.Join(t.TempDir(), "legacy-exe")
	originalExecutable := getExecutablePathForBootstrap
	defer func() {
		getExecutablePathForBootstrap = originalExecutable
	}()
	getExecutablePathForBootstrap = func() (string, error) {
		return filepath.Join(legacyExecutableRoot, "local-service.exe"), nil
	}

	roots := legacyRuntimeRootsForCompatibility()
	if len(roots) != 1 || roots[0] != filepath.Clean(legacyExecutableRoot) {
		t.Fatalf("expected compatibility roots to use executable directory only, got %+v", roots)
	}
}

func TestLegacyRuntimeRootsForCompatibilityReturnsEmptyWhenExecutableUnavailable(t *testing.T) {
	originalExecutable := getExecutablePathForBootstrap
	defer func() {
		getExecutablePathForBootstrap = originalExecutable
	}()
	getExecutablePathForBootstrap = func() (string, error) {
		return "", errors.New("executable path unavailable")
	}

	if roots := legacyRuntimeRootsForCompatibility(); len(roots) != 0 {
		t.Fatalf("expected empty compatibility roots when executable path is unavailable, got %+v", roots)
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

func TestLoadBootstrapModelConfigPreservesPersistedProviderForPlaceholderFallback(t *testing.T) {
	base := config.ModelConfig{
		Provider: model.OpenAIResponsesProvider,
		ModelID:  "gpt-bootstrap-default",
		Endpoint: "https://api.openai.com/v1/responses",
	}
	service := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "bootstrap-load-config.db")))
	defer func() { _ = service.Close() }()

	if err := service.SettingsStore().SaveSettingsSnapshot(context.Background(), map[string]any{
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

	resolved, placeholder, persistedChanged, err := loadBootstrapModelConfig(base, service.SettingsStore())
	if err != nil {
		t.Fatalf("loadBootstrapModelConfig returned error: %v", err)
	}
	if !persistedChanged {
		t.Fatal("expected persisted route change to be detected")
	}
	if resolved.Provider != model.OpenAIResponsesProvider || resolved.Endpoint != "https://example.invalid/v1/messages" || resolved.ModelID != "claude-3-7-sonnet" {
		t.Fatalf("expected resolved runtime route to stay canonical, got %+v", resolved)
	}
	if placeholder.Provider != "anthropic" || placeholder.Endpoint != "https://example.invalid/v1/messages" || placeholder.ModelID != "claude-3-7-sonnet" {
		t.Fatalf("expected placeholder route to preserve persisted provider identity, got %+v", placeholder)
	}
}

func TestLoadBootstrapModelConfigWithoutSettingsStoreKeepsBaseConfig(t *testing.T) {
	base := config.ModelConfig{
		Provider: model.OpenAIResponsesProvider,
		ModelID:  "gpt-bootstrap-default",
		Endpoint: "https://api.openai.com/v1/responses",
	}

	resolved, placeholder, persistedChanged, err := loadBootstrapModelConfig(base, nil)
	if err != nil {
		t.Fatalf("loadBootstrapModelConfig returned error: %v", err)
	}
	if persistedChanged {
		t.Fatal("expected nil settings store not to report persisted route changes")
	}
	if !reflect.DeepEqual(resolved, base) {
		t.Fatalf("expected resolved config to keep base config, got %+v want %+v", resolved, base)
	}
	if !reflect.DeepEqual(placeholder, base) {
		t.Fatalf("expected placeholder config to keep base config, got %+v want %+v", placeholder, base)
	}
}

func TestBootstrapPersistedModelProviderReadsCredentialsFallback(t *testing.T) {
	provider := bootstrapPersistedModelProvider(map[string]any{
		"models": map[string]any{
			"credentials": map[string]any{
				"provider": "  custom_gateway  ",
			},
		},
	})
	if provider != "custom_gateway" {
		t.Fatalf("expected provider fallback from credentials, got %q", provider)
	}
}

func TestBootstrapPersistedModelProviderRejectsMissingModelsScope(t *testing.T) {
	if provider := bootstrapPersistedModelProvider(map[string]any{"general": map[string]any{"language": "zh-CN"}}); provider != "" {
		t.Fatalf("expected missing models scope to return empty provider, got %q", provider)
	}
	if provider := bootstrapPersistedModelProvider(map[string]any{"models": map[string]any{"provider": 123}}); provider != "" {
		t.Fatalf("expected non-string provider to be ignored, got %q", provider)
	}
}

func TestShouldFallbackBootstrapModelServiceHonorsPersistedProviderGate(t *testing.T) {
	unsupportedErr := model.ErrModelProviderUnsupported
	if shouldFallbackBootstrapModelService(unsupportedErr, false) {
		t.Fatal("expected unsupported provider without persisted route change not to fallback")
	}
	if !shouldFallbackBootstrapModelService(unsupportedErr, true) {
		t.Fatal("expected persisted unsupported provider to fallback into placeholder")
	}
	if shouldFallbackBootstrapModelService(errors.New("boom"), true) {
		t.Fatal("expected unrelated bootstrap error not to fallback")
	}
}
