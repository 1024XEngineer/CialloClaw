package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
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

const runtimeRootEnvKey = "CIALLOCLAW_RUNTIME_ROOT"

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
	debugHTTPSet := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == "debug-http" {
			debugHTTPSet = true
		}
	})

	loadOptions := config.LoadOptions{
		DataDir:             *dataDir,
		NamedPipeName:       *namedPipe,
		DebugHTTPAddress:    *debugHTTP,
		DebugHTTPAddressSet: debugHTTPSet,
	}
	cfg := loadConfigForMain(loadOptions)
	if err := activateBootstrapRuntimeRoot(loadOptions, cfg); err != nil {
		return err
	}
	return run(ctx, cfg)
}

// activateBootstrapRuntimeRoot promotes an explicit packaged data-dir into the
// process runtime-root environment so older Default* helpers keep resolving the
// same workspace and database roots as the loaded bootstrap config.
func activateBootstrapRuntimeRoot(options config.LoadOptions, cfg config.Config) error {
	if strings.TrimSpace(options.DataDir) == "" {
		return nil
	}
	if strings.TrimSpace(cfg.DataDir) == "" {
		return nil
	}
	if err := os.Setenv(runtimeRootEnvKey, cfg.DataDir); err != nil {
		return fmt.Errorf("activate runtime root: %w", err)
	}
	return nil
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
