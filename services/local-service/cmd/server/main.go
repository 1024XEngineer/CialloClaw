package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/bootstrap"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

// main starts the local JSON-RPC service and accepts host-provided runtime
// path overrides so packaged desktop builds can keep mutable data outside the
// installed application directory.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dataDir := flag.String("data-dir", "", "Path to the per-user application data directory.")
	namedPipe := flag.String("named-pipe", "", "Windows named pipe path for the local RPC transport.")
	debugHTTP := flag.String("debug-http", "", "Debug HTTP listen address for local diagnostics.")
	flag.Parse()

	cfg := config.Load(config.LoadOptions{
		DataDir:          *dataDir,
		NamedPipeName:    *namedPipe,
		DebugHTTPAddress: *debugHTTP,
	})
	app, err := bootstrap.New(cfg)
	if err != nil {
		log.Fatalf("bootstrap local service: %v", err)
	}

	log.Printf(
		"local service transport=%s named_pipe=%s debug_http=%s data_dir=%s",
		cfg.RPC.Transport,
		cfg.RPC.NamedPipeName,
		cfg.RPC.DebugHTTPAddress,
		cfg.DataDir,
	)
	if err := app.Start(ctx); err != nil {
		log.Fatalf("run local service: %v", err)
	}
}
