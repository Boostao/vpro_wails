//go:build !windows

package main

import (
	"errors"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func promptNativePictureFile(app *application.App, window *application.WebviewWindow, handle unsafe.Pointer) (string, error) {
	if app == nil || app.Dialog == nil || window == nil || handle == nil || window.NativeWindow() != handle {
		return "", errors.New("native picture selection window changed before its dialog opened")
	}
	return app.Dialog.OpenFile().
		AttachToWindow(window).
		CanChooseFiles(true).
		CanChooseDirectories(false).
		SetTitle("Add a Picture").
		PromptForSingleSelection()
}
