package main

import (
	"log"

	"github.com/caracal-dev/caracal-software-installer/internal/bootstrap"
	"github.com/caracal-dev/caracal-software-installer/internal/ui"
)

func main() {
	loaded, err := bootstrap.Load()
	if err != nil {
		log.Fatal(err)
	}

	app := ui.New(loaded.Categories, loaded.Logo)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
