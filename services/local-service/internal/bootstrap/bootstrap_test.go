package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func newBootstrapTestConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc-test`,
			DebugHTTPAddress: ":0",
		},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
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
	}
}

func seedBootstrapModelSecret(t *testing.T, cfg config.Config) {
	t.Helper()
	seed := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	if err := seed.SecretStore().PutSecret(context.Background(), storage.SecretRecord{
		Namespace: "model",
		Key:       "openai_responses_api_key",
		Value:     "test-key",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed secret store: %v", err)
	}
	_ = seed.Close()
}

func TestNewWiresStorageBackedMemoryService(t *testing.T) {
	cfg := newBootstrapTestConfig(t)
	seedBootstrapModelSecret(t, cfg)

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() { _ = app.Close() }()

	if app.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if app.storage.MemoryStore() == nil {
		t.Fatal("expected storage memory store to be available")
	}
	if app.storage.TaskRunStore() == nil {
		t.Fatal("expected storage task/run store to be available")
	}
	if app.storage.TraceStore() == nil || app.storage.EvalStore() == nil {
		t.Fatal("expected trace/eval stores to be available")
	}
	if refs, err := app.storage.CurrentExecutionAssets(context.Background()); err != nil || len(refs) != 3 {
		t.Fatalf("expected built-in execution assets to be seeded, refs=%+v err=%v", refs, err)
	}
	pluginManifests, total, err := app.storage.PluginManifestStore().ListPluginManifests(context.Background(), 10, 0)
	if err != nil || total == 0 || len(pluginManifests) == 0 {
		t.Fatalf("expected plugin manifests to be seeded, total=%d len=%d err=%v", total, len(pluginManifests), err)
	}
	capabilities := app.storage.Capabilities()
	if !capabilities.SupportsMemoryStore {
		t.Fatalf("expected storage capabilities to expose memory store: %+v", app.storage.Capabilities())
	}
	if !capabilities.SupportsRetrievalHits || !capabilities.SupportsFTS5 || !capabilities.SupportsSQLiteVecStub {
		t.Fatalf("expected retrieval and search skeleton capabilities to be exposed: %+v", capabilities)
	}
	if !capabilities.SupportsArtifactStore {
		t.Fatalf("expected artifact store capability to be exposed: %+v", capabilities)
	}
	if !capabilities.SupportsSecretStore {
		t.Fatalf("expected secret store capability to be exposed: %+v", capabilities)
	}
	if capabilities.MemoryRetrievalBackend != "sqlite_fts5+sqlite_vec" {
		t.Fatalf("expected retrieval backend to be aligned, got %+v", capabilities)
	}
	if app.toolRegistry == nil || app.toolExecutor == nil {
		t.Fatal("expected tool registry and executor to be wired")
	}
	if app.toolRegistry.Count() != 21 {
		t.Fatalf("expected 21 tools to be registered, got %d", app.toolRegistry.Count())
	}
	if _, err := app.toolRegistry.Get("generate_text"); err != nil {
		t.Fatalf("expected generate_text to be registered, got %v", err)
	}
	if _, err := app.toolRegistry.Get("read_file"); err != nil {
		t.Fatalf("expected read_file to be registered, got %v", err)
	}
	if _, err := app.toolRegistry.Get("write_file"); err != nil {
		t.Fatalf("expected write_file to be registered, got %v", err)
	}
	if _, err := app.toolRegistry.Get("list_dir"); err != nil {
		t.Fatalf("expected list_dir to be registered, got %v", err)
	}
	if _, err := app.toolRegistry.Get("exec_command"); err != nil {
		t.Fatalf("expected exec_command to be registered, got %v", err)
	}
	if _, err := app.toolRegistry.Get("page_read"); err != nil {
		t.Fatalf("expected page_read to be registered, got %v", err)
	}
	if _, err := app.toolRegistry.Get("page_search"); err != nil {
		t.Fatalf("expected page_search to be registered, got %v", err)
	}
	for _, toolName := range []string{"web_search", "page_interact", "browser_attach_current", "browser_snapshot", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact", "extract_text", "ocr_image", "ocr_pdf", "transcode_media", "extract_frames", "normalize_recording"} {
		if _, err := app.toolRegistry.Get(toolName); err != nil {
			t.Fatalf("expected %s to be registered, got %v", toolName, err)
		}
	}
	if app.playwright == nil {
		t.Fatal("expected playwright runtime to be wired")
	}
	if app.playwright.Client() == nil {
		t.Fatal("expected playwright runtime client to remain available")
	}
	if app.ocr == nil {
		t.Fatal("expected ocr runtime to be wired")
	}
	if app.media == nil {
		t.Fatal("expected media runtime to be wired")
	}
}

func TestBuildCoreDepsSeedsBootstrapInfrastructure(t *testing.T) {
	cfg := newBootstrapTestConfig(t)
	core, err := buildCoreDeps(cfg)
	if err != nil {
		t.Fatalf("buildCoreDeps returned error: %v", err)
	}
	defer func() { _ = core.storageService.Close() }()

	if core.storageService == nil || core.auditService == nil || core.checkpointService == nil {
		t.Fatalf("expected core stage to wire persistence and governance services, got %+v", core)
	}
	if core.fileSystem == nil || core.executionBackend == nil || core.osCapability == nil {
		t.Fatalf("expected core stage to wire platform adapters, got %+v", core)
	}
	if core.pluginService == nil {
		t.Fatal("expected core stage to wire plugin service")
	}
	pluginManifests, total, err := core.storageService.PluginManifestStore().ListPluginManifests(context.Background(), 10, 0)
	if err != nil || total == 0 || len(pluginManifests) == 0 {
		t.Fatalf("expected core stage to persist plugin manifests, total=%d len=%d err=%v", total, len(pluginManifests), err)
	}
}

func TestBuildRuntimesRegistersToolsAndModelRuntime(t *testing.T) {
	cfg := newBootstrapTestConfig(t)
	seedBootstrapModelSecret(t, cfg)

	core, err := buildCoreDeps(cfg)
	if err != nil {
		t.Fatalf("buildCoreDeps returned error: %v", err)
	}
	defer func() { _ = core.storageService.Close() }()

	runtimes, err := buildRuntimes(core)
	if err != nil {
		t.Fatalf("buildRuntimes returned error: %v", err)
	}
	if runtimes.modelService == nil {
		t.Fatal("expected runtime stage to resolve a model service")
	}
	if runtimes.toolRegistry == nil || runtimes.toolExecutor == nil {
		t.Fatal("expected runtime stage to wire tools")
	}
	if runtimes.toolRegistry.Count() != 21 {
		t.Fatalf("expected runtime stage to register 21 tools, got %d", runtimes.toolRegistry.Count())
	}
}

func TestNewAllowsFirstRunWithoutSeededSecret(t *testing.T) {
	baseDir, err := os.MkdirTemp("", "stronghold-first-run-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(baseDir) }()
	cfg := config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc-test`,
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
	}
	app, err := New(cfg)
	if err != nil {
		t.Fatalf("expected first run bootstrap to succeed, got %v", err)
	}
	defer func() {
		if closeErr := app.Close(); closeErr != nil {
			t.Fatalf("close app: %v", closeErr)
		}
	}()
	if app.storage == nil || app.storage.SecretStore() == nil {
		t.Fatal("expected secret store to remain wired on first run")
	}
}

func TestNewFailsFastWhenModelConfigIsInvalid(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc-test`,
			DebugHTTPAddress: ":0",
		},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model: config.ModelConfig{
			Provider:            "unsupported",
			ModelID:             "gpt-5.4",
			Endpoint:            "https://api.openai.com/v1/responses",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
	}
	_, err := New(cfg)
	if !errors.Is(err, model.ErrModelProviderUnsupported) {
		t.Fatalf("expected ErrModelProviderUnsupported, got %v", err)
	}
}

func TestChooseRuntimeOnStartKeepsFailedRuntimeState(t *testing.T) {
	failedRuntime := &stubRuntimeStarter{err: errors.New("start failed")}
	unavailableRuntime := &stubRuntimeStarter{}
	selected := chooseRuntimeOnStart[*stubRuntimeStarter](failedRuntime, nil, func() *stubRuntimeStarter {
		return unavailableRuntime
	})
	if selected != failedRuntime {
		t.Fatalf("expected start failure to keep original runtime, got %+v", selected)
	}
	selected = chooseRuntimeOnStart[*stubRuntimeStarter](failedRuntime, errors.New("build failed"), func() *stubRuntimeStarter {
		return unavailableRuntime
	})
	if selected != failedRuntime {
		t.Fatalf("expected build failure with runtime shell to keep original runtime, got %+v", selected)
	}
	selected = chooseRuntimeOnStart[*stubRuntimeStarter](nil, errors.New("build failed"), func() *stubRuntimeStarter {
		return unavailableRuntime
	})
	if selected != unavailableRuntime {
		t.Fatalf("expected nil build failure runtime to choose unavailable runtime, got %+v", selected)
	}
	successRuntime := &stubRuntimeStarter{}
	selected = chooseRuntimeOnStart[*stubRuntimeStarter](successRuntime, nil, func() *stubRuntimeStarter {
		return unavailableRuntime
	})
	if selected != successRuntime {
		t.Fatalf("expected successful runtime start to keep original runtime, got %+v", selected)
	}
}

func TestNewFailsWhenBuiltinToolRegistrationFails(t *testing.T) {
	originalRegisterBuiltinTools := registerBuiltinToolsForBootstrap
	defer func() { registerBuiltinToolsForBootstrap = originalRegisterBuiltinTools }()
	registerBuiltinToolsForBootstrap = func(*tools.Registry) error {
		return errors.New("builtin register failed")
	}
	_, err := New(config.Config{
		RPC:           config.RPCConfig{Transport: "named_pipe", NamedPipeName: `\\.\pipe\cialloclaw-rpc-builtin-fail`, DebugHTTPAddress: ":0"},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model:         config.ModelConfig{Provider: "openai_responses", ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses", SingleTaskLimit: 10.0, DailyLimit: 50.0, BudgetAutoDowngrade: true},
	})
	if err == nil || err.Error() != "builtin register failed" {
		t.Fatalf("expected builtin registration failure, got %v", err)
	}
}

func TestNewFailsWhenPlaywrightToolRegistrationFails(t *testing.T) {
	originalRegisterPlaywrightTools := registerPlaywrightToolsForBootstrap
	defer func() { registerPlaywrightToolsForBootstrap = originalRegisterPlaywrightTools }()
	registerPlaywrightToolsForBootstrap = func(*tools.Registry) error {
		return errors.New("playwright register failed")
	}
	_, err := New(config.Config{
		RPC:           config.RPCConfig{Transport: "named_pipe", NamedPipeName: `\\.\pipe\cialloclaw-rpc-playwright-fail`, DebugHTTPAddress: ":0"},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model:         config.ModelConfig{Provider: "openai_responses", ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses", SingleTaskLimit: 10.0, DailyLimit: 50.0, BudgetAutoDowngrade: true},
	})
	if err == nil || err.Error() != "playwright register failed" {
		t.Fatalf("expected playwright registration failure, got %v", err)
	}
}

func TestNewFailsWhenOCRToolRegistrationFails(t *testing.T) {
	originalRegisterOCRTools := registerOCRToolsForBootstrap
	defer func() { registerOCRToolsForBootstrap = originalRegisterOCRTools }()
	registerOCRToolsForBootstrap = func(*tools.Registry) error {
		return errors.New("ocr register failed")
	}
	_, err := New(config.Config{
		RPC:           config.RPCConfig{Transport: "named_pipe", NamedPipeName: `\\.\pipe\cialloclaw-rpc-ocr-fail`, DebugHTTPAddress: ":0"},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model:         config.ModelConfig{Provider: "openai_responses", ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses", SingleTaskLimit: 10.0, DailyLimit: 50.0, BudgetAutoDowngrade: true},
	})
	if err == nil || err.Error() != "ocr register failed" {
		t.Fatalf("expected ocr registration failure, got %v", err)
	}
}

func TestNewFailsWhenMediaToolRegistrationFails(t *testing.T) {
	originalRegisterMediaTools := registerMediaToolsForBootstrap
	defer func() { registerMediaToolsForBootstrap = originalRegisterMediaTools }()
	registerMediaToolsForBootstrap = func(*tools.Registry) error {
		return errors.New("media register failed")
	}
	_, err := New(config.Config{
		RPC:           config.RPCConfig{Transport: "named_pipe", NamedPipeName: `\\.\pipe\cialloclaw-rpc-media-fail`, DebugHTTPAddress: ":0"},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model:         config.ModelConfig{Provider: "openai_responses", ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses", SingleTaskLimit: 10.0, DailyLimit: 50.0, BudgetAutoDowngrade: true},
	})
	if err == nil || err.Error() != "media register failed" {
		t.Fatalf("expected media registration failure, got %v", err)
	}
}

func TestNewPropagatesPathPolicyErrors(t *testing.T) {
	originalNewLocalPathPolicy := newLocalPathPolicyForBootstrap
	defer func() { newLocalPathPolicyForBootstrap = originalNewLocalPathPolicy }()
	newLocalPathPolicyForBootstrap = func(string) (*platform.LocalPathPolicy, error) {
		return nil, errors.New("path policy failed")
	}
	_, err := New(config.Config{
		RPC:           config.RPCConfig{Transport: "named_pipe", NamedPipeName: `\\.\pipe\cialloclaw-rpc-path-policy-fail`, DebugHTTPAddress: ":0"},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model:         config.ModelConfig{Provider: "openai_responses", ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses", SingleTaskLimit: 10.0, DailyLimit: 50.0, BudgetAutoDowngrade: true},
	})
	if err == nil || err.Error() != "path policy failed" {
		t.Fatalf("expected path policy failure, got %v", err)
	}
}

func TestNewFailsWhenWorkspaceRootIsInvalid(t *testing.T) {
	_, err := New(config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc-invalid-workspace`,
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

type stubRuntimeStarter struct{ err error }

func (s *stubRuntimeStarter) Start() error { return s.err }
