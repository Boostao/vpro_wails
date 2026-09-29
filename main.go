package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dataDir, err := userDataDir()
	if err != nil {
		log.Fatal(err)
	}
	configDir, err := userConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	projects, err := NewProjectServiceWithConfig(dataDir, configDir)
	if err != nil {
		log.Fatal(err)
	}
	app := application.New(application.Options{
		Name:        "VPRO",
		Description: "Vegetation Processor",
		Services: []application.Service{
			application.NewService(projects),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "VPRO",
		Width:            1180,
		Height:           760,
		MinWidth:         760,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(247, 249, 247),
		URL:              "/",
	})
	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
