package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/bootstrap"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	app, err := bootstrap.New(cfg)
	if err != nil {
		log.Fatalf("bootstrap local service: %v", err)
	}

	log.Printf("local service listening on %s", cfg.RPCAddress)
	if err := app.Start(ctx); err != nil {
		log.Fatalf("run local service: %v", err)
	}
}
