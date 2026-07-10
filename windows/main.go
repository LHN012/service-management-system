package main

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app, err := NewApp()
	if err != nil {
		panic(err)
	}
	err = wails.Run(&options.App{
		Title:            "Service Management System",
		Width:            1440,
		Height:           900,
		MinWidth:         1040,
		MinHeight:        680,
		Frameless:        false,
		BackgroundColour: &options.RGBA{R: 247, G: 248, B: 250, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Windows: &windowsoptions.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			Theme:                windowsoptions.SystemDefault,
		},
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
}
