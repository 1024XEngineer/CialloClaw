package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/bootstrap"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

type localServiceRunner interface {
	Start(context.Context) error
}

type localServiceFactory func(config.Config) (localServiceRunner, error)

// main starts the local JSON-RPC service with optional runtime path overrides.
func main() {
	if err := run(os.Args[1:], log.Default(), func(cfg config.Config) (localServiceRunner, error) {
		return bootstrap.New(cfg)
	}); err != nil {
		log.Fatal(err)
	}
}

// run resolves runtime config, bootstraps the local service, and blocks until it exits.
func run(args []string, logger *log.Logger, factory localServiceFactory) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := buildRuntimeConfig(args)
	if err != nil {
		return fmt.Errorf("parse local service flags: %w", err)
	}

	app, err := factory(cfg)
	if err != nil {
		return fmt.Errorf("bootstrap local service: %w", err)
	}

	if logger != nil {
		logger.Printf(
			"local service transport=%s named_pipe=%s debug_http=%s data_dir=%s",
			cfg.RPC.Transport,
			cfg.RPC.NamedPipeName,
			cfg.RPC.DebugHTTPAddress,
			cfg.DataDir,
		)
	}
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("run local service: %w", err)
	}

	return nil
}

// buildRuntimeConfig parses CLI flags without mutating the global flag set.
func buildRuntimeConfig(args []string) (config.Config, error) {
	flagSet := flag.NewFlagSet("local-service", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	dataDir := flagSet.String("data-dir", "", "Path to the per-user application data directory.")
	if err := flagSet.Parse(args); err != nil {
		return config.Config{}, err
	}

	return config.Load(config.LoadOptions{DataDir: *dataDir}), nil
}
