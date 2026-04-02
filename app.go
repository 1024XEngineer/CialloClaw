package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {}

func (a *App) Hide() {
	if a.ctx == nil {
		return
	}
	runtime.Hide(a.ctx)
}

func (a *App) Show() {
	if a.ctx == nil {
		return
	}
	runtime.Show(a.ctx)
}

func (a *App) Quit() {
	if a.ctx == nil {
		return
	}
	runtime.Quit(a.ctx)
}
