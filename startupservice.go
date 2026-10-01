package main

import (
	"log"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type StartupState struct {
	Ready      bool   `json:"ready"`
	Message    string `json:"message"`
	ConfigPath string `json:"configPath"`
}

type StartupService struct {
	state StartupState
}

func (s *StartupService) GetState() StartupState {
	return s.state
}

func runStartupRecovery(windows application.WindowsOptions, configDir string, cause error) {
	log.Printf("Saved context remains unavailable: %v", cause)
	startup := &StartupService{state: StartupState{Message: cause.Error(), ConfigPath: filepath.Join(configDir, "config.yml")}}
	var app *application.App
	closeService, err := NewCloseService(func(nonce string) bool { return app.Event.Emit(closeRequestEvent, nonce) }, func() { app.Quit() })
	if err != nil {
		log.Fatal(err)
	}
	app = application.New(application.Options{
		Name: "VPRO", Windows: windows, ShouldQuit: closeService.shouldQuit,
		Services: []application.Service{application.NewService(startup), application.NewService(closeService)},
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "VPRO", Width: 1000, Height: 720, MinWidth: 560, MinHeight: 520, URL: "/",
	})
	window.RegisterHook(events.Common.WindowClosing, closeService.handleWindowClosing)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
