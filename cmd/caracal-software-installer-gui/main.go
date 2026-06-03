package main

import (
	"log"

	"github.com/caracal-dev/caracal-software-installer/internal/bootstrap"
	"github.com/caracal-dev/caracal-software-installer/internal/guiapp"
	"github.com/caracal-dev/caracal-software-installer/internal/guiassets"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	loaded, err := bootstrap.Load()
	if err != nil {
		log.Fatal(err)
	}

	frontend, err := guiassets.FrontendFS()
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
