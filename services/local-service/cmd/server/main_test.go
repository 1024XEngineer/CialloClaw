package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

type stubLocalServiceRunner struct {
	started  bool
	startErr error
}

func (s *stubLocalServiceRunner) Start(context.Context) error {
	s.started = true
	return s.startErr
}

func TestBuildRuntimeConfigUsesExplicitDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "desktop-data")
	cfg, err := buildRuntimeConfig([]string{"--data-dir", dataDir})
	if err != nil {
		t.Fatalf("buildRuntimeConfig returned error: %v", err)
	}

	if cfg.DataDir != filepath.Clean(dataDir) {
		t.Fatalf("expected data dir %q, got %q", filepath.Clean(dataDir), cfg.DataDir)
	}
	if cfg.WorkspaceRoot != filepath.Join(filepath.Clean(dataDir), "workspace") {
		t.Fatalf("expected workspace root to derive from data dir, got %q", cfg.WorkspaceRoot)
	}
	if cfg.DatabasePath != filepath.Join(filepath.Clean(dataDir), "data", "cialloclaw.db") {
		t.Fatalf("expected database path to derive from data dir, got %q", cfg.DatabasePath)
	}
}

func TestBuildRuntimeConfigRejectsUnknownFlags(t *testing.T) {
	if _, err := buildRuntimeConfig([]string{"--unknown"}); err == nil {
		t.Fatal("expected buildRuntimeConfig to reject unknown flags")
	}
}

func TestRunBootstrapsAndStartsLocalService(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "desktop-data")
	runner := &stubLocalServiceRunner{}
	var logged bytes.Buffer
	logger := log.New(&logged, "", 0)

	err := run([]string{"--data-dir", dataDir}, logger, func(cfg config.Config) (localServiceRunner, error) {
		if cfg.DataDir != filepath.Clean(dataDir) {
			t.Fatalf("expected run to pass parsed data dir %q, got %q", filepath.Clean(dataDir), cfg.DataDir)
		}
		return runner, nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !runner.started {
		t.Fatal("expected run to start the local service")
	}
	if !strings.Contains(logged.String(), "data_dir=") {
		t.Fatalf("expected startup log to include data dir, got %q", logged.String())
	}
}

func TestRunWrapsBootstrapErrors(t *testing.T) {
	bootstrapErr := errors.New("bootstrap failed")
	err := run(nil, nil, func(config.Config) (localServiceRunner, error) {
		return nil, bootstrapErr
	})
	if !errors.Is(err, bootstrapErr) {
		t.Fatalf("expected run to wrap bootstrap error %v, got %v", bootstrapErr, err)
	}
}

func TestRunWrapsStartErrors(t *testing.T) {
	startErr := errors.New("start failed")
	err := run(nil, nil, func(config.Config) (localServiceRunner, error) {
		return &stubLocalServiceRunner{startErr: startErr}, nil
	})
	if !errors.Is(err, startErr) {
		t.Fatalf("expected run to wrap start error %v, got %v", startErr, err)
	}
}
