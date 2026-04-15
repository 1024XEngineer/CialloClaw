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
)

func TestNewWiresStorageBackedMemoryService(t *testing.T) {
	cfg := config.Config{
		RPC: config.RPCConfig{
			Transport:        "named_pipe",
			NamedPipeName:    `\\.\pipe\cialloclaw-rpc-test`,
			DebugHTTPAddress: ":0",
		},
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		DatabasePath:  filepath.Join(t.TempDir(), "data", "local.db"),
		Model: config.ModelConfig{
			Provider:            "openai_responses",
			ModelID:             "gpt-5.4",
			Endpoint:            "https://api.openai.com/v1/responses",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
		},
	}
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
	if app.toolRegistry.Count() != 7 {
		t.Fatalf("expected 7 tools to be registered, got %d", app.toolRegistry.Count())
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
	if app.playwright == nil {
		t.Fatal("expected playwright runtime to be wired")
	}
	if app.playwright.Available() && !app.playwright.Ready() {
		t.Fatal("expected available playwright sidecar runtime to be ready")
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
			Provider:            "openai_responses",
			ModelID:             "gpt-5.4",
			Endpoint:            "https://api.openai.com/v1/responses",
			SingleTaskLimit:     10.0,
			DailyLimit:          50.0,
			BudgetAutoDowngrade: true,
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
