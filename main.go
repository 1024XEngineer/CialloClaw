package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "CialloClaw Demo4",
		Width:             260,
		Height:            260,
		MinWidth:          220,
		MinHeight:         220,
		Frameless:         true,
		DisableResize:     false,
		StartHidden:       false,
		HideWindowOnClose: true,
		AlwaysOnTop:       true,
		BackgroundColour:  options.NewRGBA(0, 0, 0, 0),
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup:         app.startup,
		OnShutdown:        app.shutdown,
		Bind:              []any{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
