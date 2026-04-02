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
		Title:            "CialloClaw Demo3",
		Width:            1480,
		Height:           920,
		MinWidth:         1280,
		MinHeight:        780,
		Frameless:        true,
		DisableResize:    false,
		BackgroundColour: &options.RGBA{R: 6, G: 10, B: 18, A: 1},
		CSSDragProperty:  "--wails-draggable",
		CSSDragValue:     "drag",
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []any{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
