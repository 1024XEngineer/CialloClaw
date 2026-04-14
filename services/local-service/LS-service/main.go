package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := serviceconfig.Load()
	externalCfg, err := loadExternalModelConfig(defaultExternalConfigPath)
	if err != nil {
		log.Fatalf("load LS-service model config: %v", err)
	}

	app, err := newApp(cfg, externalCfg)
	if err != nil {
		log.Fatalf("bootstrap LS-service: %v", err)
	}
	defer func() {
		if closeErr := app.Close(); closeErr != nil {
			log.Printf("close LS-service: %v", closeErr)
		}
	}()

	log.Printf(
		"LS-service transport=%s named_pipe=%s debug_http=%s provider=%s model=%s endpoint=%s external_config=%s",
		cfg.RPC.Transport,
		cfg.RPC.NamedPipeName,
		cfg.RPC.DebugHTTPAddress,
		lsServiceProvider,
		externalCfg.Model,
		externalCfg.BaseURL,
		externalCfg.ConfigPath,
	)

	if err := app.Start(ctx); err != nil {
		log.Fatalf("run LS-service: %v", err)
	}
}
