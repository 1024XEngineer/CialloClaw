package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cialloclaw/internal/bootstrap"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	app, err := bootstrap.NewApp(root)
	if err != nil {
		panic(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Start(ctx); err != nil {
		panic(err)
	}
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = app.Shutdown(shutdownCtx)
}
