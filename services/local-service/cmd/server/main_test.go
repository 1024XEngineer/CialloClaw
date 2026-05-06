package main

import (
	"context"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

func TestRunReturnsBootstrapErrorWithContext(t *testing.T) {
	err := run(context.Background(), config.Config{
		RPC:           config.RPCConfig{Transport: "named_pipe", NamedPipeName: `\\.\pipe\cialloclaw-rpc-test`, DebugHTTPAddress: ":0"},
		WorkspaceRoot: "invalid\x00workspace",
		DatabasePath:  t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected bootstrap error")
	}
	if !strings.Contains(err.Error(), "bootstrap local service:") {
		t.Fatalf("expected bootstrap context, got %v", err)
	}
	if !strings.Contains(err.Error(), "workspace root contains invalid null byte") {
		t.Fatalf("expected workspace validation error, got %v", err)
	}
}
