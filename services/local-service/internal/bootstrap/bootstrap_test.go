package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
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
			Provider: "openai_responses",
			ModelID:  "gpt-5.4",
			Endpoint: "https://api.openai.com/v1/responses",
		},
	}

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
	capabilities := app.storage.Capabilities()
	if !capabilities.SupportsMemoryStore {
		t.Fatalf("expected storage capabilities to expose memory store: %+v", app.storage.Capabilities())
	}
	if !capabilities.SupportsRetrievalHits || !capabilities.SupportsFTS5 || !capabilities.SupportsSQLiteVecStub {
		t.Fatalf("expected retrieval and search skeleton capabilities to be exposed: %+v", capabilities)
	}
	if capabilities.MemoryRetrievalBackend != "sqlite_fts5+sqlite_vec" {
		t.Fatalf("expected retrieval backend to be aligned, got %+v", capabilities)
	}
	if app.toolRegistry == nil || app.toolExecutor == nil {
		t.Fatal("expected tool registry and executor to be wired")
	}
	if app.toolRegistry.Count() != 4 {
		t.Fatalf("expected 4 builtin tools to be registered, got %d", app.toolRegistry.Count())
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
}
