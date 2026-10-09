package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func pictureNativeChooserOwner(ctx context.Context) (*application.App, *application.WebviewWindow, error) {
	if ctx == nil {
		return nil, nil, errors.New("picture selection requires a native request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	window, ok := ctx.Value(application.WindowKey).(*application.WebviewWindow)
	if !ok || window == nil || window.NativeWindow() == nil {
		return nil, nil, errors.New("picture selection requires its live native request window")
	}
	app := application.Get()
	if app == nil || app.Window == nil || app.Dialog == nil {
		return nil, nil, errors.New("picture selection requires the active native application")
	}
	registered, present := app.Window.GetByID(window.ID())
	native, ok := registered.(*application.WebviewWindow)
	if !present || !ok || native != window {
		return nil, nil, errors.New("picture selection request window is no longer owned by this application")
	}
	return app, window, nil
}

func chooseNativePictureFile(ctx context.Context) (string, error) {
	app, window, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	handle := window.NativeWindow()
	if handle == nil {
		return "", errors.New("native picture selection window closed before opening its dialog")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selected, err := promptNativePictureFile(app, window, handle)
	if err != nil {
		return "", fmt.Errorf("native picture selection failed: %w", err)
	}
	currentApp, current, err := pictureNativeChooserOwner(ctx)
	if err != nil {
		return "", err
	}
	if currentApp != app || current != window || current.NativeWindow() != handle {
		return "", errors.New("native picture selection owner changed while the dialog was open")
	}
	if selected == "" {
		return "", nil
	}
	if _, _, err := splitPictureCreationPath(selected); err != nil {
		return "", fmt.Errorf("native picture selection cannot preserve the returned literal path: %w", err)
	}
	return selected, nil
}
