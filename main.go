package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/caracal-os/caracal-software-installer/internal/bootstrap"
	"github.com/caracal-os/caracal-software-installer/internal/guiapp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var embeddedFrontend embed.FS

func main() {
	loaded, err := bootstrap.Load()
	if err != nil {
		log.Fatal(err)
	}

	frontend, err := fs.Sub(embeddedFrontend, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	app := guiapp.New(loaded)
	if err := wails.Run(&options.App{
		Title:            "Caracal Software Installer",
		Width:            1440,
		Height:           920,
		MinWidth:         1080,
		MinHeight:        720,
		BackgroundColour: options.NewRGBA(24, 22, 22, 255),
		AssetServer: &assetserver.Options{
			Assets: frontend,
		},
		OnStartup: app.Startup,
		Bind: []interface{}{
			app,
		},
	}); err != nil {
		log.Fatal(err)
	}
}
