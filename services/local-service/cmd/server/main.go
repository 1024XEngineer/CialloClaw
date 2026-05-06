package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/bootstrap"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, config.Load()); err != nil {
		log.Fatalf("local service: %v", err)
	}
}

// run owns startup wiring so main remains the only process-exit boundary.
func run(ctx context.Context, cfg config.Config) error {
	app, err := bootstrap.New(cfg)
	if err != nil {
		return fmt.Errorf("bootstrap local service: %w", err)
	}

	log.Printf(
		"local service transport=%s named_pipe=%s debug_http=%s",
		cfg.RPC.Transport,
		cfg.RPC.NamedPipeName,
		cfg.RPC.DebugHTTPAddress,
	)
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("run local service: %w", err)
	}
	return nil
}
