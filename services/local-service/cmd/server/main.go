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

type appStarter interface {
	Start(context.Context) error
}

var (
	loadConfigForMain = func(options config.LoadOptions) config.Config {
		return config.Load(options)
	}
	newBootstrapForMain = func(cfg config.Config) (appStarter, error) {
		return bootstrap.New(cfg)
	}
	logPrintfForMain     = log.Printf
	notifyContextForMain = signal.NotifyContext
	runMainForProcess    = runMain
	logFatalForMain      = func(err error) { log.Fatalf("local service: %v", err) }
)

func main() {
	ctx, stop := notifyContextForMain(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runMainForProcess(ctx, os.Args[1:]); err != nil {
		logFatalForMain(err)
	}
}

// runMain parses process-level bootstrap overrides before delegating to run.
func runMain(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("local-service", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	dataDir := flags.String("data-dir", "", "Path to the per-user application data directory.")
	namedPipe := flags.String("named-pipe", "", "Windows named pipe path for the local RPC transport.")
	debugHTTP := flags.String("debug-http", "", "Debug HTTP listen address for local diagnostics.")
	if err := flags.Parse(args); err != nil {
		return err
	}

	return run(ctx, loadConfigForMain(config.LoadOptions{
		DataDir:          *dataDir,
		NamedPipeName:    *namedPipe,
		DebugHTTPAddress: *debugHTTP,
	}))
}

// run owns startup wiring after config resolution so main remains the only
// process-exit boundary.
func run(ctx context.Context, cfg config.Config) error {
	app, err := newBootstrapForMain(cfg)
	if err != nil {
		return fmt.Errorf("bootstrap local service: %w", err)
	}

	logPrintfForMain(
		"local service transport=%s named_pipe=%s debug_http=%s data_dir=%s",
		cfg.RPC.Transport,
		cfg.RPC.NamedPipeName,
		cfg.RPC.DebugHTTPAddress,
		cfg.DataDir,
	)
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("run local service: %w", err)
	}
	return nil
}
